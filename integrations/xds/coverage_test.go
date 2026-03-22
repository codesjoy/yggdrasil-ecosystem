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

package xds

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

type fakeADS struct {
	started bool
	lds     []string
	rds     []string
	cds     []string
	eds     []string
	err     error
}

func (f *fakeADS) Start() error {
	f.started = true
	return f.err
}

func (f *fakeADS) UpdateSubscriptions(lds, rds, cds, eds []string) {
	f.lds = append([]string(nil), lds...)
	f.rds = append([]string(nil), rds...)
	f.cds = append([]string(nil), cds...)
	f.eds = append([]string(nil), eds...)
}

type xdsStateRecorder struct {
	ch chan resolver.State
}

func (r *xdsStateRecorder) UpdateState(state resolver.State) {
	select {
	case r.ch <- state:
	default:
	}
}

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

func TestResolverCoreSubscriptionsAndNotifications(t *testing.T) {
	oldFactory := adsClientFactory
	fake := &fakeADS{}
	adsClientFactory = func(ResolverConfig, func(discoveryEvent)) (adsSubscriptionClient, error) {
		return fake, nil
	}
	t.Cleanup(func() { adsClientFactory = oldFactory })

	resolverAny, err := NewResolver("default", ResolverConfig{
		Protocol:   "grpc",
		ServiceMap: map[string]string{"svc": "listener-1"},
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	r := resolverAny.(*xdsResolver)
	rec := &xdsStateRecorder{ch: make(chan resolver.State, 4)}

	if err := r.AddWatch("svc", rec); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}
	if !fake.started {
		t.Fatal("ADS client was not started")
	}
	if len(fake.lds) != 1 || fake.lds[0] != "listener-1" {
		t.Fatalf("unexpected LDS subscriptions: %#v", fake.lds)
	}

	r.core.onUpdate = func(_ string, state resolver.State) {
		rec.UpdateState(state)
	}
	r.core.apps["svc"].clusters["cluster-a"] = true

	r.core.handleDiscoveryEvent(discoveryEvent{
		typ:  listenerAdded,
		name: "listener-1",
		data: &listenerSnapshot{route: "route-1"},
	})
	r.core.handleDiscoveryEvent(discoveryEvent{
		typ:  routeAdded,
		name: "route-1",
		data: &routeSnapshot{
			vhosts: []*VirtualHost{
				{
					Name: "vh",
					Routes: []*Route{
						{Action: &RouteAction{Cluster: "cluster-a"}},
						{
							Action: &RouteAction{
								WeightedClusters: &WeightedClusters{
									Clusters: []*WeightedCluster{{Name: "cluster-b", Weight: 10}},
								},
							},
						},
					},
				},
			},
		},
	})
	r.core.handleDiscoveryEvent(discoveryEvent{
		typ:  endpointAdded,
		name: "cluster-a",
		data: &edsSnapshot{
			endpoints: []*weightedEndpoint{
				{
					endpoint: Endpoint{Address: "127.0.0.1", Port: 8080},
					weight:   5,
					priority: 1,
					metadata: map[string]string{"env": "test"},
				},
			},
		},
	})

	if len(fake.rds) != 1 || fake.rds[0] != "route-1" {
		t.Fatalf("unexpected RDS subscriptions: %#v", fake.rds)
	}
	if len(fake.cds) < 2 || len(fake.eds) < 2 {
		t.Fatalf("unexpected CDS/EDS subscriptions: %#v %#v", fake.cds, fake.eds)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case state := <-rec.ch:
			if len(state.GetEndpoints()) == 0 {
				continue
			}
			if state.GetEndpoints()[0].GetAddress() != "127.0.0.1:8080" {
				t.Fatalf("unexpected endpoint address: %s", state.GetEndpoints()[0].GetAddress())
			}
			if len(buildRouteConfig(r.core.apps["svc"], r.core.routes, r.core.listeners)) == 0 {
				t.Fatal("buildRouteConfig() returned no virtual hosts")
			}
			if _, ok := buildClusterMap(r.core.apps["svc"])["cluster-a"]; !ok {
				t.Fatal("buildClusterMap() missing cluster-a")
			}
			goto done
		case <-deadline:
			t.Fatal("timeout waiting for resolver state update")
		}
	}
done:

	if err := r.DelWatch("svc", rec); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}
	if len(r.core.apps) != 0 {
		t.Fatalf("apps should be empty after DelWatch: %#v", r.core.apps)
	}
}

