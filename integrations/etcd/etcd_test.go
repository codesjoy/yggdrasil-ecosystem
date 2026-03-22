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
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
)

func TestBuildersRegistered(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_etcd_builders"
	defer func() { config.KeyBase = origKeyBase }()

	if yregistry.GetBuilder("etcd") == nil {
		t.Fatal("registry builder for etcd is not registered")
	}
	if _, err := yregistry.New("etcd", config.Get(config.Join(config.KeyBase, "registry", "config"))); err != nil {
		t.Fatalf("registry builder invoke: %v", err)
	}

	if err := config.Set(config.Join(config.KeyBase, "resolver", "demo", "type"), "etcd"); err != nil {
		t.Fatalf("config.Set resolver type: %v", err)
	}
	res, err := yresolver.Get("demo")
	if err != nil {
		t.Fatalf("resolver builder invoke: %v", err)
	}
	if res == nil || res.Type() != "etcd" {
		t.Fatalf("resolver = %#v, want non-nil etcd resolver", res)
	}
}
