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
	"math"
	"math/rand/v2"
	"time"
)

type backoff struct {
	cfg BackoffConfig
}

func newBackoff(cfg BackoffConfig) *backoff {
	return &backoff{cfg: cfg}
}

func (b *backoff) Backoff(retry int) time.Duration {
	if retry == 0 {
		return b.cfg.BaseDelay
	}
	delay := float64(b.cfg.BaseDelay) * math.Pow(b.cfg.Multiplier, float64(retry))
	if b.cfg.Jitter > 0 {
		delay *= 1.0 + b.cfg.Jitter*(2*rand.Float64()-1.0) //nolint:gosec
	}
	result := time.Duration(delay)
	if result > b.cfg.MaxDelay {
		return b.cfg.MaxDelay
	}
	return result
}
