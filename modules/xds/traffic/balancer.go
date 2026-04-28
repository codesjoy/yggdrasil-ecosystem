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

package traffic

import (
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"sync"
	"time"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

const name = "xds"

// BalancerProvider returns the xDS v3 client balancer provider.
func BalancerProvider() balancer.Provider {
	return balancer.NewProvider(name, newXdsBalancer)
}

type xdsBalancer struct {
	cli balancer.Client

	mu               sync.RWMutex
	remotesClient    map[string]remote.Client
	vhosts           []*xdsresource.VirtualHost
	clusterPolicies  map[string]clusterPolicy
	endpoints        map[string][]*weightedEndpoint
	circuitBreakers  map[string]*CircuitBreaker
	outlierDetectors map[string]*OutlierDetector
	rateLimiters     map[string]*RateLimiter
	inFlight         map[string]*int32
	rng              *mrand.Rand
}

func newXdsBalancer(_ string, _ string, cli balancer.Client) (balancer.Balancer, error) {
	//nolint:gosec // G404: Weak random is acceptable for load balancing selection (non-cryptographic use)
	return &xdsBalancer{
		cli:              cli,
		remotesClient:    make(map[string]remote.Client),
		vhosts:           make([]*xdsresource.VirtualHost, 0),
		clusterPolicies:  make(map[string]clusterPolicy),
		endpoints:        make(map[string][]*weightedEndpoint),
		circuitBreakers:  make(map[string]*CircuitBreaker),
		outlierDetectors: make(map[string]*OutlierDetector),
		rateLimiters:     make(map[string]*RateLimiter),
		inFlight:         make(map[string]*int32),
		rng:              mrand.New(mrand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (b *xdsBalancer) UpdateState(state resolver.State) {
	endpoints := state.GetEndpoints()
	if endpoints == nil {
		return
	}

	b.mu.Lock()
	staleClients := b.refreshRemoteClientsLocked(endpoints)
	b.applyAttributesLocked(state.GetAttributes())
	b.rebuildEndpointsLocked(endpoints)
	picker := b.buildPicker()
	b.mu.Unlock()

	b.cli.UpdateState(balancer.State{Picker: picker})
	b.closeRemoteClients(staleClients)
}

func (b *xdsBalancer) refreshRemoteClientsLocked(endpoints []resolver.Endpoint) []remote.Client {
	nextClients := make(map[string]remote.Client, len(endpoints))
	for _, endpoint := range endpoints {
		endpointKey := endpoint.GetAddress()
		if endpointKey == "" {
			endpointKey = endpoint.Name()
		}

		if client, ok := b.remotesClient[endpointKey]; ok {
			nextClients[endpointKey] = client
			continue
		}

		client, err := b.cli.NewRemoteClient(
			endpoint,
			balancer.NewRemoteClientOptions{StateListener: b.UpdateRemoteClientState},
		)
		if err != nil {
			slog.Error("new remote client error", slog.Any("error", err))
			continue
		}
		if client != nil {
			nextClients[endpointKey] = client
			client.Connect()
		}
	}

	staleClients := make([]remote.Client, 0)
	for key, client := range b.remotesClient {
		if _, ok := nextClients[key]; !ok {
			staleClients = append(staleClients, client)
		}
	}

	b.remotesClient = nextClients
	return staleClients
}

func (b *xdsBalancer) applyAttributesLocked(attributes map[string]any) {
	if vhosts, ok := attributes[xdsresource.AttributeRoutes].([]*xdsresource.VirtualHost); ok {
		b.vhosts = vhosts
	} else {
		b.vhosts = nil
	}

	clusters, ok := attributes[xdsresource.AttributeClusters].(map[string]clusterPolicy)
	if !ok {
		for _, detector := range b.outlierDetectors {
			detector.Stop()
		}
		for _, limiter := range b.rateLimiters {
			limiter.Stop()
		}
		b.clusterPolicies = make(map[string]clusterPolicy)
		b.circuitBreakers = make(map[string]*CircuitBreaker)
		b.outlierDetectors = make(map[string]*OutlierDetector)
		b.rateLimiters = make(map[string]*RateLimiter)
		return
	}

	for _, detector := range b.outlierDetectors {
		detector.Stop()
	}
	for _, limiter := range b.rateLimiters {
		limiter.Stop()
	}

	nextPolicies := make(map[string]clusterPolicy, len(clusters))
	nextCircuitBreakers := make(map[string]*CircuitBreaker, len(clusters))
	nextOutlierDetectors := make(map[string]*OutlierDetector, len(clusters))
	nextRateLimiters := make(map[string]*RateLimiter, len(clusters))

	for clusterName, policy := range clusters {
		nextPolicies[clusterName] = policy

		if policy.CircuitBreaker != nil {
			nextCircuitBreakers[clusterName] = NewCircuitBreaker(policy.CircuitBreaker)
		}
		if policy.OutlierDetection != nil {
			detector := NewOutlierDetector(policy.OutlierDetection)
			nextOutlierDetectors[clusterName] = detector
			detector.Start()
		}
		if policy.RateLimiter != nil {
			nextRateLimiters[clusterName] = NewRateLimiter(policy.RateLimiter)
		}
	}

	b.clusterPolicies = nextPolicies
	b.circuitBreakers = nextCircuitBreakers
	b.outlierDetectors = nextOutlierDetectors
	b.rateLimiters = nextRateLimiters
}

func (b *xdsBalancer) rebuildEndpointsLocked(endpoints []resolver.Endpoint) {
	b.endpoints = make(map[string][]*weightedEndpoint)
	for _, endpoint := range endpoints {
		weighted, endpointKey, ok := b.buildWeightedEndpoint(endpoint)
		if !ok {
			continue
		}

		b.endpoints[weighted.Cluster] = append(b.endpoints[weighted.Cluster], weighted)

		if _, ok := b.inFlight[endpointKey]; !ok {
			value := int32(0)
			b.inFlight[endpointKey] = &value
		}
	}
}

func (b *xdsBalancer) buildWeightedEndpoint(
	endpoint resolver.Endpoint,
) (*weightedEndpoint, string, bool) {
	address := endpoint.GetAddress()
	if address == "" {
		return nil, "", false
	}

	host, port := splitEndpointAddress(address)
	weighted := &weightedEndpoint{
		Endpoint: xdsresource.Endpoint{
			Address: host,
			Port:    port,
		},
		Weight:   1,
		Priority: 0,
		Metadata: make(map[string]string),
	}

	attributes := endpoint.GetAttributes()
	cluster, ok := attributes[xdsresource.AttributeEndpointCluster].(string)
	if !ok || cluster == "" {
		return nil, "", false
	}
	weighted.Cluster = cluster
	if weight, ok := attributes[xdsresource.AttributeEndpointWeight].(uint32); ok {
		weighted.Weight = weight
	}
	if priority, ok := attributes[xdsresource.AttributeEndpointPriority].(uint32); ok {
		weighted.Priority = priority
	}
	if metadata, ok := attributes[xdsresource.AttributeEndpointMetadata].(map[string]string); ok {
		weighted.Metadata = metadata
	}

	return weighted, address, true
}

func splitEndpointAddress(address string) (string, int) {
	host := address
	port := 0
	for idx := len(address) - 1; idx >= 0; idx-- {
		if address[idx] != ':' {
			continue
		}
		host = address[:idx]
		_, _ = fmt.Sscanf(address[idx+1:], "%d", &port)
		break
	}
	return host, port
}

func (b *xdsBalancer) closeRemoteClients(clients []remote.Client) {
	for _, client := range clients {
		if err := client.Close(); err != nil {
			slog.Warn(
				"remove remote client error",
				slog.String("name", name),
				slog.Any("error", err),
			)
		}
	}
}

func (b *xdsBalancer) UpdateRemoteClientState(_ remote.ClientState) {
	b.mu.RLock()
	picker := b.buildPicker()
	b.mu.RUnlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
}

func (b *xdsBalancer) Close() error {
	b.mu.Lock()
	clients := make([]remote.Client, 0, len(b.remotesClient))
	for _, client := range b.remotesClient {
		clients = append(clients, client)
	}
	for _, detector := range b.outlierDetectors {
		detector.Stop()
	}
	for _, limiter := range b.rateLimiters {
		limiter.Stop()
	}
	b.remotesClient = nil
	picker := b.buildPicker()
	b.mu.Unlock()

	b.cli.UpdateState(balancer.State{Picker: picker})

	var multiErr error
	for _, client := range clients {
		if err := client.Close(); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

func (b *xdsBalancer) Type() string {
	return name
}

// BalancerStats contains statistics for the xDS balancer.
type BalancerStats struct {
	CircuitBreakers  map[string]CircuitBreakerStats
	OutlierDetectors map[string]map[string]any
	RateLimiters     map[string]RateLimiterStats
}

func (b *xdsBalancer) GetStats() BalancerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	circuitBreakerStats := make(map[string]CircuitBreakerStats)
	for name, circuitBreaker := range b.circuitBreakers {
		circuitBreakerStats[name] = circuitBreaker.GetStats()
	}

	outlierDetectorStats := make(map[string]map[string]interface{})
	for name, detector := range b.outlierDetectors {
		outlierDetectorStats[name] = detector.GetStats()
	}

	rateLimiterStats := make(map[string]RateLimiterStats)
	for name, limiter := range b.rateLimiters {
		rateLimiterStats[name] = limiter.GetStats()
	}

	return BalancerStats{
		CircuitBreakers:  circuitBreakerStats,
		OutlierDetectors: outlierDetectorStats,
		RateLimiters:     rateLimiterStats,
	}
}

func (b *xdsBalancer) buildPicker() *xdsPicker {
	return &xdsPicker{balancer: b}
}
