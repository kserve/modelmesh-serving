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

package config

import (
	"bytes"
	"reflect"
	"testing"

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var tests = []struct {
	name    string
	base    string
	overlay string
	output  string
}{
	{"replace environment variable",
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    spec:
      containers:
        - name: container1
          env:
            - name: KEY
              value: original_value
            - name: OTHERKEY
              value: other_value
        - name: container2`,
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    spec:
      containers:
        - name: container1
          env:
            - name: KEY
              value: new_value`,
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    spec:
      containers:
        - name: container1
          env:
            - name: KEY
              value: new_value
            - name: OTHERKEY
              value: other_value
        - name: container2`,
	},
	{"merge annotations",
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    spec:
      containers:
        - name: container1`,
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    metadata:
      annotations:
        prometheus.io/path: /metrics
        prometheus.io/port: "2112"
        prometheus.io/scheme: https
        prometheus.io/scrape: "true"`,
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: mydeployment
spec:
  template:
    metadata:
      annotations:
        prometheus.io/path: /metrics
        prometheus.io/port: "2112"
        prometheus.io/scheme: https
        prometheus.io/scrape: "true"
    spec:
      containers:
        - name: container1`},
}

func parse(t *testing.T, s string) *unstructured.Unstructured {
	m, _ := mf.ManifestFrom(mf.Reader(bytes.NewReader([]byte(s))))
	resources := m.Resources()
	if len(resources) != 1 {
		t.Fatal("No k8s resources found for ", s)
	}
	return &resources[0]
}

func TestOverlay(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseRes := parse(t, tt.base)
			overlayRes := parse(t, tt.overlay)
			err := overlay(baseRes, overlayRes)
			if err != nil {
				t.Fatal(err)
			}

			/* to dump the output:
			fmt.Println("Processed")
			b, err := yaml.Marshal(baseRes)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println(string(b))
			*/

			expectedRes := parse(t, tt.output)
			if !reflect.DeepEqual(baseRes, expectedRes) {
				b, _ := yaml.Marshal(baseRes)
				result := string(b)
				b, _ = yaml.Marshal(expectedRes)
				expected := string(b)

				_ = result
				_ = expected

				t.Fatal("Test failed")

				t.Fatalf("Output does not match expectation:\nHave:\n%sExpect:\n%s", result, expected)
			}
		})
	}
}
