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
	"strconv"

	"github.com/go-logr/logr"
	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/controllers/config"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const logYaml = false

const ModelMeshEtcdPrefix = "mm"

// Models a deployment
type Deployment struct {
	ServiceName        string
	ServicePort        uint16
	Name               string
	Namespace          string
	Owner              mf.Owner
	SRSpec             *kserveapi.ServingRuntimeSpec
	DefaultVModelOwner string
	Log                logr.Logger
	Metrics            bool
	PrometheusPort     uint16
	PrometheusScheme   string
	PayloadProcessors  string
	ModelMeshImage     string
	ModelMeshResources *corev1.ResourceRequirements
	RESTProxyEnabled   bool
	RESTProxyImage     string
	RESTProxyResources *corev1.ResourceRequirements
	RESTProxyPort      uint16
	PVCs               []string
	// internal fields used when templating
	ModelMeshLimitCPU          string
	ModelMeshRequestsCPU       string
	ModelMeshLimitMemory       string
	ModelMeshRequestsMemory    string
	ModelMeshAdditionalEnvVars []corev1.EnvVar
	// end internal fields
	PullerImage         string
	PullerImageCommand  []string
	PullerResources     *corev1.ResourceRequirements
	Replicas            int32
	Port                uint16
	TLSSecretName       string
	TLSClientAuth       string
	EtcdSecretName      string
	ServiceAccountName  string
	GrpcMaxMessageSize  int
	AnnotationConfigMap *corev1.ConfigMap
	AnnotationsMap      map[string]string
	LabelsMap           map[string]string
	ImagePullSecrets    []corev1.LocalObjectReference
	EnableAccessLogging bool
	Client              client.Client
}

func (m *Deployment) Apply(ctx context.Context) error {
	clientParam := m.Client

	m.Log.Info("Applying model mesh deployment", "pods", m.Replicas)

	// set internal fields before rendering from the template
	m.ModelMeshLimitCPU = m.ModelMeshResources.Limits.Cpu().String()
	m.ModelMeshLimitMemory = m.ModelMeshResources.Limits.Memory().String()
	m.ModelMeshRequestsCPU = m.ModelMeshResources.Requests.Cpu().String()
	m.ModelMeshRequestsMemory = m.ModelMeshResources.Requests.Memory().String()

	manifest, err := config.Manifest(clientParam, "config/internal/base/deployment.yaml.tmpl", m)
	if err != nil {
		return fmt.Errorf("Error loading model mesh deployment yaml: %w", err)
	}

	if len(manifest.Resources()) != 1 {
		// manifestival.ManifestFrom will hide yaml parsing errors and not include those resources. This check ensures we parsed the proper number of resources.
		return fmt.Errorf("Unexpected number of resources (%d) found in the deployment template. This is likely due to bad or missing config which caused a hidden yaml parsing error.", len(manifest.Resources()))
	}

	configMapErr := m.setConfigMap()
	if configMapErr != nil {
		return configMapErr
	}

	manifest, err = manifest.Transform(
		mf.InjectOwner(m.Owner),
		mf.InjectNamespace(m.Namespace),
		func(resource *unstructured.Unstructured) error {
			var deployment = &appsv1.Deployment{}
			if tErr := scheme.Scheme.Convert(resource, deployment, nil); tErr != nil {
				return tErr
			}

			if tErr := m.transform(deployment,
				m.addVolumesToDeployment,
				m.addMMDomainSocketMount,
				m.addPassThroughPodFieldsToDeployment,
				m.addRuntimeToDeployment,
				m.syncGracePeriod,
				m.addMMEnvVars,
				m.addModelTypeConstraints,
				m.configureMMDeploymentForEtcdSecret,
				m.addRESTProxyToDeployment,
				m.configureMMDeploymentForTLSSecret,
				m.configureRuntimePodSpecAnnotations,
				m.configureRuntimePodSpecLabels,
				m.ensureMMContainerIsLast,
				m.configureRuntimePodSpecImagePullSecrets,
			); tErr != nil {
				return tErr
			}

			return scheme.Scheme.Convert(deployment, resource, nil)
		},
	)
	if err != nil {
		return fmt.Errorf("Error transforming: %w", err)
	}

	if useStorageHelper(m.SRSpec) {
		manifest, err = manifest.Transform(
			addPullerTransform(m.SRSpec, m.PullerImage, m.PullerImageCommand, m.PullerResources, m.PVCs),
		)
		if err != nil {
			return fmt.Errorf("Error transforming: %w", err)
		}
	}

	if logYaml {
		b, _ := yaml.Marshal(manifest.Resources())
		m.Log.Info(string(b))
	}

	if err = manifest.Apply(); err != nil {
		return err
	}

	return nil
}

// Kubernetes starts containers sequentially in order, which can mean the start of later
// containers is held up if their images have to be pulled. For large model server images
// this can cause a problem because model-mesh waits for a limited amount of time at
// startup for the runtime to become ready before returning ready from its own probe.
// Making the mm container last in the list ensures that no image pulling time will be
// included in this startup polling time and avoids unnecessary timeouts.
func (m *Deployment) ensureMMContainerIsLast(deployment *appsv1.Deployment) error {
	if i, _ := findContainer(ModelMeshContainerName, deployment); i >= 0 {
		last := len(deployment.Spec.Template.Spec.Containers) - 1
		if i != last {
			c := deployment.Spec.Template.Spec.Containers[last]
			deployment.Spec.Template.Spec.Containers[last] = deployment.Spec.Template.Spec.Containers[i]
			deployment.Spec.Template.Spec.Containers[i] = c
		}
	}
	return nil
}

