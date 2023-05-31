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
	"fmt"
	"testing"

	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveConstants "github.com/kserve/kserve/pkg/constants"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildBasePredictorFromInferenceService_ModelSpecSimple(t *testing.T) {
	formatName := "tensorflow"
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
			},
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: formatName,
					},
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.NoError(t, err)
	assert.Equal(t, formatName, predictor.Spec.Model.Type.Name)
	assert.Nil(t, predictor.Spec.Model.Type.Version)
	assert.Nil(t, predictor.Spec.Runtime)
}

func TestBuildBasePredictorFromInferenceService_ModelSpecRuntime(t *testing.T) {
	formatName := "pytorch"
	formatVersion := "1"
	runtimeName := "triton-x"
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
			},
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name:    formatName,
						Version: &formatVersion,
					},
					Runtime: &runtimeName,
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.NoError(t, err)
	assert.Equal(t, formatName, predictor.Spec.Model.Type.Name)
	if assert.NotNil(t, predictor.Spec.Model.Type.Version) {
		assert.Equal(t, formatVersion, *predictor.Spec.Model.Type.Version)
	}
	if assert.NotNil(t, predictor.Spec.Runtime) {
		assert.Equal(t, runtimeName, predictor.Spec.Runtime.Name)
	}
}

func TestBuildBasePredictorFromInferenceService_FrameworkSpec(t *testing.T) {
	uri := "s3://foo/bar"
	runtimeName := "triton-x"
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
				runtimeAnnotation:              runtimeName,
			},
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: &uri,
					},
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.NoError(t, err)
	assert.Equal(t, "sklearn", predictor.Spec.Model.Type.Name)
	assert.Nil(t, predictor.Spec.Model.Type.Version)
	if assert.NotNil(t, predictor.Spec.Runtime) {
		assert.Equal(t, runtimeName, predictor.Spec.Runtime.Name)
	}
}

func TestBuildBasePredictorFromInferenceService_InvalidSpec(t *testing.T) {
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
			},
		},
		Spec: v1beta1.InferenceServiceSpec{},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.Error(t, err)
	assert.Nil(t, predictor)
}

func TestBuildBasePredictorFromInferenceService_NonMM(t *testing.T) {
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "foo",
					},
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.NoError(t, err)
	assert.Nil(t, predictor)
}

func TestBuildBasePredictorFromInferenceService_BothSpecs(t *testing.T) {
	uri := "s3://foo/bar"
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
			},
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "foo",
					},
				},
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: &uri,
					},
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot have both the model spec and a framework")
	assert.Nil(t, predictor)
}

func TestBuildBasePredictorFromInferenceService_AnnotationWithModelSpec(t *testing.T) {
	runtimeName := "mlserver-x"
	inferenceService := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kserveConstants.DeploymentMode: string(kserveConstants.ModelMeshDeployment),
				runtimeAnnotation:              runtimeName,
			},
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "foo",
					},
				},
			},
		},
	}
	predictor, err := BuildBasePredictorFromInferenceService(inferenceService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot have both the model spec and the runtime annotation")
	assert.Nil(t, predictor)
}

func TestProcessInferenceServiceStorage_Simple(t *testing.T) {
	storageKey := "localMinIO"
	storagePath := "sklearn/mnist-svm.joblib"
	storageBucket := "modelmesh-example-models"
	storageSchemaPath := "graph/graph.lib"

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						Storage: &v1beta1.StorageSpec{
							StorageKey: &storageKey,
							Path:       &storagePath,
							SchemaPath: &storageSchemaPath,
							Parameters: &map[string]string{
								"bucket": storageBucket,
							},
						},
					},
				},
			},
		},
	}
	secretKey, parameters, modelPath, schemaPath, err := processInferenceServiceStorage(inferenceService, nname)
	assert.NoError(t, err)

	expected := [4]string{*secretKey, parameters["bucket"], modelPath, *schemaPath}
	result := [4]string{storageKey, storageBucket, storagePath, storageSchemaPath}
	assert.Equal(t, result, expected)
}

func TestProcessInferenceServiceStorage_S3UriProcessing(t *testing.T) {
	uriBucket := "uri-bucket"
	uriPath := "some/path/in/the/uri"
	uri := "s3://" + uriBucket + "/" + uriPath

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: &uri,
					},
				},
			},
		},
	}

	_, parameters, modelPath, _, err := processInferenceServiceStorage(inferenceService, nname)
	assert.NoError(t, err)
	assert.Equal(t, uriPath, modelPath)
	assert.Equal(t, uriBucket, parameters["bucket"])
	assert.Equal(t, "s3", parameters["type"])
}

func TestProcessInferenceServiceStorage_AzureUriProcessing(t *testing.T) {
	uriAccount := "az-account"
	uriContainer := "az-container"
	uriPath := "some/path/in/the/uri"
	uri := fmt.Sprintf("https://%s.%s/%s/%s", uriAccount, azureBlobHostSuffix, uriContainer, uriPath)

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: &uri,
					},
				},
			},
		},
	}

	_, parameters, modelPath, _, err := processInferenceServiceStorage(inferenceService, nname)
	assert.NoError(t, err)
	assert.Equal(t, uriPath, modelPath)
	assert.Equal(t, uriAccount, parameters["account_name"])
	assert.Equal(t, uriContainer, parameters["container"])
	assert.Equal(t, "azure", parameters["type"])
}

func TestProcessInferenceServiceStorage_OverlappingParameters(t *testing.T) {
	storageBucket := "storage-parameters-bucket"
	uriBucket := "uri-bucket"

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: strRef("s3://" + uriBucket + "/test-path"),
						Storage: &v1beta1.StorageSpec{
							Parameters: &map[string]string{
								"type":   "override_me",
								"bucket": storageBucket,
							},
						},
					},
				},
			},
		},
	}
	_, parameters, _, _, err := processInferenceServiceStorage(inferenceService, nname)
	assert.NoError(t, err)
	// parameters from URI take precedence
	assert.Equal(t, uriBucket, parameters["bucket"])
	assert.Equal(t, "s3", parameters["type"])
}

func TestProcessInferenceServiceStorage_ErrorUriAndPath(t *testing.T) {
	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				SKLearn: &v1beta1.SKLearnSpec{
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageURI: strRef("s3://test-bucket/test-path"),
						Storage: &v1beta1.StorageSpec{
							Path: strRef("some-other-path"),
						},
					},
				},
			},
		},
	}

	_, _, _, _, err := processInferenceServiceStorage(inferenceService, nname)
	assert.Error(t, err)
}

func strRef(s string) *string {
	return &s
}
