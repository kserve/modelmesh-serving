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
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/dereklstinson/cifar"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"
	"github.com/moverest/mnist"
)

// Predictor struct - used to store data about the predictor that FVT suite can use
// Here, predictorName is the predictor that is being tested.
// And differentPredictorName is another predictor that is paired up with the predictor being tested
//
// For example - if an onnx model is being tested in this suite, predictorName will have onnx in it.
// and for the test case where 2 models of different types are loaded, the second model could be pytorch
// hence two different predictors in one FVTPredictor struct

type FVTPredictor struct {
	predictorName              string
	predictorFilename          string
	currentModelPath           string
	updatedModelPath           string
	schemaPath                 string
	differentPredictorName     string
	differentPredictorFilename string
}

// relative path of the predictor sample files
const samplesPath string = "testdata/predictors/"
const userConfigMapName string = "model-serving-config"

// Used for checking if floats are sufficiently close enough.
const EPSILON float64 = 0.000001

var xgBoostInputData []float32 = []float32{1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0}

// Array of all the predictors that need to be tested
var predictorsArray = []FVTPredictor{
	{
		predictorName:              "tf",
		predictorFilename:          "tf-predictor.yaml",
		currentModelPath:           "fvt/tensorflow/mnist.savedmodel",
		updatedModelPath:           "fvt/tensorflow/mnist-dup.savedmodel",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
	{
		predictorName:              "onnx",
		predictorFilename:          "onnx-predictor.yaml",
		currentModelPath:           "fvt/onnx/onnx-mnist",
		updatedModelPath:           "fvt/onnx/onnx-mnist-new",
		differentPredictorName:     "pytorch",
		differentPredictorFilename: "pytorch-predictor.yaml",
	},
	{
		predictorName:              "onnx-withschema",
		predictorFilename:          "onnx-predictor-withschema.yaml",
		currentModelPath:           "fvt/onnx/onnx-withschema",
		updatedModelPath:           "fvt/onnx/onnx-withschema-new",
		schemaPath:                 "fvt/onnx/schema/schema.json",
		differentPredictorName:     "pytorch",
		differentPredictorFilename: "pytorch-predictor.yaml",
	},
	{
		predictorName:              "pytorch",
		predictorFilename:          "pytorch-predictor.yaml",
		currentModelPath:           "fvt/pytorch/pytorch-cifar",
		updatedModelPath:           "fvt/pytorch/pytorch-cifar-new",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
	{
		predictorName:              "xgboost",
		predictorFilename:          "xgboost-predictor.yaml",
		currentModelPath:           "fvt/xgboost/mushroom",
		updatedModelPath:           "fvt/xgboost/mushroom-dup",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
	{
		predictorName:              "lightgbm",
		predictorFilename:          "lightgbm-predictor.yaml",
		currentModelPath:           "fvt/lightgbm/mushroom",
		updatedModelPath:           "fvt/lightgbm/mushroom-dup",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
}

var _ = Describe("Predictor", func() {
	// Many tests in this block assume a stable state of scaled up deployments
	// which may not be the case if other Describe blocks run first. So we want to
	// confirm the expected state before executing any test, but we also don't
	// want to check the deployment state for each test since that would waste
	// time. The sole purpose of the following test case is to ensure we are
	// starting from the desired state.
	Specify("Preparing the cluster for Predictor tests", func() {
		// ensure configuration has scale-to-zero disabled
		config := map[string]interface{}{
			"scaleToZero": map[string]interface{}{
				"enabled": false,
			},
		}
		fvtClient.ApplyUserConfigMap(config)

		// ensure that there are no predictors to start
		fvtClient.DeleteAllPredictors()

		// ensure a stable deploy state
		watcher := fvtClient.StartWatchingDeploys(servingRuntimeDeploymentsListOptions)
		defer watcher.Stop()
		WaitForStableActiveDeployState(watcher)
	})

	for _, p := range predictorsArray {
		predictor := p
		var _ = Describe("create "+predictor.predictorName+" predictor", func() {
			var predictorObject *unstructured.Unstructured
			var predictorName string
			var differentPredictorObject *unstructured.Unstructured
			var differentPredictorName string
			var startTime string

			BeforeEach(func() {
				// verify clean state (no predictors)
				list := fvtClient.ListPredictors(metav1.ListOptions{})
				Expect(list.Items).To(BeEmpty())

				// load the test predictor object
				predictorObject = DecodeResourceFromFile(samplesPath + predictor.predictorFilename)
				predictorName = GetString(predictorObject, "metadata", "name")
				differentPredictorObject = DecodeResourceFromFile(samplesPath + predictor.differentPredictorFilename)
				differentPredictorName = GetString(differentPredictorObject, "metadata", "name")

				// update if schema is not empty
				if predictor.schemaPath != "" {
					SetString(predictorObject, predictor.schemaPath, "spec", "schemaPath")
				}
			})

			AfterEach(func() {
				if CurrentGinkgoTestDescription().Failed {
					fvtClient.PrintPredictors()
					fvtClient.TailPodLogs(startTime)
				}
				fvtClient.DeleteAllPredictors()
			})

			It("should successfully load a model", func() {
				By("Creating a " + predictor.predictorName + " predictor")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'Loaded'")
				// TODO: "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be (see issue#994)
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)

				By("Listing the predictor")
				obj = ListAllPredictorsExpectOne()
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, true, "Loaded", "", "UpToDate")

			})

			It("should successfully load two models of different types", func() {
				By("Creating the " + predictor.predictorName + " predictor")
				predictorWatcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, defaultTimeout)
				defer predictorWatcher.Stop()
				predictorObject = fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(predictorObject, "metadata", "creationTimestamp")
				ExpectPredictorState(predictorObject, false, "Pending", "", "UpToDate")

				By("Creating the " + predictor.differentPredictorName + " predictor")
				differentPredictorWatcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + differentPredictorName}, defaultTimeout)
				defer differentPredictorWatcher.Stop()
				differentPredictorObject = fvtClient.CreatePredictorExpectSuccess(differentPredictorObject)
				ExpectPredictorState(differentPredictorObject, false, "Pending", "", "UpToDate")

				By("Waiting for the first predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, predictorWatcher)
				By("Waiting for the second predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, differentPredictorWatcher)

				By("Listing the predictors")
				predictorObject = ListPredictorsByNameExpectOne(predictorName)
				ExpectPredictorState(predictorObject, true, "Loaded", "", "UpToDate")
				differentPredictorObject = ListPredictorsByNameExpectOne(differentPredictorName)
				ExpectPredictorState(differentPredictorObject, true, "Loaded", "", "UpToDate")
			})

			It("should successfully load two models of the same type", func() {
				By("Creating the first " + predictor.predictorName + " predictor")
				name1 := "minimal-" + predictor.predictorName + "-predictor1"
				SetString(predictorObject, name1, "metadata", "name")
				watcher1 := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + name1}, defaultTimeout)
				defer watcher1.Stop()
				obj1 := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj1, "metadata", "creationTimestamp")
				ExpectPredictorState(obj1, false, "Pending", "", "UpToDate")

				By("Creating the second " + predictor.predictorName + " predictor")
				name2 := "minimal-" + predictor.predictorName + "-predictor2"
				SetString(predictorObject, name2, "metadata", "name")
				watcher2 := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + name2}, defaultTimeout)
				defer watcher2.Stop()
				obj2 := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				ExpectPredictorState(obj2, false, "Pending", "", "UpToDate")

				By("Waiting for the first predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher1)
				By("Waiting for the second predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher2)

				By("Listing the predictors")
				obj1 = ListPredictorsByNameExpectOne(name1)
				ExpectPredictorState(obj1, true, "Loaded", "", "UpToDate")
				obj2 = ListPredictorsByNameExpectOne(name2)
				ExpectPredictorState(obj2, true, "Loaded", "", "UpToDate")
			})

		})

		var _ = Describe("update "+predictor.predictorName+" predictor", func() {
			var predictorObject *unstructured.Unstructured
			var predictorName string
			var startTime string

			BeforeEach(func() {
				// verify clean state (no predictors)
				list := fvtClient.ListPredictors(metav1.ListOptions{})
				Expect(list.Items).To(BeEmpty())

				// load the test predictor object
				predictorObject = DecodeResourceFromFile(samplesPath + predictor.predictorFilename)
				predictorName = GetString(predictorObject, "metadata", "name")

				// update if schema is not empty
				if predictor.schemaPath != "" {
					SetString(predictorObject, predictor.schemaPath, "spec", "schemaPath")
				}

				// create the predictor
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				createdPredictor := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
				By("Waiting for the predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
				resourceVersion := GetString(obj, "metadata", "resourceVersion")
				SetString(predictorObject, resourceVersion, "metadata", "resourceVersion")

			})

			AfterEach(func() {
				if CurrentGinkgoTestDescription().Failed {
					fvtClient.PrintPredictors()
					fvtClient.TailPodLogs(startTime)
				}
				fvtClient.DeleteAllPredictors()
			})

			It("should successfully update and reload the model", func() {
				// verify starting model path
				obj := ListAllPredictorsExpectOne()
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.currentModelPath))

				// modify the object with a new valid path
				SetString(predictorObject, predictor.updatedModelPath, "spec", "path")

				By("Updating the predictor with new model path")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				fvtClient.UpdatePredictorExpectSuccess(predictorObject)

				By("Waiting for the Predictor's target model state to move from Loaded (empty) to Loading")
				loadingObj := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading"}, watcher)
				Expect(loadingObj.GetName()).To(Equal(predictorName))
				Expect(GetString(loadingObj, "spec", "path")).To(Equal(predictor.updatedModelPath))
				ExpectPredictorState(loadingObj, true, "Loaded", "Loading", "InProgress")

				By("Waiting for the predictor to be 'Loaded'")
				loadedObj := WaitForLastStateInExpectedList("targetModelState", []string{"Loading", "Loaded", ""}, watcher)
				ExpectPredictorState(loadedObj, true, "Loaded", "", "UpToDate")
				watcher.Stop()

				// get the object with List and verify it one more time
				By("Listing the predictors")
				obj = ListAllPredictorsExpectOne()
				Expect(obj.GetName()).To(Equal(predictorName))
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.updatedModelPath))
				ExpectPredictorState(obj, true, "Loaded", "", "UpToDate")
			})

			It("should fail to load the target model with invalid path", func() {
				// verify starting model path
				obj := ListAllPredictorsExpectOne()
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.currentModelPath))

				// modify the object with a new valid path
				SetString(predictorObject, "invalid/path", "spec", "path")

				By("Updating the predictor with invalid model path")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				fvtClient.UpdatePredictorExpectSuccess(predictorObject)

				// watch for the 'Failed' targetModelState
				By("Waiting for the predictor to be 'FailedToLoad'")
				failedObj := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading", "FailedToLoad"}, watcher)
				watcher.Stop()

				Expect(failedObj.GetName()).To(Equal(predictorName))
				Expect(GetString(failedObj, "spec", "path")).To(Equal("invalid/path"))
				ExpectPredictorState(failedObj, true, "Loaded", "FailedToLoad", "BlockedByFailedLoad")
				ExpectPredictorFailureInfo(failedObj, "ModelLoadFailed", true, true, "")

				// get the object with List and verify it one more time
				By("Listing the predictors")
				obj = ListAllPredictorsExpectOne()
				Expect(obj.GetName()).To(Equal(predictorName))
				Expect(GetString(obj, "spec", "path")).To(Equal("invalid/path"))
				ExpectPredictorState(failedObj, true, "Loaded", "FailedToLoad", "BlockedByFailedLoad")
				ExpectPredictorFailureInfo(failedObj, "ModelLoadFailed", true, true, "")
			})
		})

	}

	var _ = Describe("test transition of Predictor between models", func() {
		var predictorObject *unstructured.Unstructured
		var predictorName string
		var startTime string

		//using a generic predictorName
		predictorName = "transition-predictor"

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object from tf-predictor sample yaml file
			predictorObject = DecodeResourceFromFile(samplesPath + "tf-predictor.yaml")
			// rename it so that it has a generic name throughout the testing
			SetString(predictorObject, predictorName, "metadata", "name")

			// create the tf predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(predictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(predictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run an inference, update the model and run an inference again on the updated model", func() {
			// verify starting model path
			obj := ListAllPredictorsExpectOne()
			Expect(GetString(obj, "spec", "path")).To(Equal("fvt/tensorflow/mnist.savedmodel"))

			// Prepare for tf inference
			// load the first image of the mnist test set
			image := LoadMnistImage(0)

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "inputs",
				Shape:    []int64{1, 784},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: predictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// First - run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
			Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(7)) // this model predicts 7 for the first image

			// Modify & set the predictor object with xgboost model and model type
			SetString(predictorObject, "fvt/xgboost/mushroom", "spec", "path")
			SetString(predictorObject, "xgboost", "spec", "modelType", "name")

			// SECOND - update the predictor with the xgboost predictor object we prepared in the previous lines
			By("Updating the predictor with new model path")
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			fvtClient.UpdatePredictorExpectSuccess(predictorObject)

			// watch for the 'Loading' state followed by the 'Loaded' state
			By("Waiting for the Predictor's target model state to move from Loaded (empty) to Loading")
			loadingObj := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading"}, watcher)
			Expect(loadingObj.GetName()).To(Equal(predictorName))
			Expect(GetString(loadingObj, "spec", "path")).To(Equal("fvt/xgboost/mushroom"))
			Expect(GetString(loadingObj, "spec", "modelType", "name")).To(Equal("xgboost"))
			ExpectPredictorState(loadingObj, true, "Loaded", "Loading", "InProgress")

			By("Waiting for the predictor to be 'Loaded'")
			loadedObj := WaitForLastStateInExpectedList("targetModelState", []string{"Loading", "Loaded", ""}, watcher)
			ExpectPredictorState(loadedObj, true, "Loaded", "", "UpToDate")
			watcher.Stop()
			// xgboost predictor should be loaded by now

			// get the object with List and verify it one more time
			By("Listing the predictors")
			obj = ListAllPredictorsExpectOne()
			Expect(obj.GetName()).To(Equal(predictorName))
			Expect(GetString(obj, "spec", "path")).To(Equal("fvt/xgboost/mushroom"))
			Expect(GetString(obj, "spec", "modelType", "name")).To(Equal("xgboost"))
			ExpectPredictorState(obj, true, "Loaded", "", "UpToDate")

			// build the grpc inference call
			inferInput2 := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 126},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: xgBoostInputData},
			}
			inferRequest2 := &inference.ModelInferRequest{
				ModelName: predictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput2},
			}

			// THIRD - Run the xgboost inference on the updated predictor
			inferResponse2, err := fvtClient.RunKfsInference(inferRequest2)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse2).ToNot(BeNil())
			// check if the model predicted the input as close to 0 as possible
			Expect(math.Round(float64(inferResponse2.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
		})
	})

	var _ = Describe("TensorFlow inference", func() {
		var tfPredictorObject *unstructured.Unstructured
		var tfPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			tfPredictorObject = DecodeResourceFromFile(samplesPath + "tf-predictor.yaml")
			tfPredictorName = GetString(tfPredictorObject, "metadata", "name")

			// create the tf predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(tfPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(tfPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run an inference", func() {
			// load the first image of the mnist test set
			image := LoadMnistImage(0)

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "inputs",
				Shape:    []int64{1, 784},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: tfPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(tfPredictorName))
			Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(7)) // this model predicts 7 for the first image

		})

		It("should successfully run an inference on an updated model", func() {

			By("Updating the predictor with new model path")
			SetString(tfPredictorObject, "fvt/tensorflow/mnist-dup.savedmodel", "spec", "path")
			fvtClient.UpdatePredictorExpectSuccess(tfPredictorObject)

			By("Waiting for the model transition to complete")
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			// watch for the 'Loading' state followed by the 'Loaded' state
			By("Waiting for the predictor to be 'Loading'")
			WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading"}, watcher)
			By("Waiting for the predictor to be 'Loaded'")
			WaitForLastStateInExpectedList("targetModelState", []string{"Loading", "Loaded", ""}, watcher)
			watcher.Stop()

			// load the first image of the mnist test set
			image := LoadMnistImage(0)

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "inputs",
				Shape:    []int64{1, 784},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: tfPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(tfPredictorName))
			Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(7)) // this model predicts 7 for the first image

		})

		It("should fail with invalid data type", func() {
			// run inference on invalid []int32 instead of expected []float32
			image := []int32{0, 1, 2, 3}

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "inputs",
				Shape:    []int64{1, 784},
				Datatype: "INT32", // this is an invalid datatype
				Contents: &inference.InferTensorContents{IntContents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: tfPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("model expects 'FP32'"))
			Expect(inferResponse).To(BeNil())
		})
	})

	var _ = Describe("ONNX inference", func() {
		var onnxPredictorObject *unstructured.Unstructured
		var onnxPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			onnxPredictorObject = DecodeResourceFromFile(samplesPath + "onnx-predictor.yaml")
			onnxPredictorName = GetString(onnxPredictorObject, "metadata", "name")

			// create the onnx predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(onnxPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(onnxPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run an inference", func() {
			image := LoadMnistImage(0)

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "Input3",
				Shape:    []int64{1, 1, 28, 28},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: onnxPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(onnxPredictorName))
			// Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(0))
		})

		It("should fail to run an inference with invalid shape", func() {
			image := LoadMnistImage(0)

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "Input3",
				Shape:    []int64{1, 1, 2999},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: onnxPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(inferResponse).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: unexpected shape for input"))
		})
	})

	var _ = Describe("MLServer inference", func() {

		var mlsPredictorObject *unstructured.Unstructured
		var mlsPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			mlsPredictorObject = DecodeResourceFromFile(samplesPath + "mlserver-sklearn-predictor.yaml")
			mlsPredictorName = GetString(mlsPredictorObject, "metadata", "name")

			// create the predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(mlsPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(mlsPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run inference", func() {
			// the example model for FVT is an MNIST model provided as an example in
			// the MLServer repo:
			// https://github.com/SeldonIO/MLServer/tree/8925ad5/examples/sklearn

			// this example model takes 8x8 floating point images as input flattened
			// to a 64 float array
			image := []float32{
				0., 0., 1., 11., 14., 15., 3., 0., 0., 1., 13., 16., 12.,
				16., 8., 0., 0., 8., 16., 4., 6., 16., 5., 0., 0., 5.,
				15., 11., 13., 14., 0., 0., 0., 0., 2., 12., 16., 13., 0.,
				0., 0., 0., 0., 13., 16., 16., 6., 0., 0., 0., 0., 16.,
				16., 16., 7., 0., 0., 0., 0., 11., 13., 12., 1., 0.,
			}

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 64},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: mlsPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(mlsPredictorName))
			Expect(inferResponse.Outputs[0].Contents.Fp32Contents[0]).To(BeEquivalentTo(8))
		})

		It("should fail with an invalid input", func() {
			image := []float32{0.}

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 1},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: mlsPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("1 should be equal to 64"))
		})
	})

	var _ = Describe("XGBoost inference", func() {
		var xgboostPredictorObject *unstructured.Unstructured
		var xgboostPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			xgboostPredictorObject = DecodeResourceFromFile(samplesPath + "xgboost-predictor.yaml")
			xgboostPredictorName = GetString(xgboostPredictorObject, "metadata", "name")

			// create the xgboost predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(xgboostPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(xgboostPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run an inference", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 126},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: xgBoostInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: xgboostPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			// check if the model predicted the input as close to 0 as possible
			Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
		})

		It("should fail with invalid shape", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: xgBoostInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: xgboostPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: Invalid input to XGBoostModel"))
		})
	})

	var _ = Describe("Pytorch inference", func() {

		var ptPredictorObject *unstructured.Unstructured
		var ptPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			ptPredictorObject = DecodeResourceFromFile(samplesPath + "pytorch-predictor.yaml")
			ptPredictorName = GetString(ptPredictorObject, "metadata", "name")

			// create the predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(ptPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(ptPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run inference", func() {
			image := LoadCifarImage(1)

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "INPUT__0",
				Shape:    []int64{1, 3, 32, 32},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: ptPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(ptPredictorName))
			// convert raw_output_contents in bytes to array of 10 float32s
			output, err := convertRawOutputContentsTo10Floats(inferResponse.GetRawOutputContents()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(math.Abs(float64(output[8]-7.343689441680908)) < EPSILON).To(BeTrue()) // the 9th class gets the highest activation for this net/image
		})

		It("should fail with an invalid input", func() {
			image := LoadCifarImage(1)

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "INPUT__0",
				Shape:    []int64{1, 3, 16, 64}, // wrong shape
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: ptPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			log.Info(err.Error())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: unexpected shape for input"))
			Expect(inferResponse).To(BeNil())
		})
	})

	// This an inference testcase for pytorch that mandates schema in config.pbtxt
	// However config.pbtxt (in COS) by default does not include schema section, instead
	// schema passed in Predictor CR is updated (in config.pbtxt) after model downloaded.
	var _ = Describe("Pytorch inference with schema", func() {

		var ptPredictorObject *unstructured.Unstructured
		var ptPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			ptPredictorObject = DecodeResourceFromFile(samplesPath + "pytorch-predictor-withschema.yaml")
			ptPredictorName = GetString(ptPredictorObject, "metadata", "name")

			// create the predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(ptPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(ptPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run inference", func() {
			image := LoadCifarImage(1)

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "INPUT__0",
				Shape:    []int64{1, 3, 32, 32},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: ptPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(ptPredictorName))
			// convert raw_output_contents in bytes to array of 10 float32s
			output, err := convertRawOutputContentsTo10Floats(inferResponse.GetRawOutputContents()[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(math.Abs(float64(output[8]-7.343689441680908)) < EPSILON).To(BeTrue()) // the 9th class gets the highest activation for this net/image
		})

		It("should fail with an invalid input", func() {
			image := LoadCifarImage(1)

			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "INPUT__0",
				Shape:    []int64{1, 3, 16, 64}, // wrong shape
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: ptPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			// run the inference
			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			log.Info(err.Error())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: unexpected shape for input"))
			Expect(inferResponse).To(BeNil())
		})
	})

	var _ = Describe("LightGBM inference", func() {
		var lightGBMPredictorObject *unstructured.Unstructured
		var lightGBMPredictorName string
		var startTime string
		var lightGBMInputData []float32 = []float32{1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0}

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			lightGBMPredictorObject = DecodeResourceFromFile(samplesPath + "lightgbm-predictor.yaml")
			lightGBMPredictorName = GetString(lightGBMPredictorObject, "metadata", "name")

			// create the xgboost predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(lightGBMPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(lightGBMPredictorObject, resourceVersion, "metadata", "resourceVersion")

			err := fvtClient.ConnectToModelMesh(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
		})

		It("should successfully run an inference", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 126},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: lightGBMInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: lightGBMPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			// check if the model predicted the input as close to 0 as possible
			Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
		})

		It("should fail with invalid shape input", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: lightGBMInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: lightGBMPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: Invalid input to LightGBM"))
		})
	})

	var _ = Describe("TLS XGBoost inference", func() {
		var xgboostPredictorObject *unstructured.Unstructured
		var xgboostPredictorName string
		var startTime string

		BeforeEach(func() {
			// verify clean state (no predictors)
			list := fvtClient.ListPredictors(metav1.ListOptions{})
			Expect(list.Items).To(BeEmpty())

			// load the test predictor object
			xgboostPredictorObject = DecodeResourceFromFile(samplesPath + "xgboost-predictor.yaml")
			xgboostPredictorName = GetString(xgboostPredictorObject, "metadata", "name")

			// create the tf predictor
			watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
			defer watcher.Stop()
			createdPredictor := fvtClient.CreatePredictorExpectSuccess(xgboostPredictorObject)
			startTime = GetString(createdPredictor, "metadata", "creationTimestamp")
			By("Waiting for the predictor to be 'Loaded'")
			// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
			resourceVersion := GetString(obj, "metadata", "resourceVersion")
			SetString(xgboostPredictorObject, resourceVersion, "metadata", "resourceVersion")
		})

		AfterEach(func() {
			fvtClient.DisconnectFromModelMesh()
			if CurrentGinkgoTestDescription().Failed {
				fvtClient.PrintPredictors()
				fvtClient.TailPodLogs(startTime)
			}
			fvtClient.DeleteAllPredictors()
			fvtClient.DeleteConfigMap(userConfigMapName)
			time.Sleep(time.Second * 10)
		})

		It("should successfully run an inference with basic TLS", func() {
			fvtClient.UpdateConfigMapTLS("san-tls-secret", "optional")

			watcher := fvtClient.StartWatchingDeploys(servingRuntimeDeploymentsListOptions)
			defer watcher.Stop()
			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState(watcher)

			var timeAsleep int
			var mmeshErr error
			for timeAsleep <= 30 {
				mmeshErr = fvtClient.ConnectToModelMesh(TLS)

				if mmeshErr == nil {
					log.Info("Successfully connected to model mesh tls")
					break
				}

				log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
				fvtClient.DisconnectFromModelMesh()
				timeAsleep += 10
				time.Sleep(time.Second * time.Duration(timeAsleep))
			}

			Expect(mmeshErr).ToNot(HaveOccurred())

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 126},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: xgBoostInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: xgboostPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, inferErr := fvtClient.RunKfsInference(inferRequest)
			Expect(inferErr).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(xgboostPredictorName))
			// check if the model predicted the input as close to 0 as possible
			Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
		})

		It("should successfully run an inference with mutual TLS", func() {
			fvtClient.UpdateConfigMapTLS("san-tls-secret-client-auth", "require")

			watcher := fvtClient.StartWatchingDeploys(servingRuntimeDeploymentsListOptions)
			defer watcher.Stop()
			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState(watcher)

			var timeAsleep int
			var mmeshErr error
			for timeAsleep <= 30 {
				mmeshErr = fvtClient.ConnectToModelMesh(MutualTLS)

				if mmeshErr == nil {
					log.Info("Successfully connected to model mesh tls")
					break
				}

				log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
				fvtClient.DisconnectFromModelMesh()
				timeAsleep += 10
				time.Sleep(time.Second * time.Duration(timeAsleep))
			}
			Expect(mmeshErr).ToNot(HaveOccurred())

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 126},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: xgBoostInputData},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: xgboostPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, inferErr := fvtClient.RunKfsInference(inferRequest)
			Expect(inferErr).ToNot(HaveOccurred())
			Expect(inferResponse).ToNot(BeNil())
			Expect(inferResponse.ModelName).To(HavePrefix(xgboostPredictorName))
			// check if the model predicted the input as close to 0 as possible
			Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
		})

		It("should fail to run inference when the server has mutual TLS but the client does not present a certificate", func() {
			fvtClient.UpdateConfigMapTLS("san-tls-secret-client-auth", "require")

			watcher := fvtClient.StartWatchingDeploys(servingRuntimeDeploymentsListOptions)
			defer watcher.Stop()
			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState(watcher)

			// this test is expected to fail to connect due to the TLS cert, so we
			// don't retry if it fails
			mmeshErr := fvtClient.ConnectToModelMesh(TLS)
			Expect(mmeshErr).To(HaveOccurred())
		})
	})
	// The TLS tests `Describe` block should be the last one in the list to
	// improve efficiency of the tests. Any test after the TLS tests would need
	// to wait for the configuration changes to roll out to all Deployments.
})

