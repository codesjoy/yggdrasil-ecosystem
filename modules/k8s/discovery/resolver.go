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
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/k8s/v3/internal/kube"
	yresolver "github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type resolverMode string

const (
	modeEndpointSlice resolverMode = "endpointslice"
)

const (
	resolverType = "kubernetes"
)

// BackoffConfig configures resolver watch retry timing.
type BackoffConfig struct {
	BaseDelay  time.Duration `mapstructure:"base_delay"`
	Multiplier float64       `mapstructure:"multiplier"`
	Jitter     float64       `mapstructure:"jitter"`
	MaxDelay   time.Duration `mapstructure:"max_delay"`
}

// ResolverConfig configures the Kubernetes resolver.
type ResolverConfig struct {
	Namespace          string            `mapstructure:"namespace"`
	Mode               string            `mapstructure:"mode"`
	PortName           string            `mapstructure:"port_name"`
	Port               int32             `mapstructure:"port"`
	Protocol           string            `mapstructure:"protocol"`
	Kubeconfig         string            `mapstructure:"kubeconfig"`
	ResyncPeriod       time.Duration     `mapstructure:"resync_period"`
	Timeout            time.Duration     `mapstructure:"timeout"`
	Backoff            BackoffConfig     `mapstructure:"backoff"`
	EndpointAttributes map[string]string `mapstructure:"endpoint_attributes"`
}

// ResolverConfigLoader loads resolver config for a named resolver.
type ResolverConfigLoader func(name string) ResolverConfig

