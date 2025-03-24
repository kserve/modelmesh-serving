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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/pkg/config"
)

const defaultTypeConstraint = "_default"

const ModelTypeLabelThatNoRuntimeSupports = "_no_runtime"

var dataPlaneApiJsonConfigBytes = []byte(`{
    "rpcConfigs": {
        "inference.GRPCInferenceService/ModelInfer": {
            "idExtractionPath": [1],
            "vModelId": true
        },
        "inference.GRPCInferenceService/ModelMetadata": {
            "idExtractionPath": [1],
            "vModelId": true
        },
        "tensorflow.serving.PredictionService/Predict": {
            "idExtractionPath": [1, 1],
            "vModelId": true
        },
        "org.pytorch.serve.grpc.inference.InferenceAPIsService/Predictions": {
            "idExtractionPath": [1],
            "vModelId": true
        }
    },
    "allowOtherRpcs": true
}`)

// A ClusterConfig represents the configuration shared across
// a logical model mesh cluster
type ClusterConfig struct {
	SRSpecs map[string]*kserveapi.ServingRuntimeSpec
	Scheme  *runtime.Scheme
}

func (cc ClusterConfig) Reconcile(ctx context.Context, namespace string, cl client.Client, cfg *config.Config) error {
	m := &corev1.ConfigMap{}
	err := cl.Get(ctx, types.NamespacedName{Name: InternalConfigMapName, Namespace: namespace}, m)
	notfound := errors.IsNotFound(err)
	if err != nil && !notfound {
		return err
	}

	if len(cc.SRSpecs) == 0 {
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
	cc.addConstraints(cc.SRSpecs, m, cfg.RESTProxy.Enabled)

	if notfound {
		return cl.Create(ctx, m)
	} else {
		return cl.Update(ctx, m)
	}
}

// Add constraint data to the provided config map
func (cc ClusterConfig) addConstraints(srSpecs map[string]*kserveapi.ServingRuntimeSpec, m *corev1.ConfigMap, restProxyEnabled bool) {
	b := calculateConstraintData(srSpecs, restProxyEnabled)
	if m.BinaryData == nil {
		m.BinaryData = make(map[string][]byte)
	}
	m.BinaryData[MMTypeConstraintsKey] = b
	m.BinaryData[MMDataPlaneConfigKey] = dataPlaneApiJsonConfigBytes
}

func calculateConstraintData(srSpecs map[string]*kserveapi.ServingRuntimeSpec, restProxyEnabled bool) []byte {

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
	for name, spec := range srSpecs {
		if !spec.IsDisabled() && spec.IsMultiModelRuntime() {
			mtLabels, pvLabels, rtLabel := GetServingRuntimeLabelSets(spec, restProxyEnabled, name)
			m[rtLabel] = map[string]interface{}{"required": []string{rtLabel}}
			// treat each combo of model-type label and proto version label as a separate model type
			for l := range mtLabels {
				m[l] = map[string]interface{}{"required": []string{l}}
				for pvl := range pvLabels {
					m[fmt.Sprintf("%s|%s", l, pvl)] = map[string]interface{}{"required": []string{l, pvl}}
				}
			}
		}
	}
	// Have a default requirement that is not satisfied by any runtime
	m[defaultTypeConstraint] = map[string]interface{}{"required": []string{ModelTypeLabelThatNoRuntimeSupports}}

	b, _ := json.Marshal(m)
	return b
}
