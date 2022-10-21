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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/kserve/modelmesh-serving/pkg/config"

	"github.com/kserve/modelmesh-serving/controllers/modelmesh"

	bld "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kserve/modelmesh-serving/pkg/mmesh"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	commonLabelValue   = "modelmesh-controller"
	serviceMonitorName = "modelmesh-metrics-monitor"
)

// This is a map in preparation for multi-namespace support
type MMServiceMap sync.Map // string->*mmesh.MMService

func (m *MMServiceMap) Get(namespace string) *mmesh.MMService {
	if v, ok := (*sync.Map)(m).Load(namespace); ok {
		return v.(*mmesh.MMService)
	}
	return nil
}

func (m *MMServiceMap) GetOrCreate(namespace string, tlsConfig mmesh.TLSConfigLookup) (*mmesh.MMService, bool) {
	if mms := m.Get(namespace); mms != nil {
		return mms, false
	}
	v, loaded := (*sync.Map)(m).LoadOrStore(namespace, mmesh.NewMMService(namespace, tlsConfig))
	return v.(*mmesh.MMService), !loaded
}

func (m *MMServiceMap) Delete(namespace string) {
	(*sync.Map)(m).Delete(namespace)
}

// ServiceReconciler reconciles a ServingRuntime object
type ServiceReconciler struct {
	client.Client
	Log                  logr.Logger
	Scheme               *runtime.Scheme
	ConfigProvider       *config.ConfigProvider
	ConfigMapName        types.NamespacedName
	ControllerDeployment types.NamespacedName
	ClusterScope         bool

	MMServices       *MMServiceMap
	ModelEventStream *mmesh.ModelMeshEventStream

	ServiceMonitorCRDExists bool
}

func (r *ServiceReconciler) getMMService(namespace string,
	cp *config.ConfigProvider, newConfig bool) (*mmesh.MMService, *config.Config, bool) {
	mms, newSvc := r.MMServices.GetOrCreate(namespace, r.tlsConfigFromSecret)
	if newSvc || newConfig {
		if newSvc {
			r.Log.Info("MMService created for namespace", "namespace", namespace)
		}
		cfg, changed := mms.UpdateConfig(cp)
		return mms, cfg, changed
	}
	return mms, cp.GetConfig(), false
}

