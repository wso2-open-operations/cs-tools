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

package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Overridable only by tests, so a keyset-pagination test can force multiple
// pages without seeding hundreds of rows.
var recomputePageSize = 200

// TickSummary reports what one RunTickOnce pass did.
type TickSummary struct {
	Processed       int
	StateCounts     map[string]int
	UnknownStatuses map[string]int // status -> occurrence count
}

// recomputeIssue is one issue's frozen-since-last-sync state plus its full
// event timeline, as read for one recompute pass.
type recomputeIssue struct {
	ID              int32
	RepositoryID    int32
	Priority        *string
	CurrentStatus   *string
	CurrentStatusAt *time.Time
	Events          []sla.StatusEvent
}

// RunTickOnce is one recompute pass (port of v3's runTickOnce, SPEC §8.2).
// It does NOT call GitHub: currentStatus, priority, and the event log stay
// frozen at their last-synced values — this only advances the open interval
// of already-known state (the issue_sla projection and today's sla_snapshots
// row). SyncRun/watermark belong to internal/sync.
func RunTickOnce(ctx context.Context, pool *pgxpool.Pool, runtime *ingest.RuntimeConfig, now time.Time) (TickSummary, error) {
	snapshotDate := startOfUTCDay(now)
	unknownStatuses := make(map[string]int)
	stateCounts := make(map[string]int)
	processed := 0

	trackStatus := func(status *string) {
		if status == nil || runtime.KnownNames[*status] {
			return
		}
		unknownStatuses[*status]++
	}

	var lastID int32
	for {
		// Keyset pagination (AUDIT-FINDINGS B1): OFFSET re-scans and discards
		// every prior row on each page, making a full tick O(N²/pageSize).
		// Walking i.id > lastID does strictly less work per page and stays
		// correct even if rows shift between pages.
		page, err := fetchIssuePage(ctx, pool, recomputePageSize, lastID)
		if err != nil {
			return TickSummary{}, err
		}
		if len(page) == 0 {
			break
		}

		// Batch this page's writes (AUDIT-FINDINGS B2): 2 statements/issue as
		// separate round trips (up to 400 for a full page) dominates tick
		// latency. pgx.Batch pipelines them into one round trip — but a
		// pipeline terminated by a single Sync (what pool.SendBatch sends)
		// runs as ONE implicit transaction server-side: a genuine SQL error
		// anywhere in the page rolls back every statement in it, unlike the
		// old one-exec-per-issue loop where each statement committed on its
		// own (verified empirically: a batch with a constraint-violating
		// second statement left zero rows from the first). This is fine
		// here — every statement is idempotent and a failed page is retried
		// wholesale on the next tick — but it is a real behavior change, not
		// "isolation unchanged". Same SQL, same conflict clauses either way.
		batch := &pgx.Batch{}
		for _, issue := range page {
			currentStatus := runtime.Normalize(issue.CurrentStatus)
			trackStatus(currentStatus)

			events := make([]sla.StatusEvent, len(issue.Events))
			for i, e := range issue.Events {
				status := runtime.Normalize(e.Status)
				trackStatus(status)
				events[i] = sla.StatusEvent{Status: status, OccurredAt: e.OccurredAt}
			}

			slaEvents := sla.WithCurrentStatusBoundary(events, currentStatus, issue.CurrentStatusAt, now)
			result := sla.ComputeSla(issue.Priority, slaEvents, currentStatus, runtime.Cfg, now)

			queueUpdateIssueSla(batch, issue.ID, issue.Priority, result, now)
			// Recomputing today's row on every tick is idempotent and keeps
			// the hero spark delta live all day; historical rows are never
			// touched.
			queueUpsertSnapshot(batch, snapshotDate, issue, currentStatus, result)

			stateCounts[string(result.SlaState)]++
			processed++
		}
		if err := execRecomputeBatch(ctx, pool, batch, page); err != nil {
			return TickSummary{}, err
		}

		lastID = page[len(page)-1].ID
		if len(page) < recomputePageSize {
			break
		}
	}

	return TickSummary{Processed: processed, StateCounts: stateCounts, UnknownStatuses: unknownStatuses}, nil
}

