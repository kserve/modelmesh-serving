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
	"strings"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:object:generate=false
type TrainedModelRegistry struct {
	Client client.Client
}

func BuildPredictorWithBase(t *TrainedModel) *api.Predictor {
	p := &api.Predictor{}
	p.Spec = api.PredictorSpec{
		Model: api.Model{
			Type: api.ModelType{
				Name: t.Spec.Model.Framework,
			},
		},
	}

	if t.Spec.InferenceService != "" {
		p.Spec.Runtime = &api.PredictorRuntime{
			RuntimeRef: &api.RuntimeRef{
				Name: t.Spec.InferenceService,
			},
		}
	}
	return p
}

func (tmr TrainedModelRegistry) Get(ctx context.Context, nname types.NamespacedName) (*api.Predictor, error) {
	trainedModel := &TrainedModel{}
	err := tmr.Client.Get(ctx, nname, trainedModel)

	if err != nil {
		return nil, err
	}

	p := BuildPredictorWithBase(trainedModel)
	p.TypeMeta = trainedModel.TypeMeta
	p.ObjectMeta = trainedModel.ObjectMeta
	p.Status = trainedModel.Status.PredictorStatus

	if p.Status.ActiveModelState == "" {
		p.Status.ActiveModelState = api.Pending
	}

	storageUri := trainedModel.Spec.Model.StorageURI
	if !strings.HasPrefix(storageUri, "s3://") {
		p.Spec.Storage = &api.Storage{
			S3: nil,
		}
	} else {
		s3Uri := strings.TrimPrefix(storageUri, "s3://")
		urlParts := strings.Split(s3Uri, "/")
		bucket := urlParts[0]
		modelPath := strings.Join(urlParts[1:], "/")
		secretKey := trainedModel.ObjectMeta.Annotations[SecretKeyAnnotation]
		p.Spec.Storage = &api.Storage{
			S3: &api.S3StorageSource{
				SecretKey: secretKey,
				Bucket:    &bucket,
			},
		}
		p.Spec.Path = modelPath
	}
	return p, err
}

func (tmr TrainedModelRegistry) UpdateStatus(ctx context.Context, predictor *api.Predictor) (bool, error) {
	trainedModel := &TrainedModel{}
	trainedModel.TypeMeta = predictor.TypeMeta
	trainedModel.ObjectMeta = predictor.ObjectMeta
	trainedModel.Status.PredictorStatus = predictor.Status

	if predictor.Status.Available {
		trainedModel.Status.Conditions = Conditions{
			Condition{
				Type:   "Ready",
				Status: corev1.ConditionTrue,
			},
		}
		trainedModel.Status.URL = predictor.Status.GrpcEndpoint
	} else {
		trainedModel.Status.Conditions = Conditions{
			Condition{
				Type:   "Ready",
				Status: corev1.ConditionFalse,
			},
		}
	}
	if err := tmr.Client.Status().Update(ctx, trainedModel); err != nil {
		if errors.IsConflict(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (tmr TrainedModelRegistry) GetSourceName() string {
	return "TrainedModel"
}
