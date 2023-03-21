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
package fvt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func InitializeFVTClient() {
	Log = zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
	Log.Info("Initializing test suite")

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = DefaultTestNamespace
	}
	serviceName := os.Getenv("SERVICENAME")
	if serviceName == "" {
		serviceName = DefaultTestServiceName
	}
	controllerNamespace := os.Getenv("CONTROLLERNAMESPACE")
	if controllerNamespace == "" {
		controllerNamespace = DefaultControllerNamespace
	}
	NameSpaceScopeMode = os.Getenv("NAMESPACESCOPEMODE") == "true"
	Log.Info("Using environment variables", "NAMESPACE", namespace, "SERVICENAME", serviceName,
		"CONTROLLERNAMESPACE", controllerNamespace, "NAMESPACESCOPEMODE", NameSpaceScopeMode)

	var err error
	FVTClientInstance, err = GetFVTClient(Log, namespace, serviceName, controllerNamespace)
	Expect(err).ToNot(HaveOccurred())
	Expect(FVTClientInstance).ToNot(BeNil())
	Log.Info("FVTClientInstance created", "client", FVTClientInstance)
}

// Utility function to return the testdata directory
func TestDataPath(resourcePathWithinTestData string) string {
	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	parent := filepath.Dir(wd)
	return parent + "/testdata/" + resourcePathWithinTestData
}

func DecodeResourceFromFile(resourcePath string) *unstructured.Unstructured {
	content, err := os.ReadFile(resourcePath)
	Expect(err).ToNot(HaveOccurred())

	obj := &unstructured.Unstructured{}

	// decode YAML into unstructured.Unstructured
	dec := yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	decodedObj, _, err := dec.Decode([]byte(content), nil, obj)
	Expect(err).ToNot(HaveOccurred())

	obj = decodedObj.(*unstructured.Unstructured)
	Expect(obj).ToNot(BeNil())
	return obj
}

func CreateSecret(secret *corev1.Secret, namespace string, fvt *FVTClient) {
	patchJson, err := json.Marshal(secret)
	Expect(err).ToNot(HaveOccurred())

	updatedSecret, err := fvt.Resource(gvrSecret).Namespace(namespace).
		Patch(context.TODO(), secret.Name, types.ApplyPatchType, patchJson, applyPatchOptions)

	Expect(err).ToNot(HaveOccurred())
	Expect(updatedSecret).ToNot(BeNil())

	fvt.log.Info(fmt.Sprintf("Secret '%s' created", updatedSecret.GetName()))
}

// Small functions to work with unstructured objects

func GetInt64(obj *unstructured.Unstructured, fieldPath ...string) int64 {
	value, _, err := unstructured.NestedInt64(obj.Object, fieldPath...)
	Expect(err).ToNot(HaveOccurred())
	return value
}

func GetString(obj *unstructured.Unstructured, fieldPath ...string) string {
	value, exists, err := unstructured.NestedString(obj.Object, fieldPath...)
	Expect(exists).To(BeTrue())
	Expect(err).ToNot(HaveOccurred())
	return value
}

func GetSlice(obj *unstructured.Unstructured, fieldPath ...string) ([]interface{}, bool) {
	value, exists, err := unstructured.NestedSlice(obj.Object, fieldPath...)
	Expect(err).ToNot(HaveOccurred())
	return value, exists
}

func GetMap(obj *unstructured.Unstructured, fieldPath ...string) map[string]interface{} {
	value, _, err := unstructured.NestedMap(obj.Object, fieldPath...)
	Expect(err).ToNot(HaveOccurred())
	return value
}

func SetString(obj *unstructured.Unstructured, value string, fieldPath ...string) {
	err := unstructured.SetNestedField(obj.Object, value, fieldPath...)
	Expect(err).ToNot(HaveOccurred())
}

func GetBool(obj *unstructured.Unstructured, fieldPath ...string) bool {
	value, exists, err := unstructured.NestedBool(obj.Object, fieldPath...)
	Expect(exists).To(BeTrue())
	Expect(err).ToNot(HaveOccurred())
	return value
}
