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

package polaris

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type remoteClientTrack struct {
	fakeRemoteClient
	closed bool
}

func (r *remoteClientTrack) Close() error {
	r.closed = true
	return nil
}

func TestPolarisBalancer_TypeCloseAndHelpers(t *testing.T) {
	bc := &fakeBalancerClient{}
	cli := &remoteClientTrack{fakeRemoteClient: fakeRemoteClient{scheme: "grpc/127.0.0.1:8080"}}
	pb := &polarisBalancer{
		serviceName:      "svc",
		cli:              bc,
		remoteByName:     map[string]remote.Client{"grpc/127.0.0.1:8080": cli},
		remoteByInstance: map[string]remote.Client{"ins-1": cli},
	}
	if got := pb.Type(); got != polarisBalancerName {
		t.Fatalf("Type() = %q, want %q", got, polarisBalancerName)
	}
	pb.updateRemoteClientState(remote.ClientState{})
	if bc.lastPicker == nil {
		t.Fatal("updateRemoteClientState() did not publish picker")
	}
	if err := pb.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !cli.closed {
		t.Fatal("Close() did not close remote client")
	}
}

func TestNewPolarisBalancerAndPickerBranches(t *testing.T) {
	resetSDKHolderState(t)

	holder := &sdkHolder{sdkName: "svc"}
	holder.router = &fakeRouter{pickInstanceID: "ins-1"}
	holder.limit = &fakeLimit{
		future: &fakeQuotaFuture{resp: &model.QuotaResponse{Code: model.QuotaResultOk}},
	}
	holder.cb = &fakeCircuitBreaker{result: &model.CheckResult{Pass: true}}
	doneOnce(&holder.routerOnce)
	doneOnce(&holder.limitOnce)
	doneOnce(&holder.cbOnce)
	installSDKHolder(t, "svc", nil, holder)

	b, err := newPolarisBalancer("svc", "", &fakeBalancerClient{})
	if err != nil {
		t.Fatalf("newPolarisBalancer() error = %v", err)
	}
	if b.Type() != polarisBalancerName {
		t.Fatalf("newPolarisBalancer().Type() = %q", b.Type())
	}

	picker := &polarisPicker{
		serviceName: "svc",
		readyAny: []remote.Client{
			&remoteClientTrack{fakeRemoteClient: fakeRemoteClient{scheme: "grpc/127.0.0.1:8080"}},
		},
		governance: governanceConfig{
			Namespace: "default",
			RateLimit: rateLimitConfig{
				Enable:     true,
				Release:    true,
				Arguments:  map[string]string{"source": "unit"},
				RetryCount: 1,
				Timeout:    time.Second,
			},
			CircuitBreaker: circuitBreakerConfig{Enable: true},
		},
		limit: holder.limit,
		cb:    holder.cb,
	}

	ctx := metadata.WithOutContext(context.Background(), metadata.MD{
		"x-env": []string{"test"},
	})
	result, err := picker.Next(balancer.RPCInfo{Ctx: ctx, Method: "/svc/method"})
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if result.RemoteClient() == nil {
		t.Fatal("Next() returned nil remote client")
	}
	result.Report(nil)
	result.Report(errors.New("boom"))
	if len(holder.cb.(*fakeCircuitBreaker).reports) != 2 {
		t.Fatalf("Report() calls = %d, want 2", len(holder.cb.(*fakeCircuitBreaker).reports))
	}
}

func boolPtr(v bool) *bool { return &v }

