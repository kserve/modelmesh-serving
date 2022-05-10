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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

	By("Waiting for predictor" + predictorName + " to be 'Loaded'")
	// TODO: "Standby" (or) "FailedToLoad" states are currently encountered after the "Loading" state but they shouldn't be (see issue#994)
	resultingPredictor := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "FailedToLoad", "Loading", "Loaded"}, watcher)
	ExpectPredictorState(resultingPredictor, true, "Loaded", "", "UpToDate")
	return resultingPredictor
}

func CreateIsvcAndWaitAndExpectReady(isvcManifest *unstructured.Unstructured) *unstructured.Unstructured {
	isvcName := isvcManifest.GetName()
	By("Creating inference service " + isvcName)
	watcher := FVTClientInstance.StartWatchingIsvcs(metav1.ListOptions{FieldSelector: "metadata.name=" + isvcName}, DefaultTimeout)
	defer watcher.Stop()
	FVTClientInstance.CreateIsvcExpectSuccess(isvcManifest)
	By("Waiting for inference service" + isvcName + " to be 'Ready'")
	// ISVC does not have the status field set initially.
	resultingIsvc := WaitForIsvcReady(watcher)
	return resultingIsvc
}

func CreatePredictorAndWaitAndExpectFailed(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Creating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	createdPredictor := FVTClientInstance.CreatePredictorExpectSuccess(predictorManifest)
	ExpectPredictorState(createdPredictor, false, "Pending", "", "UpToDate")

	By("Waiting for predictor" + predictorName + " to be 'FailedToLoaded'")
	// "Standby" state is encountered after the "Loading" state but it shouldn't be
	resultingPredictor := WaitForLastStateInExpectedList("activeModelState", []string{"Pending", "Loading", "Standby", "Loading", "FailedToLoad"}, watcher)
	ExpectPredictorState(resultingPredictor, false, "FailedToLoad", "", "UpToDate")
	return resultingPredictor
}

func CreatePredictorAndWaitAndExpectInvalidSpec(predictorManifest *unstructured.Unstructured) *unstructured.Unstructured {
	predictorName := predictorManifest.GetName()

	By("Creating predictor " + predictorName)
	watcher := FVTClientInstance.StartWatchingPredictors(metav1.ListOptions{FieldSelector: "metadata.name=" + predictorName}, DefaultTimeout)
	defer watcher.Stop()
	createdPredictor := FVTClientInstance.CreatePredictorExpectSuccess(predictorManifest)
	ExpectPredictorState(createdPredictor, false, "Pending", "", "UpToDate")

	By("Waiting for predictor" + predictorName + " to have transitionStatus 'InvalidSpec'")
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

func WaitForIsvcReady(watcher watch.Interface) *unstructured.Unstructured {
	ch := watcher.ResultChan()
	isReady := false
	var obj *unstructured.Unstructured
	var isvcName string

	timeout := time.After(predictorTimeout)
	done := false
	for !done {
		select {
		// Exit the loop if InferenceService is not ready before given timeout.
		case <-timeout:
			done = true
		case event, ok := <-ch:
			if !ok {
				// the channel was closed (watcher timeout reached)
				done = true
				break
			}
			obj, ok = event.Object.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())
			isvcName = GetString(obj, "metadata", "name")
			conditions, exists := GetSlice(obj, "status", "conditions")
			if !exists {
				time.Sleep(time.Second)
				continue
			}
			for _, condition := range conditions {
				conditionMap := condition.(map[string]interface{})
				if conditionMap["type"] == "Ready" {
					if conditionMap["status"] == "True" {
						isReady = true
						done = true
						break
					}
				}
			}

		}
	}
	Expect(isReady).To(BeTrue(), "Timeout before InferenceService '%s' ready", isvcName)
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

func WaitForStableActiveDeployState() {
	watcher := FVTClientInstance.StartWatchingDeploys()
	defer watcher.Stop()
	WaitForDeployStatus(watcher, timeForStatusToStabilize)
}

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

			Log.Info("Watcher got event with object",
				"name", deployName,
				"status.replicas", replicas,
				"status.availableReplicas", availableReplicas,
				"status.updatedReplicas", updatedReplicas)

			if (updatedReplicas == replicas) && (availableReplicas == updatedReplicas) {
				deployStatusesReady[deployName] = true
				Log.Info(fmt.Sprintf("deployStatusesReady: %v", deployStatusesReady))
			} else {
				deployStatusesReady[deployName] = false
			}

		// this case executes if no events are received during the timeToStabilize duration
		case <-time.After(timeToStabilize):
			// check if all deployments are ready
			stable := true
			for _, status := range deployStatusesReady {
				if !status {
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
