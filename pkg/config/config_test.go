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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewMergedConfigFromString(t *testing.T) {
	yaml := `
storageHelperImage:
  name: override
  tag: "1.0"`

	expectedStorageHelperImage := "override:1.0"
	expectedPodsPerRuntime := uint16(2)

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	//Verify system config map default
	if conf.PodsPerRuntime != expectedPodsPerRuntime {
		t.Fatalf("Expected PodsPerDeployment=%v but found %v", expectedPodsPerRuntime, conf.PodsPerRuntime)
	}

	//Verify user override
	if conf.StorageHelperImage.TaggedImage() != expectedStorageHelperImage {
		t.Fatalf("Expected StorageHelperImage=%v but found %v",
			expectedStorageHelperImage, conf.StorageHelperImage.TaggedImage())
	}
}

func TestNewMergedConfigFromStringWithDotNotationKeys(t *testing.T) {
	yaml := `
runtimePodLabels:
  foo.bar: test
  network-policy: allow-egress`

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, conf.RuntimePodLabels["foo.bar"], "test")
	assert.Equal(t, conf.RuntimePodLabels["network-policy"], "allow-egress")
}

func TestNewMergedConfigFromStringImage(t *testing.T) {
	type testCase struct {
		configYaml                 string
		expectedStorageHelperImage string
		expectedModelMeshImage     string
	}
	testCases := []testCase{
		{
			configYaml:                 "storageHelperImage:\n  tag: tag-override",
			expectedStorageHelperImage: "kserve/modelmesh-runtime-adapter:tag-override",
		},
		{
			configYaml:             "modelMeshImage:\n  name: model-mesh\n  tag: some-mm-tag",
			expectedModelMeshImage: "model-mesh:some-mm-tag",
		},
		{
			configYaml:             "modelMeshImage:\n  name: model-mesh\n  tag: some-mm-tag\n  digest: sha256:hash",
			expectedModelMeshImage: "model-mesh@sha256:hash",
		},
		{
			configYaml:                 "storageHelperImage:\n  name: storage-helper\n  digest: sha256:anotherhash",
			expectedStorageHelperImage: "storage-helper@sha256:anotherhash",
		},
	}

	for i, tc := range testCases {
		conf, err := NewMergedConfigFromString(tc.configYaml)
		if err != nil {
			t.Errorf("Could not parse config in test case [%d]: %v", i, err)
		}

		// verify expected tags
		if tc.expectedStorageHelperImage != "" {
			if conf.StorageHelperImage.TaggedImage() != tc.expectedStorageHelperImage {
				t.Errorf("Failed test case [%d]: Expected StorageHelperImage=%v but found %v",
					i, tc.expectedStorageHelperImage, conf.StorageHelperImage.TaggedImage())
			}
		}

		if tc.expectedModelMeshImage != "" {
			if conf.ModelMeshImage.TaggedImage() != tc.expectedModelMeshImage {
				t.Errorf("Failed test case [%d]: Expected ModelMeshImage=%v but found %v",
					i, tc.expectedModelMeshImage, conf.ModelMeshImage.TaggedImage())
			}
		}
	}
}

func TestNewMergedConfigFromStringWithDigest(t *testing.T) {
	yaml := `
storageHelperImage:
  name: override
  tag: 1.0
  digest: sha256:97399986727cc54cae86f09fb22b1ca31793ad3ca7b73caaef1ed70bfcf42c6a`

	expectedStorageHelperImage := "override@sha256:97399986727cc54cae86f09fb22b1ca31793ad3ca7b73caaef1ed70bfcf42c6a"
	expectedPodsPerRuntime := uint16(2)

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	//Verify system config map default
	if conf.PodsPerRuntime != expectedPodsPerRuntime {
		t.Fatalf("Expected PodsPerDeployment=%v but found %v", expectedPodsPerRuntime, conf.PodsPerRuntime)
	}

	//Verify user override
	if conf.StorageHelperImage.TaggedImage() != expectedStorageHelperImage {
		t.Fatalf("Expected StorageHelperImage=%v but found %v",
			expectedStorageHelperImage, conf.StorageHelperImage.TaggedImage())
	}
}

func TestNewMergedConfigFromStringFailures(t *testing.T) {
	invalidConfigs := []string{
		// wrong type
		`
podsPerRuntime: "none"`,
		// nested value wronge type
		`
modelMeshResources:
  requests:
    cpu: "30"
    memory: "asdf"`,
	}

	for i, yaml := range invalidConfigs {

		_, err := NewMergedConfigFromString(yaml)
		if err == nil {
			t.Fatalf("Expected error for test case [%d], but did not get one", i)
		}
	}
}

