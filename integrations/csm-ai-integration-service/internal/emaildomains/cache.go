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

package emaildomains

import (
	"sync"
	"time"
)

// maxCached bounds the cache; when full, expired entries are dropped, then all.
const maxCached = 10000

type cacheEntry struct {
	class   Classification
	expires time.Time
}

// cache keeps the model's answers by domain, in memory only.
type cache struct {
	mu  sync.Mutex
	m   map[string]cacheEntry
	ttl time.Duration
	now func() time.Time
}

func newCache(ttl time.Duration) *cache {
	return &cache{m: map[string]cacheEntry{}, ttl: ttl, now: time.Now}
}

func (c *cache) get(domain string) (Classification, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[domain]
	if !ok || c.now().After(e.expires) {
		return "", false
	}
	return e.class, true
}

func (c *cache) set(domain string, class Classification) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.m) >= maxCached {
		for d, e := range c.m {
			if now.After(e.expires) {
				delete(c.m, d)
			}
		}
		if len(c.m) >= maxCached {
			clear(c.m)
		}
	}
	c.m[domain] = cacheEntry{class: class, expires: now.Add(c.ttl)}
}
