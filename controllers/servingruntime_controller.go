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
/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kfsv1alpha1 "github.com/kserve/modelmesh-serving/apis/kfserving/v1alpha1"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
)

// ServingRuntimeReconciler reconciles a ServingRuntime object
type ServingRuntimeReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	ConfigProvider      *ConfigProvider
	ConfigMapName       types.NamespacedName
	DeploymentName      string
	DeploymentNamespace string
	// store some information about current runtimes for making scaling decisions
	runtimeInfoMap          map[types.NamespacedName]*runtimeInfo
	runtimeInfoMapMutex     sync.Mutex
	EnableTrainedModelWatch bool
}

type runtimeInfo struct {
	// used to implement the scale down grace period
	// nil signals that the last check had predictors
	TimeTransitionedToNoPredictors *time.Time
}

var builtInServerTypes = map[api.ServerType]interface{}{
	api.MLServer: nil, api.Triton: nil}

// +kubebuilder:rbac:namespace="model-serving",groups=serving.kserve.io,resources=servingruntimes;servingruntimes/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace="model-serving",groups=serving.kserve.io,resources=servingruntimes/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace="model-serving",groups=apps,resources=deployments;deployments/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace="model-serving",groups="",resources=services;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace="model-serving",groups="",resources=secrets,verbs=get;list;watch

func (r *ServingRuntimeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("servingruntime", req.NamespacedName)
	log.V(1).Info("ServingRuntime reconciler called")

	// Reconcile the model mesh cluster config map
	runtimes := &api.ServingRuntimeList{}
	err := r.Client.List(ctx, runtimes)
	if err != nil {
		return RequeueResult, err
	}

	d := &appsv1.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      r.DeploymentName,
		Namespace: r.DeploymentNamespace,
	}, d)
	if err != nil {
		return RequeueResult, fmt.Errorf("Could not get the controller deployment: %w", err)
	}

	cc := modelmesh.ClusterConfig{
		Runtimes:  runtimes,
		Namespace: req.Namespace,
		Scheme:    r.Scheme,
	}

	if err = cc.Apply(ctx, d, r.Client); err != nil {
		return RequeueResult, fmt.Errorf("Could not apply the modelmesh type-constraints configmap: %w", err)
	}

	//reconcile this serving runtime
	rt := &api.ServingRuntime{}
	err = r.Client.Get(ctx, req.NamespacedName, rt)
	if errors.IsNotFound(err) {
		r.Log.Info("Runtime is not found")

		// remove runtime from info map
		r.runtimeInfoMapMutex.Lock()
		defer r.runtimeInfoMapMutex.Unlock()

		if r.runtimeInfoMap != nil {
			// this is safe even if the entry doesn't exist
			delete(r.runtimeInfoMap, req.NamespacedName)
		}
		return ctrl.Result{}, nil
	}

	// If invalid ServerType is provided in rt.Spec or if this value doesn't match with that of the specified container, delete the deployment
	if err = validateServingRuntimeSpec(rt); err != nil {
		werr := fmt.Errorf("Invalid ServingRuntime Spec: %w", err)
		return ctrl.Result{}, werr
	}

	// construct the deployment
	config := r.ConfigProvider.GetConfig()
	mmDeployment := modelmesh.Deployment{
		ServiceName:        config.InferenceServiceName,
		Name:               req.Name,
		Namespace:          req.Namespace,
		Owner:              rt,
		DefaultVModelOwner: PredictorCRSourceId,
		Log:                log,
		Metrics:            config.Metrics.Enabled,
		PrometheusPort:     config.Metrics.Port,
		PrometheusScheme:   config.Metrics.Scheme,
		ModelMeshImage:     config.ModelMeshImage.TaggedImage(),
		ModelMeshResources: config.ModelMeshResources.ToKubernetesType(),
		PullerImage:        config.StorageHelperImage.TaggedImage(),
		PullerImageCommand: config.StorageHelperImage.Command,
		PullerResources:    config.StorageHelperResources.ToKubernetesType(),
		Port:               config.InferenceServicePort,
		GrpcMaxMessageSize: config.GrpcMaxMessageSizeBytes,
		// Replicas is set below
		TLSSecretName:       config.TLS.SecretName,
		TLSClientAuth:       config.TLS.ClientAuth,
		EtcdSecretName:      config.GetEtcdSecretName(),
		ServiceAccountName:  config.ServiceAccountName,
		EnableAccessLogging: config.EnableAccessLogging,
		Client:              r.Client,
	}

	// if the runtime is disabled, delete the deployment
	if rt.Disabled() {
		log.Info("Deployment is disabled for this runtime")
		err = mmDeployment.Delete(ctx, r.Client)
		if err != nil {
			werr := fmt.Errorf("Could not delete the model mesh deployment: %w", err)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, werr
		}
		return ctrl.Result{}, nil
	}

	replicas, requeueDuration, err := r.determineReplicasAndRequeueDuration(ctx, log, config, rt)
	if err != nil {
		werr := fmt.Errorf("Could not determine replicas: %w", err)
		return RequeueResult, werr
	}
	mmDeployment.Replicas = replicas
	err = mmDeployment.Apply(ctx)
	if err != nil {
		if errors.IsConflict(err) {
			// this can occur during normal operations if the deployment was updated
			// during this reconcile loop
			log.Info("Could not apply model mesh deployment due to resource conflict")
			return RequeueResult, nil
		}
		werr := fmt.Errorf("Could not apply the model mesh deployment: %w", err)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, werr
	}
	return ctrl.Result{RequeueAfter: requeueDuration}, nil
}

