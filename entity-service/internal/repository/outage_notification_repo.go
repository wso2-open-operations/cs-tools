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
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OutageNotificationRepository reads outages that want internal-stakeholder
// notification and records what has been sent for them.
type OutageNotificationRepository interface {
	// PendingOutages returns up to limit outages opted into internal
	// notification whose notification is not finished, oldest first. Returns
	// no rows — not an error — when the mirrored outage columns are absent.
	PendingOutages(ctx context.Context, limit int) ([]domain.OutageForNotification, error)
	// TryLockSweep makes sweeps take turns across replicas and callers; see
	// tryAdvisoryLock. ok is false when another sweep is running.
	TryLockSweep(ctx context.Context) (release func(), ok bool, err error)
	// RecordSent stores the outcome of one email. phaseChanged is false for
	// the update arm, which advances no phase.
	RecordSent(ctx context.Context, outageID string, kind domain.OutageNotificationKind,
		phase domain.OutageNotificationPhase, seeded bool) error
	// State returns what has been sent for one outage, or a NotFoundError.
	State(ctx context.Context, outageID string) (domain.OutageNotificationState, error)
}

type outageNotificationRepo struct {
	db db.Pool
}

// NewOutageNotificationRepository constructs the repository over the pool.
func NewOutageNotificationRepository(db db.Pool) OutageNotificationRepository {
	return &outageNotificationRepo{db: db}
}

// pendingOutagesSQL selects the outages a sweep must consider.
//
// The `notify_internal_stakeholders IS TRUE` filter is ServiceNow's trigger
// condition, and it is the whole scope rule: an outage nobody opted in for is
// not merely uninteresting, it was never in this flow. IS TRUE rather than
// `= true` because the column is nullable in the mirror and NULL must not
// pass.
//
// The LEFT JOIN is what makes the sweep resumable and the cutover safe: an
// outage with no notification row has never been evaluated here, and its
// `synced_phase` is ServiceNow's own record of what it already announced.
// Both are returned, and the service decides which to trust.
//
// Outages this service has already driven to RESOLVED are excluded outright —
// nothing further is ever sent for them, so carrying them in every sweep
// would grow the scan without end. An outage ServiceNow marked resolved but
// we have not is NOT excluded: it still needs a row of our own, and the
// seeding rule means it will be closed out without an email.
const pendingOutagesSQL = `
SELECT o.id::text,
       COALESCE(o.number, ''),
       COALESCE(o.name, ''),
       COALESCE(o.message, ''),
       COALESCE(o.type::text, ''),
       o.start_on,
       o.end_on,
       o.updated_on,
       COALESCE(o.internal_notification_phase::text, 'NONE'),
       COALESCE(s.name, ''),
       COALESCE(so.name, ''),
       n.outage_id::text,
       n.phase::text,
       n.declared_on,
       n.resolved_on,
       n.last_update_on,
       COALESCE(n.update_count, 0),
       COALESCE(n.seeded_from_sync, FALSE)
  FROM outage o
  LEFT JOIN outage_notifications n ON n.outage_id = o.id
  LEFT JOIN service          s  ON s.id  = o.service_id
  LEFT JOIN service_offering so ON so.id = o.service_offering_id
 WHERE o.notify_internal_stakeholders IS TRUE
   AND (n.phase IS NULL OR n.phase <> 'RESOLVED')
 ORDER BY COALESCE(n.updated_on, o.created_on), o.id
 LIMIT $1`

// TryLockSweep takes the internal-notification sweep lock.
func (r *outageNotificationRepo) TryLockSweep(ctx context.Context) (func(), bool, error) {
	return tryAdvisoryLock(ctx, r.db, outageNotificationSweepLockKey)
}

