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
	"time"

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

var _ = Describe("ISVCs", func() {

	Describe("with PVC in storage-config", Ordered, func() {

		for _, name := range isvcWithPvcInStorageConfig {

			Describe("\""+name+"\"", Ordered, func() {
				var isvcName = name
				var fileName = isvcFiles[name]

				AfterAll(func() {
					FVTClientInstance.DeleteIsvc(isvcName)
				})

				It("should successfully load a model", func() {
					isvcObject := NewIsvcForFVT(fileName)
					isvcName = isvcObject.GetName()
					CreateIsvcAndWaitAndExpectReady(isvcObject, PredictorTimeout)
				})

				It("should successfully run inference", func() {
					ExpectSuccessfulInference_sklearnMnistSvm(isvcName)
				})
			})
		}
	})

	Describe("with PVC not in storage-config", Ordered, Serial, func() {
		var isvcObject *unstructured.Unstructured

		It("should fail with PVC not mounted", func() {
			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithPvcNotInStorageConfig])

			obj := CreateIsvcAndWaitAndExpectFailed(isvcObject)

			By("Asserting on the ISVC state")
			ExpectIsvcFailureInfo(obj, "ModelLoadFailed", true, true, "")

			FVTClientInstance.DeleteIsvc(isvcObject.GetName())
		})

		It("should load a model when allowAnyPVC", func() {
			// This ISVC needs a new PVC which is not in the storage-config secret.
			// The controller will update the deployment with the pvc_mount, but
			// if the old runtime pods are still around, the ISVC will get deployed
			// on an old runtime pod without the PVC mounted and keep failing
			// until a new pod with the PVC is ready and the controller decides
			// to move the ISVC onto the new pod that has the PVC mounted.
			// If this process takes a long time so, we may want to take some extra
			// measures to increase our chances for quick success:
			// - enable scale to zero? to prevent the new ISVC to land on an old
			//   runtime pod that does not have the "any" PVC mounted yet
			// - use more than 1 pod per runtime? assumption: controller keeps the
			//   old runtime pod (without the PVC mounted) around (pre-stop hook,
			//   terminationGracePeriodSeconds: 90 # to allow for model propagation)
			//   since it still has the ISVC on it but no new pod yet to place it
			//   on. With 2 runtime pods, one pod can get updated with the PVC mount
			//   and the controller can "move" the ISVC without service interruption
			//   from the old to the new pod

			// make a shallow copy of default configmap (don't modify the DefaultConfig reference)
			// keeping 1 pod per runtime and don't scale to 0
			config := make(map[string]interface{})
			for k, v := range DefaultConfig {
				config[k] = v
			}
			// update the model-serving-config to allow any PVC
			config["allowAnyPVC"] = true

			By("Updating the user config to allow any PVC")
			FVTClientInstance.ApplyUserConfigMap(config)

			By("Deleting any PVC entries from the storage-config secret")
			FVTClientInstance.DeleteStorageConfigSecret()

			// TODO: create a separate test case for not having storage secret at all.
			// Currently that is not working. Runtime pod events without it:
			// ---
			// TYPE     REASON       MESSAGE
			// Normal   Scheduled    Successfully assigned modelmesh-serving/modelmesh-serving-mlserver-0.x-54685b95d5-6xmck to 10.87.76.74
			// Warning  FailedMount  Unable to attach or mount volumes: unmounted volumes=[storage-config], unattached volumes=[models-dir models-pvc-3 storage-config tc-config etcd-config kube-api-access-pqz7t]: timed out waiting for the condition
			// Warning  FailedMount  MountVolume.SetUp failed for volume "storage-config" : secret "storage-config" not found
			// ---
			// recreate the storage-config secret without the PVCs
			FVTClientInstance.CreateStorageConfigSecret(StorageConfigDataMinio)

			// after changing the storage-config, the runtime pod(s) restart with
			// updated PVC mounts, wait for stability
			By("Waiting for stable deploy state")
			WaitForStableActiveDeployState(time.Second * 30)

			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithPvcNotInStorageConfig])

			// print pods before deploying the predictor for debugging purposes
			FVTClientInstance.PrintPods()

			// after mounting the new PVC the runtime pod(s) restart again, but
			// the ISVC could have landed on the previous runtime pod without the
			// PVC mounted yet and hence will fail to load the first time.
			// So we extend the standard predictor timeout to wait a bit longer
			extendedTimeout := PredictorTimeout * 2
			CreateIsvcAndWaitAndExpectReady(isvcObject, extendedTimeout)

			// print pods after the predictor is ready for debugging purposes
			FVTClientInstance.PrintPods()

			// after adding predictor with allowAnyPVC, new pods get created with
			// the new pvc mounts, so we want to wait for deployments to stabilize
			// in order to avoid connecting to terminating pod which causes the
			// port-forward got killed and needs to be re-established (rarely happens)
			By("Waiting for stable deploy state before connecting gRPC connection")
			WaitForStableActiveDeployState(time.Second * 10)
			By("Connecting to model serving service")
			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())

			isvcName := isvcObject.GetName()
			By("Running an inference request")
			ExpectSuccessfulInference_sklearnMnistSvm(isvcName)

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

			By("Waiting for stable deploy state")
			WaitForStableActiveDeployState(time.Second * 30)

			isvcObject = NewIsvcForFVT(isvcFiles[isvcWithNonExistentPvc])

			obj := CreateIsvcAndWaitAndExpectFailed(isvcObject)
			ExpectIsvcFailureInfo(obj, "ModelLoadFailed", true, true, "")

			FVTClientInstance.DeleteIsvc(isvcObject.GetName())
		})
	})
})
