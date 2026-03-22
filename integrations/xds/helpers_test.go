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
	"io"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/remote"
	clusterType "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	matcherType "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeADSStream struct {
	ctx       context.Context
	sent      []*discoveryv3.DiscoveryRequest
	responses []*discoveryv3.DiscoveryResponse
	recvErr   error
}

func (f *fakeADSStream) Send(req *discoveryv3.DiscoveryRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

func (f *fakeADSStream) Recv() (*discoveryv3.DiscoveryResponse, error) {
	if len(f.responses) == 0 {
		if f.recvErr != nil {
			return nil, f.recvErr
		}
		return nil, io.EOF
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeADSStream) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (f *fakeADSStream) Trailer() metadata.MD         { return metadata.MD{} }
func (f *fakeADSStream) CloseSend() error             { return nil }
func (f *fakeADSStream) Context() context.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return context.Background()
}
func (f *fakeADSStream) SendMsg(any) error { return nil }
func (f *fakeADSStream) RecvMsg(any) error { return nil }

func TestADSHelpersAndErrors(t *testing.T) {
	client, err := newADSClient(ResolverConfig{
		MaxRetries: 2,
		Node: NodeConfig{
			ID:       "node-a",
			Cluster:  "cluster-a",
			Metadata: map[string]string{"env": "test"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	if err := client.Start(); err == nil {
		t.Fatal("Start() expected empty address error")
	}

	client.sub = subscriptions{
		lds: []string{"listener-a"},
		rds: []string{"route-a"},
		cds: []string{"cluster-a"},
		eds: []string{"cluster-a"},
	}
	client.resendSubscriptions()
	if len(client.sendCh) != 4 {
		t.Fatalf("resendSubscriptions() queued %d requests, want 4", len(client.sendCh))
	}
	client.UpdateSubscriptions([]string{"b", "a"}, []string{"r"}, []string{"c"}, []string{"e"})
	if !subscriptionsEqual(client.sub, subscriptions{
		lds: []string{"a", "b"},
		rds: []string{"r"},
		cds: []string{"c"},
		eds: []string{"e"},
	}) {
		t.Fatalf("unexpected subscriptions: %#v", client.sub)
	}
	if !client.shouldReconnect() {
		t.Fatal("shouldReconnect() = false, want true")
	}
	client.incrementRetry()
	client.incrementRetry()
	if client.shouldReconnect() {
		t.Fatal("shouldReconnect() = true, want false after max retries")
	}
	client.resetRetries()
	if !client.shouldReconnect() {
		t.Fatal("resetRetries() did not reset retry state")
	}

	xerr := NewXDSError("CODE", "message", errors.New("cause"))
	if xerr.Error() == "" || !errors.Is(xerr, xerr.Cause) {
		t.Fatalf("unexpected XDSError behavior: %v", xerr)
	}
	if ErrConnectionFailed(errors.New("boom")).Error() == "" ||
		ErrSubscriptionFailed("lds", "listener-a", nil).Error() == "" ||
		ErrResourceNotFound("lds", "listener-a").Error() == "" ||
		ErrUnmarshalFailed("lds", errors.New("bad")).Error() == "" ||
		ErrClientClosed().Error() == "" ||
		ErrRateLimitExceeded().Error() == "" ||
		ErrCircuitBreakerOpen().Error() == "" ||
		ErrInvalidConfig("bad").Error() == "" {
		t.Fatal("error helper returned empty message")
	}
	if getResourceName(nil) != "" {
		t.Fatal("getResourceName(nil) expected empty string")
	}
}

func TestADSStreamHelpersAndResponseHandling(t *testing.T) {
	client, err := newADSClient(ResolverConfig{
		Node: NodeConfig{ID: "node-a", Cluster: "cluster-a"},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	defer client.Close()

	client.ctx, client.cancel = context.WithCancel(context.Background())

	stream := &fakeADSStream{
		ctx: context.Background(),
		responses: []*discoveryv3.DiscoveryResponse{
			{
				TypeUrl: resource.ListenerType,
				Resources: []*anypb.Any{
					func() *anypb.Any {
						a, _ := anypb.New(&routeType.RouteConfiguration{Name: "listener-a"})
						return a
					}(),
				},
			},
		},
		recvErr: io.EOF,
	}

	client.sendCh <- &discoveryv3.DiscoveryRequest{TypeUrl: resource.ListenerType}
	client.cancel()
	if err := client.sendLoop(stream); err != nil {
		t.Fatalf("sendLoop() error = %v", err)
	}
	for len(client.sendCh) > 0 {
		<-client.sendCh
	}

	invalidResp := &discoveryv3.DiscoveryResponse{
		TypeUrl:     resource.ListenerType,
		VersionInfo: "v1",
		Nonce:       "nonce-1",
		Resources:   []*anypb.Any{{TypeUrl: "bad", Value: []byte("bad")}},
	}
	client.handleResponse(invalidResp)
	select {
	case req := <-client.sendCh:
		if req.ErrorDetail == nil {
			t.Fatal("handleResponse() expected NACK request")
		}
	default:
		t.Fatal("handleResponse() did not enqueue NACK")
	}

	validListener, _ := anypb.New(&routeType.RouteConfiguration{Name: "route-a"})
	client.handleResponse(&discoveryv3.DiscoveryResponse{
		TypeUrl:     resource.RouteType,
		VersionInfo: "v2",
		Nonce:       "nonce-2",
		Resources:   []*anypb.Any{validListener},
	})
	select {
	case req := <-client.sendCh:
		if req.ErrorDetail != nil {
			t.Fatal("expected ACK without error detail")
		}
	default:
		t.Fatal("handleResponse() did not enqueue ACK")
	}

	watchStream := &fakeADSStream{
		responses: []*discoveryv3.DiscoveryResponse{
			{
				TypeUrl:   resource.ClusterType,
				Resources: []*anypb.Any{},
			},
		},
		recvErr: io.EOF,
	}
	if err := client.watchResources(watchStream); err == nil {
		t.Fatal("watchResources() expected EOF to bubble up")
	}
}

func TestCircuitBreakerAndRouteParsingHelpers(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxConnections:     1,
		MaxPendingRequests: 1,
		MaxRequests:        1,
		MaxRetries:         1,
	})
	if !cb.TryAcquire(ResourceConnection) || cb.TryAcquire(ResourceConnection) {
		t.Fatal("unexpected connection acquisition behavior")
	}
	if !cb.TryAcquire(ResourcePendingRequest) || cb.TryAcquire(ResourcePendingRequest) {
		t.Fatal("unexpected pending request acquisition behavior")
	}
	if !cb.TryAcquire(ResourceRequest) || cb.TryAcquire(ResourceRetry) != true {
		t.Fatal("unexpected request/retry acquisition behavior")
	}
	if cb.TryAcquire(ResourceRequest) || cb.TryAcquire(ResourceRetry) {
		t.Fatal("expected request/retry limits to reject")
	}
	cb.Release(ResourceConnection)
	cb.Release(ResourcePendingRequest)
	cb.Release(ResourceRequest)
	cb.Release(ResourceRetry)
	stats := cb.GetStats()
	if stats.RejectedRequests == 0 || stats.RejectedRetries == 0 {
		t.Fatalf("unexpected circuit breaker stats: %#v", stats)
	}

	cfg := LoadBalancerConfig("ignored")
	if cfg.String() == "" {
		t.Fatal("LoadBalancerConfig().String() returned empty string")
	}
	if DefaultOutlierDetectionConfig() == nil {
		t.Fatal("DefaultOutlierDetectionConfig() returned nil")
	}

	regex := matcherRegex("api.*")
	match := parseRouteMatch(&routeType.RouteMatch{
		PathSpecifier: &routeType.RouteMatch_SafeRegex{
			SafeRegex: &regex,
		},
		Headers: []*routeType.HeaderMatcher{
			{
				Name:                 "x-exact",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_ExactMatch{ExactMatch: "a"},
			},
			{
				Name:                 "x-prefix",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_PrefixMatch{PrefixMatch: "pre"},
			},
			{
				Name:                 "x-suffix",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_SuffixMatch{SuffixMatch: "suf"},
			},
			{
				Name:                 "x-present",
				HeaderMatchSpecifier: &routeType.HeaderMatcher_PresentMatch{PresentMatch: true},
			},
		},
	})
	if match == nil || match.Regex == nil || len(match.Headers) != 4 {
		t.Fatalf("parseRouteMatch() = %#v", match)
	}

	action := parseRouteAction(&routeType.RouteAction{
		ClusterSpecifier: &routeType.RouteAction_WeightedClusters{
			WeightedClusters: &routeType.WeightedCluster{
				Clusters: []*routeType.WeightedCluster_ClusterWeight{
					{Name: "cluster-a", Weight: wrapperspb.UInt32(10)},
					{Name: "cluster-b", Weight: wrapperspb.UInt32(20)},
				},
				TotalWeight: wrapperspb.UInt32(30),
			},
		},
	})
	if action == nil || action.WeightedClusters == nil ||
		len(action.WeightedClusters.Clusters) != 2 {
		t.Fatalf("parseRouteAction() = %#v", action)
	}

	clusterEvents := parseCluster(&clusterType.Cluster{
		Name:     "cluster-a",
		LbPolicy: clusterType.Cluster_RANDOM,
		ClusterDiscoveryType: &clusterType.Cluster_Type{
			Type: clusterType.Cluster_EDS,
		},
	})
	if len(clusterEvents) != 1 {
		t.Fatalf("parseCluster() events = %d, want 1", len(clusterEvents))
	}
}

func TestBalancerAndPickResultHelpers(t *testing.T) {
	cli := &mockBalancerClient{}
	b, err := newXdsBalancer("svc", "", cli)
	if err != nil {
		t.Fatalf("newXdsBalancer() error = %v", err)
	}
	xb := b.(*xdsBalancer)
	rc := &mockClient{address: "127.0.0.1", port: 8080}
	xb.remotesClient["127.0.0.1:8080"] = rc
	inflight := int32(1)
	xb.inFlight["127.0.0.1:8080"] = &inflight
	xb.circuitBreakers["cluster-a"] = NewCircuitBreaker(&CircuitBreakerConfig{MaxRequests: 1})
	xb.outlierDetectors["cluster-a"] = NewOutlierDetector(&OutlierDetectionConfig{
		Consecutive5xx:          1,
		MaxEjectionPercent:      100,
		EnforcingConsecutive5xx: 100,
		BaseEjectionTime:        time.Millisecond,
		MaxEjectionTime:         2 * time.Millisecond,
	})
	xb.rateLimiters["cluster-a"] = NewRateLimiter(&RateLimiterConfig{
		MaxTokens:     1,
		TokensPerFill: 1,
		FillInterval:  time.Millisecond,
	})

	xb.UpdateRemoteClientState(remote.ClientState{})
	if cli.state.Picker == nil {
		t.Fatal("UpdateRemoteClientState() did not publish picker")
	}
	stats := xb.GetStats()
	if len(stats.CircuitBreakers) != 1 || len(stats.OutlierDetectors) != 1 ||
		len(stats.RateLimiters) != 1 {
		t.Fatalf("unexpected balancer stats: %#v", stats)
	}
	if got := xb.selectWeightedCluster(&WeightedClusters{
		Clusters: []*WeightedCluster{{Name: "cluster-a", Weight: 10}},
	}); got != "cluster-a" {
		t.Fatalf("selectWeightedCluster() = %q, want cluster-a", got)
	}

	pr := &pickResult{
		ctx:             context.Background(),
		endpoint:        rc,
		balancer:        xb,
		inflightKey:     "127.0.0.1:8080",
		circuitBreaker:  xb.circuitBreakers["cluster-a"],
		outlierDetector: xb.outlierDetectors["cluster-a"],
	}
	if pr.RemoteClient() == nil {
		t.Fatal("RemoteClient() returned nil")
	}
	pr.Report(errors.New("boom"))
	if stats := xb.outlierDetectors["cluster-a"].GetStats(); stats["total_ejections"] == nil {
		t.Fatalf("unexpected outlier stats after report: %#v", stats)
	}

	if err := xb.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if cli.state.Picker == nil {
		t.Fatal("Close() did not republish picker")
	}
}

func TestADSRunAndConnectHelpers(t *testing.T) {
	client, err := newADSClient(ResolverConfig{
		Server: ServerConfig{Timeout: 5 * time.Millisecond},
		Node:   NodeConfig{ID: "node-a", Cluster: "cluster-a"},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	client.cancel()
	client.run()
	if err := client.connect(); err == nil {
		t.Fatal("connect() expected error without address")
	}
}

func matcherRegex(expr string) matcherType.RegexMatcher {
	return matcherType.RegexMatcher{Regex: expr}
}
