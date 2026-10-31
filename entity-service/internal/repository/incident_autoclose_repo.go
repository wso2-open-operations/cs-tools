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
)

// IncidentAutocloseRepository backs POST /incidents/autoclose, the port of
// ServiceNow's hourly close of incidents left RESOLVED for 14 days. It is
// separate from IncidentRepository because it closes without the resolve /
// close validation (ServiceNow closes regardless of resolution code and
// notes) and writes no comment or work note on the incident.
type IncidentAutocloseRepository interface {
	// ListIncidentsDueForAutoclose returns at most limit RESOLVED incidents
	// whose resolved_on (their last update when none is recorded) is at least
	// resolvedForDays days ago, oldest first.
	ListIncidentsDueForAutoclose(ctx context.Context, resolvedForDays, limit int) ([]IncidentAutocloseCandidate, error)
	// AutocloseIncident closes one incident in its own transaction: it locks
	// the row, re-checks that it is still RESOLVED and still due (closed false,
	// nil error when not), sets state CLOSED, closed_on now and closed_by_id =
	// resolved_by_id, stamps work_item.updated_on / updated_by with actorEmail
	// and closes the incident's open tasks as a manual close does
	// (cascadeIncidentTaskClosure).
	AutocloseIncident(ctx context.Context, id string, resolvedForDays int, actorEmail string) (closed bool, err error)
	// RecordIncidentFieldChanges writes activity-feed rows; see
	// IncidentRepository.RecordIncidentFieldChanges.
	RecordIncidentFieldChanges(ctx context.Context, incidentID string, changes []IncidentFieldChange, actorEmail string) error
}

// IncidentAutocloseCandidate is one incident due for auto-close.
type IncidentAutocloseCandidate struct {
	ID         string
	Number     string
	ResolvedOn *time.Time
}

// incidentAutocloseDue is the one "due" predicate the listing and the
// per-incident re-check share. $1 is resolvedForDays. An incident with no
// resolved_on is measured from its last update (synced rows can lack it).
const incidentAutocloseDue = `inc.state = 'RESOLVED'
	AND COALESCE(inc.resolved_on, wi.updated_on) <= NOW() - make_interval(days => $1::int)`

type incidentAutocloseRepo struct {
	*incidentRepo
}

// NewIncidentAutocloseRepository constructs an IncidentAutocloseRepository.
func NewIncidentAutocloseRepository(db *Scoped) IncidentAutocloseRepository {
	return &incidentAutocloseRepo{incidentRepo: &incidentRepo{db: db}}
}

// ListIncidentsDueForAutoclose implements IncidentAutocloseRepository.
func (r *incidentAutocloseRepo) ListIncidentsDueForAutoclose(ctx context.Context, resolvedForDays, limit int) ([]IncidentAutocloseCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT inc.id::text, wi.number, inc.resolved_on
		FROM incident inc
		JOIN work_item wi ON wi.id = inc.id
		WHERE `+incidentAutocloseDue+`
		ORDER BY COALESCE(inc.resolved_on, wi.updated_on), inc.id
		LIMIT $2`, resolvedForDays, limit)
	if err != nil {
		return nil, fmt.Errorf("list incidents due for auto-close: %w", err)
	}
	defer rows.Close()
	out := []IncidentAutocloseCandidate{}
	for rows.Next() {
		var c IncidentAutocloseCandidate
		if err := rows.Scan(&c.ID, &c.Number, &c.ResolvedOn); err != nil {
			return nil, fmt.Errorf("list incidents due for auto-close: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list incidents due for auto-close: %w", err)
	}
	return out, nil
}

// AutocloseIncident implements IncidentAutocloseRepository. The lock and the
// re-check are one statement: under READ COMMITTED a row updated while we
// waited for its lock is re-evaluated against the WHERE clause, so an
// incident someone re-opened or closed in the meantime is skipped.
func (r *incidentAutocloseRepo) AutocloseIncident(ctx context.Context, id string, resolvedForDays int, actorEmail string) (bool, error) {
	closed := false
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var number string
		err := tx.QueryRow(ctx, `
			SELECT wi.number
			FROM incident inc
			JOIN work_item wi ON wi.id = inc.id
			WHERE inc.id = $2 AND `+incidentAutocloseDue+`
			FOR UPDATE OF inc`, resolvedForDays, id).Scan(&number)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock incident: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE incident
			SET state = 'CLOSED', closed_on = NOW(), closed_by_id = resolved_by_id
			WHERE id = $1`, id); err != nil {
			return fmt.Errorf("close incident: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE work_item SET updated_on = NOW(), updated_by = $2
			WHERE id = $1`, id, actorEmail); err != nil {
			return fmt.Errorf("stamp incident work item: %w", err)
		}
		if err := cascadeIncidentTaskClosure(ctx, tx, id, number, "CLOSED", actorEmail); err != nil {
			return err
		}
		closed = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("auto-close incident %s: %w", id, err)
	}
	return closed, nil
}
