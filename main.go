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

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/kserve/modelmesh-serving/pkg/predictor_source"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/operator-framework/operator-lib/leader"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	servingv1alpha1 "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	"github.com/kserve/modelmesh-serving/controllers"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
	"github.com/kserve/modelmesh-serving/pkg/mmesh"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	kfsv1alpha1 "github.com/kserve/modelmesh-serving/apis/kfserving/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme              = runtime.NewScheme()
	setupLog            = ctrl.Log.WithName("setup")
	ControllerNamespace string
)

const (
	ControllerNamespaceEnvVar      = "NAMESPACE"
	DefaultControllerNamespace     = "model-serving"
	KubeNamespaceFile              = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	ControllerPodNameEnvVar        = "POD_NAME"
	ControllerDeploymentNameEnvVar = "CONTROLLER_DEPLOYMENT"
	DefaultControllerName          = "modelmesh-controller"
	UserConfigMapName              = "model-serving-config"
	DevModeLoggingEnvVar           = "DEV_MODE_LOGGING"
	serviceMonitorCRDName          = "servicemonitors.monitoring.coreos.com"
	LeaderLockName                 = "modelmesh-controller-leader-lock"
	LeaderForLifeLockName          = "modelmesh-controller-leader-for-life-lock"
	EnableTrainedModelEnvVar       = "ENABLE_KSTM_WATCH"
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	err := servingv1alpha1.AddToScheme(scheme)
	if err != nil {
		log.Fatalf("cannot add model serving v1 scheme, %v", err)
	}
	_ = batchv1.AddToScheme(scheme)
	_ = servingv1alpha1.AddToScheme(scheme)
	_ = kfsv1alpha1.AddToScheme(scheme)
	_ = monitoringv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	_, devLoggingSetting := os.LookupEnv(DevModeLoggingEnvVar)
	ctrl.SetLogger(zap.New(zap.UseDevMode(devLoggingSetting)))

	// ----- mmesh related envar setup -----
	controllerNamespace := os.Getenv(ControllerNamespaceEnvVar)
	if controllerNamespace == "" {
		bytes, err := ioutil.ReadFile(KubeNamespaceFile)
		if err != nil {
			//TODO check kube context and retrieve namespace from there
			setupLog.Info("Error reading Kube-mounted namespace file, reverting to default namespace",
				"file", KubeNamespaceFile, "err", err, "default", DefaultControllerNamespace)
			controllerNamespace = DefaultControllerNamespace
		} else {
			controllerNamespace = string(bytes)
		}
	}
	ControllerNamespace = controllerNamespace

	controllerDeploymentName := os.Getenv(ControllerDeploymentNameEnvVar)
	if controllerDeploymentName == "" {
		podName := os.Getenv(ControllerPodNameEnvVar)
		if podName != "" {
			if matches := regexp.MustCompile("(.*)-.*-.*").FindStringSubmatch(podName); len(matches) == 2 {
				deployment := matches[1]
				setupLog.Info("Use controller deployment from POD_NAME", "Deployment", deployment)
				controllerDeploymentName = deployment
			}
		}
		if controllerDeploymentName == "" {
			setupLog.Info("Skip empty Controller deployment from Env Var, use default",
				"name", DefaultControllerName)
			controllerDeploymentName = DefaultControllerName
		}
	}

	// TODO: use the manager client instead. This will require restructuring the dependency
	// relationship with the manager so that this code runs after mgr.Start()
	cfg := config.GetConfigOrDie()
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create an api server client")
		os.Exit(1)
	}

	cp, err := controllers.NewConfigProvider(context.Background(), cl, types.NamespacedName{Name: UserConfigMapName, Namespace: ControllerNamespace})
	if err != nil {
		setupLog.Error(err, "Error loading user config from configmap", "ConfigMapName", UserConfigMapName)
		os.Exit(1)
	}
	conf := cp.GetConfig()

	setupLog.Info("Using adapter", "image", conf.StorageHelperImage.TaggedImage())
	setupLog.Info("Using modelmesh", "image", conf.ModelMeshImage.TaggedImage())

	if conf.RESTProxy.Enabled {
		setupLog.Info("Using modelmesh REST proxy", "image", conf.RESTProxy.Image.TaggedImage())
	}
	setupLog.Info("Using inference service", "name", conf.InferenceServiceName, "port", conf.InferenceServicePort)

	// mmesh service kubedns or hostname
	mmeshEndpoint := conf.ModelMeshEndpoint

	setupLog.Info("MMesh Configuration", "serviceName", conf.InferenceServiceName, "port", conf.InferenceServicePort,
		"mmeshEndpoint", mmeshEndpoint)

	//TODO: this should be moved out of package globals
	modelmesh.StorageSecretName = conf.StorageSecretName

	// ----- end of mmesh related envar setup -----

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var leaderElectionType string
	var leaseDuration time.Duration
	var leaseRenewDeadline time.Duration
	var leaseRetryPeriod time.Duration
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionType, "leader-elect-type", "lease",
		"Leader election type. Values: ['lease'=leader with lease]; ['life'=leader for life].")
	flag.DurationVar(&leaseDuration, "leader-elect-lease-duration-sec", 15*time.Second,
		"Duration that non-leader candidates will wait to force acquire leadership.")
	flag.DurationVar(&leaseRenewDeadline, "leader-elect-lease-renew-deadline-sec", 10*time.Second,
		"Duration the leader will retry refreshing leadership before giving up.")
	flag.DurationVar(&leaseRetryPeriod, "leader-elect-retry-period-sec", 2*time.Second,
		"Duration the Leader elector clients should wait between tries of actions.")
	flag.Parse()

	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Namespace:              ControllerNamespace,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
	}

	if enableLeaderElection {
		if leaderElectionType == "lease" {
			setupLog.Info("using leader-with-lease election method")
			mgrOpts.LeaderElectionNamespace = ControllerNamespace
			mgrOpts.LeaderElection = enableLeaderElection
			mgrOpts.LeaderElectionID = LeaderLockName
			mgrOpts.LeaseDuration = &leaseDuration
			mgrOpts.RenewDeadline = &leaseRenewDeadline
			mgrOpts.RetryPeriod = &leaseRetryPeriod
		} else if leaderElectionType == "life" {
			setupLog.Info("using leader-for-life election method")
			// try to become leader using leader-for-life
			err = leader.Become(context.TODO(), LeaderForLifeLockName)
			if err != nil {
				setupLog.Error(err, "Failed to obtain leader-for-life lock")
				os.Exit(1)
			}
		} else {
			err = fmt.Errorf("Invalid value for leader-elect-type: '%s'. Use 'lease' or 'life'", leaderElectionType)
			setupLog.Error(err, "Error processing command-line flags.")
			os.Exit(1)
		}
	} else {
		setupLog.Info("leader election is disabled")
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	_, err = mmesh.InitGrpcResolver(ControllerNamespace, mgr)
	if err != nil {
		setupLog.Error(err, "Failed to Initialize Grpc Resolver, exit")
		os.Exit(1)
	}

	mmService := mmesh.NewMMService()

	modelEventStream, err := mmesh.NewModelEventStream(ctrl.Log.WithName("ModelMeshEventStream"),
		mgr.GetClient(), ControllerNamespace)
	if err != nil {
		setupLog.Error(err, "Failed to Initialize Model Event Stream, exit")
		os.Exit(1)
	}

	// Check if the ServiceMonitor CRD exists in the cluster
	sm := &monitoringv1.ServiceMonitor{}
	serviceMonitorCRDExists := true
	err = cl.Get(context.Background(), client.ObjectKey{Name: "foo", Namespace: controllerNamespace}, sm)
	if meta.IsNoMatchError(err) {
		serviceMonitorCRDExists = false
		setupLog.Info("Service Monitor CRD is not found in the cluster")
	} else if err != nil && !errors.IsNotFound(err) {
		serviceMonitorCRDExists = false
		setupLog.Error(err, "Unable to access Service Monitor CRD", "CRDName", serviceMonitorCRDName)
	}

	if err = (&controllers.ServiceReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("controllers").WithName("Service"),
		Scheme:                  mgr.GetScheme(),
		ControllerDeployment:    types.NamespacedName{Namespace: ControllerNamespace, Name: controllerDeploymentName},
		ModelMeshService:        mmService,
		ModelEventStream:        modelEventStream,
		ConfigProvider:          cp,
		ConfigMapName:           types.NamespacedName{Namespace: ControllerNamespace, Name: UserConfigMapName},
		ServiceMonitorCRDExists: serviceMonitorCRDExists,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "Service")
		os.Exit(1)
	}

	//TODO populate with registered/loaded plugins
	sources := []predictor_source.PredictorSource{}

	registryMap := map[string]predictor_source.PredictorRegistry{
		controllers.PredictorCRSourceId: predictor_source.PredictorCRRegistry{Client: mgr.GetClient()},
	}

	// Watch TrainedModels only if env var is explicitly set to "true" or if
	// env var is unset/empty and TrainedModel CRD is present and accessible.
	enableTMWatch := false
	envVarVal, _ := os.LookupEnv(EnableTrainedModelEnvVar)
	if envVarVal != "false" {
		tm := &kfsv1alpha1.TrainedModel{}
		err = cl.Get(context.Background(), client.ObjectKey{Name: "foo", Namespace: ControllerNamespace}, tm)
		if err == nil || errors.IsNotFound(err) {
			enableTMWatch = true
			registryMap[controllers.TrainedModelCRSourceId] = predictor_source.TrainedModelRegistry{Client: mgr.GetClient()}
			setupLog.Info("Reconciliation of TrainedModels is enabled")
		} else if envVarVal == "true" {
			// If env var is explicitly true, require that TrainedModel CRD is present
			setupLog.Error(err, "Unable to access TrainedModel Custom Resource")
			os.Exit(1)
		} else if meta.IsNoMatchError(err) {
			setupLog.Info("TrainedModel CRD not found, will not reconcile")
		} else {
			setupLog.Error(err, "TrainedModel CRD not accessible, will not reconcile")
		}
	}

	var predictorControllerEvents, runtimeControllerEvents chan event.GenericEvent
	if len(sources) != 0 {
		predictorControllerEvents = make(chan event.GenericEvent, 256)
		runtimeControllerEvents = make(chan event.GenericEvent, 256)
		dispatchers := make([]func(), 0, len(sources))
		for _, s := range sources {
			sid := s.GetSourceId()
			if sid == "" || sid == controllers.PredictorCRSourceId || sid == controllers.TrainedModelCRSourceId {
				setupLog.Error(nil, "Invalid predictor source plugin id",
					"sourceId", sid)
				os.Exit(1)
			}
			if _, ok := registryMap[sid]; ok {
				setupLog.Error(nil, "More than one predictor source plugin is registered with the same id",
					"sourceId", sid)
				os.Exit(1)
			}
			r, c, serr := s.StartWatch(context.Background())
			if serr != nil {
				setupLog.Error(serr, "Error starting predictor source plugin", "sourceId", sid)
				os.Exit(1)
			}
			registryMap[sid] = r
			dispatchers = append(dispatchers, func() {
				for {
					pe, ok := <-c
					if !ok {
						break
					}
					event := event.GenericEvent{Object: &v1.PartialObjectMetadata{ObjectMeta: v1.ObjectMeta{
						Name: pe.Name, Namespace: fmt.Sprintf("%s_%s", sid, pe.Namespace)},
					}}
					predictorControllerEvents <- event
					runtimeControllerEvents <- event
				}
				setupLog.Info("Predictor source plugin event channel closed", "sourceId", sid)
			})
		}
		for _, d := range dispatchers {
			go d()
		}
	}

	if err = (&controllers.PredictorReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("Predictor"),
		MMService:      mmService,
		RegistryLookup: registryMap,
	}).SetupWithManager(mgr, modelEventStream, enableTMWatch, predictorControllerEvents); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Predictor")
		os.Exit(1)
	}

	if err = (&controllers.ServingRuntimeReconciler{
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("ServingRuntime"),
		Scheme:              mgr.GetScheme(),
		ConfigProvider:      cp,
		ConfigMapName:       types.NamespacedName{Namespace: ControllerNamespace, Name: UserConfigMapName},
		DeploymentNamespace: ControllerNamespace,
		DeploymentName:      controllerDeploymentName,
		RegistryMap:         registryMap,
	}).SetupWithManager(mgr, enableTMWatch, runtimeControllerEvents); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServingRuntime")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	// Add Healthz Endpoint
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// Add Readyz Endpoint
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
