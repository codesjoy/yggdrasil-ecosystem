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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/sdk"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
	"google.golang.org/genproto/googleapis/rpc/code"
)

type trafficQuotaFuture struct {
	resp     *model.QuotaResponse
	released int
}

func (f *trafficQuotaFuture) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (f *trafficQuotaFuture) Get() *model.QuotaResponse { return f.resp }
func (f *trafficQuotaFuture) GetImmediately() *model.QuotaResponse {
	return f.resp
}
func (f *trafficQuotaFuture) Release() { f.released++ }

type trafficLimitAPI struct {
	reqs   []polaris.QuotaRequest
	future polaris.QuotaFuture
	err    error
}

func (a *trafficLimitAPI) GetQuota(req polaris.QuotaRequest) (polaris.QuotaFuture, error) {
	a.reqs = append(a.reqs, req)
	return a.future, a.err
}
func (a *trafficLimitAPI) Destroy() {}

type trafficCircuitBreakerAPI struct {
	checks    []model.Resource
	checkResp *model.CheckResult
	checkErr  error
	reports   []*model.ResourceStat
	reportErr error
}

func (a *trafficCircuitBreakerAPI) Check(res model.Resource) (*model.CheckResult, error) {
	a.checks = append(a.checks, res)
	return a.checkResp, a.checkErr
}
func (a *trafficCircuitBreakerAPI) Report(stat *model.ResourceStat) error {
	a.reports = append(a.reports, stat)
	return a.reportErr
}

type trafficRouterAPI struct {
	routerReqs []*polaris.ProcessRoutersRequest
	routerResp *model.InstancesResponse
	routerErr  error
	lbReqs     []*polaris.ProcessLoadBalanceRequest
	lbResp     *model.OneInstanceResponse
	lbErr      error
}

func (a *trafficRouterAPI) ProcessRouters(
	req *polaris.ProcessRoutersRequest,
) (*model.InstancesResponse, error) {
	a.routerReqs = append(a.routerReqs, req)
	if a.routerResp != nil || a.routerErr != nil {
		return a.routerResp, a.routerErr
	}
	if resp, ok := req.DstInstances.(*model.InstancesResponse); ok {
		return resp, nil
	}
	return &model.InstancesResponse{}, nil
}

func (a *trafficRouterAPI) ProcessLoadBalance(
	req *polaris.ProcessLoadBalanceRequest,
) (*model.OneInstanceResponse, error) {
	a.lbReqs = append(a.lbReqs, req)
	if a.lbResp != nil || a.lbErr != nil {
		return a.lbResp, a.lbErr
	}
	if resp, ok := req.DstInstances.(*model.InstancesResponse); ok && len(resp.Instances) > 0 {
		return &model.OneInstanceResponse{
			InstancesResponse: model.InstancesResponse{Instances: []model.Instance{resp.Instances[0]}},
		}, nil
	}
	return &model.OneInstanceResponse{}, nil
}

type trafficRemoteClient struct {
	name      string
	state     remote.State
	closeErr  error
	connects  int
	closeHits int
}

func (c *trafficRemoteClient) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}
func (c *trafficRemoteClient) Close() error {
	c.closeHits++
	return c.closeErr
}
func (c *trafficRemoteClient) Protocol() string    { return c.name }
func (c *trafficRemoteClient) State() remote.State { return c.state }
func (c *trafficRemoteClient) Connect()            { c.connects++ }

type trafficBalancerClient struct {
	updates     []balancer.State
	newRemoteFn func(yresolver.Endpoint, balancer.NewRemoteClientOptions) (remote.Client, error)
}

func (c *trafficBalancerClient) UpdateState(state balancer.State) {
	c.updates = append(c.updates, state)
}

func (c *trafficBalancerClient) NewRemoteClient(
	endpoint yresolver.Endpoint,
	opts balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	if c.newRemoteFn == nil {
		return &trafficRemoteClient{name: endpoint.Name(), state: remote.Ready}, nil
	}
	return c.newRemoteFn(endpoint, opts)
}

func restoreTrafficGlobals(t *testing.T) {
	t.Helper()

	origRateLimit := getRateLimitAPI
	origCircuitBreaker := getCircuitBreakerAPI
	origBalancerAPIs := getBalancerAPIs

	t.Cleanup(func() {
		getRateLimitAPI = origRateLimit
		getCircuitBreakerAPI = origCircuitBreaker
		getBalancerAPIs = origBalancerAPIs
	})
}

