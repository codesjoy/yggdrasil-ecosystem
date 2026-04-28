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
	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/configsource"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/discovery"
	internalclient "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3/internal/client"
)

const (
	// ConfigSourceKind is the declarative config source kind registered by the
	// etcd module.
	ConfigSourceKind = configsource.Kind
	// ConfigSourceModeBlob loads one full document from a single etcd key.
	ConfigSourceModeBlob = configsource.ModeBlob
	// ConfigSourceModeKV loads one structured map from a key prefix.
	ConfigSourceModeKV = configsource.ModeKV
)

// ClientConfig configures one named etcd client under `yggdrasil.etcd.clients`.
type ClientConfig = internalclient.Config

// ConfigSourceConfig configures one etcd-backed config source.
type ConfigSourceConfig = configsource.Config

// RegistryConfig configures the etcd registry provider.
type RegistryConfig = discovery.RegistryConfig

// ResolverConfig configures the etcd resolver provider.
type ResolverConfig = discovery.ResolverConfig
