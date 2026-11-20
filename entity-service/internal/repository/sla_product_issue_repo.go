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

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The product-issue half of the SLA engine's storage: the facts a policy's
// conditions are evaluated on, the issue's status timeline, and the one write
// that restates a clock from that timeline. It adds methods to slaEngineRepo
// and changes no existing one.

// ProductIssueSLAFacts is what the SLA engine reads about one product issue.
type ProductIssueSLAFacts struct {
	// Priority and Category are product_issue.priority/category as stored; nil
	// when NULL (a work item with no recognised label).
	Priority *string
	Category *string
	// GitHubState is github_issue_link.github_state (OPEN or CLOSED); "" when
	// the work item has no github_issue_link row.
	GitHubState string
	// BoardStatus is github_issue_link.board_status (nil when the issue is not on
	// a board).
	BoardStatus *string
	// BoardWaitsOn is waits_on of the newest board event whose to_status equals
	// BoardStatus; "" when there is none.
	BoardWaitsOn string
	// LatestBoardEventOn is the occurred_at of the newest board event (one with a
	// raw_status); nil when there is none.
	LatestBoardEventOn *time.Time
	// LatestReopenEventOn is the occurred_at of the newest GitHub reopen event
	// (to_status REOPENED, no raw_status); nil when there is none.
	LatestReopenEventOn *time.Time
	// GitHubClosedOn and GitHubUpdatedOn are github_issue_link.github_closed_on and
	// github_updated_on (nil when NULL or there is no link row).
	GitHubClosedOn  *time.Time
	GitHubUpdatedOn *time.Time
	// CreatedOn is github_issue_link.github_created_on, falling back to
	// work_item.created_on: the instant a retroactive clock starts at.
	CreatedOn time.Time
}

// ProductIssueStatusEvent is one work_item_status_event, as the replay needs it.
type ProductIssueStatusEvent struct {
	ToStatus   string
	WaitsOn    string
	OccurredAt time.Time
	CreatedOn  time.Time
	// IsBoardMove is true for an event with a raw_status (a project board move);
	// false for a GitHub close/reopen event (to_status CLOSED/REOPENED).
	IsBoardMove bool
}

// ProductIssueClockRow identifies the newest sla row of one policy on a work item.
type ProductIssueClockRow struct {
	ID    string
	Stage string
}

// ProductIssueClockState is every column a clock restated from history carries.
type ProductIssueClockState struct {
	StartOn  time.Time
	Stage    string // IN_PROGRESS, PAUSED or ACHIEVED
	EndOn    *time.Time
	PausedOn *time.Time

	PauseDuration         time.Duration
	BusinessPauseDuration time.Duration

	// Snapshot columns: the clock's figures as of end_on, paused_on or now.
	ElapsedPercentage float64
	BusinessDuration  time.Duration
	RemainingDuration time.Duration
	HasBreached       bool
}

// ProductIssueSLATx is the storage of one locked, transactional pass over a
// product issue's clocks (see ProductIssueSLARepository.WithProductIssueLock).
type ProductIssueSLATx interface {
	// Facts reads the issue's current facts.
	Facts(ctx context.Context) (ProductIssueSLAFacts, error)
	// Timeline lists the issue's status events ordered by occurred_at, then
	// created_on.
	Timeline(ctx context.Context) ([]ProductIssueStatusEvent, error)
	// LatestClockRow returns the current sla row of the issue and policyID: the open
	// one if there is one (created_on is a transaction start time and is not
	// monotonic under the lock, so a reopen row can sort before an older ACHIEVED
	// row), else the newest by created_on, then id. ok is false when there is none
	// (any stage).
	LatestClockRow(ctx context.Context, policyID string) (row ProductIssueClockRow, ok bool, err error)
	// RegisterClock inserts an IN_PROGRESS clock for policy starting at startOn
	// unless the policy already has an open clock (RegisterWorkItemClock's rule).
	RegisterClock(ctx context.Context, policy SLAPolicyRef, startOn time.Time) (bool, error)
	// RewriteClock writes every figure of one sla row in one UPDATE.
	RewriteClock(ctx context.Context, slaID string, st ProductIssueClockState) error
}

