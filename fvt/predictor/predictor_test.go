// Copyright 2022 IBM Corporation
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
	"crypto/sha1"
	"fmt"
	"time"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"
	tfsframework "github.com/kserve/modelmesh-serving/fvt/generated/tensorflow/core/framework"
	tfsapi "github.com/kserve/modelmesh-serving/fvt/generated/tensorflow_serving/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		predictorName:              "keras",
		predictorFilename:          "keras-predictor.yaml",
		currentModelPath:           "fvt/tensorflow/keras-mnist/mnist.h5",
		updatedModelPath:           "fvt/tensorflow/keras-mnistnew/mnist.h5",
		differentPredictorName:     "tf",
		differentPredictorFilename: "tf-predictor.yaml",
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
	{
		predictorName:              "openvino",
		predictorFilename:          "openvino-mnist-predictor.yaml",
		currentModelPath:           "fvt/openvino/mnist",
		updatedModelPath:           "fvt/openvino/mnist-dup",
		differentPredictorName:     "xgboost",
		differentPredictorFilename: "xgboost-predictor.yaml",
	},
	{
		predictorName:              "xgboost-fil",
		predictorFilename:          "xgboost-fil-predictor.yaml",
		currentModelPath:           "fvt/xgboost/mushroom-fil",
		updatedModelPath:           "fvt/xgboost/mushroom-fil-dup",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
	{
		predictorName:              "lightgbm-fil",
		predictorFilename:          "lightgbm-fil-predictor.yaml",
		currentModelPath:           "fvt/lightgbm/mushroom-fil",
		updatedModelPath:           "fvt/lightgbm/mushroom-fil-dup",
		differentPredictorName:     "onnx",
		differentPredictorFilename: "onnx-predictor.yaml",
	},
	// TorchServe test is currently disabled
	// {
	// 	predictorName:              "pytorch-mar",
	// 	predictorFilename:          "pytorch-mar-predictor.yaml",
	// 	currentModelPath:           "fvt/pytorch/pytorch-mar/mnist.mar",
	// 	updatedModelPath:           "fvt/pytorch/pytorch-mar-dup/mnist.mar",
	// 	differentPredictorName:     "pytorch",
	// 	differentPredictorFilename: "pytorch-predictor.yaml",
	// },
}

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

