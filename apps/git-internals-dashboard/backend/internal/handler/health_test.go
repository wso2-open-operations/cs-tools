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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/db"
)

type readyBody struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status    string `json:"status"`
		LatencyMs int64  `json:"latencyMs"`
	} `json:"checks"`
	Pool *struct {
		Acquired           int32 `json:"acquired"`
		Idle               int32 `json:"idle"`
		Total              int32 `json:"total"`
		Max                int32 `json:"max"`
		UtilizationPercent int   `json:"utilizationPercent"`
	} `json:"pool"`
}

// TestGetHealthzAlwaysReturnsOk verifies liveness never touches the pool —
// a nil pool must not panic or change the response.
func TestGetHealthzAlwaysReturnsOk(t *testing.T) {
	h := NewHealthHandler(nil, appconfig.Default().Readiness)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.GetHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !body.Ok {
		t.Error("expected ok=true")
	}
}

// TestGetReadyzReadyPath verifies a healthy DB reports 200, status=ready,
// JSON content type, and populated pool numbers sourced from pool.Stat().
func TestGetReadyzReadyPath(t *testing.T) {
	pool := testPool(t)
	h := NewHealthHandler(pool, appconfig.Default().Readiness)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.GetReadyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body readyBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Status != "ready" {
		t.Errorf("expected status=ready, got %q", body.Status)
	}
	if body.Checks["database"].Status != "ok" {
		t.Errorf("expected checks.database.status=ok, got %+v", body.Checks["database"])
	}
	if body.Pool == nil || body.Pool.Max != pool.Stat().MaxConns() {
		t.Errorf("expected pool.max to mirror pool.Stat().MaxConns(), got %+v", body.Pool)
	}
}

// TestGetReadyzUnreachableDBReturns503 verifies a closed pool is reported as
// not_ready/unreachable rather than panicking or hanging.
func TestGetReadyzUnreachableDBReturns503(t *testing.T) {
	pool := testPool(t)
	pool.Close()
	h := NewHealthHandler(pool, appconfig.Default().Readiness)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.GetReadyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body readyBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Status != "not_ready" {
		t.Errorf("expected status=not_ready, got %q", body.Status)
	}
	if got := body.Checks["database"].Status; got != "unreachable" && got != "timeout" {
		t.Errorf("expected checks.database.status in {unreachable,timeout}, got %q", got)
	}
}

// TestGetReadyzResponseNeverLeaksConnectionDetails verifies the failure body
// never contains the DSN, host, port, or raw driver error text — only the
// enum reason code. The real error must still reach the log (verified by
// inspection, not asserted here — see apierror.Internal's own convention).
func TestGetReadyzResponseNeverLeaksConnectionDetails(t *testing.T) {
	pool := testPool(t)
	pool.Close()
	h := NewHealthHandler(pool, appconfig.Default().Readiness)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.GetReadyz(rec, req)

	raw := rec.Body.String()
	for _, forbidden := range []string{"postgres://", "5433", "localhost", "gid:gid"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("response body leaks connection detail %q: %s", forbidden, raw)
		}
	}
	var body readyBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	switch body.Checks["database"].Status {
	case "unreachable", "timeout", "pool_saturated":
	default:
		t.Errorf("expected an enum reason code, got %q (possible raw error leak)", body.Checks["database"].Status)
	}
}

// TestGetReadyzCachesWithinTTLAndRecomputesAfter verifies that a result
// computed once is reused for cacheTTLSeconds, then recomputed after it
// elapses. Asserted by observing the handler's internal cachedAt timestamp
// change (or not), rather than instrumenting the DB call itself.
func TestGetReadyzCachesWithinTTLAndRecomputesAfter(t *testing.T) {
	pool := testPool(t)
	cfg := appconfig.Default().Readiness
	cfg.CacheTTLSeconds = 1
	h := NewHealthHandler(pool, cfg)

	get := func() {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		h.GetReadyz(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	}

	get()
	firstComputedAt := h.cachedAt

	get()
	if !h.cachedAt.Equal(firstComputedAt) {
		t.Error("expected the cached result to be reused within cacheTTLSeconds, but it was recomputed")
	}

	time.Sleep(1100 * time.Millisecond)
	get()
	if h.cachedAt.Equal(firstComputedAt) {
		t.Error("expected recompute after cacheTTLSeconds elapsed, but cachedAt did not advance")
	}
}

// TestGetReadyzPoolSaturatedSkipsPing verifies that with
// failOnPoolSaturation enabled and the threshold met, GetReadyz returns 503
// pool_saturated immediately, without attempting a ping. Proven by holding
// the pool's only connection open (so a real ping would block until
// timeoutSeconds) and asserting the handler still returns well within that
// deadline.
func TestGetReadyzPoolSaturatedSkipsPing(t *testing.T) {
	testPool(t) // validates Postgres is reachable; skips cleanly otherwise
	url := testDatabaseURL(t)

	maxConns := int32(1)
	ctx := context.Background()
	saturated, err := db.NewPoolWithConfig(ctx, url, appconfig.Database{MaxConns: &maxConns})
	if err != nil {
		t.Fatalf("dedicated single-conn pool: %v", err)
	}
	t.Cleanup(saturated.Close)

	conn, err := saturated.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire the only connection: %v", err)
	}
	defer conn.Release()

	cfg := appconfig.Default().Readiness
	cfg.FailOnPoolSaturation = true
	cfg.PoolSaturationThresholdPercent = 100
	h := NewHealthHandler(saturated, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.GetReadyz(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if elapsed >= time.Duration(cfg.TimeoutSeconds)*time.Second {
		t.Errorf("expected an immediate fail-fast response, took %s (>= timeoutSeconds, suggesting a ping was attempted)", elapsed)
	}
	var body readyBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Checks["database"].Status != "pool_saturated" {
		t.Errorf("expected checks.database.status=pool_saturated, got %+v", body.Checks["database"])
	}
	if body.Pool == nil || body.Pool.UtilizationPercent < 100 {
		t.Errorf("expected pool.utilizationPercent >= 100, got %+v", body.Pool)
	}
}

// TestReadyzDrainingReturns503WhileHealthzStaysOk verifies BeginDraining
// flips /readyz to 503 "draining" immediately (no DB work attempted) while
// /healthz — liveness — is completely unaffected.
func TestReadyzDrainingReturns503WhileHealthzStaysOk(t *testing.T) {
	pool := testPool(t)
	h := NewHealthHandler(pool, appconfig.Default().Readiness)
	h.BeginDraining()

	readyReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	h.GetReadyz(readyRec, readyReq)

	if readyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", readyRec.Code, readyRec.Body.String())
	}
	var body readyBody
	if err := json.Unmarshal(readyRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Status != "draining" {
		t.Errorf("expected status=draining, got %q", body.Status)
	}

	healthReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	h.GetHealthz(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Errorf("expected /healthz to stay 200 while draining, got %d", healthRec.Code)
	}
}
