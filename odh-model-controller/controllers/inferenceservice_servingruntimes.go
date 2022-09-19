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
	"io/ioutil"
	"os"
	"strings"

	predictorv1 "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultNamespace  = "odh-model-controller-system"
	runtimesConfigMap = "servingruntimes-config"
	runtimesConfigKey = "servingruntimes_config.yaml"
)

// GetOperatorNamespace returns the namespace the operator should be running in.
func GetOperatorNamespace() (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return defaultNamespace, nil
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

// NewInferenceServiceServingRuntimes defines the desired ServingRuntimes object
func NewInferenceServiceServingRuntimes(inferenceservice *inferenceservicev1.InferenceService, ctx context.Context, r *OpenshiftInferenceServiceReconciler) *predictorv1.ServingRuntimeList {
	log := r.Log.WithValues("inferenceservice", inferenceservice.Name, "namespace", inferenceservice.Namespace)
	srList := &predictorv1.ServingRuntimeList{}
	operatorns, err := GetOperatorNamespace()
	if err != nil {
		log.Error(err, "Could not determine operator namespace")
	}
	// Read the configmap to see the list of servingruntimes we need
	configmap := &corev1.ConfigMap{}
	r.Get(ctx, types.NamespacedName{
		Name:      runtimesConfigMap,
		Namespace: operatorns,
	}, configmap)
	runtimes := configmap.Data[runtimesConfigKey]

	decode := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer().Decode
	obj, _, _ := decode([]byte(runtimes), nil, nil)
	if obj != nil {
		cm := obj.(*corev1.ConfigMap)
		for key := range cm.Data {
			sr := cm.Data[key]
			obj, _, _ := decode([]byte(sr), nil, nil)
			srobject := obj.(*predictorv1.ServingRuntime)
			srobject.ObjectMeta.Namespace = inferenceservice.Namespace
			srList.Items = append(srList.Items, *srobject)
		}
	}

	return srList
}

// CompareInferenceServiceServingRuntimes checks if two ServingRuntimess are equal, if not return false
func CompareInferenceServiceServingRuntimes(srl1 *predictorv1.ServingRuntimeList, srl2 *predictorv1.ServingRuntimeList) bool {
	// Two ServingRuntimess will be equal if they have the same names
	// listonekeys := srl1.Items

	return false //TODO do it for real
}

// Reconcile will manage the creation, update and deletion of the ServingRuntimes returned
// by the newServingRuntimes function
func (r *OpenshiftInferenceServiceReconciler) reconcileServingRuntimes(inferenceservice *inferenceservicev1.InferenceService,
	ctx context.Context, newServingRuntimes func(*inferenceservicev1.InferenceService, context.Context, *OpenshiftInferenceServiceReconciler) *predictorv1.ServingRuntimeList) error {
	// Initialize logger format
	log := r.Log.WithValues("InferenceService", inferenceservice.Name, "namespace", inferenceservice.Namespace)

	// Generate the desired ServingRuntimes
	desiredServingRuntimes := newServingRuntimes(inferenceservice, ctx, r)

	// Create the ServingRuntimes if it does not already exist
	foundServingRuntimes := &predictorv1.ServingRuntimeList{}
	justCreated := false
	listOptions := client.ListOptions{
		Namespace: inferenceservice.Namespace,
	}
	err := r.List(ctx, foundServingRuntimes, &listOptions)
	if err != nil {
		if apierrs.IsNotFound(err) {
			// Normally, we would set an ownerreference here, but we don't want
			// to delete the servingruntimes when a predictor is deleted since
			// there may be several predictors using the same servingruntime
			// Create the ServingRuntimes in the Openshift cluster
			for key := range desiredServingRuntimes.Items {
				sr := desiredServingRuntimes.Items[key]
				r.Create(ctx, &sr)
			}
			justCreated = true
		} else {
			log.Error(err, "Unable to fetch the ServingRuntimes")
			return err
		}
	}

	// Reconcile the ServingRuntimes
	if !justCreated && !CompareInferenceServiceServingRuntimes(desiredServingRuntimes, foundServingRuntimes) {
		log.Info("Reconciling ServingRuntimes")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			listOptions := client.ListOptions{
				Namespace: inferenceservice.Namespace,
			}
			if err := r.List(ctx, foundServingRuntimes, &listOptions); err != nil {
				return err
			}
			// Reconcile ServingRuntimes by adding them as needed, updating the list isn't possible
			for key := range desiredServingRuntimes.Items {
				sr := desiredServingRuntimes.Items[key]
				r.Create(ctx, &sr)
			}
			return nil
		})
		if err != nil {
			log.Error(err, "Unable to reconcile the ServingRuntimes")
			return err
		}
	}

	return nil
}

// ReconcileServingRuntimes will manage the creation, update and deletion of the
// ServingRuntimes when the Predictor is reconciled
func (r *OpenshiftInferenceServiceReconciler) ReconcileServingRuntimes(
	inferenceservice *inferenceservicev1.InferenceService, ctx context.Context) error {
	return r.reconcileServingRuntimes(inferenceservice, ctx, NewInferenceServiceServingRuntimes)
}
