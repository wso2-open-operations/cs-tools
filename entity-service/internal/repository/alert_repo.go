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

// AlertRepository stores the SRE alerts the Service Bus consumer receives.
type AlertRepository interface {
	// ReserveAlertIDs returns n new alert ids, in ascending order.
	ReserveAlertIDs(ctx context.Context, n int) ([]string, error)
	// InsertAlerts stores alerts in one statement and returns how many were new; a row whose message was already stored is skipped.
	InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error)
}

// Alert ids are ALT plus nine digits, the format alert-ingestion writes, so they sort in arrival order.
const (
	alertIDPrefix = "ALT"
	alertIDWidth  = 9
)

const reserveAlertIDsQuery = `SELECT nextval('alert_seq') FROM generate_series(1, $1)`

const insertAlertsQuery = `INSERT INTO alert (id, source, alert, fingerprint, tier, message_id, message_index, received_on)
SELECT * FROM unnest($1::text[], $2::text[], $3::jsonb[], $4::text[], $5::text[], $6::text[], $7::int[], $8::timestamptz[])
ON CONFLICT (message_id, message_index) DO NOTHING`

type alertRepository struct {
	db *Scoped
}

// NewAlertRepository returns an AlertRepository; callers run it under the system identity because alert is internal only.
func NewAlertRepository(db *Scoped) AlertRepository {
	return &alertRepository{db: db}
}

// FormatAlertID renders a sequence value as an alert id, such as ALT000000042.
func FormatAlertID(seq int64) string {
	return fmt.Sprintf("%s%0*d", alertIDPrefix, alertIDWidth, seq)
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
	if len(seqs) != n {
		return nil, fmt.Errorf("reserve alert ids: got %d ids, want %d", len(seqs), n)
	}
	ids := make([]string, len(seqs))
	for i, seq := range seqs {
		ids[i] = FormatAlertID(seq)
	}
	return ids, nil
}

func (r *alertRepository) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error) {
	if len(alerts) == 0 {
		return 0, nil
	}
	ids := make([]string, len(alerts))
	sources := make([]string, len(alerts))
	bodies := make([]string, len(alerts))
	fingerprints := make([]string, len(alerts))
	tiers := make([]string, len(alerts))
	messageIDs := make([]string, len(alerts))
	indexes := make([]int32, len(alerts))
	receivedOn := make([]time.Time, len(alerts))
	for i, a := range alerts {
		ids[i] = a.ID
		sources[i] = a.Source
		bodies[i] = string(a.Alert)
		fingerprints[i] = a.Fingerprint
		tiers[i] = string(a.Tier)
		messageIDs[i] = a.MessageID
		indexes[i] = int32(a.MessageIndex) // #nosec G115 -- the consumer caps alerts per message far below MaxInt32
		receivedOn[i] = a.ReceivedOn
	}
	tag, err := r.db.Exec(ctx, insertAlertsQuery, ids, sources, bodies, fingerprints, tiers, messageIDs, indexes, receivedOn)
	if err != nil {
		return 0, fmt.Errorf("insert alerts: %w", err)
	}
	return tag.RowsAffected(), nil
}
