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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	clientv3 "go.etcd.io/etcd/client/v3"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	etcdEndpoint = "https://0.0.0.0:2379"
	// copied key/cert from https://github.com/IBM/etcd-java/blob/master/src/test/resources
	certificateFile = "testdata/client-certificate"
	keyFile         = "testdata/client-key"
	user            = "user"
	password        = "password"
	logger          = zap.New()
)

func Test_GetEtcdClientConfig_ErrFileDoesNotExist(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:       etcdEndpoint,
		CertificateFile: "path/to/certificate"}
	expectedError := "referenced TLS certificate secret key not found: path/to/certificate"

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)
	assert.Empty(t, etcdClientConfig)
	assert.EqualError(t, err, expectedError)
}

func Test_GetEtcdClientConfig_ErrOnlyKeyProvided(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints: etcdEndpoint,
		ClientKey: "myClientKey"}
	expectedError := fmt.Errorf("need to set both client_key/client_key_file and client_certificate/client_certificate_file")

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)

	assert.Empty(t, etcdClientConfig)
	assert.Equal(t, expectedError, err)
}

func Test_GetEtcdClientConfig_ErrKeyAndCert(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:         etcdEndpoint,
		ClientKey:         "myClientKey",
		ClientCertificate: "myClientCertificate"}
	expectedError := fmt.Errorf("could not load client key pair: %w",
		fmt.Errorf("tls: failed to find any PEM data in certificate input"))

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)

	assert.Equal(t, expectedError, err)
	assert.Empty(t, etcdClientConfig)
}

func Test_GetEtcdClientConfig_SuccessWithCertOverwrite(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:       etcdEndpoint,
		Username:        user,
		Password:        password,
		Certificate:     "myCertificate",
		CertificateFile: certificateFile}

	certData, _ := os.ReadFile(certificateFile)
	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{
		certificateFile: certData,
	}, logger)

	assert.Nil(t, err)
	assert.IsType(t, &clientv3.Config{}, etcdClientConfig)
	assert.Equal(t, etcdEndpoint, etcdClientConfig.Endpoints[0])
	assert.Equal(t, etcdDialTimeout, etcdClientConfig.DialTimeout)
	assert.NotNil(t, etcdClientConfig.TLS.RootCAs)
	assert.Nil(t, etcdClientConfig.TLS.Certificates)
	assert.Equal(t, user, etcdClientConfig.Username)
	assert.Equal(t, password, etcdClientConfig.Password)
}

func Test_GetEtcdClientConfig_SuccessWithKeyAndCert(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:             etcdEndpoint,
		Certificate:           "myCertificate",
		ClientKey:             "myClientKey",
		ClientKeyFile:         keyFile,
		ClientCertificate:     "myClientCertificate",
		ClientCertificateFile: certificateFile}

	certData1, _ := os.ReadFile(keyFile)
	certData2, _ := os.ReadFile(certificateFile)
	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{
		keyFile:         certData1,
		certificateFile: certData2,
	}, logger)

	assert.Nil(t, err)
	assert.IsType(t, &clientv3.Config{}, etcdClientConfig)
	assert.NotNil(t, etcdClientConfig.TLS.RootCAs)
	assert.NotNil(t, etcdClientConfig.TLS.Certificates)
}

func Test_GetEtcdClientConfig_SuccessWithNoCert(t *testing.T) {
	etcdConfig := EtcdConfig{Endpoints: etcdEndpoint}

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)

	assert.Nil(t, err)
	assert.IsType(t, &clientv3.Config{}, etcdClientConfig)
	assert.Equal(t, etcdEndpoint, etcdClientConfig.Endpoints[0])
	assert.NotNil(t, etcdClientConfig.TLS.RootCAs)
}

func Test_GetEtcdClientConfig_SuccessWithNoCertHttp(t *testing.T) {
	etcdConfig := EtcdConfig{Endpoints: "http://0.0.0.0:2379"}

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)

	assert.Nil(t, err)
	assert.IsType(t, &clientv3.Config{}, etcdClientConfig)
	assert.Equal(t, "http://0.0.0.0:2379", etcdClientConfig.Endpoints[0])
	assert.Nil(t, etcdClientConfig.TLS)
}

func Test_GetEtcdClientConfig_OverrideAuth(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:         etcdEndpoint,
		OverrideAuthority: "my-override",
	}

	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, map[string][]byte{}, logger)

	assert.Nil(t, err)
	assert.IsType(t, &clientv3.Config{}, etcdClientConfig)
	assert.NotNil(t, etcdClientConfig.TLS.RootCAs)
	assert.Equal(t, 1, len(etcdClientConfig.DialOptions))
}

func Test_CreateEtcdClient_Success(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:             etcdEndpoint,
		ClientKeyFile:         keyFile,
		ClientCertificateFile: certificateFile}

	certData1, _ := os.ReadFile(keyFile)
	certData2, _ := os.ReadFile(certificateFile)
	etcdClient, err := CreateEtcdClient(etcdConfig, map[string][]byte{
		keyFile:         certData1,
		certificateFile: certData2,
	}, logger)

	assert.Nil(t, err)
	assert.NotEmpty(t, etcdClient)
	etcdClient.Close()
}

func Test_CreateEtcdClient_Fail(t *testing.T) {
	etcdConfig := EtcdConfig{
		Endpoints:         etcdEndpoint,
		ClientKey:         "myClientKey",
		ClientCertificate: "myClientCertificate"}
	expectedError := "failed to create etcd client config: " +
		"could not load client key pair: tls: failed to find any PEM data in certificate input"

	etcdClient, err := CreateEtcdClient(etcdConfig, map[string][]byte{}, logger)
	assert.Equal(t, expectedError, err.Error())
	assert.Empty(t, etcdClient)
}
