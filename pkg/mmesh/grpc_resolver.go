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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"google.golang.org/grpc/resolver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KUBE_SCHEME = "kube"

type serviceResolver struct {
	name  types.NamespacedName
	port  string
	owner *KubeResolver
	cc    resolver.ClientConn
}

// ResolveNow not needed because we resolve whenever there are any endpoint changes
func (sr *serviceResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (sr *serviceResolver) Close() {
	kr := sr.owner
	kr.lock.Lock()
	defer kr.lock.Unlock()
	list, ok := kr.resolvers[sr.name]
	if ok {
		for i, r := range list {
			if r == sr {
				if last := len(list) - 1; last <= 0 {
					delete(kr.resolvers, sr.name) // remove entry if last one
				} else {
					list[i] = list[last]
					kr.resolvers[sr.name] = list[:last]
				}
				kr.logger.V(1).Info("Removed resolver", "name", sr.name)
				return
			}
		}
	}
	kr.logger.V(1).Info("Close called on unrecognized resolver", "name", sr.name)
}

// InitGrpcResolver should only be called once
func InitGrpcResolver(defaultNamespace string, mgr ctrl.Manager) (*KubeResolver, error) {
	kr := makeKubeResolver(defaultNamespace, mgr.GetClient())
	err := ctrl.NewControllerManagedBy(mgr).For(&corev1.Endpoints{}).Complete(kr)
	if err != nil {
		return nil, err
	}
	resolver.Register(kr)
	kr.logger.Info("Registered KubeResolver with kubebuilder and gRPC")
	return kr, nil
}

func makeKubeResolver(defaultNamespace string, client client.Client) *KubeResolver {
	return &KubeResolver{
		defaultNamespace: defaultNamespace, Client: client,
		resolvers: make(map[types.NamespacedName][]*serviceResolver, 2),
		logger:    ctrl.Log.WithName("KubeResolver"),
	}
}

// KubeResolver is a ResolverBuilder and a Reconciler
type KubeResolver struct {
	client.Client
	defaultNamespace string

	// Map of resolvers in use
	resolvers map[types.NamespacedName][]*serviceResolver
	lock      sync.Mutex

	logger logr.Logger
}

func (kr *KubeResolver) Build(target resolver.Target, cc resolver.ClientConn,
	_ resolver.BuildOptions) (resolver.Resolver, error) {
	if target.URL.Scheme != KUBE_SCHEME {
		return nil, fmt.Errorf("unsupported scheme: %s", target.URL.Scheme)
	}
	host := target.URL.Host
	if host == "" {
		ep := target.URL.Path
		if ep == "" {
			ep = target.URL.Opaque
		}
		host = strings.TrimPrefix(ep, "/")
	}
	parts := strings.Split(host, ":")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return nil, fmt.Errorf("target must be of form: %s:///servicename:port", KUBE_SCHEME)
	}

	nameParts := strings.Split(parts[0], ".")
	nn := types.NamespacedName{Name: nameParts[0], Namespace: kr.defaultNamespace}
	if len(nameParts) >= 2 {
		nn.Namespace = nameParts[1]
	}
	r := &serviceResolver{name: nn, port: parts[1], cc: cc, owner: kr}
	kr.lock.Lock()
	defer kr.lock.Unlock()
	list, ok := kr.resolvers[nn]
	singleton := []*serviceResolver{r}
	if ok {
		kr.resolvers[nn] = append(list, r)
	} else {
		kr.resolvers[nn] = singleton
	}
	log := r.owner.logger.V(1)
	log.Info("Built new resolver", "target", target, "name", nn)
	// Initialize resolver state before returning via a synchronous reconciliation
	_, err := kr.reconcile(context.TODO(), ctrl.Request{NamespacedName: nn}, singleton, log)
	return r, err
}

func (*KubeResolver) Scheme() string {
	return KUBE_SCHEME
}

func (kr *KubeResolver) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := kr.logger.WithName("Reconcile").V(1)
	kr.lock.Lock()
	defer kr.lock.Unlock()
	if list, ok := kr.resolvers[req.NamespacedName]; ok {
		return kr.reconcile(ctx, req, list, log)
	}
	log.Info("Ignoring event for Endpoints with no resolver", "endpoints", req.NamespacedName)
	return ctrl.Result{}, nil
}

// called under lock
func (kr *KubeResolver) reconcile(ctx context.Context, req ctrl.Request,
	list []*serviceResolver, log logr.Logger) (ctrl.Result, error) {
	endpoints := &corev1.Endpoints{}
	err := kr.Get(ctx, req.NamespacedName, endpoints)
	if err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error obtaining endpoints for service %s: %w",
				req.NamespacedName, err)
		} else {
			log.Info("Endpoints not found", "endpoints", req.NamespacedName)
		}
	}
	result := ctrl.Result{}
	var updateError error
	for _, r := range list {
		if errors.IsNotFound(err) {
			r.cc.ReportError(fmt.Errorf("kube Service %s not found", req.NamespacedName))
			continue // not an error from reconciler pov
		}
		var addrs []resolver.Address
		for _, s := range endpoints.Subsets {
			if p := hasTargetPort(&s, r.port); p > 0 {
				for _, ea := range s.Addresses {
					addrs = append(addrs, resolver.Address{Addr: fmt.Sprintf("%s:%d", ea.IP, p)})
				}
			}
		}
		if err := r.cc.UpdateState(resolver.State{Addresses: addrs}); err != nil {
			if err.Error() == "bad resolver state" {
				// This is possible/expected when we are reconfiguring the client
				log.Info("Failed to update resolver due to bad state, requeuing endpoint reconciliation")
				result = ctrl.Result{RequeueAfter: 1 * time.Second}
			} else {
				updateError = fmt.Errorf("error updating state of ClientConn with new addresses: %w", err)
			}
		} else {
			log.Info("Updated resolver state with new endpoints",
				"endpoints", req.NamespacedName, "count", len(addrs))
		}
	}
	return result, updateError
}

// returns int32 port number if port string matches name or number of port in EndpointSubset
func hasTargetPort(s *corev1.EndpointSubset, port string) int32 {
	for _, p := range s.Ports {
		if p.Name == port || strconv.Itoa(int(p.Port)) == port {
			return p.Port
		}
	}
	return -1
}
