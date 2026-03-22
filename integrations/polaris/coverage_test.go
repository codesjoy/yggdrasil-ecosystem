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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
	polarisgo "github.com/polarismesh/polaris-go"
	polariscfg "github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
	"google.golang.org/genproto/googleapis/rpc/code"
)

type fakeQuotaFuture struct {
	resp     *model.QuotaResponse
	released int
}

func (f *fakeQuotaFuture) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (f *fakeQuotaFuture) Get() *model.QuotaResponse            { return f.resp }
func (f *fakeQuotaFuture) GetImmediately() *model.QuotaResponse { return f.resp }
func (f *fakeQuotaFuture) Release()                             { f.released++ }

type slowConfigAPI struct {
	delay time.Duration
	file  model.ConfigFile
	err   error
}

func (s *slowConfigAPI) FetchConfigFile(*polarisgo.GetConfigFileRequest) (model.ConfigFile, error) {
	time.Sleep(s.delay)
	return s.file, s.err
}

type fakeLimit struct {
	requests []polarisgo.QuotaRequest
	future   polarisgo.QuotaFuture
	err      error
}

func (f *fakeLimit) GetQuota(req polarisgo.QuotaRequest) (polarisgo.QuotaFuture, error) {
	f.requests = append(f.requests, req)
	return f.future, f.err
}

func (f *fakeLimit) Destroy() {}

type fakeCircuitBreaker struct {
	result  *model.CheckResult
	err     error
	reports []*model.ResourceStat
}

func (f *fakeCircuitBreaker) Check(model.Resource) (*model.CheckResult, error) {
	return f.result, f.err
}

func (f *fakeCircuitBreaker) Report(stat *model.ResourceStat) error {
	f.reports = append(f.reports, stat)
	return nil
}

type polarisStateRecorder struct {
	ch chan yresolver.State
}

func (r *polarisStateRecorder) UpdateState(state yresolver.State) {
	select {
	case r.ch <- state:
	default:
	}
}

func resetSDKHolderState(t *testing.T) {
	t.Helper()
	sdkMu.Lock()
	old := sdkHolders
	sdkHolders = map[string]*sdkHolder{}
	sdkMu.Unlock()
	t.Cleanup(func() {
		sdkMu.Lock()
		sdkHolders = old
		sdkMu.Unlock()
	})
}

func sdkKey(name string, addresses []string, configAddresses []string) string {
	cp := append([]string(nil), addresses...)
	sort.Strings(cp)
	ccp := append([]string(nil), configAddresses...)
	sort.Strings(ccp)
	return name + "|" + strings.Join(cp, ",") + "|" + strings.Join(ccp, ",")
}

func installSDKHolder(t *testing.T, name string, addresses []string, holder *sdkHolder) {
	t.Helper()
	sdkMu.Lock()
	sdkHolders[sdkKey(name, addresses, holder.configAddresses)] = holder
	sdkMu.Unlock()
}

func doneOnce(o *sync.Once) {
	o.Do(func() {})
}

func setPolarisConfig(t *testing.T, key string, value any) {
	t.Helper()
	if err := config.Set(key, value); err != nil {
		t.Fatalf("config.Set(%q) error = %v", key, err)
	}
}

func TestResolveSDKConfigAndApplyAddresses(t *testing.T) {
	sdkBase := config.Join(config.KeyBase, "polaris", "sdk-coverage")
	setPolarisConfig(t, config.Join(sdkBase, "addresses"), []string{"127.0.0.1:1001"})
	setPolarisConfig(t, config.Join(sdkBase, "config_addresses"), []string{"127.0.0.1:2001"})
	setPolarisConfig(t, config.Join(sdkBase, "token"), "secret")

	cfg := LoadSDKConfig("sdk-coverage")
	if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "127.0.0.1:1001" {
		t.Fatalf("unexpected SDK addresses: %#v", cfg.Addresses)
	}
	if got := resolveSDKName("owner", "explicit"); got != "explicit" {
		t.Fatalf("resolveSDKName() = %q, want explicit", got)
	}
	if got := resolveSDKName("owner", ""); got != "owner" {
		t.Fatalf("resolveSDKName() = %q, want owner", got)
	}
	if got := resolveSDKAddresses("owner", "sdk-coverage", nil); got[0] != "127.0.0.1:1001" {
		t.Fatalf("resolveSDKAddresses() = %#v", got)
	}
	if got := resolveSDKConfigAddresses("owner", "sdk-coverage", nil); got[0] != "127.0.0.1:2001" {
		t.Fatalf("resolveSDKConfigAddresses() = %#v", got)
	}

	conf := polariscfg.NewDefaultConfiguration([]string{"127.0.0.1:8080"})
	applyConfigAddressesToConfig(conf, []string{"10.0.0.1:9000"})
	got := conf.GetConfigFile().GetConfigConnectorConfig().GetAddresses()
	if len(got) != 1 || got[0] != "10.0.0.1:9000" {
		t.Fatalf("applyConfigAddressesToConfig() = %#v", got)
	}
}

