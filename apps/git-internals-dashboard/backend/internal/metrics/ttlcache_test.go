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

package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestTTLCacheGetMissReturnsFalse(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 10)
	if _, ok := c.Get("x"); ok {
		t.Error("expected a miss on an empty cache")
	}
}

func TestTTLCacheSetThenGetHits(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 10)
	c.Set("x", 42)
	v, ok := c.Get("x")
	if !ok || v != 42 {
		t.Errorf("expected hit with value 42, got ok=%v v=%v", ok, v)
	}
}

func TestTTLCacheExpiresAfterTTL(t *testing.T) {
	c := NewTTLCache[string, int](10*time.Millisecond, 10)
	c.Set("x", 1)
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("x"); ok {
		t.Error("expected the entry to have expired")
	}
}

func TestTTLCacheEvictsOldestWhenOverCapacity(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 2)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // over capacity — "a" (oldest) should be evicted

	if _, ok := c.Get("a"); ok {
		t.Error("expected oldest entry 'a' to have been evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("expected 'b' to survive, got ok=%v v=%v", ok, v)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("expected 'c' to survive, got ok=%v v=%v", ok, v)
	}
}

func TestTTLCacheUpdatingExistingKeyDoesNotEvict(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 2)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("a", 100) // update, not a new key — must not trigger eviction

	if v, ok := c.Get("a"); !ok || v != 100 {
		t.Errorf("expected updated value 100, got ok=%v v=%v", ok, v)
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected 'b' to still be present after updating an existing key")
	}
}

func TestTTLCacheGetOrSetComputesOnceOnHit(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 10)
	calls := 0
	compute := func() (int, error) {
		calls++
		return 7, nil
	}

	v1, err := c.GetOrSet("k", compute)
	if err != nil || v1 != 7 {
		t.Fatalf("expected 7, nil; got %v, %v", v1, err)
	}
	v2, err := c.GetOrSet("k", compute)
	if err != nil || v2 != 7 {
		t.Fatalf("expected 7, nil; got %v, %v", v2, err)
	}
	if calls != 1 {
		t.Errorf("expected compute to run once (cache hit second time), got %d calls", calls)
	}
}

func TestTTLCacheGetOrSetPropagatesComputeError(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute, 10)
	wantErr := errors.New("boom")
	_, err := c.GetOrSet("k", func() (int, error) { return 0, wantErr })
	if !errors.Is(err, wantErr) {
		t.Errorf("expected compute's error to propagate, got %v", err)
	}
	if _, ok := c.Get("k"); ok {
		t.Error("expected nothing cached after a failed compute")
	}
}
