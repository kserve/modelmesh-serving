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

package autoscaler

import (
	"fmt"

	"github.com/pkg/errors"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/constants"
	"github.com/kserve/modelmesh-serving/controllers/hpa"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	AutoscalerClassNone = "none"
)

type Autoscaler struct {
	AutoscalerClass constants.AutoscalerClassType
	HPA             *hpa.HPAReconciler
}

// AutoscalerReconciler is the struct of Raw K8S Object
type AutoscalerReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	Autoscaler *Autoscaler
}

func NewAutoscalerReconciler(client client.Client,
	scheme *runtime.Scheme,
	servingRuntime interface{}, mmDeploymentName string, mmNamespace string) (*AutoscalerReconciler, error) {

	as, err := createAutoscaler(client, scheme, servingRuntime, mmDeploymentName, mmNamespace)
	if err != nil {
		return nil, err
	}
	return &AutoscalerReconciler{
		client:     client,
		scheme:     scheme,
		Autoscaler: as,
	}, err
}

func getAutoscalerClass(metadata metav1.ObjectMeta) constants.AutoscalerClassType {
	annotations := metadata.Annotations
	if value, ok := annotations[constants.AutoscalerClass]; ok {
		return constants.AutoscalerClassType(value)
	} else {
		return AutoscalerClassNone
	}
}

func createAutoscaler(client client.Client,
	scheme *runtime.Scheme, servingRuntime interface{}, mmDeploymentName string, mmNamespace string) (*Autoscaler, error) {
	var runtimeMeta metav1.ObjectMeta
	isSR := false

	sr, ok := servingRuntime.(*kserveapi.ServingRuntime)
	if ok {
		runtimeMeta = sr.ObjectMeta
		isSR = true
	}
	csr, ok := servingRuntime.(*kserveapi.ClusterServingRuntime)
	if ok {
		runtimeMeta = csr.ObjectMeta
	}

	as := &Autoscaler{}
	ac := getAutoscalerClass(runtimeMeta)
	as.AutoscalerClass = ac

	switch ac {
	case constants.AutoscalerClassHPA:
		as.HPA = hpa.NewHPAReconciler(client, scheme, runtimeMeta, mmDeploymentName, mmNamespace)
		if isSR {
			if err := controllerutil.SetControllerReference(sr, as.HPA.HPA, scheme); err != nil {
				return nil, fmt.Errorf("fails to set HPA owner reference for ServingRuntime: %w", err)
			}
		} else {
			if err := controllerutil.SetControllerReference(csr, as.HPA.HPA, scheme); err != nil {
				return nil, fmt.Errorf("fails to set HPA owner reference for ClusterServingRuntime: %w", err)
			}
		}
	case AutoscalerClassNone:
		// Set HPA reconciler even though AutoscalerClass is None to delete existing hpa
		as.HPA = hpa.NewHPAReconciler(client, scheme, runtimeMeta, mmDeploymentName, mmNamespace)
		return as, nil
	case constants.AutoscalerClassExternal:
		// Set HPA reconciler even though AutoscalerClass is External to delete existing hpa
		as.HPA = hpa.NewHPAReconciler(client, scheme, runtimeMeta, mmDeploymentName, mmNamespace)
		return as, nil
	default:
		return nil, errors.New("unknown autoscaler class type.")
	}
	return as, nil
}

// Reconcile ...
func (r *AutoscalerReconciler) Reconcile(scaleToZero bool) (*Autoscaler, error) {
	//reconcile Autoscaler
	//In the case of a new autoscaler plugin, it checks AutoscalerClassType
	if r.Autoscaler.AutoscalerClass == constants.AutoscalerClassHPA || r.Autoscaler.AutoscalerClass == AutoscalerClassNone {
		_, err := r.Autoscaler.HPA.Reconcile(scaleToZero)
		if err != nil {
			return nil, err
		}
	}

	if scaleToZero {
		r.Autoscaler.HPA.HPA = nil
	}

	return r.Autoscaler, nil
}
