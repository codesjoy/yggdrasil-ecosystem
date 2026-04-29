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

import "testing"

func TestCircuitBreakerDefaultsAndUnlimited(t *testing.T) {
	defaults := NewCircuitBreaker(nil)
	if defaults.config.MaxConnections != 1024 ||
		defaults.config.MaxPendingRequests != 1024 ||
		defaults.config.MaxRequests != 1024 ||
		defaults.config.MaxRetries != 3 {
		t.Fatalf("default config = %#v", defaults.config)
	}

	unlimited := NewCircuitBreaker(&CircuitBreakerConfig{})
	for _, resource := range []ResourceType{
		ResourceConnection,
		ResourcePendingRequest,
		ResourceRequest,
		ResourceRetry,
	} {
		if !unlimited.TryAcquire(resource) {
			t.Fatalf("TryAcquire(%v) = false, want true for unlimited config", resource)
		}
		unlimited.Release(resource)
	}

	if unlimited.TryAcquire(ResourceType(99)) {
		t.Fatal("TryAcquire(unknown) = true, want false")
	}
	unlimited.Release(ResourceType(99))
}

func TestCircuitBreakerLimitsAndStats(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxConnections:     1,
		MaxPendingRequests: 1,
		MaxRequests:        1,
		MaxRetries:         1,
	})

	if !cb.TryAcquire(ResourceConnection) || cb.TryAcquire(ResourceConnection) {
		t.Fatal("connection limit enforcement failed")
	}
	if !cb.TryAcquire(ResourcePendingRequest) || cb.TryAcquire(ResourcePendingRequest) {
		t.Fatal("pending-request limit enforcement failed")
	}
	if !cb.TryAcquire(ResourceRequest) || cb.TryAcquire(ResourceRequest) {
		t.Fatal("request limit enforcement failed")
	}
	if !cb.TryAcquire(ResourceRetry) || cb.TryAcquire(ResourceRetry) {
		t.Fatal("retry limit enforcement failed")
	}

	cb.Release(ResourceConnection)
	cb.Release(ResourcePendingRequest)
	cb.Release(ResourceRequest)
	cb.Release(ResourceRetry)

	stats := cb.GetStats()
	if stats.ActiveConnections != 0 ||
		stats.PendingRequests != 0 ||
		stats.ActiveRequests != 0 ||
		stats.ActiveRetries != 0 {
		t.Fatalf("expected released counters, got %#v", stats)
	}
	if stats.RejectedRequests != 1 {
		t.Fatalf("RejectedRequests = %d, want 1", stats.RejectedRequests)
	}
	if stats.RejectedRetries != 1 {
		t.Fatalf("RejectedRetries = %d, want 1", stats.RejectedRetries)
	}
}