// These tests verify that an invalid Predictor fails to load. These are in a
// separate block in part because a high frequency of failures can trigger Model
// Mesh's "bootstrap failure" mechanism which prevents rollouts of new pods that
// fail frequently by causing them to fail the readiness check.
// At the end of this block, all runtime deployments are rolled out to remove
// any that may have gone unready.
var _ = Describe("Invalid Predictors", func() {
	var predictorObject *unstructured.Unstructured
	var predictorName string
	var startTime string

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			fvtClient.PrintPredictors()
			fvtClient.TailPodLogs(startTime)
		}
		fvtClient.DeleteAllPredictors()
	})

	Specify("Preparing the cluster for Invalid Predictor tests", func() {
		// ensure configuration has scale-to-zero disabled
		config := map[string]interface{}{
			"scaleToZero": map[string]interface{}{
				"enabled": false,
			},
		}
		fvtClient.ApplyUserConfigMap(config)

		// ensure a stable deploy state
		watcher := fvtClient.StartWatchingDeploys(servingRuntimeDeploymentsListOptions)
		defer watcher.Stop()
		WaitForStableActiveDeployState(watcher)
	})

	for _, p := range predictorsArray {
		predictor := p

		Describe("invalid cases for the "+predictor.predictorName+" predictor", func() {
			BeforeEach(func() {
				// load the test predictor object
				predictorObject = DecodeResourceFromFile(samplesPath + predictor.predictorFilename)
				predictorName = GetString(predictorObject, "metadata", "name")
			})

			It("predictor should fail to load with invalid storage path", func() {
				// modify the object with an invalid storage path
				SetString(predictorObject, "invalid/Storage/Path", "spec", "path")

				By("Creating a predictor with invalid storage")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'FailedToLoad'")
				// "Standby" state is encountered after the "Loading" state but it shouldn't be
				obj = WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)

				By("Asserting on the predictor state")
				ExpectPredictorState(obj, false, "FailedToLoad", "", "UpToDate")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
			})

			It("should fail to load with invalid storage bucket", func() {
				// modify the object with an invalid storage bucket
				SetString(predictorObject, "invalidBucket", "spec", "storage", "s3", "bucket")

				By("Creating a predictor with invalid storage")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'FailedToLoad'")
				// "Standby" state is encountered after the "Loading" state but it shouldn't be
				obj = WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)

				By("Asserting on the predictor state")
				ExpectPredictorState(obj, false, "FailedToLoad", "", "UpToDate")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
				// TODO can we check for a more detailed error message?
			})

			It("should fail to load with invalid storage key", func() {
				// modify the object with an invalid storage path
				SetString(predictorObject, "invalidKey", "spec", "storage", "s3", "secretKey")

				By("Creating a predictor with invalid storage")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'FailedToLoad'")
				// "Standby" state is encountered after the "Loading" state but it shouldn't be
				obj = WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)

				By("Asserting on the predictor state")
				ExpectPredictorState(obj, false, "FailedToLoad", "", "UpToDate")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
				// TODO can we check for a more detailed error message?
			})

			It("predictor should fail to load with unrecognized model type", func() {
				// modify the object with an unrecognized model type
				SetString(predictorObject, "invalidModelType", "spec", "modelType", "name")

				By("Creating a predictor with invalid model type")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'FailedToLoad'")
				// "Standby" state is encountered after the "Loading" state but it shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)

				By("Listing the predictor")
				obj = ListAllPredictorsExpectOne()
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "FailedToLoad", "", "UpToDate")
				ExpectPredictorFailureInfo(obj, "NoSupportingRuntime", false, true, "No ServingRuntime supports specified model type")
			})

			It("predictor should fail to load with unrecognized runtime type", func() {
				// modify the object with an unrecognized runtime type
				SetString(predictorObject, "invalidRuntimeType", "spec", "runtime", "name")

				By("Creating a predictor with invalid runtime type")
				watcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{}, defaultTimeout)
				defer watcher.Stop()
				obj := fvtClient.CreatePredictorExpectSuccess(predictorObject)
				startTime = GetString(obj, "metadata", "creationTimestamp")
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

				By("Waiting for the predictor to be 'FailedToLoad'")
				// "Standby" state is encountered after the "Loading" state but it shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)

				By("Listing the predictor")
				obj = ListAllPredictorsExpectOne()
				Expect(obj.GetName()).To(Equal(predictorName))
				ExpectPredictorState(obj, false, "FailedToLoad", "", "UpToDate")
				ExpectPredictorFailureInfo(obj, "RuntimeNotRecognized", false, true, "Specified runtime name not recognized")
			})
		})
	}

	Specify("Restart pods to reset Bootstrap failure checks", func() {
		fvtClient.RestartDeploys()
	})
})

