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

package discovery

import internalresolver "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resolver"

type (
	// ResolverConfig holds the configuration for xDS resolver.
	ResolverConfig = internalresolver.Config
	// ServerConfig holds the xDS server connection configuration.
	ServerConfig = internalresolver.ServerConfig
	// TLSConfig holds TLS configuration for xDS server connection.
	TLSConfig = internalresolver.TLSConfig
	// NodeConfig holds the node identification information.
	NodeConfig = internalresolver.NodeConfig
	// Locality holds the node locality information.
	Locality = internalresolver.Locality
	// HealthConfig holds health check configuration.
	HealthConfig = internalresolver.HealthConfig
	// RetryConfig holds retry configuration.
	RetryConfig = internalresolver.RetryConfig
	// ResolverConfigLoader loads resolver config for a named resolver.
	ResolverConfigLoader = internalresolver.ConfigLoader
)

// DefaultResolverConfig returns the default xDS resolver configuration.
func DefaultResolverConfig() ResolverConfig {
	return internalresolver.DefaultResolverConfig()
}
