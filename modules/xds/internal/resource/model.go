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

package resource

import (
	"regexp"
	"time"
)

// DiscoveryEventType identifies the kind of xDS resource update.
type DiscoveryEventType int

const (
	// ListenerAdded indicates a listener snapshot was parsed.
	ListenerAdded DiscoveryEventType = iota
	// RouteAdded indicates a route snapshot was parsed.
	RouteAdded
	// ClusterAdded indicates a cluster snapshot was parsed.
	ClusterAdded
	// EndpointAdded indicates an endpoint snapshot was parsed.
	EndpointAdded
)

// DiscoveryEvent is a parsed xDS resource update.
type DiscoveryEvent struct {
	Typ  DiscoveryEventType
	Name string
	Data any
}

// ClusterPolicy is the traffic policy parsed from an xDS cluster.
type ClusterPolicy struct {
	LBPolicy         string
	MaxRequests      uint32
	CircuitBreaker   *CircuitBreakerConfig
	OutlierDetection *OutlierDetectionConfig
	RateLimiter      *RateLimiterConfig
}

// WeightedEndpoint is an endpoint plus xDS load-balancing metadata.
type WeightedEndpoint struct {
	Cluster  string
	Endpoint Endpoint
	Weight   uint32
	Priority uint32
	Metadata map[string]string
}

// Endpoint represents a service endpoint.
type Endpoint struct {
	Address string
	Port    int
}

// ListenerSnapshot is the parsed subset of an xDS listener.
type ListenerSnapshot struct {
	Route string
}

// RouteSnapshot is the parsed subset of an xDS route configuration.
type RouteSnapshot struct {
	Vhosts []*VirtualHost
}

// ClusterSnapshot is the parsed subset of an xDS cluster.
type ClusterSnapshot struct {
	Policy ClusterPolicy
}

// EDSSnapshot is the parsed subset of an xDS cluster load assignment.
type EDSSnapshot struct {
	Endpoints []*WeightedEndpoint
}

const (
	// AttributeRoutes is the resolver state attribute key for route data.
	AttributeRoutes = "xds_routes"
	// AttributeClusters is the resolver state attribute key for cluster policies.
	AttributeClusters = "xds_clusters"
	// AttributeEndpointCluster is the endpoint attribute key for cluster ownership.
	AttributeEndpointCluster = "xds_cluster"
	// AttributeEndpointWeight is the endpoint attribute key for xDS weight.
	AttributeEndpointWeight = "weight"
	// AttributeEndpointPriority is the endpoint attribute key for xDS priority.
	AttributeEndpointPriority = "priority"
	// AttributeEndpointMetadata is the endpoint attribute key for xDS metadata.
	AttributeEndpointMetadata = "metadata"
)

// CircuitBreakerConfig holds circuit breaker configuration parsed from xDS.
type CircuitBreakerConfig struct {
	MaxConnections     uint32
	MaxPendingRequests uint32
	MaxRequests        uint32
	MaxRetries         uint32
}

// OutlierDetectionConfig holds outlier detection configuration parsed from xDS.
type OutlierDetectionConfig struct {
	Consecutive5xx                 uint32
	ConsecutiveGatewayFailure      uint32
	ConsecutiveLocalOriginFailure  uint32
	Interval                       time.Duration
	BaseEjectionTime               time.Duration
	MaxEjectionTime                time.Duration
	MaxEjectionPercent             uint32
	EnforcingConsecutive5xx        uint32
	EnforcingSuccessRate           uint32
	SuccessRateMinimumHosts        uint32
	SuccessRateRequestVolume       uint32
	SuccessRateStdevFactor         uint32
	FailurePercentageThreshold     uint32
	EnforcingFailurePercentage     uint32
	FailurePercentageMinimumHosts  uint32
	FailurePercentageRequestVolume uint32
	SplitExternalLocalOriginErrors bool
}

// RateLimiterConfig holds rate limiter configuration parsed from xDS.
type RateLimiterConfig struct {
	MaxTokens     uint32
	TokensPerFill uint32
	FillInterval  time.Duration
}

// VirtualHost represents the xDS VirtualHost configuration.
type VirtualHost struct {
	Name    string
	Domains []string
	Routes  []*Route
}

// Route represents a single route within a VirtualHost.
type Route struct {
	Match  *RouteMatch
	Action *RouteAction
}

// RouteMatch defines how to match a request.
type RouteMatch struct {
	Prefix   string
	Path     string
	Suffix   string
	Contains string
	Regex    *regexp.Regexp
	Headers  []*HeaderMatcher
}

// HeaderMatcher matches HTTP headers.
type HeaderMatcher struct {
	Name        string
	ExactMatch  string
	PrefixMatch string
	SuffixMatch string
	RegexMatch  *regexp.Regexp
	Present     bool
}

// RouteAction defines what to do when a route matches.
type RouteAction struct {
	Cluster          string
	WeightedClusters *WeightedClusters
}

// WeightedClusters supports traffic splitting.
type WeightedClusters struct {
	Clusters    []*WeightedCluster
	TotalWeight uint32
}

// WeightedCluster represents a single cluster in a traffic split.
type WeightedCluster struct {
	Name   string
	Weight uint32
}