// NormalizeConfig fills in default resolver settings.
func NormalizeConfig(cfg ResolverConfig) ResolverConfig {
	if cfg.Namespace == "" {
		if namespace := os.Getenv("KUBERNETES_NAMESPACE"); namespace != "" {
			cfg.Namespace = namespace
		} else {
			cfg.Namespace = "default"
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = string(modeEndpointSlice)
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "grpc"
	}
	if cfg.Backoff.BaseDelay == 0 {
		cfg.Backoff.BaseDelay = time.Second
	}
	if cfg.Backoff.Multiplier == 0 {
		cfg.Backoff.Multiplier = 1.6
	}
	if cfg.Backoff.Jitter == 0 {
		cfg.Backoff.Jitter = 0.2
	}
	if cfg.Backoff.MaxDelay == 0 {
		cfg.Backoff.MaxDelay = 30 * time.Second
	}
	return cfg
}

// Resolver is the Kubernetes resolver.
type Resolver struct {
	name            string
	cfg             ResolverConfig
	bo              *backoff
	initErr         error
	clientForConfig func(string) (kubernetes.Interface, error)

	mu       sync.Mutex
	watchers map[string]map[yresolver.Client]struct{}
	cancels  map[string]context.CancelFunc
	states   map[string]yresolver.State
}

// NewResolver creates a new Kubernetes resolver.
func NewResolver(name string, cfg ResolverConfig) (*Resolver, error) {
	cfg = NormalizeConfig(cfg)
	factory := kube.NewClientFactory()
	return &Resolver{
		name:            name,
		cfg:             cfg,
		bo:              newBackoff(cfg.Backoff),
		clientForConfig: factory.Client,
		watchers:        map[string]map[yresolver.Client]struct{}{},
		cancels:         map[string]context.CancelFunc{},
		states:          map[string]yresolver.State{},
	}, nil
}

// ResolverProvider returns the Kubernetes v3 resolver provider.
func ResolverProvider(load ResolverConfigLoader) yresolver.Provider {
	if load == nil {
		load = func(string) ResolverConfig { return ResolverConfig{} }
	}
	return yresolver.NewProvider(resolverType, func(name string) (yresolver.Resolver, error) {
		return NewResolver(name, load(name))
	})
}

// NewResolverWithError creates a new resolver that always returns initErr.
func NewResolverWithError(name string, cfg ResolverConfig, initErr error) *Resolver {
	cfg = NormalizeConfig(cfg)
	factory := kube.NewClientFactory()
	return &Resolver{
		name:            name,
		cfg:             cfg,
		bo:              newBackoff(cfg.Backoff),
		initErr:         initErr,
		clientForConfig: factory.Client,
		watchers:        map[string]map[yresolver.Client]struct{}{},
		cancels:         map[string]context.CancelFunc{},
		states:          map[string]yresolver.State{},
	}
}

// Type returns the resolver type.
func (r *Resolver) Type() string { return resolverType }

// AddWatch registers a watch for one service name.
func (r *Resolver) AddWatch(appName string, watcher yresolver.Client) error {
	if r.initErr != nil {
		return r.initErr
	}
	if appName == "" {
		return errors.New("empty app name")
	}

	r.mu.Lock()
	ws := r.watchers[appName]
	if ws == nil {
		ws = map[yresolver.Client]struct{}{}
		r.watchers[appName] = ws
	}
	ws[watcher] = struct{}{}
	_, running := r.cancels[appName]
	if !running {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancels[appName] = cancel
		go r.watchLoop(ctx, appName)
	}
	r.mu.Unlock()

	if state, ok := r.getState(appName); ok {
		watcher.UpdateState(state)
	}
	return nil
}

// DelWatch removes a watch for one service name.
func (r *Resolver) DelWatch(appName string, watcher yresolver.Client) error {
	if r.initErr != nil {
		return r.initErr
	}

	r.mu.Lock()
	ws := r.watchers[appName]
	if ws != nil {
		delete(ws, watcher)
		if len(ws) == 0 {
			delete(r.watchers, appName)
			if cancel, ok := r.cancels[appName]; ok {
				delete(r.cancels, appName)
				cancel()
			}
			delete(r.states, appName)
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Resolver) getState(appName string) (yresolver.State, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[appName]
	return state, ok
}

func (r *Resolver) setState(appName string, state yresolver.State) {
	r.mu.Lock()
	r.states[appName] = state
	r.mu.Unlock()
}

func (r *Resolver) snapshotWatchers(appName string) []yresolver.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := r.watchers[appName]
	out := make([]yresolver.Client, 0, len(ws))
	for watcher := range ws {
		out = append(out, watcher)
	}
	return out
}

func (r *Resolver) notify(appName string, state yresolver.State) {
	for _, watcher := range r.snapshotWatchers(appName) {
		watcher.UpdateState(state)
	}
}

func (r *Resolver) watchLoop(ctx context.Context, appName string) {
	retries := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := r.watch(ctx, appName); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
		}

		retries++
		timer := time.NewTimer(r.bo.Backoff(retries))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (r *Resolver) watch(ctx context.Context, appName string) error {
	client, err := r.clientForConfig(r.cfg.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if r.cfg.Mode == string(modeEndpointSlice) {
		err = r.watchEndpointSlice(ctx, client, appName)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
	}
	return r.watchEndpoints(ctx, client, appName)
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters.
func (r *Resolver) watchEndpoints(
	ctx context.Context,
	client kubernetes.Interface,
	appName string,
) error {
	listOpts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", appName),
	}
	for {
		w, err := client.CoreV1().Endpoints(r.cfg.Namespace).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to watch endpoints: %w", err)
		}

		state, err := r.listEndpoints(ctx, client, appName)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to list endpoints: %w", err)
		}
		r.setState(appName, state)
		r.notify(appName, state)

		for event := range w.ResultChan() {
			if event.Type == watch.Deleted {
				emptyState := yresolver.BaseState{Endpoints: []yresolver.Endpoint{}}
				r.setState(appName, emptyState)
				r.notify(appName, emptyState)
				continue
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}

			state, err := r.listEndpoints(ctx, client, appName)
			if err != nil {
				w.Stop()
				return fmt.Errorf("failed to list endpoints after event: %w", err)
			}
			r.setState(appName, state)
			r.notify(appName, state)
		}
		w.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters.
func (r *Resolver) listEndpoints(
	ctx context.Context,
	client kubernetes.Interface,
	appName string,
) (yresolver.State, error) {
	endpoints, err := client.CoreV1().
		Endpoints(r.cfg.Namespace).
		Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %w", err)
	}
	return r.endpointsToState(endpoints), nil
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters.
func (r *Resolver) endpointsToState(endpoints *corev1.Endpoints) yresolver.State {
	baseState := yresolver.BaseState{
		Attributes: map[string]any{
			"service":   endpoints.Name,
			"namespace": endpoints.Namespace,
		},
		Endpoints: []yresolver.Endpoint{},
	}

	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP == "" {
				continue
			}
			port := r.selectPort(subset.Ports)
			if port == nil {
				continue
			}
			endpointAddr := net.JoinHostPort(addr.IP, strconv.Itoa(int(port.Port)))
			attrs := map[string]any{
				"hostname": addr.Hostname,
				"nodeName": addr.NodeName,
			}
			if addr.TargetRef != nil {
				attrs["targetRefKind"] = addr.TargetRef.Kind
				attrs["targetRefName"] = addr.TargetRef.Name
			}
			for key, value := range r.cfg.EndpointAttributes {
				attrs[key] = value
			}
			baseState.Endpoints = append(baseState.Endpoints, yresolver.BaseEndpoint{
				Address:    endpointAddr,
				Protocol:   r.cfg.Protocol,
				Attributes: attrs,
			})
		}
	}

	return baseState
}

//nolint:staticcheck // SA1019: corev1.EndpointPort is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters.
func (r *Resolver) selectPort(ports []corev1.EndpointPort) *corev1.EndpointPort {
	if len(ports) == 0 {
		return nil
	}
	if r.cfg.PortName != "" {
		for i := range ports {
			if ports[i].Name == r.cfg.PortName {
				return &ports[i]
			}
		}
	}
	if r.cfg.Port != 0 {
		for i := range ports {
			if ports[i].Port == r.cfg.Port {
				return &ports[i]
			}
		}
	}
	return &ports[0]
}

func (r *Resolver) watchEndpointSlice(
	ctx context.Context,
	client kubernetes.Interface,
	appName string,
) error {
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", appName),
	}
	for {
		w, err := client.DiscoveryV1().EndpointSlices(r.cfg.Namespace).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to watch endpointslices: %w", err)
		}

		state, err := r.listEndpointSlice(ctx, client, appName)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to list endpointslices: %w", err)
		}
		r.setState(appName, state)
		r.notify(appName, state)

		for event := range w.ResultChan() {
			if event.Type == watch.Deleted {
				emptyState := yresolver.BaseState{Endpoints: []yresolver.Endpoint{}}
				r.setState(appName, emptyState)
				r.notify(appName, emptyState)
				continue
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}

			state, err := r.listEndpointSlice(ctx, client, appName)
			if err != nil {
				w.Stop()
				return fmt.Errorf("failed to list endpointslices after event: %w", err)
			}
			r.setState(appName, state)
			r.notify(appName, state)
		}
		w.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (r *Resolver) listEndpointSlice(
	ctx context.Context,
	client kubernetes.Interface,
	appName string,
) (yresolver.State, error) {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", appName),
	}
	slices, err := client.DiscoveryV1().EndpointSlices(r.cfg.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list endpointslices: %w", err)
	}
	return r.endpointSlicesToState(slices.Items), nil
}

