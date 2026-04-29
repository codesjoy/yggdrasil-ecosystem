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

package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config/source"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestInferParser(t *testing.T) {
	tests := []struct {
		key string
	}{
		{key: "config.yaml"},
		{key: "config.yml"},
		{key: "config.json"},
		{key: "config.toml"},
		{key: "config.txt"},
	}

	for _, test := range tests {
		if parser := inferParser(test.key); parser == nil {
			t.Fatalf("inferParser(%q) returned nil parser", test.key)
		}
	}
}

func TestInferKeyFromData(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "yaml key",
			data: map[string]any{"app.yaml": "foo: bar"},
			want: "app.yaml",
		},
		{
			name: "json key",
			data: map[string]any{"app.json": `{"foo":"bar"}`},
			want: "app.json",
		},
		{
			name: "first key",
			data: map[string]any{"foo": "bar"},
			want: "foo",
		},
		{
			name: "empty",
			data: map[string]any{},
			want: "config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := inferKeyFromData(test.data)
			if got != test.want {
				t.Fatalf("inferKeyFromData() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConfigMapSourceReadWatchAndClose(t *testing.T) {
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

	raw, err := NewConfigMapSource(Config{
		Namespace: "default",
		Name:      "app",
		Key:       "config.yaml",
		Watch:     true,
	})
	if err != nil {
		t.Fatalf("NewConfigMapSource() error = %v", err)
	}
	src := raw.(*configSource)
	src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

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
		t.Fatal("timeout waiting for configmap watch channel close")
	}

	if err := src.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSecretSourceReadMergeAndErrorPaths(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"},
		Data: map[string][]byte{
			"password": []byte("secret"),
			"user":     []byte("admin"),
		},
	})

	raw, err := NewSecretSource(Config{
		Namespace:    "default",
		Name:         "secret",
		MergeAllKeys: true,
	})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	src := raw.(*configSource)
	src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

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
	if _, err := src.Watch(); err == nil {
		t.Fatal("Watch() expected error when disabled")
	}

	src.clientForConfig = func(string) (kubernetes.Interface, error) {
		return nil, errors.New("boom")
	}
	if _, err := src.Read(); err == nil {
		t.Fatal("Read() expected kube client error")
	}
}

func TestConfigSourceConstructorsAndIdentity(t *testing.T) {
	if _, err := NewConfigMapSource(Config{}); err == nil {
		t.Fatal("NewConfigMapSource() expected empty name error")
	}
	if _, err := NewSecretSource(Config{}); err == nil {
		t.Fatal("NewSecretSource() expected empty name error")
	}

	rawConfigMap, err := NewConfigMapSource(Config{Name: "app"})
	if err != nil {
		t.Fatalf("NewConfigMapSource() error = %v", err)
	}
	configMapSource := rawConfigMap.(*configSource)
	if configMapSource.Kind() != KindConfigMap {
		t.Fatalf("Kind() = %q, want %q", configMapSource.Kind(), KindConfigMap)
	}
	if configMapSource.Name() != "app" {
		t.Fatalf("Name() = %q, want app", configMapSource.Name())
	}

	rawSecret, err := NewSecretSource(Config{Name: "secret"})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	secretSource := rawSecret.(*configSource)
	if secretSource.Kind() != KindSecret {
		t.Fatalf("Kind() = %q, want %q", secretSource.Kind(), KindSecret)
	}
	if secretSource.Name() != "secret" {
		t.Fatalf("Name() = %q, want secret", secretSource.Name())
	}
}

func TestConfigSourceReadAndPayloadBranches(t *testing.T) {
	t.Run("infer key and parser", func(t *testing.T) {
		client := k8sfake.NewSimpleClientset(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			Data: map[string]string{
				"app.json": `{"foo":"bar"}`,
			},
		})
		raw, err := NewConfigMapSource(Config{
			Namespace: "default",
			Name:      "app",
		})
		if err != nil {
			t.Fatalf("NewConfigMapSource() error = %v", err)
		}
		src := raw.(*configSource)
		src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

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
	})

	t.Run("explicit format is used", func(t *testing.T) {
		called := false
		client := k8sfake.NewSimpleClientset(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			Data: map[string]string{
				"config.txt": `{"foo":"bar"}`,
			},
		})
		raw, err := NewConfigMapSource(Config{
			Namespace: "default",
			Name:      "app",
			Key:       "config.txt",
			Format: func(data []byte, out any) error {
				called = true
				return json.Unmarshal(data, out)
			},
		})
		if err != nil {
			t.Fatalf("NewConfigMapSource() error = %v", err)
		}
		src := raw.(*configSource)
		src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

		data, err := src.Read()
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		var got map[string]any
		if err := data.Unmarshal(&got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if !called {
			t.Fatal("expected explicit parser to be used")
		}
		if got["foo"] != "bar" {
			t.Fatalf("foo = %v, want bar", got["foo"])
		}
	})

	t.Run("missing key returns error", func(t *testing.T) {
		client := k8sfake.NewSimpleClientset(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			Data: map[string]string{
				"other.yaml": "foo: bar",
			},
		})
		raw, err := NewConfigMapSource(Config{
			Namespace: "default",
			Name:      "app",
			Key:       "missing.yaml",
		})
		if err != nil {
			t.Fatalf("NewConfigMapSource() error = %v", err)
		}
		src := raw.(*configSource)
		src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

		if _, err := src.Read(); err == nil {
			t.Fatal("Read() expected missing key error")
		}
	})

	t.Run("payload error paths", func(t *testing.T) {
		src := &configSource{cfg: Config{Key: "config.yaml"}}
		if _, _, err := src.payload(map[string]any{}, nil); err == nil {
			t.Fatal("payload() expected missing key error")
		}
		if _, _, err := src.payload(map[string]any{"config.yaml": 1}, nil); err == nil {
			t.Fatal("payload() expected non-string value error")
		}

		mergeSource := &configSource{cfg: Config{MergeAllKeys: true}}
		payload, content, err := mergeSource.payload(map[string]any{"foo": "bar"}, nil)
		if err != nil {
			t.Fatalf("payload() merge error = %v", err)
		}
		if content == "" {
			t.Fatal("payload() merge content is empty")
		}
		var got map[string]any
		if err := payload.Unmarshal(&got); err != nil {
			t.Fatalf("merge payload Unmarshal() error = %v", err)
		}
		if got["foo"] != "bar" {
			t.Fatalf("foo = %v, want bar", got["foo"])
		}
	})
}

