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

package traffic

import (
	"testing"
	"time"
)

func TestDecodeGovernanceConfigUsesSnakeCase(t *testing.T) {
	cfg := decodeGovernanceConfig(map[string]any{
		"caller_service":   "caller",
		"caller_namespace": "default",
		"rate_limit": map[string]any{
			"enable":      true,
			"retry_count": 2,
			"timeout":     "500ms",
		},
		"circuit_breaker": map[string]any{"enable": true},
		"routing": map[string]any{
			"enable":      true,
			"recover_all": true,
			"retry_count": 3,
			"lb_policy":   "weightedRandom",
		},
	})
	if cfg.CallerService != "caller" || cfg.CallerNamespace != "default" {
		t.Fatalf("caller config = %#v", cfg)
	}
	if !cfg.RateLimit.Enable || cfg.RateLimit.RetryCount != 2 ||
		cfg.RateLimit.Timeout != 500*time.Millisecond {
		t.Fatalf("rate_limit config = %#v", cfg.RateLimit)
	}
	if !cfg.CircuitBreaker.Enable {
		t.Fatalf("circuit_breaker config = %#v", cfg.CircuitBreaker)
	}
	if !cfg.Routing.Enable || !cfg.Routing.RecoverAll ||
		cfg.Routing.RetryCount != 3 || cfg.Routing.LbPolicy != "weightedRandom" {
		t.Fatalf("routing config = %#v", cfg.Routing)
	}
}
