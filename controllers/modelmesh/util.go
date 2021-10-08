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
	"fmt"
	"path/filepath"
	"strings"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Find a container by name in the given deployment, returns (-1, nil) if not found
func findContainer(name string, deployment *appsv1.Deployment) (index int, container *corev1.Container) {
	for i := range deployment.Spec.Template.Spec.Containers {
		if c := &deployment.Spec.Template.Spec.Containers[i]; c.Name == name {
			return i, c
		}
	}
	return -1, nil
}

// Sets an environment variable by name
func setEnvironmentVar(container string, variable string, value string, deployment *appsv1.Deployment) error {
	if _, c := findContainer(container, deployment); c != nil {
		for i := range c.Env {
			if c.Env[i].Name == variable {
				c.Env[i].Value = value
				return nil
			}
		}
		c.Env = append(c.Env, corev1.EnvVar{Name: variable, Value: value})
		return nil
	}

	return fmt.Errorf("Cannot find container: %v", container)
}

// Determines if any unix domain sockets are present and returns
// the unix:/// path and mount directory
func unixDomainSockets(rt *api.ServingRuntime) (bool, []string, []string) {
	endpoints := []*string{
		rt.Spec.GrpcDataEndpoint,
		//rt.Spec.HTTPDataEndpoint,
		rt.Spec.GrpcMultiModelManagementEndpoint,
	}

	var _endpoints, _fspaths []string
	for _, endpoint := range endpoints {
		if endpoint != nil && strings.HasPrefix(*endpoint, "unix:") {
			fspath := strings.Replace(*endpoint, "unix://", "", 1)
			fspath = strings.Replace(fspath, "unix:", "", 1)
			fspath = filepath.Dir(fspath)
			_endpoints = append(_endpoints, *endpoint)
			_fspaths = append(_fspaths, fspath)
		}
	}

	if len(_endpoints) > 0 {
		return true, _endpoints, _fspaths
	}

	return false, nil, nil
}

// useStorageHelper returns true if the model puller needs to be injected into the runtime deployment
// either built-in adapter is not specified or storage helper is enabled
func useStorageHelper(rt *api.ServingRuntime) bool {
	return rt.Spec.BuiltInAdapter == nil && (rt.Spec.StorageHelper == nil || !rt.Spec.StorageHelper.Disabled)
}

var (
	_ = toYaml
)

func toYaml(resources []unstructured.Unstructured) string {
	res := ""
	for _, resource := range resources {
		b, _ := yaml.Marshal(resource)
		res = res + "---\n"
		res = res + string(b) + "\n"
	}
	return res
}

// Finds the common mount point for required unix domain sockets
func mountPoint(rt *api.ServingRuntime) (bool, string, error) {
	findParentPath := func(str string) (bool, string, error) {
		e, err := ParseEndpoint(str)
		if err != nil {
			return false, "", err
		}
		if _, ok := e.(TCPEndpoint); ok {
			return false, "", nil
		}
		if udsE, ok := e.(UnixEndpoint); ok {
			return true, udsE.ParentPath, nil
		}
		return false, "", errors.New("Cannot find the mount point for input " + str)
	}

	//if rt.Spec.HTTPDataEndpoint != nil {
	//	isUnix, path, err := findParentPath(*rt.Spec.HTTPDataEndpoint)
	//	return isUnix, path, err
	//}
	if rt.Spec.GrpcDataEndpoint != nil {
		isUnix, path, err := findParentPath(*rt.Spec.GrpcDataEndpoint)
		return isUnix, path, err
	}
	if rt.Spec.GrpcMultiModelManagementEndpoint != nil {
		isUnix, path, err := findParentPath(*rt.Spec.GrpcMultiModelManagementEndpoint)
		return isUnix, path, err
	}

	return false, "", nil
}
