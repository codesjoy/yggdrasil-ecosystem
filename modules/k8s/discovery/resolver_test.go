// Copyright 2022 The codesjoy Authors.
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

package discovery //nolint:staticcheck // Deprecated corev1 endpoint types are covered intentionally for compatibility.

import (
	"context"
	"errors"
	"testing"
	"time"

	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type stateRecorder struct {
	ch chan yresolver.State
}

func (s *stateRecorder) UpdateState(state yresolver.State) {
	select {
	case s.ch <- state:
	default:
	}
}

func TestNormalizeConfig(t *testing.T) {
	t.Setenv("KUBERNETES_NAMESPACE", "env-ns")

	cfg := NormalizeConfig(ResolverConfig{})
	if cfg.Namespace != "env-ns" {
		t.Fatalf("Namespace = %q, want env-ns", cfg.Namespace)
	}
	if cfg.Mode != string(modeEndpointSlice) {
		t.Fatalf("Mode = %q, want endpointslice", cfg.Mode)
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("Protocol = %q, want grpc", cfg.Protocol)
	}
	if cfg.Backoff.BaseDelay == 0 || cfg.Backoff.Multiplier == 0 ||
		cfg.Backoff.Jitter == 0 || cfg.Backoff.MaxDelay == 0 {
		t.Fatalf("expected defaulted backoff config, got %#v", cfg.Backoff)
	}
}

func TestBackoffBounds(t *testing.T) {
	bo := newBackoff(BackoffConfig{
		BaseDelay:  time.Millisecond,
		Multiplier: 2,
		Jitter:     0,
		MaxDelay:   3 * time.Millisecond,
	})
	if got := bo.Backoff(0); got != time.Millisecond {
		t.Fatalf("Backoff(0) = %v, want 1ms", got)
	}
	if got := bo.Backoff(3); got != 3*time.Millisecond {
		t.Fatalf("Backoff(3) = %v, want capped 3ms", got)
	}
}

func TestResolverProviderUsesLoader(t *testing.T) {
	provider := ResolverProvider(func(name string) ResolverConfig {
		if name != "k8s" {
			t.Fatalf("loader name = %q, want k8s", name)
		}
		return ResolverConfig{Namespace: "default"}
	})

	resolver, err := provider.New("k8s")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if resolver.Type() != "kubernetes" {
		t.Fatalf("resolver.Type() = %q, want kubernetes", resolver.Type())
	}
}

func TestEndpointsToState(t *testing.T) {
	r := &Resolver{cfg: ResolverConfig{Protocol: "grpc"}}
	//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"},
		//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{
				{IP: "10.0.0.1", TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-1"}},
				{IP: "10.0.0.2"},
			},
			Ports: []corev1.EndpointPort{{Name: "grpc", Port: 9090}},
		}},
	}

	state := r.endpointsToState(endpoints)
	if state.GetAttributes()["service"] != "test-svc" {
		t.Fatalf("service attribute = %v, want test-svc", state.GetAttributes()["service"])
	}
	items := state.GetEndpoints()
	if len(items) != 2 {
		t.Fatalf("endpoints len = %d, want 2", len(items))
	}
	if items[0].GetAddress() != "10.0.0.1:9090" {
		t.Fatalf("first endpoint address = %s, want 10.0.0.1:9090", items[0].GetAddress())
	}
}

func TestEndpointSlicesToState(t *testing.T) {
	r := &Resolver{cfg: ResolverConfig{Protocol: "http"}}
	portName := "http"
	portNum := int32(8080)
	addr1 := "10.0.0.3"
	addr2 := "10.0.0.4"
	slices := []discoveryv1.EndpointSlice{{
		ObjectMeta:  metav1.ObjectMeta{Name: "test-svc-abc", Namespace: "default"},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       []discoveryv1.EndpointPort{{Name: &portName, Port: &portNum}},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{addr1, addr2},
			TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-2"},
			NodeName:  strPtr("node-1"),
			Zone:      strPtr("zone-a"),
		}},
	}}

	state := r.endpointSlicesToState(slices)
	items := state.GetEndpoints()
	if len(items) != 2 {
		t.Fatalf("endpoints len = %d, want 2", len(items))
	}
	if items[0].GetAddress() != "10.0.0.3:8080" {
		t.Fatalf("first endpoint address = %s, want 10.0.0.3:8080", items[0].GetAddress())
	}
	if items[0].GetAttributes()["nodeName"] != "node-1" {
		t.Fatalf("nodeName = %v, want node-1", items[0].GetAttributes()["nodeName"])
	}
	if items[0].GetAttributes()["zone"] != "zone-a" {
		t.Fatalf("zone = %v, want zone-a", items[0].GetAttributes()["zone"])
	}
}

func TestResolverTypeAndSelectPort(t *testing.T) {
	r := &Resolver{cfg: ResolverConfig{PortName: "grpc", Port: 9090}}
	if got := r.Type(); got != "kubernetes" {
		t.Fatalf("Type() = %q, want kubernetes", got)
	}
	if port := r.selectPort(nil); port != nil {
		t.Fatalf("selectPort(nil) = %#v, want nil", port)
	}
	ports := []corev1.EndpointPort{
		{Name: "http", Port: 8080},
		{Name: "grpc", Port: 9090},
	}
	if port := r.selectPort(ports); port == nil || port.Port != 9090 {
		t.Fatalf("selectPort() by name = %#v, want 9090", port)
	}
	r.cfg.PortName = ""
	if port := r.selectPort(ports); port == nil || port.Port != 9090 {
		t.Fatalf("selectPort() by number = %#v, want 9090", port)
	}
	r.cfg.Port = 0
	if port := r.selectPort(ports); port == nil || port.Port != 8080 {
		t.Fatalf("selectPort() fallback = %#v, want first port 8080", port)
	}
}

