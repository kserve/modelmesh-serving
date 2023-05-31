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
	controllerNamespace string
	k8sClient           k8sClient.Client

	secretName     string
	etcdClient     *etcd3.Client
	etcdRootPrefix string

	// accessed only in UpdateWatchedService func, called only from service reconcile func
	watchedServices map[string]*namespaceWatch

	MMEvents chan event.GenericEvent
	ctx      context.Context

	logger logr.Logger
}

type namespaceWatch struct {
	watchedServiceName string
	cancelFunc         context.CancelFunc
}

func (nw *namespaceWatch) cancelWatch() {
	if nw.cancelFunc != nil {
		nw.cancelFunc()
		nw.cancelFunc = nil
	}
}

const (
	ModelRegistryPrefix  = "registry"
	VModelRegistryPrefix = "vmodels"
)

func NewModelEventStream(logger logr.Logger, k8sClient k8sClient.Client,
	namespace string) (this *ModelMeshEventStream, err error) {
	this = new(ModelMeshEventStream)
	this.logger = logger
	this.controllerNamespace = namespace

	this.k8sClient = k8sClient

	// These will get set on service reconciling
	this.etcdClient = nil
	this.etcdRootPrefix = ""

	this.watchedServices = map[string]*namespaceWatch{namespace: {}}

	this.MMEvents = make(chan event.GenericEvent, 512) //TODO buffer size TBD

	this.ctx = context.Background() // context.WithCancel(context.Background()) //TODO cancellation

	go func() {
		<-this.ctx.Done()
		close(this.MMEvents)
	}()

	return this, nil
}

// UpdateWatchedService is called from service reconciler
func (mes *ModelMeshEventStream) UpdateWatchedService(ctx context.Context,
	etcdSecretName, serviceName, namespace string) error {

	if serviceName == "" {
		return fmt.Errorf("serviceName must not be an empty string")
	}
	if etcdSecretName == "" {
		return fmt.Errorf("etcdSecretName must not be an empty string")
	}

	nw, ok := mes.watchedServices[namespace]
	if !ok {
		nw = &namespaceWatch{}
		mes.watchedServices[namespace] = nw
	}

	if etcdSecretName != mes.secretName {
		// etcd config secret changed
		mes.logger.V(1).Info("Etcd config secret changed. Creating a new etcd client and restarting watchers.",
			"oldSecretName", mes.secretName, "newSecretName", etcdSecretName)
		for _, w := range mes.watchedServices {
			w.cancelWatch()
		}
		if mes.etcdClient != nil {
			if err := mes.etcdClient.Close(); err != nil {
				mes.logger.Error(err, "Could not close existing etcd client")
			}
		}
		if err := mes.connectToEtcd(ctx, etcdSecretName); err != nil {
			return fmt.Errorf("Could not create etcd client: %w", err)
		}
		mes.secretName = etcdSecretName

		for n, w := range mes.watchedServices {
			sn := w.watchedServiceName
			if n == namespace {
				sn = serviceName
			}
			mes.refreshWatches(w, n, sn)
		}
	} else if serviceName != nw.watchedServiceName {
		// only service name changed
		nw.cancelWatch()
		mes.refreshWatches(nw, namespace, serviceName)
	}

	return nil
}

// RemoveWatchedService is called from service reconciler
func (mes *ModelMeshEventStream) RemoveWatchedService(serviceName, namespace string) {
	nw, ok := mes.watchedServices[namespace]
	if ok && nw.watchedServiceName == serviceName {
		delete(mes.watchedServices, namespace)
		nw.cancelWatch()
	}
}

func (mes *ModelMeshEventStream) refreshWatches(nw *namespaceWatch, namespace, serviceName string) {
	rp := mes.etcdRootPrefix
	if namespace != mes.controllerNamespace {
		rp = fmt.Sprintf("%s/mm_ns/%s", rp, namespace) //TODO double check root prefix restrictions
	}

	servicePrefix := fmt.Sprintf("%s/%s/%s", rp, modelmesh.ModelMeshEtcdPrefix, serviceName)

	logger := mes.logger.WithValues("namespace", namespace)
	logger.Info("Initialize Model Event Stream", "servicePrefix", servicePrefix)

	var watchCtx context.Context
	watchCtx, nw.cancelFunc = context.WithCancel(mes.ctx)

	vmodelRegistryPrefix := fmt.Sprintf("%s/%s/", servicePrefix, VModelRegistryPrefix)
	NewEtcdRangeWatcher(logger, mes.etcdClient, vmodelRegistryPrefix).
		Start(watchCtx, false, func(eventType KeyEventType, key string, value []byte) {
			if eventType != UPDATE && (eventType != DELETE || value == nil) {
				logger.V(1).Info("ModelMesh VModel Event", "vModelId", key, "event", eventType)
				return
			}
			if owner, err := ownerIDFromVModelRecord(value); err == nil {
				encodedNamespace := namespace
				if owner != "" {
					encodedNamespace = fmt.Sprintf("%s_%s", owner, namespace)
				}
				logger.V(1).Info("ModelMesh VModel Event",
					"vModelId", key, "owner", owner, "event", eventType)
				mes.MMEvents <- event.GenericEvent{Object: &v1.PartialObjectMetadata{
					ObjectMeta: v1.ObjectMeta{Name: key, Namespace: encodedNamespace},
				}}
			} else {
				logger.Error(err, "Error parsing VModel record to determine owner, ignoring event",
					"vModelId", key, "event", eventType)
			}
		})

	modelRegistryPrefix := fmt.Sprintf("%s/%s/", servicePrefix, ModelRegistryPrefix)
	NewEtcdRangeWatcher(logger, mes.etcdClient, modelRegistryPrefix).
		Start(watchCtx, true, func(eventType KeyEventType, key string, _ []byte) {
			logger.V(1).Info("ModelMesh Model Event", "modelId", key, "event", eventType)
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
							Namespace: fmt.Sprintf("%s_%s", sourceId, namespace),
						}}}
						return
					}
				}
				logger.Info("Ignoring event for unrecognized ModelMesh model",
					"modelId", key, "eventType", eventType)
			}
		})

	// wait until just before returning to set this so we know we didn't have any errors
	nw.watchedServiceName = serviceName
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
	var err error
	etcdSecret := v12.Secret{}
	if err = mes.k8sClient.Get(ctx, k8sClient.ObjectKey{Name: secretName, Namespace: mes.controllerNamespace}, &etcdSecret); err != nil {
		return fmt.Errorf("Unable to access etcd secret with name '%s': %w", secretName, err)
	}
	etcdSecretJsonData, ok := etcdSecret.Data[modelmesh.EtcdSecretKey]
	if !ok {
		return fmt.Errorf("Key '%s' was not found in etcd secret '%s'", modelmesh.EtcdSecretKey, secretName)
	}

	var etcdConfig EtcdConfig
	if err = json.Unmarshal(etcdSecretJsonData, &etcdConfig); err != nil {
		return fmt.Errorf("failed to parse etcd config json: %w", err)
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
