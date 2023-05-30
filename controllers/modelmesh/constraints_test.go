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

package modelmesh

import (
	"testing"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateLabel(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "test",
			expected: "mt:tensorflow,mt:tensorflow:1,rt:tf-serving-runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := "1"
			a := true
			rt := &kserveapi.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tf-serving-runtime",
				},
				Spec: kserveapi.ServingRuntimeSpec{
					SupportedModelFormats: []kserveapi.SupportedModelFormat{
						{
							Name:       "tensorflow",
							Version:    &v,
							AutoSelect: &a,
						},
					},
				},
			}

			labelString := generateLabelsEnvVar(&rt.Spec, false, rt.Name)
			if labelString != tt.expected {
				t.Fatalf("Expected label %v but found %v", tt.expected, labelString)
			}
		})
	}
}
