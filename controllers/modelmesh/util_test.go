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
	"testing"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	unixEndpoint = "unix:///directory/socket.filename"
	httpEndpoint = "http://host/path"
)

var tests = []struct {
	name                             string
	GrpcDataEndpoint                 *string
	HTTPDataEndpoint                 *string
	MultiModelManagementGrpcEndpoint *string
	expected                         bool
	expectedEndpoints                []string
	expectedPaths                    []string
}{
	{
		name:                             "all nil",
		GrpcDataEndpoint:                 nil,
		HTTPDataEndpoint:                 nil,
		MultiModelManagementGrpcEndpoint: nil,
		expected:                         false,
	},
	{
		name:                             "mixed",
		GrpcDataEndpoint:                 &unixEndpoint,
		HTTPDataEndpoint:                 &httpEndpoint,
		MultiModelManagementGrpcEndpoint: nil,
		expected:                         true,
		expectedEndpoints:                []string{unixEndpoint},
		expectedPaths:                    []string{"/directory"},
	},
	{
		name:                             "http only",
		GrpcDataEndpoint:                 nil,
		HTTPDataEndpoint:                 &httpEndpoint,
		MultiModelManagementGrpcEndpoint: &httpEndpoint,
		expected:                         false,
	},
}

func TestIsUnix(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					GrpcDataEndpoint: tt.GrpcDataEndpoint,
					//HTTPDataEndpoint:                 tt.HTTPDataEndpoint,
					GrpcMultiModelManagementEndpoint: tt.MultiModelManagementGrpcEndpoint,
				},
			}
			result, endpoints, paths := unixDomainSockets(&rt.Spec)

			if result != tt.expected {
				t.Fatalf("Expected %v but result was %v", tt.expected, result)
			}
			if !reflect.DeepEqual(tt.expectedEndpoints, endpoints) {
				t.Fatalf("Expected endpoints %v but result was %v", tt.expectedEndpoints, endpoints)
			}
			if !reflect.DeepEqual(tt.expectedPaths, paths) {
				t.Fatalf("Expected paths %v but result was %v", tt.expectedPaths, paths)
			}
		})
	}
}

func TestSetEnvironmentVar(t *testing.T) {
	d := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "container-name",
						},
					},
				},
			},
		},
	}

	setEnvironmentVar("container-name", "variable", "value", d)

	if len(d.Spec.Template.Spec.Containers[0].Env) != 1 ||
		d.Spec.Template.Spec.Containers[0].Env[0].Name != "variable" ||
		d.Spec.Template.Spec.Containers[0].Env[0].Value != "value" {
		t.Error("Could not find the expected env var", d.Spec.Template.Spec.Containers[0].Env)
	}
}