func TestRouteMatchAndConfigLoading(t *testing.T) {
	setConfig := func(t *testing.T, key string, value any) {
		t.Helper()
		if err := config.Set(key, value); err != nil {
			t.Fatalf("config.Set(%q) error = %v", key, err)
		}
	}

	base := config.Join(config.KeyBase, "xds", "cfg", "config")
	setConfig(t, config.Join(config.KeyBase, "resolver", "xds-test", "config", "name"), "cfg")
	setConfig(t, config.Join(base, "server", "address"), "127.0.0.1:19000")
	setConfig(t, config.Join(base, "server", "timeout"), "3s")
	setConfig(t, config.Join(base, "node", "id"), "node-a")
	setConfig(t, config.Join(base, "node", "cluster"), "cluster-a")
	setConfig(t, config.Join(base, "node", "metadata"), map[string]string{"env": "test"})
	setConfig(t, config.Join(base, "node", "locality", "region"), "cn")
	setConfig(t, config.Join(base, "service_map"), map[string]string{"svc": "listener-1"})
	setConfig(t, config.Join(base, "retry", "max_retries"), 7)
	setConfig(t, config.Join(base, "retry", "backoff"), "250ms")
	setConfig(t, config.Join(base, "max_retries"), 5)

	cfg := LoadResolverConfig("xds-test")
	if cfg.Server.Address != "127.0.0.1:19000" || cfg.Node.ID != "node-a" ||
		cfg.Retry.MaxRetries != 7 {
		t.Fatalf("unexpected xds config: %#v", cfg)
	}
	if cfg.MaxRetries != 5 {
		t.Fatalf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.Node.Locality == nil || cfg.Node.Locality.Region != "cn" {
		t.Fatalf("unexpected locality: %#v", cfg.Node.Locality)
	}
	if (&BalancerConfig{}).String() == "" {
		t.Fatal("BalancerConfig.String() returned empty string")
	}

	match := &RouteMatch{
		Prefix: "/api",
		Headers: []*HeaderMatcher{
			{Name: "x-env", ExactMatch: "prod"},
			{Name: "x-user", RegexMatch: regexp.MustCompile("^user-")},
		},
	}
	if !match.Matches("/api/v1", map[string]string{"x-env": "prod", "x-user": "user-1"}) {
		t.Fatal("RouteMatch.Matches() expected true")
	}
	if match.Matches("/web", map[string]string{"x-env": "prod"}) {
		t.Fatal("RouteMatch.Matches() expected false")
	}
	action := MatchRoute([]*VirtualHost{
		{
			Routes: []*Route{
				{Match: match, Action: &RouteAction{Cluster: "cluster-a"}},
			},
		},
	}, "/api/v1", map[string]string{"x-env": "prod", "x-user": "user-1"})
	if action == nil || action.Cluster != "cluster-a" {
		t.Fatalf("MatchRoute() = %#v, want cluster-a", action)
	}
}

func TestOutlierHelperMethods(t *testing.T) {
	od := NewOutlierDetector(&OutlierDetectionConfig{
		Consecutive5xx:                1,
		ConsecutiveGatewayFailure:     1,
		ConsecutiveLocalOriginFailure: 1,
		MaxEjectionPercent:            100,
		EnforcingConsecutive5xx:       100,
		BaseEjectionTime:              time.Millisecond,
		MaxEjectionTime:               2 * time.Millisecond,
	})
	ep1 := &EndpointStats{address: "ep-1", consecutive5xx: 1}
	od.endpoints["ep-1"] = ep1
	od.checkConsecutive5xx(ep1)
	if !ep1.ejected {
		t.Fatal("checkConsecutive5xx() did not eject endpoint")
	}
	ep2 := &EndpointStats{address: "ep-2", consecutiveGatewayFailure: 1}
	od.endpoints["ep-2"] = ep2
	od.checkConsecutiveGatewayFailure(ep2)
	if !ep2.ejected {
		t.Fatal("checkConsecutiveGatewayFailure() did not eject endpoint")
	}
	ep3 := &EndpointStats{address: "ep-3", consecutiveLocalFailure: 1}
	od.endpoints["ep-3"] = ep3
	od.checkConsecutiveLocalFailure(ep3)
	if !ep3.ejected {
		t.Fatal("checkConsecutiveLocalFailure() did not eject endpoint")
	}
}
