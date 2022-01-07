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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kserve/modelmesh-serving/apis/serving/common"
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

	common.PredictorStatus `json:",inline"`

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

type ModelFormat struct {
	// Name of the model format.
	// +required
	Name string `json:"name"`
	// Version of the model format.
	// +optional
	Version *string `json:"version,omitempty"`
}

type ModelSpec struct {
	// ModelFormat being served.
	// +required
	ModelFormat ModelFormat `json:"modelFormat"`

	// Specific ClusterServingRuntime/ServingRuntime name to use for deployment.
	// +optional
	Runtime *string `json:"runtime,omitempty"`

	PredictorExtensionSpec `json:",inline"`
}

type InferenceServicePredictorSpec struct {
	Model *ModelSpec `json:"model,omitempty"`

	// Note: the below fields are still supported, but deprecated.

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
	// Storage Spec for model location
	// +optional
	Storage *common.StorageSpec `json:"storage,omitempty"`
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
