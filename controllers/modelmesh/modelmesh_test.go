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

	"github.com/stretchr/testify/assert"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestAddMMDomainSocketMount(t *testing.T) {
	path := "unix:///var/mount/path"
	rt := &kserveapi.ServingRuntime{
		Spec: kserveapi.ServingRuntimeSpec{
			GrpcDataEndpoint: &path,
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

	m := Deployment{Owner: rt, SRSpec: &rt.Spec}
	err := m.addMMDomainSocketMount(deployment)
	if err != nil {
		t.Fatal(err)
	}

	createdVolumeMountName := deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name
	if createdVolumeMountName != "domain-socket" {
		t.Fatal(toString(deployment))
	}
}

func TestEnableAccessLogging(t *testing.T) {
	rt := &kserveapi.ServingRuntime{}
	d := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "mm",
					},
				},
			},
		},
	}}
	m := &Deployment{Owner: rt, EnableAccessLogging: true, SRSpec: &rt.Spec}
	m.addMMEnvVars(d)

	if _, c := findContainer("mm", d); c == nil {
		t.Fatal("Could not find the model mesh container")
	} else {
		for _, env := range c.Env {
			if env.Name == "MM_LOG_EACH_INVOKE" && env.Value == "true" {
				return
			}
		}
		t.Fatal("Expected to find an env variable MM_LOG_EACH_INVOKE but not found")
	}
}

func TestSetConfigMap(t *testing.T) {
	rt := &kserveapi.ServingRuntime{}
	m := Deployment{Owner: rt, SRSpec: &rt.Spec}

	err := m.setConfigMap()
	assert.Nil(t, err)
	assert.Nil(t, m.AnnotationConfigMap)
}

func TestModelMeshAdditionalEnvVars(t *testing.T) {
	rt := &kserveapi.ServingRuntime{}
	d := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "mm",
					},
				},
			},
		},
	}}
	m := &Deployment{Owner: rt, ModelMeshAdditionalEnvVars: []corev1.EnvVar{
		{Name: "ENV_VAR", Value: "0"},
	}, SRSpec: &rt.Spec}
	m.addMMEnvVars(d)

	if _, c := findContainer("mm", d); c == nil {
		t.Fatal("Could not find the model mesh container")
	} else {
		for _, env := range c.Env {
			if env.Name == "ENV_VAR" && env.Value == "0" {
				return
			}
		}
		t.Fatal("Expected to find an env variable ENV_VAR but not found")
	}
}
