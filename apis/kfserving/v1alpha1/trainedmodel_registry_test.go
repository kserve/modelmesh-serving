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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestProcessTrainedModelStorage(t *testing.T) {
	storageKey := "localMinIO"
	storagePath := "sklearn/mnist-svm.joblib"
	storageBucket := "modelmesh-example-models"

	var secretKey string
	var bucket string
	var modelPath string
	nname := types.NamespacedName{Name: "tm-test-model", Namespace: "modelmesh-serving"}
	trainedModel := &TrainedModel{
		Spec: TrainedModelSpec{
			Model: ModelSpec{
				Storage: &StorageSpec{
					StorageKey: &storageKey,
					Path:       &storagePath,
					Parameters: &map[string]string{
						"bucket": storageBucket,
					},
				},
			},
		},
	}
	err := ProcessTrainedModelStorage(&secretKey, &bucket, &modelPath, trainedModel, nname)
	if err != nil {
		fmt.Println(err)
	}
	expected := [3]string{secretKey, bucket, modelPath}
	result := [3]string{storageKey, storageBucket, storagePath}
	assert.Equal(t, result, expected)
}
