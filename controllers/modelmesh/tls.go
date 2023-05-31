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
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TLSSecretCertKey    = "tls.crt"
	TLSSecretKeyKey     = "tls.key"
	tlsSecretVolume     = "tls-certs"
	tlsSecretMountPath  = "/opt/kserve/mmesh/tls"
	TLSClientCertKey    = "ca.crt"
	tlsCertEnvVar       = "MM_TLS_KEY_CERT_PATH"
	tlsKeyEnvVar        = "MM_TLS_PRIVATE_KEY_PATH"
	tlsClientAuthEnvVar = "MM_TLS_CLIENT_AUTH"
	tlsClientCertEnvVar = "MM_TLS_TRUST_CERT_PATH"
)

// NOTE: the inspected contents of the Secret might not match what gets mounted
// if the Secret is updated. Currently not triggering a reconcile when Secret changes.
// TODO: react to tls secret content changes - (#611)
func (m *Deployment) configureMMDeploymentForTLSSecret(deployment *appsv1.Deployment) error {
	clientParam := m.Client

	var tlsSecret *corev1.Secret = nil
	if m.TLSSecretName != "" {
		tlsSecret = &corev1.Secret{}
		tlsSecretErr := clientParam.Get(context.TODO(), client.ObjectKey{
			Name:      m.TLSSecretName,
			Namespace: m.Namespace}, tlsSecret)
		if tlsSecretErr != nil {
			m.Log.Error(tlsSecretErr, "Unable to access TLS secret", "secretName", m.TLSSecretName)
			return fmt.Errorf("unable to access TLS secret '%s': %v", m.TLSSecretName, tlsSecretErr)
		}
	}

	if tlsSecret == nil {
		return nil // TLS disabled
	}

	clientAuth := strings.TrimSpace(m.TLSClientAuth)
	_, clientCertExists := tlsSecret.Data[TLSClientCertKey]

	podSpec := &deployment.Spec.Template.Spec
	for ci := range podSpec.Containers {
		container := &podSpec.Containers[ci]
		if container.Name == ModelMeshContainerName || (m.RESTProxyEnabled && container.Name == RESTProxyContainerName) {
			container.Env = append(container.Env, corev1.EnvVar{
				Name: tlsCertEnvVar, Value: tlsSecretMountPath + "/" + TLSSecretCertKey,
			})
			container.Env = append(container.Env, corev1.EnvVar{
				Name: tlsKeyEnvVar, Value: tlsSecretMountPath + "/" + TLSSecretKeyKey,
			})

			if clientAuth != "" {
				container.Env = append(container.Env, corev1.EnvVar{
					Name: tlsClientAuthEnvVar, Value: clientAuth,
				})

				tlsClientCerts := tlsSecretMountPath + "/" + TLSSecretCertKey
				if clientCertExists {
					tlsClientCerts = tlsClientCerts + "," + tlsSecretMountPath + "/" + TLSClientCertKey
				}
				container.Env = append(container.Env, corev1.EnvVar{
					Name: tlsClientCertEnvVar, Value: tlsClientCerts,
				})
			}

			var vm *corev1.VolumeMount
			for vmi := range container.VolumeMounts {
				if container.VolumeMounts[vmi].Name == tlsSecretVolume {
					vm = &container.VolumeMounts[vmi]
					break
				}
			}
			if vm == nil {
				container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: tlsSecretVolume})
				vm = &container.VolumeMounts[len(container.VolumeMounts)-1]
			}
			vm.ReadOnly = true
			vm.MountPath = tlsSecretMountPath
		}
	}

	var v *corev1.Volume
	for vi := range podSpec.Volumes {
		if podSpec.Volumes[vi].Name == tlsSecretVolume {
			v = &podSpec.Volumes[vi]
			break
		}
	}
	if v == nil {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{Name: tlsSecretVolume})
		v = &podSpec.Volumes[len(podSpec.Volumes)-1]
	}
	v.VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName: tlsSecret.Name,
		},
	}

	return nil

}
