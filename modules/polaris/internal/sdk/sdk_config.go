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

import "sync"

// Config is the config for the Polaris SDK.
type Config struct {
	Addresses     []string `mapstructure:"addresses"`
	ConfigAddress []string `mapstructure:"config_addresses"`
	Token         string   `mapstructure:"token"`
	ConfigFile    string   `mapstructure:"config_file"`
}

var (
	configMu     sync.RWMutex
	configLoader = func(string) Config { return Config{} }
)

// ConfigureConfigLoader replaces the SDK config loader used by shared holders.
func ConfigureConfigLoader(loader func(string) Config) {
	if loader == nil {
		loader = func(string) Config { return Config{} }
	}
	configMu.Lock()
	configLoader = loader
	configMu.Unlock()
}

// LoadSDKConfig loads the SDK config for the given name.
func LoadSDKConfig(name string) Config {
	configMu.RLock()
	loader := configLoader
	configMu.RUnlock()
	return loader(name)
}

// ResolveSDKName resolves the named SDK reference for one owner.
func ResolveSDKName(ownerName string, sdkName string) string {
	if sdkName != "" {
		return sdkName
	}
	return ownerName
}

// ResolveSDKAddresses resolves naming addresses for one SDK reference.
func ResolveSDKAddresses(ownerName string, sdkName string, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	return LoadSDKConfig(ResolveSDKName(ownerName, sdkName)).Addresses
}

// ResolveSDKConfigAddresses resolves config-center addresses for one SDK reference.
func ResolveSDKConfigAddresses(ownerName string, sdkName string, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	cfg := LoadSDKConfig(ResolveSDKName(ownerName, sdkName))
	return cfg.ConfigAddress
}
