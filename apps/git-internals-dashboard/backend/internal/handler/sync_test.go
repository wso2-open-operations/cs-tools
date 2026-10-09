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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/jobs"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDatabaseURL returns the DSN testPool(t) already validated as
// reachable; callers must call testPool(t) first in the same test so an
// unreachable DB skips cleanly instead of failing here.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	return testdb.URL(t)
}

// TestPostSyncRunsReturns400WhenTokenMissing verifies POST /sync/runs
// rejects with 400 sync_token_missing when GITHUB_TOKEN is unset.
func TestPostSyncRunsReturns400WhenTokenMissing(t *testing.T) {
	pool := testPool(t)
	lock := jobs.NewLock(testDatabaseURL(t))
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "", time.Duration(appconfig.Default().Jobs.SyncRunDeadlineMinutes)*time.Minute, 0)

	req := httptest.NewRequest(http.MethodPost, "/sync/runs", nil)
	rec := httptest.NewRecorder()
	h.PostSyncRuns(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if body.Error.Code != apierror.CodeSyncTokenMissing {
		t.Errorf("expected code=%s, got %s", apierror.CodeSyncTokenMissing, body.Error.Code)
	}
}

// TestPostSyncRunsReturns409WhenLockBusy verifies a request that arrives
// while another goroutine holds the job lock gets 409 sync_in_progress
// instead of blocking or double-running the sync.
func TestPostSyncRunsReturns409WhenLockBusy(t *testing.T) {
	pool := testPool(t)
	url := testDatabaseURL(t)
	lock := jobs.NewLock(url)
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "fake-token", time.Duration(appconfig.Default().Jobs.SyncRunDeadlineMinutes)*time.Minute, 0)

	// Hold the lock via a concurrent TryRun that blocks until released.
	release := make(chan struct{})
	started := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = jobs.TryRun(context.Background(), lock, func(ctx context.Context) (struct{}, error) {
			close(started)
			<-release
			return struct{}{}, nil
		})
	}()
	<-started

	req := httptest.NewRequest(http.MethodPost, "/sync/runs", nil)
	rec := httptest.NewRecorder()
	h.PostSyncRuns(rec, req)

	close(release)
	wg.Wait()

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if body.Error.Code != apierror.CodeSyncInProgress {
		t.Errorf("expected code=%s, got %s", apierror.CodeSyncInProgress, body.Error.Code)
	}
}

// TestGetSyncStatusReportsRunningAndWatermarks verifies GET /sync/status
// reports running=false when no sync is in flight.
func TestGetSyncStatusReportsRunningAndWatermarks(t *testing.T) {
	pool := testPool(t)
	lock := jobs.NewLock(testDatabaseURL(t))
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "", time.Duration(appconfig.Default().Jobs.SyncRunDeadlineMinutes)*time.Minute, 0)

	req := httptest.NewRequest(http.MethodGet, "/sync/status", nil)
	rec := httptest.NewRecorder()
	h.GetSyncStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body syncStatusWire
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Running {
		t.Error("expected running=false when no sync is in flight")
	}
}

// insertSyncRunFinishedAgo inserts a repo-less sync_runs row that finished
// ago ago, removing it when the test ends.
func insertSyncRunFinishedAgo(t *testing.T, pool *pgxpool.Pool, ago time.Duration) {
	t.Helper()
	var id int32
	err := pool.QueryRow(context.Background(), `
		INSERT INTO sync_runs (kind, status, started_at, finished_at)
		VALUES ('manual', 'success', now() - $1::interval - interval '1 second', now() - $1::interval)
		RETURNING id`, fmt.Sprintf("%d milliseconds", ago.Milliseconds())).Scan(&id)
	if err != nil {
		t.Fatalf("insert sync run: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM sync_runs WHERE id = $1`, id) })
}

func clearSyncRuns(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM sync_runs`); err != nil {
		t.Fatalf("clear sync_runs: %v", err)
	}
}

func newCooldownHandler(t *testing.T, pool *pgxpool.Pool, cooldown time.Duration) *SyncHandler {
	t.Helper()
	return NewSyncHandler(pool, &config.AppConfig{}, jobs.NewLock(testDatabaseURL(t)), ingest.BuildRuntimeConfig(&config.AppConfig{}), "fake-token", time.Minute, cooldown)
}

func TestCooldownRemaining(t *testing.T) {
	pool := testPool(t)
	clearSyncRuns(t, pool)

	t.Run("no sync has ever run", func(t *testing.T) {
		got, err := newCooldownHandler(t, pool, 30*time.Second).cooldownRemaining(context.Background())
		if err != nil || got != 0 {
			t.Errorf("expected 0, nil; got %v, %v", got, err)
		}
	})
	t.Run("recent run leaves the rest of the window", func(t *testing.T) {
		insertSyncRunFinishedAgo(t, pool, 10*time.Second)
		got, err := newCooldownHandler(t, pool, 30*time.Second).cooldownRemaining(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got < 15*time.Second || got > 20*time.Second {
			t.Errorf("expected about 20s remaining, got %v", got)
		}
	})
	t.Run("zero cooldown disables the check", func(t *testing.T) {
		got, err := newCooldownHandler(t, pool, 0).cooldownRemaining(context.Background())
		if err != nil || got != 0 {
			t.Errorf("expected 0, nil; got %v, %v", got, err)
		}
	})
	t.Run("run older than the window", func(t *testing.T) {
		clearSyncRuns(t, pool)
		insertSyncRunFinishedAgo(t, pool, 45*time.Second)
		got, err := newCooldownHandler(t, pool, 30*time.Second).cooldownRemaining(context.Background())
		if err != nil || got != 0 {
			t.Errorf("expected 0, nil; got %v, %v", got, err)
		}
	})
}

// TestPostSyncRunsReturns429DuringCooldown verifies a request arriving
// within the cooldown of the last finished sync is rejected with 429
// sync_cooldown and a Retry-After header, without running a sync.
func TestPostSyncRunsReturns429DuringCooldown(t *testing.T) {
	pool := testPool(t)
	clearSyncRuns(t, pool)
	insertSyncRunFinishedAgo(t, pool, 5*time.Second)
	h := newCooldownHandler(t, pool, 30*time.Second)

	rec := httptest.NewRecorder()
	h.PostSyncRuns(rec, httptest.NewRequest(http.MethodPost, "/sync/runs", nil))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs < 20 || secs > 26 {
		t.Errorf("expected Retry-After of about 25s, got %q", rec.Header().Get("Retry-After"))
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON envelope: %v", err)
	}
	if body.Error.Code != apierror.CodeSyncCooldown {
		t.Errorf("expected code=%s, got %s", apierror.CodeSyncCooldown, body.Error.Code)
	}
}

func TestGetSyncStatusReportsManualSyncCooldownRemaining(t *testing.T) {
	pool := testPool(t)
	clearSyncRuns(t, pool)
	h := newCooldownHandler(t, pool, 30*time.Second)
	status := func() syncStatusWire {
		rec := httptest.NewRecorder()
		h.GetSyncStatus(rec, httptest.NewRequest(http.MethodGet, "/sync/status", nil))
		var body syncStatusWire
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		return body
	}

	if got := status().ManualSyncCooldownRemainingSeconds; got != 0 {
		t.Errorf("expected 0 with no sync history, got %d", got)
	}
	insertSyncRunFinishedAgo(t, pool, 10*time.Second)
	if got := status().ManualSyncCooldownRemainingSeconds; got < 15 || got > 20 {
		t.Errorf("expected about 20s remaining, got %d", got)
	}
}