func trafficTestState(ids ...string) yresolver.State {
	endpoints := make([]yresolver.Endpoint, 0, len(ids))
	instances := make([]model.Instance, 0, len(ids))
	for idx, id := range ids {
		port := uint32(9000 + idx)
		instances = append(instances, &fakeInstance{
			namespace: "default",
			service:   "svc",
			id:        id,
			host:      "127.0.0.1",
			port:      port,
			protocol:  "grpc",
			healthy:   true,
			weight:    100,
			metadata:  map[string]string{},
		})
		endpoints = append(endpoints, yresolver.BaseEndpoint{
			Address:    "127.0.0.1:" + strconv.Itoa(int(port)),
			Protocol:   "grpc",
			Attributes: map[string]any{"instance_id": id},
		})
	}
	return yresolver.BaseState{
		Attributes: map[string]any{
			"polaris_instances_response": &model.InstancesResponse{
				ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
				Instances:   instances,
			},
		},
		Endpoints: endpoints,
	}
}

func quotaArgsToMap(args []model.Argument) map[string]string {
	out := make(map[string]string, len(args))
	for _, arg := range args {
		out[arg.Key()] = arg.Value()
	}
	return out
}

func TestUnaryClientInterceptorProvidersExposeNames(t *testing.T) {
	providers := UnaryClientInterceptorProviders(nil)
	if len(providers) != 2 {
		t.Fatalf("providers len = %d, want 2", len(providers))
	}
	if providers[0].Name() != "polaris_ratelimit" || providers[1].Name() != "polaris_circuitbreaker" {
		t.Fatalf("provider names = %q, %q", providers[0].Name(), providers[1].Name())
	}
}

