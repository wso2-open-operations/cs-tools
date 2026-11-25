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
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ErrAlertClaimLost means another processor took the alert over after this one's claim expired; nothing was written.
var ErrAlertClaimLost = errors.New("alert claim lost")

// AlertProcessorRepository is the alert processor's storage: it claims NEW alerts and records what was decided.
type AlertProcessorRepository interface {
	// ClaimAlerts claims up to limit waiting alerts for owner, for lease, priority tier first and oldest first.
	ClaimAlerts(ctx context.Context, owner string, lease time.Duration, limit int) ([]domain.ClaimedAlert, error)
	// WithFingerprintLock runs fn in one transaction holding a lock on the fingerprint, so one fingerprint is decided by one worker at a time.
	WithFingerprintLock(ctx context.Context, fingerprint string, fn func(AlertFingerprintTx) error) error
	// RecordFailure counts a failed attempt; failed makes it final (FAILED), else the alert is retried at nextAttempt.
	RecordFailure(ctx context.Context, alertID, owner, lastError string, nextAttempt time.Time, failed bool) error
	// ServiceIDByName returns the newest service named name, ignoring case, as alert-core's service lookup picks it.
	ServiceIDByName(ctx context.Context, name string) (string, bool, error)
}

// AlertFingerprintTx is what a worker may do while it holds a fingerprint's lock.
type AlertFingerprintTx interface {
	// OpenIncidentByTagPrefix returns the newest open incident whose correlation id starts with prefix and, when maxAge
	// is positive, that was created within maxAge.
	OpenIncidentByTagPrefix(ctx context.Context, prefix string, maxAge time.Duration) (string, bool, error)
	// IncidentByCorrelationID returns an incident with exactly this correlation id, in any state.
	IncidentByCorrelationID(ctx context.Context, correlationID string) (string, bool, error)
	// RecordOutcome makes the alert's decision final; ErrAlertClaimLost when owner no longer holds the claim.
	RecordOutcome(ctx context.Context, alertID, owner string, outcome domain.AlertProcessorOutcome) error
}

// claimAlertsQuery takes waiting alerts whose claim is free or expired and whose retry is due; an error row (a message
// that could not be normalised) is never taken. SKIP LOCKED lets several workers and replicas claim at once without
// waiting on each other. The first two conditions are alert_pending_idx's predicate (migration 0309).
const claimAlertsQuery = `WITH picked AS (
    SELECT id FROM alert
    WHERE processed_on IS NULL AND error_reason IS NULL
      AND (claim_expires_on IS NULL OR claim_expires_on < now())
      AND (next_attempt_on IS NULL OR next_attempt_on <= now())
    ORDER BY tier DESC, id
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
UPDATE alert a SET claimed_by = $1, claim_expires_on = now() + make_interval(secs => $2)
FROM picked WHERE a.id = picked.id
RETURNING a.id, a.source, a.alert, a.fingerprint, a.tier, a.received_on, a.attempts`

// openIncidentByTagPrefixQuery repeats idx_incident_open_alert_correlation's predicate (migration 0310) word for word,
// so the planner reads the small partial index instead of the incident table.
const openIncidentByTagPrefixQuery = `SELECT i.id::text FROM incident i
JOIN work_item w ON w.id = i.id
WHERE i.correlation_id LIKE $1
  AND i.correlation_id LIKE '[fp:%' AND i.state IN ('NEW', 'IN_PROGRESS', 'ON_HOLD')
  AND ($2::double precision <= 0 OR w.created_on > now() - make_interval(secs => $2))
ORDER BY w.created_on DESC, i.id
LIMIT 1`

const incidentByCorrelationIDQuery = `SELECT id::text FROM incident WHERE correlation_id = $1 ORDER BY id LIMIT 1`

const recordAlertOutcomeQuery = `UPDATE alert SET
    status = $3,
    incident_id = $4::uuid,
    mute_rule_id = $5::uuid,
    shadow_decision = $6,
    shadow_incident_id = $7::uuid,
    processed_on = now(),
    claimed_by = NULL,
    claim_expires_on = NULL,
    next_attempt_on = NULL,
    last_error = NULL
WHERE id = $1 AND claimed_by = $2 AND processed_on IS NULL`

const recordAlertFailureQuery = `UPDATE alert SET
    attempts = attempts + 1,
    last_error = $3,
    next_attempt_on = CASE WHEN $5::boolean THEN NULL ELSE $4::timestamptz END,
    status = CASE WHEN $5::boolean THEN 'FAILED' ELSE status END,
    processed_on = CASE WHEN $5::boolean THEN now() ELSE NULL END,
    claimed_by = NULL,
    claim_expires_on = NULL
WHERE id = $1 AND claimed_by = $2 AND processed_on IS NULL`

