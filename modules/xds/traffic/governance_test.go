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
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateLimiterLifecycle(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{
		MaxTokens:     1,
		TokensPerFill: 1,
		FillInterval:  10 * time.Millisecond,
	})
	defer rl.Stop()

	if !rl.Allow() {
		t.Fatal("Allow() = false, want true on first request")
	}
	if rl.Allow() {
		t.Fatal("Allow() = true, want false after tokens are exhausted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	stats := rl.GetStats()
	if stats.AllowedCount == 0 || stats.MaxTokens != 1 {
		t.Fatalf("unexpected rate limiter stats: %#v", stats)
	}
}

func TestOutlierDetectorPaths(t *testing.T) {
	if ParseHealthStatus("healthy") != HealthHealthy {
		t.Fatal("ParseHealthStatus() did not parse HEALTHY")
	}
	if HealthUnknown.String() != "UNKNOWN" {
		t.Fatal("HealthUnknown.String() = unexpected value")
	}

	od := NewOutlierDetector(&OutlierDetectionConfig{
		Consecutive5xx:                 1,
		ConsecutiveGatewayFailure:      1,
		ConsecutiveLocalOriginFailure:  1,
		Interval:                       5 * time.Millisecond,
		BaseEjectionTime:               5 * time.Millisecond,
		MaxEjectionTime:                20 * time.Millisecond,
		MaxEjectionPercent:             100,
		EnforcingConsecutive5xx:        100,
		EnforcingSuccessRate:           100,
		SuccessRateMinimumHosts:        3,
		SuccessRateRequestVolume:       10,
		SuccessRateStdevFactor:         500,
		FailurePercentageThreshold:     50,
		EnforcingFailurePercentage:     100,
		FailurePercentageMinimumHosts:  2,
		FailurePercentageRequestVolume: 2,
	})
	od.Start()
	defer od.Stop()

	od.ReportResult("ep-a", errors.New("boom"), 503)
	if !od.IsEjected("ep-a") {
		t.Fatal("endpoint should be ejected after consecutive 5xx")
	}

	od.mu.Lock()
	od.endpoints["ep-a"].ejectionTime = time.Now().Add(-time.Millisecond)
	od.mu.Unlock()
	od.performHealthSweep()
	if od.IsEjected("ep-a") {
		t.Fatal("endpoint should recover after ejection timeout")
	}

	od.endpoints["ep-b"] = &EndpointStats{address: "ep-b", totalRequests: 10, successCount: 10}
	od.endpoints["ep-c"] = &EndpointStats{address: "ep-c", totalRequests: 10, successCount: 10}
	od.endpoints["ep-d"] = &EndpointStats{
		address:       "ep-d",
		totalRequests: 10,
		successCount:  0,
		failureCount:  10,
	}
	od.detectSuccessRateOutliers([]*EndpointStats{
		od.endpoints["ep-b"],
		od.endpoints["ep-c"],
		od.endpoints["ep-d"],
	})
	if !od.IsEjected("ep-d") {
		t.Fatal("success-rate outlier should be ejected")
	}

	od.endpoints["ep-e"] = &EndpointStats{address: "ep-e", totalRequests: 4, failureCount: 3}
	od.endpoints["ep-f"] = &EndpointStats{address: "ep-f", totalRequests: 4, failureCount: 0}
	od.detectFailurePercentageOutliers([]*EndpointStats{
		od.endpoints["ep-e"],
		od.endpoints["ep-f"],
	})
	if !od.IsEjected("ep-e") {
		t.Fatal("failure-percentage outlier should be ejected")
	}

	stats := od.GetStats()
	if stats["total_endpoints"] == 0 || stats["total_ejections"] == 0 {
		t.Fatalf("unexpected outlier detector stats: %#v", stats)
	}
	if !od.shouldEnforce(1) || od.shouldEnforce(0) {
		t.Fatal("shouldEnforce() returned unexpected values")
	}
}
