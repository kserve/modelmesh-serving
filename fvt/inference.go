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
	"encoding/json"
	"math"
	"os"

	"github.com/dereklstinson/cifar"
	. "github.com/onsi/gomega"

	"github.com/moverest/mnist"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"

	tfsframework "github.com/kserve/modelmesh-serving/fvt/generated/tensorflow/core/framework"
	tfsapi "github.com/kserve/modelmesh-serving/fvt/generated/tensorflow_serving/apis"
	torchserveapi "github.com/kserve/modelmesh-serving/fvt/generated/torchserve/apis"
)

// Used for checking if floats are sufficiently close enough.
const EPSILON float64 = 0.000001

// Inference for each sample predictor

// ONNX MNIST
// COS path: fvt/onnx/onnx-mnist
func ExpectSuccessfulInference_onnxMnist(predictorName string) {
	image := LoadMnistImage(0)

	// build the grpc inference call
	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "Input3",
		Shape:    []int64{1, 1, 28, 28},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: image},
	}
	inferRequest := &inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	// Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(0))
}

func ExpectSuccessfulInference_openvinoMnistTFSPredict(predictorName string) {
	image := LoadMnistImage(0)

	// build the grpc inference call
	// convert the image array of floats to raw bytes
	imageBytes, err := convertFloatArrayToRawContent(image)
	Expect(err).ToNot(HaveOccurred())

	inferRequest := &tfsapi.PredictRequest{
		ModelSpec: &tfsapi.ModelSpec{
			Name: predictorName,
		},
		Inputs: map[string]*tfsframework.TensorProto{
			"Input3": {
				Dtype: tfsframework.DataType_DT_FLOAT,
				TensorShape: &tfsframework.TensorShapeProto{
					Dim: []*tfsframework.TensorShapeProto_Dim{
						{Size: 1}, {Size: 1}, {Size: 28}, {Size: 28},
					},
				},
				TensorContent: imageBytes,
			},
		},
	}

	inferResponse, err := FVTClientInstance.RunTfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	// NOTE: ModelSpec is not included in the response, so we can't assert on the name
	// validate the activation for the digit 7 is the maximum
	activations, err := convertRawOutputContentsTo10Floats(inferResponse.Outputs["Plus214_Output_0"].TensorContent)
	max := activations[0]
	maxI := 0
	for i := 1; i < 10; i++ {
		if activations[i] > max {
			max = activations[i]
			maxI = i
		}
	}
	Expect(maxI).To(Equal(7))
	Expect(err).ToNot(HaveOccurred())
}

func ExpectSuccessfulInference_torchserveMARPredict(predictorName string) {
	imageBytes, err := os.ReadFile(TestDataPath("0.png"))
	Expect(err).ToNot(HaveOccurred())

	inferRequest := &torchserveapi.PredictionsRequest{
		ModelName: predictorName,
		Input:     map[string][]byte{"data": imageBytes},
	}

	inferResponse, err := FVTClientInstance.RunTorchserveInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
}

// PyTorch CIFAR
// COS path: fvt/pytorch/pytorch-cifar
func ExpectSuccessfulInference_pytorchCifar(predictorName string) {
	image := LoadCifarImage(1)

	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "INPUT__0",
		Shape:    []int64{1, 3, 32, 32},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: image},
	}
	inferRequest := &inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	// run the inference
	inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	// convert raw_output_contents in bytes to array of 10 float32s
	output, err := convertRawOutputContentsTo10Floats(inferResponse.GetRawOutputContents()[0])
	Expect(err).ToNot(HaveOccurred())
	Expect(math.Abs(float64(output[8]-7.343689441680908)) < EPSILON).To(BeTrue()) // the 9th class gets the highest activation for this net/image
}

func ExpectSuccessfulRESTInference_sklearnMnistSvm(predictorName string) {
	// the example model for FVT is an MNIST model provided as an example in
	// the MLServer repo:
	// https://github.com/SeldonIO/MLServer/tree/8925ad5/examples/sklearn

	// this example model takes 8x8 floating point images as input flattened
	// to a 64 float array
	image := []float32{0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0}

	body := map[string]interface{}{
		"inputs": []map[string]interface{}{
			{
				"name":     "predict",
				"shape":    []int64{1, 64},
				"datatype": "FP32",
				"data":     image,
			},
		},
	}

	jsonBytes, err := json.Marshal(body)
	Expect(err).ToNot(HaveOccurred())

	inferResponse, err := FVTClientInstance.RunKfsRestInference(predictorName, jsonBytes, false)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse).To(ContainSubstring(`"model_name":"` + predictorName))
	Expect(inferResponse).To(ContainSubstring(`"data":[8]`))
}

