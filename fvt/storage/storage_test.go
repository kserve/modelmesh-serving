// Copyright 2023 IBM Corporation
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
// limitations under the License.package storage

package storage

import (
	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var isvcFiles = map[string]string{
	"isvc-pvc-storage-uri":  "isvc-pvc-uri.yaml",
	"isvc-pvc-storage-path": "isvc-pvc-path.yaml",
	"isvc-pvc2":             "isvc-pvc-2.yaml",
	"isvc-pvc3":             "isvc-pvc-3.yaml",
	"isvc-pvc4":             "isvc-pvc-4.yaml",
}

// ISVCs using PVCs from the FVT `storage-config` Secret (config/dependencies/fvt.yaml)
var isvcWithPvcInStorageConfig = []string{"isvc-pvc-storage-uri", "isvc-pvc-storage-path", "isvc-pvc2"}

// ISVC using PVC not in the FVT `storage-config` Secret (config/dependencies/fvt.yaml)
// this should work only after setting allowAnyPVC = true
var isvcWithPvcNotInStorageConfig = "isvc-pvc3"

// ISVC using a PVC that does not exist at all, this ISVC should fail to load
var isvcWithNonExistentPvc = "isvc-pvc4"

var _ = Describe("ISVCs", Ordered, func() {

	Describe("with PVC in storage-config", Ordered, func() {

		for _, name := range isvcWithPvcInStorageConfig {

			Describe("\""+name+"\"", Ordered, func() {
				var isvcName = name
				var fileName = isvcFiles[name]

				It("should successfully load a model", func() {
					isvcObject := NewIsvcForFVT(fileName)
					isvcName = isvcObject.GetName()
					CreateIsvcAndWaitAndExpectReady(isvcObject, PredictorTimeout)
				})

				It("should successfully run inference", func() {
					ExpectSuccessfulInference_sklearnMnistSvm(isvcName)
				})

				BeforeEach(func() {
					WaitForStableActiveDeployState()
				})

				BeforeAll(func() {
					err := FVTClientInstance.ConnectToModelServing(Insecure)
					Expect(err).ToNot(HaveOccurred())
				})

				AfterAll(func() {
					FVTClientInstance.DeleteIsvc(isvcName)
					FVTClientInstance.DisconnectFromModelServing()
				})

			})
		}
	})

	Describe("with PVC not in storage-config", Ordered, func() {
		var isvcObject *unstructured.Unstructured

		It("should fail with PVC not mounted", func() {
			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithPvcNotInStorageConfig])

			obj := CreateIsvcAndWaitAndExpectFailed(isvcObject)

			By("Asserting on the ISVC state")
			ExpectIsvcFailureInfo(obj, "ModelLoadFailed", true, true, "")

			FVTClientInstance.DeleteIsvc(isvcObject.GetName())
		})

		It("should load a model when allowAnyPVC", FlakeAttempts(3), func() {
			// This ISVC needs a new PVC which is not in the storage-config secret.
			// The controller will update the deployment with the pvc_mount, but
			// if the old runtime pods are still around, the ISVC will get deployed
			// on an old runtime pod without the PVC mounted and keep failing
			// until a new pod with the PVC is ready and the controller finally
			// decides to move the ISVC onto the new pod that has the PVC mounted.
			// However, this process can take a long time (how long?) so, we take
			// some extra measures to increase our chances for quick success:
			// - scale to 0 prohibits the new ISVC to land on an old runtime pod
			//   that does not have the "any" PVC mounted yet
			// - use more than 1 pod per runtime so controller will not kill new
			//   pods that have the PVC mounted but because the ISVC is loaded on
			//   the old pod (without the PVC) but the old pod gets kept around
			//   instead of the new because the ISVC is still on there -- even
			//   though its failing
			// - allowAnyPVC needs rest-proxy enabled (not sure why)
			config := map[string]interface{}{
				"allowAnyPVC":    true,
				"podsPerRuntime": 1,
				"scaleToZero": map[string]interface{}{
					"enabled": true,
				},
				"restProxy": map[string]interface{}{
					"enabled": true,
				},
			}
			By("Updating the user config to allow any PVC")
			FVTClientInstance.ApplyUserConfigMap(config)

			// after applying configmap, the runtime pod(s) restart, wait for stability
			By("Waiting for stable deploy state")
			WaitForStableActiveDeployState()

			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithPvcNotInStorageConfig])

			FVTClientInstance.PrintPods()

			// after mounting the new PVC the runtime pod(s) restart again, but the ISVC
			// if not scaleToZero, it could have landed on the previous runtime pod will
			// fail to load the first time, so we extend the standard predictor timeout
			extendedTimeout := PredictorTimeout * 2
			CreateIsvcAndWaitAndExpectReady(isvcObject, extendedTimeout)

			FVTClientInstance.PrintPods()

			// since the runtime pod(s) restarted twice, but (some of) the old runtime pods
			// are lingering around (Terminating) we may have gotten a defunct connection
			// after applying configmap, the runtime pod(s) restart, wait for stability
			WaitForStableActiveDeployState()

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())

			isvcName := isvcObject.GetName()
			ExpectSuccessfulInference_sklearnMnistSvm(isvcName)

			FVTClientInstance.DisconnectFromModelServing()
			FVTClientInstance.DeleteIsvc(isvcObject.GetName())
		})

		It("should fail with non-existent PVC", func() {
			// make a shallow copy of default configmap (don't modify the DefaultConfig reference)
			// keeping 1 pod per runtime and don't scale to 0
			config := make(map[string]interface{})
			for k, v := range DefaultConfig {
				config[k] = v
			}
			// update the model-serving-config to allow any PVC
			config["allowAnyPVC"] = true
			FVTClientInstance.ApplyUserConfigMap(config)

			// TODO: runtime pods will remain pending with unbound PVC, don't wait, need controller fix
			//By("Waiting for stable deploy state")
			//WaitForStableActiveDeployState()

			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithNonExistentPvc])

			CreateIsvcAndWaitAndExpectFailed(isvcObject)
			// TODO: ISVC model will remain pending until controller fix
			//ExpectIsvcFailureInfo(obj, "ModelLoadFailed", true, true, "")

			FVTClientInstance.DeleteIsvc(isvcObject.GetName())
		})
	})
})