func ExpectPredictorState(obj *unstructured.Unstructured, available bool, activeModelState, targetModelState, transitionStatus string) {
	actualActiveModelState := GetString(obj, "status", "activeModelState")
	Expect(actualActiveModelState).To(Equal(activeModelState))

	actualAvailable := GetBool(obj, "status", "available")
	Expect(actualAvailable).To(Equal(available))

	actualTargetModel := GetString(obj, "status", "targetModelState")
	Expect(actualTargetModel).To(Equal(targetModelState))

	actualTransitionStatus := GetString(obj, "status", "transitionStatus")
	Expect(actualTransitionStatus).To(Equal(transitionStatus))

	if transitionStatus != "BlockedByFailedLoad" && transitionStatus != "InvalidSpec" &&
		activeModelState != "FailedToLoad" && targetModelState != "FailedToLoad" {
		actualFailureInfo := GetMap(obj, "status", "lastFailureInfo")
		Expect(actualFailureInfo).To(BeNil())
	}
}

func ExpectPredictorFailureInfo(obj *unstructured.Unstructured, reason string, hasLocation bool, hasTime bool, message string) {
	actualFailureInfo := GetMap(obj, "status", "lastFailureInfo")
	Expect(actualFailureInfo).ToNot(BeNil())

	Expect(actualFailureInfo["reason"]).To(Equal(reason))
	if hasLocation {
		Expect(actualFailureInfo["location"]).ToNot(BeEmpty())
	} else {
		Expect(actualFailureInfo["location"]).To(BeNil())
	}
	if message != "" {
		Expect(actualFailureInfo["message"]).To(Equal(message))
	} else {
		Expect(actualFailureInfo["message"]).ToNot(BeEmpty())
	}
	if !hasTime {
		Expect(actualFailureInfo["time"]).To(BeNil())
	} else {
		Expect(actualFailureInfo["time"]).ToNot(BeNil())
		actualTime, err := time.Parse(time.RFC3339, actualFailureInfo["time"].(string))
		Expect(err).To(BeNil())
		Expect(time.Since(actualTime) < time.Minute).To(BeTrue())
	}
}

