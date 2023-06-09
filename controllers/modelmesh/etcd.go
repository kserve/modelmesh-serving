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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	etcdMountPath = "/opt/kserve/mmesh/etcd"
	kvStoreEnvVar = "KV_STORE"
)

// mimics base/patches/etcd.yaml
func (m *Deployment) configureMMDeploymentForEtcdSecret(deployment *appsv1.Deployment) error {
	EtcdSecretName := m.EtcdSecretName

	for containerI, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == ModelMeshContainerName {
			for i, env := range container.Env {
				if env.Name == kvStoreEnvVar {
					env.Value = "etcd:" + etcdMountPath + "/" + EtcdSecretKey
				}
				container.Env[i] = env
			}

			volumeMountExists := false
			for i := range container.VolumeMounts {
				if container.VolumeMounts[i].Name == EtcdVolume {
					volumeMountExists = true
					container.VolumeMounts[i].ReadOnly = true
					container.VolumeMounts[i].MountPath = etcdMountPath
				}
			}
			if !volumeMountExists {
				container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
					Name:      EtcdVolume,
					ReadOnly:  true,
					MountPath: etcdMountPath,
				})
			}
			deployment.Spec.Template.Spec.Containers[containerI] = container
			break
		}
	}

	volumeExists := false
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == EtcdVolume {
			volumeExists = true
			volume.Secret = &corev1.SecretVolumeSource{
				SecretName: EtcdSecretName,
			}
		}
	}
	if !volumeExists {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: EtcdVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: EtcdSecretName,
				},
			},
		})
	}

	return nil
}
