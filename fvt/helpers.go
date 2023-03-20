// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"
)

func NewPredictorForFVT(filename string) *unstructured.Unstructured {
	p := DecodeResourceFromFile(TestDataPath(SamplesPath + filename))
	uniqueName := MakeUniquePredictorName(p.GetName())
	p.SetName(uniqueName)

	return p
}

func NewIsvcForFVT(filename string) *unstructured.Unstructured {
	p := DecodeResourceFromFile(TestDataPath(IsvcSamplesPath + filename))
	uniqueName := MakeUniquePredictorName(p.GetName())
	p.SetName(uniqueName)

	return p
}

// to enable tests to run in parallel even when loading from the same Predictor
// sample
func MakeUniquePredictorName(base string) string {
	// uses the same function as Kubernetes
	// https://github.com/kubernetes/apiserver/blob/d37241544f69aa89a8030eadd9ca51b4eec867d2/pkg/storage/names/generate.go#L53
	return fmt.Sprintf("%s-%s", base, utilrand.String(5))
}

func CreatePredictorAndWaitAndExpectLoaded(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Creating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	createdPredictor := FVTClientInstance.CreatePredictorExpectSuccess(predictorManifest)
	ExpectPredictorState(createdPredictor, false, "Pending", "", "UpToDate")

	By("Waiting for predictor " + predictorName + " to be 'Loaded'")
	// TODO: "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be (see issue#994)
	resultingPredictor := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
	ExpectPredictorState(resultingPredictor, true, "Loaded", "", "UpToDate")
	return resultingPredictor
}

func CreateIsvcAndWaitAndExpectReady(isvcManifest *unstructured.Unstructured, timeout time.Duration) *unstructured.Unstructured {
	isvcName := isvcManifest.GetName()
	By("Creating inference service " + isvcName)
	watcher := FVTClientInstance.StartWatchingIsvcs(metav1.ListOptions{FieldSelector: "metadata.name=" + isvcName}, int64(timeout.Seconds()))
	defer watcher.Stop()
	FVTClientInstance.CreateIsvcExpectSuccess(isvcManifest)
	By("Waiting for inference service " + isvcName + " to be 'Ready' and model is 'Loaded'")
	// ISVC does not have the status field set initially.
	resultingIsvc := WaitForIsvcState(watcher, []api.ModelState{api.Standby, api.Loaded}, isvcName, timeout)
	return resultingIsvc
}

func CreatePredictorAndWaitAndExpectFailed(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Creating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	createdPredictor := FVTClientInstance.CreatePredictorExpectSuccess(predictorManifest)
	ExpectPredictorState(createdPredictor, false, "Pending", "", "UpToDate")

	By("Waiting for predictor " + predictorName + " to have 'FailedToLoad'")
	// "Standby" state is encountered after the "Loading" state, but it shouldn't be
	resultingPredictor := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)
	ExpectPredictorState(resultingPredictor, false, "FailedToLoad", "", "UpToDate")
	return resultingPredictor
}

func CreateIsvcAndWaitAndExpectFailed(isvcManifest *unstructured.Unstructured) *unstructured.Unstructured {
	isvcName := isvcManifest.GetName()
	By("Creating inference service " + isvcName)
	watcher := FVTClientInstance.StartWatchingIsvcs(metav1.ListOptions{FieldSelector: "metadata.name=" + isvcName}, DefaultTimeout)
	defer watcher.Stop()
	FVTClientInstance.CreateIsvcExpectSuccess(isvcManifest)
	By("Waiting for inference service " + isvcName + " to fail")
	// ISVC does not have the status field set initially.
	resultingIsvc := WaitForIsvcState(watcher, []api.ModelState{api.FailedToLoad}, isvcName, PredictorTimeout)
	return resultingIsvc
}

func CreatePredictorAndWaitAndExpectInvalidSpec(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Creating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	createdPredictor := FVTClientInstance.CreatePredictorExpectSuccess(predictorManifest)
	ExpectPredictorState(createdPredictor, false, "Pending", "", "UpToDate")

	By("Waiting for predictor " + predictorName + " to have transitionStatus 'InvalidSpec'")
	return WaitForLastStateInExpectedList("transitionStatus", []string{"UpToDate", "InvalidSpec"}, watcher)
}

func UpdatePredictorAndWaitAndExpectLoaded(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Updating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	FVTClientInstance.ApplyPredictorExpectSuccess(predictorManifest)

	By("Waiting for the predictor " + predictorName + "'s target model state to move from Loaded (empty) to Loading")
	loadingPredictor := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading"}, watcher)
	ExpectPredictorState(loadingPredictor, true, "Loaded", "Loading", "InProgress")

	By("Waiting for predictor " + predictorName + "'s target model state to be 'Loaded'")
	resultingPredictor := WaitForLastStateInExpectedList("targetModelState", []string{"Loading", "Loaded", ""}, watcher)
	ExpectPredictorState(resultingPredictor, true, "Loaded", "", "UpToDate")
	return resultingPredictor
}

