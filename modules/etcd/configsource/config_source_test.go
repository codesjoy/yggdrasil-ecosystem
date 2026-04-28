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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/testutil"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

func TestNewConfigSourceDefaults(t *testing.T) {
	src, err := NewConfigSource(Config{Key: "/demo/app"})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	defer func() { _ = src.Close() }()

	s, ok := src.(*configSource)
	if !ok {
		t.Fatalf("source type = %T, want *configSource", src)
	}
	if s.cfg.Mode != ModeBlob {
		t.Fatalf("mode = %q, want %q", s.cfg.Mode, ModeBlob)
	}
	if s.Name() != "/demo/app" {
		t.Fatalf("name = %q, want /demo/app", s.Name())
	}
	if !s.watch {
		t.Fatal("watch = false, want true")
	}
	if s.dialTimeout != 5*time.Second {
		t.Fatalf("dialTimeout = %v, want 5s", s.dialTimeout)
	}
	if s.cfg.Format == nil {
		t.Fatal("default format is nil")
	}
}

func TestNewConfigSourcePrefixDefaultsToKV(t *testing.T) {
	src, err := NewConfigSource(Config{Prefix: "/demo/config", Watch: testutil.BoolPtr(false)})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	defer func() { _ = src.Close() }()

	s := src.(*configSource)
	if s.cfg.Mode != ModeKV {
		t.Fatalf("mode = %q, want %q", s.cfg.Mode, ModeKV)
	}
	if s.Name() != "/demo/config" {
		t.Fatalf("name = %q, want /demo/config", s.Name())
	}
	if s.watch {
		t.Fatal("watch = true, want false")
	}
}

func TestNewConfigSourceErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "empty key and prefix", cfg: Config{}, want: "empty etcd config key/prefix"},
		{
			name: "key and prefix both set",
			cfg:  Config{Key: "a", Prefix: "b"},
			want: "both etcd config key and prefix are set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfigSource(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestConfigSourceReadBlobAndKV(t *testing.T) {
	blob := &configSource{
		cfg: Config{Mode: ModeBlob, Key: "/blob", Format: yaml.Unmarshal},
		client: &testutil.FakeClient{
			GetFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return testutil.GetResp(1, testutil.KV("/blob", "foo: bar")), nil
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	blobData, err := blob.Read()
	if err != nil {
		t.Fatalf("blob Read() error = %v", err)
	}
	var blobMap map[string]any
	if err := blobData.Unmarshal(&blobMap); err != nil {
		t.Fatalf("blob Unmarshal() error = %v", err)
	}
	if blobMap["foo"] != "bar" {
		t.Fatalf("blob data = %#v, want foo=bar", blobMap)
	}

	kvSource := &configSource{
		cfg: Config{Mode: ModeKV, Prefix: "/cfg", Format: yaml.Unmarshal},
		client: &testutil.FakeClient{
			GetFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return testutil.GetResp(
					2,
					testutil.KV("/cfg/a", "1"),
					testutil.KV("/cfg/parent/child", "true"),
					testutil.KV("/cfg/raw", "plain-text"),
				), nil
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	mapData, err := kvSource.Read()
	if err != nil {
		t.Fatalf("kv Read() error = %v", err)
	}
	var out map[string]any
	if err := mapData.Unmarshal(&out); err != nil {
		t.Fatalf("kv Unmarshal() error = %v", err)
	}
	if out["a"].(int) != 1 {
		t.Fatalf("a = %#v, want 1", out["a"])
	}
	parent := out["parent"].(map[string]any)
	if parent["child"] != true {
		t.Fatalf("parent.child = %#v, want true", parent["child"])
	}
	if out["raw"] != "plain-text" {
		t.Fatalf("raw = %#v, want plain-text", out["raw"])
	}
}

func TestConfigSourceReadErrorsAndUnknownMode(t *testing.T) {
	wantErr := errors.New("boom")
	blob := &configSource{
		cfg: Config{Mode: ModeBlob, Key: "/blob"},
		client: &testutil.FakeClient{
			GetFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return nil, wantErr
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	if _, err := blob.Read(); !errors.Is(err, wantErr) {
		t.Fatalf("Read() error = %v, want %v", err, wantErr)
	}

	unknown := &configSource{cfg: Config{Mode: "mystery"}}
	if _, err := unknown.Read(); err == nil ||
		!strings.Contains(err.Error(), "unknown etcd config source mode") {
		t.Fatalf("Read() error = %v, want unknown mode", err)
	}
}

func TestConfigSourceWatch(t *testing.T) {
	watchCh := make(chan clientv3.WatchResponse, 2)
	defer close(watchCh)

	s := &configSource{
		cfg: Config{Mode: ModeBlob, Key: "/blob", Format: yaml.Unmarshal},
		client: &testutil.FakeClient{
			WatchFunc: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh },
			GetFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return testutil.GetResp(1, testutil.KV("/blob", "foo: bar")), nil
			},
		},
		watch:       true,
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}

	ch, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	watchCh <- clientv3.WatchResponse{}

	select {
	case data := <-ch:
		var out map[string]any
		if err := data.Unmarshal(&out); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if out["foo"] != "bar" {
			t.Fatalf("data = %#v, want foo=bar", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch data")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestConfigSourceWatchSkipsReadErrorsAndHonorsFlags(t *testing.T) {
	s := &configSource{watch: false}
	if _, err := s.Watch(); err == nil || !strings.Contains(err.Error(), "not changeable") {
		t.Fatalf("Watch() error = %v, want not changeable", err)
	}

	watchCh := make(chan clientv3.WatchResponse, 2)
	defer close(watchCh)
	s = &configSource{
		cfg: Config{Mode: ModeBlob, Key: "/blob"},
		client: &testutil.FakeClient{
			WatchFunc: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh },
			GetFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return nil, errors.New("read failed")
			},
		},
		watch:       true,
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}

	ch, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	watchCh <- clientv3.WatchResponse{}
	mustNotReceiveSourceData(t, ch)
	watchCh <- clientv3.WatchResponse{Canceled: true}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("watch channel still open after canceled response")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch channel close")
	}
}

func TestConfigSourceHelperMethods(t *testing.T) {
	s := &configSource{cfg: Config{Mode: ModeKV, Prefix: "/prefix"}}
	if got := s.watchKey(); got != "/prefix" {
		t.Fatalf("watchKey() = %q, want /prefix", got)
	}
	if got := len(s.watchOptions()); got != 1 {
		t.Fatalf("watchOptions() length = %d, want 1", got)
	}

	blob := &configSource{cfg: Config{Mode: ModeBlob, Key: "/key"}}
	if got := blob.watchKey(); got != "/key" {
		t.Fatalf("watchKey() = %q, want /key", got)
	}
	if got := blob.watchOptions(); got != nil {
		t.Fatalf("watchOptions() = %#v, want nil", got)
	}

	if got := parseScalarOrDoc([]byte("hello"), nil); got != "hello" {
		t.Fatalf("parseScalarOrDoc(nil) = %#v, want hello", got)
	}
	if got := parseScalarOrDoc([]byte("a: 1"), yaml.Unmarshal); !reflect.DeepEqual(
		got,
		map[string]any{"a": 1},
	) {
		t.Fatalf("parseScalarOrDoc(yaml) = %#v", got)
	}
	parserErr := source.Parser(func([]byte, any) error { return errors.New("bad parser") })
	if got := parseScalarOrDoc([]byte("fallback"), parserErr); got != "fallback" {
		t.Fatalf("parseScalarOrDoc(fallback) = %#v, want fallback", got)
	}

	parts := splitConfigPath("root.{a.b}.tail", ".")
	if !reflect.DeepEqual(parts, []string{"root", "a.b", "tail"}) {
		t.Fatalf("splitConfigPath() = %#v", parts)
	}

	nested := map[string]any{}
	setNested(nested, []string{"a", "b", "c"}, 3)
	setNested(nested, []string{"a", "flat"}, "x")
	if nested["a"].(map[string]any)["b"].(map[string]any)["c"] != 3 {
		t.Fatalf("nested = %#v, want a.b.c=3", nested)
	}
	setNested(map[string]any{"a": "oops"}, []string{"a", "b"}, 1)
	setNested(nested, nil, 1)
}

func mustNotReceiveSourceData(t *testing.T, ch <-chan source.Data) {
	t.Helper()
	select {
	case data := <-ch:
		t.Fatalf("unexpected source data: %#v", data)
	case <-time.After(150 * time.Millisecond):
	}
}
