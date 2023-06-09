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

package hpa

import (
	"strings"
	"time"

	"github.com/kserve/kserve/pkg/constants"
	mmcontstant "github.com/kserve/modelmesh-serving/pkg/constants"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"

	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Scaling of runtime deployments with HPA Autoscaler", Ordered, func() {
	// constants
	testPredictorObject := NewPredictorForFVT("mlserver-sklearn-predictor.yaml")
	// runtime expected to serve the test predictor
	expectedRuntimeName := "mlserver-1.x"

	// checkDeploymentState returns the replicas value for the expected runtime
	// and expects others to be scaled to zero
	checkDeploymentState := func() int32 {
		deployments := FVTClientInstance.ListDeploys()
		var replicas int32
		for _, d := range deployments.Items {
			Log.Info("Checking deployment scale", "name", d.ObjectMeta.Name)
			// the service prefix may change
			if strings.HasSuffix(d.ObjectMeta.Name, expectedRuntimeName) {
				// since we list existing deploys Replicas should never be nil
				replicas = *d.Spec.Replicas
			} else {
				Expect(*d.Spec.Replicas).To(BeEquivalentTo(int32(0)))
			}
		}
		return replicas
	}
	expectScaledToTargetReplicas := func(targetReplicas int32) {
		replicas := checkDeploymentState()
		Expect(replicas).To(BeEquivalentTo(targetReplicas))
	}

	expectScaledToZero := func() {
		replicas := checkDeploymentState()
		Expect(replicas).To(BeEquivalentTo(int32(0)))
	}

	checkHPAState := func() *hpav2beta2.HorizontalPodAutoscaler {
		hpaList := FVTClientInstance.ListHPAs()

		var hpa *hpav2beta2.HorizontalPodAutoscaler
		if len(hpaList.Items) == 0 {
			hpa = nil
		} else {
			for _, d := range hpaList.Items {
				Log.Info("Checking if HPA exist", "name", d.ObjectMeta.Name)
				// the service prefix may change
				if strings.HasSuffix(d.ObjectMeta.Name, expectedRuntimeName) {
					hpa = &d
				}
			}
		}
		return hpa
	}

	expectHPAExist := func(exist bool) {
		hpa := checkHPAState()
		if exist {
			Expect(hpa).NotTo(BeNil())
		} else {
			Expect(hpa).To(BeNil())
		}
	}

	expectHPAMinReplicas := func(minReplicas int32) {
		hpa := checkHPAState
		Expect(*hpa().Spec.MinReplicas).To(Equal(minReplicas))
	}

	expectHPAMaxReplicas := func(maxReplicas int32) {
		hpa := checkHPAState
		Expect(hpa().Spec.MaxReplicas).To(Equal(maxReplicas))
	}

	expectHPATargetUtilizationPercentage := func(targetUtilizationPercentage int32) {
		hpa := checkHPAState
		Expect(*hpa().Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(targetUtilizationPercentage))
	}

	expectHPAResourceName := func(resourceName corev1.ResourceName) {
		hpa := checkHPAState
		Expect(hpa().Spec.Metrics[0].Resource.Name).To(Equal(resourceName))
	}

	deployTestPredictorAndCheckDefaultHPA := func() {
		CreatePredictorAndWaitAndExpectLoaded(testPredictorObject)
		expectScaledToTargetReplicas(int32(constants.DefaultMinReplicas))

		// check HPA object
		expectHPAExist(true)
		expectHPAMinReplicas(1)
		expectHPAMaxReplicas(1)
		expectHPATargetUtilizationPercentage(80)
		expectHPAResourceName(corev1.ResourceCPU)
	}
	BeforeAll(func() {
		srAnnotations := make(map[string]interface{})
		srAnnotations[constants.AutoscalerClass] = string(constants.AutoscalerClassHPA)

		FVTClientInstance.SetServingRuntimeAnnotation(expectedRuntimeName, srAnnotations)
	})

	BeforeEach(func() {
		FVTClientInstance.DeleteAllPredictors()
		// ensure a stable deploy state
		WaitForStableActiveDeployState(10 * time.Second)
	})

	AfterAll(func() {
		FVTClientInstance.DeleteAllPredictors()

		annotations := make(map[string]interface{})
		FVTClientInstance.SetServingRuntimeAnnotation(expectedRuntimeName, annotations)
	})

	Context("when there are no predictors", func() {
		It("Scale all runtimes down", func() {
			// check that all runtimes are scaled to zero
			By("Check ScaleToZero and No HPA")
			expectScaledToZero()
			expectHPAExist(false)
		})
		It("Scale all runtimes down after a created test predictor is deleted", func() {
			By("Creating a test predictor for one Runtime")
			deployTestPredictorAndCheckDefaultHPA()

			By("Delete all predictors")
			FVTClientInstance.DeleteAllPredictors()
			// ensure a stable deploy state
			WaitForStableActiveDeployState(10 * time.Second)

			By("Check ScaleToZero and No HPA")
			expectScaledToZero()
			expectHPAExist(false)
		})
	})
	Context("when there are predictors", func() {
		It("Creating a predictor should create an HPA and scale up the runtime to minReplicas of HPA", func() {
			By("Creating a test predictor for one Runtime")
			deployTestPredictorAndCheckDefaultHPA()
		})
		It("Scaleup/Scaledown and Change targetUtilizationPercentage by an annotation in ServingRuntime", func() {
			By("Creating a test predictor for one Runtime")
			deployTestPredictorAndCheckDefaultHPA()

			// ScaleUp Test
			By("ScaleUp to min(2)/max(4): " + mmcontstant.MinScaleAnnotationKey)
			By("Increase TargetUtilizationPercentage to 90: " + constants.TargetUtilizationPercentage)
			By("Change Metrics to memory: " + constants.TargetUtilizationPercentage)
			srAnnotationsScaleUp := make(map[string]interface{})
			srAnnotationsScaleUp[constants.AutoscalerClass] = string(constants.AutoscalerClassHPA)
			srAnnotationsScaleUp[mmcontstant.MinScaleAnnotationKey] = "2"
			srAnnotationsScaleUp[mmcontstant.MaxScaleAnnotationKey] = "4"
			srAnnotationsScaleUp[constants.TargetUtilizationPercentage] = "90"
			srAnnotationsScaleUp[constants.AutoscalerMetrics] = "memory"

			// set modified annotations
			FVTClientInstance.SetServingRuntimeAnnotation(expectedRuntimeName, srAnnotationsScaleUp)

			// sleep to give time for changes to propagate to the deployment
			time.Sleep(10 * time.Second)
			WaitForStableActiveDeployState(time.Second * 30)

			// check that all runtimes except the one are scaled up to minimum replicas of HPA
			expectScaledToTargetReplicas(2)

			// check HPA
			expectHPAExist(true)
			expectHPAMinReplicas(2)
			expectHPAMaxReplicas(4)
			expectHPATargetUtilizationPercentage(90)
			expectHPAResourceName(corev1.ResourceMemory)

			// ScaleDown Test
			By("ScaleDown to min(1)/max(1): " + mmcontstant.MinScaleAnnotationKey)
			By("Decrease TargetUtilizationPercentage to 80: " + constants.TargetUtilizationPercentage)
			By("Change Metrics to cpu: " + constants.TargetUtilizationPercentage)
			srAnnotationsScaleDown := make(map[string]interface{})
			srAnnotationsScaleDown[constants.AutoscalerClass] = string(constants.AutoscalerClassHPA)
			srAnnotationsScaleDown[mmcontstant.MinScaleAnnotationKey] = "1"
			srAnnotationsScaleDown[mmcontstant.MaxScaleAnnotationKey] = "1"
			srAnnotationsScaleDown[constants.TargetUtilizationPercentage] = "80"
			srAnnotationsScaleDown[constants.AutoscalerMetrics] = "cpu"

			// set modified annotations
			FVTClientInstance.SetServingRuntimeAnnotation(expectedRuntimeName, srAnnotationsScaleDown)

			// sleep to give time for changes to propagate to the deployment
			time.Sleep(10 * time.Second)
			WaitForStableActiveDeployState(time.Second * 30)

			// check that all runtimes except the one are scaled up to minimum replicas of HPA
			expectScaledToTargetReplicas(1)

			// check HPA object
			expectHPAExist(true)
			expectHPAMinReplicas(1)
			expectHPAMaxReplicas(1)
			expectHPATargetUtilizationPercentage(80)
			expectHPAResourceName(corev1.ResourceCPU)
		})
	})
	// This test must be the last because it will remove hpa annotation from servingruntime/clusterservingruntime
	Context("When the model does not need autoscaler anymore", func() {
		It("Disable autoscaler", func() {
			deployTestPredictorAndCheckDefaultHPA()

			// set modified annotations
			By("Deleting this annotation: " + constants.AutoscalerClass)
			srAnnotationsNone := make(map[string]interface{})
			FVTClientInstance.SetServingRuntimeAnnotation(expectedRuntimeName, srAnnotationsNone)

			// sleep to give time for changes to propagate to the deployment
			time.Sleep(10 * time.Second)
			WaitForStableActiveDeployState(time.Second * 30)

			// check that all runtimes except the one are scaled up to servingRuntime default replicas
			expectScaledToTargetReplicas(1)

			// check if HPA deleted
			expectHPAExist(false)
		})
	})
})