func TestSDKHolder_ContextErrorPropagates(t *testing.T) {
	holder := &sdkHolder{sdkName: "broken-sdk"}
	sdkBase := config.Join(config.KeyBase, "polaris", "broken-sdk")
	setPolarisConfig(t, config.Join(sdkBase, "config_file"), "/definitely/missing-polaris.yaml")

	if _, err := holder.getContext(); err == nil {
		t.Fatal("getContext() expected error")
	}
	if _, err := holder.getProvider(); err == nil {
		t.Fatal("getProvider() expected error")
	}
	if _, err := holder.getConsumer(); err == nil {
		t.Fatal("getConsumer() expected error")
	}
	if _, err := holder.getConfig(); err == nil {
		t.Fatal("getConfig() expected error")
	}
	if _, err := holder.getLimit(); err == nil {
		t.Fatal("getLimit() expected error")
	}
	if _, err := holder.getCircuitBreaker(); err == nil {
		t.Fatal("getCircuitBreaker() expected error")
	}
	if _, err := holder.getRouter(); err == nil {
		t.Fatal("getRouter() expected error")
	}
}

func TestGovernanceConfigLoadAndBuilders(t *testing.T) {
	resetSDKHolderState(t)

	baseKey := config.Join(config.KeyBase, "polaris", "governance", "config")
	serviceKey := config.Join(config.KeyBase, "polaris", "governance", "{svc}", "config")
	setPolarisConfig(t, baseKey, map[string]any{
		"namespace": "default",
		"sdk":       "svc",
		"rateLimit": map[string]any{
			"enable":  true,
			"token":   uint32(2),
			"release": true,
		},
		"circuitBreaker": map[string]any{
			"enable": true,
		},
	})
	setPolarisConfig(t, serviceKey, map[string]any{
		"callerService": "client-a",
	})

	cfg := loadGovernanceConfig("svc")
	if cfg.Namespace != "default" || cfg.CallerService != "client-a" || !cfg.RateLimit.Enable {
		t.Fatalf("unexpected governance config: %#v", cfg)
	}

	quotaFuture := &fakeQuotaFuture{resp: &model.QuotaResponse{Code: model.QuotaResultOk}}
	limitAPI := &fakeLimit{future: quotaFuture}
	cbAPI := &fakeCircuitBreaker{result: &model.CheckResult{Pass: true}}

	holder := &sdkHolder{sdkName: "svc"}
	holder.limit = limitAPI
	holder.cb = cbAPI
	doneOnce(&holder.limitOnce)
	doneOnce(&holder.cbOnce)
	installSDKHolder(t, "svc", nil, holder)

	called := false
	limitInterceptor := buildPolarisRateLimitUnary("svc")
	if err := limitInterceptor(context.Background(), "/svc/method", nil, nil, func(
		context.Context,
		string,
		any,
		any,
	) error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("buildPolarisRateLimitUnary() error = %v", err)
	}
	if !called {
		t.Fatal("rate limit interceptor did not invoke downstream call")
	}
	if len(limitAPI.requests) != 1 {
		t.Fatalf("GetQuota() calls = %d, want 1", len(limitAPI.requests))
	}
	if quotaFuture.released != 1 {
		t.Fatalf("quota future released = %d, want 1", quotaFuture.released)
	}

	cbInterceptor := buildPolarisCircuitBreakerUnary("svc")
	if err := cbInterceptor(context.Background(), "/svc/method", nil, nil, func(
		context.Context,
		string,
		any,
		any,
	) error {
		return errors.New("backend failed")
	}); err == nil {
		t.Fatal("buildPolarisCircuitBreakerUnary() expected downstream error")
	}
	if len(cbAPI.reports) != 1 {
		t.Fatalf("circuit breaker reports = %d, want 1", len(cbAPI.reports))
	}
}

