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
	"errors"
	"strings"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	MMTypeConstraintsKey = "type_constraints"
	MMDataPlaneConfigKey = "dataplane_api_config"
)

func (m *Deployment) addModelTypeConstraints(deployment *appsv1.Deployment) error {
	rts := m.SRSpec
	var container *corev1.Container
	if _, container = findContainer(ModelMeshContainerName, deployment); container == nil {
		return errors.New("unable to find the model mesh container")
	}

	labelString := generateLabelsEnvVar(rts, m.RESTProxyEnabled, m.Name)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "MM_LABELS",
		Value: labelString,
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "MM_TYPE_CONSTRAINTS_PATH",
		Value: "/etc/watson/mmesh/config/" + MMTypeConstraintsKey,
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "MM_DATAPLANE_CONFIG_PATH",
		Value: "/etc/watson/mmesh/config/" + MMDataPlaneConfigKey,
	})
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      InternalConfigMapName,
		MountPath: "/etc/watson/mmesh/config",
	})

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: InternalConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: InternalConfigMapName,
				},
			},
		},
	})

	return nil
}

func generateLabelsEnvVar(rts *kserveapi.ServingRuntimeSpec, restProxyEnabled bool, rtName string) string {
	return strings.Join(GetServingRuntimeLabelSet(rts, restProxyEnabled, rtName).List(), ",")
}
