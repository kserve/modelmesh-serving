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

	"sigs.k8s.io/yaml"
)

func TestMarshalPredictor(t *testing.T) {
	bucket := "bucket"
	_gpu := Required
	schemaPath := "schemaPath"
	v := Predictor{
		Spec: PredictorSpec{
			Model: Model{
				Path:       "path",
				SchemaPath: &schemaPath,
				Storage: &Storage{
					S3: &S3StorageSource{
						Bucket:    &bucket,
						SecretKey: "secretkey",
					},
				},
			},
			Gpu: &_gpu,
		},
	}

	b, err := yaml.Marshal(v)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}
