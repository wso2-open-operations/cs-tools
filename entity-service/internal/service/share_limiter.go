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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"sync"
	"time"
)

// shareLimiter caps how many SFTPGo shares one caller may have created in a
// window. Each share is a stored record in SFTPGo, so this is what stops a
// caller looping on the link endpoints. It is per pod and in memory: the limit is
// a guard against abuse, not an exact quota.
type shareLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	buckets map[string]shareBucket
	now     func() time.Time
}

type shareBucket struct {
	start time.Time
	used  int
}

func newShareLimiter(max int, window time.Duration) *shareLimiter {
	return &shareLimiter{max: max, window: window, buckets: map[string]shareBucket{}, now: time.Now}
}

// allow reserves n shares for caller and reports whether they fit in the
// current window; when they do not, nothing is reserved.
func (l *shareLimiter) allow(caller string, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.buckets[caller]
	if now.Sub(b.start) >= l.window {
		b = shareBucket{start: now}
		// Drop windows that have ended so the map does not grow with callers.
		for k, v := range l.buckets {
			if now.Sub(v.start) >= l.window {
				delete(l.buckets, k)
			}
		}
	}
	if b.used+n > l.max {
		return false
	}
	b.used += n
	l.buckets[caller] = b
	return true
}
