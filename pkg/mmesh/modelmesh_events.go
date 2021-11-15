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
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
	etcd3 "go.etcd.io/etcd/client/v3"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// ModelMeshEventStream generates events for Predictors based on
// changes to models/vmodels within model-mesh's etcd-based registries.
// This is unfortunately tightly coupled to certain model-mesh internal
// details (i.e. etcd layout), and in future we would like to expose
// streaming gRPC APIs from model-mesh for an "official" way to watch
// for model events.
type ModelMeshEventStream struct {
	k8sClient          k8sClient.Client
	etcdClient         *etcd3.Client
	etcdRootPrefix     string
	watchedServiceName string
	watchedEtcdSecret  string

	MMEvents    chan event.GenericEvent
	ctx         context.Context
	cancelWatch context.CancelFunc

	logger logr.Logger

	//TODO multi-namespace TBD
	namespace string
}

const (
	ModelRegistryPrefix  = "registry"
	VModelRegistryPrefix = "vmodels"
)

func NewModelEventStream(logger logr.Logger, k8sClient k8sClient.Client,
	namespace string) (this *ModelMeshEventStream, err error) {
	this = new(ModelMeshEventStream)
	this.logger = logger
	this.namespace = namespace

	this.k8sClient = k8sClient

	// These will get set on service reconciling
	this.etcdClient = nil
	this.etcdRootPrefix = ""

	this.MMEvents = make(chan event.GenericEvent, 512) //TODO buffer size TBD

	this.ctx = context.Background() // context.WithCancel(context.Background()) //TODO cancellation

	go func() {
		<-this.ctx.Done()
		close(this.MMEvents)
	}()

	return this, nil
}

func (mes *ModelMeshEventStream) IsWatching() bool {
	return mes.cancelWatch != nil
}

