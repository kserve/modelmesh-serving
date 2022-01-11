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
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultTypeConstraint = "_default"

const ModelTypeLabelThatNoRuntimeSupports = "_no_runtime"

var dataPlaneApiJsonConfigBytes = []byte(`{
    "rpcConfigs": {
        "inference.GRPCInferenceService/ModelInfer": {
            "idExtractionPath": [1],
            "vModelId": true
        }
    },
    "allowOtherRpcs": true
}`)

// A ClusterConfig represents the configuration shared across
// a logical model mesh cluster
type ClusterConfig struct {
	Runtimes *api.ServingRuntimeList
	Scheme   *runtime.Scheme
}

func (cc ClusterConfig) Reconcile(ctx context.Context, namespace string, cl client.Client) error {
	m := &corev1.ConfigMap{}
	err := cl.Get(ctx, types.NamespacedName{Name: InternalConfigMapName, Namespace: namespace}, m)
	notfound := errors.IsNotFound(err)
	if err != nil && !notfound {
		return err
	}
	if cc.Runtimes == nil || len(cc.Runtimes.Items) == 0 {
		if !notfound {
			return cl.Delete(ctx, m)
		}
		return nil
	}

	commonLabelValue := "modelmesh-controller"
	m.ObjectMeta = metav1.ObjectMeta{
		Name:      InternalConfigMapName,
		Namespace: namespace,
		Labels: map[string]string{
			"app.kubernetes.io/instance":   commonLabelValue,
			"app.kubernetes.io/managed-by": commonLabelValue,
			"app.kubernetes.io/name":       commonLabelValue,
		},
	}
	cc.addConstraints(cc.Runtimes, m)

	if notfound {
		return cl.Create(ctx, m)
	} else {
		return cl.Update(ctx, m)
	}
}

// Add constraint data to the provided config map
func (cc ClusterConfig) addConstraints(rts *api.ServingRuntimeList, m *corev1.ConfigMap) {
	b := calculateConstraintData(rts.Items)
	if m.BinaryData == nil {
		m.BinaryData = make(map[string][]byte)
	}
	m.BinaryData[MMTypeConstraintsKey] = b
	m.BinaryData[MMDataPlaneConfigKey] = dataPlaneApiJsonConfigBytes
}

func calculateConstraintData(rts []api.ServingRuntime) []byte {
	/*b := []byte(`{
	  "rt:tf-serving-runtime": {
	    "required": ["rt:tf-serving-runtime"]
	  },
	  "mt:tensorflow": {
	    "required": ["mt:tensorflow"]
	  },
	  "mt:tensorflow:1.15": {
	    "required": ["mt:tensorflow:1.15"]
	  },
	  "_default": {
	    "required": ["_no_runtime"]
	  }
	}`)*/

	m := make(map[string]interface{})
	for _, rt := range rts {
		if !rt.Disabled() && rt.IsMultiModelRuntime() {
			labels := GetServingRuntimeSupportedModelTypeLabelSet(&rt)
			// treat each label as a separate model type
			for l := range labels {
				m[l] = map[string]interface{}{"required": []string{l}}
			}
		}
	}
	// Have a default requirement that is not satisfied by any runtime
	m[defaultTypeConstraint] = map[string]interface{}{"required": []string{ModelTypeLabelThatNoRuntimeSupports}}

	b, _ := json.Marshal(m)
	return b
}
