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

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	polarisgo "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/common"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3/internal/sdk"
	yregistry "github.com/codesjoy/yggdrasil/v3/discovery/registry"
)

var newRegistryProviderAPI = func(name string, cfg RegistryConfig) (sdk.ProviderAPI, error) {
	sdkName := sdk.ResolveSDKName(name, cfg.SDK)
	addresses := sdk.ResolveSDKAddresses(name, cfg.SDK, cfg.Addresses)
	return sdk.GetHolder(sdkName, addresses, nil).Provider()
}

// RegistryConfig is the config for the Polaris registry.
type RegistryConfig struct {
	Addresses     []string      `mapstructure:"addresses"`
	SDK           string        `mapstructure:"sdk"`
	Namespace     string        `mapstructure:"namespace"`
	ServiceToken  string        `mapstructure:"service_token"`
	TTL           time.Duration `mapstructure:"ttl"`
	AutoHeartbeat bool          `mapstructure:"auto_heartbeat"`
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryCount    int           `mapstructure:"retry_count"`
}

// Registry is the Polaris registry.
type Registry struct {
	cfg          RegistryConfig
	api          sdk.ProviderAPI
	initErr      error
	instanceName string

	mu         sync.Mutex
	registered map[string]registeredInstance
}

type registeredInstance struct {
	service    string
	namespace  string
	host       string
	port       int
	instanceID string
}

// NewRegistry creates a new Polaris registry.
func NewRegistry(name string, cfg RegistryConfig) (*Registry, error) {
	sdkName := sdk.ResolveSDKName(name, cfg.SDK)
	api, err := newRegistryProviderAPI(name, cfg)
	if err != nil {
		return nil, err
	}
	return &Registry{
		cfg:          cfg,
		api:          api,
		instanceName: sdkName,
		registered:   map[string]registeredInstance{},
	}, nil
}

// NewRegistryWithError creates a new Polaris registry with the given error.
func NewRegistryWithError(cfg RegistryConfig, initErr error) *Registry {
	return &Registry{
		cfg:          cfg,
		initErr:      initErr,
		instanceName: sdk.ResolveSDKName("default", cfg.SDK),
		registered:   map[string]registeredInstance{},
	}
}

// NewRegistryFromMap creates a registry from a v3 registry provider config map.
func NewRegistryFromMap(cfgMap map[string]any) (yregistry.Registry, error) {
	var cfg RegistryConfig
	if err := decodeMap(cfgMap, &cfg); err != nil {
		return nil, err
	}
	return NewRegistry("default", cfg)
}

// RegistryProvider returns the Polaris v3 registry provider.
func RegistryProvider() yregistry.Provider {
	return yregistry.NewProvider("polaris", NewRegistryFromMap)
}

// Type returns the type of the registry.
func (r *Registry) Type() string { return "polaris" }

// Register registers the instance with the Polaris registry.
func (r *Registry) Register(ctx context.Context, inst yregistry.Instance) error {
	if r.initErr != nil {
		return r.initErr
	}

	serviceName := inst.Name()
	if serviceName == "" {
		return errors.New("empty instance name")
	}
	namespace := r.cfg.Namespace
	if namespace == "" {
		namespace = inst.Namespace()
	}
	if namespace == "" {
		namespace = "default"
	}

	for _, ep := range inst.Endpoints() {
		host, port, err := common.SplitHostPort(ep.Address())
		if err != nil {
			return err
		}

		req := &polarisgo.InstanceRegisterRequest{}
		req.Service = serviceName
		req.Namespace = namespace
		req.Host = host
		req.Port = port
		req.ServiceToken = r.cfg.ServiceToken
		req.Metadata = common.MergeStringMap(
			inst.Metadata(),
			ep.Metadata(),
			map[string]string{"protocol": ep.Scheme()},
		)
		if inst.Version() != "" {
			version := inst.Version()
			req.Version = &version
		}
		if inst.Region() != "" || inst.Zone() != "" || inst.Campus() != "" {
			req.Location = &model.Location{
				Region: inst.Region(),
				Zone:   inst.Zone(),
				Campus: inst.Campus(),
			}
		}
		if d := common.EffectiveTimeout(ctx, r.cfg.Timeout); d > 0 {
			req.Timeout = &d
		}
		if r.cfg.RetryCount > 0 {
			retry := r.cfg.RetryCount
			req.RetryCount = &retry
		}
		if r.cfg.TTL > 0 {
			ttlSeconds := int(r.cfg.TTL.Seconds())
			req.TTL = &ttlSeconds
			req.AutoHeartbeat = r.cfg.AutoHeartbeat
		}

		protocol := ep.Scheme()
		if protocol != "" {
			req.Protocol = &protocol
		}

		resp, err := r.api.RegisterInstance(req)
		if err != nil {
			return err
		}

		key := r.instanceName + "|" + namespace + "|" + serviceName + "|" + ep.Scheme() + "|" + ep.Address()
		r.mu.Lock()
		r.registered[key] = registeredInstance{
			service:    serviceName,
			namespace:  namespace,
			host:       host,
			port:       port,
			instanceID: resp.InstanceID,
		}
		r.mu.Unlock()
	}
	return nil
}

// Deregister deregisters the instance with the Polaris registry.
func (r *Registry) Deregister(ctx context.Context, inst yregistry.Instance) error {
	if r.initErr != nil {
		return r.initErr
	}
	serviceName := inst.Name()
	if serviceName == "" {
		return errors.New("empty instance name")
	}
	namespace := r.cfg.Namespace
	if namespace == "" {
		namespace = inst.Namespace()
	}
	if namespace == "" {
		namespace = "default"
	}

	var firstErr error
	for _, ep := range inst.Endpoints() {
		key := r.instanceName + "|" + namespace + "|" + serviceName + "|" + ep.Scheme() + "|" + ep.Address()
		r.mu.Lock()
		reg, ok := r.registered[key]
		if ok {
			delete(r.registered, key)
		}
		r.mu.Unlock()
		if !ok {
			continue
		}

		req := &polarisgo.InstanceDeRegisterRequest{}
		req.Service = serviceName
		req.Namespace = namespace
		req.ServiceToken = r.cfg.ServiceToken
		req.InstanceID = reg.instanceID
		req.Host = reg.host
		req.Port = reg.port
		if d := common.EffectiveTimeout(ctx, r.cfg.Timeout); d > 0 {
			req.Timeout = &d
		}
		if r.cfg.RetryCount > 0 {
			retry := r.cfg.RetryCount
			req.RetryCount = &retry
		}

		if err := r.api.Deregister(req); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
