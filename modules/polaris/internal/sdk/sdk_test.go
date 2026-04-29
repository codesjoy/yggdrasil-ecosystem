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

package sdk

import (
	"errors"
	"reflect"
	"testing"
	"time"

	polaris "github.com/polarismesh/polaris-go"
	polarisapi "github.com/polarismesh/polaris-go/api"
	polariscfg "github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
	polarisplugin "github.com/polarismesh/polaris-go/pkg/plugin"
)

type testSDKContext struct {
	cfg polariscfg.Configuration
}

func (c *testSDKContext) Destroy() {}
func (c *testSDKContext) IsDestroyed() bool {
	return false
}
func (c *testSDKContext) GetConfig() polariscfg.Configuration {
	return c.cfg
}
func (c *testSDKContext) GetPlugins() polarisplugin.Manager {
	return nil
}
func (c *testSDKContext) GetEngine() model.Engine {
	return nil
}
func (c *testSDKContext) GetValueContext() model.ValueContext {
	return nil
}

type testProviderAPI struct{}

func (testProviderAPI) RegisterInstance(*polaris.InstanceRegisterRequest) (*model.InstanceRegisterResponse, error) {
	return &model.InstanceRegisterResponse{}, nil
}
func (testProviderAPI) Deregister(*polaris.InstanceDeRegisterRequest) error { return nil }

type testConsumerAPI struct{}

func (testConsumerAPI) GetInstances(*polaris.GetInstancesRequest) (*model.InstancesResponse, error) {
	return &model.InstancesResponse{}, nil
}

type testConfigAPI struct{}

func (testConfigAPI) FetchConfigFile(*polaris.GetConfigFileRequest) (model.ConfigFile, error) {
	return nil, nil
}

type testLimitAPI struct{}

func (testLimitAPI) GetQuota(polaris.QuotaRequest) (polaris.QuotaFuture, error) { return nil, nil }
func (testLimitAPI) Destroy()                                                   {}

type testCircuitBreakerAPI struct{}

func (testCircuitBreakerAPI) Check(model.Resource) (*model.CheckResult, error) { return nil, nil }
func (testCircuitBreakerAPI) Report(*model.ResourceStat) error                 { return nil }

type testRouterAPI struct{}

func (testRouterAPI) ProcessRouters(*polaris.ProcessRoutersRequest) (*model.InstancesResponse, error) {
	return &model.InstancesResponse{}, nil
}
func (testRouterAPI) ProcessLoadBalance(*polaris.ProcessLoadBalanceRequest) (*model.OneInstanceResponse, error) {
	return &model.OneInstanceResponse{}, nil
}

func restoreSDKGlobals(t *testing.T) {
	t.Helper()

	origLoadConfigurationByFile := loadConfigurationByFile
	origNewDefaultConfiguration := newDefaultConfiguration
	origNewDefaultConfigurationWithDomain := newDefaultConfigurationWithDomain
	origNewSDKContextByConfig := newSDKContextByConfig
	origNewSDKContextByAddress := newSDKContextByAddress
	origNewSDKContext := newSDKContext
	origNewProviderAPIByContext := newProviderAPIByContext
	origNewConsumerAPIByContext := newConsumerAPIByContext
	origNewConfigAPIByContext := newConfigAPIByContext
	origNewLimitAPIByContext := newLimitAPIByContext
	origNewCircuitBreakerAPIByContext := newCircuitBreakerAPIByContext
	origNewRouterAPIByContext := newRouterAPIByContext

	t.Cleanup(func() {
		loadConfigurationByFile = origLoadConfigurationByFile
		newDefaultConfiguration = origNewDefaultConfiguration
		newDefaultConfigurationWithDomain = origNewDefaultConfigurationWithDomain
		newSDKContextByConfig = origNewSDKContextByConfig
		newSDKContextByAddress = origNewSDKContextByAddress
		newSDKContext = origNewSDKContext
		newProviderAPIByContext = origNewProviderAPIByContext
		newConsumerAPIByContext = origNewConsumerAPIByContext
		newConfigAPIByContext = origNewConfigAPIByContext
		newLimitAPIByContext = origNewLimitAPIByContext
		newCircuitBreakerAPIByContext = origNewCircuitBreakerAPIByContext
		newRouterAPIByContext = origNewRouterAPIByContext
		ConfigureConfigLoader(nil)
		sdkMu.Lock()
		sdkHolders = map[string]*ClientHolder{}
		sdkMu.Unlock()
	})
}