// +kubebuilder:rbac:groups="",resources=services;services/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces;namespaces/finalizers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(1).Info("Service reconciler called", "name", req.NamespacedName)

	var namespace string
	var owner metav1.Object
	if r.ClusterScope {
		// Per-namespace Services owned by the Namespace resource itself
		namespace = req.Name
		n := &corev1.Namespace{}
		if err := r.Client.Get(ctx, req.NamespacedName, n); err != nil {
			return ctrl.Result{}, err
		}
		if !modelMeshEnabled(n, r.ControllerDeployment.Namespace) {
			sl := &corev1.ServiceList{}
			err := r.List(ctx, sl, client.HasLabels{"modelmesh-service"}, client.InNamespace(namespace))
			if err == nil {
				for i := range sl.Items {
					s := &sl.Items[i]
					if err2 := r.Delete(ctx, s); err2 != nil && err == nil {
						err = err2
					}
				}
			}
			if mms := r.MMServices.Get(namespace); mms != nil {
				mms.Disconnect()
				r.MMServices.Delete(namespace)
			}
			return ctrl.Result{}, err
		}
		owner = n
	} else {
		// Service in same namespace as controller, owned by controller Deployment
		namespace = req.Namespace
		d := &appsv1.Deployment{}
		if err := r.Client.Get(ctx, r.ControllerDeployment, d); err != nil {
			if k8serr.IsNotFound(err) {
				// No need to delete the Service because the Deployment is the owner
				if mms := r.MMServices.Get(namespace); mms != nil {
					mms.Disconnect()
				}
				return ctrl.Result{}, nil
			}
			return RequeueResult, fmt.Errorf("could not get the controller deployment: %w", err)
		}
		owner = d
	}
	mms, cfg, _ := r.getMMService(namespace, r.ConfigProvider, false)

	var s *corev1.Service
	svc, err2, requeue := r.reconcileService(ctx, mms, namespace, owner)
	if err2 != nil || requeue {
		//TODO probably shorter requeue time (immediate?) for service recreate case
		return RequeueResult, err2
	}
	s = svc

	if err := r.ModelEventStream.UpdateWatchedService(ctx, cfg.GetEtcdSecretName(), cfg.InferenceServiceName, namespace); err != nil {
		return RequeueResult, err
	}

	// Service Monitor reconciliation should be called towards the end of the Service Reconcile method so that
	// errors returned from here should not impact any other functions.
	if s != nil && r.ServiceMonitorCRDExists {
		// Reconcile Service Monitor if the ServiceMonitor CRD exists
		err, requeue := r.reconcileServiceMonitor(ctx, cfg.Metrics, s)
		if err != nil || requeue {
			return RequeueResult, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) tlsConfigFromSecret(ctx context.Context, secretName string) (*tls.Config, error) {
	if secretName == "" {
		return nil, nil
	}
	tlsSecret := corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: r.ControllerDeployment.Namespace}, &tlsSecret)
	if err != nil {
		r.Log.Error(err, "Unable to access TLS secret", "secretName", secretName)
		return nil, fmt.Errorf("unable to access TLS secret '%s': %v", secretName, err)
	}
	cert, ok2 := tlsSecret.Data[modelmesh.TLSSecretCertKey]
	key, ok := tlsSecret.Data[modelmesh.TLSSecretKeyKey]
	if !ok || !ok2 {
		r.Log.Error(err, "TLS secret missing required keys", "secretName", secretName)
		return nil, fmt.Errorf("TLS secret '%s' missing %s and/or %s",
			secretName, modelmesh.TLSSecretCertKey, modelmesh.TLSSecretKeyKey)
	}
	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		r.Log.Error(err, "Could not load client key pair")
		return nil, fmt.Errorf("could not load client key pair: %v", err)
	}
	certPool, _ := x509.SystemCertPool() // this returns a copy
	if certPool == nil {
		certPool = x509.NewCertPool()
	}
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		return nil, errors.New("failed to append ca certs")
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, RootCAs: certPool}, nil
}

func (r *ServiceReconciler) reconcileService(ctx context.Context, mms *mmesh.MMService,
	namespace string, owner metav1.Object) (*corev1.Service, error, bool) {
	serviceName, target := mms.GetNameAndSpec()
	if serviceName == "" || target == nil {
		return nil, errors.New("unexpected state - MMService uninitialized"), false
	}

	sl := &corev1.ServiceList{}
	if err := r.List(ctx, sl, client.HasLabels{"modelmesh-service"}, client.InNamespace(namespace)); err != nil {
		return nil, err, false
	}
	var s *corev1.Service
	for i := range sl.Items {
		ss := &sl.Items[i]
		if ss.GetName() == serviceName {
			s = ss
		} else if err := r.Delete(ctx, ss); err != nil {
			return nil, err, false
		} else {
			r.ModelEventStream.RemoveWatchedService(ss.GetName(), ss.GetNamespace())
			r.Log.V(1).Info("Deleted Service with label but different name", "name", ss.GetName(), "namespace", ss.GetNamespace())
		}
	}

	labelMap := map[string]string{
		"modelmesh-service":            serviceName,
		"app.kubernetes.io/instance":   commonLabelValue,
		"app.kubernetes.io/managed-by": commonLabelValue,
		"app.kubernetes.io/name":       commonLabelValue,
	}

	annotationsMap := map[string]string{
		"service.alpha.openshift.io/serving-cert-secret-name": "model-serving-proxy-tls",
	}

	if s == nil {
		mms.Disconnect()
		s = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        serviceName,
				Namespace:   namespace,
				Labels:      labelMap,
				Annotations: annotationsMap,
			},
			Spec: *target,
		}
		if err := controllerutil.SetControllerReference(owner, s, r.Scheme); err != nil {
			return nil, fmt.Errorf("could not set Service owner reference: %w", err), false
		}
		if err := r.Create(ctx, s); err != nil {
			return nil, fmt.Errorf("could not create service: %w", err), false
		}
		r.Log.Info("Created Kube Service", "name", serviceName, "namespace", namespace)
		return s, nil, false
	}

	var origClusterIp = s.Spec.ClusterIP
	if origClusterIp != "None" {
		// Kube sets this when non-headless, so zero-out for comparison
		s.Spec.ClusterIP = ""
	}

	var err error
	if !reflect.DeepEqual(s.Spec, target) || !reflect.DeepEqual(s.Labels, labelMap) {
		if s.Spec.ClusterIP != target.ClusterIP {
			r.Log.Info("Deleting/recreating Service because headlessness changed",
				"service", serviceName, "headless", target.ClusterIP == "None")
			// Have to recreate since ClusterIP field is immutable
			err2 := r.Delete(ctx, s)
			// This will trigger immediate re-reconcilation
			return nil, err2, true
		}
		s.Labels = labelMap
		s.Spec = *target
		s.Spec.ClusterIP = origClusterIp
		if err = r.Update(ctx, s); err != nil {
			r.Log.Error(err, "Could not update Kube Service", "name", serviceName, "namespace", namespace)
		} else {
			r.Log.Info("Updated Kube Service", "name", serviceName, "namespace", namespace)
		}
	}

	if err2 := mms.ConnectIfNeeded(ctx); err2 != nil {
		if err == nil {
			err = fmt.Errorf("error establishing model-mesh gRPC conn: %w", err2)
		} else {
			r.Log.Error(err2, "error establishing model-mesh gRPC conn")
		}
	}

	return s, err, false
}

