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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The alert processor keeps its state in alert: incident_id marks an alert done, and a fingerprint's group is read from its alerts. Every query runs as the system identity, because alert is internal only.

// AlertGroups reads each fingerprint's group from its alerts; it implements alertprocessor.Store.
type AlertGroups struct {
	db *Scoped
}

// NewAlertGroups returns the store the alert processor reads groups and links alerts through.
func NewAlertGroups(db *Scoped) *AlertGroups {
	return &AlertGroups{db: db}
}

// InGroup implements alertprocessor.Store; the advisory lock on the fingerprint is released when the transaction ends.
func (g *AlertGroups) InGroup(ctx context.Context, fingerprint string, fn func(alertprocessor.GroupTx) error) error {
	return g.db.InTx(WithSystemIdentity(ctx), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, fingerprint); err != nil {
			return fmt.Errorf("lock alert group: %w", err)
		}
		return fn(groupTx{tx: tx, fingerprint: fingerprint})
	})
}

type groupTx struct {
	tx          pgx.Tx
	fingerprint string
}

// groupQuery takes the incident of the fingerprint's latest linked alert, then that incident's alerts of the fingerprint.
const groupQuery = `WITH current AS (
	SELECT incident_id FROM alert
	WHERE fingerprint = $1 AND incident_id IS NOT NULL
	ORDER BY received_on DESC, id DESC
	LIMIT 1
)
SELECT a.incident_id::text, min(a.received_on), max(a.received_on), count(*)
FROM alert a JOIN current c ON a.incident_id = c.incident_id
WHERE a.fingerprint = $1
GROUP BY a.incident_id`

func (t groupTx) Group(ctx context.Context) (*strategy.Group, error) {
	g := strategy.Group{Fingerprint: t.fingerprint}
	err := t.tx.QueryRow(ctx, groupQuery, t.fingerprint).Scan(&g.IncidentID, &g.FirstSeenOn, &g.LastSeenOn, &g.AlertCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read alert group: %w", err)
	}
	return &g, nil
}

func (t groupTx) Unlinked(ctx context.Context, since, until time.Time, except string) ([]event.Event, error) {
	rows, err := t.tx.Query(ctx, `SELECT id, tier, received_on, alert FROM alert
WHERE fingerprint = $1 AND incident_id IS NULL AND received_on BETWEEN $2 AND $3 AND id <> $4
ORDER BY received_on, id`, t.fingerprint, since, until, except)
	if err != nil {
		return nil, fmt.Errorf("read unlinked alerts: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (event.Event, error) {
		var e event.Event
		var tier string
		var body []byte
		if err := row.Scan(&e.AlertID, &tier, &e.ReceivedOn, &body); err != nil {
			return event.Event{}, err
		}
		e.Tier = domain.AlertTier(tier)
		if err := json.Unmarshal(body, &e.Alert); err != nil {
			return event.Event{}, fmt.Errorf("decode alert %s: %w", e.AlertID, err)
		}
		return e, nil
	})
}

func (t groupTx) AlertLinked(ctx context.Context, alertID string) (bool, error) {
	var linked bool
	if err := t.tx.QueryRow(ctx, `SELECT incident_id IS NOT NULL FROM alert WHERE id = $1`, alertID).Scan(&linked); err != nil {
		return false, fmt.Errorf("read alert %s: %w", alertID, err)
	}
	return linked, nil
}

func (t groupTx) LinkAlert(ctx context.Context, alertID, incidentID string) error {
	if _, err := t.tx.Exec(ctx, `UPDATE alert SET incident_id = $2 WHERE id = $1`, alertID, incidentID); err != nil {
		return fmt.Errorf("link alert %s: %w", alertID, err)
	}
	return nil
}

// IncidentNotes finds work notes on incidents; it implements incident.Notes.
type IncidentNotes struct {
	db *Scoped
}

// NewIncidentNotes returns the lookup the incident writer uses to add each alert's note once.
func NewIncidentNotes(db *Scoped) *IncidentNotes {
	return &IncidentNotes{db: db}
}

// HasWorkNote reports whether the incident has a work note starting with any of prefixes, compared literally.
func (n *IncidentNotes) HasWorkNote(ctx context.Context, incidentID string, prefixes []string) (bool, error) {
	var found bool
	err := n.db.QueryRow(WithSystemIdentity(ctx), `SELECT EXISTS (
	SELECT 1 FROM comment c, unnest($2::text[]) AS p(prefix)
	WHERE c.work_item_id = $1 AND c.type = 'WORK_NOTE'::comment_type_enum AND left(c.content, length(p.prefix)) = p.prefix
)`, incidentID, prefixes).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("find note on incident %s: %w", incidentID, err)
	}
	return found, nil
}