func TestConfigSourceFetchAndDoWatchBranches(t *testing.T) {
	configMapRaw, err := NewConfigMapSource(Config{
		Namespace: "default",
		Name:      "missing-configmap",
	})
	if err != nil {
		t.Fatalf("NewConfigMapSource() error = %v", err)
	}
	configMapSource := configMapRaw.(*configSource)
	configMapSource.clientForConfig = func(string) (kubernetes.Interface, error) {
		return k8sfake.NewSimpleClientset(), nil
	}
	if _, _, err := configMapSource.fetch(); err == nil {
		t.Fatal("fetch() expected missing configmap error")
	}

	secretRaw, err := NewSecretSource(Config{
		Namespace: "default",
		Name:      "missing-secret",
	})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	secretSource := secretRaw.(*configSource)
	secretSource.clientForConfig = func(string) (kubernetes.Interface, error) {
		return k8sfake.NewSimpleClientset(), nil
	}
	if _, _, err := secretSource.fetch(); err == nil {
		t.Fatal("fetch() expected missing secret error")
	}

	secretClient := k8sfake.NewSimpleClientset()
	secretWatch := watch.NewFake()
	secretClient.PrependWatchReactor(
		"secrets",
		func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, secretWatch, nil
		},
	)
	watchableRaw, err := NewSecretSource(Config{
		Namespace: "default",
		Name:      "secret",
		Watch:     true,
	})
	if err != nil {
		t.Fatalf("NewSecretSource() error = %v", err)
	}
	watchable := watchableRaw.(*configSource)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := watchable.doWatch(ctx, secretClient)
	if err != nil {
		t.Fatalf("doWatch() error = %v", err)
	}

	go secretWatch.Add(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "default"}},
	)
	select {
	case event := <-ch:
		if event.Type != watch.Added {
			t.Fatalf("event.Type = %v, want %v", event.Type, watch.Added)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for secret watch event")
	}
	secretWatch.Stop()
}

func TestConfigSourceWatchAddedDeduplicatesAndClientErrors(t *testing.T) {
	t.Run("client init error", func(t *testing.T) {
		raw, err := NewConfigMapSource(Config{
			Name:  "app",
			Watch: true,
		})
		if err != nil {
			t.Fatalf("NewConfigMapSource() error = %v", err)
		}
		src := raw.(*configSource)
		src.clientForConfig = func(string) (kubernetes.Interface, error) {
			return nil, errors.New("boom")
		}

		if _, err := src.Watch(); err == nil {
			t.Fatal("Watch() expected kube client error")
		}
	})

	t.Run("added event deduplicates and close is idempotent", func(t *testing.T) {
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

		raw, err := NewConfigMapSource(Config{
			Namespace: "default",
			Name:      "app",
			Key:       "config.yaml",
			Watch:     true,
		})
		if err != nil {
			t.Fatalf("NewConfigMapSource() error = %v", err)
		}
		src := raw.(*configSource)
		src.clientForConfig = func(string) (kubernetes.Interface, error) { return client, nil }

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
		fw.Add(cm)

		select {
		case update := <-ch:
			var got map[string]any
			if err := update.Unmarshal(&got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got["foo"] != "baz" {
				t.Fatalf("foo = %v, want baz", got["foo"])
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for added event update")
		}

		fw.Modify(cm)
		select {
		case duplicate := <-ch:
			var got map[string]any
			_ = duplicate.Unmarshal(&got)
			t.Fatalf("received duplicate update: %#v", got)
		case <-time.After(200 * time.Millisecond):
		}

		if err := src.Close(); err != nil {
			t.Fatalf("Close() first call error = %v", err)
		}
		if err := src.Close(); err != nil {
			t.Fatalf("Close() second call error = %v", err)
		}
		fw.Stop()
	})
}

func TestExplicitFormatParserCanPopulateData(t *testing.T) {
	parser := source.Parser(func(data []byte, out any) error {
		target, ok := out.(*map[string]any)
		if !ok {
			return fmt.Errorf("unexpected target type %T", out)
		}
		*target = map[string]any{"raw": string(data)}
		return nil
	})

	payload := source.NewBytesData([]byte("hello"), parser)
	var got map[string]any
	if err := payload.Unmarshal(&got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got["raw"] != "hello" {
		t.Fatalf("raw = %v, want hello", got["raw"])
	}
}