// func WaitForModelStatus(statusAttribute, targetState string, watcher watch.Interface, timeToStabilize time.Duration) *unstructured.Unstructured {
// 	ch := watcher.ResultChan()
// 	targetStateReached := false
// 	var obj *unstructured.Unstructured
// 	var lastState string
// 	var predictorName string

// 	done := false
// 	for !done {
// 		select {
// 		case event, ok := <-ch:
// 			if !ok {
// 				// the channel was closed (watcher timeout reached)
// 				done = true
// 				break
// 			}
// 			obj, ok = event.Object.(*unstructured.Unstructured)
// 			Expect(ok).To(BeTrue())
// 			log.Info("Watcher got event with object", logPredictorStatus(obj)...)

// 			lastState = GetString(obj, "status", statusAttribute)
// 			predictorName = GetString(obj, "metadata", "name")
// 			if lastState == targetState {
// 				// targetStateReached and stable
// 				targetStateReached = true
// 				done = true
// 			}

// 		// this case executes if no events are received during the timeToStabilize duration
// 		case <-time.After(timeToStabilize):
// 			if lastState == targetState {
// 				// targetStateReached and stable
// 				targetStateReached = true
// 				done = true
// 			}
// 		}
// 	}
// 	Expect(targetStateReached).To(BeTrue(), "Timeout before predictor '%s' became '%s' (last state was '%s')",
// 		predictorName, targetState, lastState)
// 	return obj
// }