func (r *ServiceReconciler) reconcileServiceMonitor(ctx context.Context, metrics config.PrometheusConfig, owner *corev1.Service) (error, bool) {
	r.Log.V(1).Info("Reconciling Service Monitor", "namespace", owner.GetNamespace())

	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceMonitorName,
			Namespace: owner.GetNamespace(),
		},
	}

	err := r.Client.Get(ctx, client.ObjectKey{Name: serviceMonitorName, Namespace: owner.GetNamespace()}, sm)
	exists := true
	if k8serr.IsNotFound(err) {
		// Create the ServiceMonitor if not found
		exists = false
	} else if err != nil {
		return fmt.Errorf("unable to get service monitor %s: %w", serviceMonitorName, err), false
	}

	// Check if the prometheus operator support is enabled
	if metrics.DisablePrometheusOperatorSupport || !metrics.Enabled {
		r.Log.V(1).Info("Configuration parameter 'DisablePrometheusOperatorSupport' is set to true or Metrics is disabled",
			"DisablePrometheusOperatorSupport", metrics.DisablePrometheusOperatorSupport, "metrics.Enabled", metrics.Enabled)
		if exists {
			// Delete ServiceMonitor CR if already exists
			if err = r.Client.Delete(ctx, sm); err != nil {
				return fmt.Errorf("unable to delete service monitor %s: %w", serviceMonitorName, err), false
			}
		}
		return nil, false
	}

	crBefore := sm.GetOwnerReferences()
	if err = controllerutil.SetControllerReference(owner, sm, r.Scheme); err != nil {
		return fmt.Errorf("could not set ServiceMonitor owner reference: %w", err), false
	}
	changed := reflect.DeepEqual(crBefore, sm.GetOwnerReferences())

	if !reflect.DeepEqual(sm.Labels, owner.Labels) {
		sm.Labels = owner.Labels
		changed = true
	}

	targetSpec := monitoringv1.ServiceMonitorSpec{
		Selector:          metav1.LabelSelector{MatchLabels: map[string]string{"modelmesh-service": owner.GetName()}},
		NamespaceSelector: monitoringv1.NamespaceSelector{MatchNames: []string{sm.Namespace}},
		Endpoints: []monitoringv1.Endpoint{{
			Interval: "30s",
			Port:     "prometheus",
			Scheme:   metrics.Scheme,
			TLSConfig: &monitoringv1.TLSConfig{
				SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: true}, //TODO: Update the TLSConfig to use cacert
			}},
		},
	}

	// Compare individual fields so that others can be set manually without getting reverted
	if !reflect.DeepEqual(sm.Spec.Selector, targetSpec.Selector) {
		sm.Spec.Selector = targetSpec.Selector
		changed = true
	}
	if !reflect.DeepEqual(sm.Spec.NamespaceSelector, targetSpec.NamespaceSelector) {
		sm.Spec.NamespaceSelector = targetSpec.NamespaceSelector
		changed = true
	}
	if !reflect.DeepEqual(sm.Spec.Endpoints, targetSpec.Endpoints) {
		sm.Spec.Endpoints = targetSpec.Endpoints
		changed = true
	}

	if changed {
		if !exists {
			err = r.Client.Create(ctx, sm)
		} else {
			err = r.Client.Update(ctx, sm)
		}
		if k8serr.IsConflict(err) {
			// this can occur during normal operations if the Service Monitor was updated during this reconcile loop
			r.Log.Info("Could not create (or) update service monitor due to resource conflict", "namespace", sm.Namespace)
			return nil, true
		} else if err != nil {
			return fmt.Errorf("could not create (or) update service monitor %s: %w", serviceMonitorName, err), false
		} else {
			r.Log.Info("Created or Updated ServiceMonitor", "name", serviceMonitorName, "namespace", sm.Namespace)
		}
	}
	return nil, false
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).Named("ServiceReconciler").Owns(&corev1.Service{})
	if r.ClusterScope {
		// Services are owned by Namespace resources
		r.setupForClusterScope(builder)
	} else {
		// Service is owned by controller Deployment (same namespace only)
		r.setupForNamespaceScope(builder)
	}
	return builder.Complete(r)
}

