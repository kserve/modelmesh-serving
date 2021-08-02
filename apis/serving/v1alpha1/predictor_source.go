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
	"context"

	"k8s.io/apimachinery/pkg/types"
)

// *IMPORTANT* The interfaces defined in this file should currently be considered
// unstable and subject to change.

// +kubebuilder:object:generate=false
// PredictorSource provides a registry of Predictors along with a channel
// of update events corresponding to the Predictors. An event should be produced
// for every Predictor upon starting, and subsequently each time any Predictor
// is added, has its Spec modified, or is deleted.
type PredictorSource interface {
	StartWatch() (PredictorRegistry, <-chan PredictorEvent, error)
	// GetSourceId return short, fixed and unique identifier for this source
	GetSourceId() string
}

// +kubebuilder:object:generate=false
type PredictorRegistry interface {
	Get(ctx context.Context, name types.NamespacedName) (*Predictor, error)
	UpdateStatus(ctx context.Context, predictor *Predictor) (bool, error)
	GetSourceName() string
}

// +kubebuilder:object:generate=false
type PredictorEvent struct {
	Event     int // enum
	Name      string
	Namespace string
}
