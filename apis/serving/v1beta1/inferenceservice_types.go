/*
Copyright 2021 IBM Corporation

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

//+kubebuilder:skip
package v1beta1

import (
	"errors"

	"github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SecretKeyAnnotation      = "serving.kserve.io/secretKey"
	DeploymentModeAnnotation = "serving.kserve.io/deploymentMode"
	SchemaPathAnnotation     = "serving.kserve.io/schemaPath"
	RuntimeAnnotation        = "serving.kserve.io/servingRuntime"
	MMDeploymentModeVal      = "ModelMesh"
)

// InferenceServiceSpec defines the desired state of InferenceService
type InferenceServiceSpec struct {
	// Predictor defines the model serving spec
	// +required
	Predictor InferenceServicePredictorSpec `json:"predictor"`
}

// InferenceServiceStatus defines the observed state of InferenceService
type InferenceServiceStatus struct {
	URL string `json:"url,omitempty"`

	v1alpha1.PredictorStatus `json:",inline"`

	// Conditions the latest available observations of a resource's current state.
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type Conditions []Condition

// This is used for reflecting the Ready status of the resource
// according to the InferenceService CRD's expected key which is:
// status.conditions[?(@.type=='Ready')].status
type Condition struct {
	Type string `json:"type"`

	Status corev1.ConditionStatus `json:"status"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// InferenceService is the Schema for the inferenceservices API
type InferenceService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceServiceSpec   `json:"spec,omitempty"`
	Status InferenceServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InferenceServiceList contains a list of InferenceService
type InferenceServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceService `json:"items"`
}

type InferenceServicePredictorSpec struct {
	SKLearn    *PredictorExtensionSpec `json:"sklearn,omitempty"`
	XGBoost    *PredictorExtensionSpec `json:"xgboost,omitempty"`
	Tensorflow *PredictorExtensionSpec `json:"tensorflow,omitempty"`
	PyTorch    *PredictorExtensionSpec `json:"pytorch,omitempty"`
	Triton     *PredictorExtensionSpec `json:"triton,omitempty"`
	ONNX       *PredictorExtensionSpec `json:"onnx,omitempty"`
	PMML       *PredictorExtensionSpec `json:"pmml,omitempty"`
	LightGBM   *PredictorExtensionSpec `json:"lightgbm,omitempty"`
	Paddle     *PredictorExtensionSpec `json:"paddle,omitempty"`
}

type PredictorExtensionSpec struct {
	// +optional
	StorageURI *string `json:"storageUri,omitempty"`
	// +optional
	RuntimeVersion *string `json:"runtimeVersion,omitempty"`
}

func (isvc *InferenceService) BuildPredictorWithBase() (*v1alpha1.Predictor, error) {
	p := &v1alpha1.Predictor{}

	// Check if resource should be reconciled.
	if isvc.ObjectMeta.Annotations[DeploymentModeAnnotation] != MMDeploymentModeVal {
		return nil, nil
	}

	framework, frameworkSpec := isvc.Spec.Predictor.GetPredictorFramework()
	if frameworkSpec == nil {
		return nil, errors.New("No valid InferenceService predictor framework found")
	}

	p.Spec = v1alpha1.PredictorSpec{
		Model: v1alpha1.Model{
			Type: v1alpha1.ModelType{
				Name: framework,
			},
		},
	}

	// If explicit ServingRuntime was passed in through an annotation
	if runtime, ok := isvc.ObjectMeta.Annotations[RuntimeAnnotation]; ok {
		p.Spec.Runtime = &v1alpha1.PredictorRuntime{
			RuntimeRef: &v1alpha1.RuntimeRef{
				Name: runtime,
			},
		}
	}
	return p, nil
}

func (s *InferenceServicePredictorSpec) GetPredictorFramework() (string, *PredictorExtensionSpec) {
	if s.XGBoost != nil {
		return "xgboost", s.XGBoost
	} else if s.LightGBM != nil {
		return "lightgbm", s.LightGBM
	} else if s.SKLearn != nil {
		return "sklearn", s.SKLearn
	} else if s.Tensorflow != nil {
		return "tensorflow", s.Tensorflow
	} else if s.ONNX != nil {
		return "onnx", s.ONNX
	} else if s.PyTorch != nil {
		return "pytorch", s.PyTorch
	} else if s.Triton != nil {
		return "triton", s.Triton
	} else if s.PMML != nil {
		return "pmml", s.PMML
	} else {
		return "", nil
	}
}

func init() {
	SchemeBuilder.Register(&InferenceService{}, &InferenceServiceList{})
}
