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

package predictor_source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/apis/serving/v1beta1"

	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ PredictorRegistry = (*InferenceServiceRegistry)(nil)

type InferenceServiceRegistry struct {
	Client client.Client
}

func BuildBasePredictorFromInferenceService(isvc *v1beta1.InferenceService) (*v1alpha1.Predictor, error) {
	p := &v1alpha1.Predictor{}

	// Check if resource should be reconciled.
	if isvc.ObjectMeta.Annotations[v1beta1.DeploymentModeAnnotation] != v1beta1.MMDeploymentModeVal {
		return nil, nil
	}

	framework, frameworkSpec := isvc.Spec.Predictor.GetPredictorFramework()
	if frameworkSpec == nil {
		return nil, errors.New("No valid InferenceService predictor framework found")
	}

	p.Spec = v1alpha1.PredictorSpec{
		Model: v1alpha1.Model{
			Type: v1alpha1.ModelType{
				Name: framework,
			},
		},
	}

	// If explicit ServingRuntime was passed in through an annotation
	if runtime, ok := isvc.ObjectMeta.Annotations[v1beta1.RuntimeAnnotation]; ok {
		p.Spec.Runtime = &v1alpha1.PredictorRuntime{
			RuntimeRef: &v1alpha1.RuntimeRef{
				Name: runtime,
			},
		}
	}
	return p, nil
}

// Return secretKey, bucket, modelPath, and error
func ProcessInferenceServiceStorage(secretKey *string, bucket *string, modelPath *string, schemaPath *string,
	inferenceService *v1beta1.InferenceService, nname types.NamespacedName) error {
	_, frameworkSpec := inferenceService.Spec.Predictor.GetPredictorFramework()
	storageUri := frameworkSpec.StorageURI
	storageSpec := frameworkSpec.Storage
	pathError := fmt.Errorf("the InferenceService %v must have either the storageUri or the storage.path", nname)
	if storageUri == nil {
		if storageSpec != nil {
			if storageSpec.Path != nil {
				*modelPath = *storageSpec.Path
				if storageSpec.SchemaPath != nil {
					*schemaPath = *storageSpec.SchemaPath
				}
			} else {
				return pathError
			}
		} else {
			return pathError
		}
	} else {
		if !strings.HasPrefix(*storageUri, "s3://") {
			return nil
		}
		s3Uri := strings.TrimPrefix(*storageUri, "s3://")
		urlParts := strings.Split(s3Uri, "/")
		*bucket = urlParts[0]
		*modelPath = strings.Join(urlParts[1:], "/")
		if storageSpec != nil {
			if storageSpec.Path != nil {
				return fmt.Errorf("the InferenceService %v cannot have both the storageUri and the storage.path", nname)
			}
		}
	}
	if storageSpec != nil {
		if storageSpec.StorageKey != nil {
			*secretKey = *storageSpec.StorageKey
		}
		if storageSpec.Parameters != nil {
			for k, v := range *storageSpec.Parameters {
				if k == "bucket" {
					*bucket = v
				}
			}
		}
	}
	if *secretKey == "" {
		*secretKey = "default"
	}
	return nil
}

func (isvcr InferenceServiceRegistry) Get(ctx context.Context, nname types.NamespacedName) (*v1alpha1.Predictor, error) {
	inferenceService := &v1beta1.InferenceService{}

	if err := isvcr.Client.Get(ctx, nname, inferenceService); err != nil {
		return nil, err
	}

	p, err := BuildBasePredictorFromInferenceService(inferenceService)

	if err != nil {
		return nil, err
	}

	// This is the case where an InferenceService was found, but the
	// ModelMesh annotation was not set.
	if p == nil {
		return nil, nil
	}

	p.ObjectMeta = inferenceService.ObjectMeta
	p.Status = inferenceService.Status.PredictorStatus

	if p.Status.ActiveModelState == "" {
		p.Status.ActiveModelState = v1alpha1.Pending
	}

	p.Spec.Storage = &v1alpha1.Storage{}
	secretKey := inferenceService.ObjectMeta.Annotations[v1beta1.SecretKeyAnnotation]
	if schemaPath, ok := inferenceService.ObjectMeta.Annotations[v1beta1.SchemaPathAnnotation]; ok {
		p.Spec.SchemaPath = &schemaPath
	}
	var bucket string
	var modelPath string
	var schemaPath string
	err = ProcessInferenceServiceStorage(&secretKey, &bucket, &modelPath, &schemaPath, inferenceService, nname)
	if err != nil {
		return nil, err
	}
	// If secretKey is empty, it means the storageSpec or the storageUri is not supported.
	if secretKey == "" {
		return p, nil
	}
	if schemaPath, ok := inferenceService.ObjectMeta.Annotations[v1beta1.SchemaPathAnnotation]; ok {
		p.Spec.SchemaPath = &schemaPath
	}
	p.Spec.Storage.S3 = &v1alpha1.S3StorageSource{
		SecretKey: secretKey,
		Bucket:    &bucket,
	}
	p.Spec.Path = modelPath

	return p, nil

}

func (isvcr InferenceServiceRegistry) Find(ctx context.Context, namespace string,
	predicate func(*v1alpha1.Predictor) bool) (bool, error) {

	list := &v1beta1.InferenceServiceList{}
	if err := isvcr.Client.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	for i := range list.Items {
		p, _ := BuildBasePredictorFromInferenceService(&list.Items[i])
		if p != nil && predicate(p) {
			return true, nil
		}
	}
	return false, nil
}

func (isvcr InferenceServiceRegistry) UpdateStatus(ctx context.Context, predictor *v1alpha1.Predictor) (bool, error) {
	inferenceService := &v1beta1.InferenceService{}

	inferenceService.ObjectMeta = predictor.ObjectMeta
	inferenceService.Status.PredictorStatus = predictor.Status

	if predictor.Status.Available {
		inferenceService.Status.Conditions = v1beta1.Conditions{
			v1beta1.Condition{
				Type:   "Ready",
				Status: corev1.ConditionTrue,
			},
		}
		inferenceService.Status.URL = predictor.Status.GrpcEndpoint
	} else {
		inferenceService.Status.Conditions = v1beta1.Conditions{
			v1beta1.Condition{
				Type:   "Ready",
				Status: corev1.ConditionFalse,
			},
		}
	}
	if err := isvcr.Client.Status().Update(ctx, inferenceService); err != nil {
		if k8serr.IsConflict(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (isvcr InferenceServiceRegistry) GetSourceName() string {
	return "InferenceService"
}
