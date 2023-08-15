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

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/constants"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetServingRuntimeLabelSets(rt *kserveapi.ServingRuntimeSpec, restProxyEnabled bool, rtName string) (
	mtLabels sets.String, pvLabels sets.String, rtLabel string) {

	// model type labels
	mtSet := make(sets.String, 2*len(rt.SupportedModelFormats))
	for _, t := range rt.SupportedModelFormats {
		// only include model type labels when autoSelect is true
		if t.AutoSelect != nil && *t.AutoSelect {
			mtSet.Insert(fmt.Sprintf("mt:%s", t.Name))
			if t.Version != nil {
				mtSet.Insert(fmt.Sprintf("mt:%s:%s", t.Name, *t.Version))
			}
		}
	}
	// protocol versions
	pvSet := make(sets.String, len(rt.ProtocolVersions))
	for _, pv := range rt.ProtocolVersions {
		pvSet.Insert(fmt.Sprintf("pv:%s", pv))
		if restProxyEnabled && pv == constants.ProtocolGRPCV2 {
			pvSet.Insert(fmt.Sprintf("pv:%s", constants.ProtocolV2))
		}
	}
	// runtime label
	return mtSet, pvSet, fmt.Sprintf("rt:%s", rtName)
}

func GetServingRuntimeLabelSet(rt *kserveapi.ServingRuntimeSpec, restProxyEnabled bool, rtName string) sets.String {
	s1, s2, l := GetServingRuntimeLabelSets(rt, restProxyEnabled, rtName)
	s1 = s1.Union(s2)
	s1.Insert(l)
	return s1
}

func GetPredictorTypeLabel(p *api.Predictor) string {
	runtime := p.Spec.Runtime
	if runtime != nil && runtime.Name != "" {
		// constrain placement to specific runtime
		return "rt:" + runtime.Name
	}
	// constrain placement based on model type
	mt := p.Spec.Model.Type
	label := "mt:" + mt.Name
	if mt.Version != nil {
		label = fmt.Sprintf("%s:%s", label, *mt.Version)
	}
	if pv := p.Spec.ProtocolVersion; pv != nil {
		label = fmt.Sprintf("%s|pv:%s", label, *pv)
	}
	return label
}
