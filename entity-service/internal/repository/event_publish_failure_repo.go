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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// EventPublishFailureRepository defines the persistence operations for the
// event_publish_failures table.
type EventPublishFailureRepository interface {
	// Create inserts a new unresolved failure row.
	Create(ctx context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error)
	// MarkResolved sets id's resolved_on, and returns the updated row.
	// Idempotent: calling it again on an already-resolved row is a no-op
	// success that preserves the original resolved_on (there is no
	// meaningful "unresolve" — a caller retrying its own HTTP call is not a
	// race against a different caller the way event_outbox's Claim was).
	// Returns a *apierror.NotFoundError if id does not exist.
	MarkResolved(ctx context.Context, id string) (domain.EventPublishFailure, error)
	// Search returns rows matching req's filters, newest first, together
	// with the total count before pagination.
	Search(ctx context.Context, req domain.SearchEventPublishFailuresRequest) ([]domain.EventPublishFailure, int, error)
}

type eventPublishFailureRepo struct {
	db *pgxpool.Pool
}

// NewEventPublishFailureRepository constructs an EventPublishFailureRepository
// backed by the given connection pool.
func NewEventPublishFailureRepository(db *pgxpool.Pool) EventPublishFailureRepository {
	return &eventPublishFailureRepo{db: db}
}

// eventPublishFailureColumns is the column list shared by every query that
// returns a full row, kept in one place so Create/MarkResolved/Search can't
// drift out of sync with scanEventPublishFailure's field order.
const eventPublishFailureColumns = `id, event_type, entity_id, payload, error, created_on, resolved_on`

func scanEventPublishFailure(row pgx.Row) (domain.EventPublishFailure, error) {
	var f domain.EventPublishFailure
	if err := row.Scan(
		&f.ID, &f.EventType, &f.EntityID, &f.Payload, &f.Error,
		&f.CreatedOn, &f.ResolvedOn,
	); err != nil {
		return domain.EventPublishFailure{}, err
	}
	return f, nil
}

// Create implements EventPublishFailureRepository.
func (r *eventPublishFailureRepo) Create(ctx context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error) {
	query := `
		INSERT INTO event_publish_failures (event_type, entity_id, payload, error)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + eventPublishFailureColumns

	f, err := scanEventPublishFailure(r.db.QueryRow(ctx, query, req.EventType, req.EntityID, req.Payload, req.Error))
	if err != nil {
		return domain.EventPublishFailure{}, fmt.Errorf("create event_publish_failure: %w", err)
	}
	return f, nil
}

// MarkResolved implements EventPublishFailureRepository.
func (r *eventPublishFailureRepo) MarkResolved(ctx context.Context, id string) (domain.EventPublishFailure, error) {
	query := `
		UPDATE event_publish_failures
		SET resolved_on = COALESCE(resolved_on, NOW())
		WHERE id = $1
		RETURNING ` + eventPublishFailureColumns

	f, err := scanEventPublishFailure(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EventPublishFailure{}, &apierror.NotFoundError{Msg: "event_publish_failure not found: " + id}
		}
		return domain.EventPublishFailure{}, fmt.Errorf("resolve event_publish_failure %s: %w", id, err)
	}
	return f, nil
}

// Search implements EventPublishFailureRepository.
func (r *eventPublishFailureRepo) Search(ctx context.Context, req domain.SearchEventPublishFailuresRequest) ([]domain.EventPublishFailure, int, error) {
	var whereClause string
	args := []any{req.Pagination.Limit, req.Pagination.Offset}
	if req.Filters.Resolved != nil {
		if *req.Filters.Resolved {
			whereClause = "WHERE resolved_on IS NOT NULL"
		} else {
			whereClause = "WHERE resolved_on IS NULL"
		}
	}

	countQuery := `SELECT COUNT(*) FROM event_publish_failures ` + whereClause
	dataQuery := `
		SELECT ` + eventPublishFailureColumns + `
		FROM event_publish_failures
		` + whereClause + `
		ORDER BY created_on DESC, id
		LIMIT $1 OFFSET $2`

	var total int
	var rowsOut []domain.EventPublishFailure

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery).Scan(&total); err != nil {
			return fmt.Errorf("count event_publish_failures: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, args...)
		if err != nil {
			return fmt.Errorf("query event_publish_failures: %w", err)
		}
		defer rows.Close()

		result := make([]domain.EventPublishFailure, 0, req.Pagination.Limit)
		for rows.Next() {
			f, err := scanEventPublishFailure(rows)
			if err != nil {
				return fmt.Errorf("scan event_publish_failure: %w", err)
			}
			result = append(result, f)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate event_publish_failures: %w", err)
		}
		rowsOut = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return rowsOut, total, nil
}
