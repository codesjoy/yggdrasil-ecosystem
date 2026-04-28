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

package client

import "testing"

func TestNewDefaults(t *testing.T) {
	cli, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = cli.Close() }()

	got := cli.Endpoints()
	if len(got) != 1 || got[0] != "127.0.0.1:2379" {
		t.Fatalf("Endpoints() = %#v, want [127.0.0.1:2379]", got)
	}
}

func TestLoadConfigUsesDefaultName(t *testing.T) {
	ConfigureConfigLoader(func(name string) Config {
		if name != DefaultClientName {
			t.Fatalf("loader name = %q, want %q", name, DefaultClientName)
		}
		return Config{Endpoints: []string{"10.0.0.1:2379"}}
	})
	t.Cleanup(func() { ConfigureConfigLoader(nil) })

	cfg := LoadConfig("")
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0] != "10.0.0.1:2379" {
		t.Fatalf("LoadConfig() = %#v", cfg)
	}
}
