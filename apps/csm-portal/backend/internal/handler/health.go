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
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthPinger is satisfied by any upstream client that exposes a health
// check — scim.Client, updates.Client, csmnotification.Client,
// csmintegration.Client and entity.EngineeringEntityClient all do.
type HealthPinger interface {
	Health(ctx context.Context) error
}

// dependencyHealthCheckTimeout bounds each individual dependency check so one
// slow or hanging upstream cannot make the whole aggregation hang.
const dependencyHealthCheckTimeout = 5 * time.Second

// dependencyHealthCacheTTL bounds how often GetHealthDependencies actually
// fans out to upstream dependencies. This route is unauthenticated and
// public (see HealthHandler's own doc comment), so an unauthenticated caller
// repeating the request could otherwise trigger unbounded concurrent
// upstream calls on every hit. Caching the result for a short window, and
// collapsing any request that arrives while a check is already in flight
// (see GetHealthDependencies below), bounds that to at most one fan-out per
// TTL window regardless of request volume.
const dependencyHealthCacheTTL = 10 * time.Second

// healthNow is time.Now, indirected so tests can fake the passage of time
// without a real sleep — same override-for-tests convention as
// csmnotification/csmintegration/scim/updates' own tokenFetchTimeout.
var healthNow = time.Now

// Dependency health status vocabulary for HealthDependenciesResponse.
const (
	dependencyStatusOK            = "ok"
	dependencyStatusDown          = "down"
	dependencyStatusNotConfigured = "not_configured"
)

// DependencyHealth is one entry of HealthDependenciesResponse. It carries no
// failure detail by design — this route is unauthenticated and public, the
// same posture entity-service's own GET /health/database documents ("failure
// bodies carry no detail... this endpoint is unauthenticated and public").
type DependencyHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// HealthDependenciesResponse is the body of GET /health/dependencies.
type HealthDependenciesResponse struct {
	Status       string             `json:"status"`
	Dependencies []DependencyHealth `json:"dependencies"`
}

// HealthHandler serves GET /health/dependencies — an aggregating check over
// this backend's own upstream integrations, distinct from the bare
// GET /health liveness probe registered directly on the mux in
// cmd/server/main.go. The two are kept separate on purpose, mirroring
// entity-service's own GET /health vs GET /health/database split: a
// liveness probe that fails because SCIM or Updates is briefly down would
// have the orchestrator restart or drain an instance that is otherwise
// working fine. This endpoint is for monitoring/on-call visibility; do not
// wire it up as the restart-triggering probe.
//
// entity-service (this repo's own core entity service, distinct from
// Engineering Entity Service) is deliberately not checked here — this
// backend depends on it for nearly every request, and checking it is out of
// scope for this aggregation by explicit product decision.
type HealthHandler struct {
	scim         HealthPinger
	updates      HealthPinger
	notification HealthPinger // nil when CSM_NOTIFICATION_SERVICE_BASE_URL is unset
	integration  HealthPinger // nil when CSM_INTEGRATION_SERVICE_BASE_URL is unset
	engineering  HealthPinger // nil when ENGINEERING_ENTITY_BASE_URL is unset
	ai           HealthPinger // nil when CSM_AI_INTEGRATION_SERVICE_BASE_URL is unset

	// mu guards the cached result below and is held for the full duration of
	// a cache-miss recomputation, not just the read/write of the cached
	// fields. That is deliberate, not an oversight: a request that arrives
	// while another is already recomputing blocks on this same mutex rather
	// than starting its own concurrent fan-out, so concurrent callers within
	// one TTL window collapse onto a single set of upstream calls (the same
	// effect a singleflight.Group gives, without adding that dependency for
	// one call site).
	mu               sync.Mutex
	cachedResponse   HealthDependenciesResponse
	cachedStatusCode int
	cachedAt         time.Time
}

// NewHealthHandler constructs a HealthHandler. Any of notification/
// integration/engineering may be nil (pass an untyped nil, never a
// nil-valued concrete pointer — see entity-service's own CLAUDE.md for why a
// typed nil boxed into an interface is a common bug here) when that
// service's base URL is not configured.
func NewHealthHandler(scim, updates, notification, integration, engineering HealthPinger) *HealthHandler {
	return &HealthHandler{
		scim:         scim,
		updates:      updates,
		notification: notification,
		integration:  integration,
		engineering:  engineering,
	}
}

// WithAIIntegration adds integrations/csm-ai-integration-service to the
// checks. ai may be nil (an untyped nil, as for NewHealthHandler's own
// optional pingers): it then reports not_configured.
func (h *HealthHandler) WithAIIntegration(ai HealthPinger) *HealthHandler {
	h.ai = ai
	return h
}

// GetHealthDependencies serves the cached result when it's younger than
// dependencyHealthCacheTTL, else recomputes it (see HealthHandler.mu's own
// doc comment for why recomputation holds the lock for its full duration).
// Reports 200 when every dependency is ok, 503 when any is down. No auth
// check — this route is registered directly on the mux and is exempt from
// the Auth middleware, the same way GET /health is (see cmd/server/main.go
// and internal/middleware/auth.go).
func (h *HealthHandler) GetHealthDependencies(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if healthNow().Sub(h.cachedAt) >= dependencyHealthCacheTTL {
		// context.Background(), not r.Context(): this computation is shared
		// with every other request that queues behind h.mu while it runs, so
		// it must not be cancelled just because the request that happened to
		// trigger it disconnected. Each individual check still has its own
		// bounded timeout via checkDependency.
		h.cachedResponse, h.cachedStatusCode = computeHealthDependencies(context.Background(), h)
		h.cachedAt = healthNow()
	}
	resp, statusCode := h.cachedResponse, h.cachedStatusCode
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// computeHealthDependencies checks every listed dependency at the same time,
// each under its own timeout. The status is "degraded" (503) when any is
// down; one that is not configured does not count as down.
func computeHealthDependencies(ctx context.Context, h *HealthHandler) (HealthDependenciesResponse, int) {
	checks := []struct {
		name   string
		pinger HealthPinger
	}{
		{"SCIM Service", h.scim},
		{"Updates Service", h.updates},
		{"CSM Notification Service", h.notification},
		{"CSM Integration Service", h.integration},
		{"Engineering Entity Service", h.engineering},
		{"CSM AI Integration Service", h.ai},
	}

	deps := make([]DependencyHealth, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func(i int, name string, pinger HealthPinger) {
			defer wg.Done()
			deps[i] = checkDependency(ctx, name, pinger)
		}(i, c.name, c.pinger)
	}
	wg.Wait()

	overall := dependencyStatusOK
	statusCode := http.StatusOK
	for _, d := range deps {
		if d.Status == dependencyStatusDown {
			overall = "degraded"
			statusCode = http.StatusServiceUnavailable
			break
		}
	}
	return HealthDependenciesResponse{Status: overall, Dependencies: deps}, statusCode
}

func checkDependency(ctx context.Context, name string, pinger HealthPinger) DependencyHealth {
	if pinger == nil {
		return DependencyHealth{Name: name, Status: dependencyStatusNotConfigured}
	}
	ctx, cancel := context.WithTimeout(ctx, dependencyHealthCheckTimeout)
	defer cancel()
	if err := pinger.Health(ctx); err != nil {
		return DependencyHealth{Name: name, Status: dependencyStatusDown}
	}
	return DependencyHealth{Name: name, Status: dependencyStatusOK}
}
