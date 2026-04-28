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

package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/configsource"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/discovery"
	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/mitchellh/mapstructure"
)

const (
	moduleName   = "k8s"
	resolverType = "kubernetes"
)

// ConfigSourceConfig is the config for Kubernetes ConfigMap/Secret sources.
type ConfigSourceConfig = configsource.Config

// ResolverConfig is the config for the Kubernetes resolver.
type ResolverConfig = discovery.ResolverConfig

// BackoffConfig is the backoff config used by the Kubernetes resolver.
type BackoffConfig = discovery.BackoffConfig

type k8sModule struct {
	mu       sync.RWMutex
	settings settings
}

type settings struct {
	Discovery struct {
		Resolvers map[string]struct {
			Type   string         `mapstructure:"type"`
			Config map[string]any `mapstructure:"config"`
		} `mapstructure:"resolvers"`
	} `mapstructure:"discovery"`
}

// Module returns the Yggdrasil v3 Kubernetes capability module.
func Module() module.Module {
	return &k8sModule{}
}

// WithModule registers the Kubernetes capability module on a Yggdrasil v3 app.
func WithModule() yggdrasil.Option {
	return yggdrasil.WithModules(Module())
}

// NewConfigMapSource creates a Kubernetes ConfigMap-backed config source.
func NewConfigMapSource(cfg ConfigSourceConfig) (source.Source, error) {
	return configsource.NewConfigMapSource(cfg)
}

// NewSecretSource creates a Kubernetes Secret-backed config source.
func NewSecretSource(cfg ConfigSourceConfig) (source.Source, error) {
	return configsource.NewSecretSource(cfg)
}

// WithConfigMapSource registers an explicit ConfigMap-backed config source.
func WithConfigMapSource(
	name string,
	priority config.Priority,
	cfg ConfigSourceConfig,
) yggdrasil.Option {
	src, err := NewConfigMapSource(cfg)
	if err != nil {
		src = invalidSource{
			kind: configsource.KindConfigMap,
			name: fallbackSourceName(name, cfg.Name),
			err:  err,
		}
	}
	return yggdrasil.WithConfigSource(fallbackSourceName(name, src.Name()), priority, src)
}

// WithSecretSource registers an explicit Secret-backed config source.
func WithSecretSource(
	name string,
	priority config.Priority,
	cfg ConfigSourceConfig,
) yggdrasil.Option {
	src, err := NewSecretSource(cfg)
	if err != nil {
		src = invalidSource{
			kind: configsource.KindSecret,
			name: fallbackSourceName(name, cfg.Name),
			err:  err,
		}
	}
	return yggdrasil.WithConfigSource(fallbackSourceName(name, src.Name()), priority, src)
}

type invalidSource struct {
	kind string
	name string
	err  error
}

func (s invalidSource) Kind() string { return s.kind }

func (s invalidSource) Name() string { return s.name }

func (s invalidSource) Read() (source.Data, error) { return nil, s.err }

func (s invalidSource) Close() error { return nil }

func (m *k8sModule) Name() string { return moduleName }

func (m *k8sModule) ConfigPath() string { return "yggdrasil" }

func (m *k8sModule) Init(_ context.Context, view config.View) error {
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

func (m *k8sModule) Capabilities() []module.Capability {
	return []module.Capability{
		capabilities.ProvideNamed(
			capabilities.ResolverProviderSpec,
			resolverType,
			discovery.ResolverProvider(m.resolverConfig),
		),
	}
}

func (m *k8sModule) ConfigSourceBuilders() map[string]configchain.ContextBuilder {
	return map[string]configchain.ContextBuilder{
		configsource.KindConfigMap: m.configMapSourceBuilder,
		configsource.KindSecret:    m.secretSourceBuilder,
	}
}

func (m *k8sModule) configMapSourceBuilder(
	_ configchain.BuildContext,
	spec configchain.SourceSpec,
) (source.Source, config.Priority, error) {
	return m.buildConfigSource(configsource.KindConfigMap, spec)
}

func (m *k8sModule) secretSourceBuilder(
	_ configchain.BuildContext,
	spec configchain.SourceSpec,
) (source.Source, config.Priority, error) {
	return m.buildConfigSource(configsource.KindSecret, spec)
}

func (m *k8sModule) buildConfigSource(
	kind string,
	spec configchain.SourceSpec,
) (source.Source, config.Priority, error) {
	var cfg ConfigSourceConfig
	if err := decodeMap(spec.Config, &cfg); err != nil {
		return nil, 0, err
	}
	priority, err := configchain.ParsePriority(spec.Priority, config.PriorityRemote)
	if err != nil {
		return nil, 0, err
	}
	switch kind {
	case configsource.KindConfigMap:
		src, err := NewConfigMapSource(cfg)
		return src, priority, err
	case configsource.KindSecret:
		src, err := NewSecretSource(cfg)
		return src, priority, err
	default:
		return nil, 0, fmt.Errorf("unsupported config source kind %q", kind)
	}
}

func (m *k8sModule) resolverConfig(name string) discovery.ResolverConfig {
	m.mu.RLock()
	spec := m.settings.Discovery.Resolvers[name]
	m.mu.RUnlock()

	var cfg discovery.ResolverConfig
	_ = decodeMap(spec.Config, &cfg)
	return discovery.NormalizeConfig(cfg)
}

func fallbackSourceName(name string, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
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
