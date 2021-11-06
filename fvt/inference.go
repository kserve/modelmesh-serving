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
	"math"
	"os"

	"github.com/dereklstinson/cifar"
	. "github.com/onsi/gomega"

	"github.com/moverest/mnist"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"
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

	inferResponse, err := fvtClient.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	// Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(0))
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
	inferResponse, err := fvtClient.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	// convert raw_output_contents in bytes to array of 10 float32s
	output, err := convertRawOutputContentsTo10Floats(inferResponse.GetRawOutputContents()[0])
	Expect(err).ToNot(HaveOccurred())
	Expect(math.Abs(float64(output[8]-7.343689441680908)) < EPSILON).To(BeTrue()) // the 9th class gets the highest activation for this net/image
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
	inferResponse, err := fvtClient.RunKfsInference(inferRequest)
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
	inferResponse, err := fvtClient.RunKfsInference(&inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	Expect(inferResponse.ModelName).To(HavePrefix(predictorName))
	Expect(inferResponse.RawOutputContents[0][0]).To(BeEquivalentTo(7))
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

	inferResponse, err := fvtClient.RunKfsInference(inferRequest)
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

	inferResponse, err := fvtClient.RunKfsInference(inferRequest)
	Expect(err).ToNot(HaveOccurred())
	Expect(inferResponse).ToNot(BeNil())
	// check that the model predicted a value close to 0
	Expect(math.Round(float64(inferResponse.Outputs[0].Contents.Fp32Contents[0])*10) / 10).To(BeEquivalentTo(0.0))
}

// Helpers

var mushroomInputData []float32 = []float32{1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0}

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
		// - https://github.com/kserve/kserve/tree/release-0.6/docs/samples/v1alpha2/pytorch
		// - https://pytorch.org/tutorials/beginner/blitz/cifar10_tutorial.html
		// These models are trained on images with pixels normalized to the range
		// [-1 1]. The testdata contains images with pixels in bytes [0 255] that
		// must be transformed
		imageFloat[i] = (float32(v) / 127.5) - 1
	}

	return imageFloat[:]
}

func convertRawOutputContentsTo10Floats(raw []byte) ([10]float32, error) {
	var floats [10]float32
	r := bytes.NewReader(raw)

	err := binary.Read(r, binary.LittleEndian, &floats)
	return floats, err
}
