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

package resolver

import (
	"context"
	"io"
	"testing"

	routeType "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
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

func TestADSClientHelpers(t *testing.T) {
	client, err := newADSClient(ResolverConfig{
		MaxRetries: 2,
		Node: NodeConfig{
			ID:       "node-a",
			Cluster:  "cluster-a",
			Metadata: map[string]string{"env": "test"},
			Locality: &Locality{Region: "cn", Zone: "hz"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	if client.node.Locality == nil || client.node.Locality.Region != "cn" {
		t.Fatalf("node locality not propagated: %#v", client.node.Locality)
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
	client.Close()
}

func TestADSResponseHandling(t *testing.T) {
	client, err := newADSClient(ResolverConfig{
		Node: NodeConfig{ID: "node-a", Cluster: "cluster-a"},
	}, nil)
	if err != nil {
		t.Fatalf("newADSClient() error = %v", err)
	}
	defer client.Close()

	client.ctx, client.cancel = context.WithCancel(context.Background())

	stream := &fakeADSStream{ctx: context.Background()}
	client.sendCh <- &discoveryv3.DiscoveryRequest{TypeUrl: resource.ListenerType}
	client.cancel()
	if err := client.sendLoop(stream); err != nil {
		t.Fatalf("sendLoop() error = %v", err)
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

	validRoute, _ := anypb.New(&routeType.RouteConfiguration{Name: "route-a"})
	client.handleResponse(&discoveryv3.DiscoveryResponse{
		TypeUrl:     resource.RouteType,
		VersionInfo: "v2",
		Nonce:       "nonce-2",
		Resources:   []*anypb.Any{validRoute},
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
		responses: []*discoveryv3.DiscoveryResponse{{
			TypeUrl:   resource.ClusterType,
			Resources: []*anypb.Any{},
		}},
		recvErr: io.EOF,
	}
	if err := client.watchResources(watchStream); err == nil {
		t.Fatal("watchResources() expected EOF to bubble up")
	}
}
