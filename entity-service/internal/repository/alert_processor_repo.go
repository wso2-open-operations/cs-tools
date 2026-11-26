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

// The alert processor keeps all its state in alert: published_on queues an alert for the relay and incident_id marks it done. Every query runs as the system identity, because alert is internal only.

// UnpublishedAlerts is the relay's queue: the alert rows whose published_on is NULL.
type UnpublishedAlerts struct {
	db *Scoped
}

// NewUnpublishedAlerts returns the queue the alert relay drains.
func NewUnpublishedAlerts(db *Scoped) *UnpublishedAlerts {
	return &UnpublishedAlerts{db: db}
}

// unpublishedLock serializes drains across replicas, so batches publish in arrival order and a newer alert never overtakes an older one for its fingerprint.
const unpublishedLock = `SELECT pg_advisory_xact_lock(hashtextextended('alert_unpublished', 0))`

const drainUnpublishedQuery = `SELECT id, tier, received_on, alert
FROM alert
WHERE published_on IS NULL AND error_reason IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE`

const markPublishedQuery = `UPDATE alert SET published_on = NOW() WHERE id = ANY($1::text[])`

// Drain implements relay.Queue.
func (u *UnpublishedAlerts) Drain(ctx context.Context, limit int, publish func([]event.Event) []string) (int, error) {
	marked := 0
	err := u.db.InTx(WithSystemIdentity(ctx), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, unpublishedLock); err != nil {
			return fmt.Errorf("lock unpublished alerts: %w", err)
		}
		rows, err := tx.Query(ctx, drainUnpublishedQuery, limit)
		if err != nil {
			return fmt.Errorf("read unpublished alerts: %w", err)
		}
		batch, err := pgx.CollectRows(rows, scanAlertEvent)
		if err != nil || len(batch) == 0 {
			return err
		}
		published := publish(batch)
		if len(published) == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, markPublishedQuery, published); err != nil {
			return fmt.Errorf("mark alerts published: %w", err)
		}
		marked = len(published)
		return nil
	})
	return marked, err
}

// MaxAlertRequeues bounds how often an alert whose processing keeps failing is queued again, so one poison alert cannot cycle forever.
const MaxAlertRequeues = 10

// Requeue queues a published alert again, in its arrival place; false means it reached MaxAlertRequeues and stays published.
func (u *UnpublishedAlerts) Requeue(ctx context.Context, alertID string) (bool, error) {
	tag, err := u.db.Exec(WithSystemIdentity(ctx),
		`UPDATE alert SET published_on = NULL, requeues = requeues + 1 WHERE id = $1 AND requeues < $2`, alertID, MaxAlertRequeues)
	if err != nil {
		return false, fmt.Errorf("requeue alert %s: %w", alertID, err)
	}
	return tag.RowsAffected() == 1, nil
}

func scanAlertEvent(row pgx.CollectableRow) (event.Event, error) {
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
}

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

func (t groupTx) Unlinked(ctx context.Context, since, until time.Time, except string) ([]string, error) {
	rows, err := t.tx.Query(ctx, `SELECT id FROM alert
WHERE fingerprint = $1 AND incident_id IS NULL AND received_on BETWEEN $2 AND $3 AND id <> $4
ORDER BY received_on, id`, t.fingerprint, since, until, except)
	if err != nil {
		return nil, fmt.Errorf("read unlinked alerts: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
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

// IncidentNotes finds work notes on incidents; it implements sink.Notes.
type IncidentNotes struct {
	db *Scoped
}

// NewIncidentNotes returns the lookup the alert sink uses to add each alert's note once.
func NewIncidentNotes(db *Scoped) *IncidentNotes {
	return &IncidentNotes{db: db}
}

// HasNote reports whether the incident has a comment containing marker; marker is matched literally.
func (n *IncidentNotes) HasNote(ctx context.Context, incidentID, marker string) (bool, error) {
	var found bool
	err := n.db.QueryRow(WithSystemIdentity(ctx),
		`SELECT EXISTS (SELECT 1 FROM comment WHERE work_item_id = $1 AND strpos(content, $2) > 0)`, incidentID, marker).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("find note on incident %s: %w", incidentID, err)
	}
	return found, nil
}
