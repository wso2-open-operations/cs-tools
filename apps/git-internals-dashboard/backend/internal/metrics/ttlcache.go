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

// Package metrics builds the /metrics/overview and /metrics/timeseries
// responses (SPEC §6.6/§6.7, ports of v3's overview.ts/timeseries.ts).
package metrics

import (
	"sync"
	"time"
)

// TTLCache is a minimal in-process TTL cache with a max-entry bound (evicts
// the oldest-inserted entry first once at capacity). Port of v3's
// ttl-cache.ts. Process-local by design: correct for the values cached here
// (overview/timeseries responses, issue titles) where a few seconds/minutes
// of staleness or a cache miss on the "wrong" replica is harmless — never
// used for anything requiring cross-replica consistency (see internal/jobs
// for that guarantee).
type TTLCache[K comparable, V any] struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	values     map[K]cacheEntry[V]
	order      []K // insertion order, oldest first; for eviction only
}

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// NewTTLCache creates a cache whose entries expire ttl after they're set,
// bounded to maxEntries (a non-positive value falls back to 1000).
func NewTTLCache[K comparable, V any](ttl time.Duration, maxEntries int) *TTLCache[K, V] {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &TTLCache[K, V]{ttl: ttl, maxEntries: maxEntries, values: make(map[K]cacheEntry[V])}
}

// Get returns the cached value for key, or ok=false if absent or expired
// (an expired entry is deleted on this call).
func (c *TTLCache[K, V]) Get(key K) (value V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, found := c.values[key]
	if !found {
		return value, false
	}
	if time.Now().After(e.expiresAt) {
		c.deleteLocked(key)
		return value, false
	}
	return e.value, true
}

// Set stores value for key with a fresh TTL, evicting the oldest entry
// first if this key is new and the cache is already at capacity.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.values[key]; !exists {
		if len(c.order) >= c.maxEntries && len(c.order) > 0 {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.values, oldest)
		}
		c.order = append(c.order, key)
	}
	c.values[key] = cacheEntry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
}

func (c *TTLCache[K, V]) deleteLocked(key K) {
	delete(c.values, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// GetOrSet returns the cached value for key if present, otherwise calls
// compute, caches its result, and returns it. Concurrent misses may compute
// twice (acceptable here — see the package doc comment).
func (c *TTLCache[K, V]) GetOrSet(key K, compute func() (V, error)) (V, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	value, err := compute()
	if err != nil {
		var zero V
		return zero, err
	}
	c.Set(key, value)
	return value, nil
}
