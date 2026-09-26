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
	"testing"
	"time"
)

func TestServiceCache_MissWhenEmpty(t *testing.T) {
	c := newServiceCache(15 * time.Minute)
	if _, ok := c.get("svc", time.Now()); ok {
		t.Error("get on an empty cache reported a hit, want a miss")
	}
}

func TestServiceCache_HitBeforeExpiry(t *testing.T) {
	c := newServiceCache(15 * time.Minute)
	now := time.Now()
	c.set("Azure Monitoring", "33333333-3333-3333-3333-333333333333", now)

	id, ok := c.get("Azure Monitoring", now.Add(10*time.Minute))
	if !ok {
		t.Fatal("get reported a miss before the TTL elapsed, want a hit")
	}
	if id != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("id = %q, want the cached UUID", id)
	}
}

func TestServiceCache_MissAfterExpiry(t *testing.T) {
	c := newServiceCache(15 * time.Minute)
	now := time.Now()
	c.set("Azure Monitoring", "33333333-3333-3333-3333-333333333333", now)

	if _, ok := c.get("Azure Monitoring", now.Add(15*time.Minute+time.Second)); ok {
		t.Error("get reported a hit past the TTL, want a miss")
	}
}

func TestServiceCache_DistinctLabelsDoNotCollide(t *testing.T) {
	c := newServiceCache(15 * time.Minute)
	now := time.Now()
	c.set("Azure Monitoring", "33333333-3333-3333-3333-333333333333", now)
	c.set("Site24x7 Monitoring", "44444444-4444-4444-4444-444444444444", now)

	if id, ok := c.get("Azure Monitoring", now); !ok || id != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("Azure Monitoring = (%q, %v), want the Azure UUID and a hit", id, ok)
	}
	if id, ok := c.get("Site24x7 Monitoring", now); !ok || id != "44444444-4444-4444-4444-444444444444" {
		t.Errorf("Site24x7 Monitoring = (%q, %v), want the Site24x7 UUID and a hit", id, ok)
	}
}
