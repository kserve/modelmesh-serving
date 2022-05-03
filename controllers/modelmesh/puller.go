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
	"strconv"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

var StorageSecretName string

func addPullerTransform(rt *kserveapi.ServingRuntime, pullerImage string, pullerImageCommand []string, pullerResources *corev1.ResourceRequirements) func(*unstructured.Unstructured) error {
	return func(resource *unstructured.Unstructured) error {
		var deployment = &appsv1.Deployment{}
		if err := scheme.Scheme.Convert(resource, deployment, nil); err != nil {
			return err
		}

		err := addPullerSidecar(rt, deployment, pullerImage, pullerImageCommand, pullerResources)
		if err != nil {
			return err
		}

		return scheme.Scheme.Convert(deployment, resource, nil)
	}
}

func addPullerSidecar(rt *kserveapi.ServingRuntime, deployment *appsv1.Deployment, pullerImage string, pullerImageCommand []string, pullerResources *corev1.ResourceRequirements) error {
	endpoint, err := ValidateEndpoint(*rt.Spec.GrpcMultiModelManagementEndpoint)
	if err != nil {
		return err
	}
	e, _ := ParseEndpoint(endpoint)
	var udsParentPath string
	requiresUdsVolMount := false
	if udsE, ok := e.(UnixEndpoint); ok {
		udsParentPath = udsE.ParentPath
		requiresUdsVolMount = true
	}
	cspec := corev1.Container{
		Env: []corev1.EnvVar{
			{
				Name:  PullerEnvListenPort,
				Value: strconv.Itoa(PullerPortNumber),
			}, {
				Name:  PullerEnvModelServerEndpoint,
				Value: endpoint,
			}, {
				Name:  PullerEnvModelDir,
				Value: PullerModelPath,
			}, {
				Name:  PullerEnvStorageConfigDir,
				Value: PullerConfigPath,
			},
		},
		Image:   pullerImage,
		Name:    PullerContainerName,
		Command: pullerImageCommand,
		Ports: []corev1.ContainerPort{
			{
				Name:          "grpc",
				ContainerPort: PullerPortNumber,
			},
		},
		Resources: *pullerResources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ModelsDirVolume,
				MountPath: PullerModelPath,
			},
			{
				Name:      ConfigStorageMount,
				MountPath: PullerConfigPath,
				ReadOnly:  true,
			},
		},
	}

	if requiresUdsVolMount {
		cspec.VolumeMounts = append(cspec.VolumeMounts, corev1.VolumeMount{
			Name:      udsVolMountName,
			MountPath: udsParentPath,
		})
	}

	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, cspec)

	return nil
}
