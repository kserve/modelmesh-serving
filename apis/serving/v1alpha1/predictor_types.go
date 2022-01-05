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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kserve/modelmesh-serving/apis/serving/common"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// see if this is possible
type PredictorRuntime struct {
	// one-of these must be present
	*RuntimeRef `json:",inline"`
	//TODO this option will be incorporated later
	//*ServingRuntimePodSpec `json:",inline"`
}

type RuntimeRef struct {
	Name string `json:"name"`
}

type Storage struct {
	// new way to specify the storage configuration
	common.StorageSpec `json:",inline"`

	// Below fields DEPRECATED and remain for backwards compatibility only
	// if one is present, no other fields from common.StorageSpec can be
	// specified

	// (DEPRECATED) PersistentVolmueClaim was never supported this way and will be removed
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// (DEPRECATED) S3 has configuration to connect to an S3 instance. It is now deprecated, use fields from Spec.Storage instead.
	S3 *S3StorageSource `json:"s3,omitempty"`
}

type S3StorageSource struct {
	// +required
	SecretKey string `json:"secretKey" validation:"required"`
	// +optional
	Bucket *string `json:"bucket,omitempty" validation:"required"`
}

type ModelType struct {
	// +required
	Name string `json:"name"`
	// +optional
	Version *string `json:"version,omitempty"`
}

type Model struct {
	// +required
	Type ModelType `json:"modelType"`

	// (DEPRECATED) The path to the model files within the storage
	// +optional
	Path string `json:"path"`

	// (DEPRECATED) The path to the schema file within the storage
	// +optional
	SchemaPath *string `json:"schemaPath,omitempty"`

	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// GpuRequest constant for specifying GPU requirement or preference
// +kubebuilder:validation:Enum=required;preferred
type GpuRequest string

// GpuRequest Enum
const (
	// Predictor requires GPU
	Required GpuRequest = "required"
	// Predictor prefers GPU
	Preferred GpuRequest = "preferred"
)

// PredictorSpec defines the desired state of Predictor
type PredictorSpec struct {
	// NOT YET SUPPORTED
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// +required
	Model `json:",inline"`
	// May be absent, "preferred" or "required"
	// +optional
	Gpu *GpuRequest `json:"gpu,omitempty"`
	// If omitted a compatible runtime is selected based on the model type (if available)
	// +optional
	Runtime *PredictorRuntime `json:"runtime,omitempty"`
}

// too wide if this is included
// // +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.grpcEndpoint"

// Predictor is the Schema for the predictors API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.modelType.name"
// +kubebuilder:printcolumn:name="Available",type="boolean",JSONPath=".status.available"
// +kubebuilder:printcolumn:name="ActiveModel",type="string",JSONPath=".status.activeModelState"
// +kubebuilder:printcolumn:name="TargetModel",type="string",JSONPath=".status.targetModelState"
// +kubebuilder:printcolumn:name="Transition",type="string",JSONPath=".status.transitionStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Predictor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PredictorSpec `json:"spec,omitempty"`

	// Add these to the default below once reinstated: {loadedCopies:0, loadingCopies:0}

	// +kubebuilder:default={transitionStatus:UpToDate, activeModelState:Pending, targetModelState:"", available:false, failedCopies:0}
	Status common.PredictorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PredictorList contains a list of Predictor
type PredictorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Predictor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Predictor{}, &PredictorList{})
}
