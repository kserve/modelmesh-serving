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
	"net/url"
	"strings"

	"github.com/kserve/modelmesh-serving/apis/serving/common"
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

	p.ObjectMeta = isvc.ObjectMeta

	framework, frameworkSpec := isvc.Spec.Predictor.GetPredictorFramework()
	runtimeFromAnnotation, runtimeAnnotationExists := isvc.ObjectMeta.Annotations[v1beta1.RuntimeAnnotation]

	if isvc.Spec.Predictor.Model != nil {

		if frameworkSpec != nil {
			return nil, fmt.Errorf("the InferenceService %v cannot have both the model spec and a framework spec (%v)", isvc.Name, framework)
		}
		if runtimeAnnotationExists {
			return nil, fmt.Errorf("the InferenceService %v cannot have both the model spec and the "+
				"runtime annotation %v", isvc.Name, v1beta1.RuntimeAnnotation)
		}

		p.Spec = v1alpha1.PredictorSpec{
			Model: v1alpha1.Model{
				Type: v1alpha1.ModelType{
					Name:    isvc.Spec.Predictor.Model.ModelFormat.Name,
					Version: isvc.Spec.Predictor.Model.ModelFormat.Version,
				},
			},
		}
		if isvc.Spec.Predictor.Model.Runtime != nil {
			p.Spec.Runtime = &v1alpha1.PredictorRuntime{
				RuntimeRef: &v1alpha1.RuntimeRef{
					Name: *isvc.Spec.Predictor.Model.Runtime,
				},
			}
		}
	} else {
		// Note: This block of logic is here to maintain backwards compatibility and
		// will be removed in the future.
		if frameworkSpec == nil {
			return nil, errors.New("no valid InferenceService predictor framework found")
		}

		p.Spec = v1alpha1.PredictorSpec{
			Model: v1alpha1.Model{
				Type: v1alpha1.ModelType{
					Name: framework,
				},
			},
		}

		// If explicit ServingRuntime was passed in through an annotation
		if runtimeAnnotationExists {
			p.Spec.Runtime = &v1alpha1.PredictorRuntime{
				RuntimeRef: &v1alpha1.RuntimeRef{
					Name: runtimeFromAnnotation,
				},
			}
		}
	}
	return p, nil
}

// Return secretKey, bucket, modelPath, schemaPath, and error
func processInferenceServiceStorage(inferenceService *v1beta1.InferenceService, nname types.NamespacedName) (
	secretKey *string, parameters map[string]string, modelPath string, schemaPath *string, err error) {

	var pSpec *v1beta1.PredictorExtensionSpec
	if inferenceService.Spec.Predictor.Model != nil {
		pSpec = &inferenceService.Spec.Predictor.Model.PredictorExtensionSpec

	} else {
		_, pSpec = inferenceService.Spec.Predictor.GetPredictorFramework()
	}

	storageUri := pSpec.StorageURI
	storageSpec := pSpec.Storage
	uriParameters := make(map[string]string)

	if storageUri == nil {
		if storageSpec == nil || storageSpec.Path == nil {
			err = fmt.Errorf("the InferenceService %v must have either the storageUri or the storage.path", nname)
			return
		}
		modelPath = *storageSpec.Path
	} else {
		if storageSpec != nil && storageSpec.Path != nil {
			err = fmt.Errorf("the InferenceService %v cannot have both the storageUri and the storage.path", nname)
			return
		}

		u, urlErr := url.Parse(*storageUri)
		if urlErr != nil {
			err = fmt.Errorf("could not parse storageUri in InferenceService %v: %w", nname, urlErr)
			return
		}

		switch u.Scheme {
		case "s3":
			modelPath = strings.TrimPrefix(u.Path, "/")
			uriParameters["type"] = "s3"
			uriParameters["bucket"] = u.Host
		case "gs":
			modelPath = strings.TrimPrefix(u.Path, "/")
			uriParameters["type"] = "gcs"
			uriParameters["bucket"] = u.Host
		case "http", "https":
			uriParameters["type"] = "http"
			uriParameters["url"] = *storageUri
		default:
			err = fmt.Errorf("the InferenceService %v has an unsupported storageUri scheme %v", nname, u.Scheme)
			return
		}

	}

	var storageSpecParameters map[string]string
	if storageSpec != nil {
		if storageSpec.StorageKey != nil {
			secretKey = storageSpec.StorageKey
		}
		if storageSpec.Parameters != nil {
			storageSpecParameters = *storageSpec.Parameters
		}
		if storageSpec.SchemaPath != nil {
			schemaPath = storageSpec.SchemaPath
		}
	}

	// resolve the parameters, URI parameters taking precedence
	if storageSpecParameters != nil {
		parameters = storageSpecParameters
		for k, v := range uriParameters {
			parameters[k] = v
		}
	} else {
		parameters = uriParameters
	}

	// alternative source for SecretKey for backwards compatibility
	if secretKey == nil {
		if sk, ok := inferenceService.ObjectMeta.Annotations[v1beta1.SecretKeyAnnotation]; ok {
			secretKey = &sk
		}
	}

	// alternative source for SchemaPath for backwards compatibility
	if schemaPath == nil {
		if sp, ok := inferenceService.ObjectMeta.Annotations[v1beta1.SchemaPathAnnotation]; ok {
			schemaPath = &sp
		}
	}

	return
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

	p.Status = inferenceService.Status.PredictorStatus

	if p.Status.ActiveModelState == "" {
		p.Status.ActiveModelState = common.Pending
	}

	secretKey, parameters, modelPath, schemaPath, err := processInferenceServiceStorage(inferenceService, nname)
	if err != nil {
		return nil, err
	}

	p.Spec.Storage = &v1alpha1.Storage{}
	p.Spec.Storage.Path = &modelPath
	p.Spec.Storage.SchemaPath = schemaPath
	p.Spec.Storage.Parameters = &parameters
	p.Spec.Storage.StorageKey = secretKey

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