func ExpectSuccessfulRESTInference_xgboostMushroom(predictorName string, tls bool) {
	body := map[string]interface{}{
		"inputs": []map[string]interface{}{
			{
				"name":     "predict",
				"shape":    []int64{1, 126},
				"datatype": "FP32",
				"data":     mushroomInputData,
			},
		},
	}

	jsonBytes, err := json.Marshal(body)
	Expect(err).ToNot(HaveOccurred())

	inferResponse, err := FVTClientInstance.RunKfsRestInference(predictorName, jsonBytes, tls)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse).To(ContainSubstring(`"model_name":"` + predictorName))
	Expect(inferResponse).To(ContainSubstring(`"data":[0.0`))
}

// SKLearn MNIST SVM
// COS path: fvt/sklearn/mnist-svm
func ExpectSuccessfulInference_sklearnMnistSvm(predictorName string) {
	// the example model for FVT is an MNIST model provided as an example in
	// the MLServer repo:
	// https://github.com/SeldonIO/MLServer/tree/8925ad5/examples/sklearn

	// this example model takes 8x8 floating point images as input flattened
	// to a 64 float array
	image := []float32{0.0, 0.0, 1.0, 11.0, 14.0, 15.0, 3.0, 0.0, 0.0, 1.0, 13.0, 16.0, 12.0, 16.0, 8.0, 0.0, 0.0, 8.0, 16.0, 4.0, 6.0, 16.0, 5.0, 0.0, 0.0, 5.0, 15.0, 11.0, 13.0, 14.0, 0.0, 0.0, 0.0, 0.0, 2.0, 12.0, 16.0, 13.0, 0.0, 0.0, 0.0, 0.0, 0.0, 13.0, 16.0, 16.0, 6.0, 0.0, 0.0, 0.0, 0.0, 16.0, 16.0, 16.0, 7.0, 0.0, 0.0, 0.0, 0.0, 11.0, 13.0, 12.0, 1.0, 0.0}

	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "predict",
		Shape:    []int64{1, 64},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: image},
	}
	inferRequest := &inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	// run the inference
	inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	Expect(inferResponse.Outputs).To(Not(BeEmpty()))
	Expect(inferResponse.Outputs[0].Contents.Int64Contents).To(Not(BeEmpty()))
	Expect(inferResponse.Outputs[0].Contents.Int64Contents[0]).To(BeEquivalentTo(8))
}

// Tensorflow MNIST
// COS path: fvt/tensorflow/mnist.savedmodel
func ExpectSuccessfulInference_tensorflowMnist(predictorName string) {
	image := LoadMnistImage(0)

	// build the grpc inference call
	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "inputs",
		Shape:    []int64{1, 784},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: image},
	}
	inferRequest := inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	// First - run the inference
	inferResponse, err := FVTClientInstance.RunKfsInference(&inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(7))
}

