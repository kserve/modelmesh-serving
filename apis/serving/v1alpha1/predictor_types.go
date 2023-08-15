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
	"github.com/kserve/kserve/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// see if this is possible
// +k8s:openapi-gen=true
type PredictorRuntime struct {
	// one-of these must be present
	*RuntimeRef `json:",inline"`
	//TODO this option will be incorporated later
	//*ServingRuntimePodSpec `json:",inline"`
}

// +k8s:openapi-gen=true
type RuntimeRef struct {
	Name string `json:"name"`
}

// +k8s:openapi-gen=true
type Storage struct {
	// new way to specify the storage configuration
	StorageSpec `json:",inline"`

	// Below fields DEPRECATED and remain for backwards compatibility only
	// if one is present, no other fields from StorageSpec can be
	// specified

	// (DEPRECATED) PersistentVolmueClaim was never supported this way and will be removed
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// (DEPRECATED) S3 has configuration to connect to an S3 instance. It is now deprecated, use fields from Spec.Storage instead.
	S3 *S3StorageSource `json:"s3,omitempty"`
}

// +k8s:openapi-gen=true
type S3StorageSource struct {
	// +required
	SecretKey string `json:"secretKey" validation:"required"`
	// +optional
	Bucket *string `json:"bucket,omitempty" validation:"required"`
}

// +k8s:openapi-gen=true
type ModelType struct {
	// +required
	Name string `json:"name"`
	// +optional
	Version *string `json:"version,omitempty"`
}

// +k8s:openapi-gen=true
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
// +k8s:openapi-gen=true
type GpuRequest string

// GpuRequest Enum
const (
	// Predictor requires GPU
	Required GpuRequest = "required"
	// Predictor prefers GPU
	Preferred GpuRequest = "preferred"
)

// PredictorSpec defines the desired state of Predictor
// +k8s:openapi-gen=true
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
	// Protocol version to be exposed by the predictor (i.e. v1 or v2 or grpc-v1 or grpc-v2)
	// +optional
	ProtocolVersion *constants.InferenceServiceProtocol `json:"protocolVersion,omitempty"`
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
// +k8s:openapi-gen=true
type Predictor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PredictorSpec `json:"spec,omitempty"`

	// Add these to the default below once reinstated: {loadedCopies:0, loadingCopies:0}

	// +kubebuilder:default={transitionStatus:UpToDate, activeModelState:Pending, targetModelState:"", available:false, failedCopies:0, totalCopies:0}
	Status PredictorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PredictorList contains a list of Predictor
// +k8s:openapi-gen=true
type PredictorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Predictor `json:"items"`
}

// +k8s:openapi-gen=true
type StorageSpec struct {
	// The path to the model object in the storage. It cannot co-exist
	// with the storageURI.
	// +optional
	Path *string `json:"path,omitempty"`
	// The path to the model schema file in the storage.
	// +optional
	SchemaPath *string `json:"schemaPath,omitempty"`
	// Parameters to override the default storage credentials and config.
	// +optional
	Parameters *map[string]string `json:"parameters,omitempty"`
	// The Storage Key in the secret for this model.
	// +optional
	StorageKey *string `json:"key,omitempty"`
}

// TransitionStatus enum
// +kubebuilder:validation:Enum=UpToDate;InProgress;BlockedByFailedLoad;InvalidSpec
// +k8s:openapi-gen=true
type TransitionStatus string

// TransitionStatus Enum values
const (
	// Predictor is up-to-date (reflects current spec)
	UpToDate TransitionStatus = "UpToDate"
	// Waiting for target model to reach state of active model
	InProgress TransitionStatus = "InProgress"
	// Target model failed to load
	BlockedByFailedLoad TransitionStatus = "BlockedByFailedLoad"
	// TBD
	InvalidSpec TransitionStatus = "InvalidSpec"
)

// ModelState enum
// +kubebuilder:validation:Enum="";Pending;Standby;Loading;Loaded;FailedToLoad
// +k8s:openapi-gen=true
type ModelState string

