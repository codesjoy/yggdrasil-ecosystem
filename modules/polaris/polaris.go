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
	"sync"

	"github.com/codesjoy/pkg/utils/xmap"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/configsource"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/discovery"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/sdk"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/traffic"
	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/mitchellh/mapstructure"
)

type polarisModule struct {
	mu       sync.RWMutex
	settings settings
}

type settings struct {
	Polaris struct {
		SDKs       map[string]sdk.Config `mapstructure:"sdks"`
		Governance struct {
			Defaults map[string]any            `mapstructure:"defaults"`
			Services map[string]map[string]any `mapstructure:"services"`
		} `mapstructure:"governance"`
	} `mapstructure:"polaris"`
	Discovery struct {
		Resolvers map[string]struct {
			Type   string         `mapstructure:"type"`
			Config map[string]any `mapstructure:"config"`
		} `mapstructure:"resolvers"`
	} `mapstructure:"discovery"`
	Balancers struct {
		Defaults map[string]struct {
			Type   string         `mapstructure:"type"`
			Config map[string]any `mapstructure:"config"`
		} `mapstructure:"defaults"`
		Services map[string]map[string]struct {
			Type   string         `mapstructure:"type"`
			Config map[string]any `mapstructure:"config"`
		} `mapstructure:"services"`
	} `mapstructure:"balancers"`
}

// Module returns the Yggdrasil v3 Polaris capability module.
func Module() module.Module {
	m := &polarisModule{}
	sdk.ConfigureConfigLoader(m.sdkConfig)
	return m
}

// WithModule registers the Polaris capability module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

// WithConfigSource registers a Polaris-backed configuration source layer.
func WithConfigSource(name string, cfg configsource.Config) yggdrasil.Option {
	src, err := configsource.NewConfigSource(cfg)
	if err != nil {
		src = invalidSource{name: name, err: err}
	}
	return yggdrasil.WithConfigSource(name, config.PriorityRemote, src)
}

type invalidSource struct {
	name string
	err  error
}

func (s invalidSource) Kind() string { return "polaris" }

func (s invalidSource) Name() string { return s.name }

func (s invalidSource) Read() (source.Data, error) { return nil, s.err }

func (s invalidSource) Close() error { return nil }

func (m *polarisModule) Name() string { return "polaris" }

func (m *polarisModule) ConfigPath() string { return "yggdrasil" }

func (m *polarisModule) ConfigSourceBuilders() map[string]configchain.ContextBuilder {
	return map[string]configchain.ContextBuilder{
		"polaris": m.configSourceBuilder,
	}
}

func (m *polarisModule) Init(_ context.Context, view config.View) error {
	var next settings
	if view.Exists() {
		if err := view.Decode(&next); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.settings = next
	m.mu.Unlock()
	return nil
}

func (m *polarisModule) Capabilities() []module.Capability {
	caps := []module.Capability{
		capabilities.ProvideNamed(
			capabilities.RegistryProviderSpec,
			"polaris",
			discovery.RegistryProvider(),
		),
		capabilities.ProvideNamed(
			capabilities.ResolverProviderSpec,
			"polaris",
			discovery.ResolverProvider(m.resolverConfig),
		),
		capabilities.ProvideNamed(
			capabilities.BalancerProviderSpec,
			"polaris",
			traffic.BalancerProvider(m.governanceConfig),
		),
	}
	for _, provider := range traffic.UnaryClientInterceptorProviders(m.governanceConfig) {
		caps = append(caps, capabilities.ProvideOrdered(
			capabilities.UnaryClientInterceptorSpec,
			provider.Name(),
			provider,
		))
	}
	return caps
}

func (m *polarisModule) configSourceBuilder(
	ctx configchain.BuildContext,
	spec configchain.SourceSpec,
) (source.Source, config.Priority, error) {
	var base settings
	if !ctx.Snapshot.Empty() {
		if err := ctx.Snapshot.Section("yggdrasil").Decode(&base); err != nil {
			return nil, 0, err
		}
		m.mu.Lock()
		m.settings = base
		m.mu.Unlock()
	}

	var cfg configsource.Config
	if err := decodeMap(spec.Config, &cfg); err != nil {
		return nil, 0, err
	}
	priority, err := configchain.ParsePriority(spec.Priority, config.PriorityRemote)
	if err != nil {
		return nil, 0, err
	}
	src, err := configsource.NewConfigSource(cfg)
	if err != nil {
		return nil, 0, err
	}
	return src, priority, nil
}

func (m *polarisModule) sdkConfig(name string) sdk.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.Polaris.SDKs[name]
}

func (m *polarisModule) resolverConfig(name string) discovery.ResolverConfig {
	m.mu.RLock()
	spec := m.settings.Discovery.Resolvers[name]
	m.mu.RUnlock()
	var cfg discovery.ResolverConfig
	_ = decodeMap(spec.Config, &cfg)
	return cfg
}

func (m *polarisModule) governanceConfig(serviceName string) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]any{}
	xmap.MergeStringMap(out, m.settings.Polaris.Governance.Defaults)
	if svc := m.settings.Polaris.Governance.Services[serviceName]; len(svc) > 0 {
		xmap.MergeStringMap(out, svc)
	}
	if spec, ok := m.settings.Balancers.Defaults["polaris"]; ok {
		xmap.MergeStringMap(out, spec.Config)
	}
	if svc := m.settings.Balancers.Services[serviceName]; svc != nil {
		if spec, ok := svc["polaris"]; ok {
			xmap.MergeStringMap(out, spec.Config)
		}
	}
	xmap.CoverInterfaceMapToStringMap(out)
	return out
}

func decodeMap(input map[string]any, target any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result: target,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}
