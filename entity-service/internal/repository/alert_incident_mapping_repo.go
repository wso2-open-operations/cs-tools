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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertIncidentMappingRepository defines the persistence operations for the
// alert_incident_mapping table — see domain.AlertIncidentMappingView's doc
// comment for what it's for.
type AlertIncidentMappingRepository interface {
	// Create inserts a new mapping row. Returns a *apierror.ConflictError if
	// req.AlertNumber is already mapped (alert_number is UNIQUE).
	Create(ctx context.Context, req domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error)
	// Lookup returns every mapping for (source, uniqueIdentifier),
	// most-recent-first. Returns an empty (never nil) slice, not an error,
	// when nothing matches.
	Lookup(ctx context.Context, source, uniqueIdentifier string) ([]domain.AlertIncidentMappingView, error)
}

type alertIncidentMappingRepo struct {
	db *pgxpool.Pool
}

// NewAlertIncidentMappingRepository constructs an AlertIncidentMappingRepository
// backed by the given connection pool.
func NewAlertIncidentMappingRepository(db *pgxpool.Pool) AlertIncidentMappingRepository {
	return &alertIncidentMappingRepo{db: db}
}

// alertIncidentMappingColumns is the column list shared by every query that
// returns a full row, kept in one place so the various methods below can't
// drift out of sync with scanAlertIncidentMapping's field order.
const alertIncidentMappingColumns = `id, alert_number, source, unique_identifier, service, metric_name, alert_status, incident_id, incident_number, created_at`

func scanAlertIncidentMapping(row pgx.Row) (domain.AlertIncidentMappingView, error) {
	var m domain.AlertIncidentMappingView
	var createdAt time.Time
	if err := row.Scan(
		&m.ID, &m.AlertNumber, &m.Source, &m.UniqueIdentifier, &m.Service, &m.MetricName,
		&m.AlertStatus, &m.IncidentID, &m.IncidentNumber, &createdAt,
	); err != nil {
		return domain.AlertIncidentMappingView{}, err
	}
	m.CreatedAt = createdAt.Format(time.RFC3339)
	return m, nil
}

// Create implements AlertIncidentMappingRepository.
func (r *alertIncidentMappingRepo) Create(ctx context.Context, req domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error) {
	query := `
		INSERT INTO alert_incident_mapping
			(alert_number, source, unique_identifier, service, metric_name, alert_status, incident_id, incident_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + alertIncidentMappingColumns

	m, err := scanAlertIncidentMapping(r.db.QueryRow(ctx, query,
		req.AlertNumber, req.Source, req.UniqueIdentifier, req.Service, req.MetricName,
		req.AlertStatus, req.IncidentID, req.IncidentNumber))
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation on alert_number
				return domain.AlertIncidentMappingView{}, &apierror.ConflictError{Msg: "alertNumber is already mapped to an incident: " + req.AlertNumber}
			}
		}
		return domain.AlertIncidentMappingView{}, fmt.Errorf("create alert_incident_mapping: %w", err)
	}
	return m, nil
}

// Lookup implements AlertIncidentMappingRepository.
func (r *alertIncidentMappingRepo) Lookup(ctx context.Context, source, uniqueIdentifier string) ([]domain.AlertIncidentMappingView, error) {
	query := `SELECT ` + alertIncidentMappingColumns + `
		FROM alert_incident_mapping
		WHERE source = $1 AND unique_identifier = $2
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, source, uniqueIdentifier)
	if err != nil {
		return nil, fmt.Errorf("lookup alert_incident_mapping: %w", err)
	}
	defer rows.Close()

	mappings := []domain.AlertIncidentMappingView{}
	for rows.Next() {
		m, err := scanAlertIncidentMapping(rows)
		if err != nil {
			return nil, fmt.Errorf("lookup alert_incident_mapping: scan: %w", err)
		}
		mappings = append(mappings, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lookup alert_incident_mapping: %w", err)
	}
	return mappings, nil
}
