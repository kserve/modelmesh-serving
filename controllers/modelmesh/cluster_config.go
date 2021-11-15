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

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	Runtimes  *api.ServingRuntimeList
	Namespace string
	Scheme    *runtime.Scheme
}

func (cc ClusterConfig) Apply(ctx context.Context, owner metav1.Object, cl client.Client) error {
	commonLabelValue := "modelmesh-controller"
	m := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      InternalConfigMapName,
			Namespace: cc.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/instance":   commonLabelValue,
				"app.kubernetes.io/managed-by": commonLabelValue,
				"app.kubernetes.io/name":       commonLabelValue,
			},
		},
	}
	cc.addConstraints(cc.Runtimes, m)
	err := controllerutil.SetControllerReference(owner, m, cc.Scheme)
	if err != nil {
		return err
	}

	if err = cl.Create(ctx, m); err != nil && errors.IsAlreadyExists(err) {
		err = cl.Update(ctx, m)
	}

	return err
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
		if !rt.Disabled() {
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
