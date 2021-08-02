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
package v1alpha1

import (
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrainedModelSpec defines the desired state of TrainedModel
type TrainedModelSpec struct {
	// parent inference service to deploy to
	// +required
	InferenceService string `json:"inferenceService"`
	// Predictor model spec
	// +required
	Model ModelSpec `json:"model"`
}

type Conditions []Condition

// This is used for reflecting the Ready status of the resource
// according to the TrainedModel CRD's expected key which is:
// status.conditions[?(@.type=='Ready')].status
type Condition struct {
	Type string `json:"type"`

	Status corev1.ConditionStatus `json:"status"`
}

const (
	SecretKeyAnnotation = "serving.kserve.io/secret-key"
)

// TrainedModelStatus defines the observed state of TrainedModel
type TrainedModelStatus struct {
	// predictorapi.PredictorStatus `json:",inline"`

	// +optional
	URL string `json:"url,omitempty"`

	api.PredictorStatus `json:",inline"`

	// Conditions the latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=trainedmodels,shortName=tm,singular=trainedmodel
// TrainedModel is the Schema for the trainedmodels API
type TrainedModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainedModelSpec   `json:"spec,omitempty"`
	Status TrainedModelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TrainedModelList contains a list of TrainedModel
type TrainedModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainedModel `json:"items"`
}

// ModelSpec describes a TrainedModel
type ModelSpec struct {
	// Storage URI for the model repository
	StorageURI string `json:"storageUri"`
	// Machine Learning <framework name>
	// The values could be: "tensorflow","pytorch","sklearn","onnx","xgboost", "myawesomeinternalframework" etc.
	Framework string `json:"framework"`
	// Maximum memory this model will consume, this field is used to decide if a model server has enough memory to load this model.
	Memory resource.Quantity `json:"memory"`
}

func init() {
	SchemeBuilder.Register(&TrainedModel{}, &TrainedModelList{})
}
