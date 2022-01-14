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
	"strings"
	"testing"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetServingRuntimeSupportedModelTypeLabelSet(t *testing.T) {
	version_semver := "12345.312.2"
	version_someString := "someString"
	autoSelectVal := true
	rt := api.ServingRuntime{
		ObjectMeta: v1.ObjectMeta{
			Name: "runtimename",
		},
		Spec: api.ServingRuntimeSpec{
			SupportedModelFormats: []api.SupportedModelFormat{
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
		},
	}

	expectedLabels := []string{
		"mt:type1",
		"mt:type2",
		"mt:type2:" + version_semver,
		"mt:type2:" + version_someString,
		//runtime
		"rt:runtimename",
	}

	labelSet := GetServingRuntimeSupportedModelTypeLabelSet(&rt)
	if len(labelSet) != len(expectedLabels) {
		t.Errorf("Length of set %v should be %d, but got %d", labelSet, len(expectedLabels), len(labelSet))
	}
	for _, e := range expectedLabels {
		if !labelSet.Contains(e) {
			t.Errorf("Missing expected entry [%s] in set: %v", e, labelSet)
		}
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
			label := GetPredictorModelTypeLabel(&p)
			if label != tt.expectedLabel {
				t.Errorf("Got wrong predictor label, expected [%s], got [%s]", tt.expectedLabel, label)
			}
		})
	}
}

func makeStringSet(input []string) StringSet {
	ss := make(StringSet, len(input))
	for _, s := range input {
		ss.Add(s)
	}
	return ss
}

func TestStringSetContains(t *testing.T) {
	inputs := []string{"cat", "frog", "aardvark", "aardvark", "aardvark"}
	ss := makeStringSet(inputs)

	// should contain these
	if !ss.Contains("cat") {
		t.Errorf("Missing expected entry [cat] in set: %v", ss)
	}
	if !ss.Contains("aardvark") {
		t.Errorf("Missing expected entry [aardvark] in set: %v", ss)
	}
	if !ss.Contains("frog") {
		t.Errorf("Missing expected entry [frog] in set: %v", ss)
	}
	// and not contain these
	if ss.Contains("billy goat") {
		t.Errorf("Unexpected entry [billy goat] in set: %v", ss)
	}
	if ss.Contains(".") {
		t.Errorf("Unexpected entry [.] in set: %v", ss)
	}
}

func TestStringSetToSlice(t *testing.T) {
	inputs := []string{"c", "f", "a", "a", "f", "a"}
	ss := makeStringSet(inputs)

	hasC := false
	hasF := false
	hasA := false

	ssSlice := ss.ToSlice()

	for i, s := range ssSlice {
		// each entry should only show up once
		if s == "c" && !hasC {
			hasC = true
			continue
		}
		if s == "f" && !hasF {
			hasF = true
			continue
		}
		if s == "a" && !hasA {
			hasA = true
			continue
		}
		t.Errorf("Unexpected entry in ToSlice at index %d: %v", i, ssSlice)
	}
	if !hasC || !hasA || !hasF {
		t.Errorf("Missing expected entry in ToSlice: %v", ssSlice)
	}

	// test that the order of entries in ToSlice is consistent
	expected := strings.Join(ss.ToSlice(), ",")
	for i := 1; i <= 20; i++ {
		ssNew := makeStringSet(inputs)
		got := strings.Join(ssNew.ToSlice(), ",")
		if got != expected {
			t.Fatalf("Expected order of ToSlice to result in %v but got %v", expected, got)
		}
	}
}
