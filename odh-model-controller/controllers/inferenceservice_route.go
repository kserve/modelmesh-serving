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
	predictorv1 "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	modelmeshServiceName     = "modelmesh-serving"
	modelmeshAuthServicePort = 8443
	modelmeshServicePort     = 8008
)

// NewInferenceServiceRoute defines the desired route object
func NewInferenceServiceRoute(inferenceservice *inferenceservicev1.InferenceService, enableAuth bool) *routev1.Route {

	routePortName := modelmeshServicePort
	if enableAuth {
		routePortName = modelmeshAuthServicePort
	}

	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inferenceservice.Name,
			Namespace: inferenceservice.Namespace,
			Labels: map[string]string{
				"inferenceservice-name": inferenceservice.Name,
			},
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   modelmeshServiceName,
				Weight: pointer.Int32Ptr(100),
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(routePortName),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
			Path:           "/v2/models/" + inferenceservice.Name,
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{},
		},
	}
}

// CompareInferenceServiceRoutes checks if two routes are equal, if not return false
func CompareInferenceServiceRoutes(r1 routev1.Route, r2 routev1.Route) bool {
	// Omit the host field since it is reconciled by the ingress controller
	r1.Spec.Host, r2.Spec.Host = "", ""

	// Two routes will be equal if the labels and spec are identical
	return reflect.DeepEqual(r1.ObjectMeta.Labels, r2.ObjectMeta.Labels) &&
		reflect.DeepEqual(r1.Spec, r2.Spec)
}

// Reconcile will manage the creation, update and deletion of the route returned
// by the newRoute function
func (r *OpenshiftInferenceServiceReconciler) reconcileRoute(inferenceservice *inferenceservicev1.InferenceService,
	ctx context.Context, newRoute func(service *inferenceservicev1.InferenceService, enableAuth bool) *routev1.Route) error {
	// Initialize logger format
	log := r.Log.WithValues("inferenceservice", inferenceservice.Name, "namespace", inferenceservice.Namespace)

	enableAuth := true
	desiredServingRuntime := &predictorv1.ServingRuntime{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      *inferenceservice.Spec.Predictor.Model.Runtime,
		Namespace: inferenceservice.Namespace,
	}, desiredServingRuntime)
	if err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Serving Runtime ", *inferenceservice.Spec.Predictor.Model.Runtime, " desired by ", inferenceservice.Name, "was not found in namespace")
		}
	}

	if desiredServingRuntime.Annotations["enable-auth"] != "True" {
		enableAuth = false
	}
	createRoute := true
	if desiredServingRuntime.Annotations["create-route"] != "True" {
		createRoute = false
	}

	// Generate the desired route
	desiredRoute := newRoute(inferenceservice, enableAuth)

	// Create the route if it does not already exist
	foundRoute := &routev1.Route{}
	justCreated := false
	err = r.Get(ctx, types.NamespacedName{
		Name:      desiredRoute.Name,
		Namespace: inferenceservice.Namespace,
	}, foundRoute)
	if err != nil {
		if !createRoute {
			log.Info("Serving runtime does not have 'create-route' annotation set to 'True'. Skipping route creation")
			return nil
		}
		if apierrs.IsNotFound(err) {
			log.Info("Creating Route")
			// Add .metatada.ownerReferences to the route to be deleted by the
			// Kubernetes garbage collector if the predictor is deleted
			err = ctrl.SetControllerReference(inferenceservice, desiredRoute, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to add OwnerReference to the Route")
				return err
			}
			// Create the route in the Openshift cluster
			err = r.Create(ctx, desiredRoute)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				log.Error(err, "Unable to create the Route")
				return err
			}
			justCreated = true
		} else {
			log.Error(err, "Unable to fetch the Route")
			return err
		}
	}

	if !createRoute {
		log.Info("Serving Runtime does not have 'create-route' annotation set to 'True'. Deleting existing route")
		return r.Delete(ctx, foundRoute)
	}
	// Reconcile the route spec if it has been manually modified
	if !justCreated && !CompareInferenceServiceRoutes(*desiredRoute, *foundRoute) {
		log.Info("Reconciling Route")
		// Retry the update operation when the ingress controller eventually
		// updates the resource version field
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the last route revision
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desiredRoute.Name,
				Namespace: inferenceservice.Namespace,
			}, foundRoute); err != nil {
				return err
			}
			// Reconcile labels and spec field
			foundRoute.Spec = desiredRoute.Spec
			foundRoute.ObjectMeta.Labels = desiredRoute.ObjectMeta.Labels
			return r.Update(ctx, foundRoute)
		})
		if err != nil {
			log.Error(err, "Unable to reconcile the Route")
			return err
		}
	}

	return nil
}

// ReconcileRoute will manage the creation, update and deletion of the
// TLS route when the predictor is reconciled
func (r *OpenshiftInferenceServiceReconciler) ReconcileRoute(
	inferenceservice *inferenceservicev1.InferenceService, ctx context.Context) error {
	return r.reconcileRoute(inferenceservice, ctx, NewInferenceServiceRoute)
}
