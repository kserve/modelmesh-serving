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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	"github.com/go-logr/logr"
	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	inference "github.com/kserve/modelmesh-serving/fvt/generated"
	tfsapi "github.com/kserve/modelmesh-serving/fvt/generated/tensorflow_serving/apis"
)

const predictorTimeout = time.Second * 120
const timeForStatusToStabilize = time.Second * 5

type ModelServingConnectionType int

const (
	Insecure ModelServingConnectionType = iota
	TLS
	MutualTLS
)

// for use in resource Patch calls
var applyPatchOptions = metav1.PatchOptions{
	FieldManager: "fvtclient",
	// force the change (fvtclient should be the only manager)
	Force: func() *bool { t := true; return &t }(),
}

type FVTClient struct {
	dynamic.Interface
	namespace           string
	serviceName         string
	controllerNamespace string
	grpcPort            int
	grpcPortForward     *ModelMeshPortForward
	grpcConn            *grpc.ClientConn
	restPort            int
	restPortForward     *ModelMeshPortForward
	restConn            *http.Client
	log                 logr.Logger
	certGenerator   		CertGenerator
}

type ModelMeshPortForward struct {
	cmd     *exec.Cmd
	cmdArgs []string
	done    chan struct{}
	log     logr.Logger
}

func (pf *ModelMeshPortForward) EnsureStarted() error {
	// quick return if command is still running
	if pf.cmd != nil && pf.cmd.Process != nil {
		pf.log.Info("Found existing port-forward process")
		return nil
	}
	// port forward localhost to the cluster's model-serving service
	pf.cmd = exec.Command("kubectl", pf.cmdArgs...)
	pf.log.Info("Running port-forward in the background", "Command", strings.Join(pf.cmd.Args, " "))

	pf.done = make(chan struct{})
	go func() {
		var commandErr error
		commandOutput, commandErr := pf.cmd.CombinedOutput()
		pf.log.Info("Port-forward command exited", "Error", commandErr, "Command Output", string(commandOutput))
		pf.cmd = nil
		// close the channel to signal that the command exited
		close(pf.done)
	}()

	// check that the port forward process is still running after 2s
	select {
	case <-pf.done:
		return fmt.Errorf("Expected the port-forward process to still be running but it is not.")
	case <-time.After(time.Second * 2):
		break
	}

	return nil
}

func (pf *ModelMeshPortForward) EnsureStopped() {
	// quick return if command is not running
	if pf.cmd == nil {
		return
	}
	pf.log.Info("Killing port-forward process")
	if err := pf.cmd.Process.Kill(); err != nil {
		pf.log.Error(err, "Failed to send kill signal to the port-forward process, but will attempt to continue")
		return
	}
	// wait until the process exits
	<-pf.done
}

func NewModelMeshPortForward(namespace string, podName string, localPort int, targetPort int, log logr.Logger) *ModelMeshPortForward {
	portMap := fmt.Sprintf("%d:%d", localPort, targetPort)
	cmdArgs := []string{"port-forward", "--namespace", namespace, "--address", "0.0.0.0",
		"pod/" + podName, portMap}

	return &ModelMeshPortForward{nil, cmdArgs, nil, log}
}

func GetFVTClient(log logr.Logger, namespace, serviceName, controllerNamespace string) (*FVTClient, error) {
	var err error
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	err = api.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	// set ports based on worker index to support parallel port-forwards
	grpcPort := 50000 + ginkgo.GinkgoParallelProcess()
	restPort := 8000 + ginkgo.GinkgoParallelProcess()

	return &FVTClient{
		Interface:       		 client,
		namespace:           namespace,
		serviceName:         serviceName,
		controllerNamespace: controllerNamespace,
		grpcPort:            grpcPort,
		grpcPortForward:     nil,
		grpcConn:            nil,
		restPort:            restPort,
		restPortForward:     nil,
		restConn:            nil,
		log:                 log,
		certGenerator:       CertGenerator{Namespace: namespace},
	}, nil
}

