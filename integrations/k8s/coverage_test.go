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

package k8s //nolint:staticcheck // Deprecated corev1 endpoints types are exercised intentionally for compatibility coverage.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type stateRecorder struct {
	ch chan resolver.State
}

func (s *stateRecorder) UpdateState(state resolver.State) {
	select {
	case s.ch <- state:
	default:
	}
}

func withKubeProvider(
	t *testing.T,
	provider func(string) (kubernetes.Interface, error),
) {
	t.Helper()
	old := kubeClientProvider
	kubeClientProvider = provider
	t.Cleanup(func() {
		kubeClientProvider = old
		ResetKubeClient()
	})
}

func TestConfigSource_ReadWatchAndClose(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Data: map[string]string{
			"config.yaml": "foo: bar",
		},
	})
	fw := watch.NewFake()
	client.PrependWatchReactor(
		"configmaps",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, fw, nil
		},
	)
	withKubeProvider(t, func(string) (kubernetes.Interface, error) {
		return client, nil
	})

	src, err := NewConfigMapSource(ConfigSourceConfig{
		Namespace: "default",
		Name:      "app",
		Key:       "config.yaml",
		Watch:     true,
	})
	if err != nil {
		t.Fatalf("NewConfigMapSource() error = %v", err)
	}

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	var got map[string]any
	if err := data.Unmarshal(&got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got["foo"] != "bar" {
		t.Fatalf("foo = %v, want bar", got["foo"])
	}

	ch, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	cm, err := client.CoreV1().
		ConfigMaps("default").
		Get(context.Background(), "app", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	cm.Data["config.yaml"] = "foo: baz"
	if _, err := client.CoreV1().ConfigMaps("default").Update(
		context.Background(),
		cm,
		metav1.UpdateOptions{},
	); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	fw.Modify(cm)

	select {
	case update := <-ch:
		var updated map[string]any
		if err := update.Unmarshal(&updated); err != nil {
			t.Fatalf("watch Unmarshal() error = %v", err)
		}
		if updated["foo"] != "baz" {
			t.Fatalf("foo = %v, want baz", updated["foo"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for configmap watch update")
	}

	fw.Delete(cm)
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("watch channel should be closed after delete")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watch channel close")
	}

	if err := src.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestConfigSource_SecretAndErrorPaths(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
		Data: map[string][]byte{
			"password": []byte("secret"),
			"user":     []byte("admin"),
		},
	})
	withKubeProvider(t, func(string) (kubernetes.Interface, error) { return client, nil })

	src, err := NewSecretSource(ConfigSourceConfig{
		Namespace:   "default",
		Name:        "secret",
		MergeAllKey: true,
	})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	var got map[string]any
	if err := data.Unmarshal(&got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got["password"] != "secret" || got["user"] != "admin" {
		t.Fatalf("unexpected secret data: %#v", got)
	}
	if src.Changeable() {
		t.Fatal("Changeable() = true, want false")
	}
	if _, err := src.Watch(); err == nil {
		t.Fatal("Watch() expected error when disabled")
	}

	withKubeProvider(t, func(string) (kubernetes.Interface, error) {
		return nil, errors.New("boom")
	})
	if _, err := src.Read(); err == nil {
		t.Fatal("Read() expected kube client error")
	}
}

func TestResolver_WatchesEndpoints(t *testing.T) {
	//nolint:staticcheck // SA1019: corev1.Endpoints remains intentionally covered for older clusters.
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
		//nolint:staticcheck // SA1019: corev1.EndpointSubset remains intentionally covered for older clusters.
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}},
				Ports:     []corev1.EndpointPort{{Name: "grpc", Port: 8080}},
			},
		},
	}
	client := k8sfake.NewSimpleClientset(endpoints)
	fw := watch.NewFake()
	client.PrependWatchReactor(
		"endpoints",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, fw, nil
		},
	)
	withKubeProvider(t, func(string) (kubernetes.Interface, error) {
		return client, nil
	})

	r, err := NewResolver("default", ResolverConfig{
		Namespace: "default",
		Protocol:  "grpc",
		Backoff: backoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	rec := &stateRecorder{ch: make(chan resolver.State, 4)}
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

	if err := r.DelWatch("svc", rec); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}
	fw.Modify(updated)
	select {
	case <-rec.ch:
		t.Fatal("unexpected resolver update after DelWatch")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestResolver_WatchesEndpointSlices(t *testing.T) {
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
	withKubeProvider(t, func(string) (kubernetes.Interface, error) {
		return client, nil
	})

	r, err := NewResolver("default", ResolverConfig{
		Namespace: "default",
		Mode:      string(modeEndpointSlice),
		Protocol:  "grpc",
		Backoff: backoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	rec := &stateRecorder{ch: make(chan resolver.State, 4)}
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

func TestLoadResolverConfigAndKubeHelpers(t *testing.T) {
	t.Setenv("KUBERNETES_NAMESPACE", "env-ns")

	setConfig := func(t *testing.T, key string, value any) {
		t.Helper()
		if err := config.Set(key, value); err != nil {
			t.Fatalf("config.Set(%q) error = %v", key, err)
		}
	}

	cfgBase := config.Join(config.KeyBase, "resolver", "k8s-test", "config")
	setConfig(t, config.Join(cfgBase, "protocol"), "http")
	setConfig(t, config.Join(cfgBase, "mode"), "")
	setConfig(t, config.Join(cfgBase, "backoff", "baseDelay"), 0)
	setConfig(t, config.Join(cfgBase, "backoff", "multiplier"), 0)
	setConfig(t, config.Join(cfgBase, "backoff", "jitter"), 0)
	setConfig(t, config.Join(cfgBase, "backoff", "maxDelay"), 0)

	cfg := LoadResolverConfig("k8s-test")
	if cfg.Namespace != "env-ns" {
		t.Fatalf("Namespace = %q, want env-ns", cfg.Namespace)
	}
	if cfg.Mode != string(modeEndpointSlice) {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, modeEndpointSlice)
	}
	if cfg.Protocol != "http" {
		t.Fatalf("Protocol = %q, want http", cfg.Protocol)
	}
	if cfg.Backoff.BaseDelay == 0 || cfg.Backoff.Multiplier == 0 ||
		cfg.Backoff.Jitter == 0 || cfg.Backoff.MaxDelay == 0 {
		t.Fatalf("expected defaulted backoff config, got %#v", cfg.Backoff)
	}

	ResetKubeClient()
	if _, err := GetKubeClient("/definitely/missing-kubeconfig"); err == nil {
		t.Fatal("GetKubeClient() expected error for missing kubeconfig")
	}
	ResetKubeClient()

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	if IsInCluster() {
		t.Fatal("IsInCluster() = true, want false")
	}
}

func TestBackoffBounds(t *testing.T) {
	bo := newBackoff(backoffConfig{
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

func TestResolverBuilderAndWatchLoopCancel(t *testing.T) {
	setConfig := func(t *testing.T, key string, value any) {
		t.Helper()
		if err := config.Set(key, value); err != nil {
			t.Fatalf("config.Set(%q) error = %v", key, err)
		}
	}

	setConfig(t, config.Join(config.KeyBase, "resolver", "k8s-builder", "type"), "kubernetes")
	got, err := resolver.Get("k8s-builder")
	if err != nil {
		t.Fatalf("resolver.Get() error = %v", err)
	}
	if got.Type() != "kubernetes" {
		t.Fatalf("resolver builder Type() = %q, want kubernetes", got.Type())
	}

	withKubeProvider(t, func(string) (kubernetes.Interface, error) {
		return nil, errors.New("boom")
	})
	r, err := NewResolver("default", ResolverConfig{
		Backoff: backoffConfig{
			BaseDelay:  time.Millisecond,
			Multiplier: 1,
			MaxDelay:   2 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
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
