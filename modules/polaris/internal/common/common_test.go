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

package common

import (
	"context"
	"testing"
	"time"
)

func TestNetworkAndMapHelpers(t *testing.T) {
	host, port, err := SplitHostPort("127.0.0.1:8080")
	if err != nil || host != "127.0.0.1" || port != 8080 {
		t.Fatalf("SplitHostPort() = (%q, %d, %v)", host, port, err)
	}
	if got := NetAddr("::1", 8080); got != "[::1]:8080" {
		t.Fatalf("NetAddr() = %q", got)
	}
	merged := MergeStringMap(map[string]string{"a": "1"}, map[string]string{"a": "2", "b": "3"})
	if merged["a"] != "2" || merged["b"] != "3" {
		t.Fatalf("MergeStringMap() = %#v", merged)
	}
	if got := EffectiveTimeout(context.Background(), time.Second); got != time.Second {
		t.Fatalf("EffectiveTimeout() = %v", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if got := EffectiveTimeout(ctx, time.Second); got <= 0 || got > time.Minute {
		t.Fatalf("EffectiveTimeout(ctx) = %v", got)
	}
}