func (r *Resolver) endpointSlicesToState(slices []discoveryv1.EndpointSlice) yresolver.State {
	baseState := yresolver.BaseState{
		Attributes: map[string]any{},
		Endpoints:  []yresolver.Endpoint{},
	}

	for _, slice := range slices {
		for _, port := range slice.Ports {
			portNumber, ok := r.slicePortValue(port)
			if !ok {
				continue
			}
			for _, endpoint := range slice.Endpoints {
				if len(endpoint.Addresses) == 0 {
					continue
				}
				for _, addr := range endpoint.Addresses {
					if addr == "" {
						continue
					}
					endpointAddr := net.JoinHostPort(addr, strconv.Itoa(int(portNumber)))
					attrs := map[string]any{}
					if endpoint.NodeName != nil {
						attrs["nodeName"] = *endpoint.NodeName
					}
					if endpoint.Zone != nil {
						attrs["zone"] = *endpoint.Zone
					}
					if endpoint.TargetRef != nil {
						attrs["targetRefKind"] = endpoint.TargetRef.Kind
						attrs["targetRefName"] = endpoint.TargetRef.Name
					}
					for key, value := range r.cfg.EndpointAttributes {
						attrs[key] = value
					}
					baseState.Endpoints = append(baseState.Endpoints, yresolver.BaseEndpoint{
						Address:    endpointAddr,
						Protocol:   r.cfg.Protocol,
						Attributes: attrs,
					})
				}
			}
		}
	}

	return baseState
}

func (r *Resolver) slicePortValue(port discoveryv1.EndpointPort) (int32, bool) {
	if port.Port == nil {
		return 0, false
	}
	if r.cfg.PortName != "" {
		if port.Name == nil || *port.Name != r.cfg.PortName {
			return 0, false
		}
	}
	if r.cfg.Port != 0 && *port.Port != r.cfg.Port {
		return 0, false
	}
	return *port.Port, true
}
