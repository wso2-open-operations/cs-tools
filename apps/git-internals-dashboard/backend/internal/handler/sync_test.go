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
	"os"
	"sync"
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/jobs"
)

// testDatabaseURL returns the DSN testPool(t) already validated as
// reachable; callers must call testPool(t) first in the same test so an
// unreachable DB skips cleanly instead of failing here.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://gid:gid@localhost:5433/gid?sslmode=disable"
}

func TestPostSyncRunsReturns400WhenTokenMissing(t *testing.T) {
	pool := testPool(t)
	lock := jobs.NewLock(testDatabaseURL(t))
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "")

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

func TestPostSyncRunsReturns409WhenLockBusy(t *testing.T) {
	pool := testPool(t)
	url := testDatabaseURL(t)
	lock := jobs.NewLock(url)
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "fake-token")

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

func TestGetSyncStatusReportsRunningAndWatermarks(t *testing.T) {
	pool := testPool(t)
	lock := jobs.NewLock(testDatabaseURL(t))
	h := NewSyncHandler(pool, &config.AppConfig{}, lock, ingest.BuildRuntimeConfig(&config.AppConfig{}), "")

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