func (mes *ModelMeshEventStream) UpdateWatchedService(ctx context.Context, etcdSecretName string, serviceName string) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName must not be an empty string")
	}
	if etcdSecretName == "" {
		return fmt.Errorf("etcdSecretName must not be an empty string")
	}

	serviceNameChanged := serviceName != mes.watchedServiceName
	etcdSecretChanged := etcdSecretName != mes.watchedEtcdSecret

	if !serviceNameChanged && !etcdSecretChanged {
		// nothing changed, nothing to update
		return nil
	}

	var watchCtx context.Context
	if mes.cancelWatch != nil {
		mes.cancelWatch()
		mes.cancelWatch = nil
	}

	if etcdSecretChanged {
		mes.logger.V(1).Info("Etcd config secret changed. Creating a new etcd client and restarting watchers.",
			"oldSecretName", mes.watchedEtcdSecret, "newSecretName", etcdSecretName)
		if mes.etcdClient != nil {
			err := mes.etcdClient.Close()
			if err != nil {
				mes.logger.Error(err, "Could not close existing etcd client")
			}
		}
		err := mes.connectToEtcd(ctx, etcdSecretName)
		if err != nil {
			return fmt.Errorf("Could not create etcd client: %w", err)
		}
		mes.watchedEtcdSecret = etcdSecretName
	}

	servicePrefix := fmt.Sprintf("%s/%s/%s", mes.etcdRootPrefix, modelmesh.ModelMeshEtcdPrefix, serviceName)
	mes.logger.Info("Initialize Model Event Stream", "servicePrefix", servicePrefix)

	watchCtx, mes.cancelWatch = context.WithCancel(mes.ctx)

	vmodelRegistryPrefix := fmt.Sprintf("%s/%s/", servicePrefix, VModelRegistryPrefix)
	NewEtcdRangeWatcher(mes.logger, mes.etcdClient, vmodelRegistryPrefix).
		Start(watchCtx, false, func(eventType KeyEventType, key string, value []byte) {
			if eventType != UPDATE && (eventType != DELETE || value == nil) {
				mes.logger.V(1).Info("ModelMesh VModel Event", "vModelId", key, "event", eventType)
				return
			}
			if owner, err := ownerIDFromVModelRecord(value); err == nil {
				namespace := mes.namespace
				if owner != "" {
					namespace = fmt.Sprintf("%s_%s", owner, namespace)
				}
				mes.logger.V(1).Info("ModelMesh VModel Event", "vModelId", key, "owner", owner, "event", eventType)
				mes.MMEvents <- event.GenericEvent{Object: &v1.PartialObjectMetadata{
					ObjectMeta: v1.ObjectMeta{Name: key, Namespace: namespace},
				}}
			} else {
				mes.logger.Error(err, "Error parsing VModel record to determine owner, ignoring event",
					"vModelId", key, "event", eventType)
			}
		})

	modelRegistryPrefix := fmt.Sprintf("%s/%s/", servicePrefix, ModelRegistryPrefix)
	NewEtcdRangeWatcher(mes.logger, mes.etcdClient, modelRegistryPrefix).
		Start(watchCtx, true, func(eventType KeyEventType, key string, _ []byte) {
			mes.logger.V(1).Info("ModelMesh Model Event", "modelId", key, "event", eventType)
			if eventType == UPDATE {
				// key is like "vmodelname__owner-0123456789"
				ownerIdx := strings.LastIndex(key, "__") + 2
				if ownerIdx > 2 {
					hashIdx := len(key) - 11 // 11 is ('-' plus 10 hash chars)
					if hashIdx > ownerIdx && key[hashIdx] == '-' {
						// Infer predictor/vmodel and source ids from concrete model id by removing hash suffix
						sourceId, predictorName := key[ownerIdx:hashIdx], key[:ownerIdx-2]
						mes.MMEvents <- event.GenericEvent{Object: &v1.PartialObjectMetadata{ObjectMeta: v1.ObjectMeta{
							Name:      predictorName,
							Namespace: fmt.Sprintf("%s_%s", sourceId, mes.namespace),
						}}}
						return
					}
				}
				mes.logger.Info("Ignoring event for unrecognized ModelMesh model",
					"modelId", key, "eventType", eventType)
			}
		})

	// wait until just before returning to set this so we know we didn't have any errors
	mes.watchedServiceName = serviceName
	return nil
}

func ownerIDFromVModelRecord(data []byte) (string, error) {
	type record struct{ O string } // owner field is called "o"; ignore others
	vmr := record{}
	if err := json.Unmarshal(data, &vmr); err != nil {
		return "", err
	}
	return vmr.O, nil
}

func (mes *ModelMeshEventStream) connectToEtcd(ctx context.Context, secretName string) error {
	etcdSecret := v12.Secret{}
	err := mes.k8sClient.Get(ctx, k8sClient.ObjectKey{Name: secretName, Namespace: mes.namespace}, &etcdSecret)
	if err != nil {
		return fmt.Errorf("Unable to access etcd secret with name '%s': %w", secretName, err)
	}
	etcdSecretJsonData, ok := etcdSecret.Data[modelmesh.EtcdSecretKey]
	if !ok {
		return fmt.Errorf("Key '%s' was not found in etcd secret '%s'", modelmesh.EtcdSecretKey, secretName)
	}

	var etcdConfig EtcdConfig
	err = json.Unmarshal(etcdSecretJsonData, &etcdConfig)
	if err != nil {
		return fmt.Errorf("Failed to Parse Etcd Config Json: %w", err)
	}

	mes.etcdClient, err = CreateEtcdClient(etcdConfig, etcdSecret.Data, mes.logger)
	if err != nil {
		return fmt.Errorf("Failed to connect to Etcd: %w", err)
	}
	mes.etcdRootPrefix = etcdConfig.RootPrefix

	// TODO should we test the etcd client connection here? Otherwise there's no failure even if the URL is bad
	return nil
}

//func (mes *ModelMeshEventStream) Stop() {
//	//TODO cancel mes.ctx here
//	mes.grpcConn.Close()
//	mes.etcdClient.Close()
//}
