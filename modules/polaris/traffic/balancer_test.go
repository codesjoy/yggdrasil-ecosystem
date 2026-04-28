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

	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeRemoteClient struct {
	name  string
	state remote.State
}

func (f *fakeRemoteClient) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}
func (f *fakeRemoteClient) Close() error        { return nil }
func (f *fakeRemoteClient) Protocol() string    { return f.name }
func (f *fakeRemoteClient) State() remote.State { return f.state }
func (f *fakeRemoteClient) Connect()            {}

type fakeBalancerClient struct {
	lastPicker balancer.Picker
	stateByID  map[string]remote.State
}

func (f *fakeBalancerClient) UpdateState(state balancer.State) {
	f.lastPicker = state.Picker
}

func (f *fakeBalancerClient) NewRemoteClient(
	endpoint yresolver.Endpoint,
	_ balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	id, _ := endpoint.GetAttributes()["instance_id"].(string)
	st := remote.Ready
	if f.stateByID != nil {
		if s, ok := f.stateByID[id]; ok {
			st = s
		}
	}
	return &fakeRemoteClient{name: endpoint.Name(), state: st}, nil
}

type fakeCBStatus struct{}

func (s *fakeCBStatus) GetCircuitBreaker() string            { return "" }
func (s *fakeCBStatus) GetStatus() model.Status              { return 0 }
func (s *fakeCBStatus) GetStartTime() time.Time              { return time.Time{} }
func (s *fakeCBStatus) GetFallbackInfo() *model.FallbackInfo { return nil }
func (s *fakeCBStatus) SetFallbackInfo(*model.FallbackInfo)  {}

type fakeInstance struct {
	namespace string
	service   string
	id        string
	host      string
	port      uint32
	protocol  string
	version   string
	weight    int
	priority  uint32
	metadata  map[string]string
	healthy   bool
	isolated  bool
	region    string
	zone      string
	campus    string
	revision  string
	ttl       int64
}

func (i *fakeInstance) GetInstanceKey() model.InstanceKey { return model.InstanceKey{} }
func (i *fakeInstance) GetNamespace() string              { return i.namespace }
func (i *fakeInstance) GetService() string                { return i.service }
func (i *fakeInstance) GetId() string                     { return i.id } //nolint:revive
func (i *fakeInstance) GetHost() string                   { return i.host }
func (i *fakeInstance) GetPort() uint32                   { return i.port }
func (i *fakeInstance) GetVpcId() string                  { return "" } //nolint:revive
func (i *fakeInstance) GetProtocol() string               { return i.protocol }
func (i *fakeInstance) GetVersion() string                { return i.version }
func (i *fakeInstance) GetWeight() int                    { return i.weight }
func (i *fakeInstance) GetPriority() uint32               { return i.priority }
func (i *fakeInstance) GetMetadata() map[string]string    { return i.metadata }
func (i *fakeInstance) GetLogicSet() string               { return "" }
func (i *fakeInstance) GetCircuitBreakerStatus() model.CircuitBreakerStatus {
	return &fakeCBStatus{}
}
func (i *fakeInstance) IsHealthy() bool           { return i.healthy }
func (i *fakeInstance) IsIsolated() bool          { return i.isolated }
func (i *fakeInstance) IsEnableHealthCheck() bool { return false }
func (i *fakeInstance) GetRegion() string         { return i.region }
func (i *fakeInstance) GetZone() string           { return i.zone }
func (i *fakeInstance) GetIDC() string            { return i.campus }
func (i *fakeInstance) GetCampus() string         { return i.campus }
func (i *fakeInstance) GetRevision() string       { return i.revision }
func (i *fakeInstance) GetTtl() int64             { return i.ttl } //nolint:revive
func (i *fakeInstance) SetHealthy(status bool)    { i.healthy = status }
func (i *fakeInstance) DeepClone() model.Instance { return i }

type fakeRouter struct {
	pickInstanceID string
}

func (f *fakeRouter) ProcessRouters(
	req *polarisgo.ProcessRoutersRequest,
) (*model.InstancesResponse, error) {
	return req.DstInstances.(*model.InstancesResponse), nil
}

