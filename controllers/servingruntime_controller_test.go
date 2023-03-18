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
	"fmt"
	"path/filepath"
	"time"

	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func waitForAndGetRuntimeDeployment(runtimeName string) *appsv1.Deployment {
	var err error

	serviceName := reconcilerConfig.InferenceServiceName
	dName := fmt.Sprintf("%s-%s", serviceName, runtimeName)
	dKey := types.NamespacedName{Name: dName, Namespace: namespace}
	d := &appsv1.Deployment{}
	// poll while waiting for the Deployment
	for i := 1; i < 5; i++ {
		time.Sleep(1 * time.Second)
		err = k8sClient.Get(context.TODO(), dKey, d)
		if !errors.IsNotFound(err) {
			break
		}
	}
	Expect(err).ToNot(HaveOccurred())

	return d
}

var _ = Describe("Sample Runtime", func() {
	samplesToTest := []string{
		"config/runtimes/mlserver-0.x.yaml",
		"config/runtimes/triton-2.x.yaml",
		"config/runtimes/ovms-1.x.yaml",
		"config/runtimes/torchserve-0.x.yaml",
	}
	for _, f := range samplesToTest {
		// capture the value in new variable for each iteration
		sampleFilename := f
		Context(sampleFilename, func() {
			It("should be a valid runtime specification", func() {
				var m mf.Manifest
				var err error

				By("create the runtime")
				m, err = mf.ManifestFrom(mf.Path(filepath.Join("..", sampleFilename)))
				m.Client = mfc.NewClient(k8sClient)
				Expect(err).ToNot(HaveOccurred())
				m, err = m.Transform(convertToServingRuntime, mf.InjectNamespace(namespace))
				Expect(err).ToNot(HaveOccurred())
				err = m.Apply()
				Expect(err).ToNot(HaveOccurred())

				By("wait for the Deployment to be created")
				// generate the expected Name of the Deployment
				srName := m.Resources()[0].GetName()
				d := waitForAndGetRuntimeDeployment(srName)

				By("compare the generated Deployment")
				// discard objectmeta before snapshot compare to remove generated values
				d.ObjectMeta = metav1.ObjectMeta{}
				Expect(d).To(SnapshotMatcher())
			})
		})
	}
})

var _ = Describe("Prometheus metrics configuration", func() {
	var m mf.Manifest
	var err error

	It("should enable model-mesh metrics", func() {
		By("enable metrics in the config")
		reconcilerConfig.Metrics.Enabled = true

		By("create a sample runtime")
		m, err = mf.ManifestFrom(mf.Path("../config/runtimes/mlserver-0.x.yaml"))
		m.Client = mfc.NewClient(k8sClient)
		Expect(err).ToNot(HaveOccurred())
		m, err = m.Transform(convertToServingRuntime, mf.InjectNamespace(namespace))
		Expect(err).ToNot(HaveOccurred())
		err = m.Apply()
		Expect(err).ToNot(HaveOccurred())

		By("check the generated Deployment")
		srName := m.Resources()[0].GetName()
		d := waitForAndGetRuntimeDeployment(srName)
		// discard objectmeta before snapshot compare to remove generated values
		d.ObjectMeta = metav1.ObjectMeta{}
		Expect(d).To(SnapshotMatcher())
	})
})

var _ = It("Replicas field should override podsPerRuntime", func() {
	var m mf.Manifest
	var err error

	By("enable metrics in the config")
	reconcilerConfig.PodsPerRuntime = 1

	By("create a test runtime with replicas set")
	m, err = mf.ManifestFrom(mf.Path("./testdata/test-servingruntime.yaml"))
	m.Client = mfc.NewClient(k8sClient)
	Expect(err).ToNot(HaveOccurred())
	m, err = m.Transform(
		mf.InjectNamespace(namespace),
		// set spec.replicas
		func(u *unstructured.Unstructured) error {
			// value must be cast to int64 to avoid "cannot deep copy" errors
			return unstructured.SetNestedField(u.Object, int64(5), "spec", "replicas")
		},
	)
	Expect(err).ToNot(HaveOccurred())
	err = m.Apply()
	Expect(err).ToNot(HaveOccurred())

	By("check the generated Deployment")
	srName := m.Resources()[0].GetName()
	d := waitForAndGetRuntimeDeployment(srName)
	// check the value of the replicas field directly
	Expect(*d.Spec.Replicas).To(BeEquivalentTo(5))
})

