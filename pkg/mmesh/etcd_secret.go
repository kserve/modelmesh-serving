// Copyright 2021 IBM Corporation
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
package mmesh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EtcdConnectionKey        = "etcd_connection"
	EndpointsKey             = "endpoints"
	UsernameKey              = "userid"
	PasswordKey              = "password"
	RootPrefixKey            = "root_prefix"
	CertificateKey           = "certificate"
	CertificateFileKey       = "certificate_file"
	ClientKeyKey             = "client_key"
	ClientKeyFileKey         = "client_key_file"
	ClientCertificateKey     = "client_certificate"
	ClientCertificateFileKey = "client_certificate_file"
	OverrideAuthorityKey     = "override_authority"
)

// An EtcdSecret represents the etcd configuation for a
// user namespace
type EtcdSecret struct {
	Log                 logr.Logger
	Name                string
	Namespace           string
	ControllerNamespace string
	EtcdConfig          *EtcdConfig
	Scheme              *runtime.Scheme
}

func (es EtcdSecret) Apply(ctx context.Context, cl client.Client) error {
	commonLabelValue := "modelmesh-controller"

	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      es.Name,
			Namespace: es.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": commonLabelValue,
			},
		},
	}
	es.addStringData(s)
	err := cl.Create(ctx, s)
	if err != nil && errors.IsAlreadyExists(err) {
		err = cl.Update(ctx, s)
	}

	return err
}

// Add string data to the provided secret
func (es EtcdSecret) addStringData(s *corev1.Secret) {
	m := make(map[string]interface{})
	// debugging, remove later
	es.Log.Info("============in EtcdSecret=========", "es.EtcdConfig", es.EtcdConfig)

	etcdEndpoints := strings.Split(es.EtcdConfig.Endpoints, ",")

	newEtcdEndpoints := ""
	// For each endpoint, insert controller namespace if not there
	// TODO: revisit and come up with ideal solution
	for i := range etcdEndpoints {
		if !strings.Contains(etcdEndpoints[i], ".") {
			// This looks like http://etcd:2379, so insert controller namespace before :port
			parts := strings.Split(etcdEndpoints[i], ":")
			etcdEndpoints[i] = parts[0] + ":" + parts[1] + "." + es.ControllerNamespace + ":" + parts[2]
		}
		if i != 0 {
			newEtcdEndpoints = newEtcdEndpoints + ","
		}
		newEtcdEndpoints = newEtcdEndpoints + etcdEndpoints[i]
	}

	m[EndpointsKey] = newEtcdEndpoints
	m[RootPrefixKey] = fmt.Sprintf("%s/mm_ns/%s", es.EtcdConfig.RootPrefix, es.Namespace)
	if es.EtcdConfig.Username != "" {
		m[UsernameKey] = es.EtcdConfig.Username
	}
	if es.EtcdConfig.Password != "" {
		m[PasswordKey] = es.EtcdConfig.Password
	}
	if es.EtcdConfig.Certificate != "" {
		m[CertificateKey] = es.EtcdConfig.Certificate
	}
	if es.EtcdConfig.CertificateFile != "" {
		m[CertificateFileKey] = es.EtcdConfig.CertificateFile
	}
	if es.EtcdConfig.ClientKey != "" {
		m[ClientKeyKey] = es.EtcdConfig.ClientKey
	}
	if es.EtcdConfig.ClientKeyFile != "" {
		m[ClientKeyFileKey] = es.EtcdConfig.ClientKeyFile
	}
	if es.EtcdConfig.ClientCertificate != "" {
		m[ClientCertificateKey] = es.EtcdConfig.ClientCertificate
	}
	if es.EtcdConfig.ClientCertificateFile != "" {
		m[ClientCertificateFileKey] = es.EtcdConfig.ClientCertificateFile
	}
	if es.EtcdConfig.OverrideAuthority != "" {
		m[OverrideAuthorityKey] = es.EtcdConfig.OverrideAuthority
	}

	t, _ := json.Marshal(m)
	if s.StringData == nil {
		s.StringData = make(map[string]string)
	}
	s.StringData[EtcdConnectionKey] = string(t)
}