func TestBuildPolarisRateLimitUnaryCoversControlFlow(t *testing.T) {
	restoreTrafficGlobals(t)

	t.Run("disabled passes through", func(t *testing.T) {
		unary := buildPolarisRateLimitUnary(func(string) map[string]any {
			return map[string]any{"rate_limit": map[string]any{"enable": false}}
		}, "svc")
		called := 0
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			called++
			return nil
		}); err != nil {
			t.Fatalf("disabled unary error = %v", err)
		}
		if called != 1 {
			t.Fatalf("invoker calls = %d, want 1", called)
		}
	})

	t.Run("init error", func(t *testing.T) {
		wantErr := errors.New("init failed")
		getRateLimitAPI = func(string, governanceConfig) (sdk.LimitAPI, error) {
			return nil, wantErr
		}
		unary := buildPolarisRateLimitUnary(func(string) map[string]any {
			return map[string]any{"rate_limit": map[string]any{"enable": true}}
		}, "svc")
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		}); !errors.Is(err, wantErr) {
			t.Fatalf("unary error = %v, want %v", err, wantErr)
		}
	})

	t.Run("request failure and response branches", func(t *testing.T) {
		future := &trafficQuotaFuture{resp: &model.QuotaResponse{Code: model.QuotaResultOk}}
		api := &trafficLimitAPI{future: future}
		getRateLimitAPI = func(string, governanceConfig) (sdk.LimitAPI, error) { return api, nil }

		unary := buildPolarisRateLimitUnary(func(string) map[string]any {
			return map[string]any{
				"namespace": "custom",
				"rate_limit": map[string]any{
					"enable":      true,
					"token":       2,
					"timeout":     "5ms",
					"retry_count": 3,
					"release":     true,
					"arguments":   map[string]any{"tenant": "gold"},
				},
			}
		}, "svc")

		ctx := metadata.WithOutContext(context.Background(), metadata.MD{"region": {"ap-sh"}})
		invoked := 0
		if err := unary(ctx, "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			invoked++
			return nil
		}); err != nil {
			t.Fatalf("unary error = %v", err)
		}
		if invoked != 1 {
			t.Fatalf("invoker calls = %d, want 1", invoked)
		}
		if future.released != 1 {
			t.Fatalf("future released = %d, want 1", future.released)
		}
		if len(api.reqs) != 1 {
			t.Fatalf("quota reqs = %d, want 1", len(api.reqs))
		}
		req := api.reqs[0].(*model.QuotaRequestImpl)
		if req.GetNamespace() != "custom" || req.GetService() != "svc" || req.GetMethod() != "/svc/method" {
			t.Fatalf("quota request = %#v", req)
		}
		if req.GetToken() != 2 {
			t.Fatalf("quota token = %d, want 2", req.GetToken())
		}
		if req.GetTimeoutPtr() == nil || *req.GetTimeoutPtr() != 5*time.Millisecond {
			t.Fatalf("quota timeout = %v", req.GetTimeoutPtr())
		}
		if req.GetRetryCountPtr() == nil || *req.GetRetryCountPtr() != 3 {
			t.Fatalf("quota retry = %v", req.GetRetryCountPtr())
		}
		if got := quotaArgsToMap(req.Arguments()); got["tenant"] != "gold" || len(got) != 1 {
			t.Fatalf("quota args = %#v", got)
		}

		api.err = errors.New("quota failed")
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		}); err == nil || err.Error() != "quota failed" {
			t.Fatalf("quota error = %v", err)
		}

		api.err = nil
		future.resp = nil
		err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		})
		if status.FromError(err).Code() != code.Code_UNKNOWN {
			t.Fatalf("empty response code = %v", status.FromError(err).Code())
		}

		future.resp = &model.QuotaResponse{Code: model.QuotaResultLimited}
		err = unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		})
		if status.FromError(err).Code() != code.Code_RESOURCE_EXHAUSTED ||
			!strings.Contains(err.Error(), "polaris rate limit exceeded") {
			t.Fatalf("limited error = %v", err)
		}
	})

	t.Run("wait branch honors context cancellation", func(t *testing.T) {
		future := &trafficQuotaFuture{resp: &model.QuotaResponse{Code: model.QuotaResultOk, WaitMs: 50}}
		api := &trafficLimitAPI{future: future}
		getRateLimitAPI = func(string, governanceConfig) (sdk.LimitAPI, error) { return api, nil }
		unary := buildPolarisRateLimitUnary(func(string) map[string]any {
			return map[string]any{"rate_limit": map[string]any{"enable": true}}
		}, "svc")

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		err := unary(cancelCtx, "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		})
		if status.FromError(err).Code() != code.Code_CANCELLED {
			t.Fatalf("cancel error code = %v", status.FromError(err).Code())
		}

		deadlineCtx, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer deadlineCancel()
		err = unary(deadlineCtx, "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		})
		if status.FromError(err).Code() != code.Code_DEADLINE_EXCEEDED {
			t.Fatalf("deadline error code = %v", status.FromError(err).Code())
		}
	})
}

func TestBuildPolarisCircuitBreakerUnaryCoversControlFlow(t *testing.T) {
	restoreTrafficGlobals(t)

	t.Run("disabled passes through", func(t *testing.T) {
		unary := buildPolarisCircuitBreakerUnary(func(string) map[string]any {
			return map[string]any{"circuit_breaker": map[string]any{"enable": false}}
		}, "svc")
		called := 0
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			called++
			return nil
		}); err != nil {
			t.Fatalf("disabled unary error = %v", err)
		}
		if called != 1 {
			t.Fatalf("invoker calls = %d, want 1", called)
		}
	})

	t.Run("init error and open circuit", func(t *testing.T) {
		wantErr := errors.New("init failed")
		getCircuitBreakerAPI = func(string, governanceConfig) (sdk.CircuitBreakerAPI, error) {
			return nil, wantErr
		}
		unary := buildPolarisCircuitBreakerUnary(func(string) map[string]any {
			return map[string]any{"circuit_breaker": map[string]any{"enable": true}}
		}, "svc")
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		}); !errors.Is(err, wantErr) {
			t.Fatalf("unary error = %v, want %v", err, wantErr)
		}

		api := &trafficCircuitBreakerAPI{checkResp: &model.CheckResult{Pass: false, RuleName: "rule-a"}}
		getCircuitBreakerAPI = func(string, governanceConfig) (sdk.CircuitBreakerAPI, error) { return api, nil }
		unary = buildPolarisCircuitBreakerUnary(func(string) map[string]any {
			return map[string]any{"circuit_breaker": map[string]any{"enable": true}}
		}, "svc")
		err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		})
		if status.FromError(err).Code() != code.Code_UNAVAILABLE || !strings.Contains(err.Error(), "rule-a") {
			t.Fatalf("open circuit error = %v", err)
		}
	})

	t.Run("check error and reporting", func(t *testing.T) {
		api := &trafficCircuitBreakerAPI{}
		getCircuitBreakerAPI = func(string, governanceConfig) (sdk.CircuitBreakerAPI, error) { return api, nil }
		unary := buildPolarisCircuitBreakerUnary(func(string) map[string]any {
			return map[string]any{
				"namespace":        "dst",
				"caller_namespace": "src-ns",
				"caller_service":   "src-svc",
				"circuit_breaker":  map[string]any{"enable": true},
			}
		}, "svc")

		api.checkErr = errors.New("check failed")
		if err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return nil
		}); err == nil || err.Error() != "check failed" {
			t.Fatalf("check error = %v", err)
		}

		api.checkErr = nil
		api.checkResp = &model.CheckResult{Pass: true}
		invokeErr := xerror.New(code.Code_INTERNAL, "invoke failed")
		err := unary(context.Background(), "/svc/method", nil, nil, func(context.Context, string, any, any) error {
			return invokeErr
		})
		if !errors.Is(err, invokeErr) {
			t.Fatalf("invoke error = %v, want %v", err, invokeErr)
		}
		if len(api.reports) != 1 {
			t.Fatalf("reports len = %d, want 1", len(api.reports))
		}
		if api.reports[0].RetStatus != model.RetFail || api.reports[0].RetCode != code.Code_INTERNAL.String() {
			t.Fatalf("failure report = %#v", api.reports[0])
		}

		api.reports = nil
		if err := unary(context.Background(), "/svc/ok", nil, nil, func(context.Context, string, any, any) error {
			return nil
		}); err != nil {
			t.Fatalf("success invoke error = %v", err)
		}
		if len(api.reports) != 1 || api.reports[0].RetStatus != model.RetSuccess || api.reports[0].RetCode != "0" {
			t.Fatalf("success report = %#v", api.reports)
		}
	})
}

