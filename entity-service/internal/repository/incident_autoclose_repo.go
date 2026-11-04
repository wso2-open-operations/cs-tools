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
)

// IncidentAutocloseRepository lists the incidents POST /incidents/autoclose
// closes: incidents left RESOLVED for resolvedForDays days. The close itself
// goes through the incident service (IncidentRepository.UpdateIncidentLifecycle
// with AutocloseDueDays set), as case auto-closure closes through the case
// service, so it is mirrored to the backing data source in dual-write.
type IncidentAutocloseRepository interface {
	// ListIncidentsDueForAutoclose returns at most limit RESOLVED incidents
	// whose resolved_on (their last update when none is recorded) is at least
	// resolvedForDays days ago, oldest first.
	ListIncidentsDueForAutoclose(ctx context.Context, resolvedForDays, limit int) ([]IncidentAutocloseCandidate, error)
	// CountIncidentsClosedSince returns how many incidents entered CLOSED at or
	// after since (incident.closed_on), however they were closed.
	CountIncidentsClosedSince(ctx context.Context, since time.Time) (int, error)
}

// ErrIncidentNotDueForAutoclose is what UpdateIncidentLifecycle returns, with
// nothing written, when an update with AutocloseDueDays finds the incident no
// longer due under its row lock: re-opened, closed or touched since it was
// listed.
var ErrIncidentNotDueForAutoclose = errors.New("incident is no longer due for auto-close")

// IncidentAutocloseCandidate is one incident due for auto-close.
type IncidentAutocloseCandidate struct {
	ID         string
	Number     string
	ResolvedOn *time.Time
}

// incidentAutocloseDue is the one "due" predicate the listing and the
// per-incident re-check (updateIncidentLifecycleTx) share. $1 is
// resolvedForDays. An incident with no resolved_on is measured from its last
// update (synced rows can lack it), as case auto-closure falls back to
// work_item.updated_on for its reference time.
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

// CountIncidentsClosedSince implements IncidentAutocloseRepository. It counts
// every close, manual ones included: closed_on is stamped on entering CLOSED
// whoever closes, and closed_by_id is NULL for the system user without a user
// row, so the auto-closes cannot be told apart reliably. Counting all of them
// is the conservative reading of a daily ceiling.
func (r *incidentAutocloseRepo) CountIncidentsClosedSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM incident WHERE closed_on >= $1`, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("count incidents closed since %s: %w", since.UTC().Format(time.RFC3339), err)
	}
	return n, nil
}