func (r *outageNotificationRepo) PendingOutages(ctx context.Context, limit int) ([]domain.OutageForNotification, error) {
	rows, err := r.db.Query(ctx, pendingOutagesSQL, limit)
	if err != nil {
		// The two columns this depends on arrive with digiops-cs migration
		// 0089. A database without them should report nothing to notify
		// rather than fail the sweep. Narrow on purpose: only
		// undefined_table/undefined_column degrade, because every other error
		// looks identical to "no outages" once swallowed.
		pgErr := (*pgconn.PgError)(nil)
		if !errors.As(err, &pgErr) || (pgErr.Code != "42P01" && pgErr.Code != "42703") {
			return nil, fmt.Errorf("querying outages pending internal notification: %w", err)
		}
		slog.WarnContext(ctx, "outage notification: mirrored outage columns absent (digiops-cs 0089 not applied); nothing to notify",
			"sqlstate", pgErr.Code)
		return nil, nil
	}
	defer rows.Close()

	var out []domain.OutageForNotification
	for rows.Next() {
		var o domain.OutageForNotification
		var stateID, statePhase *string
		var st domain.OutageNotificationState
		if err := rows.Scan(
			&o.OutageID, &o.Number, &o.Name, &o.Message, &o.Type,
			&o.StartOn, &o.EndOn, &o.UpdatedOn, &o.SyncedPhase,
			&o.ServiceName, &o.ServiceOfferingName,
			&stateID, &statePhase, &st.DeclaredOn, &st.ResolvedOn,
			&st.LastUpdateOn, &st.UpdateCount, &st.SeededFromSync,
		); err != nil {
			return nil, fmt.Errorf("scanning outage pending internal notification: %w", err)
		}
		if stateID != nil && statePhase != nil {
			st.OutageID = *stateID
			st.Phase = domain.OutageNotificationPhase(*statePhase)
			o.State = &st
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading outages pending internal notification: %w", err)
	}
	return out, nil
}

// recordSentSQL writes one send.
//
// NULLIF($2,”) is load-bearing. The update arm advances no phase and so
// passes an empty string, and PostgreSQL types and validates a parameter cast
// even inside a CASE branch it will not take -- so a bare
// $2::outage_notification_phase_enum fails with 22P02 on every update email,
// while the declaration and resolution arms pass. Caught only by running a
// real sweep against a real database; the enum has no empty member, so the
// empty value has to become NULL before it is ever cast.
//
// The update arm passes phaseChanged=false and the COALESCE keeps whatever
// phase is already stored, so many update emails never disturb the state
// machine — the original writes no phase there either.
//
// seeded_from_sync is only ever set on INSERT, and only when the phase being
// written did not come from an email we sent. Once a row exists it is left
// alone: a row seeded at cutover that later sends a real email stays marked,
// which is the honest reading of "this outage's history did not start here".
const recordSentSQL = `
INSERT INTO outage_notifications (
    outage_id, phase, declared_on, resolved_on, last_update_on, update_count,
    seeded_from_sync, created_on, updated_on
) VALUES (
    $1,
    $2::outage_notification_phase_enum,
    CASE WHEN $4 = 'DECLARED' THEN NOW() END,
    CASE WHEN $4 = 'RESOLVED' THEN NOW() END,
    CASE WHEN $4 = 'UPDATE'   THEN NOW() END,
    CASE WHEN $4 = 'UPDATE'   THEN 1 ELSE 0 END,
    $3, NOW(), NOW()
)
ON CONFLICT (outage_id) DO UPDATE SET
    phase          = $2::outage_notification_phase_enum,
    declared_on    = COALESCE(outage_notifications.declared_on,
                              CASE WHEN $4 = 'DECLARED' THEN NOW() END),
    resolved_on    = COALESCE(outage_notifications.resolved_on,
                              CASE WHEN $4 = 'RESOLVED' THEN NOW() END),
    last_update_on = CASE WHEN $4 = 'UPDATE' THEN NOW()
                          ELSE outage_notifications.last_update_on END,
    update_count   = outage_notifications.update_count
                     + CASE WHEN $4 = 'UPDATE' THEN 1 ELSE 0 END,
    updated_on     = NOW()`

func (r *outageNotificationRepo) RecordSent(ctx context.Context, outageID string,
	kind domain.OutageNotificationKind, phase domain.OutageNotificationPhase, seeded bool) error {

	if _, err := r.db.Exec(ctx, recordSentSQL, outageID, string(phase), seeded, string(kind)); err != nil {
		return fmt.Errorf("recording outage notification for %s: %w", outageID, err)
	}
	return nil
}

const outageNotificationStateSQL = `
SELECT outage_id::text, phase::text, declared_on, resolved_on,
       last_update_on, update_count, seeded_from_sync
  FROM outage_notifications
 WHERE outage_id = $1`

func (r *outageNotificationRepo) State(ctx context.Context, outageID string) (domain.OutageNotificationState, error) {
	var s domain.OutageNotificationState
	var phase string
	err := r.db.QueryRow(ctx, outageNotificationStateSQL, outageID).Scan(
		&s.OutageID, &phase, &s.DeclaredOn, &s.ResolvedOn,
		&s.LastUpdateOn, &s.UpdateCount, &s.SeededFromSync,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OutageNotificationState{}, &apierror.NotFoundError{
			Msg: fmt.Sprintf("no notification state for outage %s", outageID)}
	}
	if err != nil {
		return domain.OutageNotificationState{}, fmt.Errorf("reading outage notification state for %s: %w", outageID, err)
	}
	s.Phase = domain.OutageNotificationPhase(phase)
	return s, nil
}
