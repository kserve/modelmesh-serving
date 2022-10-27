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
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
)

func newMockModelMeshDeployment(t *testing.T, rt *kserveapi.ServingRuntime) *Deployment {
	return &Deployment{
		Owner:  rt,
		SRSpec: &rt.Spec,
		Log:    testr.New(t),

		PullerResources: &v1.ResourceRequirements{},
		// may need to add more as tests expand
	}

}

func TestOverlayMockRuntime(t *testing.T) {
	const adapterEnvOverrideName = "ADAPTER_PORT"
	const adapterEnvOverrideValue = "override"
	const adapterEnvNewName = "NEW_ENV_VAR"
	const adapterEnvNewValue = "some value"
	const adapterType = "custom"
	v := &kserveapi.ServingRuntime{
		Spec: kserveapi.ServingRuntimeSpec{
			ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
				Containers: []v1.Container{
					{
						Name:            adapterType,
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
			SupportedModelFormats: []kserveapi.SupportedModelFormat{
				{
					Name:    "name",
					Version: &[]string{"version"}[0],
				},
			},
			BuiltInAdapter: &kserveapi.BuiltInAdapter{
				ServerType:                adapterType,
				RuntimeManagementPort:     0,
				MemBufferBytes:            1337,
				ModelLoadingTimeoutMillis: 1000,
				Env: []v1.EnvVar{
					{
						Name:  adapterEnvOverrideName,
						Value: adapterEnvOverrideValue,
					},
					{
						Name:  adapterEnvNewName,
						Value: adapterEnvNewValue,
					},
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

	m := newMockModelMeshDeployment(t, v)
	m.addRuntimeToDeployment(deployment)

	scontainer := v.Spec.Containers[0]
	tcontainer := deployment.Spec.Template.Spec.Containers[1]
	if tcontainer.Name != scontainer.Name {
		t.Error("The runtime should have added a container into the deployment")
	}
	if tcontainer.Image != scontainer.Image {
		t.Errorf("Expected the added container image to be %v but it was %v", scontainer.Image, tcontainer.Image)
	}
	if !reflect.DeepEqual(tcontainer.Args, scontainer.Args) {
		t.Errorf("Expected the added container args to be %v but it was %v", scontainer.Args, tcontainer.Args)
	}
	if !reflect.DeepEqual(tcontainer.Env, scontainer.Env) {
		t.Errorf("Expected the env in target container to be \n%v but it was \n%v", toString(scontainer.Env), toString(tcontainer.Env))
	}

	// check the injected adapter
	if len(deployment.Spec.Template.Spec.Containers) != 3 {
		t.Fatalf("Expected 3 containers to be be added, but got \n%v", len(deployment.Spec.Template.Spec.Containers))
	}

	acontainer := deployment.Spec.Template.Spec.Containers[2]
	expectedAdapterName := fmt.Sprintf("%s-adapter", adapterType)
	if acontainer.Name != expectedAdapterName {
		t.Errorf("Expected the adapter container name to be %v but it was %v", expectedAdapterName, acontainer.Name)
	}

	for _, env := range acontainer.Env {
		if env.Name == adapterEnvOverrideName {
			if env.Value != adapterEnvOverrideValue {
				t.Errorf("Expected the env var %s in adapter container to be \"%s\" but it was \"%s\"", adapterEnvOverrideName, adapterEnvOverrideValue, env.Value)
			}
		}
		if env.Name == adapterEnvNewName {
			if env.Value != adapterEnvNewValue {
				t.Errorf("Expected the env var %s in adapter container to be \"%s\" but it was \"%s\"", adapterEnvNewName, adapterEnvNewValue, env.Value)
			}
		}
	}
}

func toString(o interface{}) string {
	b, _ := yaml.Marshal(o)
	return string(b)
}

func TestAddVolumesToDeployment(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		servingRuntime       *kserveapi.ServingRuntime
		expectedExtraVolumes []string
		expectStorageVolumes bool
		expectSocketVolume   bool
	}{
		{
			name: "with-volume",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
						Volumes: []v1.Volume{
							{
								Name: "my-volume",
							},
						},
					},
				},
			},
			expectedExtraVolumes: []string{"my-volume"},
			expectStorageVolumes: true,
			expectSocketVolume:   false,
		},
		{
			name: "unix-socket-grpc",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					GrpcDataEndpoint: &[]string{"unix:///socket"}[0],
				},
			},
			expectStorageVolumes: true,
			expectSocketVolume:   true,
		},
		{
			name: "built-in-adapter",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					BuiltInAdapter: &kserveapi.BuiltInAdapter{
						ServerType: kserveapi.MLServer,
					},
				},
			},
			expectStorageVolumes: true,
			expectSocketVolume:   false,
		},
		{
			name: "helper-disabled",
			servingRuntime: &kserveapi.ServingRuntime{
				Spec: kserveapi.ServingRuntimeSpec{
					StorageHelper: &kserveapi.StorageHelper{
						Disabled: true,
					},
				},
			},
			expectStorageVolumes: false,
			expectSocketVolume:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{}
			rt := tt.servingRuntime

			m := Deployment{Owner: rt, SRSpec: &rt.Spec}
			if err := m.addVolumesToDeployment(deployment); err != nil {
				t.Errorf("Call to add volumes failed: %v", err)
			}

			// map of expected volume names to bool of whether or not it is found
			expectedVolumes := map[string]bool{
				// models dir is always mounted
				ModelsDirVolume: false,
			}
			for _, v := range tt.expectedExtraVolumes {
				expectedVolumes[v] = false
			}
			if tt.expectStorageVolumes {
				expectedVolumes[ConfigStorageMount] = false
			}
			if tt.expectSocketVolume {
				expectedVolumes[SocketVolume] = false
			}

			for _, v := range deployment.Spec.Template.Spec.Volumes {
				if _, ok := expectedVolumes[v.Name]; !ok {
					t.Errorf("Unexpected volume found: %s", v.Name)
				} else if expectedVolumes[v.Name] {
					t.Errorf("Duplicate volume found: %s", v.Name)
				} else {
					expectedVolumes[v.Name] = true
				}
			}

			for volumeName, volumeFound := range expectedVolumes {
				if !volumeFound {
					t.Errorf("Expected to find volume that does not exist: %s", volumeName)
				}
			}
		})
	}
}