// Waiting for predictor state to reach the last one in the expected list
// Predictor state is allowed to directly reach the last state in the expected list i.e; Loaded. Also, Predictor state can be
// one of the earlier states (i.e; Pending or Loading), but state change should happen in the following order:
// [Pending => Loaded]  (or) [Pending => Loading => Loaded]  (or) [Loading => Loaded] (or) [Pending => Loading => FailedToLoad]
func WaitForLastStateInExpectedList(statusAttribute string, expectedStates []string, watcher watch.Interface) *unstructured.Unstructured {
	ch := watcher.ResultChan()
	targetStateReached := false
	var obj *unstructured.Unstructured
	var targetState string = expectedStates[len(expectedStates)-1]
	var lastState string
	var predictorName string

	timeout := time.After(predictorTimeout)
	lastStateIndex := 0
	done := false
	for !done {
		select {
		// Exit the loop if the final state in the list is not reached within given timeout
		case <-timeout:
			if lastState == targetState {
				// targetStateReached and stable
				targetStateReached = true
			}
			done = true

		case event, ok := <-ch:
			if !ok {
				// the channel was closed (watcher timeout reached)
				done = true
				break
			}
			obj, ok = event.Object.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())
			log.Info("Watcher got event with object", logPredictorStatus(obj)...)

			lastState = GetString(obj, "status", statusAttribute)
			predictorName = GetString(obj, "metadata", "name")
			if lastState == targetState {
				// targetStateReached and stable
				targetStateReached = true
				done = true
			} else {
				// Verify the order of predictor state change
				validStateChange := false
				for i, state := range expectedStates[lastStateIndex:] {
					if state == lastState {
						lastStateIndex += i
						validStateChange = true
						break
					}
				}
				Expect(validStateChange).To(BeTrue(), "Predictor %s state should not be changed from '%s' to '%s'", predictorName, expectedStates[lastStateIndex], lastState)
			}
		}
	}
	Expect(targetStateReached).To(BeTrue(), "Timeout before predictor '%s' became '%s' (last state was '%s')",
		predictorName, targetState, lastState)
	return obj
}

