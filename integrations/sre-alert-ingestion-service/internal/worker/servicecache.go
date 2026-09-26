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

package worker

import (
	"sync"
	"time"
)

// serviceCache is a small in-memory, TTL-bounded cache mapping an alert's
// raw Service label (internal/handler.AlertRequest.Service, as preserved in
// alertpayload.Payload.Service) to a CMDB service UUID already resolved via
// csmclient.Client.SearchServices. It exists purely so that a label with no
// SRE_ALERT_SERVICE_MAP entry (internal/handler.MapToIncident's static fast
// path) doesn't pay for a live /services/search call on every single
// delivery attempt for every alert reporting that label — one successful
// resolution is reused for ServiceCacheTTL instead.
//
// Only a successful resolution is ever cached — see resolveServiceID. A
// confirmed zero-result search, or a transient search error, is
// deliberately never cached: either could become a different outcome on a
// later attempt (a new CMDB service could be added after a zero-result
// search; a transient failure is, by definition, expected to clear), and
// caching either would risk pinning an alert to a stale wrong answer for a
// full TTL window.
//
// A plain mutex-guarded map, not sync.Map: this cache is read-then-write
// (a miss always leads to a SearchServices call and then a set), which
// sync.Map offers no real benefit for over a plain map + one mutex at this
// service's traffic volume.
//
// Safe for concurrent use.
type serviceCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]serviceCacheEntry
}

// serviceCacheEntry is one cached resolution.
type serviceCacheEntry struct {
	id        string
	expiresAt time.Time
}

// newServiceCache constructs an empty cache with the given TTL. ttl should
// already be a resolved, positive value (Config.withDefaults() guarantees
// this for Worker's own construction) — newServiceCache applies no default
// of its own.
func newServiceCache(ttl time.Duration) *serviceCache {
	return &serviceCache{ttl: ttl, entries: make(map[string]serviceCacheEntry)}
}

// get returns the cached UUID for label and true, if a still-unexpired entry
// exists as of now. now is caller-supplied (Worker.now(), overridable in
// tests) rather than time.Now(), matching this package's existing
// deterministic-time convention (see Worker.now's own doc comment).
func (c *serviceCache) get(label string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[label]
	if !ok || now.After(e.expiresAt) {
		return "", false
	}
	return e.id, true
}

// set records a successful resolution for label, expiring ttl after now.
func (c *serviceCache) set(label, id string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[label] = serviceCacheEntry{id: id, expiresAt: now.Add(c.ttl)}
}
