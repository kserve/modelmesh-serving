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
	"sync"
	"unsafe"

	"google.golang.org/grpc/credentials/insecure"

	"github.com/go-logr/logr"
	"github.com/kserve/modelmesh-serving/pkg/config"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	mmeshapi "github.com/kserve/modelmesh-serving/generated/mmesh"
)

type TLSConfigLookup func(context.Context, string) (*tls.Config, error)

// Encapsulates ModelMesh gRPC service
type MMService struct {
	// these don't change
	Log       logr.Logger
	namespace string
	tlsConfig TLSConfigLookup

	// these protected by mutex
	name               string
	port               uint16
	restPort           uint16
	managementEndpoint string
	headless           bool
	tlsSecretName      string
	metricsPort        uint16
	reconnect          bool // indicates dirty client
	serviceSpec        *v1.ServiceSpec

	// updates protected by mutex, read with atomic load
	mmClient atomic.UnsafePointer // stores type *mmClient

	mutex sync.Mutex
}

func NewMMService(namespace string, tlsConfig TLSConfigLookup) *MMService {
	return &MMService{
		Log:       ctrl.Log.WithName("MMService").WithValues("namespace", namespace),
		namespace: namespace,
		tlsConfig: tlsConfig,
	}
}

func (mms *MMService) dnsName() string {
	return fmt.Sprintf("%s.%s", mms.name, mms.namespace)
}

func (mms *MMService) GetNameAndSpec() (string, *v1.ServiceSpec) {
	mms.mutex.Lock()
	defer mms.mutex.Unlock()
	return mms.name, mms.serviceSpec
}

func (mms *MMService) UpdateConfig(cp *config.ConfigProvider) (*config.Config, bool) {
	mms.mutex.Lock()
	defer mms.mutex.Unlock()

	cfg := cp.GetConfig()

	specChange, clientChange := false, false
	if cfg.InferenceServiceName != mms.name {
		mms.name = cfg.InferenceServiceName
		specChange = true
		clientChange = true
	}
	if cfg.InferenceServicePort != mms.port {
		mms.port = cfg.InferenceServicePort
		specChange = true
		clientChange = true
	}
	var restPort uint16
	if cfg.RESTProxy.Enabled {
		restPort = cfg.RESTProxy.Port
	}
	if restPort != mms.restPort {
		mms.restPort = restPort
		specChange = true
	}
	endpoint := cfg.ModelMeshEndpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s:///%s:%d", KUBE_SCHEME, mms.dnsName(), mms.port)
	}
	if endpoint != mms.managementEndpoint {
		mms.managementEndpoint = endpoint
		clientChange = true
	}
	if cfg.TLS.SecretName != mms.tlsSecretName {
		mms.tlsSecretName = cfg.TLS.SecretName
		clientChange = true
	}
	if cfg.HeadlessService != mms.headless {
		mms.headless = cfg.HeadlessService
		specChange = true
		clientChange = true // technically not but good to reconnect
	}
	var metricsPort uint16 = 0
	if cfg.Metrics.Enabled {
		metricsPort = cfg.Metrics.Port
	}
	if metricsPort != mms.metricsPort {
		mms.metricsPort = metricsPort
		specChange = true
	}

	if specChange {
		spec := &v1.ServiceSpec{
			Selector: map[string]string{"modelmesh-service": mms.name},
			Ports: []v1.ServicePort{{
				Name:       "grpc",
				Port:       int32(mms.port),
				TargetPort: intstr.FromString("grpc"),
			}},
		}
		if restPort > 0 {
			spec.Ports = append(spec.Ports, v1.ServicePort{
				Name:       "http",
				Port:       int32(restPort),
				TargetPort: intstr.FromString("http"),
			})
			spec.Ports = append(spec.Ports, v1.ServicePort{
				Name:       "https",
				Port:       int32(8443),
				TargetPort: intstr.FromString("https"),
			})
		}
		if mms.headless {
			spec.ClusterIP = "None"
		}
		if metricsPort > 0 {
			spec.Ports = append(spec.Ports, v1.ServicePort{
				Name:       "prometheus",
				Port:       int32(metricsPort),
				TargetPort: intstr.FromString("prometheus"),
			})
		}
		mms.serviceSpec = spec
		mms.Log.Info("Updated target ServiceSpec")
	}
	if clientChange {
		mms.reconnect = true
	}

	return cfg, specChange || clientChange
}