func TestBalancerProviderAndConstructorUseInjectedAPIs(t *testing.T) {
	restoreTrafficGlobals(t)

	router := &trafficRouterAPI{}
	limit := &trafficLimitAPI{}
	cb := &trafficCircuitBreakerAPI{}
	getBalancerAPIs = func(
		serviceName string,
		cfg governanceConfig,
	) (sdk.RouterAPI, error, sdk.LimitAPI, error, sdk.CircuitBreakerAPI, error) {
		if serviceName != "svc" {
			t.Fatalf("serviceName = %q, want svc", serviceName)
		}
		if cfg.Namespace != "default" || !cfg.Routing.Enable || cfg.Routing.LbPolicy != "round_robin" {
			t.Fatalf("governance config = %#v", cfg)
		}
		return router, nil, limit, nil, cb, nil
	}

	provider := BalancerProvider(func(string) map[string]any {
		return map[string]any{
			"namespace": "default",
			"routing": map[string]any{
				"enable":    true,
				"lb_policy": "round_robin",
			},
		}
	})
	if provider.Type() != polarisBalancerName {
		t.Fatalf("provider type = %q", provider.Type())
	}
	b, err := provider.New("svc", "ignored", &trafficBalancerClient{})
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	pb, ok := b.(*polarisBalancer)
	if !ok {
		t.Fatalf("provider.New() type = %T, want *polarisBalancer", b)
	}
	if pb.router != router || pb.limit != limit || pb.cb != cb {
		t.Fatalf("balancer APIs not wired: %#v", pb)
	}
}