func TestMetricsEnabling(t *testing.T) {
	yaml := `
metrics:
  enabled: true`

	expectedMetricsEnabledValue := true

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	//Verify system config map default
	if conf.Metrics.Enabled != expectedMetricsEnabledValue {
		t.Fatalf("Expected MerticsEnabled=%v but found %v", expectedMetricsEnabledValue, conf.Metrics.Enabled)
	}
}

func TestGrpcMaxMessageSize(t *testing.T) {
	yaml := `
grpcMaxMessageSizeBytes: 33554432`

	expectedGrpcMessageSize := 33554432

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	//Verify system config map default
	if conf.GrpcMaxMessageSizeBytes != expectedGrpcMessageSize {
		t.Fatalf("Expected GrpcMaxMessageSizeBytes=%v but found %v", expectedGrpcMessageSize, conf.GrpcMaxMessageSizeBytes)
	}
}

func TestBuiltInServerTypes(t *testing.T) {
	yaml := `
builtInServerTypes:
 - triton
 - mlserver
 - ovms
 - a_new_one`

	expectedTypes := []string{"triton", "mlserver", "ovms", "a_new_one"}

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedTypes, conf.BuiltInServerTypes) {
		t.Fatalf("Expected BuiltInServerTypes=%v but found %v", expectedTypes, conf.BuiltInServerTypes)
	}
}

func TestResourceRequirements(t *testing.T) {
	rr := ResourceRequirements{
		Requests: ResourceQuantities{
			CPU:    "15000m",
			Memory: "512Mi",
		},
		Limits: ResourceQuantities{
			CPU:    "1000",
			Memory: "5Gi",
		},
	}

	// this should validate
	err := rr.parseAndValidate()
	if err != nil {
		t.Fatal(err)
	}
	// and now should convert as expected
	actual := rr.ToKubernetesType()
	// expected uses different, but equivalent resource strings
	expected := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000"),
			corev1.ResourceMemory: resource.MustParse("5120Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("15"),
			corev1.ResourceMemory: resource.MustParse("0.5Gi"),
		},
	}

	if !cmp.Equal(actual.Limits.Cpu(), expected.Limits.Cpu()) {
		t.Error("limits.cpu did not compare equal.")
	}
	if !cmp.Equal(actual.Limits.Memory(), expected.Limits.Memory()) {
		t.Error("limits.memory did not compare equal.")
	}

	if !cmp.Equal(actual.Requests.Cpu(), expected.Requests.Cpu()) {
		t.Error("requests.cpu did not compare equal.")
	}
	if !cmp.Equal(actual.Requests.Memory(), expected.Requests.Memory()) {
		t.Error("requests.memory did not compare equal.")
	}

	// now if we change a field to an invalid value
	rr.Limits.CPU = "invalid"
	// it should not parse
	err2 := rr.parseAndValidate()
	if err2 == nil {
		t.Fatal("Expected a parse error")
	}
}

func TestInternalModelMeshEnvVars(t *testing.T) {
	yaml := `
internalModelMeshEnvVars:
  - name: "BOOTSTRAP_CLEARANCE_PERIOD_MS"
    value: "0"
`
	expectedEnvVar := "BOOTSTRAP_CLEARANCE_PERIOD_MS"
	expectedValue := "0"

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	envvar := conf.InternalModelMeshEnvVars.ToKubernetesType()[0]
	if envvar.Name != expectedEnvVar {
		t.Fatalf("Expected InternalModelMeshEnvVars to have env var with key [%s], but got [%s]", expectedEnvVar, envvar.Name)
	}

	if envvar.Value != expectedValue {
		t.Fatalf("Expected InternalModelMeshEnvVars to have env var with value [%s], but got [%s]", expectedValue, envvar.Value)
	}
}

func TestImagePullSecrets(t *testing.T) {
	yaml := `
imagePullSecrets:
  - name: "config-image-pull-secret"
`
	expectedSecretName := "config-image-pull-secret"

	conf, err := NewMergedConfigFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	secret := conf.ImagePullSecrets[0]
	if secret.Name != expectedSecretName {
		t.Fatalf("Expected ImagePullSecrets to have secret with name [%s], but got [%s]", expectedSecretName, secret.Name)
	}
}
