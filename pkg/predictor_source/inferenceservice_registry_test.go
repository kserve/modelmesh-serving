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

	"github.com/kserve/modelmesh-serving/apis/serving/common"
	"github.com/kserve/modelmesh-serving/apis/serving/v1beta1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestProcessInferenceServicelStorage(t *testing.T) {
	storageKey := "localMinIO"
	storagePath := "sklearn/mnist-svm.joblib"
	storageBucket := "modelmesh-example-models"
	storageSchemaPath := "graph/graph.lib"

	var secretKey string
	var bucket *string
	var modelPath string
	var schemaPath *string
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
	secretKey, bucket, modelPath, schemaPath, err := processInferenceServiceStorage(inferenceService, nname)
	if err != nil {
		fmt.Println(err)
	}
	expected := [4]string{secretKey, *bucket, modelPath, *schemaPath}
	result := [4]string{storageKey, storageBucket, storagePath, storageSchemaPath}
	assert.Equal(t, result, expected)
}
