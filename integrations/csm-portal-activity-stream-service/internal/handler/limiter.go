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

import "sync"

// Defaults for the concurrent-stream caps. Overridable via
// WithConnectionLimits (wired to STREAM_MAX_CONNECTIONS_PER_USER and
// STREAM_MAX_CONNECTIONS in cmd/server/main.go). The browser holds at most
// one stream per visible case tab, so the per-user default is generous for
// legitimate use while still bounding a runaway client.
const (
	DefaultMaxStreamsPerUser = 8
	DefaultMaxStreamsTotal   = 2000
)

// connLimiter is the admission control for StreamCaseActivities: a per-user
// and a per-replica cap on concurrently open streams. Each open stream holds
// a goroutine, a file descriptor and a hub subscription for as long as the
// client likes (up to the lifetime bound), so without a cap one caller could
// exhaust the replica. A limit of 0 disables that dimension.
type connLimiter struct {
	mu         sync.Mutex
	perUser    map[string]int
	total      int
	maxPerUser int
	maxTotal   int
}

func newConnLimiter(maxPerUser, maxTotal int) *connLimiter {
	return &connLimiter{
		perUser:    make(map[string]int),
		maxPerUser: maxPerUser,
		maxTotal:   maxTotal,
	}
}

// admission is the outcome of connLimiter.acquire.
type admission int

const (
	admitted admission = iota
	userLimitReached
	replicaLimitReached
)

// acquire reserves a slot for userID. On admitted the returned release must
// be called exactly once when the stream ends; on either rejection release
// is a no-op. The replica-wide cap is checked first so a saturated replica
// answers consistently regardless of who is asking.
func (l *connLimiter) acquire(userID string) (release func(), outcome admission) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.maxTotal > 0 && l.total >= l.maxTotal {
		return func() {}, replicaLimitReached
	}
	if l.maxPerUser > 0 && l.perUser[userID] >= l.maxPerUser {
		return func() {}, userLimitReached
	}
	l.total++
	l.perUser[userID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.total--
			if l.perUser[userID] <= 1 {
				delete(l.perUser, userID)
			} else {
				l.perUser[userID]--
			}
		})
	}, admitted
}

// counts returns the current per-user count for userID and the replica
// total. Test helper; not used on the request path.
func (l *connLimiter) counts(userID string) (user, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.perUser[userID], l.total
}