// ProductIssueSLARepository is the storage the product-issue SLA methods need.
// *slaEngineRepo implements it next to SLAEngineRepository; it is a separate
// interface so existing fakes of SLAEngineRepository need no change.
type ProductIssueSLARepository interface {
	// GetProductIssueSLAFacts reads the issue's current facts in one query.
	GetProductIssueSLAFacts(ctx context.Context, workItemID string) (ProductIssueSLAFacts, error)
	// WithProductIssueLock runs fn in ONE transaction (fn must use only tx, never
	// the pool: it holds a connection, so a pool read inside can starve the pool) holding a per-work-item
	// advisory lock (pg_advisory_xact_lock on 'sla-product-issue:'||id), so the
	// rebuilds of one issue never interleave. The lock key differs from
	// RegisterWorkItemClock's own (work item, policy) key, so registering inside
	// fn cannot deadlock with itself.
	WithProductIssueLock(ctx context.Context, workItemID string, fn func(ctx context.Context, tx ProductIssueSLATx) error) error
}

var _ ProductIssueSLARepository = (*slaEngineRepo)(nil)

// ErrProductIssueNotFound means the work item has no product_issue row.
var ErrProductIssueNotFound = errors.New("sla engine: product issue not found")

// piDB is the part of Scoped and pgx.Tx the product-issue queries use.
type piDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// piQueries runs the product-issue queries of one work item on db (the pool or a
// transaction).
type piQueries struct {
	db         piDB
	workItemID string
}

func (q piQueries) Facts(ctx context.Context) (ProductIssueSLAFacts, error) {
	var f ProductIssueSLAFacts
	var state *string
	err := q.db.QueryRow(ctx, `
		SELECT pi.priority, pi.category, l.github_state, l.board_status,
		       COALESCE((SELECT e.waits_on FROM work_item_status_event e
		                  WHERE e.work_item_id = pi.id AND e.raw_status IS NOT NULL AND e.to_status = l.board_status
		                  ORDER BY e.occurred_at DESC, e.created_on DESC LIMIT 1), ''),
		       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
		         WHERE e.work_item_id = pi.id AND e.raw_status IS NOT NULL),
		       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
		         WHERE e.work_item_id = pi.id AND e.raw_status IS NULL AND e.to_status = 'REOPENED'),
		       l.github_closed_on, l.github_updated_on,
		       COALESCE(l.github_created_on, wi.created_on)
		  FROM product_issue pi
		  JOIN work_item wi ON wi.id = pi.id
		  LEFT JOIN github_issue_link l ON l.work_item_id = pi.id
		 WHERE pi.id = $1::uuid`, q.workItemID).
		Scan(&f.Priority, &f.Category, &state, &f.BoardStatus, &f.BoardWaitsOn, &f.LatestBoardEventOn, &f.LatestReopenEventOn, &f.GitHubClosedOn, &f.GitHubUpdatedOn, &f.CreatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductIssueSLAFacts{}, ErrProductIssueNotFound
	}
	if err != nil {
		return ProductIssueSLAFacts{}, fmt.Errorf("get product issue sla facts: %w", err)
	}
	if state != nil {
		f.GitHubState = *state
	}
	return f, nil
}

