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
	"testing"

	"github.com/kserve/modelmesh-serving/apis/serving/common"
	"github.com/kserve/modelmesh-serving/apis/serving/v1beta1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestProcessInferenceServiceStorage_Simple(t *testing.T) {
	storageKey := "localMinIO"
	storagePath := "sklearn/mnist-svm.joblib"
	storageBucket := "modelmesh-example-models"
	storageSchemaPath := "graph/graph.lib"

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.InferenceServicePredictorSpec{
				SKLearn: &v1beta1.PredictorExtensionSpec{
					Storage: &common.StorageSpec{
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
	}
	secretKey, parameters, modelPath, schemaPath, err := processInferenceServiceStorage(inferenceService, nname)
	assert.NoError(t, err)

	expected := [4]string{*secretKey, parameters["bucket"], modelPath, *schemaPath}
	result := [4]string{storageKey, storageBucket, storagePath, storageSchemaPath}
	assert.Equal(t, result, expected)
}

func TestProcessInferenceServiceStorage_S3UriProcessing(t *testing.T) {
	uriBucket := "uri-bucket"
	uriPath := "/some/path/in/the/uri"
	uri := "s3://" + uriBucket + uriPath

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.InferenceServicePredictorSpec{
				SKLearn: &v1beta1.PredictorExtensionSpec{
					StorageURI: &uri,
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

func TestProcessInferenceServiceStorage_OverlappingParameters(t *testing.T) {
	storageBucket := "storage-parameters-bucket"
	uriBucket := "uri-bucket"

	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	inferenceService := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.InferenceServicePredictorSpec{
				SKLearn: &v1beta1.PredictorExtensionSpec{
					StorageURI: strRef("s3://" + uriBucket + "/test-path"),
					Storage: &common.StorageSpec{
						Parameters: &map[string]string{
							"type":   "override_me",
							"bucket": storageBucket,
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
			Predictor: v1beta1.InferenceServicePredictorSpec{
				SKLearn: &v1beta1.PredictorExtensionSpec{
					StorageURI: strRef("s3://test-bucket/test-path"),
					Storage: &common.StorageSpec{
						Path: strRef("some-other-path"),
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
