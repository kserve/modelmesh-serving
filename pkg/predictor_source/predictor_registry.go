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

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ PredictorRegistry = (*PredictorCRRegistry)(nil)

type PredictorCRRegistry struct {
	Client client.Client
}

func (pr PredictorCRRegistry) Get(ctx context.Context, name types.NamespacedName) (*api.Predictor, error) {
	p := &api.Predictor{}
	if err := pr.Client.Get(ctx, name, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (pr PredictorCRRegistry) Find(ctx context.Context, namespace string,
	predicate func(*api.Predictor) bool) (bool, error) {
	list := &api.PredictorList{}
	if err := pr.Client.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return false, err
	}
	for i := range list.Items {
		if predicate(&list.Items[i]) {
			return true, nil
		}
	}
	return false, nil
}

// UpdateStatus returns true if update was successful
func (pr PredictorCRRegistry) UpdateStatus(ctx context.Context, predictor *api.Predictor) (bool, error) {
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
