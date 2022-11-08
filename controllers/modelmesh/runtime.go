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
	"regexp"
	"strconv"
	"strings"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ModelsDir     string  = "/models"
	ModelDirScale float64 = 1.5
)

// Sets the model mesh grace period to match the deployment grace period
func (m *Deployment) syncGracePeriod(deployment *appsv1.Deployment) error {
	if deployment.Spec.Template.Spec.TerminationGracePeriodSeconds != nil {
		gracePeriodS := deployment.Spec.Template.Spec.TerminationGracePeriodSeconds
		gracePeriodMs := *gracePeriodS * int64(1000)
		gracePeriodMsStr := strconv.FormatInt(gracePeriodMs, 10)
		err := setEnvironmentVar(ModelMeshContainerName, "SHUTDOWN_TIMEOUT_MS", gracePeriodMsStr, deployment)
		return err
	}

	return nil
}

func (m *Deployment) addVolumesToDeployment(deployment *appsv1.Deployment) error {
	rts := m.SRSpec
	modelsDirSize := calculateModelDirSize(rts)

	// start from the volumes specified in the runtime spec
	volumes := rts.Volumes

	volumes = append(volumes, corev1.Volume{
		Name: ModelsDirVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{Medium: "", SizeLimit: modelsDirSize},
		},
	})

	if hasUnixSockets, _, _ := unixDomainSockets(rts); hasUnixSockets {
		volumes = append(volumes, corev1.Volume{
			Name: SocketVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	// need to mount storage config for built-in adapters and the scenarios where StorageHelper is not disabled
	if rts.BuiltInAdapter != nil || useStorageHelper(rts) {
		storageVolume := corev1.Volume{
			Name: ConfigStorageMount,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: StorageSecretName,
				},
			},
		}

		volumes = append(volumes, storageVolume)
	}

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volumes...)

	return nil
}

// calculate emptyDir Size
func calculateModelDirSize(rts *kserveapi.ServingRuntimeSpec) *resource.Quantity {

	memorySize := resource.MustParse("0")

	for _, cspec := range rts.Containers {
		memorySize.Add(cspec.Resources.Limits[corev1.ResourceMemory])
	}

	return resource.NewQuantity(int64(float64(memorySize.Value())*ModelDirScale), resource.BinarySI)
}

// Adds the provided runtime to the deployment
func (m *Deployment) addRuntimeToDeployment(deployment *appsv1.Deployment) error {
	rts := m.SRSpec

	// first prepare the common variables needed for both adapter and other containers
	lifecycle := &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/prestop",
				Port: intstr.FromInt(8090),
			},
		},
	}
	dropAllSecurityContext := &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      ModelsDirVolume,
			MountPath: ModelsDir,
		},
	}

	// Now add the containers specified in serving runtime spec
	for i := range rts.Containers {
		// by modifying in-place we rely on the fact that the cacheing
		// client in controller-runtime deep copies objects it retrieves
		// by default, this would be a problem if that was disabled
		// REF: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cache
		cspec := &rts.Containers[i]

		// defaults
		if cspec.SecurityContext == nil {
			cspec.SecurityContext = dropAllSecurityContext
		}

		// add internal fields
		cspec.VolumeMounts = append(volumeMounts, cspec.VolumeMounts...)
		cspec.Lifecycle = lifecycle

		if err := addDomainSocketMount(rts, cspec); err != nil {
			return err
		}

		// if multiple containers with the same name are included, the last one wins
		if i, _ := findContainer(cspec.Name, deployment); i >= 0 {
			deployment.Spec.Template.Spec.Containers[i] = *cspec
		} else {
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *cspec)
		}
	}

	if rts.BuiltInAdapter != nil {
		// BuiltInAdapter is specified, so prepare adapter container
		// Validation is already happened in reconcile logic, so just append "-adapter" to runtimeName for adapterName
		runtimeName := string(rts.BuiltInAdapter.ServerType)
		runtimeAdapterName := runtimeName + "-adapter"

		builtInAdapterContainer := corev1.Container{
			Command:         []string{"/opt/app/" + runtimeAdapterName},
			Image:           m.PullerImage,
			Name:            runtimeAdapterName,
			Lifecycle:       lifecycle,
			SecurityContext: dropAllSecurityContext,
		}

		var runtimeVersion string
		if _, c := findContainer(runtimeName, deployment); c != nil {
			if i := strings.IndexRune(c.Image, ':'); i >= 0 {
				runtimeVersion = c.Image[i+1:]
			}
		} else {
			m.Log.Error(nil, "ServingRuntime uses built-in adapter"+
				" but does not include a container with the name of the specified server type",
				"servingRuntime", m.Name, "serverType", runtimeName)
		}

		// the puller and adapter containers are the same image and are given the
		// same resources
		builtInAdapterContainer.Resources = *m.PullerResources

		builtInAdapterContainer.VolumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ConfigStorageMount,
			MountPath: PullerConfigPath,
			ReadOnly:  true,
		})

		var rtDataEndpoint string
		if rts.GrpcDataEndpoint != nil {
			rtDataEndpoint = *rts.GrpcDataEndpoint
			if err := addDomainSocketMount(rts, &builtInAdapterContainer); err != nil {
				return err
			}
		} else {
			rtDataEndpoint = fmt.Sprintf("port:%d", rts.BuiltInAdapter.RuntimeManagementPort)
		}

		adapterPort := "8085"
		if rts.GrpcMultiModelManagementEndpoint != nil {
			ep := *rts.GrpcMultiModelManagementEndpoint
			if match, _ := regexp.MatchString("^port:[0-9]+$", ep); !match {
				return fmt.Errorf("Built-in adapter grpcEndpoint must be of the form \"port:N\": %s", ep)
			}
			adapterPort = strings.TrimPrefix(ep, "port:")
		}

		builtInAdapterContainer.Env = []corev1.EnvVar{
			{
				Name:  "ADAPTER_PORT",
				Value: adapterPort,
			},
			{
				Name:  "RUNTIME_PORT",
				Value: strconv.Itoa(rts.BuiltInAdapter.RuntimeManagementPort),
			},
			{
				Name:  "RUNTIME_DATA_ENDPOINT",
				Value: rtDataEndpoint,
			},
			{
				Name: "CONTAINER_MEM_REQ_BYTES",
				ValueFrom: &corev1.EnvVarSource{
					ResourceFieldRef: &corev1.ResourceFieldSelector{
						ContainerName: runtimeName,
						Resource:      "requests.memory",
					},
				},
			},
			{
				Name:  "MEM_BUFFER_BYTES",
				Value: strconv.Itoa(rts.BuiltInAdapter.MemBufferBytes),
			},
			{
				Name:  "LOADTIME_TIMEOUT",
				Value: strconv.Itoa(rts.BuiltInAdapter.ModelLoadingTimeoutMillis),
			},
			{
				Name:  "USE_EMBEDDED_PULLER",
				Value: "true",
			},
			{
				Name:  "RUNTIME_VERSION",
				Value: runtimeVersion,
			},
			{}, {}, {}, {}, // allocate larger array to avoid reallocation
		}[:8]

		if mlc, ok := m.Owner.GetAnnotations()["maxLoadingConcurrency"]; ok {
			builtInAdapterContainer.Env = append(builtInAdapterContainer.Env, corev1.EnvVar{
				Name:  "LOADING_CONCURRENCY",
				Value: mlc,
			})
		}

		if pmcl, ok := m.Owner.GetAnnotations()["perModelConcurrencyLimit"]; ok {
			builtInAdapterContainer.Env = append(builtInAdapterContainer.Env, corev1.EnvVar{
				Name:  "LIMIT_PER_MODEL_CONCURRENCY",
				Value: pmcl,
			})
		}
	outer:
		for oidx := range rts.BuiltInAdapter.Env {
			for eidx := range builtInAdapterContainer.Env {
				if builtInAdapterContainer.Env[eidx].Name == rts.BuiltInAdapter.Env[oidx].Name {
					builtInAdapterContainer.Env[eidx] = rts.BuiltInAdapter.Env[oidx]
					continue outer
				}
			}
			builtInAdapterContainer.Env = append(builtInAdapterContainer.Env, rts.BuiltInAdapter.Env[oidx])
		}
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, builtInAdapterContainer)
	}

	return nil
}

