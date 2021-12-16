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
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// TestFVT is the main Ginko test driver. This adds a junit report to a target dir.
func TestFVT(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter(fmt.Sprintf("../target/test-reports/junit_fvt_%d.xml", config.GinkgoConfig.ParallelNode))

	config.DefaultReporterConfig.SlowSpecThreshold = time.Minute.Seconds() * 3

	RunSpecsWithDefaultAndCustomReporters(t,
		"Fvt Suite",
		[]Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	log = zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
	log.Info("Initializing test suite")

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
	log.Info("Using environment variables", "NAMESPACE", namespace, "SERVICENAME", serviceName, "CONTROLLERNAMESPACE", controllerNamespace)

	var err error
	fvtClient, err = GetFVTClient(log, namespace, serviceName, controllerNamespace)
	Expect(err).ToNot(HaveOccurred())
	log.Info("FVTClient created", "client", fvtClient)

	// confirm 3 serving runtimes exist
	list, err := fvtClient.ListServingRuntimes(metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(list.Items).To(HaveLen(2))

	// cleanup any predictors if they exist
	fvtClient.DeleteAllPredictors()

	// Create TLS secrets if they don't yet exist
	fvtClient.CreateTLSSecrets()

	log.Info("Setup completed")
}, 60)

var _ = AfterSuite(func() {
	// Clean up any custom TLS secrets generated for this testing
	err := fvtClient.DeleteTLSSecrets()
	if err != nil {
		log.Info("Error deleting test-specific TLS secrets", "error", err)
	}

	// ensure we cleanup any port-forward
	fvtClient.DisconnectFromModelServing()
})

// register hanlders for a failed test case to print info to the console
var startTime string
var _ = JustBeforeEach(func() {
	startTime = time.Now().Format("2006-01-02T15:04:05Z")
})
var _ = JustAfterEach(func() {
	if CurrentGinkgoTestDescription().Failed {
		fvtClient.PrintPredictors()
		fvtClient.TailPodLogs(startTime)
	}
})