const serviceIDByNameQuery = `SELECT id::text FROM service WHERE LOWER(name) = LOWER($1) ORDER BY created_on DESC, id LIMIT 1`

// maxAlertErrorLength keeps a stored error short; the full one is in the log.
const maxAlertErrorLength = 1000

type alertProcessorRepository struct {
	db *Scoped
}

// NewAlertProcessorRepository returns an AlertProcessorRepository; callers run it under the system identity because alert is internal only.
func NewAlertProcessorRepository(db *Scoped) AlertProcessorRepository {
	return &alertProcessorRepository{db: db}
}

func (r *alertProcessorRepository) ClaimAlerts(ctx context.Context, owner string, lease time.Duration, limit int) ([]domain.ClaimedAlert, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, claimAlertsQuery, owner, lease.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("claim alerts: %w", err)
	}
	claimed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ClaimedAlert, error) {
		var a domain.ClaimedAlert
		var body []byte
		var fingerprint *string
		var tier string
		if err := row.Scan(&a.ID, &a.Source, &body, &fingerprint, &tier, &a.ReceivedOn, &a.Attempts); err != nil {
			return a, err
		}
		a.Alert = body
		if fingerprint != nil {
			a.Fingerprint = *fingerprint
		}
		a.Tier = domain.AlertTier(tier)
		return a, nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim alerts: %w", err)
	}
	// RETURNING has no order; work the priority tier first, then oldest first, as the claim chose them.
	slices.SortFunc(claimed, func(a, b domain.ClaimedAlert) int {
		if a.Tier != b.Tier {
			return strings.Compare(string(b.Tier), string(a.Tier))
		}
		return strings.Compare(a.ID, b.ID)
	})
	return claimed, nil
}

func (r *alertProcessorRepository) WithFingerprintLock(ctx context.Context, fingerprint string, fn func(AlertFingerprintTx) error) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('alert-fingerprint:' || $1, 0))`, fingerprint); err != nil {
			return fmt.Errorf("lock alert fingerprint: %w", err)
		}
		return fn(alertFingerprintTx{tx: tx})
	})
}

func (r *alertProcessorRepository) RecordFailure(ctx context.Context, alertID, owner, lastError string, nextAttempt time.Time, failed bool) error {
	tag, err := r.db.Exec(ctx, recordAlertFailureQuery, alertID, owner, truncateRunes(lastError, maxAlertErrorLength), nextAttempt, failed)
	if err != nil {
		return fmt.Errorf("record alert failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAlertClaimLost
	}
	return nil
}

func (r *alertProcessorRepository) ServiceIDByName(ctx context.Context, name string) (string, bool, error) {
	var id string
	err := r.db.QueryRow(ctx, serviceIDByNameQuery, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find service by name: %w", err)
	}
	return id, true, nil
}

type alertFingerprintTx struct {
	tx pgx.Tx
}

func (t alertFingerprintTx) OpenIncidentByTagPrefix(ctx context.Context, prefix string, maxAge time.Duration) (string, bool, error) {
	pattern := escapeLike(prefix) + "%"
	var id string
	err := t.tx.QueryRow(ctx, openIncidentByTagPrefixQuery, pattern, maxAge.Seconds()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find open incident by correlation tag: %w", err)
	}
	return id, true, nil
}

func (t alertFingerprintTx) IncidentByCorrelationID(ctx context.Context, correlationID string) (string, bool, error) {
	var id string
	err := t.tx.QueryRow(ctx, incidentByCorrelationIDQuery, correlationID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find incident by correlation id: %w", err)
	}
	return id, true, nil
}

func (t alertFingerprintTx) RecordOutcome(ctx context.Context, alertID, owner string, o domain.AlertProcessorOutcome) error {
	tag, err := t.tx.Exec(ctx, recordAlertOutcomeQuery, alertID, owner, o.Status,
		nullIfEmpty(o.IncidentID), nullIfEmpty(o.MuteRuleID), nullIfEmpty(o.ShadowDecision), nullIfEmpty(o.ShadowIncidentID))
	if err != nil {
		return fmt.Errorf("record alert outcome: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAlertClaimLost
	}
	return nil
}

// escapeLike escapes LIKE's wildcards and its escape character, so prefix matches literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