func TestGovernanceBuildersRejectLimitedAndOpenCircuit(t *testing.T) {
	resetSDKHolderState(t)

	baseKey := config.Join(config.KeyBase, "polaris", "governance", "config")
	setPolarisConfig(t, baseKey, map[string]any{
		"namespace": "default",
		"sdk":       "limited-svc",
		"rateLimit": map[string]any{
			"enable": true,
		},
		"circuitBreaker": map[string]any{
			"enable": true,
		},
	})

	limitedFuture := &fakeQuotaFuture{
		resp: &model.QuotaResponse{
			Code: model.QuotaResultLimited,
			Info: "limited",
		},
	}
	holder := &sdkHolder{sdkName: "limited-svc"}
	holder.limit = &fakeLimit{future: limitedFuture}
	holder.cb = &fakeCircuitBreaker{
		result: &model.CheckResult{
			Pass:     false,
			RuleName: "rule-a",
		},
	}
	doneOnce(&holder.limitOnce)
	doneOnce(&holder.cbOnce)
	installSDKHolder(t, "limited-svc", nil, holder)

	if err := buildPolarisRateLimitUnary("limited-svc")(context.Background(), "", nil, nil, func(
		context.Context,
		string,
		any,
		any,
	) error {
		t.Fatal("downstream call should not run when limited")
		return nil
	}); err == nil {
		t.Fatal("rate limit interceptor expected limited error")
	}

	if err := buildPolarisCircuitBreakerUnary("limited-svc")(context.Background(), "", nil, nil, func(
		context.Context,
		string,
		any,
		any,
	) error {
		t.Fatal("downstream call should not run when circuit is open")
		return nil
	}); err == nil {
		t.Fatal("circuit breaker interceptor expected unavailable error")
	}
}

