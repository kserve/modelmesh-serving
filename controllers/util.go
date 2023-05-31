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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func modelMeshEnabled(n *corev1.Namespace, controllerNamespace string) bool {
	if v, ok := n.Labels["modelmesh-enabled"]; ok {
		return v == "true"
	}
	return n.Name == controllerNamespace
}

func modelMeshEnabled2(ctx context.Context, namespace, controllerNamespace string,
	client client.Client, clusterScope bool) (bool, error) {

	// don't attempt to access namespace resource if not cluster scope
	if !clusterScope {
		return namespace == controllerNamespace, nil
	}
	n := &corev1.Namespace{}
	if err := client.Get(ctx, types.NamespacedName{Name: namespace}, n); err != nil {
		return false, err
	}
	return modelMeshEnabled(n, controllerNamespace), nil
}
