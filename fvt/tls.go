// Copyright 2022 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fvt

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

type CertGenerator struct {
	Namespaces    []string
	ServiceName   string
	CAPEM         *bytes.Buffer
	PublicKeyPEM  *bytes.Buffer
	PrivateKeyPEM *bytes.Buffer
}

func (g *CertGenerator) generate() error {

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(3008),
		Subject: pkix.Name{
			Organization: []string{"KServe"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		return err
	}

	dnsNames := make([]string, len(g.Namespaces)+1)
	dnsNames[0] = g.ServiceName
	for i, ns := range g.Namespaces {
		dnsNames[i+1] = fmt.Sprintf("%s.%s", g.ServiceName, ns)
	}

	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
		Subject:      ca.Subject,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(0, 0, 0, 0), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     dnsNames,
	}

	certPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		return err
	}

	caBytes, err := x509.CreateCertificate(cryptorand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(cryptorand.Reader, certTemplate, ca, &certPrivKey.PublicKey, certPrivKey)
	if err != nil {
		return err
	}

	g.CAPEM = new(bytes.Buffer)
	if err = pem.Encode(g.CAPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return err
	}

	g.PublicKeyPEM = new(bytes.Buffer)
	if err = pem.Encode(g.PublicKeyPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return err
	}

	g.PrivateKeyPEM = new(bytes.Buffer)
	privBytes, err := x509.MarshalPKCS8PrivateKey(certPrivKey)
	if err != nil {
		return err
	}

	if err = pem.Encode(g.PrivateKeyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}); err != nil {
		return err
	}

	return nil
}