var (
	gvrRuntime = schema.GroupVersionResource{
		Group:    api.GroupVersion.Group,
		Version:  api.GroupVersion.Version,
		Resource: "servingruntimes", // this must be the plural form
	}
	gvrPredictor = schema.GroupVersionResource{
		Group:    api.GroupVersion.Group,
		Version:  api.GroupVersion.Version,
		Resource: "predictors", // this must be the plural form
	}
	gvrConfigMap = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps", // this must be the plural form
	}
	gvrSecret = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets", // this must be the plural form
	}
	gvrDeployment = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments", // this must be the plural form
	}
	gvrIsvc = schema.GroupVersionResource{
		Group:    v1beta1.SchemeGroupVersion.Group,
		Version:  v1beta1.SchemeGroupVersion.Version,
		Resource: "inferenceservices", // this must be the plural form
	}
	gvrEndpoints = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints", // this must be the plural form
	}
)

func (fvt *FVTClient) CreatePredictorExpectSuccess(resource *unstructured.Unstructured) *unstructured.Unstructured {
	obj, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(obj).ToNot(BeNil())
	Expect(obj.GetKind()).To(Equal(PredictorKind))
	return obj
}

func (fvt *FVTClient) CreateIsvcExpectSuccess(resource *unstructured.Unstructured) *unstructured.Unstructured {
	obj, err := fvt.Resource(gvrIsvc).Namespace(fvt.namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(obj).ToNot(BeNil())
	Expect(obj.GetKind()).To(Equal(IsvcKind))
	return obj
}

func (fvt *FVTClient) UpdatePredictorExpectSuccess(resource *unstructured.Unstructured) *unstructured.Unstructured {
	obj, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Update(context.TODO(), resource, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(obj).ToNot(BeNil())
	return obj
}

func (fvt *FVTClient) ApplyPredictorExpectSuccess(predictor *unstructured.Unstructured) *unstructured.Unstructured {
	// use server-side-apply with Patch
	patch, err := yaml.Marshal(predictor)
	Expect(err).ToNot(HaveOccurred())

	obj, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Patch(context.TODO(), predictor.GetName(), types.ApplyPatchType, patch, applyPatchOptions)
	Expect(err).ToNot(HaveOccurred())
	Expect(obj).ToNot(BeNil())
	Expect(obj.GetKind()).To(Equal(PredictorKind))
	return obj
}

func (fvt *FVTClient) GetServingRuntime(name string) *unstructured.Unstructured {
	obj, err := fvt.Resource(gvrRuntime).Namespace(fvt.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return obj
}

func (fvt *FVTClient) ListServingRuntimes(options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return fvt.Resource(gvrRuntime).Namespace(fvt.namespace).List(context.TODO(), options)
}

func (fvt *FVTClient) GetPredictor(name string) *unstructured.Unstructured {
	obj, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return obj
}

func (fvt *FVTClient) ListPredictors(options metav1.ListOptions) *unstructured.UnstructuredList {
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.TimeoutSeconds != nil && *options.TimeoutSeconds == int64(0) {
		options.TimeoutSeconds = &DefaultTimeout
	}
	list, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).List(context.TODO(), options)
	Expect(err).ToNot(HaveOccurred())
	return list
}

func (fvt *FVTClient) DeletePredictor(resourceName string) {
	fvt.log.Info("Deleting Predictor " + resourceName)
	err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
}

func (fvt *FVTClient) DeleteIsvc(resourceName string) {
	fvt.log.Info("Deleting inference services " + resourceName)
	err := fvt.Resource(gvrIsvc).Namespace(fvt.namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
}

func (fvt *FVTClient) DeleteAllPredictors() {
	fvt.log.Info("Delete all predictors ...")
	err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	time.Sleep(2 * time.Second)
}

func (fvt *FVTClient) DeleteAllIsvcs() {
	fvt.log.Info("Delete all inference services ...")
	err := fvt.Resource(gvrIsvc).Namespace(fvt.namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	time.Sleep(2 * time.Second)
}

func (fvt *FVTClient) StartWatchingPredictors(options metav1.ListOptions, timeoutSeconds int64) watch.Interface {
	options.TimeoutSeconds = &timeoutSeconds
	watcher, err := fvt.Resource(gvrPredictor).Namespace(fvt.namespace).Watch(context.TODO(), options)
	if err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	return watcher
}

func (fvt *FVTClient) StartWatchingIsvcs(options metav1.ListOptions, timeoutSeconds int64) watch.Interface {
	options.TimeoutSeconds = &timeoutSeconds
	watcher, err := fvt.Resource(gvrIsvc).Namespace(fvt.namespace).Watch(context.TODO(), options)
	if err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	return watcher
}

func (fvt *FVTClient) WatchPredictorsAsync(c chan *unstructured.Unstructured, options metav1.ListOptions, timeoutSeconds int64) {

}

func (fvt *FVTClient) GetRandomReadyRuntimePodNameFromEndpoints() string {
	obj, err := fvt.Resource(gvrEndpoints).Namespace(fvt.namespace).Get(context.TODO(), fvt.serviceName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	var endpoints corev1.Endpoints
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &endpoints)
	Expect(err).ToNot(HaveOccurred())

	addresses := endpoints.Subsets[0].Addresses
	randomAddress := addresses[rand.Intn(len(addresses))]

	return randomAddress.TargetRef.Name
}

func (fvt *FVTClient) PrintPredictors() {
	err := fvt.RunKubectl("get", "predictors")
	if err != nil {
		fvt.log.Error(err, "Error running get predictors command")
	}
}

func (fvt *FVTClient) PrintIsvcs() {
	err := fvt.RunKubectl("get", "inferenceservices")
	if err != nil {
		fvt.log.Error(err, "Error running get inferenceservices command")
	}
}

func (fvt *FVTClient) PrintPods() {
	err := fvt.RunKubectl("get", "pods")
	if err != nil {
		fvt.log.Error(err, "Error running get pods command")
	}
}

func (fvt *FVTClient) PrintDescribeNodes() {
	err := fvt.RunKubectl("describe", "nodes")
	if err != nil {
		fvt.log.Error(err, "Error running describe nodes command")
	}
}

func (fvt *FVTClient) PrintEvents() {
	err := fvt.RunKubectl("get", "events")
	if err != nil {
		fvt.log.Error(err, "Error running get events command")
	}
}

func (fvt *FVTClient) TailPodLogs(sinceTime string) {
	var err error
	// grab logs from the controller
	err = fvt.RunKubectl("logs", "-l", "control-plane=modelmesh-controller", "--all-containers", "--tail=100", "--prefix", "--since-time", sinceTime, "--timestamps")
	if err != nil {
		fvt.log.Error(err, "Error running kubectl logs for the controller")
	}

	// grab logs from the runtime pods
	err = fvt.RunKubectl("logs", "-l", "modelmesh-service=modelmesh-serving", "--all-containers", "--tail=100", "--prefix", "--since-time", sinceTime, "--timestamps")
	if err != nil {
		fvt.log.Error(err, "Error running kubectl logs for runtime pods")
	}
}

func (fvt *FVTClient) RunKubectl(args ...string) error {
	args = append(args, "-n", fvt.namespace)
	getPredictorCommand := exec.Command("kubectl", args...)
	getPredictorCommand.Stdout = ginkgo.GinkgoWriter
	getPredictorCommand.Stderr = ginkgo.GinkgoWriter
	fvt.log.Info("Running command", "args", strings.Join(getPredictorCommand.Args, " "))
	fmt.Fprintf(ginkgo.GinkgoWriter, "=====================================================================================================================================\n")
	err := getPredictorCommand.Run()
	fmt.Fprintf(ginkgo.GinkgoWriter, "=====================================================================================================================================\n")
	return err
}

func (fvt *FVTClient) RunKfsInference(req *inference.ModelInferRequest) (*inference.ModelInferResponse, error) {
	if fvt.grpcConn == nil {
		return nil, errors.New("you must connect to model mesh before running an inference")
	}

	grpcClient := inference.NewGRPCInferenceServiceClient(fvt.grpcConn)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return grpcClient.ModelInfer(ctx, req)
}

func (fvt *FVTClient) RunKfsRestInference(modelName string, body []byte, tls bool) (string, error) {
	if fvt.restConn == nil {
		return "", errors.New("you must connect to model mesh before running an inference")
	}

	protocol := "http"
	if tls {
		protocol = "https"
	}

	response, err := fvt.restConn.Post(fmt.Sprintf("%s://localhost:%d/v2/models/%s/infer", protocol, fvt.restPort, modelName), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("Request failed with code %d", response.StatusCode)
	}

	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return string(resp), err
	}
	return string(resp), nil
}

func (fvt *FVTClient) RunTfsInference(req *tfsapi.PredictRequest) (*tfsapi.PredictResponse, error) {
	if fvt.grpcConn == nil {
		return nil, errors.New("you must connect to model mesh before running an inference")
	}

	grpcClient := tfsapi.NewPredictionServiceClient(fvt.grpcConn)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return grpcClient.Predict(ctx, req)
}

func (fvt *FVTClient) ConnectToModelServing(connectionType ModelServingConnectionType) error {
	if fvt.grpcPortForward == nil {
		podName := fvt.GetRandomReadyRuntimePodNameFromEndpoints()
		fvt.grpcPortForward = NewModelMeshPortForward(fvt.namespace, podName, fvt.grpcPort, 8033, fvt.log)
	}
	if fvt.restPortForward == nil {
		podName := fvt.GetRandomReadyRuntimePodNameFromEndpoints()
		fvt.restPortForward = NewModelMeshPortForward(fvt.namespace, podName, fvt.restPort, 8008, fvt.log)
	}

	err := fvt.grpcPortForward.EnsureStarted()
	if err != nil {
		return fmt.Errorf("Error with grpc port-forward, could not connect to model serving")
	}

	err = fvt.restPortForward.EnsureStarted()
	if err != nil {
		return fmt.Errorf("Error with rest port-forward, could not connect to model serving")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	var connErr error
	if connectionType == Insecure {
		conn, connErr = grpc.DialContext(
			ctx,
			fmt.Sprintf("localhost:%d", fvt.grpcPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
	} else {
		conn, connErr = grpc.DialContext(
			ctx,
			fmt.Sprintf("localhost:%d", fvt.grpcPort),
			grpc.WithTransportCredentials(credentials.NewTLS(fvt.createTLSConfig(connectionType))),
			grpc.WithBlock(),
		)
	}

	if connErr != nil {
		return fmt.Errorf("Could not connect to grpc server at localhost. Check port forwarding command for issues. %v", connErr)
	}
	fvt.grpcConn = conn

	// create the HTTP transport for the REST proxy
	httpTransport := http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
		MaxIdleConnsPerHost: 100,
	}
	if connectionType != Insecure {
		httpTransport.TLSClientConfig = fvt.createTLSConfig(connectionType)
	}
	fvt.restConn = &http.Client{
		Transport: &httpTransport,
		Timeout:   2 * time.Minute,
	}

	return nil
}

func (fvt *FVTClient) createTLSConfig(connectionType ModelServingConnectionType) *tls.Config {
	// Create the credentials and return it
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	if connectionType == MutualTLS {
		tlsCert, err := tls.X509KeyPair(fvt.certGenerator.PublicKeyPEM.Bytes(), fvt.certGenerator.PrivateKeyPEM.Bytes())
		if err != nil {
			panic("failed to load tls client key pair")
		}

		config.Certificates = []tls.Certificate{tlsCert}
	}

	return config
}

func (fvt *FVTClient) DisconnectFromModelServing() {
	if fvt == nil {
		return
	}
	if fvt.grpcConn != nil {
		fvt.grpcConn.Close()
		fvt.grpcConn = nil
	}
	if fvt.grpcPortForward != nil {
		fvt.grpcPortForward.EnsureStopped()
		fvt.grpcPortForward = nil
	}

	if fvt.restConn != nil {
		fvt.restConn.CloseIdleConnections()
		fvt.restConn = nil
	}
	if fvt.restPortForward != nil {
		fvt.restPortForward.EnsureStopped()
		fvt.restPortForward = nil
	}
}

func (fvt *FVTClient) SetDefaultUserConfigMap() {
	fvt.ApplyUserConfigMap(DefaultConfig)
}

func (fvt *FVTClient) ApplyUserConfigMap(config map[string]interface{}) {
	var err error

	configYaml, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	cmu := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": UserConfigMapName,
			},
			"data": map[string]interface{}{
				"config.yaml": string(configYaml),
			},
		},
	}

	p, err := json.Marshal(cmu)
	Expect(err).ToNot(HaveOccurred())

	// use server-side-apply with Patch to create/update the configmap
	_, err = fvt.Resource(gvrConfigMap).Namespace(fvt.controllerNamespace).Patch(context.TODO(), cmu.GetName(), types.ApplyPatchType, p, applyPatchOptions)
	Expect(err).ToNot(HaveOccurred())
}

func (fvt *FVTClient) CreateTLSSecrets() {
	err := fvt.certGenerator.generate()
	Expect(err).ToNot(HaveOccurred())

	var TLSSecret = corev1.Secret{
		Type: corev1.SecretTypeTLS,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       SecretKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: TLSSecretName,
		},
		StringData: map[string]string{
			"tls.crt": fvt.certGenerator.PublicKeyPEM.String(),
			"tls.key": fvt.certGenerator.PrivateKeyPEM.String(),
			"ca.crt":  fvt.certGenerator.PublicKeyPEM.String(),
		},
	}

	CreateSecret(&TLSSecret, fvt)
}

func (fvt *FVTClient) UpdateConfigMapTLS(tlsConfig map[string]interface{}) {
	// Make a shallow copy of the default configmap so that we don't alter the reference to the DefaultConfig
	mergedConfigs := make(map[string]interface{})
	for k, v := range DefaultConfig {
		mergedConfigs[k] = v
	}

	// Add in the TLS configs
	// assuming we only have 1 key in tlsConfig ("tls")
	mergedConfigs["tls"] = tlsConfig["tls"]

	fvt.ApplyUserConfigMap(mergedConfigs) // CREATE or UPDATE configmap with the merged configs

	fvt.log.Info(fmt.Sprintf("Updated ConfigMap '%s'", gvrConfigMap))
}

func (fvt *FVTClient) StartWatchingDeploys() watch.Interface {
	listOptions := metav1.ListOptions{
		LabelSelector:  "modelmesh-service",
		TimeoutSeconds: &DefaultTimeout,
	}
	deployWatcher, err := fvt.Resource(gvrDeployment).Namespace(fvt.namespace).Watch(context.TODO(), listOptions)
	Expect(err).ToNot(HaveOccurred())
	return deployWatcher
}

func (fvt *FVTClient) ListDeploys() appsv1.DeploymentList {
	var err error

	// query for UnstructuredList using the dynamic client
	listOptions := metav1.ListOptions{LabelSelector: "modelmesh-service", TimeoutSeconds: &DefaultTimeout}
	u, err := fvt.Resource(gvrDeployment).Namespace(fvt.namespace).List(context.TODO(), listOptions)
	Expect(err).ToNot(HaveOccurred())

	// convert elements from Unstructured to Deployment
	var deployments appsv1.DeploymentList
	for _, ud := range u.Items {
		var d appsv1.Deployment
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, &d)
		Expect(err).ToNot(HaveOccurred())
		deployments.Items = append(deployments.Items, d)
	}

	return deployments
}