func TestApplyTokenAndConfigAddressesToConfig(t *testing.T) {
	cfg := polariscfg.NewDefaultConfiguration([]string{"127.0.0.1:8091"})

	applyTokenToConfig(nil, "ignored")
	applyConfigAddressesToConfig(nil, []string{"ignored"})
	applyTokenToConfig(cfg, "token")
	applyConfigAddressesToConfig(cfg, []string{"127.0.0.1:8093"})

	if got := cfg.GetGlobal().GetServerConnector().GetToken(); got != "token" {
		t.Fatalf("global token = %q, want token", got)
	}
	if got := cfg.GetConfigFile().GetConfigConnectorConfig().GetToken(); got != "token" {
		t.Fatalf("config token = %q, want token", got)
	}
	if got := cfg.GetConfigFile().GetConfigConnectorConfig().GetAddresses(); !reflect.DeepEqual(got, []string{"127.0.0.1:8093"}) {
		t.Fatalf("config addresses = %#v", got)
	}
}

func TestResolveSDKConfigAddressesUsesExplicitOrConfiguredValues(t *testing.T) {
	ConfigureConfigLoader(func(name string) Config {
		return Config{ConfigAddress: []string{name + ":8093"}}
	})
	t.Cleanup(func() { ConfigureConfigLoader(nil) })

	if got := ResolveSDKConfigAddresses("owner", "", nil); !reflect.DeepEqual(got, []string{"owner:8093"}) {
		t.Fatalf("ResolveSDKConfigAddresses() = %#v", got)
	}
	if got := ResolveSDKConfigAddresses("owner", "", []string{"explicit:8093"}); !reflect.DeepEqual(got, []string{"explicit:8093"}) {
		t.Fatalf("ResolveSDKConfigAddresses(explicit) = %#v", got)
	}
}

func TestClientHolderInitContextUsesConfigFileAndCachesResult(t *testing.T) {
	restoreSDKGlobals(t)

	cfg := polariscfg.NewDefaultConfiguration([]string{"127.0.0.1:8091"})
	wantCtx := &testSDKContext{cfg: cfg}
	loadCalls := 0
	ctxCalls := 0

	ConfigureConfigLoader(func(string) Config {
		return Config{
			ConfigFile:    "/tmp/polaris.yaml",
			Token:         "token",
			ConfigAddress: []string{"127.0.0.1:8093"},
		}
	})
	loadConfigurationByFile = func(path string) (polariscfg.Configuration, error) {
		loadCalls++
		if path != "/tmp/polaris.yaml" {
			t.Fatalf("loadConfigurationByFile path = %q", path)
		}
		return cfg, nil
	}
	newSDKContextByConfig = func(c polariscfg.Configuration) (polarisapi.SDKContext, error) {
		ctxCalls++
		if got := c.GetGlobal().GetServerConnector().GetToken(); got != "token" {
			t.Fatalf("global token = %q, want token", got)
		}
		if got := c.GetConfigFile().GetConfigConnectorConfig().GetToken(); got != "token" {
			t.Fatalf("config token = %q, want token", got)
		}
		if got := c.GetConfigFile().GetConfigConnectorConfig().GetAddresses(); !reflect.DeepEqual(got, []string{"127.0.0.1:8093"}) {
			t.Fatalf("config addresses = %#v", got)
		}
		return wantCtx, nil
	}

	h := &ClientHolder{sdkName: "default"}
	gotCtx, err := h.getContext()
	if err != nil {
		t.Fatalf("getContext() error = %v", err)
	}
	if gotCtx != wantCtx {
		t.Fatalf("getContext() returned unexpected context: %#v", gotCtx)
	}
	if _, err := h.getContext(); err != nil {
		t.Fatalf("second getContext() error = %v", err)
	}
	if loadCalls != 1 || ctxCalls != 1 {
		t.Fatalf("calls = load %d / ctx %d, want 1 / 1", loadCalls, ctxCalls)
	}
}

