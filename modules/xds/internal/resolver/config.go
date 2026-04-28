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
	"time"

	"github.com/mitchellh/mapstructure"
)

// ResolverConfig holds the configuration for xDS resolver.
type ResolverConfig struct {
	Server     ServerConfig      `mapstructure:"server"`
	Node       NodeConfig        `mapstructure:"node"`
	ServiceMap map[string]string `mapstructure:"service_map"`
	Protocol   string            `mapstructure:"protocol"`
	MaxRetries int               `mapstructure:"max_retries"`
	Health     HealthConfig      `mapstructure:"health"`
	Retry      RetryConfig       `mapstructure:"retry"`
}

// ServerConfig holds the xDS server connection configuration.
type ServerConfig struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
	TLS     TLSConfig     `mapstructure:"tls"`
}

// TLSConfig holds TLS configuration for xDS server connection.
type TLSConfig struct {
	Enable   bool   `mapstructure:"enable"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

// NodeConfig holds the node identification information.
type NodeConfig struct {
	ID       string            `mapstructure:"id"`
	Cluster  string            `mapstructure:"cluster"`
	Metadata map[string]string `mapstructure:"metadata"`
	Locality *Locality         `mapstructure:"locality"`
}

// Locality holds the node locality information.
type Locality struct {
	Region  string `mapstructure:"region"`
	Zone    string `mapstructure:"zone"`
	SubZone string `mapstructure:"sub_zone"`
}

// HealthConfig holds health check configuration.
type HealthConfig struct {
	HealthyOnly    bool     `mapstructure:"healthy_only"`
	IgnoreStatuses []string `mapstructure:"ignore_statuses"`
}

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxRetries int           `mapstructure:"max_retries"`
	Backoff    time.Duration `mapstructure:"backoff"`
}

// ResolverConfigLoader loads resolver config for a named resolver.
type ResolverConfigLoader func(name string) ResolverConfig

func defaultResolverConfig() ResolverConfig {
	return ResolverConfig{
		Server: ServerConfig{
			Address: "127.0.0.1:18000",
			Timeout: 5 * time.Second,
			TLS: TLSConfig{
				Enable: false,
			},
		},
		Node: NodeConfig{
			ID:       "yggdrasil-node",
			Cluster:  "yggdrasil-cluster",
			Metadata: map[string]string{},
		},
		ServiceMap: map[string]string{},
		Protocol:   "grpc",
		Health: HealthConfig{
			HealthyOnly:    true,
			IgnoreStatuses: []string{},
		},
		Retry: RetryConfig{
			MaxRetries: 3,
			Backoff:    100 * time.Millisecond,
		},
	}
}

// DefaultResolverConfig returns the default xDS resolver configuration.
func DefaultResolverConfig() ResolverConfig {
	return defaultResolverConfig()
}

// DecodeConfig decodes an xDS resolver config map with defaults.
func DecodeConfig(input map[string]any) ResolverConfig {
	cfg := defaultResolverConfig()
	if len(input) == 0 {
		return cfg
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result: &cfg,
	})
	if err != nil {
		return cfg
	}
	if err := decoder.Decode(input); err != nil {
		return defaultResolverConfig()
	}
	return cfg
}
