// Copyright 2023 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package storage

import (
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/kserve/modelmesh-serving/fvt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

func createFVTClient() {
	Log = zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
	Log.Info("Initializing test suite")

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = DefaultTestNamespace
	}
	serviceName := os.Getenv("SERVICENAME")
	if serviceName == "" {
		serviceName = DefaultTestServiceName
	}
	controllerNamespace := os.Getenv("CONTROLLERNAMESPACE")
	if controllerNamespace == "" {
		controllerNamespace = DefaultControllerNamespace
	}
	NameSpaceScopeMode = os.Getenv("NAMESPACESCOPEMODE") == "true"
	Log.Info("Using environment variables", "NAMESPACE", namespace, "SERVICENAME", serviceName,
		"CONTROLLERNAMESPACE", controllerNamespace, "NAMESPACESCOPEMODE", NameSpaceScopeMode)

	var err error
	FVTClientInstance, err = GetFVTClient(Log, namespace, serviceName, controllerNamespace)
	Expect(err).ToNot(HaveOccurred())
	Expect(FVTClientInstance).ToNot(BeNil())
	Log.Info("FVTClientInstance created", "client", FVTClientInstance)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// runs *only* on process #1
	createFVTClient()

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

	// create TLS secrets before start of tests
	FVTClientInstance.CreateTLSSecrets()

	// ensure a stable deploy state
	Log.Info("Waiting for stable deploy state")
	WaitForStableActiveDeployState()

	return nil
}, func(_ []byte) {
	// runs on *all* processes
	// create the fvtClient Instance on every other process except the first, since it got created in the above function.
	if FVTClientInstance == nil {
		createFVTClient()
	}
	// don't connect to model serving service once for all tests, flaky runtime pods, broken connections
	//FVTClientInstance.ConnectToModelServingService(Insecure)
	Log.Info("Setup completed")
})

var _ = SynchronizedAfterSuite(func() {
	// runs on *all* processes
	// ensure we clean up any port-forward
	FVTClientInstance.DisconnectFromModelServing()
}, func() {
	// runs *only* on process #1
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
		FVTClientInstance.PrintPredictors()
		FVTClientInstance.PrintIsvcs()
		FVTClientInstance.PrintPods()
		FVTClientInstance.PrintDescribeNodes()
		FVTClientInstance.PrintEvents()
		FVTClientInstance.TailPodLogs(startTime)
	}
})
