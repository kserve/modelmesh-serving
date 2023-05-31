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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestPuller(t *testing.T) {
	portURI := "port:9103"
	rt := &kserveapi.ServingRuntime{
		Spec: kserveapi.ServingRuntimeSpec{
			GrpcMultiModelManagementEndpoint: &portURI,
		},
	}
	deployment := &appsv1.Deployment{}

	err := addPullerSidecar(&rt.Spec, deployment, "", nil, &corev1.ResourceRequirements{}, nil)
	if err != nil {
		t.Fatal(err)
	}
}
