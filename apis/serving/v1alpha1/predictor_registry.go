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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:object:generate=false
type PredictorCRRegistry struct {
	Client client.Client
}

func (pr PredictorCRRegistry) Get(ctx context.Context, name types.NamespacedName) (*Predictor, error) {
	p := &Predictor{}
	if err := pr.Client.Get(ctx, name, p); err != nil {
		return p, err
	}
	return p, nil
}

// UpdateStatus returns true if update was successful
func (pr PredictorCRRegistry) UpdateStatus(ctx context.Context, predictor *Predictor) (bool, error) {
	if err := pr.Client.Status().Update(ctx, predictor); err != nil {
		if errors.IsConflict(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (pr PredictorCRRegistry) GetSourceName() string {
	return "Predictor"
}
