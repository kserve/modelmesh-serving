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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"
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
	differentPredictorName     string
	differentPredictorFilename string
}

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
			// disable scale-to-zero to prevent pods flapping as
			// Predictors are created and deleted
			"scaleToZero": map[string]interface{}{
				"enabled": false,
			},
			// disable the model-mesh bootstrap failure check so
			// that the expected failures for invalid predictor
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
		fvtClient.DeleteAllPredictors()

		// ensure a stable deploy state
		WaitForStableActiveDeployState()
	})

	for _, p := range predictorsArray {
		predictor := p
		var _ = Describe("create "+predictor.predictorName+" predictor", func() {

			It("should successfully load a model", func() {
				predictorObject := NewPredictorForFVT(predictor.predictorFilename)
				CreatePredictorAndWaitAndExpectLoaded(predictorObject)

				// clean up
				fvtClient.DeletePredictor(predictorObject.GetName())
			})

			It("should successfully load two models of different types", func() {
				predictorObject := NewPredictorForFVT(predictor.predictorFilename)
				predictorName := predictorObject.GetName()

				differentPredictorObject := NewPredictorForFVT(predictor.differentPredictorFilename)
				differentPredictorName := differentPredictorObject.GetName()

				By("Creating the " + predictor.predictorName + " predictor")
				predictorWatcher := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, defaultTimeout)
				defer predictorWatcher.Stop()
				predictorObject = fvtClient.CreatePredictorExpectSuccess(predictorObject)
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

				By("Verifying the predictors")
				predictorObject = fvtClient.GetPredictor(predictorName)
				ExpectPredictorState(predictorObject, true, "Loaded", "", "UpToDate")
				differentPredictorObject = fvtClient.GetPredictor(differentPredictorName)
				ExpectPredictorState(differentPredictorObject, true, "Loaded", "", "UpToDate")

				// clean up
				fvtClient.DeletePredictor(predictorName)
				fvtClient.DeletePredictor(differentPredictorName)
			})

			It("should successfully load two models of the same type", func() {
				By("Creating the first " + predictor.predictorName + " predictor")
				pred1 := NewPredictorForFVT(predictor.predictorFilename)
				watcher1 := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + pred1.GetName()}, defaultTimeout)
				defer watcher1.Stop()
				obj1 := fvtClient.CreatePredictorExpectSuccess(pred1)
				ExpectPredictorState(obj1, false, "Pending", "", "UpToDate")

				By("Creating a second " + predictor.predictorName + " predictor")
				pred2 := NewPredictorForFVT(predictor.predictorFilename)
				watcher2 := fvtClient.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + pred2.GetName()}, defaultTimeout)
				defer watcher2.Stop()
				obj2 := fvtClient.CreatePredictorExpectSuccess(pred2)
				ExpectPredictorState(obj2, false, "Pending", "", "UpToDate")

				By("Waiting for the first predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher1)
				By("Waiting for the second predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher2)

				By("Verifying the predictors")
				obj1 = fvtClient.GetPredictor(pred1.GetName())
				ExpectPredictorState(obj1, true, "Loaded", "", "UpToDate")
				obj2 = fvtClient.GetPredictor(pred2.GetName())
				ExpectPredictorState(obj2, true, "Loaded", "", "UpToDate")

				// clean up
				fvtClient.DeletePredictor(pred1.GetName())
				fvtClient.DeletePredictor(pred2.GetName())
			})

		})

		var _ = Describe("update "+predictor.predictorName+" predictor", func() {
			var predictorObject *unstructured.Unstructured
			var predictorName string

			BeforeEach(func() {
				// load the test predictor object
				predictorObject = NewPredictorForFVT(predictor.predictorFilename)
				predictorName = predictorObject.GetName()

				CreatePredictorAndWaitAndExpectLoaded(predictorObject)
			})

			AfterEach(func() {
				fvtClient.DeletePredictor(predictorName)
			})

			It("should successfully update and reload the model", func() {
				// verify starting model path
				obj := fvtClient.GetPredictor(predictorName)
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.currentModelPath))

				// modify the object with a new valid path
				SetString(predictorObject, predictor.updatedModelPath, "spec", "path")

				obj = UpdatePredictorAndWaitAndExpectLoaded(predictorObject)

				By("Verifying the predictors")
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.updatedModelPath))
			})

			It("should fail to load the target model with invalid path", func() {
				// verify starting model path
				obj := fvtClient.GetPredictor(predictorName)
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.currentModelPath))

				// modify the object with a new valid path
				SetString(predictorObject, "invalid/path", "spec", "path")

				obj = UpdatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Verifying the predictors")
				Expect(GetString(obj, "spec", "path")).To(Equal("invalid/path"))
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
			})
		})

	}

	var _ = Describe("test transition of Predictor between models", func() {
		var predictorObject *unstructured.Unstructured
		var predictorName string

		BeforeEach(func() {
			// load the test predictor object from tf-predictor sample yaml file
			predictorObject = NewPredictorForFVT("tf-predictor.yaml")
			predictorName = MakeUniquePredictorName("transition-predictor")
			predictorObject.SetName(predictorName)

			CreatePredictorAndWaitAndExpectLoaded(predictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(predictorName)
		})

		It("should successfully run an inference, update the model and run an inference again on the updated model", func() {
			// verify starting model path
			obj := fvtClient.GetPredictor(predictorName)
			Expect(GetString(obj, "spec", "path")).To(Equal("fvt/tensorflow/mnist.savedmodel"))

			ExpectSuccessfulInference_tensorflowMnist(predictorName)

			By("Updating the predictor with new model path")
			// Modify & set the predictor object with xgboost model and model type
			SetString(predictorObject, "fvt/xgboost/mushroom", "spec", "path")
			SetString(predictorObject, "xgboost", "spec", "modelType", "name")

			obj = UpdatePredictorAndWaitAndExpectLoaded(predictorObject)

			By("Verifying the predictor")
			Expect(GetString(obj, "spec", "path")).To(Equal("fvt/xgboost/mushroom"))
			Expect(GetString(obj, "spec", "modelType", "name")).To(Equal("xgboost"))

			ExpectSuccessfulInference_xgboostMushroom(predictorName)
		})
	})

	var _ = Describe("Missing storage field", func() {
		var predictorObject *unstructured.Unstructured

		BeforeEach(func() {
			// load the test predictor object
			predictorObject = NewPredictorForFVT("no-storage-predictor.yaml")
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(predictorObject.GetName())
		})

		It("predictor should fail to load with invalid storage path", func() {
			obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

			By("Asserting on the predictor state")
			ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "Predictor Storage field missing")
		})
	})

	var _ = Describe("TensorFlow inference", func() {
		var tfPredictorObject *unstructured.Unstructured
		var tfPredictorName string

		BeforeEach(func() {
			// load the test predictor object
			tfPredictorObject = NewPredictorForFVT("tf-predictor.yaml")
			tfPredictorName = tfPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(tfPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(tfPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_tensorflowMnist(tfPredictorName)
		})

		It("should successfully run an inference on an updated model", func() {

			By("Updating the predictor with new model path")
			SetString(tfPredictorObject, "fvt/tensorflow/mnist-dup.savedmodel", "spec", "path")

			UpdatePredictorAndWaitAndExpectLoaded(tfPredictorObject)

			ExpectSuccessfulInference_tensorflowMnist(tfPredictorName)
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

		BeforeEach(func() {
			// load the test predictor object
			onnxPredictorObject = NewPredictorForFVT("onnx-predictor.yaml")
			onnxPredictorName = onnxPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(onnxPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(onnxPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_onnxMnist(onnxPredictorName)
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

		BeforeEach(func() {
			// load the test predictor object
			mlsPredictorObject = NewPredictorForFVT("mlserver-sklearn-predictor.yaml")
			mlsPredictorName = mlsPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(mlsPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(mlsPredictorName)
		})

		It("should successfully run inference", func() {
			ExpectSuccessfulInference_sklearnMnistSvm(mlsPredictorName)
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

		BeforeEach(func() {
			// load the test predictor object
			xgboostPredictorObject = NewPredictorForFVT("xgboost-predictor.yaml")
			xgboostPredictorName = xgboostPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(xgboostPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_xgboostMushroom(xgboostPredictorName)
		})

		It("should fail with invalid shape", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: []float32{}},
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

		BeforeEach(func() {
			// load the test predictor object
			ptPredictorObject = NewPredictorForFVT("pytorch-predictor.yaml")
			ptPredictorName = ptPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(ptPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(ptPredictorName)
		})

		It("should successfully run inference", func() {
			ExpectSuccessfulInference_pytorchCifar(ptPredictorName)
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

		BeforeEach(func() {
			// load the test predictor object
			ptPredictorObject = NewPredictorForFVT("pytorch-predictor-withschema.yaml")
			ptPredictorName = ptPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(ptPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(ptPredictorName)
		})

		It("should successfully run inference", func() {
			ExpectSuccessfulInference_pytorchCifar(ptPredictorName)
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

		BeforeEach(func() {
			// load the test predictor object
			lightGBMPredictorObject = NewPredictorForFVT("lightgbm-predictor.yaml")
			lightGBMPredictorName = lightGBMPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(lightGBMPredictorObject)

			err := fvtClient.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(lightGBMPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_lightgbmMushroom(lightGBMPredictorName)
		})

		It("should fail with invalid shape input", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "predict",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: []float32{}},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: lightGBMPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := fvtClient.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unexpected <class 'ValueError'>: cannot reshape array"))
		})
	})

	var _ = Describe("TLS XGBoost inference", func() {
		var xgboostPredictorObject *unstructured.Unstructured
		var xgboostPredictorName string

		BeforeEach(func() {
			// load the test predictor object
			xgboostPredictorObject = NewPredictorForFVT("xgboost-predictor.yaml")
			xgboostPredictorName = xgboostPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)
		})

		AfterEach(func() {
			fvtClient.DeletePredictor(xgboostPredictorName)
			fvtClient.DeleteConfigMap(userConfigMapName)
			time.Sleep(time.Second * 10)
		})

		It("should successfully run an inference with basic TLS", func() {
			fvtClient.UpdateConfigMapTLS("basic-tls-secret", "optional")

			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState()

			By("Creating a new connection to ModelServing")
			// ensure we are establishing a new connection after the TLS change
			fvtClient.DisconnectFromModelServing()

			var timeAsleep int
			var mmeshErr error
			for timeAsleep <= 30 {
				mmeshErr = fvtClient.ConnectToModelServing(TLS)

				if mmeshErr == nil {
					log.Info("Successfully connected to model mesh tls")
					break
				}

				log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
				fvtClient.DisconnectFromModelServing()
				timeAsleep += 10
				time.Sleep(time.Second * time.Duration(timeAsleep))
			}

			Expect(mmeshErr).ToNot(HaveOccurred())

			By("Expect inference to succeed")
			ExpectSuccessfulInference_xgboostMushroom(xgboostPredictorName)

			// disconnect because TLS config will change
			fvtClient.DisconnectFromModelServing()
		})

		It("should successfully run an inference with mutual TLS", func() {
			fvtClient.UpdateConfigMapTLS("mutual-tls-secret", "require")

			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState()

			By("Creating a new connection to ModelServing")
			// ensure we are establishing a new connection after the TLS change
			fvtClient.DisconnectFromModelServing()

			var timeAsleep int
			var mmeshErr error
			for timeAsleep <= 30 {
				mmeshErr = fvtClient.ConnectToModelServing(MutualTLS)

				if mmeshErr == nil {
					log.Info("Successfully connected to model mesh tls")
					break
				}

				log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
				fvtClient.DisconnectFromModelServing()
				timeAsleep += 10
				time.Sleep(time.Second * time.Duration(timeAsleep))
			}
			Expect(mmeshErr).ToNot(HaveOccurred())

			By("Expect inference to succeed")
			ExpectSuccessfulInference_xgboostMushroom(xgboostPredictorName)

			// disconnect because TLS config will change
			fvtClient.DisconnectFromModelServing()
		})

		It("should fail to run inference when the server has mutual TLS but the client does not present a certificate", func() {
			fvtClient.UpdateConfigMapTLS("mutual-tls-secret", "require")

			By("Waiting for the deployments replicas to be ready")
			WaitForStableActiveDeployState()

			By("Expect a new connection to fail")
			// since the connection switches to TLS, ensure we are establishing a new connection
			fvtClient.DisconnectFromModelServing()
			// this test is expected to fail to connect due to the TLS cert, so we
			// don't retry if it fails
			mmeshErr := fvtClient.ConnectToModelServing(TLS)
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

	Specify("Preparing the cluster for Invalid Predictor tests", func() {
		config := map[string]interface{}{
			// disable scale-to-zero to prevent pods flapping as
			// Predictors are created and deleted
			"scaleToZero": map[string]interface{}{
				"enabled": false,
			},
			// disable the model-mesh bootstrap failure check so
			// that the expected failures for invalid predictor
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

		// ensure a stable deploy state
		WaitForStableActiveDeployState()
	})

	for _, p := range predictorsArray {
		predictor := p

		Describe("invalid cases for the "+predictor.predictorName+" predictor", func() {
			BeforeEach(func() {
				// load the test predictor object
				predictorObject = NewPredictorForFVT(predictor.predictorFilename)
			})

			AfterEach(func() {
				fvtClient.DeletePredictor(predictorObject.GetName())
			})

			It("predictor should fail to load with invalid storage path", func() {
				// modify the object with an invalid storage path
				SetString(predictorObject, "invalid/Storage/Path", "spec", "path")

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Asserting on the predictor state")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
			})

			It("predictor should fail to load with invalid storage bucket", func() {
				// modify the object with an invalid storage bucket
				SetString(predictorObject, "invalidBucket", "spec", "storage", "s3", "bucket")

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Asserting on the predictor state")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
				// TODO can we check for a more detailed error message?
			})

			It("predictor should fail to load with invalid storage key", func() {
				// modify the object with an invalid storage path
				SetString(predictorObject, "invalidKey", "spec", "storage", "s3", "secretKey")

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Asserting on the predictor state")
				ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "")
				// TODO can we check for a more detailed error message?
			})

			It("predictor should fail to load with unsupported storage type", func() {
				// modify the object with a PVC storage type, which isn't yet supported
				err := unstructured.SetNestedField(predictorObject.Object, map[string]interface{}{
					"claimName": "not-yet-supported",
				}, "spec", "storage", "persistentVolumeClaim")
				Expect(err).ToNot(HaveOccurred())

				obj := CreatePredictorAndWaitAndExpectInvalidSpec(predictorObject)

				By("Asserting on the predictor state")
				ExpectPredictorFailureInfo(obj, "InvalidPredictorSpec", false, false,
					"spec.storage.PersistentVolumeClaim is not supported")
			})

			It("predictor should fail to load with unrecognized model type", func() {
				// modify the object with an unrecognized model type
				SetString(predictorObject, "invalidModelType", "spec", "modelType", "name")

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Verifying the predictor")
				ExpectPredictorFailureInfo(obj, "NoSupportingRuntime", false, true,
					"No ServingRuntime supports specified model type")
			})

			It("predictor should fail to load with unrecognized runtime type", func() {
				// modify the object with an unrecognized runtime type
				SetString(predictorObject, "invalidRuntimeType", "spec", "runtime", "name")

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Verifying the predictor")
				ExpectPredictorFailureInfo(obj, "RuntimeNotRecognized", false, true,
					"Specified runtime name not recognized")
			})
		})
	}

	Specify("Restart pods to reset Bootstrap failure checks", func() {
		fvtClient.RestartDeploys()
	})
})

var _ = Describe("Non-ModelMesh ServingRuntime", func() {

	runtimeFile := "non-mm-runtime.yaml"
	runtimeName := "non-mm-runtime"

	BeforeEach(func() {
		var err error

		// Get a list of ServingRuntime deployments.
		deploys := fvtClient.ListDeploys()
		numDeploys := len(deploys.Items)

		// Create a non-modelmesh ServingRuntime.
		err = fvtClient.RunKubectl("create", "-f", runtimeSamplesPath+runtimeFile)
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the deployments replicas to be ready")
		WaitForStableActiveDeployState()

		By("Checking that new ServingRuntime resource exists")
		fvtClient.GetServingRuntime(runtimeName)

		By("Checking that no new deployments were created")
		deploys = fvtClient.ListDeploys()
		Expect(deploys.Items).To(HaveLen(numDeploys))
	})

	AfterEach(func() {
		err := fvtClient.RunKubectl("delete", "-f", runtimeSamplesPath+runtimeFile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("predictor should remain pending with RuntimeUnhealthy", func() {
		pred := NewPredictorForFVT("foo-predictor.yaml")

		obj := fvtClient.CreatePredictorExpectSuccess(pred)
		ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

		// Give time to process
		time.Sleep(time.Second * 5)

		obj = fvtClient.GetPredictor(obj.GetName())

		By("Verifying the predictor has failure message")
		failureInfo := GetMap(obj, "status", "lastFailureInfo")
		Expect(failureInfo).ToNot(BeNil())

		// Failure reason depends on if a ModelMesh container is up (i.e. a SR pod is running).
		// Here, just check for one of the expected failure reasons.
		Expect(failureInfo["reason"]).To(Or(Equal("RuntimeUnhealthy"), Equal("NoSupportingRuntime")))

		fvtClient.DeletePredictor(obj.GetName())
	})
})
