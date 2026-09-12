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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SLAClockRepository defines the persistence operations for the sla_clocks
// table — see domain.SLAClock's doc comment for what it's for.
type SLAClockRepository interface {
	// Register (re)creates the clock for (caseId, clockType): inserts a new
	// row, or if one already exists, overwrites started_at/due_at and clears
	// paused_at/reached_50_at/reached_75_at/reached_100_at — a full reset,
	// not an in-place adjustment.
	Register(ctx context.Context, req domain.RegisterSLAClockRequest) (domain.SLAClock, error)
	// Get returns the clock for (caseId, clockType). Returns a
	// *apierror.NotFoundError if no such clock has been registered.
	Get(ctx context.Context, caseID, clockType string) (domain.SLAClock, error)
	// SetTierReachedIfUnset writes reached_<tier>_at only if it is still
	// null, and returns the timestamp that ends up stored either way —
	// idempotent, so retrying a call whose response was lost (e.g. after a
	// network blip) doesn't clobber the original reached time. alreadySet
	// reports whether this specific call is the one that just wrote the
	// value (false) or whether it was already set by an earlier call
	// (true) — the underlying UPDATE's own WHERE ... IS NULL clause is
	// what actually decides this atomically, so it's correct even when two
	// callers race to set the same tier at the same time; only one of them
	// can ever see alreadySet=false for a given tier.
	//
	// alreadySet reflects the database claim only — it says nothing about
	// whether any caller's downstream reaction (e.g. publishing a
	// notification) to winning that claim ever actually succeeded. Using
	// alreadySet=true to skip that reaction is a real, valid choice for a
	// caller that wants duplicate-free rediscovery — csm-notification-service's
	// internal/slaengine.Engine does exactly this (see that repo's CLAUDE.md
	// for its full reasoning) — but it's a trade-off, not a free win:
	// gating this way means a caller whose own reaction failed (or crashed)
	// after it won the claim will, on retry, see alreadySet=true and skip
	// the reaction forever, having never actually completed it once. Only
	// gate a reaction on this if that residual risk is acceptable for your
	// use case, or if you separately, durably track the reaction's own
	// completion instead of relying on this field for it. Returns a
	// *apierror.NotFoundError if no such clock has been registered.
	SetTierReachedIfUnset(ctx context.Context, caseID, clockType, tier string) (reachedAt time.Time, alreadySet bool, err error)
	// SetPaused sets or clears paused_at for (caseId, clockType) — idempotent
	// either direction (pausing an already-paused clock, or resuming an
	// already-running one, is a no-op that still returns the current row).
	// Called directly, in-process, from snCaseService's case-state handling
	// — see that file's applyCaseStateSLAEffects — not from any HTTP path;
	// csm-notification-service's slaengine only ever reads paused_at (via
	// Get, before firing a tier), never writes it. Returns a
	// *apierror.NotFoundError if no such clock has been registered.
	//
	// KNOWN GAP: due_at is never adjusted here. A clock paused for any
	// stretch of time resumes with the same due_at it had before, so the
	// time spent paused is not added back — combined with
	// csm-notification-service's slaengine dropping (not rescheduling) a
	// wake entry that comes due while paused (see that package's own
	// processDueMember), a clock paused past one of its tier times can
	// permanently lose that tier's alert even after resuming. Fixing this
	// properly needs entity-service to extend due_at by the paused duration
	// AND a way to tell csm-notification-service to requeue wake entries on
	// resume (no such signal exists today — pause/resume are deliberately
	// in-process-only, see snCaseService.applyCaseStateSLAEffects) — a real
	// design addition, not a quick fix, so it's flagged here rather than
	// built speculatively.
	SetPaused(ctx context.Context, caseID, clockType string, paused bool) (domain.SLAClock, error)
}

type slaClockRepo struct {
	db *pgxpool.Pool
}

// NewSLAClockRepository constructs an SLAClockRepository backed by the given
// connection pool.
func NewSLAClockRepository(db *pgxpool.Pool) SLAClockRepository {
	return &slaClockRepo{db: db}
}

// slaClockColumns is the column list shared by every query that returns a
// full row, kept in one place so Register/Get can't drift out of sync with
// scanSLAClock's field order. The last eight are display-only — see
// domain.SLAClock's own doc comment.
const slaClockColumns = `case_id, clock_type, started_at, due_at, paused_at, reached_50_at, reached_75_at, reached_100_at, case_number, wso2_case_id, case_title, case_type, product, team, priority, state`

func scanSLAClock(row pgx.Row) (domain.SLAClock, error) {
	var c domain.SLAClock
	var caseNumber, wso2CaseID, caseTitle, caseType, product, team, priority, state *string
	if err := row.Scan(
		&c.CaseID, &c.ClockType, &c.StartedOn, &c.DueOn,
		&c.PausedOn, &c.Reached50On, &c.Reached75On, &c.Reached100On,
		&caseNumber, &wso2CaseID, &caseTitle, &caseType, &product, &team, &priority, &state,
	); err != nil {
		return domain.SLAClock{}, err
	}
	c.CaseNumber = stringOrEmpty(caseNumber)
	c.WSO2CaseID = stringOrEmpty(wso2CaseID)
	c.CaseTitle = stringOrEmpty(caseTitle)
	c.CaseType = stringOrEmpty(caseType)
	c.Product = stringOrEmpty(product)
	c.Team = stringOrEmpty(team)
	c.Priority = stringOrEmpty(priority)
	c.State = stringOrEmpty(state)
	return c, nil
}