func validateServingRuntimeSpec(rt *api.ServingRuntime) error {

	if rt.Spec.BuiltInAdapter == nil {
		return nil // nothing to check
	}
	st := rt.Spec.BuiltInAdapter.ServerType
	if _, ok := builtInServerTypes[st]; !ok {
		return fmt.Errorf("Unrecognized built-in runtime server type %s", st)
	}
	for _, c := range rt.Spec.Containers {
		if c.Name == string(st) {
			return nil // found, all good
		}
	}
	return fmt.Errorf("Must include runtime Container with name %s", st)
}

func (r *ServingRuntimeReconciler) determineReplicasAndRequeueDuration(ctx context.Context, log logr.Logger, config *Config, rt *api.ServingRuntime) (uint16, time.Duration, error) {
	var err error
	const scaledToZero = uint16(0)
	scaledUp := r.determineReplicas(rt)

	if !config.ScaleToZero.Enabled {
		return scaledUp, time.Duration(0), nil
	}

	// check if the runtime has predictors before locking the mutex
	hasPredictors, err := r.runtimeHasPredictors(ctx, rt)
	if err != nil {
		return 0, 0, err
	}

	// we'll need to inspect/update the runtime info as well
	// lock the mutex while we may be accessing the runtimeInfoMap
	r.runtimeInfoMapMutex.Lock()
	defer r.runtimeInfoMapMutex.Unlock()

	// initialize runtime information map if it is nil
	// eg. if this is the first reconcile for any runtime
	if r.runtimeInfoMap == nil {
		r.runtimeInfoMap = make(map[types.NamespacedName]*runtimeInfo)
	}

	runtimeInfoMapKey := client.ObjectKeyFromObject(rt)
	targetRuntimeInfo := r.runtimeInfoMap[runtimeInfoMapKey]

	// initialize this runtime's info if it is nil
	//  set the transition time to the zero value, then, if there are no
	//  predictors, the runtime will be scaled to zero
	if targetRuntimeInfo == nil {
		targetRuntimeInfo = &runtimeInfo{
			TimeTransitionedToNoPredictors: &time.Time{},
		}
		r.runtimeInfoMap[runtimeInfoMapKey] = targetRuntimeInfo
	}

	// if the runtime has predictors, it shouldn't be scaled down
	if hasPredictors {
		// update runtime info to have transition time set to nil
		targetRuntimeInfo.TimeTransitionedToNoPredictors = nil
		return scaledUp, time.Duration(0), nil
	}

	// if this is the first time we see no predictors update the runtime info with
	// this transition
	if targetRuntimeInfo.TimeTransitionedToNoPredictors == nil {
		log.Info("Runtime no longer has any predictors, will scale to zero after grace period",
			"gracePeriod", time.Duration(config.ScaleToZero.GracePeriodSeconds)*time.Second)
		t := time.Now()
		targetRuntimeInfo.TimeTransitionedToNoPredictors = &t
	}

	// check if we are in the grace period and will requeue a reconciliation to
	// trigger after the grace period has elapsed but won't scale to zero now
	gracePeriodDuration := time.Duration(config.ScaleToZero.GracePeriodSeconds) * time.Second
	durationSinceLastTransition := time.Since(*targetRuntimeInfo.TimeTransitionedToNoPredictors)
	if durationSinceLastTransition < gracePeriodDuration {
		requeueAfter := gracePeriodDuration - durationSinceLastTransition
		log.Info("Runtime has no predictors, will scale to zero after grace period",
			"gracePeriod", gracePeriodDuration, "timeRemaning", requeueAfter)
		return scaledUp, requeueAfter, nil
	}

	// finally, if we get here, the grace period has elapsed and we should scale
	// the deployment to zero
	log.Info("Scaling runtime to zero")
	return scaledToZero, time.Duration(0), nil
}

