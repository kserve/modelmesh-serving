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

package hpa

import (
	"context"
	"strconv"

	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"github.com/kserve/kserve/pkg/constants"
	"github.com/kserve/kserve/pkg/utils"
	mmcontstant "github.com/kserve/modelmesh-serving/pkg/constants"
	v2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("HPAReconciler")

// HPAReconciler is the struct of Raw K8S Object
type HPAReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	HPA    *v2beta2.HorizontalPodAutoscaler
}

func NewHPAReconciler(client client.Client,
	scheme *runtime.Scheme, runtimeMeta metav1.ObjectMeta, mmDeploymentName string, mmNamespace string) *HPAReconciler {
	return &HPAReconciler{
		client: client,
		scheme: scheme,
		HPA:    createHPA(runtimeMeta, mmDeploymentName, mmNamespace),
	}
}

func getHPAMetrics(metadata metav1.ObjectMeta) []v2beta2.MetricSpec {
	var metrics []v2beta2.MetricSpec
	var utilization int32 = constants.DefaultCPUUtilization

	annotations := metadata.Annotations
	resourceName := corev1.ResourceCPU

	if value, ok := annotations[constants.TargetUtilizationPercentage]; ok {
		utilizationInt, _ := strconv.Atoi(value)
		utilization = int32(utilizationInt)
	}

	if value, ok := annotations[constants.AutoscalerMetrics]; ok {
		resourceName = corev1.ResourceName(value)
	}

	metricTarget := v2beta2.MetricTarget{
		Type:               "Utilization",
		AverageUtilization: &utilization,
	}

	ms := v2beta2.MetricSpec{
		Type: v2beta2.ResourceMetricSourceType,
		Resource: &v2beta2.ResourceMetricSource{
			Name:   resourceName,
			Target: metricTarget,
		},
	}

	metrics = append(metrics, ms)
	return metrics
}

func createHPA(runtimeMeta metav1.ObjectMeta, mmDeploymentName string, mmNamespace string) *v2beta2.HorizontalPodAutoscaler {
	minReplicas := int32(constants.DefaultMinReplicas)
	maxReplicas := int32(constants.DefaultMinReplicas)
	annotations := runtimeMeta.Annotations

	if value, ok := annotations[mmcontstant.MinScaleAnnotationKey]; ok {
		minReplicasInt, _ := strconv.Atoi(value)
		minReplicas = int32(minReplicasInt)

	}
	if value, ok := annotations[mmcontstant.MaxScaleAnnotationKey]; ok {
		maxReplicasInt, _ := strconv.Atoi(value)
		maxReplicas = int32(maxReplicasInt)
	}

	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}

	metrics := getHPAMetrics(runtimeMeta)

	hpaObjectMeta := metav1.ObjectMeta{
		Name:      mmDeploymentName,
		Namespace: mmNamespace,
		Labels: utils.Union(runtimeMeta.Labels, map[string]string{
			constants.InferenceServicePodLabelKey: runtimeMeta.Name,
			constants.KServiceComponentLabel:      string(v1beta1.PredictorComponent),
		}),
		Annotations: runtimeMeta.Annotations,
	}

	hpa := &v2beta2.HorizontalPodAutoscaler{
		ObjectMeta: hpaObjectMeta,
		Spec: v2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: v2beta2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       hpaObjectMeta.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,

			Metrics:  metrics,
			Behavior: &v2beta2.HorizontalPodAutoscalerBehavior{},
		},
	}
	return hpa
}

// checkHPAExist checks if the hpa exists?
func (r *HPAReconciler) checkHPAExist(client client.Client) (constants.CheckResultType, *v2beta2.HorizontalPodAutoscaler, error) {
	existingHPA := &v2beta2.HorizontalPodAutoscaler{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.HPA.ObjectMeta.Namespace,
		Name:      r.HPA.ObjectMeta.Name,
	}, existingHPA)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	//existed, check equivalent
	if semanticHPAEquals(r.HPA, existingHPA) {
		return constants.CheckResultExisted, existingHPA, nil
	}
	return constants.CheckResultUpdate, existingHPA, nil
}

func semanticHPAEquals(desired, existing *v2beta2.HorizontalPodAutoscaler) bool {
	return equality.Semantic.DeepEqual(desired.Spec.Metrics, existing.Spec.Metrics) &&
		equality.Semantic.DeepEqual(desired.Spec.MaxReplicas, existing.Spec.MaxReplicas) &&
		equality.Semantic.DeepEqual(*desired.Spec.MinReplicas, *existing.Spec.MinReplicas)
}

// Reconcile ...
func (r *HPAReconciler) Reconcile(scaleToZero bool) (*v2beta2.HorizontalPodAutoscaler, error) {
	//reconcile
	checkResult, existingHPA, err := r.checkHPAExist(r.client)
	log.Info("service reconcile", "checkResult", checkResult, "scaleToZero", scaleToZero, "err", err)
	if err != nil {
		return nil, err
	}

	if checkResult == constants.CheckResultCreate && !scaleToZero {
		if err = r.client.Create(context.TODO(), r.HPA); err != nil {
			return nil, err
		}
		return r.HPA, nil

	} else if checkResult == constants.CheckResultUpdate { //CheckResultUpdate
		if err = r.client.Update(context.TODO(), r.HPA); err != nil {
			return nil, err
		}
		return r.HPA, nil

	} else if checkResult == constants.CheckResultExisted && scaleToZero {
		// when scaleToZero is true, delete HPA if it exist
		if err = r.client.Delete(context.TODO(), existingHPA, &client.DeleteOptions{}); err != nil {
			return nil, err
		}
		return nil, nil
	} else {
		return existingHPA, nil
	}
}
