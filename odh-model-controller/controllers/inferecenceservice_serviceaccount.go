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
	authv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"

	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	modelMeshServiceAccountName = "modelmesh-serving-sa"
)

func newInferenceServiceSA(inferenceservice *inferenceservicev1.InferenceService) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelMeshServiceAccountName,
			Namespace: inferenceservice.Namespace,
		},
	}
}

func createDelegateClusterRoleBinding(serviceAccountName string, serviceAccountNamespace string) *authv1.ClusterRoleBinding {
	return &authv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName + "-auth-delegator",
			Namespace: serviceAccountNamespace,
		},
		Subjects: []authv1.Subject{
			authv1.Subject{
				Kind:      "ServiceAccount",
				Namespace: serviceAccountNamespace,
				Name:      serviceAccountName,
			},
		},
		RoleRef: authv1.RoleRef{
			Kind: "ClusterRole",
			Name: "system:auth-delegator",
		},
	}
}

func (r *OpenshiftInferenceServiceReconciler) reconcileSA(inferenceService *inferenceservicev1.InferenceService, ctx context.Context, newSA func(service *inferenceservicev1.InferenceService) *corev1.ServiceAccount) error {

	// Initialize logger format
	log := r.Log.WithValues("inferenceservice", inferenceService.Name, "namespace", inferenceService.Namespace)

	desiredSA := newSA(inferenceService)
	foundSA := &corev1.ServiceAccount{}

	err := r.Get(ctx, types.NamespacedName{
		Name:      desiredSA.Name,
		Namespace: inferenceService.Namespace,
	}, foundSA)

	if err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating Auth Delegation Service Account")
			// Add .metatada.ownerReferences to the service account to be deleted by the
			// Kubernetes garbage collector if the predictor is deleted
			err = ctrl.SetControllerReference(inferenceService, desiredSA, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to add OwnerReference to the Auth Delegation Service Account")
				return err
			}
			// Create the SA in the Openshift cluster
			err = r.Create(ctx, desiredSA)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				log.Error(err, "Unable to create the Auth Delegation Service Account")
				return err
			}
		} else {
			log.Error(err, "Unable to fetch the Auth Delegation Service Account")
			return err
		}
	}

	// Create the corresponding auth delegation cluster role binding
	desiredCRB := createDelegateClusterRoleBinding(modelMeshServiceAccountName, desiredSA.Namespace)
	foundCRB := &authv1.ClusterRoleBinding{}
	justCreated := false

	err = r.Get(ctx, types.NamespacedName{
		Name:      desiredCRB.Name,
		Namespace: desiredCRB.Namespace,
	}, foundCRB)

	if err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating Auth Delegation Cluster Role Binding")
			// Add .metatada.ownerReferences to the CRB to be deleted by the
			// Kubernetes garbage collector if the predictor is deleted
			err = ctrl.SetControllerReference(inferenceService, desiredCRB, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to add OwnerReference to the Auth Delegation Cluster Role Binding")
				return err
			}
			// Create the CRB in the Openshift cluster
			err = r.Create(ctx, desiredCRB)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				log.Error(err, "Unable to create the Auth Delegation Cluster Role Binding")
				return err
			}
			justCreated = true
		} else {
			log.Error(err, "Unable to fetch the Auth Delegation Cluster Role Binding")
			return err
		}
	}

	// Reconcile the CRB spec if it has been manually modified
	if !justCreated && !CompareInferenceServiceCRBs(*desiredCRB, *foundCRB) {
		log.Info("Reconciling Auth Delegation Cluster Role Binding")
		// Retry the update operation when the ingress controller eventually
		// updates the resource version field
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Get the last CRB revision
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desiredCRB.Name,
				Namespace: desiredCRB.Namespace,
			}, foundCRB); err != nil {
				return err
			}
			// Reconcile labels and spec field
			foundCRB.Subjects = desiredCRB.Subjects
			foundCRB.RoleRef = desiredCRB.RoleRef
			return r.Update(ctx, desiredCRB)
		})
		if err != nil {
			log.Error(err, "Unable to reconcile the Auth Delegation Cluster Role Binding")
			return err
		}
	}
	return nil
}

// ReconcileSA will manage the creation, update and deletion of the auth delegation SA + RBAC
func (r *OpenshiftInferenceServiceReconciler) ReconcileSA(
	inferenceservice *inferenceservicev1.InferenceService, ctx context.Context) error {
	return r.reconcileSA(inferenceservice, ctx, newInferenceServiceSA)
}

// CompareInferenceServiceCRBs checks if two service accounts are equal, if not return false
func CompareInferenceServiceCRBs(crb1 authv1.ClusterRoleBinding, crb2 authv1.ClusterRoleBinding) bool {
	// Two CRBs will be equal if the role reference and subjects are equal
	return reflect.DeepEqual(crb1.RoleRef, crb2.RoleRef) &&
		reflect.DeepEqual(crb1.Subjects, crb2.Subjects)
}