func TestPolarisBalancerCloseAndUpdateState(t *testing.T) {
	t.Run("close aggregates errors and publishes empty picker", func(t *testing.T) {
		cli := &trafficBalancerClient{}
		errA := errors.New("close a")
		errB := errors.New("close b")
		b := &polarisBalancer{
			cli:              cli,
			serviceName:      "svc",
			remoteByName:     map[string]remote.Client{"a": &trafficRemoteClient{name: "a", state: remote.Ready, closeErr: errA}, "b": &trafficRemoteClient{name: "b", state: remote.Ready, closeErr: errB}},
			remoteByInstance: map[string]remote.Client{"a": &trafficRemoteClient{name: "a", state: remote.Ready}, "b": &trafficRemoteClient{name: "b", state: remote.Ready}},
		}

		err := b.Close()
		if !errors.Is(err, errA) || !errors.Is(err, errB) {
			t.Fatalf("Close() error = %v", err)
		}
		if len(cli.updates) != 1 {
			t.Fatalf("updates len = %d, want 1", len(cli.updates))
		}
		if _, err := cli.updates[0].Picker.Next(balancer.RPCInfo{}); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("closed picker error = %v", err)
		}
	})

	t.Run("update state reuses existing clients and skips failures", func(t *testing.T) {
		existing := &trafficRemoteClient{name: "grpc/127.0.0.1:9000", state: remote.Ready}
		created := &trafficRemoteClient{name: "grpc/127.0.0.1:9001", state: remote.Ready}
		cli := &trafficBalancerClient{
			newRemoteFn: func(endpoint yresolver.Endpoint, opts balancer.NewRemoteClientOptions) (remote.Client, error) {
				if opts.StateListener == nil {
					t.Fatal("StateListener should be wired")
				}
				switch endpoint.GetAddress() {
				case "127.0.0.1:9001":
					return created, nil
				case "127.0.0.1:9002":
					return nil, errors.New("dial failed")
				default:
					return nil, nil
				}
			},
		}
		b := &polarisBalancer{
			cli:              cli,
			serviceName:      "svc",
			remoteByName:     map[string]remote.Client{"grpc/127.0.0.1:9000": existing},
			remoteByInstance: map[string]remote.Client{"ins-1": existing},
		}
		state := yresolver.BaseState{
			Attributes: map[string]any{
				"polaris_instances_response": &model.InstancesResponse{
					ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
					Instances: []model.Instance{
						&fakeInstance{id: "ins-1", host: "127.0.0.1", port: 9000, protocol: "grpc"},
						&fakeInstance{id: "ins-2", host: "127.0.0.1", port: 9001, protocol: "grpc"},
						&fakeInstance{id: "ins-3", host: "127.0.0.1", port: 9002, protocol: "grpc"},
					},
				},
			},
			Endpoints: []yresolver.Endpoint{
				yresolver.BaseEndpoint{Address: "127.0.0.1:9000", Protocol: "grpc", Attributes: map[string]any{"instance_id": "ins-1"}},
				yresolver.BaseEndpoint{Address: "127.0.0.1:9001", Protocol: "grpc", Attributes: map[string]any{"instance_id": "ins-2"}},
				yresolver.BaseEndpoint{Address: "127.0.0.1:9002", Protocol: "grpc", Attributes: map[string]any{"instance_id": "ins-3"}},
			},
		}

		b.UpdateState(state)
		if len(b.remoteByName) != 2 || len(b.remoteByInstance) != 2 {
			t.Fatalf("remote maps = %#v / %#v", b.remoteByName, b.remoteByInstance)
		}
		if b.remoteByName["grpc/127.0.0.1:9000"] != existing {
			t.Fatal("existing client was not reused")
		}
		if created.connects != 1 {
			t.Fatalf("new client connects = %d, want 1", created.connects)
		}
		if len(cli.updates) != 1 || cli.updates[0].Picker == nil {
			t.Fatalf("updates = %#v", cli.updates)
		}
	})
}