var _ = Describe("Serving runtime with storageHelper disabled", func() {
	var m mf.Manifest
	var err error

	It("deployment should not contain puller container", func() {
		By("create a sample runtime")
		m, err = mf.ManifestFrom(mf.Path("../config/samples/servingruntime_pullerless.yaml"))
		m.Client = mfc.NewClient(k8sClient)
		Expect(err).ToNot(HaveOccurred())
		m, err = m.Transform(mf.InjectNamespace(namespace))
		Expect(err).ToNot(HaveOccurred())
		err = m.Apply()
		Expect(err).ToNot(HaveOccurred())

		By("check the generated Deployment")
		srName := m.Resources()[0].GetName()
		d := waitForAndGetRuntimeDeployment(srName)
		// discard objectmeta before snapshot compare to remove generated values
		d.ObjectMeta = metav1.ObjectMeta{}
		Expect(d).To(SnapshotMatcher())
	})
})

var _ = Describe("REST Proxy configuration", func() {
	var m mf.Manifest
	var err error

	It("deployment should contain REST Proxy container and extra v2 protocol label", func() {
		By("enable REST Proxy in the config")
		reconcilerConfig.RESTProxy.Enabled = true

		By("create a sample runtime")
		m, err = mf.ManifestFrom(mf.Path("../config/runtimes/mlserver-0.x.yaml"))
		m.Client = mfc.NewClient(k8sClient)
		Expect(err).ToNot(HaveOccurred())
		m, err = m.Transform(convertToServingRuntime, mf.InjectNamespace(namespace))
		Expect(err).ToNot(HaveOccurred())
		err = m.Apply()
		Expect(err).ToNot(HaveOccurred())

		By("check the generated Deployment")
		srName := m.Resources()[0].GetName()
		d := waitForAndGetRuntimeDeployment(srName)

		// discard objectmeta before snapshot compare to remove generated values
		d.ObjectMeta = metav1.ObjectMeta{}
		Expect(d).To(SnapshotMatcher())
	})
})

var _ = Describe("Add Payload Processor", func() {
	var m mf.Manifest
	var err error

	It("deployment should contain Payload Processor", func() {
		By("add payload processor to yaml config")
		resetToPayloadConfig(false)

		By("create a sample runtime")
		m, err = mf.ManifestFrom(mf.Path("../config/runtimes/mlserver-0.x.yaml"))
		m.Client = mfc.NewClient(k8sClient)
		Expect(err).ToNot(HaveOccurred())
		m, err = m.Transform(convertToServingRuntime, mf.InjectNamespace(namespace))
		Expect(err).ToNot(HaveOccurred())
		err = m.Apply()
		Expect(err).ToNot(HaveOccurred())

		By("check the generated Deployment")
		srName := m.Resources()[0].GetName()
		d := waitForAndGetRuntimeDeployment(srName)

		// discard objectmeta before snapshot compare to remove generated values
		d.ObjectMeta = metav1.ObjectMeta{}
		Expect(d).To(SnapshotMatcher())
	})
})

var _ = Describe("Add Payload Processor", func() {
	It("deployment should raise an error when the processor contains a space", func() {
		By("try to parse a config yaml that contains a space in a processor")
		// this function expects an error
		resetToPayloadConfig(true)
	})
})

func convertToServingRuntime(resource *unstructured.Unstructured) error {
	if resource.GetKind() == "ClusterServingRuntime" {
		resource.SetKind("ServingRuntime")
	}
	return nil
}
