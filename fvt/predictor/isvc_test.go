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

package predictor

import (
	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("Inference service", Ordered, func() {
	for _, i := range inferenceArray {
		var _ = Describe("test "+i.name+" isvc", Ordered, func() {
			var isvcName string

			It("should successfully load a model", func() {
				isvcObject := NewIsvcForFVT(i.inferenceServiceFileName)
				isvcName = isvcObject.GetName()
				CreateIsvcAndWaitAndExpectReady(isvcObject)

				err := FVTClientInstance.ConnectToModelServing(Insecure)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully run inference", func() {
				ExpectSuccessfulInference_sklearnMnistSvm(isvcName)
			})

			AfterAll(func() {
				FVTClientInstance.DeleteIsvc(isvcName)
			})

		})
	}
})
