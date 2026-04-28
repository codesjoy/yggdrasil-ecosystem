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

import (
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// ProviderAPI is the Polaris provider API surface used by this module.
type ProviderAPI interface {
	RegisterInstance(
		instance *polaris.InstanceRegisterRequest,
	) (*model.InstanceRegisterResponse, error)
	Deregister(instance *polaris.InstanceDeRegisterRequest) error
}

// ConsumerAPI is the Polaris consumer API surface used by this module.
type ConsumerAPI interface {
	GetInstances(req *polaris.GetInstancesRequest) (*model.InstancesResponse, error)
}

// ConfigAPI is the Polaris config API surface used by this module.
type ConfigAPI interface {
	FetchConfigFile(*polaris.GetConfigFileRequest) (model.ConfigFile, error)
}

// LimitAPI is the Polaris rate-limit API surface used by this module.
type LimitAPI interface {
	GetQuota(request polaris.QuotaRequest) (polaris.QuotaFuture, error)
	Destroy()
}

// CircuitBreakerAPI is the Polaris circuit-breaker API surface used by this module.
type CircuitBreakerAPI interface {
	Check(model.Resource) (*model.CheckResult, error)
	Report(*model.ResourceStat) error
}

// RouterAPI is the Polaris router API surface used by this module.
type RouterAPI interface {
	ProcessRouters(*polaris.ProcessRoutersRequest) (*model.InstancesResponse, error)
	ProcessLoadBalance(*polaris.ProcessLoadBalanceRequest) (*model.OneInstanceResponse, error)
}
