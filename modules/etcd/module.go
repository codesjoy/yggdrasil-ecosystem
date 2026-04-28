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

package etcd

import (
	"context"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/configsource"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/discovery"
	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/mitchellh/mapstructure"
)

const moduleName = "etcd"

type etcdModule struct {
	mu       sync.RWMutex
	settings settings
}

type settings struct {
	Etcd struct {
		Clients map[string]internalclient.Config `mapstructure:"clients"`
	} `mapstructure:"etcd"`
	Discovery struct {
		Resolvers map[string]struct {
			Type   string         `mapstructure:"type"`
			Config map[string]any `mapstructure:"config"`
		} `mapstructure:"resolvers"`
	} `mapstructure:"discovery"`
}

// Module returns the Yggdrasil v3 etcd capability module.
func Module() module.Module {
	m := &etcdModule{}
	internalclient.ConfigureConfigLoader(m.clientConfig)
	return m
}

// WithModule registers the etcd capability module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

// NewConfigSource creates a programmatic etcd-backed config source.
func NewConfigSource(cfg ConfigSourceConfig) (source.Source, error) {
	return configsource.NewConfigSource(cfg)
}

// WithConfigSource registers one programmatic etcd-backed config source layer.
func WithConfigSource(name string, cfg ConfigSourceConfig) yggdrasil.Option {
	src, err := NewConfigSource(cfg)
	if err != nil {
		src = invalidSource{
			name: fallbackSourceName(name, cfg.Name, cfg.Key, cfg.Prefix),
			err:  err,
		}
	}
	return yggdrasil.WithConfigSource(
		fallbackSourceName(name, src.Name()),
		config.PriorityRemote,
		src,
	)
}

type invalidSource struct {
	name string
	err  error
}

func (s invalidSource) Kind() string { return configsource.Kind }

func (s invalidSource) Name() string { return s.name }

func (s invalidSource) Read() (source.Data, error) { return nil, s.err }

func (s invalidSource) Close() error { return nil }

func (m *etcdModule) Name() string { return moduleName }

func (m *etcdModule) ConfigPath() string { return "yggdrasil" }

func (m *etcdModule) Init(_ context.Context, view config.View) error {
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

func (m *etcdModule) Capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideNamed(
			capabilities.RegistryProviderSpec,
			moduleName,
			discovery.RegistryProvider(),
		),
		capabilities.ProvideNamed(
			capabilities.ResolverProviderSpec,
			moduleName,
			discovery.ResolverProvider(m.resolverConfig),
		),
	}
}

func (m *etcdModule) ConfigSourceBuilders() map[string]configchain.ContextBuilder {
	return map[string]configchain.ContextBuilder{
		configsource.Kind: m.configSourceBuilder,
	}
}

func (m *etcdModule) configSourceBuilder(
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

	var cfg ConfigSourceConfig
	if err := decodeMap(spec.Config, &cfg); err != nil {
		return nil, 0, err
	}
	priority, err := configchain.ParsePriority(spec.Priority, config.PriorityRemote)
	if err != nil {
		return nil, 0, err
	}
	src, err := NewConfigSource(cfg)
	if err != nil {
		return nil, 0, err
	}
	return src, priority, nil
}

func (m *etcdModule) clientConfig(name string) internalclient.Config {
	m.mu.RLock()
	cfg := m.settings.Etcd.Clients[internalclient.ResolveName(name)]
	m.mu.RUnlock()
	return cfg
}

func (m *etcdModule) resolverConfig(name string) discovery.ResolverConfig {
	m.mu.RLock()
	spec := m.settings.Discovery.Resolvers[name]
	m.mu.RUnlock()

	var cfg discovery.ResolverConfig
	_ = decodeMap(spec.Config, &cfg)
	return discovery.NormalizeConfig(cfg)
}

func fallbackSourceName(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return moduleName
}

func decodeMap(input map[string]any, target any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:  target,
		TagName: "mapstructure",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}