func (m *Deployment) Delete(ctx context.Context, client client.Client) error {
	m.Log.Info("Deleting modelmesh deployment ", "name", m.Name, "namespace", m.Namespace)
	return config.Delete(client, m.Owner, "config/internal/base/deployment.yaml.tmpl", m, m.Namespace)
}

func (m *Deployment) transform(deployment *appsv1.Deployment, funcs ...func(deployment *appsv1.Deployment) error) error {
	for _, f := range funcs {
		if err := f(deployment); err != nil {
			return err
		}
	}

	return nil
}

func (m *Deployment) addMMDomainSocketMount(deployment *appsv1.Deployment) error {
	var c *corev1.Container
	if _, c = findContainer(ModelMeshContainerName, deployment); c == nil {
		return fmt.Errorf("Could not find the model mesh container %v", ModelMeshContainerName)
	}

	if hasUnix, mountPoint, err := mountPoint(m.SRSpec); err != nil {
		return err
	} else if hasUnix {
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "domain-socket",
			MountPath: mountPoint,
		})
	}

	return nil
}

func (m *Deployment) addMMEnvVars(deployment *appsv1.Deployment) error {
	// start with the "additional" env vars so that they are overwritten by
	// the values set below
	for _, envvar := range m.ModelMeshAdditionalEnvVars {
		if err := setEnvironmentVar(ModelMeshContainerName, envvar.Name, envvar.Value, deployment); err != nil {
			return err
		}
	}

	rts := m.SRSpec
	if rts.GrpcDataEndpoint != nil {
		e, err := ParseEndpoint(*rts.GrpcDataEndpoint)
		if err != nil {
			return err
		}
		if tcpE, ok := e.(TCPEndpoint); ok {
			if err = setEnvironmentVar(ModelMeshContainerName, ServeGrpcPortEnvVar, tcpE.Port, deployment); err != nil {
				return err
			}
		} else if udsE, ok := e.(UnixEndpoint); ok {
			if err = setEnvironmentVar(ModelMeshContainerName, ServeGrpcUdsPathEnvVar, udsE.Path, deployment); err != nil {
				return err
			}
		}
	}

	if useStorageHelper(rts) {
		if err := setEnvironmentVar(ModelMeshContainerName, GrpcPortEnvVar, strconv.Itoa(PullerPortNumber), deployment); err != nil {
			return err
		}
	} else {
		e, err := ParseEndpoint(*rts.GrpcMultiModelManagementEndpoint)
		if err != nil {
			return err
		}
		if tcpE, ok := e.(TCPEndpoint); ok {
			if err = setEnvironmentVar(ModelMeshContainerName, GrpcPortEnvVar, tcpE.Port, deployment); err != nil {
				return err
			}
		} else if udsE, ok := e.(UnixEndpoint); ok {
			if err = setEnvironmentVar(ModelMeshContainerName, GrpcUdsPathEnvVar, udsE.Path, deployment); err != nil {
				return err
			}
		}
	}

	if m.EnableAccessLogging {
		// See https://github.com/kserve/modelmesh/blob/v0.11.1/src/main/java/com/ibm/watson/modelmesh/ModelMeshEnvVars.java#L55
		if err := setEnvironmentVar(ModelMeshContainerName, "MM_LOG_EACH_INVOKE", "true", deployment); err != nil {
			return err
		}
	}

	if m.GrpcMaxMessageSize > 0 {
		// See https://github.com/kserve/modelmesh/blob/v0.11.1/src/main/java/com/ibm/watson/modelmesh/ModelMeshEnvVars.java#L38
		if err := setEnvironmentVar(ModelMeshContainerName, "MM_SVC_GRPC_MAX_MSG_SIZE", strconv.Itoa(m.GrpcMaxMessageSize), deployment); err != nil {
			return err
		}
	}

	// See https://github.com/kserve/modelmesh/blob/v0.11.1/src/main/java/com/ibm/watson/modelmesh/ModelMeshEnvVars.java#L31
	if err := setEnvironmentVar(ModelMeshContainerName, "MM_KVSTORE_PREFIX", ModelMeshEtcdPrefix, deployment); err != nil {
		return err
	}
	// See https://github.com/kserve/modelmesh/blob/v0.11.1/src/main/java/com/ibm/watson/modelmesh/ModelMeshEnvVars.java#L68
	if err := setEnvironmentVar(ModelMeshContainerName, "MM_DEFAULT_VMODEL_OWNER", m.DefaultVModelOwner, deployment); err != nil {
		return err
	}

	if len(m.PayloadProcessors) > 0 {
		// See https://github.com/kserve/modelmesh/blob/v0.11.1/src/main/java/com/ibm/watson/modelmesh/ModelMeshEnvVars.java#L26
		if err := setEnvironmentVar(ModelMeshContainerName, "MM_PAYLOAD_PROCESSORS", m.PayloadProcessors, deployment); err != nil {
			return err
		}
	}

	return nil
}

func (m *Deployment) setConfigMap() error {
	configMap := m.Owner.GetAnnotations()["productConfig"]
	if configMap == "" {
		return nil
	}

	// read configmap data.annotations
	clientParam := m.Client
	annotationConfigMap := &corev1.ConfigMap{}
	configMapErr := clientParam.Get(context.TODO(), client.ObjectKey{
		Name:      configMap,
		Namespace: m.Namespace}, annotationConfigMap)

	if configMapErr != nil {
		return fmt.Errorf("Unable to access ConfigMap '%s': %w", configMap, configMapErr)
	}

	m.AnnotationConfigMap = annotationConfigMap
	return nil
}
