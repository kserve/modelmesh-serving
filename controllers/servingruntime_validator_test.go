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

package controllers

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
)

func TestValidateServingRuntimeSpec(t *testing.T) {
	for _, tt := range []struct {
		name           string
		servingRuntime *kserveapi.ServingRuntime
		expectError    bool
	}{
		{
			name: "valid serving runtime",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "arbitrary-name",
								SecurityContext: &v1.SecurityContext{
									Capabilities: &v1.Capabilities{
										Add: []v1.Capability{"ALL"},
									},
								},
								Ports: []v1.ContainerPort{
									{
										Name:          "my-port",
										ContainerPort: 54321,
									},
								},
								VolumeMounts: []v1.VolumeMount{
									{
										Name: "my-volume",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							{
								Name: "my-volume",
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid serving runtime with adapter",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							// A container matching the name of the adapter ServerType must exist
							{
								Name: string(kserveapi.MLServer),
							},
						},
					},
					BuiltInAdapter: &kserveapi.BuiltInAdapter{
						ServerType: kserveapi.MLServer,
					},
				},
			},
			expectError: false,
		},
		{
			name: "block container name model mesh",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: modelmesh.ModelMeshContainerName,
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block container name reserved",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "mm-arbitrary",
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block container lifecycle",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name:      "bad-container",
								Lifecycle: &v1.Lifecycle{},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block container readiness probe",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name:           "bad-container",
								ReadinessProbe: &v1.Probe{},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block conflicting port name",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								Ports: []v1.ContainerPort{
									{
										Name:          "grpc",
										ContainerPort: 32768,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block reserved port name",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								Ports: []v1.ContainerPort{
									{
										Name:          "mm-arbitrary",
										ContainerPort: 32768,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block conflicting port",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								Ports: []v1.ContainerPort{
									{
										ContainerPort: 8080,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block reserved port",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								Ports: []v1.ContainerPort{
									{
										ContainerPort: 11888,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block mount of internal volume",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								VolumeMounts: []v1.VolumeMount{
									{
										Name: modelmesh.InternalConfigMapName,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block mount of reserved volume name",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
							{
								Name: "bad-container",
								VolumeMounts: []v1.VolumeMount{
									{
										Name: "mm-arbitrary",
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "block BuiltInAdapter missing runtime container",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Containers: []v1.Container{
							{
								Name: "some-container",
							},
						},
					},
					BuiltInAdapter: &kserveapi.BuiltInAdapter{
						ServerType: kserveapi.MLServer,
					},
				},
			},
			expectError: true,
		},
	} {
		cfg, err := getDefaultConfig()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		t.Run(tt.name, func(t *testing.T) {
			err := validateServingRuntimeSpec(&tt.servingRuntime.Spec, cfg)

			if tt.expectError && err == nil {
				t.Errorf("Expected an error, but didn't get one")

			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}

}
