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

package v1alpha1

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	kservev1alpha "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/constants"
	"github.com/kserve/modelmesh-serving/controllers/autoscaler"
	mmcontstant "github.com/kserve/modelmesh-serving/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-serving-modelmesh-io-v1alpha1-servingruntime,mutating=false,failurePolicy=fail,sideEffects=None,groups=serving.kserve.io,resources=servingruntimes;clusterservingruntimes,verbs=create;update,versions=v1alpha1,name=servingruntime.modelmesh-webhook-server.default,admissionReviewVersions=v1
type ServingRuntimeWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (s *ServingRuntimeWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	var srAnnotations map[string]string
	srReplicas := uint16(math.MaxUint16)
	multiModel := false

	if req.Kind.Kind == "ServingRuntime" {
		servingRuntime := &kservev1alpha.ServingRuntime{}
		err := s.decoder.Decode(req, servingRuntime)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		srAnnotations = servingRuntime.ObjectMeta.Annotations

		if (*servingRuntime).Spec.Replicas != nil {
			srReplicas = uint16(*servingRuntime.Spec.Replicas)
		}

		if (*servingRuntime).Spec.MultiModel != nil {
			multiModel = *servingRuntime.Spec.MultiModel
		}

	} else {
		clusterServingRuntime := &kservev1alpha.ClusterServingRuntime{}
		err := s.decoder.Decode(req, clusterServingRuntime)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		srAnnotations = clusterServingRuntime.ObjectMeta.Annotations

		if (*clusterServingRuntime).Spec.Replicas != nil {
			srReplicas = uint16(*clusterServingRuntime.Spec.Replicas)
		}

		if (*clusterServingRuntime).Spec.MultiModel != nil {
			multiModel = *clusterServingRuntime.Spec.MultiModel
		}
	}

	if !multiModel {
		return admission.Allowed("Not validating ServingRuntime because it is not ModelMesh compatible")
	}

	if err := validateServingRuntimeAutoscaler(srAnnotations); err != nil {
		return admission.Denied(err.Error())
	}

	if err := validateAutoscalerTargetUtilizationPercentage(srAnnotations); err != nil {
		return admission.Denied(err.Error())
	}

	if err := validateAutoScalingReplicas(srAnnotations, srReplicas); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("Passed all validation checks for ServingRuntime")
}

// InjectDecoder injects the decoder.
func (s *ServingRuntimeWebhook) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}

// Validation of servingruntime autoscaler class
func validateServingRuntimeAutoscaler(annotations map[string]string) error {
	value, ok := annotations[constants.AutoscalerClass]
	class := constants.AutoscalerClassType(value)
	if ok {
		for _, item := range constants.AutoscalerAllowedClassList {
			if class == item {
				switch class {
				case constants.AutoscalerClassHPA:
					if metric, ok := annotations[constants.AutoscalerMetrics]; ok {
						return validateHPAMetrics(constants.AutoscalerMetricsType(metric))
					} else {
						return nil
					}
				default:
					return fmt.Errorf("unknown autoscaler class [%s]", class)
				}
			}
		}
		return fmt.Errorf("[%s] is not a supported autoscaler class type.\n", value)
	}

	return nil
}

// Validate of autoscaler targetUtilizationPercentage
func validateAutoscalerTargetUtilizationPercentage(annotations map[string]string) error {
	if value, ok := annotations[constants.TargetUtilizationPercentage]; ok {
		t, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("The target utilization percentage should be a [1-100] integer.")
		} else {
			if t < 1 || t > 100 {
				return fmt.Errorf("The target utilization percentage should be a [1-100] integer.")
			}
		}
	}

	return nil
}

// Validate scaling options
func validateAutoScalingReplicas(annotations map[string]string, srReplicas uint16) error {
	autoscalerClassType := autoscaler.AutoscalerClassNone
	if value, ok := annotations[constants.AutoscalerClass]; ok {
		autoscalerClassType = value
	}

	switch autoscalerClassType {
	case string(constants.AutoscalerClassHPA):
		if srReplicas != math.MaxUint16 {
			return fmt.Errorf("Autoscaler is enabled and also replicas variable set. You can not set both.")
		}
		return validateScalingHPA(annotations)
	default:
		return nil
	}
}

func validateScalingHPA(annotations map[string]string) error {
	metric := constants.AutoScalerMetricsCPU
	if value, ok := annotations[constants.AutoscalerMetrics]; ok {
		metric = constants.AutoscalerMetricsType(value)
	}

	minReplicas := 1
	if value, ok := annotations[mmcontstant.MinScaleAnnotationKey]; ok {
		if valueInt, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("The min replicas should be a integer.")
		} else if valueInt < 1 {
			return fmt.Errorf("The min replicas should be more than 0")
		} else {
			minReplicas = valueInt
		}
	}

	maxReplicas := 1
	if value, ok := annotations[mmcontstant.MaxScaleAnnotationKey]; ok {
		if valueInt, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("The max replicas should be a integer.")
		} else {
			maxReplicas = valueInt
		}
	}

	if minReplicas > maxReplicas {
		return fmt.Errorf("The max replicas should be same or bigger than min replicas.")
	}

	err := validateHPAMetrics(metric)
	if err != nil {
		return err
	}

	if value, ok := annotations[constants.TargetUtilizationPercentage]; ok {
		t, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("The target utilization percentage should be a [1-100] integer.")
		} else if metric == constants.AutoScalerMetricsMemory && t < 1 {
			return fmt.Errorf("The target memory should be greater than 1 MiB")
		}
	}

	return nil
}

// Validate of autoscaler HPA metrics
func validateHPAMetrics(metric constants.AutoscalerMetricsType) error {
	for _, item := range constants.AutoscalerAllowedMetricsList {
		if item == metric {
			return nil
		}
	}
	return fmt.Errorf("[%s] is not a supported metric.\n", metric)

}