func (f *fakeRouter) ProcessLoadBalance(
	req *polarisgo.ProcessLoadBalanceRequest,
) (*model.OneInstanceResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	var picked model.Instance
	for _, inst := range dst.Instances {
		if inst.GetId() == f.pickInstanceID {
			picked = inst
			break
		}
	}
	return &model.OneInstanceResponse{
		InstancesResponse: model.InstancesResponse{Instances: []model.Instance{picked}},
	}, nil
}

type assertRouterNoNonReady struct {
	forbiddenID string
}

func (r *assertRouterNoNonReady) ProcessRouters(
	req *polarisgo.ProcessRoutersRequest,
) (*model.InstancesResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	for _, inst := range dst.Instances {
		if inst != nil && inst.GetId() == r.forbiddenID {
			return nil, errors.New("non-ready instance leaked into routing candidates")
		}
	}
	return dst, nil
}

func (r *assertRouterNoNonReady) ProcessLoadBalance(
	req *polarisgo.ProcessLoadBalanceRequest,
) (*model.OneInstanceResponse, error) {
	dst := req.DstInstances.(*model.InstancesResponse)
	if len(dst.Instances) == 0 {
		return nil, errors.New("empty instances")
	}
	return &model.OneInstanceResponse{
		InstancesResponse: model.InstancesResponse{Instances: []model.Instance{dst.Instances[0]}},
	}, nil
}

func TestPolarisBalancerRoutingPickByInstanceID(t *testing.T) {
	bc := &fakeBalancerClient{}
	pb := newTestPolarisBalancer(bc, &fakeRouter{pickInstanceID: "ins-2"})

	pb.UpdateState(testResolverState())

	if bc.lastPicker == nil {
		t.Fatal("expected picker to be updated")
	}
	pr, err := bc.lastPicker.Next(
		balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"},
	)
	if err != nil {
		t.Fatalf("picker Next err: %v", err)
	}
	if pr.RemoteClient().Protocol() != "grpc/127.0.0.1:9001" {
		t.Fatalf("unexpected remote picked: %s", pr.RemoteClient().Protocol())
	}
}

func TestPolarisBalancerRoutingFiltersNonReadyInstancesBeforeCallingRouter(t *testing.T) {
	bc := &fakeBalancerClient{
		stateByID: map[string]remote.State{
			"ins-1": remote.Ready,
			"ins-2": remote.Connecting,
		},
	}
	pb := newTestPolarisBalancer(bc, &assertRouterNoNonReady{forbiddenID: "ins-2"})

	pb.UpdateState(testResolverState())

	pr, err := bc.lastPicker.Next(
		balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"},
	)
	if err != nil {
		t.Fatalf("picker Next err: %v", err)
	}
	if pr.RemoteClient().Protocol() != "grpc/127.0.0.1:9000" {
		t.Fatalf("unexpected remote picked: %s", pr.RemoteClient().Protocol())
	}
}

func newTestPolarisBalancer(
	cli balancer.Client,
	router interface {
		ProcessRouters(*polarisgo.ProcessRoutersRequest) (*model.InstancesResponse, error)
		ProcessLoadBalance(*polarisgo.ProcessLoadBalanceRequest) (*model.OneInstanceResponse, error)
	},
) *polarisBalancer {
	return &polarisBalancer{
		serviceName:      "svc",
		cli:              cli,
		remoteByName:     make(map[string]remote.Client),
		remoteByInstance: make(map[string]remote.Client),
		governance: governanceConfig{
			Namespace: "default",
			Routing: routingConfig{
				Enable: true,
			},
		},
		router: router,
	}
}

func testResolverState() yresolver.State {
	ins1 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-1",
		host:      "127.0.0.1",
		port:      9000,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	ins2 := &fakeInstance{
		namespace: "default",
		service:   "svc",
		id:        "ins-2",
		host:      "127.0.0.1",
		port:      9001,
		protocol:  "grpc",
		healthy:   true,
		weight:    100,
		metadata:  map[string]string{},
	}
	resp := &model.InstancesResponse{
		ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
		Instances:   []model.Instance{ins1, ins2},
	}

	return yresolver.BaseState{
		Attributes: map[string]any{"polaris_instances_response": resp},
		Endpoints: []yresolver.Endpoint{
			yresolver.BaseEndpoint{
				Address:    "127.0.0.1:9000",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-1"},
			},
			yresolver.BaseEndpoint{
				Address:    "127.0.0.1:9001",
				Protocol:   "grpc",
				Attributes: map[string]any{"instance_id": "ins-2"},
			},
		},
	}
}
