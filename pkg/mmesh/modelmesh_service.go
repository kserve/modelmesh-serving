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
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	mmeshapi "github.com/kserve/modelmesh-serving/generated/mmesh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Encapsulates ModelMesh gRPC service
type MMService struct {
	Log logr.Logger

	Name               string
	Port               uint16
	ManagementEndpoint string
	Headless           bool

	tlsSecretName string
	TLSConfig     *tls.Config

	mmClient *mmClient

	MetricsPort uint16
	RESTPort    uint16
}

func NewMMService() *MMService {
	return &MMService{Log: ctrl.Log.WithName("MMService")}
}

func (mms *MMService) UpdateConfig(name string, port uint16,
	endpoint, tlsSecretName string, tlsConfig *tls.Config, headless bool, metricsPort uint16, restPort uint16) bool {
	changed := false
	if name != mms.Name {
		mms.Name = name
		changed = true
	}
	if port != mms.Port {
		mms.Port = port
		changed = true
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s:///%s", KUBE_SCHEME, mms.InferenceEndpoint())
	}
	if endpoint != mms.ManagementEndpoint {
		mms.ManagementEndpoint = endpoint
		changed = true
	}
	if tlsSecretName != mms.tlsSecretName {
		mms.TLSConfig = tlsConfig
		mms.tlsSecretName = tlsSecretName
		changed = true
	}
	if headless != mms.Headless {
		mms.Headless = headless
		changed = true
	}
	if metricsPort != mms.MetricsPort {
		mms.MetricsPort = metricsPort
		changed = true
	}
	if restPort != mms.RESTPort {
		mms.RESTPort = restPort
		changed = true
	}
	return changed
}

type mmClient struct {
	grpcConn    *grpc.ClientConn
	mmeshClient mmeshapi.ModelMeshClient
}

func (mms *MMService) InferenceEndpoint() string {
	return fmt.Sprintf("%s:%d", mms.Name, mms.Port)
}

func (mms *MMService) InferenceRESTEndpoint() string {
	if mms.RESTPort <= 0 {
		return ""
	}
	scheme := "http"
	if mms.TLSConfig != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, mms.Name, mms.RESTPort)
}

func (mms *MMService) MMClient() mmeshapi.ModelMeshClient {
	mmc := mms.mmClient
	if mmc == nil {
		return nil
	}
	return mmc.mmeshClient
}

func (mms *MMService) Connect() error {
	mmc, err := newMmClient(mms.ManagementEndpoint, mms.TLSConfig, mms.Name, &mms.Log)
	if err == nil {
		mms.Disconnect()
		mms.mmClient = mmc
		mms.Log.Info("Established new MM gRPC connection",
			"endpoint", mms.ManagementEndpoint, "TLS", mms.TLSConfig != nil)
	}
	return err
}

func (mms *MMService) Disconnect() {
	if mms.mmClient != nil {
		err := mms.mmClient.grpcConn.Close()
		if err == nil {
			mms.mmClient = nil
		} else {
			mms.Log.Error(err, "Error closing MM gRPC connection")
		}
	}
}

func newMmClient(mmeshEndpoint string, tlsConfig *tls.Config,
	serviceName string, logger *logr.Logger) (*mmClient, error) {
	//grpcCtx, cancel := context.WithTimeout(context.Background(), GrpcDialTimeout)
	grpcCtx, cancel := context.WithCancel(context.Background()) //TODO
	defer cancel()

	var tlsOption grpc.DialOption
	if tlsConfig == nil {
		tlsOption = grpc.WithInsecure()
	} else {
		tc := credentials.NewTLS(tlsConfig)
		err := tc.OverrideServerName(serviceName)
		if err != nil {
			(*logger).Error(err, "Error overriding TLS server name", "serverName", serviceName)
			// continue anyhow
		}
		tlsOption = grpc.WithTransportCredentials(tc)
	}
	grpcConn, err := grpc.DialContext(grpcCtx, mmeshEndpoint, tlsOption,
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		//logger.Error(err, "failed to connect to model mesh service")
		return nil, err
	}
	return &mmClient{grpcConn, mmeshapi.NewModelMeshClient(grpcConn)}, nil
}
