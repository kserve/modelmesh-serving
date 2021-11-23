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

// The common package contains API types used by both Predictors and InferenceServices.
// Having a separate package avoids circular dependencies.
//+kubebuilder:object:generate=true
package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	// How many copies of this predictor's models failed to load recently
	// +kubebuilder:default=0
	FailedCopies int `json:"failedCopies"`
}

func (s *PredictorStatus) WaitingForRuntime() bool {
	return s.LastFailureInfo != nil && s.LastFailureInfo.Reason == RuntimeUnhealthy
}
