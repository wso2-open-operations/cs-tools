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
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertRepository stores the SRE alert messages the alert consumer receives.
type AlertRepository interface {
	// ReserveAlertIDs returns n new ALT numbers, in ascending order.
	ReserveAlertIDs(ctx context.Context, n int) ([]string, error)
	// InsertAlerts stores a message's alerts in one statement and returns the message's rows as stored, with how many were new; a redelivered message returns the rows stored the first time.
	InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, int64, error)
}

// Alert numbers are ALT plus nine digits, the format alert-ingestion writes, so they sort in arrival order.
const (
	alertIDPrefix = "ALT"
	alertIDWidth  = 9
)

const reserveAlertIDsQuery = `SELECT nextval('alert_seq') FROM generate_series(1, $1)`

// alertFieldColumns are the normalised alert's columns, the fingerprint's five first; alertFieldDests gives their scan targets in the same order.
const alertFieldColumns = `coalesce(source, ''), coalesce(service, ''), coalesce(metric_name, ''), coalesce(environment, ''), coalesce(unique_identifier, ''),
coalesce(severity, ''), coalesce(category, ''), coalesce(description, ''), coalesce(assignment_group, ''), coalesce(source_topic, ''), coalesce(source_account, '')`

// fingerprintFields is how many of alertFieldColumns make up the fingerprint: source, service, metric_name, environment and unique_identifier.
const fingerprintFields = 5

// alertFieldDests are the scan targets for alertFieldColumns.
func alertFieldDests(a *model.Alert) []any {
	return []any{&a.Source, &a.Service, &a.MetricName, &a.Environment, &a.UniqueIdentifier,
		&a.Severity, &a.Category, &a.Description, &a.AssignmentGroup, &a.SourceTopic, &a.SourceAccount}
}

// insertAlertsQuery stores a message's rows and returns them as stored. A redelivered message conflicts on (message_id, message_index); the no-op update makes RETURNING hand back the row stored the first time, so the consumer can publish it again without a second query. inserted is true only for a row this statement created.
const insertAlertsQuery = `INSERT INTO alert (number, route, tier, message_id, message_index, raw_body, received_on,
	source, service, metric_name, environment, unique_identifier, severity, category, description, assignment_group, source_topic, source_account)
SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::int[], $6::bytea[], $7::timestamptz[],
	$8::text[], $9::text[], $10::text[], $11::text[], $12::text[], $13::text[], $14::text[], $15::text[], $16::text[], $17::text[], $18::text[])
ON CONFLICT (message_id, message_index) DO UPDATE SET message_index = EXCLUDED.message_index
RETURNING id::text, number, coalesce(route, ''), tier, message_id, message_index, received_on, ` + alertFieldColumns + `, (xmax = 0)`

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

func (r *alertRepository) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, int64, error) {
	if len(alerts) == 0 {
		return nil, 0, nil
	}
	n := len(alerts)
	numbers, tiers, messageIDs := make([]string, n), make([]string, n), make([]string, n)
	// route is NULL when empty, so the database refuses an alert without one.
	routes := make([]*string, n)
	indexes := make([]int32, n)
	rawBodies := make([][]byte, n)
	receivedOn := make([]time.Time, n)
	// One array per alert field, in alertFieldColumns order; an empty field is stored as NULL.
	fields := make([][]*string, 11)
	for f := range fields {
		fields[f] = make([]*string, n)
	}
	for i, a := range alerts {
		numbers[i], tiers[i], messageIDs[i] = a.Number, string(a.Tier), a.MessageID
		routes[i] = nullIfEmpty(a.Route)
		indexes[i] = int32(a.MessageIndex) // #nosec G115 -- the consumer caps alerts per message far below MaxInt32
		rawBodies[i] = a.RawBody
		if a.MessageIndex == 0 && rawBodies[i] == nil {
			// An empty body is still the first row's body, so it is stored as empty rather than NULL.
			rawBodies[i] = []byte{}
		}
		receivedOn[i] = a.ReceivedOn
		alert := a.Alert
		for f, dest := range alertFieldDests(&alert) {
			value := *dest.(*string)
			if f < fingerprintFields {
				// The fingerprint's fields are matched with =, so an empty one is stored as '' and never NULL.
				fields[f][i] = &value
				continue
			}
			fields[f][i] = nullIfEmpty(value)
		}
	}
	args := []any{numbers, routes, tiers, messageIDs, indexes, rawBodies, receivedOn}
	for _, f := range fields {
		args = append(args, f)
	}
	rows, err := r.db.Query(ctx, insertAlertsQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("insert alerts: %w", err)
	}
	var inserted int64
	stored, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.NewAlert, error) {
		var a domain.NewAlert
		var tier string
		var isNew bool
		dests := append([]any{&a.ID, &a.Number, &a.Route, &tier, &a.MessageID, &a.MessageIndex, &a.ReceivedOn}, alertFieldDests(&a.Alert)...)
		if err := row.Scan(append(dests, &isNew)...); err != nil {
			return domain.NewAlert{}, err
		}
		a.Tier = domain.AlertTier(tier)
		if isNew {
			inserted++
		}
		return a, nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("insert alerts: %w", err)
	}
	slices.SortFunc(stored, func(a, b domain.NewAlert) int { return a.MessageIndex - b.MessageIndex })
	return stored, inserted, nil
}
