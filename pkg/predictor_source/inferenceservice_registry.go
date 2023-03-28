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

	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveConstants "github.com/kserve/kserve/pkg/constants"
	"github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"knative.dev/pkg/apis"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretKeyAnnotation  = "serving.kserve.io/secretKey"
	schemaPathAnnotation = "serving.kserve.io/schemaPath"
	runtimeAnnotation    = "serving.kserve.io/servingRuntime"

	azureBlobHostSuffix = "blob.core.windows.net"
)

var _ PredictorRegistry = (*InferenceServiceRegistry)(nil)

type InferenceServiceRegistry struct {
	Client client.Client
}

var conditionSet = apis.NewLivingConditionSet(
	v1beta1.PredictorReady,
)

func BuildBasePredictorFromInferenceService(isvc *v1beta1.InferenceService) (*v1alpha1.Predictor, error) {
	p := &v1alpha1.Predictor{}

	// Check if resource should be reconciled.
	if isvc.ObjectMeta.Annotations[kserveConstants.DeploymentMode] != string(kserveConstants.ModelMeshDeployment) {
		return nil, nil
	}

	p.ObjectMeta = isvc.ObjectMeta

	framework, frameworkSpec := getPredictorFramework(&isvc.Spec.Predictor)
	runtimeFromAnnotation, runtimeAnnotationExists := isvc.ObjectMeta.Annotations[runtimeAnnotation]

	if isvc.Spec.Predictor.Model != nil {

		if frameworkSpec != nil {
			return nil, fmt.Errorf("the InferenceService %v cannot have both the model spec and a framework spec (%v)", isvc.Name, framework)
		}
		if runtimeAnnotationExists {
			return nil, fmt.Errorf("the InferenceService %v cannot have both the model spec and the "+
				"runtime annotation %v", isvc.Name, runtimeAnnotation)
		}

		p.Spec = v1alpha1.PredictorSpec{
			Model: v1alpha1.Model{
				Type: v1alpha1.ModelType{
					Name:    isvc.Spec.Predictor.Model.ModelFormat.Name,
					Version: isvc.Spec.Predictor.Model.ModelFormat.Version,
				},
			},
		}

		protocolVersion := isvc.Spec.Predictor.Model.ProtocolVersion
		if protocolVersion != nil && *protocolVersion != kserveConstants.ProtocolUnknown {
			p.Spec.ProtocolVersion = protocolVersion
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

		protocolVersion := frameworkSpec.ProtocolVersion
		if protocolVersion != nil && *protocolVersion != kserveConstants.ProtocolUnknown {
			p.Spec.ProtocolVersion = protocolVersion
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
		_, pSpec = getPredictorFramework(&inferenceService.Spec.Predictor)
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
		case "pvc":
			modelPath = strings.TrimPrefix(u.Path, "/")
			uriParameters["type"] = "pvc"
			uriParameters["name"] = u.Host
		case "s3":
			modelPath = strings.TrimPrefix(u.Path, "/")
			uriParameters["type"] = "s3"
			uriParameters["bucket"] = u.Host
		case "gs":
			modelPath = strings.TrimPrefix(u.Path, "/")
			uriParameters["type"] = "gcs"
			uriParameters["bucket"] = u.Host
		case "http", "https":
			if strings.HasSuffix(u.Host, azureBlobHostSuffix) {
				uriParameters["type"] = "azure"

				pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
				if len(pathParts) < 1 {
					err = fmt.Errorf("the InferenceService %v has an invalid URL path %v", nname, *storageUri)
					return
				}
				hostParts := strings.Split(u.Host, ".")
				uriParameters["container"] = pathParts[0]
				uriParameters["account_name"] = hostParts[0]
				modelPath = strings.Join(pathParts[1:], "/")
			} else {
				uriParameters["type"] = "http"
				uriParameters["url"] = *storageUri
			}
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
		if sk, ok := inferenceService.ObjectMeta.Annotations[secretKeyAnnotation]; ok {
			secretKey = &sk
		}
	}

	// alternative source for SchemaPath for backwards compatibility
	if schemaPath == nil {
		if sp, ok := inferenceService.ObjectMeta.Annotations[schemaPathAnnotation]; ok {
			schemaPath = &sp
		}
	}

	return
}

func getPredictorFramework(s *v1beta1.PredictorSpec) (string, *v1beta1.PredictorExtensionSpec) {
	if s.XGBoost != nil {
		return "xgboost", &s.XGBoost.PredictorExtensionSpec
	} else if s.LightGBM != nil {
		return "lightgbm", &s.LightGBM.PredictorExtensionSpec
	} else if s.SKLearn != nil {
		return "sklearn", &s.SKLearn.PredictorExtensionSpec
	} else if s.Tensorflow != nil {
		return "tensorflow", &s.Tensorflow.PredictorExtensionSpec
	} else if s.ONNX != nil {
		return "onnx", &s.ONNX.PredictorExtensionSpec
	} else if s.PyTorch != nil {
		return "pytorch", &s.PyTorch.PredictorExtensionSpec
	} else if s.Triton != nil {
		return "triton", &s.Triton.PredictorExtensionSpec
	} else if s.PMML != nil {
		return "pmml", &s.PMML.PredictorExtensionSpec
	} else {
		return "", nil
	}
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

	p.Status = v1alpha1.PredictorStatus{}
	p.Status.TransitionStatus = v1alpha1.TransitionStatus(inferenceService.Status.ModelStatus.TransitionStatus)
	if inferenceService.Status.ModelStatus.ModelCopies != nil {
		p.Status.FailedCopies = inferenceService.Status.ModelStatus.ModelCopies.FailedCopies
		p.Status.TotalCopies = inferenceService.Status.ModelStatus.ModelCopies.TotalCopies
	}
	if inferenceService.Status.ModelStatus.ModelRevisionStates != nil {
		p.Status.ActiveModelState = v1alpha1.ModelState(inferenceService.Status.ModelStatus.ModelRevisionStates.ActiveModelState)
		p.Status.TargetModelState = v1alpha1.ModelState(inferenceService.Status.ModelStatus.ModelRevisionStates.TargetModelState)
	}
	if inferenceService.Status.ModelStatus.LastFailureInfo != nil {
		p.Status.LastFailureInfo = &v1alpha1.FailureInfo{
			Location: inferenceService.Status.ModelStatus.LastFailureInfo.Location,
			Reason:   v1alpha1.FailureReason(inferenceService.Status.ModelStatus.LastFailureInfo.Reason),
			Message:  inferenceService.Status.ModelStatus.LastFailureInfo.Message,
			ModelId:  inferenceService.Status.ModelStatus.LastFailureInfo.ModelRevisionName,
			Time:     inferenceService.Status.ModelStatus.LastFailureInfo.Time,
		}
	}
	p.Status.Available = inferenceService.Status.IsConditionReady(v1beta1.PredictorReady)
	if componentStatus, ok := inferenceService.Status.Components[v1beta1.PredictorComponent]; ok {
		if componentStatus.GrpcURL != nil {
			p.Status.GrpcEndpoint = componentStatus.GrpcURL.String()
		}
		if componentStatus.RestURL != nil {
			p.Status.HTTPEndpoint = componentStatus.RestURL.String()
		}
	}

	if p.Status.ActiveModelState == "" {
		p.Status.ActiveModelState = v1alpha1.Pending
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
		inferenceService := &list.Items[i]
		nname := types.NamespacedName{Name: inferenceService.Name, Namespace: inferenceService.Namespace}
		p, err := BuildBasePredictorFromInferenceService(inferenceService)
		if err != nil {
			return true, nil
		}
		if p != nil {
			secretKey, parameters, modelPath, schemaPath, err := processInferenceServiceStorage(inferenceService, nname)
			if err != nil {
				return true, nil
			}
			p.Spec.Storage = &v1alpha1.Storage{}
			p.Spec.Storage.Path = &modelPath
			p.Spec.Storage.SchemaPath = schemaPath
			p.Spec.Storage.Parameters = &parameters
			p.Spec.Storage.StorageKey = secretKey

			if predicate(p) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (isvcr InferenceServiceRegistry) UpdateStatus(ctx context.Context, predictor *v1alpha1.Predictor) (bool, error) {
	inferenceService := &v1beta1.InferenceService{}

	inferenceService.ObjectMeta = predictor.ObjectMeta

	inferenceService.Status.ModelStatus.TransitionStatus = v1beta1.TransitionStatus(predictor.Status.TransitionStatus)
	inferenceService.Status.ModelStatus.ModelRevisionStates = &v1beta1.ModelRevisionStates{
		ActiveModelState: v1beta1.ModelState(predictor.Status.ActiveModelState),
		TargetModelState: v1beta1.ModelState(predictor.Status.TargetModelState),
	}
	inferenceService.Status.ModelStatus.ModelCopies = &v1beta1.ModelCopies{
		FailedCopies: predictor.Status.FailedCopies,
		TotalCopies:  predictor.Status.TotalCopies,
	}
	if predictor.Status.LastFailureInfo != nil {
		inferenceService.Status.ModelStatus.LastFailureInfo = &v1beta1.FailureInfo{
			Location:          predictor.Status.LastFailureInfo.Location,
			Reason:            v1beta1.FailureReason(predictor.Status.LastFailureInfo.Reason),
			Message:           predictor.Status.LastFailureInfo.Message,
			ModelRevisionName: predictor.Status.LastFailureInfo.ModelId,
			Time:              predictor.Status.LastFailureInfo.Time,
		}
	}

	if predictor.Status.Available {
		grpcUrl, err := apis.ParseURL(predictor.Status.GrpcEndpoint)
		if err != nil {
			return false, err
		}
		restUrl, err := apis.ParseURL(predictor.Status.HTTPEndpoint)
		if err != nil {
			return false, err
		}

		inferenceService.Status.URL = grpcUrl

		if inferenceService.Status.Components == nil {
			inferenceService.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
		}
		componentStatus, ok := inferenceService.Status.Components[v1beta1.PredictorComponent]
		if !ok {
			componentStatus = v1beta1.ComponentStatusSpec{}
		}
		componentStatus.URL = grpcUrl
		componentStatus.GrpcURL = grpcUrl
		componentStatus.RestURL = restUrl
		inferenceService.Status.Components[v1beta1.PredictorComponent] = componentStatus

		conditionSet.Manage(&inferenceService.Status).MarkTrue(v1beta1.PredictorReady)
	} else {
		conditionSet.Manage(&inferenceService.Status).MarkFalse(v1beta1.PredictorReady, "", "")
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
