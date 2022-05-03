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
package controllers

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"errors"

	corev1 "k8s.io/api/core/v1"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
)

// validateServingRuntimeSpec returns an error if the spec is invalid
// Checks include:
// - BuiltInAdapter has a container matching the name of the type
// - names of entries do not overlap with reserved names
// - spec does not override required internals like volumes or containers
// - containers do not mount internal only volumes
// - some fields in containers are controlled by model mesh and cannot be set
// - check for overlaps in declared ports with internal ports
func validateServingRuntimeSpec(rt *api.ServingRuntime) error {
	return validationChain(rt,
		validateBuiltInAdapterSpec,
		validateContainers,
		validateVolumes,
	)
}

func validationChain(rt *api.ServingRuntime, funcs ...func(*api.ServingRuntime) error) error {
	for _, f := range funcs {
		if err := f(rt); err != nil {
			return err
		}
	}
	return nil
}

func validateBuiltInAdapterSpec(rt *api.ServingRuntime) error {
	if rt.Spec.BuiltInAdapter == nil {
		return nil // nothing to check
	}

	st := rt.Spec.BuiltInAdapter.ServerType
	if _, ok := builtInServerTypes[st]; !ok {
		return fmt.Errorf("unrecognized built-in runtime server type %s", st)
	}
	for ic := range rt.Spec.Containers {
		if rt.Spec.Containers[ic].Name == string(st) {
			return nil // found, all good
		}
	}

	return fmt.Errorf("must include runtime container with name %s", st)
}

func validateContainers(rt *api.ServingRuntime) error {
	for i := range rt.Spec.Containers {
		c := &rt.Spec.Containers[i]
		if err := validateContainer(c); err != nil {
			return fmt.Errorf("container spec for %s is invalid: %w", c.Name, err)
		}
	}
	return nil
}

func validateContainer(c *corev1.Container) error {
	// Block container names that conflict with injected containers or reserved prefixes
	if err := checkName(c.Name, internalContainerNames, "container name"); err != nil {
		return err
	}

	// Block fields required for model-mesh to control the pod lifecycle
	if c.ReadinessProbe != nil || c.Lifecycle != nil {
		return errors.New("ReadinessProbe and Lifecycle are managed by modelmesh and cannot be set")
	}

	// Block volume mounts for private internal volumes
	for vmi := range c.VolumeMounts {
		if err := checkName(c.VolumeMounts[vmi].Name, internalOnlyVolumeMounts, "volume"); err != nil {
			return err
		}
	}

	for _, p := range c.Ports {
		// Block port names that conflict with injected containers or reserved prefixes
		if err := checkName(p.Name, internalNamedPorts, "port name"); err != nil {
			return err
		}

		// Check for conflicting port usage
		if _, ok := internalPorts[p.ContainerPort]; ok {
			return fmt.Errorf("Port %d is reserved for internal use", p.ContainerPort)
		}
		// Reserve a range for future use
		// ascii 'm' is 109. ModelMesh -> mm -> m*m = 11881
		if p.ContainerPort >= 11881 && p.ContainerPort < 11900 {
			return fmt.Errorf("Port range [11881-11899] is reserved for internal use")
		}
	}

	return nil
}

func validateVolumes(rt *api.ServingRuntime) error {
	// Block volume names that conflict with injected volumes or reserved prefixes
	for vi := range rt.Spec.Volumes {
		if err := checkName(rt.Spec.Volumes[vi].Name, internalVolumes, "volume"); err != nil {
			return err
		}
	}

	return nil
}

func checkName(name string, internalNames sets.String, logStr string) error {
	if internalNames.Has(name) {
		return fmt.Errorf("%s %s is reserved for internal use", logStr, name)
	}

	if strings.HasPrefix(name, "mm-") {
		return fmt.Errorf("%s cannot start with \"mm-\", which is reserved for internal use", logStr)
	}
	return nil
}

var internalContainerNames = sets.NewString(
	modelmesh.ModelMeshContainerName,
	modelmesh.RESTProxyContainerName,
	modelmesh.PullerContainerName,
)

var internalOnlyVolumeMounts = sets.NewString(
	modelmesh.ConfigStorageMount,
	modelmesh.EtcdVolume,
	modelmesh.InternalConfigMapName,
	modelmesh.SocketVolume,
)

var internalNamedPorts = sets.NewString("grpc", "http", "prometheus")

var internalPorts = map[int32]struct{}{
	8080: {}, // is used for LiteLinks communication in Model Mesh
	8085: {}, // is the port the built-in adapter listens on
	8089: {}, // is used for Model Mesh probes
	8090: {}, // is used for default preStop hooks
}

var internalVolumes = sets.NewString(
	modelmesh.ConfigStorageMount,
	modelmesh.EtcdVolume,
	modelmesh.InternalConfigMapName,
	modelmesh.SocketVolume,
	modelmesh.ModelsDirVolume,
)
