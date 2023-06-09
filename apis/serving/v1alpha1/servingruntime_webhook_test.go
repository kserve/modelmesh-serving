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
	"math"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kservev1alpha "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/constants"
	mmcontstant "github.com/kserve/modelmesh-serving/pkg/constants"
)

func makeTestRawServingRuntime() kservev1alpha.ServingRuntime {
	servingRuntime := kservev1alpha.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Annotations: map[string]string{
				"serving.kserve.io/autoscalerClass":             "hpa",
				"serving.kserve.io/metrics":                     "cpu",
				"serving.kserve.io/targetUtilizationPercentage": "75",
				"serving.kserve.io/min-scale":                   "2",
				"serving.kserve.io/max-scale":                   "3",
			},
		},
	}

	return servingRuntime
}

func TestValidAutoscalerTypeAndHPAMetrics(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	g.Expect(validateServingRuntimeAutoscaler(sr.Annotations)).Should(gomega.Succeed())
}
func TestInvalidAutoscalerClassType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[constants.AutoscalerClass] = "test"
	g.Expect(validateServingRuntimeAutoscaler(sr.Annotations)).ShouldNot(gomega.Succeed())
}

func TestInvalidAutoscalerTargetUtilizationPercentageLowValue(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[constants.TargetUtilizationPercentage] = "-1"
	g.Expect(validateAutoscalerTargetUtilizationPercentage(sr.Annotations)).ShouldNot(gomega.Succeed())
}

func TestInvalidAutoscalerTargetUtilizationPercentageHighValue(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[constants.TargetUtilizationPercentage] = "101"
	g.Expect(validateAutoscalerTargetUtilizationPercentage(sr.Annotations)).ShouldNot(gomega.Succeed())
}

func TestInvalidAutoscalerLowMinReplicas(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[mmcontstant.MinScaleAnnotationKey] = "0"
	g.Expect(validateScalingHPA(sr.Annotations)).ShouldNot(gomega.Succeed())
}

func TestInvalidAutoscalerMaxReplicasMustBiggerThanMixReplicas(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[mmcontstant.MinScaleAnnotationKey] = "4"
	sr.ObjectMeta.Annotations[mmcontstant.MaxScaleAnnotationKey] = "3"
	g.Expect(validateAutoScalingReplicas(sr.Annotations, math.MaxUint16)).ShouldNot(gomega.Succeed())
}
func TestDuplicatedReplicas(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	g.Expect(validateAutoScalingReplicas(sr.Annotations, 1)).ShouldNot(gomega.Succeed())
}

func TestValidAutoscalerMetricsType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[constants.AutoscalerMetrics] = "memory"
	g.Expect(validateHPAMetrics(constants.AutoscalerMetricsType("memory"))).Should(gomega.Succeed())
}

func TestInvalidAutoscalerMetricsType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	sr := makeTestRawServingRuntime()
	sr.ObjectMeta.Annotations[constants.AutoscalerMetrics] = "conccurrency"
	g.Expect(validateHPAMetrics(constants.AutoscalerMetricsType("conccurrency"))).ShouldNot(gomega.Succeed())
}