func TestResolverWatchesEndpoints(t *testing.T) {
	//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
		//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}},
			Ports:     []corev1.EndpointPort{{Name: "grpc", Port: 8080}},
		}},
	}
	client := k8sfake.NewSimpleClientset(endpoints)
	fw := watch.NewFake()
	client.PrependWatchReactor(
		"endpoints",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, fw, nil
		},
	)

	r, err := NewResolver("default", ResolverConfig{
		Namespace: "default",
		Protocol:  "grpc",
		Mode:      "endpoints",
		Backoff: BackoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	r.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

	rec := &stateRecorder{ch: make(chan yresolver.State, 4)}
	if err := r.AddWatch("svc", rec); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}

	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 1 || st.GetEndpoints()[0].GetAddress() != "10.0.0.1:8080" {
			t.Fatalf("unexpected initial endpoints: %#v", st.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial resolver state")
	}

	updated := endpoints.DeepCopy()
	updated.Subsets[0].Addresses = append(
		updated.Subsets[0].Addresses,
		corev1.EndpointAddress{IP: "10.0.0.2"},
	)
	if _, err := client.CoreV1().Endpoints("default").Update(
		context.Background(),
		updated,
		metav1.UpdateOptions{},
	); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	fw.Modify(updated)

	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 2 {
			t.Fatalf("endpoints len = %d, want 2", len(st.GetEndpoints()))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for modified resolver state")
	}

	fw.Delete(updated)
	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 0 {
			t.Fatalf("expected empty state after delete, got %#v", st.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for deleted resolver state")
	}
}

func TestResolverWatchesEndpointSlices(t *testing.T) {
	portName := "grpc"
	port := int32(9090)
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-1",
			Namespace: "default",
			Labels: map[string]string{
				"kubernetes.io/service-name": "svc",
			},
		},
		Ports: []discoveryv1.EndpointPort{{Name: &portName, Port: &port}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.1.1"}},
		},
	}
	client := k8sfake.NewSimpleClientset(slice)
	fw := watch.NewFake()
	client.PrependWatchReactor(
		"endpointslices",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, fw, nil
		},
	)

	r, err := NewResolver("default", ResolverConfig{
		Namespace: "default",
		Mode:      string(modeEndpointSlice),
		Protocol:  "grpc",
		Backoff: BackoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	r.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

	rec := &stateRecorder{ch: make(chan yresolver.State, 4)}
	if err := r.AddWatch("svc", rec); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}

	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 1 || st.GetEndpoints()[0].GetAddress() != "10.0.1.1:9090" {
			t.Fatalf("unexpected initial endpoints: %#v", st.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial endpointslice state")
	}

	updated := slice.DeepCopy()
	updated.Endpoints = append(
		updated.Endpoints,
		discoveryv1.Endpoint{Addresses: []string{"10.0.1.2"}},
	)
	if _, err := client.DiscoveryV1().EndpointSlices("default").Update(
		context.Background(),
		updated,
		metav1.UpdateOptions{},
	); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	fw.Modify(updated)

	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 2 {
			t.Fatalf("endpoints len = %d, want 2", len(st.GetEndpoints()))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for modified endpointslice state")
	}

	fw.Delete(updated)
	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 0 {
			t.Fatalf("expected empty state after endpointslice delete, got %#v", st.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for deleted endpointslice state")
	}
}

func TestResolverFallsBackFromEndpointSliceToEndpoints(t *testing.T) {
	//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
		//nolint:staticcheck // Intentional coverage for deprecated Endpoints compatibility path.
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{IP: "10.0.2.1"}},
			Ports:     []corev1.EndpointPort{{Name: "grpc", Port: 7070}},
		}},
	}
	client := k8sfake.NewSimpleClientset(endpoints)
	endpointWatch := watch.NewFake()
	client.PrependWatchReactor(
		"endpointslices",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, nil, errors.New("endpoint slices unavailable")
		},
	)
	client.PrependWatchReactor(
		"endpoints",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, endpointWatch, nil
		},
	)

	r, err := NewResolver("default", ResolverConfig{
		Namespace: "default",
		Mode:      string(modeEndpointSlice),
		Protocol:  "grpc",
		Backoff: BackoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	r.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

	rec := &stateRecorder{ch: make(chan yresolver.State, 2)}
	if err := r.AddWatch("svc", rec); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}

	select {
	case st := <-rec.ch:
		if len(st.GetEndpoints()) != 1 || st.GetEndpoints()[0].GetAddress() != "10.0.2.1:7070" {
			t.Fatalf("unexpected fallback endpoints: %#v", st.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for endpoints fallback state")
	}
}

func TestResolverWatchLoopStopsAfterContextCancellation(t *testing.T) {
	r, err := NewResolver("default", ResolverConfig{
		Backoff: BackoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   2 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	r.clientForConfig = func(string) (kubernetes.Interface, error) {
		return nil, errors.New("boom")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.watchLoop(ctx, "svc")
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not stop after context cancellation")
	}
}

func strPtr(s string) *string {
	return &s
}
