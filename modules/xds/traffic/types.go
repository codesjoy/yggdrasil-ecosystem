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

	xdsresource "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/internal/resource"
)

//nolint:revive // Aliases keep traffic internals close to the shared xDS resource model.
type (
	// CircuitBreakerConfig holds circuit breaker configuration.
	CircuitBreakerConfig = xdsresource.CircuitBreakerConfig
	// OutlierDetectionConfig holds outlier detection configuration.
	OutlierDetectionConfig = xdsresource.OutlierDetectionConfig
	// RateLimiterConfig holds rate limiter configuration.
	RateLimiterConfig = xdsresource.RateLimiterConfig

	clusterPolicy    = xdsresource.ClusterPolicy
	weightedEndpoint = xdsresource.WeightedEndpoint
)

var errRateLimitExceeded = errors.New("rate limit exceeded")