func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func fetchIssuePage(ctx context.Context, pool *pgxpool.Pool, limit int, lastID int32) ([]recomputeIssue, error) {
	rows, err := pool.Query(ctx, `
		SELECT i.id, i.repository_id, i.priority, i.current_status, i.current_status_at
		FROM issues i
		JOIN repositories r ON r.id = i.repository_id
		WHERE r.enabled = true AND i.id > $2
		ORDER BY i.id ASC
		LIMIT $1
	`, limit, lastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []recomputeIssue
	ids := make([]int32, 0)
	for rows.Next() {
		var it recomputeIssue
		if err := rows.Scan(&it.ID, &it.RepositoryID, &it.Priority, &it.CurrentStatus, &it.CurrentStatusAt); err != nil {
			return nil, err
		}
		issues = append(issues, it)
		ids = append(ids, it.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return issues, nil
	}

	eventRows, err := pool.Query(ctx, `
		SELECT issue_id, status, occurred_at FROM issue_status_events
		WHERE issue_id = ANY($1)
		ORDER BY issue_id ASC, occurred_at ASC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer eventRows.Close()

	eventsByIssue := make(map[int32][]sla.StatusEvent)
	for eventRows.Next() {
		var issueID int32
		var status *string
		var occurredAt time.Time
		if err := eventRows.Scan(&issueID, &status, &occurredAt); err != nil {
			return nil, err
		}
		eventsByIssue[issueID] = append(eventsByIssue[issueID], sla.StatusEvent{Status: status, OccurredAt: occurredAt})
	}
	if err := eventRows.Err(); err != nil {
		return nil, err
	}

	for i := range issues {
		issues[i].Events = eventsByIssue[issues[i].ID]
	}
	return issues, nil
}

// queueUpdateIssueSla and queueUpsertSnapshot queue exactly the same
// statements updateIssueSla/upsertSnapshot used to run standalone; batching
// only changes how they're sent, never the SQL or conflict clauses.
func queueUpdateIssueSla(batch *pgx.Batch, issueID int32, priority *string, r sla.Result, now time.Time) {
	batch.Queue(`
		UPDATE issue_sla SET
			priority = $2, budget_hours = $3, consumed_hours = $4, remaining_hours = $5,
			pct_consumed = $6, sla_state = $7, sla_running = $8, computed_at = $9, computed_through = $10
		WHERE issue_id = $1
	`, issueID, priority, r.BudgetHours, r.ConsumedHours, r.RemainingHours, r.PctConsumed, string(r.SlaState), r.SlaRunning, now, now)
}

func queueUpsertSnapshot(batch *pgx.Batch, snapshotDate time.Time, issue recomputeIssue, currentStatus *string, r sla.Result) {
	batch.Queue(`
		INSERT INTO sla_snapshots (
			snapshot_date, issue_id, repository_id, priority, current_status,
			budget_hours, consumed_hours, remaining_hours, pct_consumed, sla_state, sla_running
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (snapshot_date, issue_id) DO UPDATE SET
			priority = $4, current_status = $5, budget_hours = $6, consumed_hours = $7,
			remaining_hours = $8, pct_consumed = $9, sla_state = $10, sla_running = $11
	`, snapshotDate, issue.ID, issue.RepositoryID, issue.Priority, currentStatus,
		r.BudgetHours, r.ConsumedHours, r.RemainingHours, r.PctConsumed, string(r.SlaState), r.SlaRunning)
}

// execRecomputeBatch sends one page's queued statements (2 per issue, in the
// same order they were queued: UPDATE issue_sla then INSERT sla_snapshots)
// and checks each result exactly as the pre-batch per-issue loop did.
//
// The RowsAffected()==0 check below is a defensive assertion, not a SQL
// error: pgx's BatchResults.Close drains and executes every remaining
// queued statement regardless of where this loop stops reading, so an early
// return here still lets the rest of the page's writes land (and commit,
// since nothing actually errored at the SQL level) — it only stops this
// function from reporting success. That's fine: the assertion should never
// fire in practice (ingest always creates issue_sla alongside issues), every
// write here is idempotent, and a tick that reports an error just gets
// retried wholesale.
func execRecomputeBatch(ctx context.Context, pool *pgxpool.Pool, batch *pgx.Batch, page []recomputeIssue) error {
	br := pool.SendBatch(ctx, batch)
	for _, issue := range page {
		tag, err := br.Exec() // UPDATE issue_sla
		if err != nil {
			_ = br.Close()
			return err
		}
		if tag.RowsAffected() == 0 {
			_ = br.Close()
			return fmt.Errorf("jobs: issue_sla row not found for issue %d (expected ingest to have created it)", issue.ID)
		}
		if _, err := br.Exec(); err != nil { // INSERT sla_snapshots
			_ = br.Close()
			return err
		}
	}
	return br.Close()
}

// Scheduler runs RunTickOnce on a fixed interval, guarded by a Lock so a
// tick never interleaves with a manual sync — on this replica or any other
// (port of v3's startRecomputeJob).
type Scheduler struct {
	pool     *pgxpool.Pool
	lock     *Lock
	runtime  *ingest.RuntimeConfig
	interval time.Duration
}

// NewScheduler builds a Scheduler. Call Start to begin ticking.
func NewScheduler(pool *pgxpool.Pool, lock *Lock, runtime *ingest.RuntimeConfig, interval time.Duration) *Scheduler {
	return &Scheduler{pool: pool, lock: lock, runtime: runtime, interval: interval}
}

// Start runs one tick immediately (self-heals a server started days after
// seeding) and then one every s.interval, until ctx is done. Runs in its own
// goroutine; Start returns immediately.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.tick(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *Scheduler) tick(ctx context.Context) {
	summary, ran, err := TryRun(ctx, s.lock, func(ctx context.Context) (TickSummary, error) {
		return RunTickOnce(ctx, s.pool, s.runtime, time.Now())
	})
	if err != nil {
		slog.ErrorContext(ctx, "recompute tick failed", "err", err)
		return
	}
	if !ran {
		slog.InfoContext(ctx, "recompute tick skipped: job lock busy")
		return
	}
	slog.InfoContext(ctx, "recompute tick complete",
		"processed", summary.Processed, "stateCounts", summary.StateCounts, "unknownStatusCount", len(summary.UnknownStatuses))
	if len(summary.UnknownStatuses) > 0 {
		slog.WarnContext(ctx, "recompute tick encountered unknown statuses — defaulting to pause + non-terminal",
			"unknownStatuses", summary.UnknownStatuses)
	}
}