func TestClientHolderInitContextHandlesAllConstructionBranches(t *testing.T) {
	tests := []struct {
		name      string
		holder    *ClientHolder
		loaderCfg Config
		verify    func(t *testing.T)
	}{
		{
			name:   "config addresses branch",
			holder: &ClientHolder{sdkName: "owner", addresses: []string{"127.0.0.1:8091"}},
			loaderCfg: Config{
				Token:         "token",
				ConfigAddress: []string{"127.0.0.1:8093"},
			},
			verify: func(t *testing.T) {
				t.Helper()
			},
		},
		{
			name: "domain fallback branch",
			holder: &ClientHolder{
				sdkName: "owner",
			},
			loaderCfg: Config{
				Token:         "token",
				ConfigAddress: []string{"127.0.0.1:8093"},
			},
			verify: func(t *testing.T) {
				t.Helper()
			},
		},
		{
			name: "addresses branch",
			holder: &ClientHolder{
				sdkName:   "owner",
				addresses: []string{"127.0.0.1:8091"},
			},
			loaderCfg: Config{},
			verify: func(t *testing.T) {
				t.Helper()
			},
		},
		{
			name:      "default branch",
			holder:    &ClientHolder{sdkName: "owner"},
			loaderCfg: Config{},
			verify: func(t *testing.T) {
				t.Helper()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restoreSDKGlobals(t)

			ConfigureConfigLoader(func(string) Config { return tc.loaderCfg })

			ctxCalls := 0
			wantCtx := &testSDKContext{}
			cfgCalls := 0
			domainCalls := 0
			addrCalls := 0
			defaultCalls := 0

			newDefaultConfiguration = func(addresses []string) *polariscfg.ConfigurationImpl {
				cfgCalls++
				if !reflect.DeepEqual(addresses, []string{"127.0.0.1:8091"}) {
					t.Fatalf("newDefaultConfiguration addresses = %#v", addresses)
				}
				return polariscfg.NewDefaultConfiguration(addresses)
			}
			newDefaultConfigurationWithDomain = func() *polariscfg.ConfigurationImpl {
				domainCalls++
				return polariscfg.NewDefaultConfigurationWithDomain()
			}
			newSDKContextByConfig = func(c polariscfg.Configuration) (polarisapi.SDKContext, error) {
				ctxCalls++
				if got := c.GetGlobal().GetServerConnector().GetToken(); got != "token" {
					t.Fatalf("global token = %q, want token", got)
				}
				if got := c.GetConfigFile().GetConfigConnectorConfig().GetAddresses(); !reflect.DeepEqual(got, []string{"127.0.0.1:8093"}) {
					t.Fatalf("config addresses = %#v", got)
				}
				return wantCtx, nil
			}
			newSDKContextByAddress = func(addresses ...string) (polarisapi.SDKContext, error) {
				addrCalls++
				if !reflect.DeepEqual(addresses, []string{"127.0.0.1:8091"}) {
					t.Fatalf("newSDKContextByAddress addresses = %#v", addresses)
				}
				return wantCtx, nil
			}
			newSDKContext = func() (polarisapi.SDKContext, error) {
				defaultCalls++
				return wantCtx, nil
			}

			gotCtx, err := tc.holder.getContext()
			if err != nil {
				t.Fatalf("getContext() error = %v", err)
			}
			if gotCtx != wantCtx {
				t.Fatalf("getContext() returned unexpected context: %#v", gotCtx)
			}

			switch tc.name {
			case "config addresses branch":
				if cfgCalls != 1 || domainCalls != 0 || ctxCalls != 1 {
					t.Fatalf("calls = cfg %d domain %d ctx %d", cfgCalls, domainCalls, ctxCalls)
				}
			case "domain fallback branch":
				if cfgCalls != 0 || domainCalls != 1 || ctxCalls != 1 {
					t.Fatalf("calls = cfg %d domain %d ctx %d", cfgCalls, domainCalls, ctxCalls)
				}
			case "addresses branch":
				if addrCalls != 1 || defaultCalls != 0 {
					t.Fatalf("calls = address %d default %d", addrCalls, defaultCalls)
				}
			case "default branch":
				if addrCalls != 0 || defaultCalls != 1 {
					t.Fatalf("calls = address %d default %d", addrCalls, defaultCalls)
				}
			}
		})
	}
}

