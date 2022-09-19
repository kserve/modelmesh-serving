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
	"github.com/kserve/modelmesh-serving/apis/serving/common"
	corev1 "k8s.io/api/core/v1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	inferenceservicev1 "github.com/kserve/modelmesh-serving/apis/serving/v1beta1"
)

var _ = Describe("The Openshift model controller", func() {
	// Define utility constants for testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 60
		interval = time.Second * 2
	)

	Context("When creating a InferenceService", func() {
		const (
			Name      = "test-inferenceservice"
			Namespace = "default"
		)

		var storagePath = "/testpath/test"
		var storageKey = "testkey"
		inferenceservice := &inferenceservicev1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: Namespace,
			},
			Spec: inferenceservicev1.InferenceServiceSpec{
				Predictor: inferenceservicev1.InferenceServicePredictorSpec{
					Model: &inferenceservicev1.ModelSpec{
						ModelFormat: inferenceservicev1.ModelFormat{
							Name: Name,
						},
						PredictorExtensionSpec: inferenceservicev1.PredictorExtensionSpec{
							Storage: &common.StorageSpec{
								Path:       &storagePath,
								StorageKey: &storageKey,
							},
						},
					},
				},
			},
		}

		expectedRoute := routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: Namespace,
				Labels: map[string]string{
					"inferenceservice-name": Name,
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "modelmesh-serving",
					Weight: pointer.Int32Ptr(100),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromInt(8008),
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationEdge,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
				WildcardPolicy: routev1.WildcardPolicyNone,
				Path:           "/v2/models/" + Name,
			},
			Status: routev1.RouteStatus{
				Ingress: []routev1.RouteIngress{},
			},
		}

		route := &routev1.Route{}

		It("Should create a Route to expose the traffic externally", func() {
			ctx := context.Background()

			By("By creating a new predictor")
			Expect(cli.Create(ctx, inferenceservice)).Should(Succeed())
			time.Sleep(interval)

			By("By checking that the controller has created the Route")
			Eventually(func() error {
				key := types.NamespacedName{Name: Name, Namespace: Namespace}
				return cli.Get(ctx, key, route)
			}, timeout, interval).ShouldNot(HaveOccurred())
			Expect(CompareInferenceServiceRoutes(*route, expectedRoute)).Should(BeTrue())
		})

		namespace := &corev1.Namespace{}
		It("Should add appropriate labels to the namespace", func() {
			ctx := context.Background()

			By("By checking that the controller has created the Route")
			Eventually(func() error {
				key := types.NamespacedName{Name: Namespace, Namespace: Namespace}
				return cli.Get(ctx, key, namespace)
			}, timeout, interval).ShouldNot(HaveOccurred())
			Expect(CheckForNamespaceLabel("modelmesh-enabled", "true", namespace)).Should(BeTrue())
		})

		// It("Should reconcile the Route when modified", func() {
		// 	By("By simulating a manual Route modification")
		// 	patch := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"to":{"name":"foo"}}}`))
		// 	Expect(cli.Patch(ctx, route, patch)).Should(Succeed())
		// 	time.Sleep(interval)

		// 	By("By checking that the controller has restored the Route spec")
		// 	Eventually(func() (string, error) {
		// 		key := types.NamespacedName{Name: Name, Namespace: Namespace}
		// 		err := cli.Get(ctx, key, route)
		// 		if err != nil {
		// 			return "", err
		// 		}
		// 		return route.Spec.To.Name, nil
		// 	}, timeout, interval).Should(Equal(Name))
		// 	Expect(ComparePredictorRoutes(*route, expectedRoute)).Should(BeTrue())
		// })

		// It("Should recreate the Route when deleted", func() {
		// 	By("By deleting the Predictor route")
		// 	Expect(cli.Delete(ctx, route)).Should(Succeed())
		// 	time.Sleep(interval)

		// 	By("By checking that the controller has recreated the Route")
		// 	Eventually(func() error {
		// 		key := types.NamespacedName{Name: Name, Namespace: Namespace}
		// 		return cli.Get(ctx, key, route)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())
		// 	Expect(ComparePredictorRoutes(*route, expectedRoute)).Should(BeTrue())
		// })

		// It("Should delete the Openshift Route", func() {
		// 	// Testenv cluster does not implement Kubernetes GC:
		// 	// https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		// 	// To test that the deletion lifecycle works, test the ownership
		// 	// instead of asserting on existence.
		// 	expectedOwnerReference := metav1.OwnerReference{
		// 		APIVersion:         "kubeflow.org/v1",
		// 		Kind:               "Predictor",
		// 		Name:               Name,
		// 		UID:                Predictor.GetObjectMeta().GetUID(),
		// 		Controller:         pointer.BoolPtr(true),
		// 		BlockOwnerDeletion: pointer.BoolPtr(true),
		// 	}

		// 	By("By checking that the Predictor owns the Route object")
		// 	Expect(route.GetObjectMeta().GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

		// 	By("By deleting the recently created Predictor")
		// 	Expect(cli.Delete(ctx, Predictor)).Should(Succeed())
		// 	time.Sleep(interval)

		// 	By("By checking that the Predictor is deleted")
		// 	Eventually(func() error {
		// 		key := types.NamespacedName{Name: Name, Namespace: Namespace}
		// 		return cli.Get(ctx, key, Predictor)
		// 	}, timeout, interval).Should(HaveOccurred())
		// })
	})

	// Context("When creating a Predictor with the OAuth annotation enabled", func() {
	// 	const (
	// 		Name      = "test-Predictor-oauth"
	// 		Namespace = "default"
	// 	)

	// 	Predictor := &nbv1.Predictor{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      Name,
	// 			Namespace: Namespace,
	// 			Labels: map[string]string{
	// 				"app.kubernetes.io/instance": Name,
	// 			},
	// 			Annotations: map[string]string{
	// 				"Predictors.opendatahub.io/inject-oauth": "true",
	// 				"Predictors.opendatahub.io/foo":          "bar",
	// 			},
	// 		},
	// 		Spec: nbv1.PredictorSpec{
	// 			Template: nbv1.PredictorTemplateSpec{
	// 				Spec: corev1.PodSpec{
	// 					Containers: []corev1.Container{{
	// 						Name:  Name,
	// 						Image: "registry.redhat.io/ubi8/ubi:latest",
	// 					}},
	// 					Volumes: []corev1.Volume{
	// 						{
	// 							Name: "Predictor-data",
	// 							VolumeSource: corev1.VolumeSource{
	// 								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
	// 									ClaimName: Name + "-data",
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	}

	// 	expectedPredictor := nbv1.Predictor{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      Name,
	// 			Namespace: Namespace,
	// 			Labels: map[string]string{
	// 				"app.kubernetes.io/instance": Name,
	// 			},
	// 			Annotations: map[string]string{
	// 				"Predictors.opendatahub.io/inject-oauth": "true",
	// 				"Predictors.opendatahub.io/foo":          "bar",
	// 			},
	// 		},
	// 		Spec: nbv1.PredictorSpec{
	// 			Template: nbv1.PredictorTemplateSpec{
	// 				Spec: corev1.PodSpec{
	// 					ServiceAccountName: Name,
	// 					Containers: []corev1.Container{
	// 						{
	// 							Name:  Name,
	// 							Image: "registry.redhat.io/ubi8/ubi:latest",
	// 						},
	// 						{
	// 							Name:            "oauth-proxy",
	// 							Image:           OAuthProxyImage,
	// 							ImagePullPolicy: corev1.PullAlways,
	// 							Env: []corev1.EnvVar{{
	// 								Name: "NAMESPACE",
	// 								ValueFrom: &corev1.EnvVarSource{
	// 									FieldRef: &corev1.ObjectFieldSelector{
	// 										FieldPath: "metadata.namespace",
	// 									},
	// 								},
	// 							}},
	// 							Args: []string{
	// 								"--provider=openshift",
	// 								"--https-address=:8443",
	// 								"--http-address=",
	// 								"--openshift-service-account=" + Name,
	// 								"--cookie-secret-file=/etc/oauth/config/cookie_secret",
	// 								"--cookie-expire=24h0m0s",
	// 								"--tls-cert=/etc/tls/private/tls.crt",
	// 								"--tls-key=/etc/tls/private/tls.key",
	// 								"--upstream=http://localhost:8888",
	// 								"--upstream-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	// 								"--skip-auth-regex=^/api$",
	// 								"--email-domain=*",
	// 								"--skip-provider-button",
	// 								`--openshift-sar={"verb":"get","resource":"Predictors","resourceAPIGroup":"kubeflow.org",` +
	// 									`"resourceName":"` + Name + `","namespace":"$(NAMESPACE)"}`,
	// 							},
	// 							Ports: []corev1.ContainerPort{{
	// 								Name:          OAuthServicePortName,
	// 								ContainerPort: 8443,
	// 								Protocol:      corev1.ProtocolTCP,
	// 							}},
	// 							LivenessProbe: &corev1.Probe{
	// 								ProbeHandler: corev1.ProbeHandler{
	// 									HTTPGet: &corev1.HTTPGetAction{
	// 										Path:   "/oauth/healthz",
	// 										Port:   intstr.FromString(OAuthServicePortName),
	// 										Scheme: corev1.URISchemeHTTPS,
	// 									},
	// 								},
	// 								InitialDelaySeconds: 30,
	// 								TimeoutSeconds:      1,
	// 								PeriodSeconds:       5,
	// 								SuccessThreshold:    1,
	// 								FailureThreshold:    3,
	// 							},
	// 							ReadinessProbe: &corev1.Probe{
	// 								ProbeHandler: corev1.ProbeHandler{
	// 									HTTPGet: &corev1.HTTPGetAction{
	// 										Path:   "/oauth/healthz",
	// 										Port:   intstr.FromString(OAuthServicePortName),
	// 										Scheme: corev1.URISchemeHTTPS,
	// 									},
	// 								},
	// 								InitialDelaySeconds: 5,
	// 								TimeoutSeconds:      1,
	// 								PeriodSeconds:       5,
	// 								SuccessThreshold:    1,
	// 								FailureThreshold:    3,
	// 							},
	// 							Resources: corev1.ResourceRequirements{
	// 								Requests: corev1.ResourceList{
	// 									"cpu":    resource.MustParse("100m"),
	// 									"memory": resource.MustParse("64Mi"),
	// 								},
	// 								Limits: corev1.ResourceList{
	// 									"cpu":    resource.MustParse("100m"),
	// 									"memory": resource.MustParse("64Mi"),
	// 								},
	// 							},
	// 							VolumeMounts: []corev1.VolumeMount{
	// 								{
	// 									Name:      "oauth-config",
	// 									MountPath: "/etc/oauth/config",
	// 								},
	// 								{
	// 									Name:      "tls-certificates",
	// 									MountPath: "/etc/tls/private",
	// 								},
	// 							},
	// 						},
	// 					},
	// 					Volumes: []corev1.Volume{
	// 						{
	// 							Name: "Predictor-data",
	// 							VolumeSource: corev1.VolumeSource{
	// 								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
	// 									ClaimName: Name + "-data",
	// 								},
	// 							},
	// 						},
	// 						{
	// 							Name: "oauth-config",
	// 							VolumeSource: corev1.VolumeSource{
	// 								Secret: &corev1.SecretVolumeSource{
	// 									SecretName:  Name + "-oauth-config",
	// 									DefaultMode: pointer.Int32Ptr(420),
	// 								},
	// 							},
	// 						},
	// 						{
	// 							Name: "tls-certificates",
	// 							VolumeSource: corev1.VolumeSource{
	// 								Secret: &corev1.SecretVolumeSource{
	// 									SecretName:  Name + "-tls",
	// 									DefaultMode: pointer.Int32Ptr(420),
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	}

	// 	It("Should inject the OAuth proxy as a sidecar container", func() {
	// 		ctx := context.Background()

	// 		By("By creating a new Predictor")
	// 		Expect(cli.Create(ctx, Predictor)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the webhook has injected the sidecar container")
	// 		Expect(ComparePredictors(*Predictor, expectedPredictor)).Should(BeTrue())
	// 	})

	// 	It("Should reconcile the Predictor when modified", func() {
	// 		By("By simulating a manual Predictor modification")
	// 		Predictor.Spec.Template.Spec.ServiceAccountName = "foo"
	// 		Predictor.Spec.Template.Spec.Containers[1].Image = "bar"
	// 		Predictor.Spec.Template.Spec.Volumes[1].VolumeSource = corev1.VolumeSource{}
	// 		Expect(cli.Update(ctx, Predictor)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the webhook has restored the Predictor spec")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, Predictor)
	// 		}, timeout, interval).Should(Succeed())
	// 		Expect(ComparePredictors(*Predictor, expectedPredictor)).Should(BeTrue())
	// 	})

	// 	serviceAccount := &corev1.ServiceAccount{}
	// 	expectedServiceAccount := corev1.ServiceAccount{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      Name,
	// 			Namespace: Namespace,
	// 			Labels: map[string]string{
	// 				"Predictor-name": Name,
	// 			},
	// 			Annotations: map[string]string{
	// 				"serviceaccounts.openshift.io/oauth-redirectreference.first": "" +
	// 					`{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"` + Name + `"}}`,
	// 			},
	// 		},
	// 	}

	// 	It("Should create a Service Account for the Predictor", func() {
	// 		By("By checking that the controller has created the Service Account")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, serviceAccount)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorServiceAccounts(*serviceAccount, expectedServiceAccount)).Should(BeTrue())
	// 	})

	// 	It("Should recreate the Service Account when deleted", func() {
	// 		By("By deleting the Predictor Service Account")
	// 		Expect(cli.Delete(ctx, serviceAccount)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the controller has recreated the Service Account")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, serviceAccount)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorServiceAccounts(*serviceAccount, expectedServiceAccount)).Should(BeTrue())
	// 	})

	// 	service := &corev1.Service{}
	// 	expectedService := corev1.Service{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      Name + "-tls",
	// 			Namespace: Namespace,
	// 			Labels: map[string]string{
	// 				"Predictor-name": Name,
	// 			},
	// 			Annotations: map[string]string{
	// 				"service.beta.openshift.io/serving-cert-secret-name": Name + "-tls",
	// 			},
	// 		},
	// 		Spec: corev1.ServiceSpec{
	// 			Ports: []corev1.ServicePort{{
	// 				Name:       OAuthServicePortName,
	// 				Port:       OAuthServicePort,
	// 				TargetPort: intstr.FromString(OAuthServicePortName),
	// 				Protocol:   corev1.ProtocolTCP,
	// 			}},
	// 		},
	// 	}

	// 	It("Should create a Service to expose the OAuth proxy", func() {
	// 		By("By checking that the controller has created the Service")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name + "-tls", Namespace: Namespace}
	// 			return cli.Get(ctx, key, service)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorServices(*service, expectedService)).Should(BeTrue())
	// 	})

	// 	It("Should recreate the Service when deleted", func() {
	// 		By("By deleting the Predictor Service")
	// 		Expect(cli.Delete(ctx, service)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the controller has recreated the Service")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name + "-tls", Namespace: Namespace}
	// 			return cli.Get(ctx, key, service)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorServices(*service, expectedService)).Should(BeTrue())
	// 	})

	// 	secret := &corev1.Secret{}

	// 	It("Should create a Secret with the OAuth proxy configuration", func() {
	// 		By("By checking that the controller has created the Secret")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name + "-oauth-config", Namespace: Namespace}
	// 			return cli.Get(ctx, key, secret)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())

	// 		By("By checking that the cookie secret format is correct")
	// 		Expect(len(secret.Data["cookie_secret"])).Should(Equal(32))
	// 	})

	// 	It("Should recreate the Secret when deleted", func() {
	// 		By("By deleting the Predictor Secret")
	// 		Expect(cli.Delete(ctx, secret)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the controller has recreated the Secret")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name + "-oauth-config", Namespace: Namespace}
	// 			return cli.Get(ctx, key, secret)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 	})

	// 	route := &routev1.Route{}
	// 	expectedRoute := routev1.Route{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      Name,
	// 			Namespace: Namespace,
	// 			Labels: map[string]string{
	// 				"Predictor-name": Name,
	// 			},
	// 		},
	// 		Spec: routev1.RouteSpec{
	// 			To: routev1.RouteTargetReference{
	// 				Kind:   "Service",
	// 				Name:   Name + "-tls",
	// 				Weight: pointer.Int32Ptr(100),
	// 			},
	// 			Port: &routev1.RoutePort{
	// 				TargetPort: intstr.FromString(OAuthServicePortName),
	// 			},
	// 			TLS: &routev1.TLSConfig{
	// 				Termination:                   routev1.TLSTerminationReencrypt,
	// 				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	// 			},
	// 			WildcardPolicy: routev1.WildcardPolicyNone,
	// 		},
	// 		Status: routev1.RouteStatus{
	// 			Ingress: []routev1.RouteIngress{},
	// 		},
	// 	}

	// 	It("Should create a Route to expose the traffic externally", func() {
	// 		By("By checking that the controller has created the Route")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, route)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorRoutes(*route, expectedRoute)).Should(BeTrue())
	// 	})

	// 	It("Should recreate the Route when deleted", func() {
	// 		By("By deleting the Predictor Route")
	// 		Expect(cli.Delete(ctx, route)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the controller has recreated the Route")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, route)
	// 		}, timeout, interval).ShouldNot(HaveOccurred())
	// 		Expect(ComparePredictorRoutes(*route, expectedRoute)).Should(BeTrue())
	// 	})

	// 	It("Should reconcile the Route when modified", func() {
	// 		By("By simulating a manual Route modification")
	// 		patch := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"to":{"name":"foo"}}}`))
	// 		Expect(cli.Patch(ctx, route, patch)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the controller has restored the Route spec")
	// 		Eventually(func() (string, error) {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			err := cli.Get(ctx, key, route)
	// 			if err != nil {
	// 				return "", err
	// 			}
	// 			return route.Spec.To.Name, nil
	// 		}, timeout, interval).Should(Equal(Name + "-tls"))
	// 		Expect(ComparePredictorRoutes(*route, expectedRoute)).Should(BeTrue())
	// 	})

	// 	It("Should delete the OAuth proxy objects", func() {
	// 		// Testenv cluster does not implement Kubernetes GC:
	// 		// https://book.kubebuilder.io/reference/envtest.html#testing-considerations
	// 		// To test that the deletion lifecycle works, test the ownership
	// 		// instead of asserting on existence.
	// 		expectedOwnerReference := metav1.OwnerReference{
	// 			APIVersion:         "kubeflow.org/v1",
	// 			Kind:               "Predictor",
	// 			Name:               Name,
	// 			UID:                Predictor.GetObjectMeta().GetUID(),
	// 			Controller:         pointer.BoolPtr(true),
	// 			BlockOwnerDeletion: pointer.BoolPtr(true),
	// 		}

	// 		By("By checking that the Predictor owns the Service Account object")
	// 		Expect(serviceAccount.GetObjectMeta().GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

	// 		By("By checking that the Predictor owns the Service object")
	// 		Expect(service.GetObjectMeta().GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

	// 		By("By checking that the Predictor owns the Secret object")
	// 		Expect(secret.GetObjectMeta().GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

	// 		By("By checking that the Predictor owns the Route object")
	// 		Expect(route.GetObjectMeta().GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

	// 		By("By deleting the recently created Predictor")
	// 		Expect(cli.Delete(ctx, Predictor)).Should(Succeed())
	// 		time.Sleep(interval)

	// 		By("By checking that the Predictor is deleted")
	// 		Eventually(func() error {
	// 			key := types.NamespacedName{Name: Name, Namespace: Namespace}
	// 			return cli.Get(ctx, key, Predictor)
	// 		}, timeout, interval).Should(HaveOccurred())
	// 	})
	// })
})