// stringOrEmpty returns "" for a nil column value rather than propagating a
// nil *string into domain.SLAClock's plain string fields.
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Register implements SLAClockRepository.
func (r *slaClockRepo) Register(ctx context.Context, req domain.RegisterSLAClockRequest) (domain.SLAClock, error) {
	query := `
		INSERT INTO sla_clocks (case_id, clock_type, started_at, due_at, case_number, wso2_case_id, case_title, case_type, product, team, priority, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (case_id, clock_type) DO UPDATE SET
			started_at = EXCLUDED.started_at,
			due_at = EXCLUDED.due_at,
			paused_at = NULL,
			reached_50_at = NULL,
			reached_75_at = NULL,
			reached_100_at = NULL,
			case_number = EXCLUDED.case_number,
			wso2_case_id = EXCLUDED.wso2_case_id,
			case_title = EXCLUDED.case_title,
			case_type = EXCLUDED.case_type,
			product = EXCLUDED.product,
			team = EXCLUDED.team,
			priority = EXCLUDED.priority,
			state = EXCLUDED.state,
			updated_at = NOW()
		RETURNING ` + slaClockColumns

	c, err := scanSLAClock(r.db.QueryRow(ctx, query, req.CaseID, req.ClockType, req.StartedAt, req.DueAt,
		nullIfEmpty(req.CaseNumber), nullIfEmpty(req.WSO2CaseID), nullIfEmpty(req.CaseTitle), nullIfEmpty(req.CaseType),
		nullIfEmpty(req.Product), nullIfEmpty(req.Team), nullIfEmpty(req.Priority), nullIfEmpty(req.State)))
	if err != nil {
		return domain.SLAClock{}, fmt.Errorf("register sla_clock: %w", err)
	}
	return c, nil
}

// nullIfEmpty returns nil for an empty string so it's stored as SQL NULL
// rather than an empty-string value — matches stringOrEmpty's read-back
// convention above.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Get implements SLAClockRepository.
func (r *slaClockRepo) Get(ctx context.Context, caseID, clockType string) (domain.SLAClock, error) {
	query := `SELECT ` + slaClockColumns + ` FROM sla_clocks WHERE case_id = $1 AND clock_type = $2`

	c, err := scanSLAClock(r.db.QueryRow(ctx, query, caseID, clockType))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SLAClock{}, &apierror.NotFoundError{Msg: "sla_clock not found for case " + caseID + " and clock type " + clockType}
		}
		return domain.SLAClock{}, fmt.Errorf("get sla_clock: %w", err)
	}
	return c, nil
}

// tierColumn maps a caller-supplied tier ("50"/"75"/"100") to its column
// name. tier has already been validated by the service layer before this is
// called, but the switch's default is still reachable defensively.
func tierColumn(tier string) (string, error) {
	switch tier {
	case "50":
		return "reached_50_at", nil
	case "75":
		return "reached_75_at", nil
	case "100":
		return "reached_100_at", nil
	default:
		return "", fmt.Errorf("invalid sla_clock tier: %q", tier)
	}
}

// SetTierReachedIfUnset implements SLAClockRepository.
func (r *slaClockRepo) SetTierReachedIfUnset(ctx context.Context, caseID, clockType, tier string) (time.Time, bool, error) {
	col, err := tierColumn(tier)
	if err != nil {
		return time.Time{}, false, err
	}

	var reached time.Time
	updateQuery := `UPDATE sla_clocks SET ` + col + ` = NOW(), updated_at = NOW() WHERE case_id = $1 AND clock_type = $2 AND ` + col + ` IS NULL RETURNING ` + col
	err = r.db.QueryRow(ctx, updateQuery, caseID, clockType).Scan(&reached)
	if err == nil {
		// This call's own UPDATE matched a row — it just wrote the value.
		return reached, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, fmt.Errorf("set sla_clock tier reached: %w", err)
	}

	// The UPDATE's WHERE ... IS NULL matched zero rows: either the tier was
	// already reached by an earlier call (read back that existing value
	// below), or the clock doesn't exist at all.
	selectQuery := `SELECT ` + col + ` FROM sla_clocks WHERE case_id = $1 AND clock_type = $2`
	err = r.db.QueryRow(ctx, selectQuery, caseID, clockType).Scan(&reached)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, &apierror.NotFoundError{Msg: "sla_clock not found for case " + caseID + " and clock type " + clockType}
		}
		return time.Time{}, false, fmt.Errorf("read sla_clock tier reached: %w", err)
	}
	if reached.IsZero() {
		// The UPDATE's WHERE ... IS NULL matched zero rows because the
		// column was already non-null by the time it ran, but the read-back
		// above got a NULL some other way — not expected in practice
		// (another SetTierReachedIfUnset call would have already returned a
		// non-zero value), surfaced rather than silently returned as a
		// plausible-looking zero time.
		return time.Time{}, false, fmt.Errorf("sla_clock tier %s for case %s has no reached timestamp after an unset UPDATE matched no rows", tier, caseID)
	}
	return reached, true, nil
}

// SetPaused implements SLAClockRepository.
func (r *slaClockRepo) SetPaused(ctx context.Context, caseID, clockType string, paused bool) (domain.SLAClock, error) {
	query := `
		UPDATE sla_clocks SET
			paused_at = CASE WHEN $3 THEN COALESCE(paused_at, NOW()) ELSE NULL END,
			updated_at = NOW()
		WHERE case_id = $1 AND clock_type = $2
		RETURNING ` + slaClockColumns

	c, err := scanSLAClock(r.db.QueryRow(ctx, query, caseID, clockType, paused))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SLAClock{}, &apierror.NotFoundError{Msg: "sla_clock not found for case " + caseID + " and clock type " + clockType}
		}
		return domain.SLAClock{}, fmt.Errorf("set sla_clock paused: %w", err)
	}
	return c, nil
}