// Keras MNIST
// COS path: fvt/tensorflow/keras-mnist
func ExpectSuccessfulInference_kerasMnist(predictorName string) {
	image := []float32{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.01176471, 0.07058824, 0.07058824, 0.07058824, 0.49411765, 0.53333336, 0.6862745, 0.10196079, 0.6509804, 1.0, 0.96862745, 0.49803922, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.11764706, 0.14117648, 0.36862746, 0.6039216, 0.6666667, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.88235295, 0.6745098, 0.99215686, 0.9490196, 0.7647059, 0.2509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.19215687, 0.93333334, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.9843137, 0.3647059, 0.32156864, 0.32156864, 0.21960784, 0.15294118, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.07058824, 0.85882354, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7764706, 0.7137255, 0.96862745, 0.94509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.3137255, 0.6117647, 0.41960785, 0.99215686, 0.99215686, 0.8039216, 0.04313726, 0.0, 0.16862746, 0.6039216, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.05490196, 0.00392157, 0.6039216, 0.99215686, 0.3529412, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.54509807, 0.99215686, 0.74509805, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.04313726, 0.74509805, 0.99215686, 0.27450982, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.13725491, 0.94509804, 0.88235295, 0.627451, 0.42352942, 0.00392157, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.31764707, 0.9411765, 0.99215686, 0.99215686, 0.46666667, 0.09803922, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.1764706, 0.7294118, 0.99215686, 0.99215686, 0.5882353, 0.10588235, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0627451, 0.3647059, 0.9882353, 0.99215686, 0.73333335, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.9764706, 0.99215686, 0.9764706, 0.2509804, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.18039216, 0.50980395, 0.7176471, 0.99215686, 0.99215686, 0.8117647, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.15294118, 0.5803922, 0.8980392, 0.99215686, 0.99215686, 0.99215686, 0.98039216, 0.7137255, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.09411765, 0.44705883, 0.8666667, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7882353, 0.30588236, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.09019608, 0.25882354, 0.8352941, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7764706, 0.31764707, 0.00784314, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.07058824, 0.67058825, 0.85882354, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.7647059, 0.3137255, 0.03529412, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.21568628, 0.6745098, 0.8862745, 0.99215686, 0.99215686, 0.99215686, 0.99215686, 0.95686275, 0.52156866, 0.04313726, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.53333336, 0.99215686, 0.99215686, 0.99215686, 0.83137256, 0.5294118, 0.5176471, 0.0627451, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	// build the grpc inference call
	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "conv2d_input",
		Shape:    []int64{1, 28, 28, 1},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: image},
	}
	inferRequest := inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	// First - run the inference
	inferResponse, err := FVTClientInstance.RunKfsInference(&inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(91))
}

// LightGBM Mushroom
// COS path: fvt/lightgbm/mushroom
func ExpectSuccessfulInference_lightgbmMushroom(predictorName string) {
	// build the grpc inference call
	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "predict",
		Shape:    []int64{1, 126},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: mushroomInputData},
	}
	inferRequest := &inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	// check that the model predicted a value close to 0
	Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp64Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
}

// XGBoost Mushroom
// COS path: fvt/xgboost/mushroom
func ExpectSuccessfulInference_xgboostMushroom(predictorName string) {
	// build the grpc inference call
	inferInput := &inference.ModelInferRequest_InferInputTensor{
		Name:     "predict",
		Shape:    []int64{1, 126},
		Datatype: "FP32",
		Contents: &inference.InferTensorContents{Fp32Contents: mushroomInputData},
	}
	inferRequest := &inference.ModelInferRequest{
		ModelName: predictorName,
		Inputs:    []*inference.ModelInferRequest_InferInputTensor{inferInput},
	}

	inferResponse, err := FVTClientInstance.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	// check that the model predicted a value close to 0
	Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
}

// Helpers

var mushroomInputData []float32 = []float32{1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0}

func LoadMnistImage(index int) []float32 {
	images, err := mnist.LoadImageFile(TestDataPath("t10k-images-idx3-ubyte.gz"))
	Expect(err).ToNot(HaveOccurred())

	imageBytes := [mnist.Width * mnist.Height]byte(*images[index])
	var imageFloat [mnist.Width * mnist.Height]float32
	for i, v := range imageBytes {
		imageFloat[i] = float32(v)
	}
	return imageFloat[:]
}

func LoadCifarImage(index int) []float32 {
	file, err := os.Open(TestDataPath("cifar_test_images.bin"))
	Expect(err).ToNot(HaveOccurred())
	images, err := cifar.Decode10(file)
	Expect(err).ToNot(HaveOccurred())

	imageBytes := images[index].RawData()
	var imageFloat [3 * 32 * 32]float32
	for i, v := range imageBytes {
		// the test PyTorch CIFAR model was trained based on:
		// - https://github.com/kserve/kserve/tree/release-0.6/docs/samples/v1alpha2/pytorch
		// - https://pytorch.org/tutorials/beginner/blitz/cifar10_tutorial.html
		// These models are trained on images with pixels normalized to the range
		// [-1 1]. The testdata contains images with pixels in bytes [0 255] that
		// must be transformed
		imageFloat[i] = (float32(v) / 127.5) - 1
	}

	return imageFloat[:]
}

func convertFloatArrayToRawContent(in []float32) ([]byte, error) {
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.LittleEndian, &in)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func convertRawOutputContentsTo10Floats(raw []byte) ([10]float32, error) {
	var floats [10]float32
	r := bytes.NewReader(raw)

	err := binary.Read(r, binary.LittleEndian, &floats)
	return floats, err
}