func (q piQueries) Timeline(ctx context.Context) ([]ProductIssueStatusEvent, error) {
	rows, err := q.db.Query(ctx, `
		SELECT to_status, waits_on, occurred_at, created_on, raw_status IS NOT NULL
		  FROM work_item_status_event
		 WHERE work_item_id = $1::uuid
		 ORDER BY occurred_at, created_on, id`, q.workItemID)
	if err != nil {
		return nil, fmt.Errorf("list product issue status timeline: %w", err)
	}
	defer rows.Close()
	var out []ProductIssueStatusEvent
	for rows.Next() {
		var e ProductIssueStatusEvent
		if err := rows.Scan(&e.ToStatus, &e.WaitsOn, &e.OccurredAt, &e.CreatedOn, &e.IsBoardMove); err != nil {
			return nil, fmt.Errorf("list product issue status timeline: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list product issue status timeline: rows: %w", err)
	}
	return out, nil
}

func (q piQueries) LatestClockRow(ctx context.Context, policyID string) (ProductIssueClockRow, bool, error) {
	var row ProductIssueClockRow
	err := q.db.QueryRow(ctx, `
		SELECT id::TEXT, stage::TEXT FROM sla
		 WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid
		 ORDER BY (stage::TEXT NOT IN ('ACHIEVED', 'CANCELLED', 'COMPLETED')) DESC, created_on DESC, id DESC LIMIT 1`, q.workItemID, policyID).Scan(&row.ID, &row.Stage)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductIssueClockRow{}, false, nil
	}
	if err != nil {
		return ProductIssueClockRow{}, false, fmt.Errorf("latest sla clock row: %w", err)
	}
	return row, true, nil
}

func (q piQueries) RegisterClock(ctx context.Context, policy SLAPolicyRef, startOn time.Time) (bool, error) {
	if startOn.IsZero() {
		startOn = time.Now().UTC()
	}
	tag, err := q.db.Exec(ctx, slaEngineRegisterWorkItemClockQuery, q.workItemID, policy.ID, sqlActorLiteral, formatIntervalLiteral(policy.Duration), nullableSchedule(policy.Schedule), startOn)
	if err != nil {
		return false, fmt.Errorf("register work item sla clock: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (q piQueries) RewriteClock(ctx context.Context, slaID string, st ProductIssueClockState) error {
	tag, err := q.db.Exec(ctx, `
		UPDATE sla SET start_on = $2::timestamptz, stage = $3::sla_stage_enum, is_active = $4,
		    end_on = $5::timestamptz, paused_on = $6::timestamptz,
		    pause_duration = $7::interval, business_pause_duration = $8::interval,
		    business_elapsed_percentage = $9, business_duration = $10::interval,
		    remaining_business_duration = $11::interval, has_breached = $12,
		    updated_on = NOW(), updated_by = $13
		WHERE id = $1::uuid AND work_item_id = $14::uuid`,
		slaID, st.StartOn, st.Stage, st.Stage != "ACHIEVED", st.EndOn, st.PausedOn,
		formatIntervalLiteral(st.PauseDuration), formatIntervalLiteral(st.BusinessPauseDuration),
		st.ElapsedPercentage, formatIntervalLiteral(st.BusinessDuration), formatIntervalLiteral(st.RemainingDuration),
		st.HasBreached, sqlActorLiteral, q.workItemID)
	if err != nil {
		return fmt.Errorf("rewrite sla clock: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("rewrite sla clock: row %s not found", slaID)
	}
	return nil
}

// GetProductIssueSLAFacts implements ProductIssueSLARepository.
func (r *slaEngineRepo) GetProductIssueSLAFacts(ctx context.Context, workItemID string) (ProductIssueSLAFacts, error) {
	return piQueries{db: r.db, workItemID: workItemID}.Facts(WithSystemIdentity(ctx))
}

// WithProductIssueLock implements ProductIssueSLARepository.
func (r *slaEngineRepo) WithProductIssueLock(ctx context.Context, workItemID string, fn func(ctx context.Context, tx ProductIssueSLATx) error) error {
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('sla-product-issue:' || $1::text, 0))`, workItemID); err != nil {
			return fmt.Errorf("lock product issue sla: %w", err)
		}
		return fn(ctx, piQueries{db: tx, workItemID: workItemID})
	})
}