// func GetModelNextState(statusAttribute, lastState string, watcher watch.Interface, timeToStabilize time.Duration) *unstructured.Unstructured {
// 	ch := watcher.ResultChan()
// 	var obj *unstructured.Unstructured
// 	var stateChanged bool = false
// 	var currentState string
// 	var predictorName string
//
// 	done := false
// 	for !done {
// 		select {
// 		case event, ok := <-ch:
// 			if !ok {
// 				// the channel was closed (watcher timeout reached)
// 				break
// 			}
// 			obj, ok = event.Object.(*unstructured.Unstructured)
// 			Expect(ok).To(BeTrue())
// 			log.Info("Watcher got event with object", logPredictorStatus(obj)...)
//
// 			currentState = GetString(obj, "status", statusAttribute)
// 			predictorName = GetString(obj, "metadata", "name")
// 			if lastState != currentState {
// 				// state changed
// 				stateChanged = true
// 				done = true
// 			}
//
// 		// this case executes if no events are received during the timeToStabilize duration
// 		case <-time.After(timeToStabilize):
// 			if lastState != currentState {
// 				// state changed
// 				stateChanged = true
// 			}
// 			done = true
// 		}
// 	}
// 	Expect(stateChanged).To(BeTrue(), "Timeout before predictor '%s' state is changed (last state was '%s')",
// 		predictorName, lastState)
// 	return obj
// }
//
// func WaitForStableActiveModelState(targetState string, watcher watch.Interface) *unstructured.Unstructured {
// 	return WaitForModelStatus("activeModelState", targetState, watcher, timeForStatusToStabilize)
// }

