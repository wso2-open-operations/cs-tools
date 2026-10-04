// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package eventbus

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// RetryPolicy is how often, and how far apart, a Consumer calls Handle for
// one record before handing it to OnExhausted (see Run). The pause before
// attempt n+1 is BaseDelay doubled n-1 times, capped at MaxDelay, with
// equal jitter applied (half of that value is fixed, the other half is
// random) so that a partition's worth of records failing against the same
// outage do not all retry in lock-step.
//
// MaxAttempts is the total number of Handle calls, not the number of
// retries on top of a first call: MaxAttempts=5 means 5 calls, 4 of them
// retries. The whole schedule is bounded (sum of the capped delays, plus
// each attempt's own work) so one permanently-failing record cannot block
// every later record on its partition forever — it is dead-lettered or
// parked and its offset committed once the attempts run out.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryPolicy is the main (first-tier) consumer's schedule: five
// attempts spread over roughly 15–30 s (2 s, 4 s, 8 s, 16 s nominal, each
// jittered down to no less than half). Enough to ride out a brief blip
// without holding the partition for long; anything longer is the
// dead-letter tier's job.
var DefaultRetryPolicy = RetryPolicy{MaxAttempts: 5, BaseDelay: 2 * time.Second, MaxDelay: time.Minute}

// DefaultDeadLetterRetryPolicy is the dead-letter consumer's schedule: five
// attempts spread over roughly 4–7.5 min (30 s, 60 s, 120 s, 240 s nominal,
// jittered). Combined with the delay a dead-lettered record waits before
// its first dead-letter attempt (see HeaderNotBefore), a record keeps being
// retried for well over ten minutes from its first failure before it is
// parked — long enough to outlast an upstream redeploy, which is the
// outage this most often has to survive.
var DefaultDeadLetterRetryPolicy = RetryPolicy{MaxAttempts: 5, BaseDelay: 30 * time.Second, MaxDelay: 5 * time.Minute}

// normalized fills in anything a zero or nonsensical value would otherwise
// break: at least one attempt, a non-negative base delay, and a cap no
// smaller than the base.
func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	if p.BaseDelay < 0 {
		p.BaseDelay = 0
	}
	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}
	return p
}

// Delay is the pause to take after attempt (1-based) has failed, before the
// next one. Randomized — see RetryPolicy.
func (p RetryPolicy) Delay(attempt int) time.Duration {
	return p.delay(attempt, randomFraction())
}

// delay is Delay with the jitter fraction (in [0, 1)) supplied by the
// caller, so the schedule's bounds are testable without randomness.
func (p RetryPolicy) delay(attempt int, frac float64) time.Duration {
	p = p.normalized()
	if attempt < 1 {
		attempt = 1
	}
	d := p.BaseDelay
	for i := 1; i < attempt && d < p.MaxDelay; i++ {
		d *= 2
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	half := d / 2
	return half + time.Duration(float64(half)*frac)
}

// MinTotalDelay is the smallest total time the schedule can spend waiting
// between attempts (every jitter roll at its minimum) — the guaranteed
// floor of the window a record keeps being retried over.
func (p RetryPolicy) MinTotalDelay() time.Duration {
	p = p.normalized()
	var total time.Duration
	for attempt := 1; attempt < p.MaxAttempts; attempt++ {
		total += p.delay(attempt, 0)
	}
	return total
}

// randomFraction returns a uniformly distributed value in [0, 1). It reads
// crypto/rand rather than math/rand purely so the jitter needs no seeding
// or per-consumer generator; on the (practically impossible) read failure
// it returns 0.5, the midpoint, which keeps the schedule sane.
func randomFraction() float64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0.5
	}
	// 53 random bits → a float64 in [0, 1), the same construction
	// math/rand uses.
	return float64(binary.LittleEndian.Uint64(buf[:])>>11) / (1 << 53)
}
