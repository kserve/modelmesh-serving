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
package scaleToZero

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scaling of runtime deployments to zero", Ordered, func() {

	// constants
	testPredictorObject := NewPredictorForFVT("mlserver-sklearn-predictor.yaml")
	// runtime expected to serve the test predictor
	expectedRuntimeName := "mlserver-0.x"

	// helper expectation functions
	// these use the "constants" so are created within the Describe's scope

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
	expectScaledToZero := func() {
		replicas := checkDeploymentState()
		Expect(replicas).To(BeEquivalentTo(int32(0)))
	}
	expectScaledUp := func() {
		replicas := checkDeploymentState()
		Expect(replicas).ToNot(BeEquivalentTo(int32(0)))
	}

	Context("when there are no predictors", func() {
		BeforeEach(func() {
			FVTClientInstance.DeleteAllPredictors()
		})

		It("should scale all runtimes down", func() {
			By("Waiting for the deployments to stabilize")
			WaitForStableActiveDeployState(TimeForStatusToStabilize)

			// check that all runtimes are scaled to zero
			expectScaledToZero()
		})

		It("creating a predictor should scale up the runtime and the predictor should eventually load", func() {
			By("Waiting for the predictor to be 'Loading'")
			watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + testPredictorObject.GetName()}, DefaultTimeout)
			defer watcher.Stop()

			By("Creating a test predictor for one Runtime")
			FVTClientInstance.ApplyPredictorExpectSuccess(testPredictorObject)

			By("Waiting for the deployments to stabilize")
			WaitForStableActiveDeployState(TimeForStatusToStabilize)

			// check that all runtimes except the one are scaled to zero
			expectScaledUp()

			By("Waiting for the Predictor to cleanly transition to 'Loaded' state")
			obj := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Loaded"}, watcher)
			ExpectPredictorState(obj, true, "Loaded", "", "UpToDate")
		})

	})

	Context("when there is a predictor for one runtime", func() {
		BeforeEach(func() {
			By("Creating a test predictor for one Runtime")
			// ensure single predictor exists
			FVTClientInstance.ApplyPredictorExpectSuccess(testPredictorObject)

			By("Waiting for the deployments to stabilize")
			WaitForStableActiveDeployState(TimeForStatusToStabilize)

			// ensure the runtime is ready and scaled up and others are scaled down
			expectScaledUp()
		})

		It("should not scale down after the grace period", func() {
			By("Waiting for the grace period to elapse")
			time.Sleep(6 * time.Second)

			By("Checking that the deployment is scaled up")
			expectScaledUp()
		})

		It("should scale down after deleting the predictor but only after the grace period", func() {
			By("Deleting the predictor")
			FVTClientInstance.DeletePredictor(testPredictorObject.GetName())

			By("Waiting for less than the grace period")
			time.Sleep(1 * time.Second)

			By("Checking that the deployment is scaled up")
			expectScaledUp()

			// wait for longer than the grace period
			By("Waiting for the grace period to expire")
			time.Sleep(5 * time.Second)

			By("Check that the deployment is scaled to zero")
			expectScaledToZero()
		})

		It("should not scale down if predictor is deleted and recreated within the grace period", func() {
			By("Deleting the predictor")
			FVTClientInstance.DeletePredictor(testPredictorObject.GetName())

			By("Waiting for less than the grace period")
			time.Sleep(2 * time.Second)

			By("Checking that the deployment is scaled up")
			expectScaledUp()

			By("Recreating the predictor")
			FVTClientInstance.ApplyPredictorExpectSuccess(testPredictorObject)

			By("Check that the deployment stays scaled up consistently")
			pollTimeoutSeconds := 6
			pollIntervalSeconds := 1
			Consistently(func() int32 {
				return checkDeploymentState()
			}, pollTimeoutSeconds, pollIntervalSeconds).ShouldNot(BeEquivalentTo(int32(0)))
		})
	})

})