// ModelState Enum values
const (
	// Model is not yet registered
	Pending ModelState = "Pending"
	// Model is available but not loaded (will load when used)
	Standby ModelState = "Standby"
	// Model is loading
	Loading ModelState = "Loading"
	// At least one copy of the model is loaded
	Loaded ModelState = "Loaded"
	// All copies of the model failed to load
	FailedToLoad ModelState = "FailedToLoad"
)

// FailureReason enum
// +kubebuilder:validation:Enum=ModelLoadFailed;RuntimeUnhealthy;NoSupportingRuntime;RuntimeNotRecognized;InvalidPredictorSpec
// +k8s:openapi-gen=true
type FailureReason string

// FailureReason enum values
const (
	// The model failed to load within a ServingRuntime container
	ModelLoadFailed FailureReason = "ModelLoadFailed"
	// Corresponding ServingRuntime containers failed to start or are unhealthy
	RuntimeUnhealthy FailureReason = "RuntimeUnhealthy"
	// There are no ServingRuntime which support the specified model type
	NoSupportingRuntime FailureReason = "NoSupportingRuntime"
	// There is no ServingRuntime defined with the specified runtime name
	RuntimeNotRecognized FailureReason = "RuntimeNotRecognized"
	// The current Predictor Spec is invalid or unsupported
	InvalidPredictorSpec FailureReason = "InvalidPredictorSpec"
)

// +k8s:openapi-gen=true
type FailureInfo struct {
	// Name of component to which the failure relates (usually Pod name)
	//+optional
	Location string `json:"location,omitempty"`
	// High level class of failure
	//+optional
	Reason FailureReason `json:"reason,omitempty"`
	// Detailed error message
	//+optional
	Message string `json:"message,omitempty"`
	// Internal ID of model, tied to specific Spec contents
	//+optional
	ModelId string `json:"modelId,omitempty"`
	// Time failure occurred or was discovered
	//+optional
	Time *metav1.Time `json:"time,omitempty"`
}

// PredictorStatus defines the observed state of Predictor
// +k8s:openapi-gen=true
type PredictorStatus struct {

	//TODO Conditions/Phases TBD

	// Whether the predictor endpoint is available
	Available bool `json:"available"`
	// Whether the available predictor endpoint reflects the current Spec or is in transition
	// +kubebuilder:default=UpToDate
	TransitionStatus TransitionStatus `json:"transitionStatus"`

	// High level state string: Pending, Standby, Loading, Loaded, FailedToLoad
	// +kubebuilder:default=Pending
	ActiveModelState ModelState `json:"activeModelState"`
	// +kubebuilder:default=""
	TargetModelState ModelState `json:"targetModelState"`

	// Details of last failure, when load of target model is failed or blocked
	//+optional
	LastFailureInfo *FailureInfo `json:"lastFailureInfo,omitempty"`

	// Addressable endpoint for the deployed trained model
	// This will be "static" and will not change when the model is mutated
	// +optional
	HTTPEndpoint string `json:"httpEndpoint"`
	// +optional
	GrpcEndpoint string `json:"grpcEndpoint"`

	//TODO TBC whether or not these are exposed here
	//	// How many copies of this predictor's models are currently loaded - NOT YET SUPPORTED
	//	// +kubebuilder:default=0
	//	LoadedCopies int `json:"loadedCopies"`
	//	// How many copies of this predictor's models are currently loading - NOT YET SUPPORTED
	//	// +kubebuilder:default=0
	//	LoadingCopies int `json:"loadingCopies"`

	// Total number of copies of this predictor's models
	// +kubebuilder:default=0
	TotalCopies int `json:"totalCopies"`

	// How many copies of this predictor's models failed to load recently
	// +kubebuilder:default=0
	FailedCopies int `json:"failedCopies"`
}

func (s *PredictorStatus) WaitingForRuntime() bool {
	return s.LastFailureInfo != nil && s.LastFailureInfo.Reason == RuntimeUnhealthy
}

func init() {
	SchemeBuilder.Register(&Predictor{}, &PredictorList{})
}
