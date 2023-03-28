// Copyright 2023 IBM Corporation
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
package storage

import (
	"testing"
	"time"

	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// runs *only* on process #1
	InitializeFVTClient()

	// confirm 4 cluster serving runtimes or serving runtimes exist
	var err error
	var list *unstructured.UnstructuredList
	if NameSpaceScopeMode {
		list, err = FVTClientInstance.ListServingRuntimes(metav1.ListOptions{})
	} else {
		list, err = FVTClientInstance.ListClusterServingRuntimes(metav1.ListOptions{})
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(list.Items).To(HaveLen(4))

	FVTClientInstance.SetDefaultUserConfigMap()

	// replace default storage-config secret for local Minio with PVC config
	FVTClientInstance.DeleteStorageConfigSecret()
	FVTClientInstance.CreateStorageConfigSecret(StorageConfigDataPVC)

	// ensure that there are no predictors to start
	FVTClientInstance.DeleteAllPredictors()
	FVTClientInstance.DeleteAllIsvcs()

	// ensure a stable deploy state, on each process since we updated the storage config
	Log.Info("Waiting for stable deploy state")
	WaitForStableActiveDeployState(time.Second * 60)

	return nil
}, func(_ []byte) {
	// runs on *all* processes
	// create the fvtClient Instance on every other process except the first, since it got created in the above function.
	if FVTClientInstance == nil {
		InitializeFVTClient()
	}
	// connect to model serving service once for all each process
	err := FVTClientInstance.ConnectToModelServing(Insecure)
	Expect(err).ToNot(HaveOccurred())

	Log.Info("Setup completed")
})

var _ = SynchronizedAfterSuite(func() {
	// runs on *all* processes
	// ensure we clean up any port-forward for each process
	FVTClientInstance.DisconnectFromModelServing()
}, func() {
	// runs *only* on process #1
	// restore config defaults
	FVTClientInstance.DeleteTLSSecrets()
	FVTClientInstance.SetDefaultUserConfigMap()
	// reset default storage-config secret
	FVTClientInstance.DeleteStorageConfigSecret()
	FVTClientInstance.CreateStorageConfigSecret(StorageConfigDataMinio)
	// restart pods to reset Bootstrap failure checks
	FVTClientInstance.RestartDeploys()
})

// register handlers for a failed test case to print info to the console
var startTime string
var _ = JustBeforeEach(func() {
	startTime = time.Now().Format("2006-01-02T15:04:05Z")
})
var _ = JustAfterEach(func() {
	if CurrentSpecReport().Failed() {
		FVTClientInstance.PrintIsvcs()
		FVTClientInstance.PrintPods()
		FVTClientInstance.PrintDescribeNodes()
		FVTClientInstance.PrintEvents()
		FVTClientInstance.TailPodLogs(startTime)
	}
})