func (r *ServingRuntimeReconciler) determineReplicas(rt *api.ServingRuntime) uint16 {

	if rt.Spec.Replicas == nil {
		return r.ConfigProvider.GetConfig().PodsPerRuntime
	}

	return *rt.Spec.Replicas
}

// runtimeHasPredictors returns true if the runtime supports an existing Predictor
func (r *ServingRuntimeReconciler) runtimeHasPredictors(ctx context.Context, rt *api.ServingRuntime) (bool, error) {

	// see if any Predictor is supported by this runtime
	predictors := &api.PredictorList{}
	err := r.Client.List(ctx, predictors, client.InNamespace(r.DeploymentNamespace))
	if err != nil {
		return false, err
	}

	for _, p := range predictors.Items {
		if runtimeSupportsPredictor(rt, &p) {
			return true, nil
		}
	}

	if r.EnableTrainedModelWatch {
		trainedModels := &kfsv1alpha1.TrainedModelList{}
		err = r.Client.List(ctx, trainedModels, client.InNamespace(r.DeploymentNamespace))
		if err != nil {
			return false, err
		}

		for i := range trainedModels.Items {
			if runtimeSupportsPredictor(rt, kfsv1alpha1.BuildPredictorWithBase(&trainedModels.Items[i])) {
				return true, nil
			}
		}
	}

	return false, nil
}

func runtimeSupportsPredictor(rt *api.ServingRuntime, p *api.Predictor) bool {
	// assignment to a runtime depends on the model type labels
	runtimeLabelSet := modelmesh.GetServingRuntimeSupportedModelTypeLabelSet(rt)
	predictorLabel := modelmesh.GetPredictorModelTypeLabel(p)

	// if the runtime has the predictor's label, then it supports that predictor
	return runtimeLabelSet.Contains(predictorLabel)
}

// getRuntimesSupportingPredictor returns a list of keys for runtimes that support the predictor p
//
// A predictor may be supported by multiple runtimes.
func (r *ServingRuntimeReconciler) getRuntimesSupportingPredictor(ctx context.Context, p *api.Predictor) ([]types.NamespacedName, error) {
	var err error

	// list all runtimes
	runtimes := &api.ServingRuntimeList{}
	err = r.Client.List(ctx, runtimes, client.InNamespace(r.DeploymentNamespace))
	if err != nil {
		return nil, err
	}

	srnns := make([]types.NamespacedName, 0, len(runtimes.Items))
	for _, rt := range runtimes.Items {
		if runtimeSupportsPredictor(&rt, p) {
			srnn := types.NamespacedName{
				Name:      rt.GetName(),
				Namespace: r.DeploymentNamespace,
			}
			srnns = append(srnns, srnn)
		}
	}

	return srnns, nil
}

func (r *ServingRuntimeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		Named("ServingRuntimeReconciler").
		For(&api.ServingRuntime{}).
		Owns(&appsv1.Deployment{}).
		// watch the user configmap and reconcile all runtimes when it changes
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			ConfigWatchHandler(r.ConfigMapName, func() []reconcile.Request {
				list := &api.ServingRuntimeList{}
				requests := make([]reconcile.Request, 0, 4)
				if err2 := r.Client.List(context.TODO(), list); err2 == nil {
					for _, rt := range list.Items {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{Name: rt.Name, Namespace: rt.Namespace},
						})
					}
				} else {
					r.Log.Error(err2, "Error listing ServingRuntimes to reconcile")
				}
				return requests
			}, r.ConfigProvider, &r.Client)).
		// watch predictors and reconcile the corresponding runtime(s) it could be assigned to
		Watches(&source.Kind{Type: &api.Predictor{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return r.runtimeRequestsForPredictor(o.(*api.Predictor), "Predictor")
			}))

	if r.EnableTrainedModelWatch {
		builder = builder.Watches(&source.Kind{Type: &kfsv1alpha1.TrainedModel{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				p := kfsv1alpha1.BuildPredictorWithBase(o.(*kfsv1alpha1.TrainedModel))
				return r.runtimeRequestsForPredictor(p, "TrainedModel")
			}))
	}

	return builder.Complete(r)
}

func (r *ServingRuntimeReconciler) runtimeRequestsForPredictor(p *api.Predictor, source string) []reconcile.Request {
	srnns, err := r.getRuntimesSupportingPredictor(context.TODO(), p)
	if err != nil {
		r.Log.Error(err, "Error getting runtimes that support predictor", "name", p.GetName(), "source", source)
		return []reconcile.Request{}
	}
	if len(srnns) == 0 {
		r.Log.Info("No runtime found to support predictor", "name", p.GetName(), "source", source)
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0, len(srnns))
	for _, nn := range srnns {
		requests = append(requests, reconcile.Request{NamespacedName: nn})
	}
	return requests
}