func (fvt *FVTClient) RestartDeploys() {
	// trigger a restart by patching an annotation with a timestamp
	// generate the JSON patch that adds/modifies the annotation
	patch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"fvtclient/restartedAt": time.Now().String(),
						},
					},
				},
			},
		},
	}
	patchJson, err := json.Marshal(patch)
	Expect(err).ToNot(HaveOccurred())

	deploys := fvt.ListDeploys()
	for _, d := range deploys.Items {
		dName := d.GetName()
		fvt.log.Info(fmt.Sprintf("Restarting '%s'", dName))
		// uses server-side-apply
		_, err = fvt.Resource(gvrDeployment).Namespace(fvt.namespace).
			Patch(context.TODO(), dName, types.ApplyPatchType, patchJson, applyPatchOptions)
		Expect(err).ToNot(HaveOccurred())
	}
}

func (fvt *FVTClient) DeleteConfigMap(resourceName string) error {
	configMapExists, _ := fvt.Resource(gvrConfigMap).Namespace(fvt.controllerNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})

	if configMapExists != nil {
		fvt.log.Info(fmt.Sprintf("Found configmap '%s'", resourceName))
		fvt.log.Info(fmt.Sprintf("Deleting config map '%s' ...", resourceName))
		return fvt.Resource(gvrConfigMap).Namespace(fvt.controllerNamespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	}
	return nil
}

func (fvt FVTClient) DeleteTLSSecrets() error {
	return fvt.DeleteSecret(TLSSecretName)
}

func (fvt *FVTClient) DeleteSecret(resourceName string) error {
	secretExists, _ := fvt.Resource(gvrSecret).Namespace(fvt.namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if secretExists != nil {
		fvt.log.Info(fmt.Sprintf("Found secret '%s'", resourceName))
		fvt.log.Info(fmt.Sprintf("Deleting secret '%s' ...", resourceName))
		return fvt.Resource(gvrSecret).Namespace(fvt.namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	}
	return nil
}