func TestPolarisPickerErrorBranches(t *testing.T) {
	picker := &polarisPicker{
		serviceName: "svc",
		governance: governanceConfig{
			Namespace: "default",
			RateLimit: rateLimitConfig{Enable: true},
			CircuitBreaker: circuitBreakerConfig{
				Enable: true,
			},
		},
		limit: &fakeLimit{
			future: &fakeQuotaFuture{
				resp: &model.QuotaResponse{Code: model.QuotaResultLimited, Info: "limited"},
			},
		},
		cb: &fakeCircuitBreaker{
			result: &model.CheckResult{Pass: false, RuleName: "open"},
		},
		readyAny: []remote.Client{
			&remoteClientTrack{fakeRemoteClient: fakeRemoteClient{scheme: "grpc/127.0.0.1:8080"}},
		},
	}
	if _, err := picker.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"}); err == nil {
		t.Fatal("Next() expected rate-limit error")
	}

	picker.governance.RateLimit.Enable = false
	if _, err := picker.Next(balancer.RPCInfo{Ctx: context.Background(), Method: "/svc/method"}); err == nil {
		t.Fatal("Next() expected circuit-breaker error")
	}

	picker.governance.CircuitBreaker.Enable = false
	picker.limit = &fakeLimit{future: &fakeQuotaFuture{resp: nil}}
	if err := picker.checkRateLimit(context.Background(), "/svc/method"); err == nil {
		t.Fatal("checkRateLimit() expected empty-response error")
	}
}

func TestPolarisPickerProcessRoutersAndWaitCancellation(t *testing.T) {
	picker := &polarisPicker{
		serviceName: "svc",
		governance: governanceConfig{
			Namespace: "default",
			RateLimit: rateLimitConfig{
				Enable: true,
			},
			Routing: routingConfig{
				Enable:     true,
				Timeout:    time.Second,
				RetryCount: 1,
				Arguments:  map[string]string{"route": "canary"},
			},
		},
		router: &fakeRouter{pickInstanceID: "ins-1"},
		limit: &fakeLimit{
			future: &fakeQuotaFuture{
				resp: &model.QuotaResponse{Code: model.QuotaResultOk, WaitMs: 100},
			},
		},
	}

	ctx, cancel := context.WithCancel(metadata.WithOutContext(context.Background(), metadata.MD{
		"x-user": []string{"alice"},
	}))
	cancel()
	if err := picker.checkRateLimit(ctx, "/svc/method"); err == nil {
		t.Fatal("checkRateLimit() expected cancellation error")
	}

	resp, err := picker.processRouters(
		context.Background(),
		"/svc/method",
		&model.InstancesResponse{
			Instances: []model.Instance{
				&fakeInstance{id: "ins-1", host: "127.0.0.1", port: 8080, protocol: "grpc"},
			},
		},
	)
	if err != nil {
		t.Fatalf("processRouters() error = %v", err)
	}
	if len(resp.Instances) != 1 {
		t.Fatalf("processRouters() instances = %d, want 1", len(resp.Instances))
	}
}

func TestPolarisConfigSourceBranches(t *testing.T) {
	if _, err := NewConfigSource(ConfigSourceConfig{}); err == nil {
		t.Fatal("NewConfigSource() expected empty name error")
	}

	file := &fakeConfigFile{
		namespace: "default",
		group:     "app",
		name:      "config.toml",
		mode:      model.SDKMode,
		content:   "foo = 'bar'\n",
		ch:        make(chan model.ConfigFileChangeEvent, 1),
	}
	srcAny, err := NewConfigSource(ConfigSourceConfig{
		FileName:  "config.toml",
		FileGroup: "app",
		Subscribe: boolPtr(false),
		API:       &fakeConfigAPI{file: file},
	})
	if err != nil {
		t.Fatalf("NewConfigSource() error = %v", err)
	}
	src := srcAny.(*configSource)
	if src.Name() != "config.toml" || src.Type() != "polaris" {
		t.Fatalf("unexpected source identity: %s/%s", src.Name(), src.Type())
	}
	if src.Changeable() {
		t.Fatal("Changeable() = true, want false")
	}
	if _, err := src.Watch(); err == nil {
		t.Fatal("Watch() expected unsubscribable error")
	}
	if _, _, err := (&configSource{
		cfg: ConfigSourceConfig{
			FileName:     "config.yaml",
			FileGroup:    "app",
			FetchTimeout: 10 * time.Millisecond,
			API: configAPI(&fakeConfigAPI{
				err: context.DeadlineExceeded,
			}),
		},
	}).fetchConfigFile(); err == nil {
		t.Fatal("fetchConfigFile() expected API error")
	}
	if parser := inferParserFromFilename("config.json"); parser == nil {
		t.Fatal("inferParserFromFilename(json) returned nil")
	}
}
