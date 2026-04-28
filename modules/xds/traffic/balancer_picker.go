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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

type xdsPicker struct {
	balancer *xdsBalancer
}

func (p *xdsPicker) Next(ri balancer.RPCInfo) (balancer.PickResult, error) {
	p.balancer.mu.RLock()
	defer p.balancer.mu.RUnlock()

	headers := requestHeaders(ri.Ctx)
	path := headers[":path"]
	if path == "" {
		path = ri.Method
	}

	cluster, circuitBreaker, rateLimiter := p.selectCluster(path, headers)
	if cluster == "" {
		return nil, balancer.ErrNoAvailableInstance
	}

	if rateLimiter != nil && !rateLimiter.Allow() {
		return nil, errRateLimitExceeded
	}
	if circuitBreaker != nil && !circuitBreaker.TryAcquire(ResourceRequest) {
		return nil, errors.New("circuit breaker open: max requests reached")
	}

	endpoint := p.balancer.selectEndpoint(cluster, p.balancer.outlierDetectors[cluster])
	if endpoint == nil {
		if circuitBreaker != nil {
			circuitBreaker.Release(ResourceRequest)
		}
		return nil, balancer.ErrNoAvailableInstance
	}

	endpointKey := endpointAddress(endpoint)
	client, ok := p.balancer.remotesClient[endpointKey]
	if !ok || client.State() != remote.Ready {
		if circuitBreaker != nil {
			circuitBreaker.Release(ResourceRequest)
		}
		return nil, balancer.ErrNoAvailableInstance
	}

	return &pickResult{
		endpoint:        client,
		ctx:             ri.Ctx,
		balancer:        p.balancer,
		inflightKey:     endpointKey,
		circuitBreaker:  circuitBreaker,
		rateLimiter:     rateLimiter,
		outlierDetector: p.balancer.outlierDetectors[cluster],
	}, nil
}

func requestHeaders(ctx context.Context) map[string]string {
	headers := make(map[string]string)
	md, ok := metadata.FromOutContext(ctx)
	if !ok {
		return headers
	}
	for key, values := range md {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

func (p *xdsPicker) selectCluster(
	path string,
	headers map[string]string,
) (string, *CircuitBreaker, *RateLimiter) {
	action := xdsresource.MatchRoute(p.balancer.vhosts, path, headers)
	if action == nil {
		return "", nil, nil
	}

	cluster := action.Cluster
	if action.WeightedClusters != nil && len(action.WeightedClusters.Clusters) > 0 {
		cluster = p.balancer.selectWeightedCluster(action.WeightedClusters)
	}
	if cluster == "" {
		return "", nil, nil
	}

	return cluster, p.balancer.circuitBreakers[cluster], p.balancer.rateLimiters[cluster]
}

func (b *xdsBalancer) selectWeightedCluster(weightedClusters *xdsresource.WeightedClusters) string {
	if weightedClusters.TotalWeight == 0 {
		if len(weightedClusters.Clusters) == 0 {
			return ""
		}
		return weightedClusters.Clusters[0].Name
	}

	randomWeight := b.rng.Uint32() % weightedClusters.TotalWeight
	accumulatedWeight := uint32(0)
	for _, cluster := range weightedClusters.Clusters {
		accumulatedWeight += cluster.Weight
		if randomWeight < accumulatedWeight {
			return cluster.Name
		}
	}
	return weightedClusters.Clusters[0].Name
}

func (b *xdsBalancer) selectEndpoint(cluster string, detector *OutlierDetector) *weightedEndpoint {
	endpoints, ok := b.endpoints[cluster]
	if !ok || len(endpoints) == 0 {
		return nil
	}

	healthyEndpoints := filterHealthyEndpoints(endpoints, detector)
	if len(healthyEndpoints) == 0 {
		return nil
	}

	priorityGroups := make(map[uint32][]*weightedEndpoint)
	for _, endpoint := range healthyEndpoints {
		priorityGroups[endpoint.Priority] = append(priorityGroups[endpoint.Priority], endpoint)
	}

	policy, ok := b.clusterPolicies[cluster]
	if !ok {
		policy = clusterPolicy{LBPolicy: "round_robin"}
	}

	for priority := uint32(0); priority <= 10; priority++ {
		group := priorityGroups[priority]
		if len(group) == 0 {
			continue
		}

		switch policy.LBPolicy {
		case "random":
			return b.selectRandom(group)
		case "least_request":
			return b.selectLeastRequest(group)
		default:
			return b.selectRoundRobin(group)
		}
	}

	return nil
}

func filterHealthyEndpoints(
	endpoints []*weightedEndpoint,
	detector *OutlierDetector,
) []*weightedEndpoint {
	healthyEndpoints := make([]*weightedEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if detector != nil && detector.IsEjected(endpointAddress(endpoint)) {
			continue
		}
		healthyEndpoints = append(healthyEndpoints, endpoint)
	}
	return healthyEndpoints
}

func (b *xdsBalancer) selectRoundRobin(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	totalWeight := uint32(0)
	for _, endpoint := range endpoints {
		totalWeight += endpoint.Weight
	}
	if totalWeight == 0 {
		return endpoints[0]
	}

	randomWeight := b.rng.Uint32() % totalWeight
	accumulatedWeight := uint32(0)
	for _, endpoint := range endpoints {
		accumulatedWeight += endpoint.Weight
		if randomWeight < accumulatedWeight {
			return endpoint
		}
	}
	return endpoints[0]
}

func (b *xdsBalancer) selectRandom(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}
	return endpoints[b.rng.Intn(len(endpoints))]
}

func (b *xdsBalancer) selectLeastRequest(endpoints []*weightedEndpoint) *weightedEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	minInFlight := int32(-1)
	var selected *weightedEndpoint
	for _, endpoint := range endpoints {
		inFlight := int32(0)
		if value, ok := b.inFlight[endpointAddress(endpoint)]; ok && value != nil {
			inFlight = atomic.LoadInt32(value)
		}
		if minInFlight == -1 || inFlight < minInFlight {
			minInFlight = inFlight
			selected = endpoint
		}
	}

	if selected != nil {
		if value, ok := b.inFlight[endpointAddress(selected)]; ok && value != nil {
			atomic.AddInt32(value, 1)
		}
	}
	return selected
}

func endpointAddress(endpoint *weightedEndpoint) string {
	return fmt.Sprintf("%s:%d", endpoint.Endpoint.Address, endpoint.Endpoint.Port)
}

type pickResult struct {
	ctx             context.Context
	endpoint        remote.Client
	balancer        *xdsBalancer
	inflightKey     string
	circuitBreaker  *CircuitBreaker
	rateLimiter     *RateLimiter
	outlierDetector *OutlierDetector
}

func (p *pickResult) RemoteClient() remote.Client {
	return p.endpoint
}

func (p *pickResult) Report(err error) {
	if err != nil {
		slog.Debug("rpc call failed",
			slog.String("endpoint", p.endpoint.Protocol()),
			slog.Any("error", err),
		)
	}

	p.balancer.mu.Lock()
	defer p.balancer.mu.Unlock()

	if p.inflightKey != "" {
		if value := p.balancer.inFlight[p.inflightKey]; value != nil {
			if atomic.LoadInt32(value) > 0 {
				atomic.AddInt32(value, -1)
			}
		}
	}
	if p.circuitBreaker != nil {
		p.circuitBreaker.Release(ResourceRequest)
	}
	if p.outlierDetector != nil {
		statusCode := 200
		if err != nil {
			statusCode = 500
		}
		p.outlierDetector.ReportResult(p.inflightKey, err, statusCode)
	}
}
