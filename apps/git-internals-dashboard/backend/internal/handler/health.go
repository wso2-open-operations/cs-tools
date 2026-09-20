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
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler serves GET /healthz (liveness) and GET /readyz (readiness).
//
// Liveness never touches the database — see GetHealthz. Readiness performs
// a bounded pool.Ping plus a pool.Stat() snapshot, cached briefly behind a
// mutex to collapse concurrent probes (kubelet + gateway + any external
// monitor) into one DB round trip, since every uncached check borrows a
// pooled connection.
//
// Rejected alternative: a second pgxpool reserved for health checks, to
// guarantee probe headroom under saturation. That doubles connection
// accounting for a probe that is allowed to fail when the pool is
// exhausted — not worth it here.
type HealthHandler struct {
	pool *pgxpool.Pool
	cfg  appconfig.Readiness

	mu       sync.Mutex
	cached   readyResponse
	cachedAt time.Time

	draining atomic.Bool
}

// NewHealthHandler creates a HealthHandler. pool may be nil if only
// GetHealthz will ever be called (e.g. in a liveness-only test).
func NewHealthHandler(pool *pgxpool.Pool, cfg appconfig.Readiness) *HealthHandler {
	return &HealthHandler{pool: pool, cfg: cfg}
}

// BeginDraining marks this replica as shutting down: subsequent GET /readyz
// calls return 503 "draining" immediately, before any DB work. GetHealthz is
// unaffected — the process is still alive and must not be killed.
func (h *HealthHandler) BeginDraining() {
	h.draining.Store(true)
}

// GetHealthz is liveness-only: it reports the process is up and serving,
// not that its dependencies are healthy — it never touches the database, so
// a booted process with a dead DB still reports {"ok":true}.
func (h *HealthHandler) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// checkResult is one dependency's readiness detail.
type checkResult struct {
	Status string `json:"status"`
	// LatencyMs deliberately has no `,omitempty`: on a fast loopback
	// Postgres this is routinely 0, and omitempty would silently drop it
	// from the documented response shape.
	LatencyMs int64 `json:"latencyMs"`
}

// poolStats is one pool.Stat() snapshot — all four numbers come from the
// same call so they stay mutually consistent.
type poolStats struct {
	Acquired           int32 `json:"acquired"`
	Idle               int32 `json:"idle"`
	Total              int32 `json:"total"`
	Max                int32 `json:"max"`
	UtilizationPercent int   `json:"utilizationPercent"`
}

// readyResponse is GET /readyz's body shape on both 200 and 503 — see
// response.go's package comment for why this bypasses the apierror
// envelope.
type readyResponse struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks,omitempty"`
	Pool   *poolStats             `json:"pool,omitempty"`
}

// GetReadyz handles GET /readyz. Draining short-circuits before any DB
// work; otherwise the (possibly cached) readiness result decides the status
// code.
func (h *HealthHandler) GetReadyz(w http.ResponseWriter, r *http.Request) {
	if h.draining.Load() {
		writeJSON(w, http.StatusServiceUnavailable, readyResponse{Status: "draining"})
		return
	}

	resp := h.check(r.Context())
	status := http.StatusOK
	if resp.Status != "ready" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

// check returns the cached readiness result if still within cfg.CacheTTLSeconds,
// otherwise computes a fresh one and caches it. The mutex is held across the
// whole compute (not just the cache read/write) so that truly concurrent
// callers block on the first one's in-flight DB round trip rather than each
// starting their own — this is what collapses multiple probers into one
// pool.Ping.
func (h *HealthHandler) check(ctx context.Context) readyResponse {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg.CacheTTLSeconds > 0 && time.Since(h.cachedAt) < time.Duration(h.cfg.CacheTTLSeconds)*time.Second {
		return h.cached
	}

	resp := h.computeReadiness(ctx)
	h.cached = resp
	h.cachedAt = time.Now()
	return resp
}

// computeReadiness performs the actual check: a pool.Stat() snapshot, a
// saturation fail-fast (skips the ping entirely — queueing a probe behind an
// already-exhausted pool only makes the exhaustion worse), then a bounded
// pool.Ping.
func (h *HealthHandler) computeReadiness(ctx context.Context) readyResponse {
	stat := h.pool.Stat()
	acquired := stat.AcquiredConns()
	maxConns := stat.MaxConns()

	utilization := 0
	if maxConns > 0 {
		utilization = int(int64(acquired) * 100 / int64(maxConns))
	}
	pool := &poolStats{
		Acquired:           acquired,
		Idle:               stat.IdleConns(),
		Total:              stat.TotalConns(),
		Max:                maxConns,
		UtilizationPercent: utilization,
	}

	if h.cfg.FailOnPoolSaturation && utilization >= h.cfg.PoolSaturationThresholdPercent {
		return readyResponse{
			Status: "not_ready",
			Checks: map[string]checkResult{"database": {Status: "pool_saturated"}},
			Pool:   pool,
		}
	}

	// The readiness result is shared, cached state — the ping must not
	// inherit one particular caller's cancellation (e.g. a prober's own
	// client timing out), or an unrelated disconnect would poison the
	// cache for every other prober for the rest of cacheTTLSeconds.
	timeout := time.Duration(h.cfg.TimeoutSeconds) * time.Second
	pingCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	start := time.Now()
	err := h.pool.Ping(pingCtx)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		slog.ErrorContext(ctx, "readiness: database ping failed", "err", err)
		status := "unreachable"
		if errors.Is(err, context.DeadlineExceeded) {
			status = "timeout"
		}
		return readyResponse{
			Status: "not_ready",
			Checks: map[string]checkResult{"database": {Status: status, LatencyMs: latencyMs}},
			Pool:   pool,
		}
	}

	return readyResponse{
		Status: "ready",
		Checks: map[string]checkResult{"database": {Status: "ok", LatencyMs: latencyMs}},
		Pool:   pool,
	}
}
