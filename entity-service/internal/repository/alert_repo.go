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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertRepository is the alert consumer's storage. Callers run it under the system identity: alert is internal only.
type AlertRepository interface {
	// ReserveAlertIDs returns n new alert ids (ALT followed by nine digits), in ascending order.
	ReserveAlertIDs(ctx context.Context, n int) ([]string, error)
	// InsertAlerts stores alert and error rows in one statement and returns how many were new. A row whose message was
	// already stored is skipped, so a redelivered message stores nothing. When a NEW alert was stored it also notifies
	// AlertStoredChannel, which Postgres delivers only if the statement's transaction commits.
	InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error)
}

// AlertStoredChannel is the Postgres notification channel the insert signals when it stored at least one NEW alert; the
// alert relay LISTENs on it to publish at once instead of waiting for its next poll.
const AlertStoredChannel = "alert_stored"

// alertIDFormat turns a sequence value into an alert id.
const alertIDFormat = "ALT%09d"

const reserveAlertIDsQuery = `SELECT nextval('alert_seq') FROM generate_series(1, $1)`

// insertAlertsQuery stores the rows and, in the same statement, notifies $13 once when any NEW alert was stored. An
// error row is stored as ERROR, so NEW counts only alerts the processor will take.
const insertAlertsQuery = `WITH inserted AS (
    INSERT INTO alert (id, source, alert, fingerprint, tier, message_id, message_index, raw_body, content_type,
                       error_reason, error, received_on, status)
    SELECT a.*, CASE WHEN a.error_reason IS NULL THEN 'NEW' ELSE 'ERROR' END
    FROM unnest($1::text[], $2::text[], $3::jsonb[], $4::text[], $5::text[], $6::text[], $7::int[], $8::bytea[],
                $9::text[], $10::text[], $11::text[], $12::timestamptz[])
        AS a(id, source, alert, fingerprint, tier, message_id, message_index, raw_body, content_type, error_reason,
             error, received_on)
    ON CONFLICT (message_id, message_index) DO NOTHING
    RETURNING status
), notified AS (
    SELECT pg_notify($13::text, '') FROM inserted WHERE status = 'NEW' LIMIT 1
)
SELECT (SELECT count(*) FROM inserted), (SELECT count(*) FROM notified)`

type alertRepository struct {
	db *Scoped
}

// NewAlertRepository returns an AlertRepository.
func NewAlertRepository(db *Scoped) AlertRepository {
	return &alertRepository{db: db}
}

func (r *alertRepository) ReserveAlertIDs(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, reserveAlertIDsQuery, n)
	if err != nil {
		return nil, fmt.Errorf("reserve alert ids: %w", err)
	}
	seqs, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("reserve alert ids: %w", err)
	}
	ids := make([]string, len(seqs))
	for i, v := range seqs {
		ids[i] = fmt.Sprintf(alertIDFormat, v)
	}
	return ids, nil
}

func (r *alertRepository) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error) {
	if len(alerts) == 0 {
		return 0, nil
	}
	n := len(alerts)
	var (
		ids, sources, fingerprints, tiers, messageIDs = make([]string, n), make([]*string, n), make([]*string, n), make([]string, n), make([]string, n)
		bodies                                        = make([]*string, n)
		indexes                                       = make([]int32, n)
		rawBodies                                     = make([][]byte, n)
		contentTypes, errorReasons, errorTexts        = make([]*string, n), make([]*string, n), make([]*string, n)
		receivedOn                                    = make([]time.Time, n)
	)
	for i, a := range alerts {
		ids[i] = a.ID
		sources[i] = nullIfEmpty(a.Source)
		if len(a.Alert) > 0 {
			body := string(a.Alert)
			bodies[i] = &body
		}
		fingerprints[i] = nullIfEmpty(a.Fingerprint)
		tiers[i] = string(a.Tier)
		messageIDs[i] = a.MessageID
		indexes[i] = int32(a.MessageIndex) // #nosec G115 -- a message carries a handful of alerts
		rawBodies[i] = a.RawBody
		contentTypes[i] = nullIfEmpty(a.ContentType)
		errorReasons[i] = nullIfEmpty(a.ErrorReason)
		errorTexts[i] = nullIfEmpty(a.Error)
		receivedOn[i] = a.ReceivedOn
	}
	var inserted, notified int64
	if err := r.db.QueryRow(ctx, insertAlertsQuery, ids, sources, bodies, fingerprints, tiers, messageIDs, indexes,
		rawBodies, contentTypes, errorReasons, errorTexts, receivedOn, AlertStoredChannel).Scan(&inserted, &notified); err != nil {
		return 0, fmt.Errorf("insert alerts: %w", err)
	}
	return inserted, nil
}
