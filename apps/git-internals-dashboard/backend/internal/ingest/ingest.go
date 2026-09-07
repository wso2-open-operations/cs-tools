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

package ingest

import (
	"context"
	"crypto/sha1" // #nosec G505 -- not for security; a stable, short dedupe key over public GitHub-side identifiers
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pair is one GitHub issue's search-result node plus its fetched detail
// (timeline + project statuses) — the unit ingest works on.
type Pair struct {
	Node   github.IssueNode
	Detail github.IssueDetail
}

// RepoRef is the subset of repository identity ingest needs to compute a
// stable dedupe key and scope the project-status lookup.
type RepoRef struct {
	Owner           string
	Name            string
	GithubProjectID string
}

// Context is everything IngestIssue needs beyond the (node, detail) pair
// itself. This function is the ONLY write path for GitHub-derived data
// (SPEC §8.5): the seed, the incremental sync, and any future webhook
// handler all call it, and it must not know or care which transport
// produced pair — that is the entire webhook-readiness requirement (D6). Do
// not implement any webhook route.
type Context struct {
	RepositoryID int32
	SlaProjectID int32 // DB projects.id
	Repo         RepoRef
	Runtime      *RuntimeConfig
	Source       string // "github" | "synthetic"
	Now          time.Time
}

// Result summarizes what IngestIssue did, for the caller's logging/metrics —
// never persisted itself.
type Result struct {
	Created         bool
	EventsInserted  int
	IssueID         int32
	Priority        *string
	SlaEvents       []sla.StatusEvent
	SlaState        sla.SlaState
	UnknownStatuses []string
}

var priorityRe = regexp.MustCompile(`^Priority/(.+)$`)

// extractPriority reads the first "Priority/<tier>" label and discards the
// rest — labels are read transiently to derive priority only (SPEC's
// non-negotiable #1: no labels are ever persisted).
func extractPriority(labels []string) *string {
	for _, l := range labels {
		if m := priorityRe.FindStringSubmatch(l); m != nil {
			return &m[1]
		}
	}
	return nil
}

// dedupeKey is stable across reseeds/incremental syncs: keyed on GitHub-side
// identifiers (repo/name/number/project + transition), not the local
// autoincrement id.
func dedupeKey(repoOwner, repoName string, githubNumber int, githubProjectID, occurredAt string, prev, status *string) string {
	p, s := "", ""
	if prev != nil {
		p = *prev
	}
	if status != nil {
		s = *status
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%s/%s#%d|%s|%s|%s|%s", repoOwner, repoName, githubNumber, githubProjectID, occurredAt, p, s))) // #nosec G401 -- not for security; a stable, short dedupe key over public GitHub-side identifiers
	return hex.EncodeToString(sum[:])
}

// normalizedEvent is one timeline event after alias normalization, still
// carrying its raw GitHub createdAt string (used verbatim in the dedupe key
// and parsed into a time.Time only where SLA math needs one).
type normalizedEvent struct {
	CreatedAt      string
	PreviousStatus *string
	Status         *string
}

// IngestIssue is the transport-agnostic write path for one GitHub issue
// (port of v3's ingestIssuePair, SPEC §8.5): priority extraction, scoped
// current status, alias normalization, a guarded leading "derived" event,
// event insert with dedupe, issue upsert, boundary reconciliation, computeSla,
// and the issue_sla upsert.
func IngestIssue(ctx context.Context, pool *pgxpool.Pool, pair Pair, ictx Context) (Result, error) {
	node, detail := pair.Node, pair.Detail
	normalize := ictx.Runtime.Normalize

	priority := extractPriority(node.Labels)

	// Current status scoped to THIS repo's configured project.
	var scoped *github.ProjectStatus
	for i := range detail.ProjectStatuses {
		if detail.ProjectStatuses[i].ProjectID == ictx.Repo.GithubProjectID {
			scoped = &detail.ProjectStatuses[i]
			break
		}
	}
	var rawCurrentStatus, currentStatusAtRaw, itemCreatedAtRaw *string
	if scoped != nil {
		rawCurrentStatus = scoped.Status
		currentStatusAtRaw = scoped.StatusUpdatedAt
		itemCreatedAtRaw = scoped.ItemCreatedAt
	}
	currentStatus := normalize(rawCurrentStatus)

	// Normalize formatting variants (e.g. "Re-Opened" -> "Reopened") before
	// anything downstream sees them; canonical names are what gets persisted.
	normalizedEvents := make([]normalizedEvent, len(detail.Events))
	for i, e := range detail.Events {
		normalizedEvents[i] = normalizedEvent{
			CreatedAt:      e.CreatedAt,
			PreviousStatus: normalize(e.PreviousStatus),
			Status:         normalize(e.Status),
		}
	}

	var unknownStatuses []string
	unknownSeen := make(map[string]bool)
	trackStatus := func(status *string) {
		if status == nil || ictx.Runtime.KnownNames[*status] || unknownSeen[*status] {
			return
		}
		unknownSeen[*status] = true
		unknownStatuses = append(unknownStatuses, *status)
	}
	trackStatus(currentStatus)
	for _, e := range normalizedEvents {
		trackStatus(e.PreviousStatus)
		trackStatus(e.Status)
	}

	// Synthesize a leading board-add event for the stretch before the first
	// recorded transition, when the first event demonstrably followed a
	// prior status. Guarded: absent/null itemCreatedAt or an itemCreatedAt
	// that isn't strictly before the first event => no-op (pre-first-event
	// time stays unaccounted).
	var leadingEvent *normalizedEvent
	if len(normalizedEvents) > 0 {
		first := normalizedEvents[0]
		if first.PreviousStatus != nil && itemCreatedAtRaw != nil {
			itemCreatedAt, err1 := time.Parse(time.RFC3339, *itemCreatedAtRaw)
			firstCreatedAt, err2 := time.Parse(time.RFC3339, first.CreatedAt)
			if err1 == nil && err2 == nil && itemCreatedAt.Before(firstCreatedAt) {
				leadingEvent = &normalizedEvent{CreatedAt: *itemCreatedAtRaw, PreviousStatus: nil, Status: first.PreviousStatus}
			}
		}
	}

	// Events actually written to the log (source of truth): real/synthetic
	// timeline events plus the derived leading event, if any.
	persistedEvents := normalizedEvents
	if leadingEvent != nil {
		persistedEvents = append([]normalizedEvent{*leadingEvent}, normalizedEvents...)
	}

	slaInputEvents := make([]sla.StatusEvent, len(persistedEvents))
	for i, e := range persistedEvents {
		t, err := time.Parse(time.RFC3339, e.CreatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("ingest: parse event createdAt %q: %w", e.CreatedAt, err)
		}
		slaInputEvents[i] = sla.StatusEvent{Status: e.Status, OccurredAt: t}
	}
	var currentStatusAt *time.Time
	if currentStatusAtRaw != nil {
		t, err := time.Parse(time.RFC3339, *currentStatusAtRaw)
		if err != nil {
			return Result{}, fmt.Errorf("ingest: parse currentStatusAt %q: %w", *currentStatusAtRaw, err)
		}
		currentStatusAt = &t
	}

	// SLA event list (ascending), reconciled with the project-scoped current
	// status — this is what both the current projection and the snapshot
	// replay walk.
	slaEvents := sla.WithCurrentStatusBoundary(slaInputEvents, currentStatus, currentStatusAt, ictx.Now)

	githubCreatedAt, err := time.Parse(time.RFC3339, node.CreatedAt)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: parse issue createdAt %q: %w", node.CreatedAt, err)
	}
	githubUpdatedAt, err := time.Parse(time.RFC3339, node.UpdatedAt)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: parse issue updatedAt %q: %w", node.UpdatedAt, err)
	}
	var githubClosedAt *time.Time
	if node.ClosedAt != nil {
		t, err := time.Parse(time.RFC3339, *node.ClosedAt)
		if err != nil {
			return Result{}, fmt.Errorf("ingest: parse issue closedAt %q: %w", *node.ClosedAt, err)
		}
		githubClosedAt = &t
	}

	var existingID *int32
	err = pool.QueryRow(ctx, `SELECT id FROM issues WHERE repository_id = $1 AND github_number = $2`,
		ictx.RepositoryID, node.Number).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, fmt.Errorf("ingest: check existing issue: %w", err)
	}

	var issueID int32
	err = pool.QueryRow(ctx, `
		INSERT INTO issues (
			repository_id, github_number, state, html_url, priority,
			current_status, current_status_at, github_created_at, github_closed_at,
			github_updated_at, last_synced_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (repository_id, github_number) DO UPDATE SET
			state = $3, html_url = $4, priority = $5, current_status = $6,
			current_status_at = $7, github_created_at = $8, github_closed_at = $9,
			github_updated_at = $10, last_synced_at = $11
		RETURNING id
	`, ictx.RepositoryID, node.Number, node.State, node.URL, priority,
		currentStatus, currentStatusAt, githubCreatedAt, githubClosedAt,
		githubUpdatedAt, ictx.Now).Scan(&issueID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: upsert issue: %w", err)
	}

	// Persist the event log (source of truth). ON CONFLICT DO NOTHING handles
	// real GitHub data where two events can share the same timestamp +
	// transition, and makes reseeds/incremental syncs safe to run repeatedly.
	// The leading event (if any) is marked source "derived" so every future
	// consumer reproduces the same numbers from the DB alone.
	//
	// Batched (AUDIT-FINDINGS B2): one round trip for the whole event log
	// instead of one Exec per event — same SQL, same conflict clause, same
	// per-row RowsAffected counting. Incidental benefit: a pgx batch sent
	// this way runs as one implicit transaction, so this issue's event log
	// now writes all-or-nothing instead of possibly-partial on a mid-loop
	// failure — a slice of B3's "transaction per issue" idea, delivered here
	// without wrapping the rest of IngestIssue's writes.
	eventsInserted := 0
	if len(persistedEvents) > 0 {
		batch := &pgx.Batch{}
		for i, e := range persistedEvents {
			source := ictx.Source
			if leadingEvent != nil && i == 0 {
				source = "derived"
			}
			key := dedupeKey(ictx.Repo.Owner, ictx.Repo.Name, node.Number, ictx.Repo.GithubProjectID, e.CreatedAt, e.PreviousStatus, e.Status)
			occurredAt, err := time.Parse(time.RFC3339, e.CreatedAt)
			if err != nil {
				return Result{}, fmt.Errorf("ingest: parse persisted event createdAt %q: %w", e.CreatedAt, err)
			}
			batch.Queue(`
				INSERT INTO issue_status_events (issue_id, project_id, previous_status, status, occurred_at, source, dedupe_key)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (dedupe_key) DO NOTHING
			`, issueID, ictx.SlaProjectID, e.PreviousStatus, e.Status, occurredAt, source, key)
		}

		br := pool.SendBatch(ctx, batch)
		for range persistedEvents {
			tag, err := br.Exec()
			if err != nil {
				_ = br.Close()
				return Result{}, fmt.Errorf("ingest: insert status event: %w", err)
			}
			eventsInserted += int(tag.RowsAffected())
		}
		if err := br.Close(); err != nil {
			return Result{}, fmt.Errorf("ingest: insert status event: %w", err)
		}
	}

	// Current SLA projection.
	r := sla.ComputeSla(priority, slaEvents, currentStatus, ictx.Runtime.Cfg, ictx.Now)
	_, err = pool.Exec(ctx, `
		INSERT INTO issue_sla (
			issue_id, priority, budget_hours, consumed_hours, remaining_hours,
			pct_consumed, sla_state, sla_running, computed_at, computed_through
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (issue_id) DO UPDATE SET
			priority = $2, budget_hours = $3, consumed_hours = $4, remaining_hours = $5,
			pct_consumed = $6, sla_state = $7, sla_running = $8, computed_at = $9, computed_through = $10
	`, issueID, priority, r.BudgetHours, r.ConsumedHours, r.RemainingHours,
		r.PctConsumed, string(r.SlaState), r.SlaRunning, ictx.Now, ictx.Now)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: upsert issue_sla: %w", err)
	}

	return Result{
		Created:         existingID == nil,
		EventsInserted:  eventsInserted,
		IssueID:         issueID,
		Priority:        priority,
		SlaEvents:       slaEvents,
		SlaState:        r.SlaState,
		UnknownStatuses: unknownStatuses,
	}, nil
}
