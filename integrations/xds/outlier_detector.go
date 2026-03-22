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

package xds

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of an endpoint.
type HealthStatus int

const (
	// HealthUnknown indicates health status is unknown.
	HealthUnknown HealthStatus = iota
	// HealthHealthy indicates endpoint is healthy.
	HealthHealthy
	// HealthUnhealthy indicates endpoint is unhealthy.
	HealthUnhealthy
	// HealthDraining indicates endpoint is draining.
	HealthDraining
	// HealthTimeout indicates endpoint has timed out.
	HealthTimeout
	// HealthDegraded indicates endpoint is degraded.
	HealthDegraded
)

func (h HealthStatus) String() string {
	switch h {
	case HealthHealthy:
		return "HEALTHY"
	case HealthUnhealthy:
		return "UNHEALTHY"
	case HealthDraining:
		return "DRAINING"
	case HealthTimeout:
		return "TIMEOUT"
	case HealthDegraded:
		return "DEGRADED"
	default:
		return "UNKNOWN"
	}
}

// ParseHealthStatus parses a health status string.
func ParseHealthStatus(s string) HealthStatus {
	switch strings.ToUpper(s) {
	case "HEALTHY":
		return HealthHealthy
	case "UNHEALTHY":
		return HealthUnhealthy
	case "DRAINING":
		return HealthDraining
	case "TIMEOUT":
		return HealthTimeout
	case "DEGRADED":
		return HealthDegraded
	default:
		return HealthUnknown
	}
}

// OutlierDetectionConfig holds outlier detection configuration.
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

// DefaultOutlierDetectionConfig returns default outlier detection configuration.
func DefaultOutlierDetectionConfig() *OutlierDetectionConfig {
	return &OutlierDetectionConfig{
		Consecutive5xx:                 5,
		ConsecutiveGatewayFailure:      5,
		ConsecutiveLocalOriginFailure:  5,
		Interval:                       10 * time.Second,
		BaseEjectionTime:               30 * time.Second,
		MaxEjectionTime:                300 * time.Second,
		MaxEjectionPercent:             10,
		EnforcingConsecutive5xx:        100,
		EnforcingSuccessRate:           100,
		SuccessRateMinimumHosts:        5,
		SuccessRateRequestVolume:       100,
		SuccessRateStdevFactor:         1900,
		FailurePercentageThreshold:     85,
		EnforcingFailurePercentage:     0,
		FailurePercentageMinimumHosts:  5,
		FailurePercentageRequestVolume: 50,
		SplitExternalLocalOriginErrors: false,
	}
}

// EndpointStats tracks statistics for a single endpoint.
type EndpointStats struct {
	address string

	totalRequests   uint64
	successCount    uint64
	failureCount    uint64
	localFailures   uint64
	gatewayFailures uint64

	consecutive5xx            uint32
	consecutiveGatewayFailure uint32
	consecutiveLocalFailure   uint32

	ejected       bool
	ejectionTime  time.Time
	ejectionCount uint32

	mu sync.RWMutex
}

// OutlierDetector implements error-rate based outlier detection.
type OutlierDetector struct {
	config    *OutlierDetectionConfig
	endpoints map[string]*EndpointStats
	mu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	totalEjections uint64
}

