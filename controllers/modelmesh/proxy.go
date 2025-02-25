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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	restProxyPortEnvVar           = "REST_PROXY_LISTEN_PORT"
	restProxyGrpcMaxMsgSizeEnvVar = "REST_PROXY_GRPC_MAX_MSG_SIZE_BYTES"
	restProxyGrpcPortEnvVar       = "REST_PROXY_GRPC_PORT"
	restProxyTlsEnvVar            = "REST_PROXY_USE_TLS"
	restProxySkipVerifyEnvVar     = "REST_PROXY_SKIP_VERIFY"
)

func (m *Deployment) addRESTProxyToDeployment(deployment *appsv1.Deployment) error {

	if m.RESTProxyEnabled {
		cspec := corev1.Container{
			Image: m.RESTProxyImage,
			Name:  RESTProxyContainerName,
			Env: []corev1.EnvVar{
				{
					Name:  restProxyPortEnvVar,
					Value: strconv.Itoa(int(m.RESTProxyPort)),
				}, {
					Name:  restProxyGrpcPortEnvVar,
					Value: strconv.Itoa(int(m.ServicePort)),
				}, {
					Name:  restProxyTlsEnvVar,
					Value: strconv.FormatBool(m.TLSSecretName != ""),
				}, {
					Name:  restProxyGrpcMaxMsgSizeEnvVar,
					Value: strconv.Itoa(m.GrpcMaxMessageSize),
				}, {
					Name:  restProxySkipVerifyEnvVar,
					Value: strconv.FormatBool(m.RESTProxySkipVerify),
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: int32(m.RESTProxyPort),
				},
			},
			Resources: *m.RESTProxyResources,
		}

		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, cspec)
	}
	return nil
}
