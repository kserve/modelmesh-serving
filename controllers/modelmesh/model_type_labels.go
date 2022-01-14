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
	"sort"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
)

type StringSet map[string]struct{}

var exists = struct{}{} // empty struct placeholder

func (ss StringSet) Add(s string) {
	ss[s] = exists
}

func (ss StringSet) Contains(s string) bool {
	_, found := ss[s]
	return found
}

func (ss StringSet) ToSlice() []string {
	strs := make([]string, 0, len(ss))
	for s := range ss {
		strs = append(strs, s)
	}
	// map keys are not ordered, so we sort them
	sort.Strings(strs)
	return strs
}

func GetServingRuntimeSupportedModelTypeLabelSet(rt *api.ServingRuntime) StringSet {
	set := make(StringSet, 2*len(rt.Spec.SupportedModelFormats)+1)

	// model type labels
	for _, t := range rt.Spec.SupportedModelFormats {
		// only include model type labels when autoSelect is true
		if t.AutoSelect != nil && *t.AutoSelect {
			set.Add("mt:" + t.Name)
			if t.Version != nil {
				set.Add("mt:" + t.Name + ":" + *t.Version)
			}
		}
	}
	// runtime label
	set.Add("rt:" + rt.Name)
	return set
}

func GetPredictorModelTypeLabel(p *api.Predictor) string {
	runtime := p.Spec.Runtime
	if runtime != nil && runtime.Name != "" {
		// constrain placement to specific runtime
		return "rt:" + runtime.Name
	}
	// constrain placement based on model type
	mt := p.Spec.Model.Type
	if mt.Version != nil {
		return "mt:" + mt.Name + ":" + *mt.Version
	}
	return "mt:" + mt.Name
}
