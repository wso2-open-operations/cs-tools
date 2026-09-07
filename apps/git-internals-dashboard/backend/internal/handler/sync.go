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
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/jobs"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sync"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncHandler serves POST /sync/runs and GET /sync/status (SPEC §6.8/§6.9).
// Shares the same *jobs.Lock as the recompute scheduler, so a manual sync
// and a scheduled tick never interleave on this replica or any other.
type SyncHandler struct {
	pool        *pgxpool.Pool
	cfg         *config.AppConfig
	lock        *jobs.Lock
	runtime     *ingest.RuntimeConfig
	githubToken string
}

// NewSyncHandler creates a SyncHandler. githubToken may be empty — POST
// /sync/runs then always responds 400 sync_token_missing.
func NewSyncHandler(pool *pgxpool.Pool, cfg *config.AppConfig, lock *jobs.Lock, runtime *ingest.RuntimeConfig, githubToken string) *SyncHandler {
	return &SyncHandler{pool: pool, cfg: cfg, lock: lock, runtime: runtime, githubToken: strings.TrimSpace(githubToken)}
}

// postSyncRunsDeadline is a generous cap for the whole synchronous
// POST /sync/runs handler — well above the server's default 30s
// WriteTimeout/ReadTimeout, since a real sync fetches issue detail per
// updated issue with a 150ms courtesy delay plus GraphQL round trips and can
// easily exceed 30s over a few hundred issues (AUDIT-FINDINGS A4).
const postSyncRunsDeadline = 15 * time.Minute

// PostSyncRuns handles POST /sync/runs: triggers an incremental sync
// followed by an immediate recompute tick, under the job lock. Runs
// synchronously. 409 sync_in_progress when the lock is busy; 400
// sync_token_missing when GITHUB_TOKEN is unset.
//
// The server's global 30s Read/WriteTimeout is too short for this one route:
// without extending it, a real sync completes and commits server-side but
// net/http closes the connection before the response is written, so the
// client sees a network error and the operator retries a sync that already
// ran. Extend both deadlines for this route only; every other route keeps
// the global 30s.
func (h *SyncHandler) PostSyncRuns(w http.ResponseWriter, r *http.Request) {
	if h.githubToken == "" {
		apierror.Write(w, http.StatusBadRequest, apierror.CodeSyncTokenMissing,
			"GITHUB_TOKEN is not configured — manual sync needs a fine-grained PAT with Issues:Read + Projects:Read.")
		return
	}

	rc := http.NewResponseController(w)
	deadline := time.Now().Add(postSyncRunsDeadline)
	if err := rc.SetWriteDeadline(deadline); err != nil {
		slog.WarnContext(r.Context(), "sync: could not extend write deadline; falling back to the server default", "err", err)
	}
	if err := rc.SetReadDeadline(deadline); err != nil {
		slog.WarnContext(r.Context(), "sync: could not extend read deadline; falling back to the server default", "err", err)
	}

	// Chosen deliberately: run to completion even if the client disconnects
	// (tab closed, gateway idle timeout) rather than cancel a half-finished
	// sync. Canceling mid-run is safe (the watermark only advances on
	// success), but wasteful — it discards GitHub API calls already spent
	// and leaves the repo needing a full retry on the next manual sync.
	runCtx := context.WithoutCancel(r.Context())

	client := github.NewClient(h.githubToken)
	summary, ran, err := jobs.TryRun(runCtx, h.lock, func(ctx context.Context) (sync.Summary, error) {
		s, err := sync.Run(ctx, h.pool, client, h.cfg, h.runtime)
		if err != nil {
			return sync.Summary{}, err
		}
		if _, err := jobs.RunTickOnce(ctx, h.pool, h.runtime, time.Now()); err != nil {
			return sync.Summary{}, err
		}
		return s, nil
	})
	if err != nil {
		apierror.Internal(w, r, "sync run failed", err)
		return
	}
	if !ran {
		apierror.Write(w, http.StatusConflict, apierror.CodeSyncInProgress, "a sync is already in progress")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type repoWatermarkWire struct {
	Repo         string     `json:"repo"`
	LastSyncedAt *time.Time `json:"lastSyncedAt"`
}

type lastRunWire struct {
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	FinishedAt      *time.Time `json:"finishedAt"`
	IssuesProcessed int        `json:"issuesProcessed"`
	Error           *string    `json:"error"`
}

type syncStatusWire struct {
	Running bool                `json:"running"`
	Repos   []repoWatermarkWire `json:"repos"`
	LastRun *lastRunWire        `json:"lastRun"`
}

// GetSyncStatus handles GET /sync/status: this replica's in-process lock
// flag, plus per-repo watermarks and the last run (both shared across
// replicas via the DB).
func (h *SyncHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repos, err := h.fetchRepoWatermarks(ctx)
	if err != nil {
		apierror.Internal(w, r, "fetch repo watermarks failed", err)
		return
	}
	lastRun, err := h.fetchLastRun(ctx)
	if err != nil {
		apierror.Internal(w, r, "fetch last sync run failed", err)
		return
	}

	writeJSON(w, http.StatusOK, syncStatusWire{Running: h.lock.Running(), Repos: repos, LastRun: lastRun})
}

func (h *SyncHandler) fetchRepoWatermarks(ctx context.Context) ([]repoWatermarkWire, error) {
	rows, err := h.pool.Query(ctx, `SELECT owner, name, last_synced_at FROM repositories WHERE enabled = true ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repos := make([]repoWatermarkWire, 0)
	for rows.Next() {
		var owner, name string
		var lastSyncedAt *time.Time
		if err := rows.Scan(&owner, &name, &lastSyncedAt); err != nil {
			return nil, err
		}
		repos = append(repos, repoWatermarkWire{Repo: owner + "/" + name, LastSyncedAt: lastSyncedAt})
	}
	return repos, rows.Err()
}

func (h *SyncHandler) fetchLastRun(ctx context.Context) (*lastRunWire, error) {
	var lr lastRunWire
	err := h.pool.QueryRow(ctx, `
		SELECT kind, status, finished_at, issues_processed, error
		FROM sync_runs
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&lr.Kind, &lr.Status, &lr.FinishedAt, &lr.IssuesProcessed, &lr.Error)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &lr, nil
}