type mmClient struct {
	grpcConn     *grpc.ClientConn
	mmeshClient  mmeshapi.ModelMeshClient
	endpoint     string
	restEndpoint string
}

func (mms *MMService) mmc() *mmClient {
	return (*mmClient)(mms.mmClient.Load())
}

func (mms *MMService) InferenceEndpoints() (grpc, rest string) {
	if mmc := mms.mmc(); mmc != nil {
		return mmc.endpoint, mmc.restEndpoint
	}
	return "", ""
}

// MMClient is called from predictor controller
func (mms *MMService) MMClient() mmeshapi.ModelMeshClient {
	if mmc := mms.mmc(); mmc != nil {
		return mmc.mmeshClient
	}
	return nil
}

// ConnectIfNeeded is called only in service reconcile
func (mms *MMService) ConnectIfNeeded(ctx context.Context) error {
	mms.mutex.Lock()

	if mms.mmc() != nil && !mms.reconnect {
		mms.mutex.Unlock()
		return nil
	}

	tlsSecret := mms.tlsSecretName
	endpoint := mms.managementEndpoint
	dnsName := mms.dnsName()
	inferenceEndpoint := fmt.Sprintf("grpc://%s:%d", dnsName, mms.port)
	restEndpoint := ""
	if mms.restPort > 0 {
		scheme := "http"
		if tlsSecret != "" {
			scheme = "https"
		}
		restEndpoint = fmt.Sprintf("%s://%s:%d", scheme, dnsName, mms.restPort)
	}
	mms.reconnect = false
	mms.mutex.Unlock()

	var tlsConfig *tls.Config
	if tlsSecret != "" {
		var err error
		if tlsConfig, err = mms.tlsConfig(ctx, tlsSecret); err != nil {
			return err
		}
	}

	mmc, err := newMmClient(ctx, endpoint, tlsConfig, dnsName,
		inferenceEndpoint, restEndpoint)
	if err != nil {
		mms.mutex.Lock()
		defer mms.mutex.Unlock()
		mms.reconnect = true
		return err
	}
	mms.Disconnect()
	mms.mmClient.Store(unsafe.Pointer(mmc))
	mms.Log.Info("Established new MM gRPC connection",
		"endpoint", endpoint, "TLS", tlsSecret != "")
	return nil
}

// Disconnect is called only in service reconcile
func (mms *MMService) Disconnect() {
	if mmc := mms.mmc(); mmc != nil {
		if err := mmc.grpcConn.Close(); err == nil {
			mms.mmClient.Store(nil)
		} else {
			mms.Log.Error(err, "Error closing MM gRPC connection")
		}
	}
}

func newMmClient(ctx context.Context, mmeshEndpoint string, tlsConfig *tls.Config,
	serviceName, externalEndpoint, restEndpoint string) (*mmClient, error) {
	//grpcCtx, cancel := context.WithTimeout(context.Background(), GrpcDialTimeout) //TODO TBD

	dialOpts := make([]grpc.DialOption, 1, 3)
	dialOpts[0] = grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`)
	if tlsConfig == nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tc := credentials.NewTLS(tlsConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(tc), grpc.WithAuthority(serviceName))
	}
	grpcConn, err := grpc.DialContext(ctx, mmeshEndpoint, dialOpts...)
	if err != nil {
		//logger.Error(err, "failed to connect to model mesh service")
		return nil, err
	}
	return &mmClient{grpcConn, mmeshapi.NewModelMeshClient(grpcConn), externalEndpoint, restEndpoint}, nil
}
