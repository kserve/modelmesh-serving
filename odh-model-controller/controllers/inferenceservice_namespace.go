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

	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	meshNamespaceLabel       = "modelmesh-enabled"
	meshNamespaceLabelValue  = "true"
	istioNamespaceLabel      = "istio-injection"
	istioNamespaceLabelValue = "enabled"
)

// NewInferenceServiceNamespace defines the desired Namespace object
func NewInferenceServiceNamespace(inferenceservice *inferenceservicev1.InferenceService) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: inferenceservice.Namespace, Namespace: inferenceservice.Namespace, Labels: map[string]string{"inferenceservice-name": inferenceservice.Name, "modelmesh-enabled": "true", istioNamespaceLabel: istioNamespaceLabelValue}},
	}
}

// CheckForNamespaceLabel Make sure our desired label/value is there
func CheckForNamespaceLabel(searchlabel string, searchValue string, ns *corev1.Namespace) bool {
	if value, found := ns.ObjectMeta.Labels[searchlabel]; found {
		if value == searchValue {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

// Reconcile will manage the creation, update and deletion of the Namespace returned
// by the newNamespace function
func (r *OpenshiftInferenceServiceReconciler) reconcileNamespace(inferenceservice *inferenceservicev1.InferenceService,
	ctx context.Context, newPredictorNamespace func(service *inferenceservicev1.InferenceService) *corev1.Namespace) error {
	// Initialize logger format
	log := r.Log.WithValues("InferenceService", inferenceservice.Name, "namespace", inferenceservice.Namespace)

	// Create the Namespace if it does not already exist
	foundNamespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      inferenceservice.Namespace,
		Namespace: inferenceservice.Namespace,
	}, foundNamespace)
	if err != nil {
		if apierrs.IsNotFound(err) {
			log.Error(err, "Namespace not found...this should NOT happen")
		} else {
			log.Error(err, "Unable to fetch the Namespace")
			return err
		}
	}

	// Reconcile the Namespace spec if it has been manually modified
	if !CheckForNamespaceLabel(meshNamespaceLabel, meshNamespaceLabelValue, foundNamespace) ||
		!CheckForNamespaceLabel(istioNamespaceLabel, istioNamespaceLabelValue, foundNamespace) {
		// Retry the update operation when the ingress controller eventually
		// updates the resource version field
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the last Namespace revision
			if err := r.Get(ctx, types.NamespacedName{
				Name:      inferenceservice.Namespace,
				Namespace: inferenceservice.Namespace,
			}, foundNamespace); err != nil {
				log.Error(err, "Unable to reconcile namespace")
				return err
			}
			// Reconcile labels and spec field
			labels := make(map[string]string)
			labels[meshNamespaceLabel] = meshNamespaceLabelValue
			labels[istioNamespaceLabel] = istioNamespaceLabelValue
			foundNamespace.ObjectMeta.Labels = labels
			return r.Update(ctx, foundNamespace)
		})
		if err != nil {
			log.Error(err, "Unable to reconcile the Namespace")
			return err
		}
	}
	return nil
}

// ReconcileNamespace will manage the creation, update and deletion of the
// Namespace when the Predictor is reconciled
func (r *OpenshiftInferenceServiceReconciler) ReconcileNamespace(
	inferenceservice *inferenceservicev1.InferenceService, ctx context.Context) error {
	return r.reconcileNamespace(inferenceservice, ctx, NewInferenceServiceNamespace)
}