func WaitForDeployStatus(watcher watch.Interface, timeToStabilize time.Duration) {
	ch := watcher.ResultChan()
	targetStateReached := false
	var obj *unstructured.Unstructured
	var replicas, updatedReplicas, availableReplicas int64
	var deployName string
	deployStatusesReady := make(map[string]bool)

	done := false
	for !done {
		select {
		case event, ok := <-ch:
			if !ok {
				// the channel was closed (watcher timeout reached)
				done = true
				break
			}
			obj, ok = event.Object.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())

			replicas = GetInt64(obj, "status", "replicas")
			availableReplicas = GetInt64(obj, "status", "availableReplicas")
			updatedReplicas = GetInt64(obj, "status", "updatedReplicas")
			deployName = GetString(obj, "metadata", "name")

			log.Info("Watcher got event with object",
				"name", deployName,
				"status.replicas", replicas,
				"status.availableReplicas", availableReplicas,
				"status.updatedReplicas", updatedReplicas)

			if (updatedReplicas == replicas) && (availableReplicas == updatedReplicas) {
				deployStatusesReady[deployName] = true
				log.Info(fmt.Sprintf("deployStatusesReady: %v", deployStatusesReady))
			} else {
				deployStatusesReady[deployName] = false
			}

		// this case executes if no events are received during the timeToStabilize duration
		case <-time.After(timeToStabilize):
			// check if all deployments are ready
			stable := true
			for _, status := range deployStatusesReady {
				if status == false {
					stable = false
					break
				}
			}
			if stable {
				targetStateReached = true
				done = true
			}
		}
	}
	Expect(targetStateReached).To(BeTrue(), "Timeout before deploy '%s' ready(last state was replicas: '%v' updatedReplicas: '%v' availableReplicas: '%v')",
		deployName, replicas, updatedReplicas, availableReplicas)
}

