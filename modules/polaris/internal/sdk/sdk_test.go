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

package sdk

import "testing"

func TestConfigLoaderAndHolderKeys(t *testing.T) {
	ConfigureConfigLoader(func(name string) Config {
		return Config{Addresses: []string{name + ":8091"}}
	})
	t.Cleanup(func() { ConfigureConfigLoader(nil) })

	if got := ResolveSDKName("owner", "shared"); got != "shared" {
		t.Fatalf("ResolveSDKName() = %q", got)
	}
	if got := ResolveSDKAddresses("owner", "", nil); len(got) != 1 || got[0] != "owner:8091" {
		t.Fatalf("ResolveSDKAddresses() = %#v", got)
	}
	if got := ResolveSDKAddresses("owner", "", []string{"explicit"}); got[0] != "explicit" {
		t.Fatalf("ResolveSDKAddresses(explicit) = %#v", got)
	}

	h1 := GetHolder("same", []string{"b", "a"}, []string{"d", "c"})
	h2 := GetHolder("same", []string{"a", "b"}, []string{"c", "d"})
	if h1 != h2 {
		t.Fatal("GetHolder() did not normalize address order")
	}
	if h3 := GetHolder("other", []string{"a", "b"}, []string{"c", "d"}); h3 == h1 {
		t.Fatal("GetHolder() reused holder for a different SDK name")
	}
}
