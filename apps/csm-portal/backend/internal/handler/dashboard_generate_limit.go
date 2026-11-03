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

package handler

import (
	"sync"
	"time"
)

// Defaults for GenerateRateLimiter: enough for a real chat session (each
// message waits 10-40 seconds for the model) while capping what one user can
// spend on model calls.
const (
	DefaultGenerateLimitPerMinute = 10
	DefaultGenerateLimitPerDay    = 100
)

// GenerateRateLimiter caps how many model calls one user starts through
// POST /dashboards/generate: at most perMinute in any 60 seconds and perDay
// in any 24 hours. It keeps each user's recent call times in memory, so the
// limits are per backend instance; that is enough to stop one user running
// up the model bill, which is what it is for.
type GenerateRateLimiter struct {
	perMinute, perDay int
	now               func() time.Time

	mu    sync.Mutex
	calls map[string][]time.Time // per user, oldest first, within the last 24 hours
	seen  int                    // Allow calls since the last sweep
}

// NewGenerateRateLimiter returns a limiter allowing perMinute calls per
// minute and perDay per day for each user; both must be > 0.
func NewGenerateRateLimiter(perMinute, perDay int) *GenerateRateLimiter {
	return &GenerateRateLimiter{perMinute: perMinute, perDay: perDay, now: time.Now, calls: map[string][]time.Time{}}
}

// generateLimitSweepEvery is how many Allow calls pass between sweeps that
// drop users with no call in the last 24 hours, so the map does not keep
// everyone who ever generated.
const generateLimitSweepEvery = 1000

// Allow records a call for user and reports whether it may go ahead. When it
// may not, retryAfter is how long until it would, and daily says which limit
// was hit (the 24-hour one, or the per-minute one). A refused call is not
// recorded.
func (l *GenerateRateLimiter) Allow(user string) (ok bool, retryAfter time.Duration, daily bool) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seen++
	if l.seen >= generateLimitSweepEvery {
		l.seen = 0
		for u, times := range l.calls {
			if len(times) == 0 || now.Sub(times[len(times)-1]) >= 24*time.Hour {
				delete(l.calls, u)
			}
		}
	}

	times := l.calls[user]
	keep := 0
	for keep < len(times) && now.Sub(times[keep]) >= 24*time.Hour {
		keep++
	}
	times = times[keep:]

	if len(times) >= l.perDay {
		return false, times[len(times)-l.perDay].Add(24 * time.Hour).Sub(now), true
	}
	inMinute := 0
	for i := len(times) - 1; i >= 0 && now.Sub(times[i]) < time.Minute; i-- {
		inMinute++
	}
	if inMinute >= l.perMinute {
		return false, times[len(times)-l.perMinute].Add(time.Minute).Sub(now), false
	}
	l.calls[user] = append(times, now)
	return true, 0, false
}
