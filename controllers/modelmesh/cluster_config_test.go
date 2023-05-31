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
	"github.com/kserve/kserve/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateConstraintData(t *testing.T) {
	expected := `{"_default":{"required":["_no_runtime"]},` +
		`"mt:tensorflow":{"required":["mt:tensorflow"]},` +
		`"mt:tensorflow:1.10":{"required":["mt:tensorflow:1.10"]},` +
		`"mt:tensorflow:1.10|pv:v1":{"required":["mt:tensorflow:1.10","pv:v1"]},` +
		`"mt:tensorflow:1.10|pv:v2":{"required":["mt:tensorflow:1.10","pv:v2"]},` +
		`"mt:tensorflow|pv:v1":{"required":["mt:tensorflow","pv:v1"]},` +
		`"mt:tensorflow|pv:v2":{"required":["mt:tensorflow","pv:v2"]},` +
		`"rt:tf-serving-runtime":{"required":["rt:tf-serving-runtime"]}}`

	v := "1.10"
	v2 := "2"
	a := true
	mm := true
	l := kserveapi.ServingRuntimeList{
		Items: []kserveapi.ServingRuntime{
			{
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
						{
							Name:    "tensorflow",
							Version: &v2,
						},
					},
					ProtocolVersions: []constants.InferenceServiceProtocol{"v1", "v2"},
					MultiModel:       &mm,
				},
			},
		},
	}
	srSpecs := make(map[string]*kserveapi.ServingRuntimeSpec)
	srSpecs[l.Items[0].GetName()] = &l.Items[0].Spec

	res := calculateConstraintData(srSpecs, false)

	if string(res) != expected {
		t.Errorf("%v did not match expected %v", string(res), expected)
	}
}
