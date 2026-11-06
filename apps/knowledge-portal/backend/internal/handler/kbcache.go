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

package handler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
)

const (
	// activeKBTTL is how long the active knowledge base set is reused. A
	// knowledge base deactivated upstream stops being served within this window.
	activeKBTTL = 60 * time.Second
	// activeKBLoadTimeout bounds one upstream list call, independent of any
	// single caller's request context.
	activeKBLoadTimeout = 10 * time.Second
)

// kbSet is the set of active knowledge bases, lower-case UUID -> title.
type kbSet map[string]string

// has reports whether the knowledge base id (any case) is in the set.
func (s kbSet) has(id string) bool {
	id, ok := normalizeID(id)
	if !ok {
		return false
	}
	_, ok = s[id]
	return ok
}

type kbFlight struct {
	done chan struct{}
	set  kbSet
	err  error
}

// activeKBCache holds the active knowledge base set in memory. It is a
// read-performance cache only: a failed load is never cached and an expired set
// is never served stale, so the cache fails closed (callers get the error).
// Concurrent callers share one in-flight load.
type activeKBCache struct {
	list func(context.Context) ([]entity.KnowledgeBase, error)
	ttl  time.Duration
	now  func() time.Time

	mu       sync.Mutex
	set      kbSet
	expires  time.Time
	inflight *kbFlight
}

func newActiveKBCache(list func(context.Context) ([]entity.KnowledgeBase, error)) *activeKBCache {
	return &activeKBCache{list: list, ttl: activeKBTTL, now: time.Now}
}

// get returns the active knowledge base set. The returned map must not be modified.
func (c *activeKBCache) get(ctx context.Context) (kbSet, error) {
	c.mu.Lock()
	if c.set != nil && c.now().Before(c.expires) {
		s := c.set
		c.mu.Unlock()
		return s, nil
	}
	if f := c.inflight; f != nil {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.set, f.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f := &kbFlight{done: make(chan struct{})}
	c.inflight = f
	c.mu.Unlock()

	c.load(ctx, f)
	return f.set, f.err
}

// errLoadPanicked is what callers waiting on a load see when the loader panics.
var errLoadPanicked = errors.New("active knowledge base load panicked")

// load runs the upstream list call for flight f and publishes the outcome. The
// bookkeeping is deferred so that a panic in the loader still clears the
// in-flight marker and closes done: otherwise every later caller would find a
// marker that never completes and block until its own context expired. The
// panic itself propagates to the caller that ran the load.
func (c *activeKBCache) load(ctx context.Context, f *kbFlight) {
	var (
		kbs  []entity.KnowledgeBase
		err  error
		done bool
	)
	defer func() {
		c.mu.Lock()
		switch {
		case !done:
			f.err = errLoadPanicked
		case err != nil:
			f.err = err
		default:
			set := make(kbSet, len(kbs))
			for _, kb := range kbs {
				if id, ok := normalizeID(kb.ID); ok && kb.Active {
					set[id] = kb.Title
				}
			}
			f.set = set
			c.set = set
			c.expires = c.now().Add(c.ttl)
		}
		c.inflight = nil
		c.mu.Unlock()
		close(f.done)
	}()

	// Detached from the caller's cancellation so one client hanging up does not
	// fail the load every waiting caller shares; values (correlation id) are kept.
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), activeKBLoadTimeout)
	defer cancel()
	kbs, err = c.list(loadCtx)
	done = true
}
