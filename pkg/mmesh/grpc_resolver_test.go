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
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient struct {
	client.Client
	t       *testing.T
	getfunc func(context.Context, client.ObjectKey, *v1.Endpoints, []client.GetOption) error
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	assert.NotNil(m.t, ctx)
	assert.IsType(m.t, &v1.Endpoints{}, obj)
	assert.Equal(m.t, "modelmesh-serving", key.Name)
	assert.Equal(m.t, "namespace", key.Namespace)
	return m.getfunc(ctx, key, obj.(*v1.Endpoints), opts)
}

type mockCC struct {
	t          *testing.T
	updatefunc func(state resolver.State) error
}

func (m mockCC) UpdateState(state resolver.State) error {
	assert.NotNil(m.t, state)
	fmt.Printf("updatestate called: %v\n", state)
	return m.updatefunc(state)
}

// Test for basic functionality
func Test_KubeResolver_AddRemove(t *testing.T) {
	mClient := mockClient{t: t}
	mClient.getfunc = func(ctx context.Context, key client.ObjectKey, ep *v1.Endpoints, opts []client.GetOption) error {
		ep.Subsets = []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{IP: "1.2.3.4"},
				},
				NotReadyAddresses: []v1.EndpointAddress{},
				Ports: []v1.EndpointPort{
					{Name: "grpc", Port: 8033},
					{Name: "prometheus", Port: 2112},
				},
			},
		}
		return nil
	}

	kr := makeKubeResolver("namespace", mClient)

	mCC := mockCC{}
	updateStateCalled := false
	mCC.updatefunc = func(state resolver.State) error {
		updateStateCalled = true
		assert.Len(t, state.Addresses, 1)
		assert.Equal(t, "1.2.3.4:8033", state.Addresses[0].Addr)
		return nil
	}

	fmt.Println("Build r1")
	r1, err := kr.Build(resolver.Target{URL: url.URL{Scheme: "kube", Host: "modelmesh-serving:8033"}}, mCC, resolver.BuildOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, r1)
	assert.True(t, updateStateCalled)
	updateStateCalled = false

	reconcile(t, kr)
	assert.True(t, updateStateCalled)
	updateStateCalled = false

	mCC2 := mockCC{}
	updateState2Called := false
	mCC2.updatefunc = func(state resolver.State) error {
		updateState2Called = true
		assert.Len(t, state.Addresses, 1)
		assert.Equal(t, "1.2.3.4:8033", state.Addresses[0].Addr)
		return nil
	}

	fmt.Println("Build r2")
	r2, err := kr.Build(resolver.Target{URL: url.URL{Scheme: "kube", Host: "modelmesh-serving:8033"}}, mCC2, resolver.BuildOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, r2)
	assert.False(t, updateStateCalled)
	assert.True(t, updateState2Called)
	updateState2Called = false

	reconcile(t, kr)
	assert.True(t, updateStateCalled)
	assert.True(t, updateState2Called)
	updateStateCalled, updateState2Called = false, false

	fmt.Println("Close r1")
	r1.Close()

	reconcile(t, kr)
	assert.False(t, updateStateCalled)
	assert.True(t, updateState2Called)
	updateStateCalled, updateState2Called = false, false

	fmt.Println("Close r2")
	r2.Close()

	reconcile(t, kr)
	assert.False(t, updateStateCalled)
	assert.False(t, updateState2Called)
	updateStateCalled, updateState2Called = false, false
}

func reconcile(t *testing.T, kr *KubeResolver) {
	fmt.Println("Reconcile")
	_, err := kr.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: "namespace", Name: "modelmesh-serving",
	}})
	assert.Nil(t, err)
}

// Unused mock funcs

func (m mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Status() client.StatusWriter {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) Scheme() *runtime.Scheme {
	m.t.Error("should not be called")
	return nil
}

func (m mockClient) RESTMapper() meta.RESTMapper {
	m.t.Error("should not be called")
	return nil
}

func (m mockCC) ReportError(err error) {
	m.t.Error("should not be called")
}

func (m mockCC) NewAddress(addresses []resolver.Address) {
	m.t.Error("should not be called")
}

func (m mockCC) NewServiceConfig(serviceConfig string) {
	m.t.Error("should not be called")
}

func (m mockCC) ParseServiceConfig(serviceConfigJSON string) *serviceconfig.ParseResult {
	m.t.Error("should not be called")
	return nil
}
