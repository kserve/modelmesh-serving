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
	"reflect"

	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
	"istio.io/api/meta/v1alpha1"
	"istio.io/api/networking/v1alpha3"
	virtualservicev1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewInferenceServiceVirtualService defines the desired VirtualService object
func NewInferenceServiceVirtualService(inferenceservice *inferenceservicev1.InferenceService) *virtualservicev1.VirtualService {
	return &virtualservicev1.VirtualService{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: inferenceservice.Name, Namespace: inferenceservice.Namespace, Labels: map[string]string{"inferenceservice-name": inferenceservice.Name}},
		Spec: v1alpha3.VirtualService{
			Gateways: []string{"opendatahub/odh-gateway"}, //TODO get actual gateway to be used
			Hosts:    []string{"*"},
			Http: []*v1alpha3.HTTPRoute{{
				Match: []*v1alpha3.HTTPMatchRequest{{
					Uri: &v1alpha3.StringMatch{
						MatchType: &v1alpha3.StringMatch_Prefix{
							Prefix: "/modelmesh/" + inferenceservice.Namespace + "/",
						},
					},
				}},
				Rewrite: &v1alpha3.HTTPRewrite{
					Uri: "/",
				},
				Route: []*v1alpha3.HTTPRouteDestination{{
					Destination: &v1alpha3.Destination{
						Host: "modelmesh-serving." + inferenceservice.Namespace + ".svc.cluster.local",
						Port: &v1alpha3.PortSelector{
							Number: 8008,
						},
					},
				}},
			}},
		},
		Status: v1alpha1.IstioStatus{},
	}
}

// CompareInferenceServiceVirtualServices checks if two VirtualServices are equal, if not return false
func CompareInferenceServiceVirtualServices(vs1 *virtualservicev1.VirtualService, vs2 *virtualservicev1.VirtualService) bool {
	// Two VirtualServices will be equal if the labels and spec are identical
	return reflect.DeepEqual(vs1.ObjectMeta.Labels, vs2.ObjectMeta.Labels) &&
		reflect.DeepEqual(vs1.Spec.Hosts, vs2.Spec.Hosts)
}

// Reconcile will manage the creation, update and deletion of the VirtualService returned
// by the newVirtualService function
func (r *OpenshiftInferenceServiceReconciler) reconcileVirtualService(inferenceservice *inferenceservicev1.InferenceService,
	ctx context.Context, newVirtualService func(service *inferenceservicev1.InferenceService) *virtualservicev1.VirtualService) error {
	// Initialize logger format
	log := r.Log.WithValues("inferenceservice", inferenceservice.Name, "namespace", inferenceservice.Namespace)

	// Generate the desired VirtualService
	desiredVirtualService := newVirtualService(inferenceservice)

	// Create the VirtualService if it does not already exist
	foundVirtualService := &virtualservicev1.VirtualService{}
	justCreated := false
	err := r.Get(ctx, types.NamespacedName{
		Name:      desiredVirtualService.Name,
		Namespace: inferenceservice.Namespace,
	}, foundVirtualService)
	if err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating VirtualService")
			// Add .metatada.ownerReferences to the VirtualService to be deleted by the
			// Kubernetes garbage collector if the Predictor is deleted
			err = ctrl.SetControllerReference(inferenceservice, desiredVirtualService, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to add OwnerReference to the VirtualService")
				return err
			}
			// Create the VirtualService in the Openshift cluster
			err = r.Create(ctx, desiredVirtualService)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				log.Error(err, "Unable to create the VirtualService")
				return err
			}
			justCreated = true
		} else {
			log.Error(err, "Unable to fetch the VirtualService")
			return err
		}
	}

	// Reconcile the VirtualService spec if it has been manually modified
	if !justCreated && !CompareInferenceServiceVirtualServices(desiredVirtualService, foundVirtualService) {
		log.Info("Reconciling VirtualService")
		// Retry the update operation when the ingress controller eventually
		// updates the resource version field
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the last VirtualService revision
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desiredVirtualService.Name,
				Namespace: inferenceservice.Namespace,
			}, foundVirtualService); err != nil {
				return err
			}
			// Reconcile labels and spec field
			foundVirtualService.Spec = *desiredVirtualService.Spec.DeepCopy()
			foundVirtualService.ObjectMeta.Labels = desiredVirtualService.ObjectMeta.Labels
			return r.Update(ctx, foundVirtualService)
		})
		if err != nil {
			log.Error(err, "Unable to reconcile the VirtualService")
			return err
		}
	}

	return nil
}

// ReconcileVirtualService will manage the creation, update and deletion of the
// VirtualService when the Predictor is reconciled
func (r *OpenshiftInferenceServiceReconciler) ReconcileVirtualService(
	inferenceservice *inferenceservicev1.InferenceService, ctx context.Context) error {
	return r.reconcileVirtualService(inferenceservice, ctx, NewInferenceServiceVirtualService)
}