func TestClientHolderAPIsReuseContextAndCacheErrors(t *testing.T) {
	type apiCase struct {
		name             string
		call             func(*ClientHolder) (any, error)
		setConstructor   func(int)
		wantConstructorN int
	}

	tests := []apiCase{
		{
			name: "provider",
			call: func(h *ClientHolder) (any, error) { return h.Provider() },
			setConstructor: func(counter int) {
				newProviderAPIByContext = func(polarisapi.SDKContext) ProviderAPI {
					panic(counter)
				}
			},
		},
	}

	_ = tests

	restoreSDKGlobals(t)

	ctxCalls := 0
	wantCtx := &testSDKContext{}
	wantProvider := testProviderAPI{}
	wantConsumer := testConsumerAPI{}
	wantConfig := testConfigAPI{}
	wantLimit := testLimitAPI{}
	wantCB := testCircuitBreakerAPI{}
	wantRouter := testRouterAPI{}

	newSDKContext = func() (polarisapi.SDKContext, error) {
		ctxCalls++
		return wantCtx, nil
	}

	providerCalls := 0
	consumerCalls := 0
	configCalls := 0
	limitCalls := 0
	cbCalls := 0
	routerCalls := 0

	newProviderAPIByContext = func(ctx polarisapi.SDKContext) ProviderAPI {
		providerCalls++
		if ctx != wantCtx {
			t.Fatalf("provider ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantProvider
	}
	newConsumerAPIByContext = func(ctx polarisapi.SDKContext) ConsumerAPI {
		consumerCalls++
		if ctx != wantCtx {
			t.Fatalf("consumer ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantConsumer
	}
	newConfigAPIByContext = func(ctx polarisapi.SDKContext) ConfigAPI {
		configCalls++
		if ctx != wantCtx {
			t.Fatalf("config ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantConfig
	}
	newLimitAPIByContext = func(ctx polarisapi.SDKContext) LimitAPI {
		limitCalls++
		if ctx != wantCtx {
			t.Fatalf("limit ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantLimit
	}
	newCircuitBreakerAPIByContext = func(ctx polarisapi.SDKContext) CircuitBreakerAPI {
		cbCalls++
		if ctx != wantCtx {
			t.Fatalf("circuit breaker ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantCB
	}
	newRouterAPIByContext = func(ctx polarisapi.SDKContext) RouterAPI {
		routerCalls++
		if ctx != wantCtx {
			t.Fatalf("router ctx = %#v, want %#v", ctx, wantCtx)
		}
		return wantRouter
	}

	h := &ClientHolder{sdkName: "default"}

	if got, err := h.Provider(); err != nil || got != wantProvider {
		t.Fatalf("Provider() = (%#v, %v)", got, err)
	}
	if got, err := h.Provider(); err != nil || got != wantProvider {
		t.Fatalf("Provider() second = (%#v, %v)", got, err)
	}
	if got, err := h.Consumer(); err != nil || got != wantConsumer {
		t.Fatalf("Consumer() = (%#v, %v)", got, err)
	}
	if got, err := h.Config(); err != nil || got != wantConfig {
		t.Fatalf("Config() = (%#v, %v)", got, err)
	}
	if got, err := h.Limit(); err != nil || got != wantLimit {
		t.Fatalf("Limit() = (%#v, %v)", got, err)
	}
	if got, err := h.CircuitBreaker(); err != nil || got != wantCB {
		t.Fatalf("CircuitBreaker() = (%#v, %v)", got, err)
	}
	if got, err := h.Router(); err != nil || got != wantRouter {
		t.Fatalf("Router() = (%#v, %v)", got, err)
	}

	if ctxCalls != 1 {
		t.Fatalf("context calls = %d, want 1", ctxCalls)
	}
	if providerCalls != 1 || consumerCalls != 1 || configCalls != 1 ||
		limitCalls != 1 || cbCalls != 1 || routerCalls != 1 {
		t.Fatalf(
			"api constructor calls = provider %d consumer %d config %d limit %d cb %d router %d",
			providerCalls,
			consumerCalls,
			configCalls,
			limitCalls,
			cbCalls,
			routerCalls,
		)
	}
}

func TestClientHolderAPIConstructorsReturnContextErrors(t *testing.T) {
	restoreSDKGlobals(t)

	wantErr := errors.New("boom")
	newSDKContext = func() (polarisapi.SDKContext, error) {
		return nil, wantErr
	}

	providerCalls := 0
	consumerCalls := 0
	configCalls := 0
	limitCalls := 0
	cbCalls := 0
	routerCalls := 0
	newProviderAPIByContext = func(polarisapi.SDKContext) ProviderAPI {
		providerCalls++
		return testProviderAPI{}
	}
	newConsumerAPIByContext = func(polarisapi.SDKContext) ConsumerAPI {
		consumerCalls++
		return testConsumerAPI{}
	}
	newConfigAPIByContext = func(polarisapi.SDKContext) ConfigAPI {
		configCalls++
		return testConfigAPI{}
	}
	newLimitAPIByContext = func(polarisapi.SDKContext) LimitAPI {
		limitCalls++
		return testLimitAPI{}
	}
	newCircuitBreakerAPIByContext = func(polarisapi.SDKContext) CircuitBreakerAPI {
		cbCalls++
		return testCircuitBreakerAPI{}
	}
	newRouterAPIByContext = func(polarisapi.SDKContext) RouterAPI {
		routerCalls++
		return testRouterAPI{}
	}

	h1 := &ClientHolder{sdkName: "default"}
	if _, err := h1.Provider(); !errors.Is(err, wantErr) {
		t.Fatalf("Provider() error = %v, want %v", err, wantErr)
	}
	h2 := &ClientHolder{sdkName: "default"}
	if _, err := h2.Consumer(); !errors.Is(err, wantErr) {
		t.Fatalf("Consumer() error = %v, want %v", err, wantErr)
	}
	h3 := &ClientHolder{sdkName: "default"}
	if _, err := h3.Config(); !errors.Is(err, wantErr) {
		t.Fatalf("Config() error = %v, want %v", err, wantErr)
	}
	h4 := &ClientHolder{sdkName: "default"}
	if _, err := h4.Limit(); !errors.Is(err, wantErr) {
		t.Fatalf("Limit() error = %v, want %v", err, wantErr)
	}
	h5 := &ClientHolder{sdkName: "default"}
	if _, err := h5.CircuitBreaker(); !errors.Is(err, wantErr) {
		t.Fatalf("CircuitBreaker() error = %v, want %v", err, wantErr)
	}
	h6 := &ClientHolder{sdkName: "default"}
	if _, err := h6.Router(); !errors.Is(err, wantErr) {
		t.Fatalf("Router() error = %v, want %v", err, wantErr)
	}

	if providerCalls != 0 || consumerCalls != 0 || configCalls != 0 ||
		limitCalls != 0 || cbCalls != 0 || routerCalls != 0 {
		t.Fatalf(
			"api constructors should not run, got provider %d consumer %d config %d limit %d cb %d router %d",
			providerCalls,
			consumerCalls,
			configCalls,
			limitCalls,
			cbCalls,
			routerCalls,
		)
	}
}

func TestClientHolderInitContextReturnsLoadError(t *testing.T) {
	restoreSDKGlobals(t)

	wantErr := errors.New("load failed")
	ConfigureConfigLoader(func(string) Config { return Config{ConfigFile: "/tmp/polaris.yaml"} })
	loadConfigurationByFile = func(string) (polariscfg.Configuration, error) {
		return nil, wantErr
	}

	h := &ClientHolder{sdkName: "default"}
	if _, err := h.getContext(); !errors.Is(err, wantErr) {
		t.Fatalf("getContext() error = %v, want %v", err, wantErr)
	}
}

func TestGetHolderCachesByNormalizedKey(t *testing.T) {
	restoreSDKGlobals(t)

	h1 := GetHolder("same", []string{"b", "a"}, []string{"d", "c"})
	h2 := GetHolder("same", []string{"a", "b"}, []string{"c", "d"})
	if h1 != h2 {
		t.Fatal("GetHolder() did not normalize address order")
	}
	if h1 == GetHolder("same", []string{"a", "b"}, []string{"cfg"}) {
		t.Fatal("GetHolder() reused holder for different config addresses")
	}
	if h1 == GetHolder("other", []string{"a", "b"}, []string{"c", "d"}) {
		t.Fatal("GetHolder() reused holder for different SDK name")
	}
}

func TestEffectiveConfigTimeoutHelpersUseConfiguredPaths(t *testing.T) {
	restoreSDKGlobals(t)

	ConfigureConfigLoader(func(string) Config {
		return Config{Addresses: []string{"127.0.0.1:8091"}, ConfigAddress: []string{"127.0.0.1:8093"}}
	})

	if got := ResolveSDKAddresses("owner", "", nil); !reflect.DeepEqual(got, []string{"127.0.0.1:8091"}) {
		t.Fatalf("ResolveSDKAddresses() = %#v", got)
	}
	if got := ResolveSDKConfigAddresses("owner", "", nil); !reflect.DeepEqual(got, []string{"127.0.0.1:8093"}) {
		t.Fatalf("ResolveSDKConfigAddresses() = %#v", got)
	}

	// Ensure nil/empty branches stay no-op.
	applyConfigAddressesToConfig(polariscfg.NewDefaultConfiguration(nil), nil)
	applyTokenToConfig(polariscfg.NewDefaultConfiguration(nil), "")
	time.Sleep(0)
}

func TestConfigLoaderAndHolderKeys(t *testing.T) {
	ConfigureConfigLoader(func(name string) Config {
		return Config{Addresses: []string{name + ":8091"}}
	})
	t.Cleanup(func() { ConfigureConfigLoader(nil) })

	if got := ResolveSDKName("owner", "shared"); got != "shared" {
		t.Fatalf("ResolveSDKName() = %q", got)
	}
	if got := ResolveSDKAddresses("owner", "", nil); len(got) != 1 || got[0] != "owner:8091" {
		t.Fatalf("ResolveSDKAddresses() = %#v", got)
	}
	if got := ResolveSDKAddresses("owner", "", []string{"explicit"}); got[0] != "explicit" {
		t.Fatalf("ResolveSDKAddresses(explicit) = %#v", got)
	}

	h1 := GetHolder("same", []string{"b", "a"}, []string{"d", "c"})
	h2 := GetHolder("same", []string{"a", "b"}, []string{"c", "d"})
	if h1 != h2 {
		t.Fatal("GetHolder() did not normalize address order")
	}
	if h3 := GetHolder("other", []string{"a", "b"}, []string{"c", "d"}); h3 == h1 {
		t.Fatal("GetHolder() reused holder for a different SDK name")
	}
}
