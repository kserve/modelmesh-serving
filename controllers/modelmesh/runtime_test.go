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

	"github.com/google/go-cmp/cmp"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestOverlayMockRuntime(t *testing.T) {
	version := "version"
	v := &api.ServingRuntime{
		Spec: api.ServingRuntimeSpec{
			ServingRuntimePodSpec: api.ServingRuntimePodSpec{
				Containers: []api.Container{
					{
						Name:            "mock-runtime",
						Image:           "image",
						ImagePullPolicy: "IfNotPresent",
						WorkingDir:      "mock-working-dir",
						Env: []corev1.EnvVar{
							{
								Name:  "simple",
								Value: "value",
							},
							{
								Name: "fromSecret",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{Key: "mykey"},
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
				},
			},
			SupportedModelFormats: []api.SupportedModelFormat{
				{
					Name:    "name",
					Version: &version,
				},
			},
		},
	}

	deployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "mm",
						},
					},
				},
			},
		},
	}

	m := Deployment{Owner: v}
	m.addRuntimeToDeployment(deployment)

	scontainer := v.Spec.Containers[0]
	tcontainer := deployment.Spec.Template.Spec.Containers[1]
	if tcontainer.Name != scontainer.Name {
		t.Fatal("The runtime should have added a container into the deployment")
	}
	if tcontainer.Image != scontainer.Image {
		t.Fatalf("Expected the added container image to be %v but it was %v", scontainer.Image, tcontainer.Image)
	}
	if !reflect.DeepEqual(tcontainer.Args, scontainer.Args) {
		t.Fatalf("Expected the added container args to be %v but it was %v", scontainer.Args, tcontainer.Args)
	}
	if !reflect.DeepEqual(tcontainer.Env, scontainer.Env) {
		t.Fatalf("Expected the env in target container to be \n%v but it was \n%v", toString(scontainer.Env), toString(tcontainer.Env))
	}
}

func toString(o interface{}) string {
	b, _ := yaml.Marshal(o)
	return string(b)
}

var addStorageConfigVolumeTests = []struct {
	name           string
	servingRuntime *api.ServingRuntime
	expectError    bool
	expectVolume   bool
}{
	{
		name:           "default",
		servingRuntime: &api.ServingRuntime{},
		expectError:    false,
		expectVolume:   true,
	},
	{
		name: "helper-disabled",
		servingRuntime: &api.ServingRuntime{
			Spec: api.ServingRuntimeSpec{
				StorageHelper: &api.StorageHelper{
					Disabled: true,
				},
			},
		},
		expectError:  false,
		expectVolume: false,
	},
}

func TestAddStorageConfigVolume(t *testing.T) {
	for _, tt := range addStorageConfigVolumeTests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{}
			rt := tt.servingRuntime

			m := Deployment{Owner: rt}
			err := m.addStorageConfigVolume(deployment)

			if tt.expectError && err == nil {
				t.Error("Expected an error, but didn't get one")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			numVols := len(deployment.Spec.Template.Spec.Volumes)
			var volNames []string
			for _, v := range deployment.Spec.Template.Spec.Volumes {
				volNames = append(volNames, v.Name)
			}
			if tt.expectVolume && numVols != 1 {
				t.Errorf("Expected a single volume but found %d: %s", numVols, volNames)
			}
			if !tt.expectVolume && numVols != 0 {
				t.Errorf("Unexpected volume(s) added to deployment: %s", volNames)
			}

		})
	}
}

func TestAddPassThroughPodFieldsToDeployment(t *testing.T) {
	t.Run("defaults-to-no-changes", func(t *testing.T) {
		d := &appsv1.Deployment{}
		sr := &api.ServingRuntime{}
		m := Deployment{Owner: sr}
		err := m.addPassThroughPodFieldsToDeployment(d)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// deployment should remain unchanged
		//emptyDeployment := appsv1.Deployment{}
		// 		if !cmp.Equal(*d, emptyDeployment) {
		// 			t.Error("Exepected no fields to be added to deployment")
		// 		}
	})

	t.Run("passes-through-fields", func(t *testing.T) {
		nodeSelector := map[string]string{
			"some-label": "some-label-value",
		}
		affinity := corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "some-node-label",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
		tolerations := []corev1.Toleration{
			{
				Key:      "taint-key",
				Operator: corev1.TolerationOpExists,
			},
		}

		sr := &api.ServingRuntime{
			Spec: api.ServingRuntimeSpec{
				ServingRuntimePodSpec: api.ServingRuntimePodSpec{
					NodeSelector: nodeSelector,
					Affinity:     &affinity,
					Tolerations:  tolerations,
				},
			},
		}

		m := Deployment{Owner: sr}
		d := &appsv1.Deployment{}
		err := m.addPassThroughPodFieldsToDeployment(d)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// deployment should remain unchanged
		expectedDeployment := appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						NodeSelector: nodeSelector,
						Affinity:     &affinity,
						Tolerations:  tolerations,
					},
				},
			},
		}
		if !cmp.Equal(*d, expectedDeployment) {
			t.Error("Configured Deployment did not contain expected pod template")
		}
	})
}
