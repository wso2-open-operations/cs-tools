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
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// entityHealthChecker is the one entity-client operation HealthHandler needs.
type entityHealthChecker interface {
	Health(ctx context.Context) error
}

// healthCacheTTL is how long one probe result is reused. Health polls from
// the platform and from the portal backend's dependency check arrive every
// few seconds; reusing a result keeps them from turning into a steady load on
// the entity service, while a real outage still shows within this window.
const healthCacheTTL = 15 * time.Second

// HealthHandler serves GET /health. It answers 200 {"status":"ok"} while the
// entity service — this service's only dependency, without which no other
// route can succeed — is reachable, and 503 {"status":"unavailable"} when it
// is not. The probe result is cached for healthCacheTTL, and only one probe
// runs at a time: concurrent polls during a refresh wait for it and share
// its result.
type HealthHandler struct {
	entity entityHealthChecker
	ttl    time.Duration
	now    func() time.Time

	mu        sync.Mutex
	checkedAt time.Time
	healthy   bool
}

// NewHealthHandler creates a HealthHandler probing the given entity client.
func NewHealthHandler(entity entityHealthChecker) *HealthHandler {
	return &HealthHandler{entity: entity, ttl: healthCacheTTL, now: time.Now}
}

// ServeHTTP handles GET /health.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.reachable(r.Context()) {
		writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, []byte(`{"status":"unavailable"}`))
}

func (h *HealthHandler) reachable(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	if !h.checkedAt.IsZero() && now.Sub(h.checkedAt) < h.ttl {
		return h.healthy
	}
	// The probe outlives a caller that gives up, so its result still lands in
	// the cache for the polls behind it.
	err := h.entity.Health(context.WithoutCancel(ctx))
	if err != nil && h.healthy {
		slog.WarnContext(ctx, "entity service is unreachable", "err", summarizeErr(err))
	} else if err == nil && !h.healthy && !h.checkedAt.IsZero() {
		slog.InfoContext(ctx, "entity service is reachable again")
	}
	h.healthy = err == nil
	h.checkedAt = now
	return h.healthy
}