func UpdatePredictorAndWaitAndExpectFailed(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Updating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	FVTClientInstance.ApplyPredictorExpectSuccess(predictorManifest)

	By("Waiting for the predictor " + predictorName + "'s target model state to move from Loaded (empty) to Loading")
	loadingPredictor := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading"}, watcher)
	ExpectPredictorState(loadingPredictor, true, "Loaded", "Loading", "InProgress")

	By("Waiting for predictor " + predictorName + "'s target model state to be 'FailedToLoad'")
	resultingPredictor := WaitForLastStateInExpectedList("targetModelState", []string{"", "Loading", "FailedToLoad"}, watcher)
	ExpectPredictorState(resultingPredictor, true, "Loaded", "FailedToLoad", "BlockedByFailedLoad")
	return resultingPredictor
}

func ExpectPredictorState(obj *unstructured.Unstructured, available bool, activeModelState, targetModelState, transitionStatus string) {
	actualActiveModelState := GetString(obj, "status", "activeModelState")
	Expect(actualActiveModelState).To(Equal(activeModelState))

	actualAvailable := GetBool(obj, "status", "available")
	Expect(actualAvailable).To(Equal(available))

	actualTargetModel := GetString(obj, "status", "targetModelState")
	Expect(actualTargetModel).To(Equal(targetModelState))

	actualTransitionStatus := GetString(obj, "status", "transitionStatus")
	Expect(actualTransitionStatus).To(Equal(transitionStatus))

	if transitionStatus != string(api.BlockedByFailedLoad) &&
		transitionStatus != string(api.InvalidSpec) &&
		activeModelState != string(api.FailedToLoad) &&
		targetModelState != string(api.FailedToLoad) {
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
		Expect(actualFailureInfo["message"]).To(ContainSubstring(message))
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

func ExpectIsvcState(obj *unstructured.Unstructured, activeModelState, targetModelState, transitionStatus string) {
	actualActiveModelState := GetString(obj, "status", "modelStatus", "states", "activeModelState")
	Expect(actualActiveModelState).To(Equal(activeModelState))

	actualTargetModel := GetString(obj, "status", "modelStatus", "states", "targetModelState")
	Expect(actualTargetModel).To(Equal(targetModelState))

	actualTransitionStatus := GetString(obj, "status", "modelStatus", "transitionStatus")
	Expect(actualTransitionStatus).To(Equal(transitionStatus))

	if transitionStatus != "BlockedByFailedLoad" &&
		transitionStatus != "InvalidSpec" &&
		activeModelState != string(api.FailedToLoad) &&
		targetModelState != string(api.FailedToLoad) {
		actualFailureInfo := GetMap(obj, "status", "modelStatus", "lastFailureInfo")
		Expect(actualFailureInfo).To(BeNil())
	}
}

func ExpectIsvcFailureInfo(obj *unstructured.Unstructured, reason string, hasLocation bool, hasTime bool, message string) {
	actualFailureInfo := GetMap(obj, "status", "modelStatus", "lastFailureInfo")
	Expect(actualFailureInfo).ToNot(BeNil())

	Expect(actualFailureInfo["reason"]).To(Equal(reason))
	if hasLocation {
		Expect(actualFailureInfo["location"]).ToNot(BeEmpty())
	} else {
		Expect(actualFailureInfo["location"]).To(BeNil())
	}
	if message != "" {
		Expect(actualFailureInfo["message"]).To(ContainSubstring(message))
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

func WaitForIsvcState(watcher watch.Interface, anyOfDesiredStates []api.ModelState, name string, isvcTimeout time.Duration) *unstructured.Unstructured {
	ch := watcher.ResultChan()
	reachedDesiredState := false
	var obj *unstructured.Unstructured
	var isvcName = name

	timeout := time.After(isvcTimeout)
	done := false
	for !done {
		select {
		// Exit the loop if InferenceService is not ready before given timeout.
		case <-timeout:
			done = true
			FVTClientInstance.PrintDescribeIsvc(isvcName)
		case event, ok := <-ch:
			if !ok {
				// the channel was closed (watcher timeout reached)
				done = true
				FVTClientInstance.PrintDescribeIsvc(isvcName)
				break
			}
			obj, ok = event.Object.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())
			isvcName = GetString(obj, "metadata", "name")
			// ISVC does not have the status field set initially
			//  modelStatus will not exist until status.conditions exist
			_, exists := GetSlice(obj, "status", "conditions")
			if !exists {
				time.Sleep(time.Second)
				continue
			}
			// Note: first status.conditions[{"Type": "Ready", "Status": "True"}] can
			//  occur before status.conditions[{"Type": "Ready", "Status": "False"}]
			//  so we check for "activeModelState" instead
			activeModelState := GetString(obj, "status", "modelStatus", "states", "activeModelState")
			for _, desiredState := range anyOfDesiredStates {
				if activeModelState == string(desiredState) {
					reachedDesiredState = true
					done = true
					break
				}
			}
		}
	}
	Expect(reachedDesiredState).To(BeTrue(), "Timeout before InferenceService '%s' reached any of the activeModelStates %s", isvcName, anyOfDesiredStates)
	return obj
}

// Waiting for predictor state to reach the last one in the expected list
// Predictor state is allowed to directly reach the last state in the expected list i.e; Loaded. Also, Predictor state can be
// one of the earlier states (i.e; Pending or Loading), but state change should happen in the following order:
// [Pending => Loaded]  (or) [Pending => Loading => Loaded]  (or) [Loading => Loaded] (or) [Pending => Loading => FailedToLoad]
func WaitForLastStateInExpectedList(statusAttribute string, expectedStates []string, watcher watch.Interface) *unstructured.Unstructured {
	ch := watcher.ResultChan()
	targetStateReached := false
	var obj *unstructured.Unstructured
	targetState := expectedStates[len(expectedStates)-1]
	lastState := "UNSEEN"
	var predictorName string

	timeout := time.After(PredictorTimeout)
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
			Log.Info("Watcher got event with object", logPredictorStatus(obj)...)

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

func WaitForStableActiveDeployState(timeToStabilize time.Duration) {
	watcher := FVTClientInstance.StartWatchingDeploys()
	defer watcher.Stop()
	WaitForRuntimeDeploymentsToBeStable(timeToStabilize, watcher)
}

func WaitForRuntimeDeploymentsToBeStable(timeToStabilize time.Duration, watcher watch.Interface) {
	ch := watcher.ResultChan()
	var obj *unstructured.Unstructured
	var replicas, updatedReplicas, availableReplicas int64
	var deployName string
	deploymentReady := make(map[string]bool)

	// Get a list of ServingRuntime deployments
	runtimeDeploys := FVTClientInstance.ListDeploys()
	for _, deploy := range runtimeDeploys.Items {
		// initialize all deployment statuses as not ready
		deploymentReady[deploy.Name] = false
	}

	timeout := timeToStabilize
	allReady := false
	done := false
	for !done {
		select {
		// The select statement is only used with channels to let a goroutine wait on multiple communication operations.
		// The select blocks until one of its cases can run, then it executes that case. It chooses one at random if multiple are ready.
		case <-time.After(timeout):
			// if no watcher events came in for the given length of timeForStatusToStabilize
			// then we assume the deployment status has stabilized and exit the loop
			Log.Info(fmt.Sprintf("Timed out after %v without events", timeout))
			done = true
			break
		case event, ok := <-ch:
			if !ok {
				// the channel was closed (watcher timeout reached, see DefaultTimeout)
				Log.Info(fmt.Sprintf("Watcher timed out after %v seconds", DefaultTimeout))
				done = true
				break
			}
			obj, ok = event.Object.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())

			replicas = GetInt64(obj, "status", "replicas")
			availableReplicas = GetInt64(obj, "status", "availableReplicas")
			updatedReplicas = GetInt64(obj, "status", "updatedReplicas")
			deployName = GetString(obj, "metadata", "name")

			Log.Info("Watcher got event with object",
				"name", deployName,
				"replicas", replicas,
				"available", availableReplicas,
				"updated", updatedReplicas)

			if (updatedReplicas == replicas) && (availableReplicas == updatedReplicas) {
				deploymentReady[deployName] = true
				Log.Info(fmt.Sprintf("deployStatusesReady: %v", deploymentReady))
				// check if all deployments are ready
				allReady = true
				for _, thisReady := range deploymentReady {
					allReady = allReady && thisReady
				}
				// do not exit the loop yet (done=true), deployment my become unstable again,
				// wait for timeForStatusToStabilize (see above)
				if allReady {
					// once we are ready, shorten time to wait for next event
					// if we are truly ready no more event will come in
					// if we are not yet ready, new events will come in quickly
					timeout = TimeForStatusToStabilize
					Log.Info(fmt.Sprintf("All deployments are ready: %v", deploymentReady))
				}
			} else {
				deploymentReady[deployName] = false
				// restore the full time to wait in between watcher events
				timeout = timeToStabilize
			}
		}
	}

	Expect(allReady).To(BeTrue(), fmt.Sprintf("Timed out before deployments were ready: %v", deploymentReady))
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