func TestPolarisBalancerUpdateRemoteClientStateNoopAfterClose(t *testing.T) {
	cli := &trafficBalancerClient{}
	b := &polarisBalancer{
		cli:              cli,
		serviceName:      "svc",
		remoteByName:     map[string]remote.Client{"a": &trafficRemoteClient{name: "a", state: remote.Ready}},
		remoteByInstance: map[string]remote.Client{"a": &trafficRemoteClient{name: "a", state: remote.Ready}},
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	b.updateRemoteClientState(remote.ClientState{State: remote.Ready})
	if len(cli.updates) != 1 {
		t.Fatalf("updates len = %d, want 1", len(cli.updates))
	}
}

func TestPolarisPickerRateLimitAndCircuitBreakerBranches(t *testing.T) {
	t.Run("next handles no ready instances and init errors", func(t *testing.T) {
		p := &polarisPicker{}
		if _, err := p.Next(balancer.RPCInfo{}); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("Next() error = %v", err)
		}

		p = &polarisPicker{
			readyAny: []remote.Client{&trafficRemoteClient{name: "ready", state: remote.Ready}},
			governance: governanceConfig{
				RateLimit: rateLimitConfig{Enable: true},
			},
			limitErr: errors.New("limit init failed"),
		}
		if _, err := p.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"}); err == nil || err.Error() != "limit init failed" {
			t.Fatalf("Next() rate limit error = %v", err)
		}

		p = &polarisPicker{
			readyAny: []remote.Client{&trafficRemoteClient{name: "ready", state: remote.Ready}},
			governance: governanceConfig{
				CircuitBreaker: circuitBreakerConfig{Enable: true},
			},
			cbErr: errors.New("cb init failed"),
		}
		if _, err := p.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"}); err == nil || err.Error() != "cb init failed" {
			t.Fatalf("Next() cb error = %v", err)
		}
	})

	t.Run("rate limit check covers responses", func(t *testing.T) {
		future := &trafficQuotaFuture{resp: &model.QuotaResponse{Code: model.QuotaResultOk}}
		limit := &trafficLimitAPI{future: future}
		p := &polarisPicker{
			serviceName: "svc",
			limit:       limit,
			governance: governanceConfig{
				Namespace: "custom",
				RateLimit: rateLimitConfig{
					Enable:     true,
					Token:      3,
					Timeout:    5 * time.Millisecond,
					RetryCount: 2,
					Arguments:  map[string]string{"tenant": "gold"},
					Release:    true,
				},
			},
		}

		ctx := metadata.WithOutContext(context.Background(), metadata.MD{"region": {"ap-sh"}})
		if err := p.checkRateLimit(ctx, "/svc/method"); err != nil {
			t.Fatalf("checkRateLimit() error = %v", err)
		}
		if future.released != 1 {
			t.Fatalf("future released = %d, want 1", future.released)
		}
		req := limit.reqs[0].(*model.QuotaRequestImpl)
		if req.GetNamespace() != "custom" || req.GetService() != "svc" || req.GetMethod() != "/svc/method" {
			t.Fatalf("quota request = %#v", req)
		}
		if req.GetToken() != 3 {
			t.Fatalf("quota token = %d, want 3", req.GetToken())
		}
		if got := quotaArgsToMap(req.Arguments()); got["tenant"] != "gold" || got["region"] != "ap-sh" {
			t.Fatalf("quota args = %#v", got)
		}

		limit.err = errors.New("quota failed")
		if err := p.checkRateLimit(context.Background(), "/svc/method"); err == nil || err.Error() != "quota failed" {
			t.Fatalf("quota error = %v", err)
		}
		limit.err = nil
		future.resp = nil
		if err := p.checkRateLimit(context.Background(), "/svc/method"); status.FromError(err).Code() != code.Code_UNKNOWN {
			t.Fatalf("empty response code = %v", status.FromError(err).Code())
		}
		future.resp = &model.QuotaResponse{Code: model.QuotaResultLimited, Info: "limited"}
		if err := p.checkRateLimit(context.Background(), "/svc/method"); status.FromError(err).Code() != code.Code_RESOURCE_EXHAUSTED {
			t.Fatalf("limited response code = %v", status.FromError(err).Code())
		}
		future.resp = &model.QuotaResponse{Code: model.QuotaResultOk, WaitMs: 50}
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := p.checkRateLimit(cancelCtx, "/svc/method"); status.FromError(err).Code() != code.Code_CANCELLED {
			t.Fatalf("cancel code = %v", status.FromError(err).Code())
		}
		deadlineCtx, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer deadlineCancel()
		if err := p.checkRateLimit(deadlineCtx, "/svc/method"); status.FromError(err).Code() != code.Code_DEADLINE_EXCEEDED {
			t.Fatalf("deadline code = %v", status.FromError(err).Code())
		}
	})

	t.Run("circuit breaker open and report", func(t *testing.T) {
		cb := &trafficCircuitBreakerAPI{checkResp: &model.CheckResult{Pass: false, RuleName: "rule-a"}}
		p := &polarisPicker{
			serviceName: "svc",
			readyAny:    []remote.Client{&trafficRemoteClient{name: "ready", state: remote.Ready}},
			cb:          cb,
			governance: governanceConfig{
				CircuitBreaker: circuitBreakerConfig{Enable: true},
			},
		}
		_, err := p.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"})
		if status.FromError(err).Code() != code.Code_UNAVAILABLE || !strings.Contains(err.Error(), "rule-a") {
			t.Fatalf("open circuit error = %v", err)
		}

		cb = &trafficCircuitBreakerAPI{checkResp: &model.CheckResult{Pass: true}}
		p = &polarisPicker{
			serviceName: "svc",
			readyAny:    []remote.Client{&trafficRemoteClient{name: "ready", state: remote.Ready}},
			cb:          cb,
			governance: governanceConfig{
				CircuitBreaker: circuitBreakerConfig{Enable: true},
			},
		}
		pr, err := p.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"})
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		invokeErr := xerror.New(code.Code_INTERNAL, "invoke failed")
		pr.Report(invokeErr)
		if len(cb.reports) != 1 || cb.reports[0].RetStatus != model.RetFail || cb.reports[0].RetCode != code.Code_INTERNAL.String() {
			t.Fatalf("failure reports = %#v", cb.reports)
		}
	})
}