// NewOutlierDetector creates a new outlier detector.
func NewOutlierDetector(config *OutlierDetectionConfig) *OutlierDetector {
	if config == nil {
		config = DefaultOutlierDetectionConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &OutlierDetector{
		config:    config,
		endpoints: make(map[string]*EndpointStats),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins periodic health check sweeps.
func (od *OutlierDetector) Start() {
	if od.config.Interval == 0 {
		return
	}

	od.wg.Add(1)
	go od.runHealthSweep()
	slog.Info("outlier detector started", "interval", od.config.Interval)
}

// Stop stops the outlier detector.
func (od *OutlierDetector) Stop() {
	od.cancel()
	od.wg.Wait()
	slog.Info("outlier detector stopped")
}

func (od *OutlierDetector) runHealthSweep() {
	defer od.wg.Done()

	ticker := time.NewTicker(od.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-od.ctx.Done():
			return
		case <-ticker.C:
			od.performHealthSweep()
		}
	}
}

func (od *OutlierDetector) performHealthSweep() {
	endpoints := od.snapshotEndpoints()
	od.recoverEndpoints(endpoints, time.Now())
	od.detectSuccessRateOutliers(endpoints)
	od.detectFailurePercentageOutliers(endpoints)
	od.resetIntervalStats(endpoints)
}

func (od *OutlierDetector) snapshotEndpoints() []*EndpointStats {
	od.mu.RLock()
	defer od.mu.RUnlock()

	endpoints := make([]*EndpointStats, 0, len(od.endpoints))
	for _, ep := range od.endpoints {
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

func (od *OutlierDetector) recoverEndpoints(endpoints []*EndpointStats, now time.Time) {
	for _, ep := range endpoints {
		ep.mu.Lock()
		if ep.ejected && now.After(ep.ejectionTime) {
			ep.ejected = false
			slog.Info(
				"endpoint recovered from ejection",
				"endpoint",
				ep.address,
				"ejectionCount",
				ep.ejectionCount,
			)
		}
		ep.mu.Unlock()
	}
}

func (od *OutlierDetector) resetIntervalStats(endpoints []*EndpointStats) {
	for _, ep := range endpoints {
		ep.mu.Lock()
		ep.totalRequests = 0
		ep.successCount = 0
		ep.failureCount = 0
		ep.localFailures = 0
		ep.gatewayFailures = 0
		ep.mu.Unlock()
	}
}

// ReportResult reports request result for outlier detection.
func (od *OutlierDetector) ReportResult(endpoint string, err error, statusCode int) {
	ep := od.endpointStats(endpoint)
	ep.mu.Lock()
	reasons := od.recordEndpointResultLocked(ep, err, statusCode)
	ep.mu.Unlock()

	for _, reason := range reasons {
		od.ejectEndpoint(ep, reason)
	}
}

func (od *OutlierDetector) endpointStats(endpoint string) *EndpointStats {
	od.mu.Lock()
	defer od.mu.Unlock()

	if ep, ok := od.endpoints[endpoint]; ok {
		return ep
	}

	ep := &EndpointStats{address: endpoint}
	od.endpoints[endpoint] = ep
	return ep
}

func (od *OutlierDetector) recordEndpointResultLocked(
	ep *EndpointStats,
	err error,
	statusCode int,
) []string {
	atomic.AddUint64(&ep.totalRequests, 1)

	if err == nil && statusCode >= 200 && statusCode < 500 {
		atomic.AddUint64(&ep.successCount, 1)
		ep.consecutive5xx = 0
		ep.consecutiveGatewayFailure = 0
		ep.consecutiveLocalFailure = 0
		return nil
	}

	var reasons []string
	atomic.AddUint64(&ep.failureCount, 1)
	if statusCode >= 500 && statusCode < 600 {
		ep.consecutive5xx++
		if od.shouldEjectConsecutive(
			ep.consecutive5xx,
			od.config.Consecutive5xx,
			od.config.EnforcingConsecutive5xx,
		) {
			reasons = append(reasons, "consecutive_5xx")
		}
	}
	if statusCode == 502 || statusCode == 503 || statusCode == 504 {
		atomic.AddUint64(&ep.gatewayFailures, 1)
		ep.consecutiveGatewayFailure++
		if od.shouldEjectConsecutive(
			ep.consecutiveGatewayFailure,
			od.config.ConsecutiveGatewayFailure,
			od.config.EnforcingConsecutive5xx,
		) {
			reasons = append(reasons, "consecutive_gateway_failure")
		}
	}
	if err != nil {
		atomic.AddUint64(&ep.localFailures, 1)
		ep.consecutiveLocalFailure++
		if od.shouldEjectConsecutive(
			ep.consecutiveLocalFailure,
			od.config.ConsecutiveLocalOriginFailure,
			od.config.EnforcingConsecutive5xx,
		) {
			reasons = append(reasons, "consecutive_local_failure")
		}
	}

	return reasons
}
