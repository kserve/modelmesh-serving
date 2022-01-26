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
	Namespace     string
	PublicKeyPEM  *bytes.Buffer
	PrivateKeyPEM *bytes.Buffer
}

func (g *CertGenerator) generate() error {
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
		Subject: pkix.Name{
			Organization: []string{"KServe"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(0, 0, 0, 0), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{fmt.Sprintf("modelmesh-serving.%s", g.Namespace), "modelmesh-serving"},
	}

	certPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(cryptorand.Reader, certTemplate, certTemplate, &certPrivKey.PublicKey, certPrivKey)
	if err != nil {
		return err
	}

	g.PublicKeyPEM = new(bytes.Buffer)
	err = pem.Encode(g.PublicKeyPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return err
	}

	g.PrivateKeyPEM = new(bytes.Buffer)

	privBytes, err := x509.MarshalPKCS8PrivateKey(certPrivKey)
	if err != nil {
		return err
	}
	err = pem.Encode(g.PrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})
	if err != nil {
		return err
	}

	return nil
}