func TestAddPassThroughPodFieldsToDeployment(t *testing.T) {
	t.Run("defaults-to-no-changes", func(t *testing.T) {
		d := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{}
		m := Deployment{Owner: sr, SRSpec: &sr.Spec}
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

		sr := &kserveapi.ServingRuntime{
			Spec: kserveapi.ServingRuntimeSpec{
				ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
					NodeSelector: nodeSelector,
					Affinity:     &affinity,
					Tolerations:  tolerations,
				},
			},
		}

		m := Deployment{Owner: sr, SRSpec: &sr.Spec}
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

func TestConfigureRuntimeAnnotations(t *testing.T) {
	t.Run("success-set-annotations", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{}
		annotationsData := map[string]string{
			"foo":            "bar",
			"network-policy": "allow-egress",
		}

		m := Deployment{Owner: sr, AnnotationsMap: annotationsData, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecAnnotations(deploy)
		assert.Nil(t, err)
		// assert.Equal(t, deploy.Spec.Template.Labels, labelData)
		assert.Equal(t, deploy.Spec.Template.Annotations["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Annotations["network-policy"], "allow-egress")
	})

	t.Run("success-no-annotations", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{}
		m := Deployment{Owner: sr, AnnotationsMap: map[string]string{}, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecAnnotations(deploy)
		assert.Nil(t, err)
		assert.Empty(t, deploy.Spec.Template.Annotations)
	})

	t.Run("success-set-annotations-from-servingruntime-spec", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{
			Spec: kserveapi.ServingRuntimeSpec{
				ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
					Annotations: map[string]string{
						"foo":            "bar",
						"network-policy": "allow-egress",
					},
				},
			},
		}

		m := Deployment{Owner: sr, AnnotationsMap: map[string]string{}, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecAnnotations(deploy)
		assert.Nil(t, err)
		assert.Equal(t, deploy.Spec.Template.Annotations["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Annotations["network-policy"], "allow-egress")
	})

	t.Run("success-overwrite-annotations-from-servingruntime-spec", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		// annotations from user config
		annotationsData := map[string]string{
			"foo":            "bar",
			"network-policy": "allow-egress",
		}
		sr := &kserveapi.ServingRuntime{
			Spec: kserveapi.ServingRuntimeSpec{
				ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
					Annotations: map[string]string{
						"network-policy": "overwritten-by-servingruntime",
					},
				},
			},
		}

		m := Deployment{Owner: sr, AnnotationsMap: annotationsData, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecAnnotations(deploy)
		assert.Nil(t, err)
		assert.Equal(t, deploy.Spec.Template.Annotations["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Annotations["network-policy"], "overwritten-by-servingruntime")
	})
}

func TestConfigureRuntimeLabels(t *testing.T) {

	t.Run("success-set-labels", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{}
		labelData := map[string]string{
			"foo":            "bar",
			"network-policy": "allow-egress",
			"cp4s-internet":  "allow",
		}

		m := Deployment{Owner: sr, LabelsMap: labelData, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecLabels(deploy)
		assert.Nil(t, err)
		// assert.Equal(t, deploy.Spec.Template.Labels, labelData)
		assert.Equal(t, deploy.Spec.Template.Labels["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Labels["network-policy"], "allow-egress")
		assert.Equal(t, deploy.Spec.Template.Labels["cp4s-internet"], "allow")
	})

	t.Run("success-no-labels", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{}
		m := Deployment{Owner: sr, LabelsMap: map[string]string{}, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecLabels(deploy)
		assert.Nil(t, err)
		assert.Empty(t, deploy.Spec.Template.Labels)
	})

	t.Run("success-set-labels-from-servingruntime-spec", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		sr := &kserveapi.ServingRuntime{
			Spec: kserveapi.ServingRuntimeSpec{
				ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
					Labels: map[string]string{
						"foo":            "bar",
						"network-policy": "allow-egress",
						"cp4s-internet":  "allow",
					},
				},
			},
		}

		m := Deployment{Owner: sr, LabelsMap: map[string]string{}, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecLabels(deploy)
		assert.Nil(t, err)
		assert.Equal(t, deploy.Spec.Template.Labels["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Labels["network-policy"], "allow-egress")
		assert.Equal(t, deploy.Spec.Template.Labels["cp4s-internet"], "allow")
	})

	t.Run("success-overwrite-labels-from-servingruntime-spec", func(t *testing.T) {
		deploy := &appsv1.Deployment{}
		// labels from user config
		labelData := map[string]string{
			"foo":            "bar",
			"network-policy": "allow-egress",
			"cp4s-internet":  "allow",
		}
		sr := &kserveapi.ServingRuntime{
			Spec: kserveapi.ServingRuntimeSpec{
				ServingRuntimePodSpec: kserveapi.ServingRuntimePodSpec{
					Labels: map[string]string{
						"network-policy": "overwritten-by-servingruntime",
					},
				},
			},
		}

		m := Deployment{Owner: sr, LabelsMap: labelData, SRSpec: &sr.Spec}

		err := m.configureRuntimePodSpecLabels(deploy)
		assert.Nil(t, err)
		assert.Equal(t, deploy.Spec.Template.Labels["foo"], "bar")
		assert.Equal(t, deploy.Spec.Template.Labels["network-policy"], "overwritten-by-servingruntime")
		assert.Equal(t, deploy.Spec.Template.Labels["cp4s-internet"], "allow")
	})
}
