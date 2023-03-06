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
/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	config2 "github.com/kserve/modelmesh-serving/pkg/config"

	"github.com/kserve/modelmesh-serving/controllers/config"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tommy351/goldga"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	namespace = "default"
)

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment

// need to hold the provider reference to allow config to be edited
var configProvider *config2.ConfigProvider

// pointer to the config for editing, to be used in tests
var reconcilerConfig *config2.Config

func SnapshotMatcher() *goldga.Matcher {
	matcher := goldga.Match()
	matcher.Serializer = &YAMLSerializer{}
	return matcher
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	//Set template dir to account for test working dirs
	SetTemplateDir()

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = api.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", //This disables the metrics server
	})
	Expect(err).ToNot(HaveOccurred())

	// Note: the client can also be created without a manager, but there
	// is a problem where the GVK data is not being populated
	// k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	k8sClient = k8sManager.GetClient()

	// Create prerequisite resources for the controller to use

	// Generally with kubebuilder, the controller's deployment does not need to
	// exist for testing in the bootstrapped environment (the manager runs locally).
	// However, in our controller, ServingRuntime reconciliation requires the
	// controller's deployment to exist so that it can set the OwnerRef on the
	// generated model mesh tc-config ConfigMap to reference it
	m, err := mf.ManifestFrom(mf.Path("../config/manager/manager.yaml"))
	Expect(err).ToNot(HaveOccurred())
	m.Client = mfc.NewClient(k8sClient)
	m, err = m.Transform(mf.InjectNamespace(namespace))
	Expect(err).ToNot(HaveOccurred())
	err = m.Apply()
	Expect(err).ToNot(HaveOccurred())

	// create the storage-config secret
	ctx := context.Background()
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "storage-config", Namespace: namespace}}
	err = k8sClient.Create(ctx, s, &client.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	// main usually sets this
	modelmesh.StorageSecretName = "storage-config"

	os.Setenv(config2.EnvEtcdSecretName, "secret")

	// Create the ConfigProvider object that will be used by the reconciler
	// Instead of creating the user configmap and reacting to watch events on the
	// configmap, tests can edit the Config object directly. This means that
	// config changes are seen by the controller immediately.
	configProvider = config2.NewConfigProviderForTest()
	resetReconcilerConfig()

	// create the reconciler and add it to the manager
	err = (&ServingRuntimeReconciler{
		Client:              k8sManager.GetClient(),
		Scheme:              k8sManager.GetScheme(),
		Log:                 ctrl.Log.WithName("controllers").WithName("ServingRuntime"),
		ConfigProvider:      configProvider,
		ControllerName:      "modelmesh-controller",
		ControllerNamespace: namespace,
	}).SetupWithManager(k8sManager, false, nil)
	Expect(err).ToNot(HaveOccurred())

	// TODO: create PredictorReconciler when Predictor tests are added

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	// short sleep to give time for the controller to react to deletion of
	// resources in AfterEach() to prevent error logs
	time.Sleep(1 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	resetReconcilerConfig()
})

// Cleanup resources to not contaminate between tests
var _ = AfterEach(func() {
	var err error
	inNamespace := client.InNamespace(namespace)
	// ServingRuntimes
	err = k8sClient.DeleteAllOf(context.TODO(), &kserveapi.ServingRuntime{}, inNamespace)
	Expect(err).ToNot(HaveOccurred())
	// Predictors
	err = k8sClient.DeleteAllOf(context.TODO(), &api.Predictor{}, inNamespace)
	Expect(err).ToNot(HaveOccurred())
})

var defaultTestConfigFileContents []byte
var payloadProcessingTestConfigFileContents []byte

func getDefaultConfig() (*config2.Config, error) {
	if defaultTestConfigFileContents == nil {
		var err error
		var testConfigFile = "./testdata/test-config-defaults.yaml"
		if defaultTestConfigFileContents, err = ioutil.ReadFile(testConfigFile); err != nil {
			return nil, err
		}
	}
	return config2.NewMergedConfigFromString(string(defaultTestConfigFileContents))
}

// load the payload processing config
func getPayloadProcessingConfig() (*config2.Config, error) {
	if payloadProcessingTestConfigFileContents == nil {
		var err error
		var testConfigFile = "./testdata/test-config-payload-processor.yaml"
		if payloadProcessingTestConfigFileContents, err = ioutil.ReadFile(testConfigFile); err != nil {
			return nil, err
		}
	}
	return config2.NewMergedConfigFromString(string(payloadProcessingTestConfigFileContents))
}

// set config to the payload processing config
func resetToPayloadConfig() {
	config, err := getPayloadProcessingConfig()
	Expect(err).ToNot(HaveOccurred())

	// re-assign the reference to the config
	reconcilerConfig = config

	// inject the reference into the provider used by the reconciler
	config2.SetConfigForTest(configProvider, config)
}

func resetReconcilerConfig() {
	config, err := getDefaultConfig()
	Expect(err).ToNot(HaveOccurred())

	// re-assign the reference to the config
	reconcilerConfig = config

	// inject the reference into the provider used by the reconciler
	config2.SetConfigForTest(configProvider, config)
}

type YAMLSerializer struct{}

func (y *YAMLSerializer) Serialize(w io.Writer, input interface{}) error {
	b, err := yaml.Marshal(input)
	if err != nil {
		return err
	}
	_, err = w.Write(b)

	return err
}

// Sets the working dir to the repository root
func SetTemplateDir() {
	var repoRoot string
	if _, filename, _, ok := runtime.Caller(1); !ok {
		panic("Unable to get the caller")
	} else {
		for i := 0; i < 10; i++ {
			git := path.Join(path.Dir(filename), ".git")
			if _, err := os.Stat(git); !os.IsNotExist(err) {
				repoRoot = path.Dir(filename)
				break
			}

			filename = path.Dir(filename)
			if filename == "/" {
				break
			}
		}
	}

	config.PathPrefix = repoRoot
}
