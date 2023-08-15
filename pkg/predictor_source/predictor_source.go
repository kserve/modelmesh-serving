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

package predictor_source

import (
	"context"
	"strings"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// *IMPORTANT* The interfaces defined in this file should currently be considered
// unstable and subject to change.

// PredictorSource provides a registry of Predictors along with a channel
// of update events corresponding to the Predictors. An event should be produced
// for every Predictor upon starting, and subsequently each time any Predictor
// is added, has its Spec modified, or is deleted.
type PredictorSource interface {
	// StartWatch should not return until PredictorRegistry is fully populated
	StartWatch(ctx context.Context) (PredictorRegistry, PredictorEventChan, error)
	// GetSourceId return short, fixed and unique identifier for this source
	GetSourceId() string
}

type PredictorEvent types.NamespacedName

type PredictorEventChan chan PredictorEvent

func (pec *PredictorEventChan) Event(name string, namespace string) {
	*pec <- PredictorEvent{Name: name, Namespace: namespace}
}

type PredictorRegistry interface {
	// Get should retrieve the Predictor from a local cache that's synchronized via a watch
	Get(ctx context.Context, name types.NamespacedName) (*api.Predictor, error)

	// Find is temporary until runtime deployment scaling logic is refactored
	Find(ctx context.Context, namespace string, predicate func(*api.Predictor) bool) (bool, error)

	UpdateStatus(ctx context.Context, predictor *api.Predictor) (bool, error)

	GetSourceName() string
}

func ResolveSource(nn types.NamespacedName, defaultSource string) (types.NamespacedName, string) {
	namespace := nn.Namespace
	// Check if namespace has a source prefix
	i := strings.LastIndex(namespace, "_")
	nn.Namespace = namespace[i+1:]
	if i <= 0 {
		return nn, defaultSource
	}
	return nn, namespace[:i]
}
