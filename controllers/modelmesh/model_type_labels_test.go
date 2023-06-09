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
	"reflect"
	"sort"
	"testing"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/constants"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetServingRuntimeLabelSets(t *testing.T) {
	version_semver := "12345.312.2"
	version_someString := "someString"
	autoSelectVal := true
	rt := kserveapi.ServingRuntime{
		ObjectMeta: v1.ObjectMeta{
			Name: "runtimename",
		},
		Spec: kserveapi.ServingRuntimeSpec{
			SupportedModelFormats: []kserveapi.SupportedModelFormat{
				{
					Name:       "type1",
					AutoSelect: &autoSelectVal,
				},
				{
					Name:       "type2",
					Version:    &version_semver,
					AutoSelect: &autoSelectVal,
				},
				{
					Name:       "type2",
					Version:    &version_someString,
					AutoSelect: &autoSelectVal,
				},
				{
					Name: "type1",
				},
				{
					Name: "type3",
				},
			},
			ProtocolVersions: []constants.InferenceServiceProtocol{
				"v1", "v2",
			},
		},
	}

	expectedMtLabels := []string{
		"mt:type1",
		"mt:type2",
		"mt:type2:" + version_semver,
		"mt:type2:" + version_someString,
	}
	sort.Strings(expectedMtLabels)

	expectedPvLabels := []string{
		"pv:v1",
		"pv:v2",
	}
	sort.Strings(expectedPvLabels)

	expectedRtLabel := "rt:runtimename"

	mtLabelSet, pvLabelSet, rtLabel := GetServingRuntimeLabelSets(&rt.Spec, false, rt.Name)
	if expectedRtLabel != rtLabel {
		t.Errorf("Missing expected entry [%s] in set: %v", expectedRtLabel, rtLabel)
	}
	if !reflect.DeepEqual(mtLabelSet.List(), expectedMtLabels) {
		t.Errorf("Labels [%s] don't match expected: %v", mtLabelSet.List(), expectedMtLabels)
	}
	if !reflect.DeepEqual(pvLabelSet.List(), expectedPvLabels) {
		t.Errorf("Labels [%s] don't match expected: %v", pvLabelSet.List(), expectedPvLabels)
	}
}

func TestGetPredictorModelTypeLabel(t *testing.T) {
	version := "8.3"
	tableTests := []struct {
		name          string
		expectedLabel string
		spec          api.PredictorSpec
	}{
		{
			name:          "runtime ref",
			expectedLabel: "rt:target-runtime",
			spec: api.PredictorSpec{
				Runtime: &api.PredictorRuntime{
					RuntimeRef: &api.RuntimeRef{
						Name: "target-runtime",
					},
				},
			},
		},
		{
			name:          "model type",
			expectedLabel: "mt:type",
			spec: api.PredictorSpec{
				Model: api.Model{
					Type: api.ModelType{
						Name: "type",
					},
				},
			},
		},
		{
			name:          "model type with version",
			expectedLabel: "mt:type2:8.3",
			spec: api.PredictorSpec{
				Model: api.Model{
					Type: api.ModelType{
						Name:    "type2",
						Version: &version,
					},
				},
			},
		},
	}

	for _, tt := range tableTests {
		t.Run(tt.name, func(t *testing.T) {
			p := api.Predictor{
				Spec: tt.spec,
			}
			label := GetPredictorTypeLabel(&p)
			if label != tt.expectedLabel {
				t.Errorf("Got wrong predictor label, expected [%s], got [%s]", tt.expectedLabel, label)
			}
		})
	}
}
