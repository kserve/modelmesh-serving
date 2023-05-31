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

package config

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Overlay is a transform which overlays the provided object onto the source
func Overlay(o *unstructured.Unstructured) func(*unstructured.Unstructured) error {
	return func(resource *unstructured.Unstructured) error {
		return overlay(resource, o)
	}
}

func overlay(resource *unstructured.Unstructured, overlay *unstructured.Unstructured) error {
	overlayMap(resource.UnstructuredContent()["spec"].(map[string]interface{}), overlay.UnstructuredContent()["spec"].(map[string]interface{}))
	return nil
}

func overlayMap(source map[string]interface{}, overlay map[string]interface{}) map[string]interface{} {
	// check to see if the overlay should apply
	for k, ov := range overlay {
		if v, exists := source[k]; exists {
			rt := reflect.TypeOf(v)
			switch rt.Kind() {
			case reflect.String:
				if k == "name" {
					if v == ov {
						// names match, continue with merge
						break
					} else {
						// different names, abort further processing
						return source
					}
				}
			}
		}
	}

	//Overlay matches, continue with merge
	for k, ov := range overlay {
		if v, hasKey := source[k]; !hasKey {
			//not present in target, add it
			source[k] = ov
		} else {
			rt := reflect.TypeOf(v)
			switch rt.Kind() {
			case reflect.Array:
				overlaySlice(v.([]interface{}), ov.([]interface{}))
			case reflect.Slice:
				overlaySlice(v.([]interface{}), ov.([]interface{}))
			case reflect.Map:
				//recurse into key
				overlayMap(v.(map[string]interface{}), ov.(map[string]interface{}))
			case reflect.String:
				//assign a string key from the overlay into the origina
				source[k] = ov
			default:
				panic(fmt.Sprintf("Unhandled %v", rt.Kind()))
			}
		}
	}

	return source
}

func overlaySlice(source []interface{}, overlay []interface{}) []interface{} {
	for _, oelem := range overlay {
		if ov, isMap := oelem.(map[string]interface{}); isMap {
			oname := ov["name"]

			var v map[string]interface{}
			for _, selem := range source {
				_v := selem.(map[string]interface{})
				if sname, hasName := _v["name"]; hasName && sname == oname {
					v = _v
					break
				}
			}

			overlayMap(v, ov)
		} else {
			panic(fmt.Sprintf("Unhandled: %v", oelem))
		}
	}

	return source
}
