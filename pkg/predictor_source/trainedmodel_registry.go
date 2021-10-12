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

	kfsapi "github.com/kserve/modelmesh-serving/apis/kfserving/v1alpha1"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ PredictorRegistry = (*TrainedModelRegistry)(nil)

type TrainedModelRegistry struct {
	Client client.Client
}

func BuildBasePredictorFromTrainedModel(tm *kfsapi.TrainedModel) *api.Predictor {
	p := &api.Predictor{}
	p.Spec = api.PredictorSpec{
		Model: api.Model{
			Type: api.ModelType{
				Name: tm.Spec.Model.Framework,
			},
		},
	}

	if tm.Spec.InferenceService != "" {
		p.Spec.Runtime = &api.PredictorRuntime{
			RuntimeRef: &api.RuntimeRef{
				Name: tm.Spec.InferenceService,
			},
		}
	}
	return p
}

func (tmr TrainedModelRegistry) Find(ctx context.Context, namespace string,
	predicate func(*api.Predictor) bool) (bool, error) {
	list := &kfsapi.TrainedModelList{}
	err := tmr.Client.List(ctx, list, client.InNamespace(namespace))
	if err != nil {
		return false, err
	}
	for i := range list.Items {
		if predicate(BuildBasePredictorFromTrainedModel(&list.Items[i])) {
			return true, nil
		}
	}
	return false, nil
}

func (tmr TrainedModelRegistry) Get(ctx context.Context, nname types.NamespacedName) (*api.Predictor, error) {
	trainedModel := &kfsapi.TrainedModel{}
	err := tmr.Client.Get(ctx, nname, trainedModel)

	if err != nil {
		return nil, err
	}

	p := BuildBasePredictorFromTrainedModel(trainedModel)
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
		secretKey := trainedModel.ObjectMeta.Annotations[kfsapi.SecretKeyAnnotation]
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
	trainedModel := &kfsapi.TrainedModel{}
	trainedModel.TypeMeta = predictor.TypeMeta
	trainedModel.ObjectMeta = predictor.ObjectMeta
	trainedModel.Status.PredictorStatus = predictor.Status

	if predictor.Status.Available {
		trainedModel.Status.Conditions = kfsapi.Conditions{
			kfsapi.Condition{
				Type:   "Ready",
				Status: corev1.ConditionTrue,
			},
		}
		trainedModel.Status.URL = predictor.Status.GrpcEndpoint
	} else {
		trainedModel.Status.Conditions = kfsapi.Conditions{
			kfsapi.Condition{
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