func addDomainSocketMount(rts *kserveapi.ServingRuntimeSpec, c *corev1.Container) error {
	var requiresDomainSocketMounting bool
	var domainSocketMountPoint string
	endpoints := []*string{
		rts.GrpcDataEndpoint,
		//		rts.HTTPDataEndpoint,
		rts.GrpcMultiModelManagementEndpoint,
	}
	for _, endpointStr := range endpoints {
		if endpointStr != nil {
			e, err := ParseEndpoint(*endpointStr)
			if err != nil {
				return err
			}
			if udsE, ok := e.(UnixEndpoint); ok {
				requiresDomainSocketMounting = true
				_mountPoint := udsE.ParentPath
				if domainSocketMountPoint != "" && domainSocketMountPoint != _mountPoint {
					return fmt.Errorf("Only one unix domain socket path is allowed. Found %v", endpoints)
				}

				domainSocketMountPoint = _mountPoint
			}
		}
	}
	if requiresDomainSocketMounting {
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      SocketVolume,
			MountPath: domainSocketMountPoint,
		})
	}

	return nil
}

func (m *Deployment) addPassThroughPodFieldsToDeployment(deployment *appsv1.Deployment) error {
	rts := m.SRSpec
	// these fields map directly to pod spec fields
	deployment.Spec.Template.Spec.NodeSelector = rts.NodeSelector
	deployment.Spec.Template.Spec.Tolerations = rts.Tolerations
	archNodeSelector := corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      "kubernetes.io/arch",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"amd64"},
			},
		},
	}
	deployment.Spec.Template.Spec.Affinity = rts.Affinity
	if rts.Affinity == nil {
		deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						archNodeSelector,
					},
				},
			},
		}
	} else if rts.Affinity.NodeAffinity == nil {
		deployment.Spec.Template.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					archNodeSelector,
				},
			},
		}
	} else if rts.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				archNodeSelector,
			},
		}
	} else {
		nodeSelectors := rts.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(nodeSelectors, archNodeSelector)
	}

	return nil
}

func (m *Deployment) configureRuntimePodSpecAnnotations(deployment *appsv1.Deployment) error {
	// merging the annotations
	// priority: ServingRuntimePodSpec > AnnotationsMap > whatever in the deployment template
	deployment.Spec.Template.Annotations = Union(deployment.Spec.Template.Annotations, m.AnnotationsMap, m.SRSpec.ServingRuntimePodSpec.Annotations)
	return nil
}

func (m *Deployment) configureRuntimePodSpecLabels(deployment *appsv1.Deployment) error {
	// merging the labels
	// priority: ServingRuntimePodSpec > LabelsMap > whatever in the deployment template
	deployment.Spec.Template.Labels = Union(deployment.Spec.Template.Labels, m.LabelsMap, m.SRSpec.ServingRuntimePodSpec.Labels)
	return nil
}
