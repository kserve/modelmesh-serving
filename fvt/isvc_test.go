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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FVTInferenceService struct {
	name                     string
	inferenceServiceFileName string
}

var inferenceArray = []FVTInferenceService{
	{
		name:                     "New Format",
		inferenceServiceFileName: "new-format-mm.yaml",
	},
	{
		name:                     "Old Format",
		inferenceServiceFileName: "old-format-mm.yaml",
	},
}

var _ = Describe("Inference service", func() {
	// starting from the desired state.
	Specify("Preparing the cluster for inference service tests", func() {
		// ensure configuration has scale-to-zero disabled
		config := map[string]interface{}{
			// disable scale-to-zero to prevent pods flapping as
			// Predictors are created and deleted
			"scaleToZero": map[string]interface{}{
				"enabled": false,
			},
			// disable the model-mesh bootstrap failure check so
			// that the expected failures for invalid
			// tests do not trigger it
			"internalModelMeshEnvVars": []map[string]interface{}{
				{
					"name":  "BOOTSTRAP_CLEARANCE_PERIOD_MS",
					"value": "0",
				},
			},
			"podsPerRuntime": 1,
		}
		fvtClient.ApplyUserConfigMap(config)

		// ensure that there are no predictors to start
		fvtClient.DeleteAllIsvcs()

		// ensure a stable deploy state
		WaitForStableActiveDeployState()
	})
	for _, i := range inferenceArray {
		var _ = Describe("test "+i.name+" isvc", func() {
			It("should successfully load a model", func() {
				isvcObject := NewIsvcForFVT(i.inferenceServiceFileName)
				CreateIsvcAndWaitAndExpectReady(isvcObject)
				// clean up
				fvtClient.DeleteIsvc(isvcObject.GetName())
			})

			var _ = Describe("MLServer inference", func() {

				var mlsIsvcObject *unstructured.Unstructured
				var mlsISVCName string

				BeforeEach(func() {
					// load the test predictor object
					mlsIsvcObject = NewIsvcForFVT(i.inferenceServiceFileName)
					mlsISVCName = mlsIsvcObject.GetName()

					CreateIsvcAndWaitAndExpectReady(mlsIsvcObject)

					err := fvtClient.ConnectToModelServing(Insecure)
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					fvtClient.DeleteIsvc(mlsISVCName)
					fvtClient.DisconnectFromModelServing()
				})

				It("should successfully run inference", func() {
					ExpectSuccessfulInference_sklearnMnistSvm(mlsISVCName)
				})
			})
		})
	}
})
