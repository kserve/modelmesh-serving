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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kserve/kserve/pkg/constants"
	mmcontstant "github.com/kserve/modelmesh-serving/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetHPAMetrics(t *testing.T) {
	servingRuntimeName := "my-model"
	namespace := "test"

	testCases := []struct {
		name                                string
		servingRuntimeMetaData              *metav1.ObjectMeta
		expectedTargetUtilizationPercentage int32
		expectedAutoscalerMetrics           corev1.ResourceName
	}{
		{
			name: "Check default HPAMetrics",
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},
			expectedTargetUtilizationPercentage: int32(80),
			expectedAutoscalerMetrics:           corev1.ResourceName("cpu"),
		},
		{
			name: "Check HPAMetrics if annotations has " + constants.AutoscalerMetrics,
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{constants.AutoscalerMetrics: "memory"},
			},
			expectedTargetUtilizationPercentage: int32(80),
			expectedAutoscalerMetrics:           corev1.ResourceName("memory"),
		},
		{
			name: "Check HPAMetrics if annotations has " + constants.TargetUtilizationPercentage,
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{constants.TargetUtilizationPercentage: "50"},
			},
			expectedTargetUtilizationPercentage: int32(50),
			expectedAutoscalerMetrics:           corev1.ResourceName("cpu"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := getHPAMetrics(*tt.servingRuntimeMetaData)
			if diff := cmp.Diff(tt.expectedTargetUtilizationPercentage, *result[0].Resource.Target.AverageUtilization); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", t.Name(), diff)
			}
			if diff := cmp.Diff(tt.expectedAutoscalerMetrics, result[0].Resource.Name); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", t.Name(), diff)
			}
		})
	}
}

func TestCreateHPA(t *testing.T) {
	servingRuntimeName := "my-model"
	namespace := "test"
	deploymentName := fmt.Sprintf("%s-%s", servingRuntimeName, namespace)

	testCases := []struct {
		name                   string
		servingRuntimeMetaData *metav1.ObjectMeta
		mmDeploymentName       *string
		mmNamespace            *string
		expectedMinReplicas    int32
		expectedMaxReplicas    int32
	}{
		{
			name: "Check default HPA replicas",
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{},
			},
			mmDeploymentName:    &deploymentName,
			mmNamespace:         &namespace,
			expectedMinReplicas: int32(1),
			expectedMaxReplicas: int32(1),
		},
		{
			name: "Check HPA replicas if annotations has " + mmcontstant.MaxScaleAnnotationKey,
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{mmcontstant.MaxScaleAnnotationKey: "2"},
			},
			mmDeploymentName:    &deploymentName,
			mmNamespace:         &namespace,
			expectedMinReplicas: int32(1),
			expectedMaxReplicas: int32(2),
		},
		{
			name: "Check HPA replicas if annotations has " + mmcontstant.MinScaleAnnotationKey + ". max replicas should be the same as min replicas",
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{mmcontstant.MinScaleAnnotationKey: "2"},
			},
			mmDeploymentName:    &deploymentName,
			mmNamespace:         &namespace,
			expectedMinReplicas: int32(2),
			expectedMaxReplicas: int32(2),
		},
		{
			name: "Check HPA replicas if annotations set min/max replicas both",
			servingRuntimeMetaData: &metav1.ObjectMeta{
				Name:        servingRuntimeName,
				Namespace:   namespace,
				Annotations: map[string]string{mmcontstant.MinScaleAnnotationKey: "2", mmcontstant.MaxScaleAnnotationKey: "3"},
			},
			mmDeploymentName:    &deploymentName,
			mmNamespace:         &namespace,
			expectedMinReplicas: int32(2),
			expectedMaxReplicas: int32(3),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			hpa := createHPA(*tt.servingRuntimeMetaData, *tt.mmDeploymentName, *tt.mmNamespace)
			if diff := cmp.Diff(tt.expectedMinReplicas, *hpa.Spec.MinReplicas); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", t.Name(), diff)
			}
			if diff := cmp.Diff(tt.expectedMaxReplicas, hpa.Spec.MaxReplicas); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", t.Name(), diff)
			}
		})
	}
}