var _ = Describe("Predictor", func() {
	// Many tests in this block assume a stable state of scaled up deployments
	// which may not be the case if other Describe blocks run first. So we want to
	// confirm the expected state before executing any test, but we also don't
	// want to check the deployment state for each test since that would waste
	// time. The sole purpose of the following test case is to ensure we are
	// starting from the desired state.
	for _, p := range predictorsArray {
		predictor := p
		var _ = Describe("create "+predictor.predictorName+" predictor", Ordered, func() {

			It("should successfully load a model", func() {
				predictorObject := NewPredictorForFVT(predictor.predictorFilename)
				CreatePredictorAndWaitAndExpectLoaded(predictorObject)

				// clean up
				FVTClientInstance.DeletePredictor(predictorObject.GetName())
			})

			It("should successfully load two models of different types", func() {
				predictorObject := NewPredictorForFVT(predictor.predictorFilename)
				predictorName := predictorObject.GetName()

				differentPredictorObject := NewPredictorForFVT(predictor.differentPredictorFilename)
				differentPredictorName := differentPredictorObject.GetName()

				By("Creating the " + predictor.predictorName + " predictor")
				predictorWatcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
				defer predictorWatcher.Stop()
				predictorObject = FVTClientInstance.CreatePredictorExpectSuccess(predictorObject)
				ExpectPredictorState(predictorObject, false, "Pending", "", "UpToDate")

				By("Creating the " + predictor.differentPredictorName + " predictor")
				differentPredictorWatcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + differentPredictorName}, DefaultTimeout)
				defer differentPredictorWatcher.Stop()
				differentPredictorObject = FVTClientInstance.CreatePredictorExpectSuccess(differentPredictorObject)
				ExpectPredictorState(differentPredictorObject, false, "Pending", "", "UpToDate")

				By("Waiting for the first predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, predictorWatcher)
				By("Waiting for the second predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, differentPredictorWatcher)

				By("Verifying the predictors")
				predictorObject = FVTClientInstance.GetPredictor(predictorName)
				ExpectPredictorState(predictorObject, true, "Loaded", "", "UpToDate")
				differentPredictorObject = FVTClientInstance.GetPredictor(differentPredictorName)
				ExpectPredictorState(differentPredictorObject, true, "Loaded", "", "UpToDate")

				// clean up
				FVTClientInstance.DeletePredictor(predictorName)
				FVTClientInstance.DeletePredictor(differentPredictorName)
			})

			It("should successfully load two models of the same type", func() {
				By("Creating the first " + predictor.predictorName + " predictor")
				pred1 := NewPredictorForFVT(predictor.predictorFilename)
				watcher1 := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + pred1.GetName()}, DefaultTimeout)
				defer watcher1.Stop()
				obj1 := FVTClientInstance.CreatePredictorExpectSuccess(pred1)
				ExpectPredictorState(obj1, false, "Pending", "", "UpToDate")

				By("Creating a second " + predictor.predictorName + " predictor")
				pred2 := NewPredictorForFVT(predictor.predictorFilename)
				watcher2 := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + pred2.GetName()}, DefaultTimeout)
				defer watcher2.Stop()
				obj2 := FVTClientInstance.CreatePredictorExpectSuccess(pred2)
				ExpectPredictorState(obj2, false, "Pending", "", "UpToDate")

				By("Waiting for the first predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher1)
				By("Waiting for the second predictor to be 'Loaded'")
				// "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be
				WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher2)

				By("Verifying the predictors")
				obj1 = FVTClientInstance.GetPredictor(pred1.GetName())
				ExpectPredictorState(obj1, true, "Loaded", "", "UpToDate")
				obj2 = FVTClientInstance.GetPredictor(pred2.GetName())
				ExpectPredictorState(obj2, true, "Loaded", "", "UpToDate")

				// clean up
				FVTClientInstance.DeletePredictor(pred1.GetName())
				FVTClientInstance.DeletePredictor(pred2.GetName())
			})

		})

		var _ = Describe("update "+predictor.predictorName+" predictor", Ordered, func() {
			var predictorObject *unstructured.Unstructured
			var predictorName string

			BeforeEach(func() {
				// load the test predictor object
				predictorObject = NewPredictorForFVT(predictor.predictorFilename)
				predictorName = predictorObject.GetName()

				CreatePredictorAndWaitAndExpectLoaded(predictorObject)
			})

			AfterEach(func() {
				FVTClientInstance.DeletePredictor(predictorName)
			})

			It("should successfully update and reload the model", func() {
				// verify starting model path
				obj := FVTClientInstance.GetPredictor(predictorName)
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.currentModelPath))

				// modify the object with a new valid path
				SetString(predictorObject, predictor.updatedModelPath, "spec", "path")

				obj = UpdatePredictorAndWaitAndExpectLoaded(predictorObject)

				By("Verifying the predictors")
				Expect(GetString(obj, "spec", "path")).To(Equal(predictor.updatedModelPath))
			})

			It("should fail to load the target model with invalid path", func() {
				// verify starting model path
				obj := FVTClientInstance.GetPredictor(predictorName)
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

	var _ = Describe("test transition of Predictor between models", Ordered, func() {
		var predictorObject *unstructured.Unstructured
		var predictorName string

		BeforeEach(func() {
			// load the test predictor object from tf-predictor sample yaml file
			predictorObject = NewPredictorForFVT("tf-predictor.yaml")
			predictorName = MakeUniquePredictorName("transition-predictor")
			predictorObject.SetName(predictorName)

			CreatePredictorAndWaitAndExpectLoaded(predictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			FVTClientInstance.DeletePredictor(predictorName)
		})

		It("should successfully run an inference, update the model and run an inference again on the updated model", func() {
			// verify starting model path
			obj := FVTClientInstance.GetPredictor(predictorName)
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

	var _ = Describe("Missing storage field", Ordered, func() {
		var predictorObject *unstructured.Unstructured

		BeforeEach(func() {
			// load the test predictor object
			predictorObject = NewPredictorForFVT("no-storage-predictor.yaml")
		})

		AfterEach(func() {
			FVTClientInstance.DeletePredictor(predictorObject.GetName())
		})

		It("predictor should fail to load with invalid storage path", func() {
			obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

			By("Asserting on the predictor state")
			ExpectPredictorFailureInfo(obj, "ModelLoadFailed", true, true, "Predictor Storage field missing")
		})
	})

	var _ = Describe("TensorFlow inference", Ordered, func() {
		var tfPredictorObject *unstructured.Unstructured
		var tfPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			tfPredictorObject = NewPredictorForFVT("tf-predictor.yaml")
			rd := fmt.Sprintf("%x", sha1.Sum([]byte(time.Now().String())))
			randomName := fmt.Sprintf("minimal-tf-predictor-%s", rd[len(rd)-5:])
			SetString(tfPredictorObject, randomName, "metadata", "name")

			tfPredictorName = tfPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(tfPredictorObject)

			WaitForStableActiveDeployState(time.Second * 60)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(tfPredictorName)
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
			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("model expects 'FP32'"))
			Expect(inferResponse).To(BeNil())
		})

		It("should return model metadata", func() {
			modelMetadataRequest := &inference.ModelMetadataRequest{
				Name: tfPredictorName,
			}
			modelMetadataResponse, err := FVTClientInstance.RunKfsModelMetadata(modelMetadataRequest)

			Expect(err).ToNot(HaveOccurred())
			Expect(modelMetadataResponse).ToNot(BeNil())

			Expect(modelMetadataResponse.Name).To(HavePrefix(tfPredictorName))
			Expect(modelMetadataResponse.Inputs).To(HaveLen(1))
			Expect(modelMetadataResponse.Outputs).To(HaveLen(1))

			Expect(modelMetadataResponse.Inputs[0].Name).To(Equal("inputs"))
			Expect(modelMetadataResponse.Inputs[0].Shape).To(Equal([]int64{-1, 784}))
			Expect(modelMetadataResponse.Inputs[0].Datatype).To(Equal("FP32"))
		})
	})

	var _ = Describe("Keras inference", Ordered, func() {
		var kerasPredictorObject *unstructured.Unstructured
		var kerasPredictorName string

		BeforeEach(func() {
			// load the test predictor object
			kerasPredictorObject = NewPredictorForFVT("keras-predictor.yaml")
			kerasPredictorName = kerasPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(kerasPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			FVTClientInstance.DeletePredictor(kerasPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_kerasMnist(kerasPredictorName)
		})

		It("should successfully run an inference on an updated model", func() {

			By("Updating the predictor with new model path")
			SetString(kerasPredictorObject, "fvt/tensorflow/keras-mnistnew/mnist.h5", "spec", "path")

			UpdatePredictorAndWaitAndExpectLoaded(kerasPredictorObject)

			ExpectSuccessfulInference_kerasMnist(kerasPredictorName)
		})

		It("should fail to run an inference with invalid shape", func() {
			image := []float32{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.01176471, 0.07058824, 0.07058824, 0.07058824, 0.49411765, 0.53333336, 0.6862745, 0.10196079, 0.6509804, 1.0, 0.96862745, 0.49803922, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.11764706, 0.14117648, 0.36862746, 0.6039216, 0.6666667, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.88235295, 0.6745098, 0.99215686, 0.9490196, 0.7647059, 0.2509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.19215687, 0.93333334, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.9843137, 0.3647059, 0.32156864, 0.32156864, 0.21960784, 0.15294118, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.07058824, 0.85882354, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7764706, 0.7137255, 0.96862745, 0.94509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.3137255, 0.6117647, 0.41960785, 0.99215686, 0.99215686, 0.8039216, 0.04313726, 0.0, 0.16862746, 0.6039216, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.05490196, 0.00392157, 0.6039216, 0.99215686, 0.3529412, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.54509807, 0.99215686, 0.74509805, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.04313726, 0.74509805, 0.99215686, 0.27450982, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.13725491, 0.94509804, 0.88235295, 0.627451, 0.42352942, 0.00392157, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.31764707, 0.9411765, 0.99215686, 0.99215686, 0.46666667, 0.09803922, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.1764706, 0.7294118, 0.99215686, 0.99215686, 0.5882353, 0.10588235, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0627451, 0.3647059, 0.9882353, 0.99215686, 0.73333335, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.9764706, 0.99215686, 0.9764706, 0.2509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.18039216, 0.50980395, 0.7176471, 0.99215686, 0.99215686, 0.8117647, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.15294118, 0.5803922, 0.8980392, 0.99215686, 0.99215686, 0.99215686, 0.98039216, 0.7137255, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.09411765, 0.44705883, 0.8666667, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7882353, 0.30588236, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.09019608, 0.25882354, 0.8352941, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7764706, 0.31764707, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.07058824, 0.67058825, 0.85882354, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7647059, 0.3137255, 0.03529412, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.21568628, 0.6745098, 0.8862745, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.95686275, 0.52156866, 0.04313726, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.53333336, 0.99215686, 0.99215686, 0.99215686, 0.83137256, 0.5294118, 0.5176471, 0.0627451, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "conv2d_input",
				Shape:    []int64{1, 783},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: image},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: kerasPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(inferResponse).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input"))
		})

		It("should return model metadata", func() {
			modelMetadataRequest := &inference.ModelMetadataRequest{
				Name: kerasPredictorName,
			}
			modelMetadataResponse, err := FVTClientInstance.RunKfsModelMetadata(modelMetadataRequest)

			fmt.Println(modelMetadataResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(modelMetadataResponse).ToNot(BeNil())

			Expect(modelMetadataResponse.Name).To(HavePrefix(kerasPredictorName))
			Expect(modelMetadataResponse.Inputs).To(HaveLen(1))
			Expect(modelMetadataResponse.Outputs).To(HaveLen(1))

			Expect(modelMetadataResponse.Inputs[0].Name).To(Equal("conv2d_input"))
			Expect(modelMetadataResponse.Inputs[0].Shape).To(Equal([]int64{-1, 28, 28, 1}))
			Expect(modelMetadataResponse.Inputs[0].Datatype).To(Equal("FP32"))
		})
	})

	var _ = Describe("ONNX inference", Ordered, func() {
		var onnxPredictorObject *unstructured.Unstructured
		var onnxPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			onnxPredictorObject = NewPredictorForFVT("onnx-predictor.yaml")
			onnxPredictorName = onnxPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(onnxPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(onnxPredictorName)
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

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(inferResponse).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input"))
		})

		It("should return model metadata", func() {
			modelMetadataRequest := &inference.ModelMetadataRequest{
				Name: onnxPredictorName,
			}
			modelMetadataResponse, err := FVTClientInstance.RunKfsModelMetadata(modelMetadataRequest)

			Expect(err).ToNot(HaveOccurred())
			Expect(modelMetadataResponse).ToNot(BeNil())
			Expect(modelMetadataResponse.Name).To(HavePrefix(onnxPredictorName))

			Expect(modelMetadataResponse.Inputs).To(HaveLen(1))
			Expect(modelMetadataResponse.Outputs).To(HaveLen(1))

			Expect(modelMetadataResponse.Inputs[0].Name).To(Equal("Input3"))
			Expect(modelMetadataResponse.Inputs[0].Shape).To(Equal([]int64{1, 1, 28, 28}))
			Expect(modelMetadataResponse.Inputs[0].Datatype).To(Equal("FP32"))
		})
	})

	var _ = Describe("OVMS Inference", Ordered, func() {
		var openvinoPredictorObject *unstructured.Unstructured
		var openvinoPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			openvinoPredictorObject = NewPredictorForFVT("openvino-mnist-predictor.yaml")
			openvinoPredictorName = openvinoPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(openvinoPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(openvinoPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_openvinoMnistTFSPredict(openvinoPredictorName)
		})

		It("should fail to run an inference with invalid shape", func() {
			inferRequest := &tfsapi.PredictRequest{
				ModelSpec: &tfsapi.ModelSpec{
					Name: openvinoPredictorName,
				},
				Inputs: map[string]*tfsframework.TensorProto{
					"Input3": {
						Dtype: tfsframework.DataType_DT_FLOAT,
						TensorShape: &tfsframework.TensorShapeProto{
							Dim: []*tfsframework.TensorShapeProto_Dim{
								{Size: 28}, {Size: 28},
							},
						},
						// invalid shape error occurs before the content is inspected
						// TensorContent:
					},
				},
			}

			inferResponse, err := FVTClientInstance.RunTfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Expect(inferResponse).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("INVALID_ARGUMENT: Invalid number of shape dimensions"))
		})
	})
	// TorchServe test is currently disabled
	var _ = XDescribe("TorchServe Inference", Ordered, func() {
		var torchservePredictorObject *unstructured.Unstructured
		var torchservePredictorName string

		BeforeAll(func() {
			// load the test predictor object
			torchservePredictorObject = NewPredictorForFVT("pytorch-mar-predictor.yaml")
			torchservePredictorName = torchservePredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(torchservePredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(torchservePredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_torchserveMARPredict(torchservePredictorName)
		})
	})

	var _ = Describe("MLServer inference", Ordered, func() {
		var mlsPredictorObject *unstructured.Unstructured
		var mlsPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			mlsPredictorObject = NewPredictorForFVT("mlserver-sklearn-predictor.yaml")
			mlsPredictorName = mlsPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(mlsPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(mlsPredictorName)
		})

		It("should successfully run inference using GRPC", func() {
			ExpectSuccessfulInference_sklearnMnistSvm(mlsPredictorName)
		})

		It("should successfully run inference using REST proxy", func() {
			ExpectSuccessfulRESTInference_sklearnMnistSvm(mlsPredictorName)
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
			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INTERNAL: builtins.ValueError"))
		})

		It("should return model metadata", func() {
			modelMetadataRequest := &inference.ModelMetadataRequest{
				Name: mlsPredictorName,
			}
			modelMetadataResponse, err := FVTClientInstance.RunKfsModelMetadata(modelMetadataRequest)

			Expect(err).ToNot(HaveOccurred())
			Expect(modelMetadataResponse).ToNot(BeNil())
			// Only name is returned.
			Expect(modelMetadataResponse.Name).To(HavePrefix(mlsPredictorName))
		})
	})

	var _ = Describe("XGBoost inference", Ordered, func() {
		var xgboostPredictorObject *unstructured.Unstructured
		var xgboostPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			xgboostPredictorObject = NewPredictorForFVT("xgboost-predictor.yaml")
			xgboostPredictorName = xgboostPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(xgboostPredictorName)
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

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INTERNAL: builtins.ValueError: cannot reshape array"))
		})
	})

	var _ = Describe("XGBoost FIL inference", Ordered, func() {
		var xgboostPredictorObject *unstructured.Unstructured
		var xgboostPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			xgboostPredictorObject = NewPredictorForFVT("xgboost-fil-predictor.yaml")
			xgboostPredictorName = xgboostPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(xgboostPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_xgboostFILMushroom(xgboostPredictorName)
		})

		It("should fail with invalid shape", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "input__0",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: []float32{}},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: xgboostPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input 'input__0'"))
		})
	})

	var _ = Describe("Pytorch inference", Ordered, func() {
		var ptPredictorObject *unstructured.Unstructured
		var ptPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			ptPredictorObject = NewPredictorForFVT("pytorch-predictor.yaml")
			ptPredictorName = ptPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(ptPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(ptPredictorName)
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
			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Log.Info(err.Error())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input"))
			Expect(inferResponse).To(BeNil())
		})

		It("should return model metadata", func() {
			modelMetadataRequest := &inference.ModelMetadataRequest{
				Name: ptPredictorName,
			}
			modelMetadataResponse, err := FVTClientInstance.RunKfsModelMetadata(modelMetadataRequest)

			Expect(err).ToNot(HaveOccurred())
			Expect(modelMetadataResponse).ToNot(BeNil())
			Expect(modelMetadataResponse.Name).To(HavePrefix(ptPredictorName))

			Expect(modelMetadataResponse.Inputs).To(HaveLen(1))
			Expect(modelMetadataResponse.Outputs).To(HaveLen(1))

			Expect(modelMetadataResponse.Inputs[0].Name).To(Equal("INPUT__0"))
			Expect(modelMetadataResponse.Inputs[0].Shape).To(Equal([]int64{-1, 3, 32, 32}))
			Expect(modelMetadataResponse.Inputs[0].Datatype).To(Equal("FP32"))
		})

	})

	// This an inference testcase for pytorch that mandates schema in config.pbtxt
	// However config.pbtxt (in COS) by default does not include schema section, instead
	// schema passed in Predictor CR is updated (in config.pbtxt) after model downloaded.
	var _ = Describe("Pytorch inference with schema", Ordered, func() {
		var ptPredictorObject *unstructured.Unstructured
		var ptPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			ptPredictorObject = NewPredictorForFVT("pytorch-predictor-withschema.yaml")
			ptPredictorName = ptPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(ptPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(ptPredictorName)
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
			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
			Expect(err).To(HaveOccurred())
			Log.Info(err.Error())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input"))
			Expect(inferResponse).To(BeNil())
		})
	})

	var _ = Describe("LightGBM inference", Ordered, func() {
		var lightGBMPredictorObject *unstructured.Unstructured
		var lightGBMPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			lightGBMPredictorObject = NewPredictorForFVT("lightgbm-predictor.yaml")
			lightGBMPredictorName = lightGBMPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(lightGBMPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(lightGBMPredictorName)
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

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INTERNAL: builtins.ValueError: cannot reshape array"))
		})
	})

	var _ = Describe("LightGBM FIL inference", Ordered, func() {
		var lightGBMPredictorObject *unstructured.Unstructured
		var lightGBMPredictorName string

		BeforeAll(func() {
			// load the test predictor object
			lightGBMPredictorObject = NewPredictorForFVT("lightgbm-fil-predictor.yaml")
			lightGBMPredictorName = lightGBMPredictorObject.GetName()

			CreatePredictorAndWaitAndExpectLoaded(lightGBMPredictorObject)

			err := FVTClientInstance.ConnectToModelServing(Insecure)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterAll(func() {
			FVTClientInstance.DeletePredictor(lightGBMPredictorName)
		})

		It("should successfully run an inference", func() {
			ExpectSuccessfulInference_lightgbmFILMushroom(lightGBMPredictorName)
		})

		It("should fail with invalid shape input", func() {
			// build the grpc inference call
			inferInput := &inference.ModelInferRequest_InferInputTensor{
				Name:     "input__0",
				Shape:    []int64{1, 28777},
				Datatype: "FP32",
				Contents: &inference.InferTensorContents{Fp32Contents: []float32{}},
			}
			inferRequest := &inference.ModelInferRequest{
				ModelName: lightGBMPredictorName,
				Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
			}

			inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)

			Expect(inferResponse).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected shape for input 'input__0'"))
		})
	})
})

