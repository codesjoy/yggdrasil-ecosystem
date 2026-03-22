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
	"log/slog"
	"math"
	"sync/atomic"
	"time"
)

func (od *OutlierDetector) shouldEjectConsecutive(current, threshold, enforcement uint32) bool {
	return threshold > 0 && current >= threshold && od.shouldEnforce(enforcement)
}

// checkConsecutive5xx checks for consecutive 5xx errors.
func (od *OutlierDetector) checkConsecutive5xx(ep *EndpointStats) {
	if od.shouldEjectConsecutive(
		ep.consecutive5xx,
		od.config.Consecutive5xx,
		od.config.EnforcingConsecutive5xx,
	) {
		od.ejectEndpoint(ep, "consecutive_5xx")
	}
}

// checkConsecutiveGatewayFailure checks for consecutive gateway failures.
func (od *OutlierDetector) checkConsecutiveGatewayFailure(ep *EndpointStats) {
	if od.shouldEjectConsecutive(
		ep.consecutiveGatewayFailure,
		od.config.ConsecutiveGatewayFailure,
		od.config.EnforcingConsecutive5xx,
	) {
		od.ejectEndpoint(ep, "consecutive_gateway_failure")
	}
}

// checkConsecutiveLocalFailure checks for consecutive local failures.
func (od *OutlierDetector) checkConsecutiveLocalFailure(ep *EndpointStats) {
	if od.shouldEjectConsecutive(
		ep.consecutiveLocalFailure,
		od.config.ConsecutiveLocalOriginFailure,
		od.config.EnforcingConsecutive5xx,
	) {
		od.ejectEndpoint(ep, "consecutive_local_failure")
	}
}

// detectSuccessRateOutliers detects outliers based on success rate.
func (od *OutlierDetector) detectSuccessRateOutliers(endpoints []*EndpointStats) {
	if od.config.EnforcingSuccessRate == 0 {
		return
	}

	validEndpoints := endpointsMeetingVolume(endpoints, od.config.SuccessRateRequestVolume)
	if len(validEndpoints) < int(od.config.SuccessRateMinimumHosts) {
		return
	}

	successRates := make([]float64, len(validEndpoints))
	var sum float64
	for i, ep := range validEndpoints {
		successRates[i] = successRate(ep)
		sum += successRates[i]
	}

	mean := sum / float64(len(validEndpoints))
	var variance float64
	for _, rate := range successRates {
		diff := rate - mean
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / float64(len(validEndpoints)))
	threshold := mean - (float64(od.config.SuccessRateStdevFactor)/1000.0)*stddev

	var candidates []*EndpointStats
	for i, ep := range validEndpoints {
		if successRates[i] < threshold && od.shouldEnforce(od.config.EnforcingSuccessRate) {
			candidates = append(candidates, ep)
		}
	}
	for _, ep := range candidates {
		od.ejectEndpoint(ep, "success_rate")
	}
}

// detectFailurePercentageOutliers detects outliers based on failure percentage.
func (od *OutlierDetector) detectFailurePercentageOutliers(endpoints []*EndpointStats) {
	if od.config.EnforcingFailurePercentage == 0 || od.config.FailurePercentageThreshold == 0 {
		return
	}

	validEndpoints := endpointsMeetingVolume(endpoints, od.config.FailurePercentageRequestVolume)
	if len(validEndpoints) < int(od.config.FailurePercentageMinimumHosts) {
		return
	}

	var candidates []*EndpointStats
	for _, ep := range validEndpoints {
		failurePercentage := failurePercentage(ep)
		if failurePercentage >= float64(od.config.FailurePercentageThreshold) &&
			od.shouldEnforce(od.config.EnforcingFailurePercentage) {
			candidates = append(candidates, ep)
		}
	}
	for _, ep := range candidates {
		od.ejectEndpoint(ep, "failure_percentage")
	}
}

func endpointsMeetingVolume(endpoints []*EndpointStats, minimum uint32) []*EndpointStats {
	validEndpoints := make([]*EndpointStats, 0, len(endpoints))
	for _, ep := range endpoints {
		if endpointTotalRequests(ep) >= uint64(minimum) {
			validEndpoints = append(validEndpoints, ep)
		}
	}
	return validEndpoints
}

func endpointTotalRequests(ep *EndpointStats) uint64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return atomic.LoadUint64(&ep.totalRequests)
}

func successRate(ep *EndpointStats) float64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	total := atomic.LoadUint64(&ep.totalRequests)
	if total == 0 {
		return 0
	}
	success := atomic.LoadUint64(&ep.successCount)
	return float64(success) / float64(total) * 100
}

func failurePercentage(ep *EndpointStats) float64 {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	total := atomic.LoadUint64(&ep.totalRequests)
	if total == 0 {
		return 0
	}
	failures := atomic.LoadUint64(&ep.failureCount)
	return float64(failures) / float64(total) * 100
}

// ejectEndpoint ejects an endpoint.
func (od *OutlierDetector) ejectEndpoint(ep *EndpointStats, reason string) {
	if ep.ejected {
		return
	}

	od.mu.RLock()
	totalEndpoints := len(od.endpoints)
	od.mu.RUnlock()

	var ejectedCount int
	od.mu.RLock()
	for _, candidate := range od.endpoints {
		candidate.mu.RLock()
		if candidate.ejected {
			ejectedCount++
		}
		candidate.mu.RUnlock()
	}
	od.mu.RUnlock()

	maxEjected := int(float64(totalEndpoints) * float64(od.config.MaxEjectionPercent) / 100.0)
	if ejectedCount >= maxEjected && maxEjected > 0 {
		slog.Debug(
			"max ejection percentage reached, not ejecting",
			"endpoint", ep.address,
			"ejected", ejectedCount,
			"total", totalEndpoints,
		)
		return
	}

	ep.ejected = true
	ep.ejectionCount++

	ejectionDuration := od.config.BaseEjectionTime * time.Duration(ep.ejectionCount)
	if ejectionDuration > od.config.MaxEjectionTime {
		ejectionDuration = od.config.MaxEjectionTime
	}
	ep.ejectionTime = time.Now().Add(ejectionDuration)
	atomic.AddUint64(&od.totalEjections, 1)

	slog.Warn(
		"endpoint ejected",
		"endpoint", ep.address,
		"reason", reason,
		"ejectionCount", ep.ejectionCount,
		"ejectionDuration", ejectionDuration,
	)
}

// shouldEnforce determines if enforcement should happen based on percentage.
func (od *OutlierDetector) shouldEnforce(enforcingPercentage uint32) bool {
	if enforcingPercentage == 0 {
		return false
	}
	if enforcingPercentage >= 100 {
		return true
	}
	return true
}

// IsEjected checks if an endpoint is currently ejected.
func (od *OutlierDetector) IsEjected(endpoint string) bool {
	od.mu.RLock()
	ep, exists := od.endpoints[endpoint]
	od.mu.RUnlock()
	if !exists {
		return false
	}

	ep.mu.RLock()
	defer ep.mu.RUnlock()
	return ep.ejected
}

// GetStats returns outlier detection statistics.
func (od *OutlierDetector) GetStats() map[string]interface{} {
	od.mu.RLock()
	defer od.mu.RUnlock()

	var ejectedCount int
	for _, ep := range od.endpoints {
		ep.mu.RLock()
		if ep.ejected {
			ejectedCount++
		}
		ep.mu.RUnlock()
	}

	return map[string]interface{}{
		"total_endpoints": len(od.endpoints),
		"ejected_count":   ejectedCount,
		"total_ejections": atomic.LoadUint64(&od.totalEjections),
	}
}
