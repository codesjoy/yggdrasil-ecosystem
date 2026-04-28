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
	"errors"
	"testing"
	"time"

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