func TestResolverAndRegistryConstructors(t *testing.T) {
	resetSDKHolderState(t)

	holder := &sdkHolder{sdkName: "resolver-sdk"}
	holder.consumer = &fakeConsumer{
		resp: &model.InstancesResponse{
			Instances: []model.Instance{
				&fakeInstance{
					id:       "i-1",
					host:     "127.0.0.1",
					port:     8080,
					protocol: "grpc",
					metadata: map[string]string{"env": "test"},
					weight:   100,
				},
			},
		},
	}
	doneOnce(&holder.consumerOnce)
	holder.provider = &fakeProvider{nextID: "id-1"}
	doneOnce(&holder.providerOnce)
	installSDKHolder(t, "resolver-sdk", nil, holder)

	resolverInst, err := NewResolver("resolver-sdk", ResolverConfig{
		SDK:             "resolver-sdk",
		Namespace:       "default",
		RefreshInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	rec := &polarisStateRecorder{ch: make(chan yresolver.State, 4)}
	if err := resolverInst.AddWatch("svc", rec); err != nil {
		t.Fatalf("AddWatch() error = %v", err)
	}
	select {
	case state := <-rec.ch:
		if len(state.GetEndpoints()) != 1 ||
			state.GetEndpoints()[0].GetAddress() != "127.0.0.1:8080" {
			t.Fatalf("unexpected resolver state: %#v", state.GetEndpoints())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for resolver watch state")
	}
	if err := resolverInst.DelWatch("svc", rec); err != nil {
		t.Fatalf("DelWatch() error = %v", err)
	}

	registryInst, err := NewRegistry("resolver-sdk", RegistryConfig{SDK: "resolver-sdk"})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if registryInst.Type() != "polaris" {
		t.Fatalf("Type() = %q, want polaris", registryInst.Type())
	}

	errResolver := NewResolverWithError("resolver-sdk", ResolverConfig{}, errors.New("boom"))
	if err := errResolver.AddWatch("svc", rec); err == nil {
		t.Fatal("NewResolverWithError().AddWatch() expected error")
	}
	errRegistry := NewRegistryWithError(RegistryConfig{}, errors.New("boom"))
	if err := errRegistry.Register(context.Background(), testInstance{}); err == nil {
		t.Fatal("NewRegistryWithError().Register() expected error")
	}
	if err := errRegistry.Deregister(context.Background(), testInstance{}); err == nil {
		t.Fatal("NewRegistryWithError().Deregister() expected error")
	}
}

func TestUtilityHelpers(t *testing.T) {
	host, port, err := splitHostPort("127.0.0.1:8080")
	if err != nil || host != "127.0.0.1" || port != 8080 {
		t.Fatalf("splitHostPort() = (%q, %d, %v)", host, port, err)
	}
	if _, _, err := splitHostPort("bad-address"); err == nil {
		t.Fatal("splitHostPort() expected error for invalid address")
	}
	merged := mergeStringMap(map[string]string{"a": "1"}, map[string]string{"b": "2", "a": "3"})
	if merged["a"] != "3" || merged["b"] != "2" {
		t.Fatalf("mergeStringMap() = %#v", merged)
	}
	if d := effectiveTimeout(context.Background(), time.Second); d != time.Second {
		t.Fatalf("effectiveTimeout() = %v, want 1s", d)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if d := effectiveTimeout(ctx, time.Second); d <= 0 || d > time.Second {
		t.Fatalf("effectiveTimeout(ctx) = %v, want bounded positive duration", d)
	}
	if got := netAddr("::1", 8080); got != "[::1]:8080" {
		t.Fatalf("netAddr() = %q, want [::1]:8080", got)
	}
	if !shouldBracketIPv6("::1") || shouldBracketIPv6("[::1]") {
		t.Fatal("shouldBracketIPv6() returned unexpected result")
	}
	if got := code.Code_RESOURCE_EXHAUSTED.String(); got == "" {
		t.Fatal("expected code string to be non-empty")
	}
}

func TestResolverConfigAndBuilders(t *testing.T) {
	resetSDKHolderState(t)

	setPolarisConfig(
		t,
		config.Join(config.KeyBase, "resolver", "polaris-test", "config"),
		map[string]any{
			"namespace": "default",
			"sdk":       "builder-sdk",
			"protocols": []string{"grpc"},
		},
	)
	cfg := LoadResolverConfig("polaris-test")
	if cfg.Namespace != "default" || cfg.SDK != "builder-sdk" {
		t.Fatalf("unexpected resolver config: %#v", cfg)
	}

	holder := &sdkHolder{sdkName: "builder-sdk"}
	holder.consumer = &fakeConsumer{resp: &model.InstancesResponse{}}
	holder.provider = &fakeProvider{nextID: "instance-1"}
	doneOnce(&holder.consumerOnce)
	doneOnce(&holder.providerOnce)
	installSDKHolder(t, "builder-sdk", nil, holder)

	setPolarisConfig(t, config.Join(config.KeyBase, "registry", "type"), "polaris")
	setPolarisConfig(t, config.Join(config.KeyBase, "registry", "config"), map[string]any{
		"sdk": "builder-sdk",
	})
	reg, err := yregistry.New(
		"polaris",
		config.Get(config.Join(config.KeyBase, "registry", "config")),
	)
	if err != nil {
		t.Fatalf("registry.New() error = %v", err)
	}
	if reg.Type() != "polaris" {
		t.Fatalf("registry builder Type() = %q, want polaris", reg.Type())
	}

	setPolarisConfig(
		t,
		config.Join(config.KeyBase, "resolver", "polaris-builder", "type"),
		"polaris",
	)
	setPolarisConfig(
		t,
		config.Join(config.KeyBase, "resolver", "polaris-builder", "config"),
		map[string]any{
			"sdk": "builder-sdk",
		},
	)
	res, err := yresolver.Get("polaris-builder")
	if err != nil {
		t.Fatalf("resolver.Get() error = %v", err)
	}
	if res.Type() != "polaris" {
		t.Fatalf("resolver builder Type() = %q, want polaris", res.Type())
	}
}

func TestPolarisConfigSourceTimeout(t *testing.T) {
	src := &configSource{
		cfg: ConfigSourceConfig{
			FileName:     "config.yaml",
			FileGroup:    "app",
			FetchTimeout: 5 * time.Millisecond,
			API: &slowConfigAPI{
				delay: 50 * time.Millisecond,
				file: &fakeConfigFile{
					name:    "config.yaml",
					group:   "app",
					content: "foo: bar\n",
				},
			},
		},
	}
	if _, _, err := src.fetchConfigFile(); err == nil {
		t.Fatal("fetchConfigFile() expected timeout error")
	}
}