// These tests verify that an invalid Predictor fails to load. These are in a
// separate block in part because a high frequency of failures can trigger Model
// Mesh's "bootstrap failure" mechanism which prevents rollouts of new pods that
// fail frequently by causing them to fail the readiness check.
// At the end of the suite, all runtime deployments are rolled out to remove
// any that may have gone unready.
var _ = Describe("Invalid Predictors", func() {
	var predictorObject *unstructured.Unstructured

	for _, p := range predictorsArray {
		predictor := p

		Describe("invalid cases for the "+predictor.predictorName+" predictor", func() {
			BeforeEach(func() {
				// load the test predictor object
				predictorObject = NewPredictorForFVT(predictor.predictorFilename)
			})

			AfterEach(func() {
				FVTClientInstance.DeletePredictor(predictorObject.GetName())
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

			It("predictor should fail to load with unrecognized model type", func() {
				// modify the object with an unrecognized model type
				SetString(predictorObject, "invalidModelType", "spec", "modelType", "name")

				// remove runtime field for predictors that have a runtime spec for this test
				if CheckIfStringExists(predictorObject, "spec", "runtime", "name") {
					SetString(predictorObject, "", "spec", "runtime", "name")
				}

				obj := CreatePredictorAndWaitAndExpectFailed(predictorObject)

				By("Verifying the predictor")
				ExpectPredictorFailureInfo(obj, "NoSupportingRuntime", false, true,
					"No ServingRuntime supports specified model type and/or protocol")
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
})

var _ = Describe("Non-ModelMesh ServingRuntime", func() {
	runtimeFile := "non-mm-runtime.yaml"
	runtimeName := "non-mm-runtime"

	BeforeEach(func() {
		var err error

		// Get a list of ServingRuntime deployments.
		deploys := FVTClientInstance.ListDeploys()
		numDeploys := len(deploys.Items)

		// Create a non-modelmesh ServingRuntime.
		err = FVTClientInstance.RunKubectl("create", "-f", TestDataPath(RuntimeSamplesPath+runtimeFile))
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the deployments replicas to be ready")
		WaitForStableActiveDeployState(TimeForStatusToStabilize)

		By("Checking that new ServingRuntime resource exists")
		FVTClientInstance.GetServingRuntime(runtimeName)

		By("Checking that no new deployments were created")
		deploys = FVTClientInstance.ListDeploys()
		Expect(deploys.Items).To(HaveLen(numDeploys))
	})

	AfterEach(func() {
		err := FVTClientInstance.RunKubectl("delete", "-f", TestDataPath(RuntimeSamplesPath+runtimeFile))
		Expect(err).ToNot(HaveOccurred())
	})

	It("predictor should remain pending with RuntimeUnhealthy", func() {
		pred := NewPredictorForFVT("foo-predictor.yaml")

		obj := FVTClientInstance.CreatePredictorExpectSuccess(pred)
		ExpectPredictorState(obj, false, "Pending", "", "UpToDate")

		// Give time to process
		time.Sleep(time.Second * 5)

		obj = FVTClientInstance.GetPredictor(obj.GetName())

		By("Verifying the predictor has failure message")
		failureInfo := GetMap(obj, "status", "lastFailureInfo")
		Expect(failureInfo).ToNot(BeNil())

		// Failure reason depends on if a ModelMesh container is up (i.e. a SR pod is running).
		// Here, just check for one of the expected failure reasons.
		Expect(failureInfo["reason"]).To(Or(Equal("RuntimeUnhealthy"), Equal("NoSupportingRuntime")))

		FVTClientInstance.DeletePredictor(obj.GetName())
	})
})

var _ = Describe("Inference service", Ordered, func() {
	for _, i := range inferenceArray {
		var _ = Describe("test "+i.name+" isvc", Ordered, func() {
			var isvcName string

			It("should successfully load a model", func() {
				isvcObject := NewIsvcForFVT(i.inferenceServiceFileName)
				isvcName = isvcObject.GetName()
				CreateIsvcAndWaitAndExpectReady(isvcObject, PredictorTimeout)

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

// The TLS tests `Describe` block should be the last one in the list to
// improve efficiency of the tests. Any test after the TLS tests would need
// to wait for the configuration changes to roll out to all Deployments.
// The TLS tests must run "Serial" (not in parallel with any other tests) since
// the configmap changes trigger deployment rollouts causing runtime pods to
// get terminated and any concurrently running inference requests would fail as
// the gRPC connection to terminating pods breaks.
var _ = Describe("TLS XGBoost inference", Ordered, Serial, func() {
	var xgboostPredictorObject *unstructured.Unstructured
	var xgboostPredictorName string

	AfterAll(func() {
		FVTClientInstance.SetDefaultUserConfigMap()
	})

	It("should successfully run an inference with basic TLS", func() {
		By("Updating the user ConfigMap to for basic TLS")

		FVTClientInstance.UpdateConfigMapTLS(BasicTLSConfig)

		By("Waiting for stable deploy state after UpdateConfigMapTLS")
		WaitForStableActiveDeployState(time.Second * 60)

		// load the test predictor object
		xgboostPredictorObject = NewPredictorForFVT("xgboost-predictor.yaml")
		xgboostPredictorName = xgboostPredictorObject.GetName()
		CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)

		By("Creating a new connection to ModelServing")
		// ensure we are establishing a new connection after the TLS change
		FVTClientInstance.DisconnectFromModelServing()

		var timeAsleep int
		var mmeshErr error
		for timeAsleep <= 30 {
			mmeshErr = FVTClientInstance.ConnectToModelServing(TLS)

			if mmeshErr == nil {
				Log.Info("Successfully connected to model mesh tls")
				break
			}

			Log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
			FVTClientInstance.DisconnectFromModelServing()
			timeAsleep += 10
			time.Sleep(time.Second * time.Duration(timeAsleep))
		}

		Expect(mmeshErr).ToNot(HaveOccurred())

		By("Expect inference to succeed")
		ExpectSuccessfulInference_xgboostMushroom(xgboostPredictorName)

		By("Expect inference to succeed via REST proxy")
		ExpectSuccessfulRESTInference_xgboostMushroom(xgboostPredictorName, true)

		// cleanup the predictor
		FVTClientInstance.DeletePredictor(xgboostPredictorName)

		// disconnect because TLS config will change
		FVTClientInstance.DisconnectFromModelServing()
	})

	It("should successfully run an inference with mutual TLS", func() {
		By("Updating user config for Mutual TLS")
		FVTClientInstance.UpdateConfigMapTLS(MutualTLSConfig)

		By("Waiting for stable deploy state after MutualTLSConfig")
		WaitForStableActiveDeployState(time.Second * 60)

		// load the test predictor object
		xgboostPredictorObject = NewPredictorForFVT("xgboost-predictor.yaml")
		xgboostPredictorName = xgboostPredictorObject.GetName()
		CreatePredictorAndWaitAndExpectLoaded(xgboostPredictorObject)

		By("Creating a new connection to ModelServing")
		// ensure we are establishing a new connection after the TLS change
		FVTClientInstance.DisconnectFromModelServing()

		var timeAsleep int
		var mmeshErr error
		for timeAsleep <= 30 {
			mmeshErr = FVTClientInstance.ConnectToModelServing(MutualTLS)

			if mmeshErr == nil {
				Log.Info("Successfully connected to model mesh tls")
				break
			}

			Log.Info(fmt.Sprintf("Error found, retrying connecting to model-mesh: %s", mmeshErr.Error()))
			FVTClientInstance.DisconnectFromModelServing()
			timeAsleep += 10
			time.Sleep(time.Second * time.Duration(timeAsleep))
		}
		Expect(mmeshErr).ToNot(HaveOccurred())

		By("Expect inference to succeed")
		ExpectSuccessfulInference_xgboostMushroom(xgboostPredictorName)

		// cleanup the predictor
		FVTClientInstance.DeletePredictor(xgboostPredictorName)

		// disconnect because TLS config will change
		FVTClientInstance.DisconnectFromModelServing()
	})

	It("should fail to run inference when the server has mutual TLS but the client does not present a certificate", func() {
		FVTClientInstance.UpdateConfigMapTLS(MutualTLSConfig)

		By("Waiting for the deployments replicas to be ready")
		WaitForStableActiveDeployState(TimeForStatusToStabilize)

		By("Expect a new connection to fail")
		// since the connection switches to TLS, ensure we are establishing a new connection
		FVTClientInstance.DisconnectFromModelServing()
		// this test is expected to fail to connect due to the TLS cert, so we
		// don't retry if it fails
		mmeshErr := FVTClientInstance.ConnectToModelServing(TLS)
		Expect(mmeshErr).To(HaveOccurred())
	})
})