func (r *ServiceReconciler) setupForNamespaceScope(builder *bld.Builder) {
	filter := func(meta metav1.Object) bool {
		return meta.GetName() == r.ControllerDeployment.Name &&
			meta.GetNamespace() == r.ControllerDeployment.Namespace
	}
	builder.For(&appsv1.Deployment{}, bld.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool { return filter(event.Object) },
		UpdateFunc: func(event event.UpdateEvent) bool { return filter(event.ObjectNew) },
		DeleteFunc: func(event event.DeleteEvent) bool { return false },
	})).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			config.ConfigWatchHandler(r.ConfigMapName, func() []reconcile.Request {
				if _, _, changed := r.getMMService(r.ControllerDeployment.Namespace, r.ConfigProvider, true); changed {
					r.Log.Info("Triggering service reconciliation after config change")
					return []reconcile.Request{{NamespacedName: r.ControllerDeployment}}
				}
				return []reconcile.Request{}
			}, r.ConfigProvider, &r.Client))

	// Enable ServiceMonitor watch if ServiceMonitorCRDExists
	if r.ServiceMonitorCRDExists {
		builder.Watches(&source.Kind{Type: &monitoringv1.ServiceMonitor{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				if o.GetName() == serviceMonitorName && o.GetNamespace() == r.ControllerDeployment.Namespace {
					return []reconcile.Request{{NamespacedName: r.ControllerDeployment}}
				}
				return []reconcile.Request{}
			}))
	}
}

func (r *ServiceReconciler) setupForClusterScope(builder *bld.Builder) {
	builder.For(&corev1.Namespace{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			config.ConfigWatchHandler(r.ConfigMapName, func() []reconcile.Request {
				list := &corev1.NamespaceList{}
				if err := r.Client.List(context.TODO(), list); err != nil {
					r.Log.Error(err, "Error listing Namespaces to reconcile after config change")
					return []reconcile.Request{}
				}
				requests := make([]reconcile.Request, 0, len(list.Items))
				for i := range list.Items {
					if n := &list.Items[i]; modelMeshEnabled(n, r.ControllerDeployment.Namespace) {
						if _, _, changed := r.getMMService(n.Name, r.ConfigProvider, true); changed {
							requests = append(requests, reconcile.Request{
								NamespacedName: types.NamespacedName{Name: n.Name, Namespace: n.Namespace},
							})
						}
					}
				}
				return requests
			}, r.ConfigProvider, &r.Client))

	// Enable ServiceMonitor watch if ServiceMonitorCRDExists
	if r.ServiceMonitorCRDExists {
		builder.Watches(&source.Kind{Type: &monitoringv1.ServiceMonitor{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				if o.GetName() == serviceMonitorName {
					return []reconcile.Request{{
						NamespacedName: types.NamespacedName{Name: o.GetNamespace()},
					}}
				}
				return []reconcile.Request{}
			}))
	}
}
