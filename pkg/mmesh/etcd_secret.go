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
package mmesh

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"github.com/kserve/modelmesh-serving/controllers/modelmesh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// An EtcdSecret represents the etcd configuation for a user namespace
type EtcdSecret struct {
	Log                 logr.Logger
	Name                string
	Namespace           string
	ControllerNamespace string
	EtcdConfig          *EtcdConfig
	Scheme              *runtime.Scheme
}

func (es EtcdSecret) Apply(ctx context.Context, cl client.Client) error {
	s := &corev1.Secret{}
	err := cl.Get(ctx, types.NamespacedName{Name: es.Name, Namespace: es.Namespace}, s)
	notfound := errors.IsNotFound(err)
	if err != nil && !notfound {
		return err
	}
	commonLabelValue := "modelmesh-controller"

	s.ObjectMeta = metav1.ObjectMeta{
		Name:      es.Name,
		Namespace: es.Namespace,
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": commonLabelValue,
		},
	}
	if err = es.addData(s); err != nil {
		return err
	}

	if notfound {
		return cl.Create(ctx, s)
	} else {
		return cl.Update(ctx, s)
	}
}

// Add data to the provided secret
func (es EtcdSecret) addData(s *corev1.Secret) error {
	etcdEndpoints := strings.Split(es.EtcdConfig.Endpoints, ",")
	newEtcdEndpoints := ""

	// For each endpoint, insert controller namespace if not there
	for i := range etcdEndpoints {
		re := regexp.MustCompile(`(?:https?://)?([^\\s.:]+)(:\\d+)?`)
		if se := re.FindStringSubmatchIndex(etcdEndpoints[i]); se != nil {
			etcdEndpoints[i] = fmt.Sprintf("%s.%s%s", etcdEndpoints[i][:se[3]], es.ControllerNamespace, etcdEndpoints[i][se[3]:])
		}
		if i != 0 {
			newEtcdEndpoints += ","
		}
		newEtcdEndpoints += etcdEndpoints[i]
	}

	es.EtcdConfig.Endpoints = newEtcdEndpoints
	es.EtcdConfig.RootPrefix = fmt.Sprintf("%s/mm_ns/%s", es.EtcdConfig.RootPrefix, es.Namespace)

	b, err := json.Marshal(es.EtcdConfig)
	if err != nil {
		return fmt.Errorf("error json-marshalling etcd config: %w", err)
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}
	s.Data[modelmesh.EtcdSecretKey] = b
	return nil
}
