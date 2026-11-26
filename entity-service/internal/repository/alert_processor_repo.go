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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Every query here runs as the system identity, because alert, alert_outbox and alert_group are internal only.

// AlertOutbox is alert_outbox, drained by the alert relay.
type AlertOutbox struct {
	db *Scoped
}

// NewAlertOutbox returns the outbox the alert relay drains.
func NewAlertOutbox(db *Scoped) *AlertOutbox {
	return &AlertOutbox{db: db}
}

// SKIP LOCKED lets several replicas drain at once without publishing the same row twice.
const drainAlertOutboxQuery = `SELECT o.alert_id, a.tier, a.received_on, a.alert
FROM alert_outbox o JOIN alert a ON a.id = o.alert_id
WHERE o.published_on IS NULL
ORDER BY o.created_on, o.alert_id
LIMIT $1
FOR UPDATE OF o SKIP LOCKED`

const markAlertOutboxQuery = `UPDATE alert_outbox SET published_on = NOW() WHERE alert_id = ANY($1::text[])`

// Drain implements alertprocessor.Outbox.
func (o *AlertOutbox) Drain(ctx context.Context, limit int, publish func([]alertprocessor.Event) []string) (int, error) {
	marked := 0
	err := o.db.InTx(WithSystemIdentity(ctx), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, drainAlertOutboxQuery, limit)
		if err != nil {
			return fmt.Errorf("read alert outbox: %w", err)
		}
		batch, err := pgx.CollectRows(rows, scanAlertEvent)
		if err != nil || len(batch) == 0 {
			return err
		}
		published := publish(batch)
		if len(published) == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, markAlertOutboxQuery, published); err != nil {
			return fmt.Errorf("mark alert outbox: %w", err)
		}
		marked = len(published)
		return nil
	})
	return marked, err
}

func scanAlertEvent(row pgx.CollectableRow) (alertprocessor.Event, error) {
	var e alertprocessor.Event
	var tier string
	var body []byte
	if err := row.Scan(&e.AlertID, &tier, &e.ReceivedOn, &body); err != nil {
		return alertprocessor.Event{}, err
	}
	e.Tier = domain.AlertTier(tier)
	if err := json.Unmarshal(body, &e.Alert); err != nil {
		return alertprocessor.Event{}, fmt.Errorf("decode alert %s: %w", e.AlertID, err)
	}
	return e, nil
}

// AlertGroups is alert_group, the alert processor's state.
type AlertGroups struct {
	db *Scoped
}

// NewAlertGroups returns the store the alert processor keeps its groups in.
func NewAlertGroups(db *Scoped) *AlertGroups {
	return &AlertGroups{db: db}
}

// InGroup implements alertprocessor.Store; the advisory lock is released when the transaction ends.
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

func (t groupTx) Group(ctx context.Context) (*alertprocessor.Group, error) {
	g := alertprocessor.Group{Fingerprint: t.fingerprint}
	var incidentID *string
	err := t.tx.QueryRow(ctx,
		`SELECT incident_id::text, first_seen_on, last_seen_on, alert_count FROM alert_group WHERE fingerprint = $1`, t.fingerprint,
	).Scan(&incidentID, &g.FirstSeenOn, &g.LastSeenOn, &g.AlertCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read alert group: %w", err)
	}
	if incidentID != nil {
		g.IncidentID = *incidentID
	}
	return &g, nil
}

func (t groupTx) SaveGroup(ctx context.Context, g alertprocessor.Group) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO alert_group (fingerprint, incident_id, first_seen_on, last_seen_on, alert_count, updated_on)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (fingerprint) DO UPDATE SET incident_id = EXCLUDED.incident_id, first_seen_on = EXCLUDED.first_seen_on,
	last_seen_on = EXCLUDED.last_seen_on, alert_count = EXCLUDED.alert_count, updated_on = EXCLUDED.updated_on`,
		t.fingerprint, g.IncidentID, g.FirstSeenOn, g.LastSeenOn, g.AlertCount, time.Now())
	if err != nil {
		return fmt.Errorf("save alert group: %w", err)
	}
	return nil
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
