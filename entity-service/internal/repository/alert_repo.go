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
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertRepository stores the SRE alert messages the alert consumer receives.
type AlertRepository interface {
	// ReserveAlertIDs returns n new ALT numbers, in ascending order.
	ReserveAlertIDs(ctx context.Context, n int) ([]string, error)
	// InsertAlerts stores a message's alerts in one statement and returns them as stored, with the UUID each was given.
	InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, error)
}

// Alert numbers are ALT plus nine digits, the format alert-ingestion writes, so they sort in arrival order.
const (
	alertIDPrefix = "ALT"
	alertIDWidth  = 9
)

const reserveAlertIDsQuery = `SELECT nextval('alert_seq') FROM generate_series(1, $1)`

// alertFieldColumns are the normalised alert's columns, the fingerprint's five first; alertFieldDests gives their scan targets in the same order.
const alertFieldColumns = `coalesce(source, ''), coalesce(service, ''), coalesce(metric_name, ''), coalesce(environment, ''), coalesce(unique_identifier, ''),
coalesce(severity, ''), coalesce(category, ''), coalesce(description, ''), coalesce(assignment_group, '')`

// fingerprintFields is how many of alertFieldColumns make up the fingerprint: source, service, metric_name, environment and unique_identifier.
const fingerprintFields = 5

// alertFieldDests are the scan targets for alertFieldColumns.
func alertFieldDests(a *model.Alert) []any {
	return []any{&a.Source, &a.Service, &a.MetricName, &a.Environment, &a.UniqueIdentifier,
		&a.Severity, &a.Category, &a.Description, &a.AssignmentGroup}
}

// insertAlertsQuery stores a message's alerts and returns them as stored, in the order given; $14 holds each alert's defaulted fields as JSON.
const insertAlertsQuery = `INSERT INTO alert (number, tier, raw_body, received_on,
	source, service, metric_name, environment, unique_identifier, severity, category, description, assignment_group, defaulted_fields)
SELECT * FROM unnest($1::varchar[], $2::varchar[], $3::bytea[], $4::timestamptz[],
	$5::varchar[], $6::varchar[], $7::varchar[], $8::varchar[], $9::varchar[], $10::varchar[], $11::varchar[], $12::varchar[], $13::varchar[], $14::jsonb[])
RETURNING id::text, number, tier, received_on, defaulted_fields, ` + alertFieldColumns

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

func (r *alertRepository) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, error) {
	if len(alerts) == 0 {
		return nil, nil
	}
	n := len(alerts)
	numbers, tiers := make([]string, n), make([]string, n)
	rawBodies := make([][]byte, n)
	receivedOn := make([]time.Time, n)
	defaulted := make([][]byte, n)
	// One array per alert field, in alertFieldColumns order.
	fields := make([][]*string, len(alertFieldDests(&model.Alert{})))
	for f := range fields {
		fields[f] = make([]*string, n)
	}
	for i, a := range alerts {
		numbers[i], tiers[i], rawBodies[i], receivedOn[i] = a.Number, string(a.Tier), a.RawBody, a.ReceivedOn
		alert := a.Alert
		var err error
		if defaulted[i], err = defaultedJSON(alert.Defaulted); err != nil {
			return nil, fmt.Errorf("insert alerts: %w", err)
		}
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
	args := []any{numbers, tiers, rawBodies, receivedOn}
	for _, f := range fields {
		args = append(args, f)
	}
	args = append(args, defaulted)
	// The rows are returned only after the commit succeeds, so a failed commit is never reported as stored alerts.
	stored, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) ([]domain.NewAlert, error) {
		rows, err := tx.Query(ctx, insertAlertsQuery, args...)
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.NewAlert, error) {
			var a domain.NewAlert
			var tier string
			var defaultedFields []model.Defaulted
			if err := row.Scan(append([]any{&a.ID, &a.Number, &tier, &a.ReceivedOn, &defaultedFields}, alertFieldDests(&a.Alert)...)...); err != nil {
				return domain.NewAlert{}, err
			}
			a.Tier = domain.AlertTier(tier)
			if len(defaultedFields) > 0 {
				a.Alert.Defaulted = defaultedFields
			}
			return a, nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("insert alerts: %w", err)
	}
	// Numbers are reserved in ascending order, so sorting by number restores the message's order.
	slices.SortFunc(stored, func(a, b domain.NewAlert) int { return strings.Compare(a.Number, b.Number) })
	return stored, nil
}

// defaultedJSON is the defaulted_fields value for an alert: its defaulted fields as a JSON array, [] when there are none.
func defaultedJSON(fields []model.Defaulted) ([]byte, error) {
	if len(fields) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(fields)
}