func TestPolarisPickerRoutingAndPickerHelpers(t *testing.T) {
	t.Run("routing errors and fallbacks", func(t *testing.T) {
		ready1 := &trafficRemoteClient{name: "ready-1", state: remote.Ready}
		ready2 := &trafficRemoteClient{name: "ready-2", state: remote.Ready}
		router := &trafficRouterAPI{routerErr: errors.New("route failed")}
		p := &polarisPicker{
			serviceName:       "svc",
			instancesResponse: testResolverState().GetAttributes()["polaris_instances_response"].(*model.InstancesResponse),
			readyByInstance:   map[string]remote.Client{"ins-1": ready1, "ins-2": ready2},
			readyAny:          []remote.Client{ready1, ready2},
			router:            router,
			governance: governanceConfig{
				Routing: routingConfig{Enable: true},
			},
		}
		if _, err := p.pickRemote(context.Background(), "/svc/method"); err == nil || err.Error() != "route failed" {
			t.Fatalf("pickRemote route error = %v", err)
		}

		router.routerErr = nil
		router.routerResp = &model.InstancesResponse{}
		p.governance.Routing.RecoverAll = false
		if _, err := p.pickRemote(context.Background(), "/svc/method"); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("pickRemote empty error = %v", err)
		}

		p.governance.Routing.RecoverAll = true
		cli, err := p.pickRemote(context.Background(), "/svc/method")
		if err != nil || (cli != ready1 && cli != ready2) {
			t.Fatalf("pickRemote recoverAll = (%#v, %v)", cli, err)
		}

		router.routerResp = testResolverState().GetAttributes()["polaris_instances_response"].(*model.InstancesResponse)
		router.lbErr = errors.New("lb failed")
		if _, err := p.pickRemote(context.Background(), "/svc/method"); err == nil || err.Error() != "lb failed" {
			t.Fatalf("pickRemote lb error = %v", err)
		}

		router.lbErr = nil
		router.lbResp = &model.OneInstanceResponse{
			InstancesResponse: model.InstancesResponse{
				Instances: []model.Instance{&fakeInstance{id: "missing", host: "127.0.0.1", port: 9999, protocol: "grpc"}},
			},
		}
		cli, err = p.pickRemote(context.Background(), "/svc/method")
		if err != nil || (cli != ready1 && cli != ready2) {
			t.Fatalf("pickRemote fallback = (%#v, %v)", cli, err)
		}
	})

	t.Run("randAllReady rotates and empty set errors", func(t *testing.T) {
		ready1 := &trafficRemoteClient{name: "ready-1", state: remote.Ready}
		ready2 := &trafficRemoteClient{name: "ready-2", state: remote.Ready}
		p := &polarisPicker{readyAny: []remote.Client{ready1, ready2}}

		first, err := p.randAllReady()
		if err != nil || first != ready1 {
			t.Fatalf("first randAllReady = (%#v, %v)", first, err)
		}
		second, err := p.randAllReady()
		if err != nil || second != ready2 {
			t.Fatalf("second randAllReady = (%#v, %v)", second, err)
		}
		empty := &polarisPicker{}
		if _, err := empty.randAllReady(); !errors.Is(err, balancer.ErrNoAvailableInstance) {
			t.Fatalf("empty randAllReady error = %v", err)
		}
	})

	t.Run("filter/process helpers propagate config", func(t *testing.T) {
		ready1 := &trafficRemoteClient{name: "ready-1", state: remote.Ready}
		router := &trafficRouterAPI{}
		resp := &model.InstancesResponse{
			ServiceInfo: model.ServiceInfo{Service: "svc", Namespace: "default"},
			Instances: []model.Instance{
				&fakeInstance{id: "ins-1", host: "127.0.0.1", port: 9000, protocol: "grpc"},
				nil,
				&fakeInstance{id: "ins-2", host: "127.0.0.1", port: 9001, protocol: "grpc"},
			},
		}
		p := &polarisPicker{
			serviceName:     "svc",
			router:          router,
			readyByInstance: map[string]remote.Client{"ins-1": ready1},
			governance: governanceConfig{
				Namespace:       "dst-ns",
				CallerNamespace: "src-ns",
				CallerService:   "src-svc",
				Routing: routingConfig{
					Enable:     true,
					Routers:    []string{"rule"},
					Timeout:    5 * time.Millisecond,
					RetryCount: 2,
					LbPolicy:   "hash",
					Arguments:  map[string]string{"tenant": "gold"},
				},
			},
		}

		filtered := p.filterReadyInstances(resp)
		if len(filtered.Instances) != 1 || filtered.Instances[0].GetId() != "ins-1" {
			t.Fatalf("filtered instances = %#v", filtered.Instances)
		}

		ctx := metadata.WithOutContext(context.Background(), metadata.MD{"region": {"ap-sh"}})
		routed, err := p.processRouters(ctx, "/svc/method", filtered)
		if err != nil {
			t.Fatalf("processRouters() error = %v", err)
		}
		if routed != filtered {
			t.Fatalf("processRouters() resp = %#v, want filtered", routed)
		}
		req := router.routerReqs[0].ProcessRoutersRequest
		if req.SourceService.Service != "src-svc" || req.SourceService.Namespace != "src-ns" || req.Method != "/svc/method" {
			t.Fatalf("router req = %#v", req)
		}
		if req.Timeout == nil || *req.Timeout != 5*time.Millisecond || req.RetryCount == nil || *req.RetryCount != 2 {
			t.Fatalf("router timeout/retry = %#v", req)
		}
		if got := quotaArgsToMap(req.Arguments); got["tenant"] != "gold" || got["region"] != "ap-sh" {
			t.Fatalf("router args = %#v", got)
		}

		one, err := p.processLoadBalance(filtered)
		if err != nil {
			t.Fatalf("processLoadBalance() error = %v", err)
		}
		if one.GetInstance() == nil || router.lbReqs[0].ProcessLoadBalanceRequest.LbPolicy != "hash" {
			t.Fatalf("load balance req/resp = %#v / %#v", router.lbReqs[0], one)
		}
	})
}

func TestPolarisPickResultReportNoopsAndSuccess(t *testing.T) {
	noop := &polarisPickResult{}
	noop.Report(errors.New("ignored"))

	cb := &trafficCircuitBreakerAPI{}
	pr := &polarisPickResult{
		start:          time.Now().Add(-time.Millisecond),
		methodResource: mustMethodResource(t),
		cb:             cb,
	}
	pr.Report(nil)
	if len(cb.reports) != 1 || cb.reports[0].RetStatus != model.RetSuccess || cb.reports[0].RetCode != "0" {
		t.Fatalf("success report = %#v", cb.reports)
	}
}

func mustMethodResource(t *testing.T) *model.MethodResource {
	t.Helper()

	res, err := model.NewMethodResource(
		&model.ServiceKey{Namespace: "default", Service: "svc"},
		&model.ServiceKey{Namespace: "default", Service: "caller"},
		"/svc/method",
	)
	if err != nil {
		t.Fatalf("NewMethodResource() error = %v", err)
	}
	return res
}