func WaitForStableActiveDeployState(watcher watch.Interface) {
	defer watcher.Stop()
	WaitForDeployStatus(watcher, timeForStatusToStabilize)
}

func ListAllPredictorsExpectOne() *unstructured.Unstructured {
	return ListPredictorsExpectOne(metav1.ListOptions{})
}

func ListPredictorsByNameExpectOne(name string) *unstructured.Unstructured {
	options := metav1.ListOptions{}
	options.FieldSelector = "metadata.name=" + name
	return ListPredictorsExpectOne(options)
}

func ListPredictorsExpectOne(options metav1.ListOptions) *unstructured.Unstructured {
	list := fvtClient.ListPredictors(options)
	Expect(list.Items).To(HaveLen(1))
	obj := &list.Items[0]

	log.Info("ListAllPredictorsExpectOne returned object", logPredictorStatus(obj)...)
	return obj
}

func LoadMnistImage(index int) []float32 {
	images, err := mnist.LoadImageFile("testdata/t10k-images-idx3-ubyte.gz")
	Expect(err).ToNot(HaveOccurred())

	imageBytes := [mnist.Width * mnist.Height]byte(*images[index])
	var imageFloat [mnist.Width * mnist.Height]float32
	for i, v := range imageBytes {
		imageFloat[i] = float32(v)
	}
	return imageFloat[:]
}

func LoadCifarImage(index int) []float32 {
	file, err := os.Open("testdata/cifar_test_images.bin")
	Expect(err).ToNot(HaveOccurred())
	images, err := cifar.Decode10(file)
	Expect(err).ToNot(HaveOccurred())

	imageBytes := images[index].RawData()
	var imageFloat [3 * 32 * 32]float32
	for i, v := range imageBytes {
		// the test PyTorch CIFAR model was trained based on:
		// - https://github.com/kubeflow/kfserving/tree/master/docs/samples/v1alpha2/pytorch
		// - https://pytorch.org/tutorials/beginner/blitz/cifar10_tutorial.html
		// These models are trained on images with pixels normalized to the range
		// [-1 1]. The testdata contains images with pixels in bytes [0 255] that
		// must be transformed
		imageFloat[i] = (float32(v) / 127.5) - 1
	}

	return imageFloat[:]
}

func logPredictorStatus(obj *unstructured.Unstructured) []interface{} {
	return []interface{}{
		"name", GetString(obj, "metadata", "name"),
		"status.available", GetBool(obj, "status", "available"),
		"status.activeModelState", GetString(obj, "status", "activeModelState"),
		"status.targetModelState", GetString(obj, "status", "targetModelState"),
		"status.transitionStatus", GetString(obj, "status", "transitionStatus"),
		"status.lastFailureInfo", GetMap(obj, "status", "lastFailureInfo"),
	}
}

func convertRawOutputContentsTo10Floats(raw []byte) ([10]float32, error) {
	var floats [10]float32
	r := bytes.NewReader(raw)

	err := binary.Read(r, binary.LittleEndian, &floats)
	return floats, err
}
