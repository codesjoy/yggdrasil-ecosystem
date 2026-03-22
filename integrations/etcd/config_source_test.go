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

package etcd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

func TestNewClientDefaults(t *testing.T) {
	cli, err := newClient(ClientConfig{})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	defer func() { _ = cli.Close() }()

	if got := cli.Endpoints(); len(got) != 1 || got[0] != "127.0.0.1:2379" {
		t.Fatalf("endpoints = %#v, want [127.0.0.1:2379]", got)
	}
}

func TestNewConfigSourceDefaults(t *testing.T) {
	src, err := NewConfigSource(ConfigSourceConfig{Key: "/demo/app"})
	if err != nil {
		t.Fatalf("NewConfigSource: %v", err)
	}
	defer func() { _ = src.Close() }()

	s, ok := src.(*configSource)
	if !ok {
		t.Fatalf("source type = %T, want *configSource", src)
	}
	if s.cfg.Mode != ConfigSourceModeBlob {
		t.Fatalf("mode = %q, want %q", s.cfg.Mode, ConfigSourceModeBlob)
	}
	if s.Name() != "/demo/app" {
		t.Fatalf("name = %q, want /demo/app", s.Name())
	}
	if !s.Changeable() {
		t.Fatal("Changeable() = false, want true")
	}
	if s.dialTimeout != 5*time.Second {
		t.Fatalf("dialTimeout = %v, want 5s", s.dialTimeout)
	}
	if s.cfg.Format == nil {
		t.Fatal("default format is nil")
	}
}

func TestNewConfigSourcePrefixDefaultsToKV(t *testing.T) {
	src, err := NewConfigSource(
		ConfigSourceConfig{Prefix: "/demo/config", Watch: boolPtrValue(false)},
	)
	if err != nil {
		t.Fatalf("NewConfigSource: %v", err)
	}
	defer func() { _ = src.Close() }()

	s := src.(*configSource)
	if s.cfg.Mode != ConfigSourceModeKV {
		t.Fatalf("mode = %q, want %q", s.cfg.Mode, ConfigSourceModeKV)
	}
	if s.Name() != "/demo/config" {
		t.Fatalf("name = %q, want /demo/config", s.Name())
	}
	if s.Changeable() {
		t.Fatal("Changeable() = true, want false")
	}
}

func TestNewConfigSourceErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  ConfigSourceConfig
		want string
	}{
		{
			name: "empty key and prefix",
			cfg:  ConfigSourceConfig{},
			want: "empty etcd config key/prefix",
		},
		{
			name: "key and prefix both set",
			cfg:  ConfigSourceConfig{Key: "a", Prefix: "b"},
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
		cfg: ConfigSourceConfig{Mode: ConfigSourceModeBlob, Key: "/blob", Format: yaml.Unmarshal},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return getResp(1, kv("/blob", "foo: bar")), nil
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	blobData, err := blob.Read()
	if err != nil {
		t.Fatalf("blob Read: %v", err)
	}
	var blobMap map[string]any
	if err := blobData.Unmarshal(&blobMap); err != nil {
		t.Fatalf("blob Unmarshal: %v", err)
	}
	if blobMap["foo"] != "bar" {
		t.Fatalf("blob data = %#v, want foo=bar", blobMap)
	}

	kvSource := &configSource{
		cfg: ConfigSourceConfig{
			Mode:   ConfigSourceModeKV,
			Prefix: "/cfg",
			Format: yaml.Unmarshal,
		},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return getResp(2,
					kv("/cfg/a", "1"),
					kv("/cfg/parent/child", "true"),
					kv("/cfg/raw", "plain-text"),
				), nil
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	mapData, err := kvSource.Read()
	if err != nil {
		t.Fatalf("kv Read: %v", err)
	}
	var out map[string]any
	if err := mapData.Unmarshal(&out); err != nil {
		t.Fatalf("kv Unmarshal: %v", err)
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
		cfg: ConfigSourceConfig{Mode: ConfigSourceModeBlob, Key: "/blob"},
		client: &fakeEtcdClient{
			getFunc: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				return nil, wantErr
			},
		},
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	if _, err := blob.Read(); !errors.Is(err, wantErr) {
		t.Fatalf("Read error = %v, want %v", err, wantErr)
	}

	unknown := &configSource{cfg: ConfigSourceConfig{Mode: "mystery"}}
	if _, err := unknown.Read(); err == nil ||
		!strings.Contains(err.Error(), "unknown etcd config source mode") {
		t.Fatalf("Read error = %v, want unknown mode", err)
	}
}

func TestConfigSourceWatch(t *testing.T) {
	watchCh := make(chan clientv3.WatchResponse, 2)
	defer close(watchCh)
	client := &fakeEtcdClient{}
	client.watchFunc = func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh }
	client.getFunc = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return getResp(1, kv("/blob", "foo: bar")), nil
	}

	s := &configSource{
		cfg: ConfigSourceConfig{
			Mode:   ConfigSourceModeBlob,
			Key:    "/blob",
			Format: yaml.Unmarshal,
		},
		client:      client,
		watch:       true,
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}

	ch, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	watchCh <- clientv3.WatchResponse{}

	select {
	case data := <-ch:
		var out map[string]any
		if err := data.Unmarshal(&out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if out["foo"] != "bar" {
			t.Fatalf("data = %#v, want foo=bar", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch data")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConfigSourceWatchSkipsReadErrorsAndHonorsFlags(t *testing.T) {
	s := &configSource{watch: false}
	if _, err := s.Watch(); err == nil || !strings.Contains(err.Error(), "not changeable") {
		t.Fatalf("Watch error = %v, want not changeable", err)
	}

	watchCh := make(chan clientv3.WatchResponse, 2)
	defer close(watchCh)
	client := &fakeEtcdClient{}
	client.watchFunc = func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return watchCh }
	client.getFunc = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return nil, errors.New("read failed")
	}
	s = &configSource{
		cfg:         ConfigSourceConfig{Mode: ConfigSourceModeBlob, Key: "/blob"},
		client:      client,
		watch:       true,
		dialTimeout: time.Second,
		closeCh:     make(chan struct{}),
	}
	ch, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	watchCh <- clientv3.WatchResponse{}
	mustNotReceiveStateData(t, ch)
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
	s := &configSource{cfg: ConfigSourceConfig{Mode: ConfigSourceModeKV, Prefix: "/prefix"}}
	if got := s.watchKey(); got != "/prefix" {
		t.Fatalf("watchKey = %q, want /prefix", got)
	}
	if got := len(s.watchOptions()); got != 1 {
		t.Fatalf("watchOptions length = %d, want 1", got)
	}

	blob := &configSource{cfg: ConfigSourceConfig{Mode: ConfigSourceModeBlob, Key: "/key"}}
	if got := blob.watchKey(); got != "/key" {
		t.Fatalf("watchKey = %q, want /key", got)
	}
	if got := blob.watchOptions(); got != nil {
		t.Fatalf("watchOptions = %#v, want nil", got)
	}

	if got := parseScalarOrDoc([]byte("hello"), nil); got != "hello" {
		t.Fatalf("parseScalarOrDoc(nil parser) = %#v, want hello", got)
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
		t.Fatalf("splitConfigPath = %#v", parts)
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

func TestParserMap(t *testing.T) {
	var pm parserMap
	for _, format := range []string{"json", "yaml", "yml", "toml", "unknown"} {
		if err := pm.UnmarshalText([]byte(format)); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", format, err)
		}
		if pm.Parser() == nil {
			t.Fatalf("Parser() is nil for %q", format)
		}
	}
}

func mustNotReceiveStateData(t *testing.T, ch <-chan source.Data) {
	t.Helper()
	select {
	case data := <-ch:
		t.Fatalf("unexpected source data: %#v", data)
	case <-time.After(150 * time.Millisecond):
	}
}
