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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// allocationSyncActor stamps created_by/updated_by on rows the allocation events write.
const allocationSyncActor = "allocation-sync"

// CustomerEngagementAllocationRepository runs one allocation event in one transaction.
type CustomerEngagementAllocationRepository interface {
	InTx(ctx context.Context, fn func(store AllocationEventStore) error) error
}

// AllocationEventStore is the set of statements an allocation event runs inside its transaction.
type AllocationEventStore interface {
	// FindEngagementByEngagementID returns the customer_engagement id, or nil.
	FindEngagementByEngagementID(ctx context.Context, engagementID string) (*string, error)
	// FindEngagementByLineItemSfID returns the engagement for a Salesforce line-item id, or nil.
	FindEngagementByLineItemSfID(ctx context.Context, lineItemSfID string) (*string, error)
	// FindAccountBySfID returns the account for a Salesforce id (a live row first), or nil.
	FindAccountBySfID(ctx context.Context, sfID string) (*string, error)
	// FindAccountsByName returns every account with exactly this name.
	FindAccountsByName(ctx context.Context, name string) ([]domain.AccountCandidate, error)
	// FindUserByEmailOrUserName matches lower(email) first, then user_name; nil when none.
	FindUserByEmailOrUserName(ctx context.Context, email string) (*string, error)
	// InsertEngagement inserts unless engagement_id exists; returns the id and whether it inserted.
	InsertEngagement(ctx context.Context, e domain.NewCustomerEngagement) (id string, created bool, err error)
	// UpdateAllocationResource updates the (engagement, allocation_id) row; nil when there is none.
	UpdateAllocationResource(ctx context.Context, f domain.AllocationResourceFields) (*string, error)
	// UpsertAllocationResource inserts the row, or updates it if a racing insert won.
	UpsertAllocationResource(ctx context.Context, f domain.AllocationResourceFields, resourceID string) (id string, created bool, err error)
}

type customerEngagementAllocationRepo struct {
	db *Scoped
}

// NewCustomerEngagementAllocationRepository constructs the repository.
func NewCustomerEngagementAllocationRepository(db *Scoped) CustomerEngagementAllocationRepository {
	return &customerEngagementAllocationRepo{db: db}
}

// InTx implements CustomerEngagementAllocationRepository.
func (r *customerEngagementAllocationRepo) InTx(ctx context.Context, fn func(store AllocationEventStore) error) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		return fn(&allocationEventStore{q: tx})
	})
}

type allocationEventStore struct {
	q querier
}

// optionalID scans one id column, mapping no rows to nil.
func optionalID(row pgx.Row, op string) (*string, error) {
	var id string
	err := row.Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &id, nil
}

func (s *allocationEventStore) FindEngagementByEngagementID(ctx context.Context, engagementID string) (*string, error) {
	return optionalID(s.q.QueryRow(ctx,
		`SELECT id::text FROM customer_engagement WHERE engagement_id = $1 FOR UPDATE`, engagementID),
		"find engagement by engagement_id")
}

// ServiceNow matches u_line_item.u_id (line_item_id's product row), else the sf_id copy;
// 15 and 18-character forms match, and a line-item match wins over an sf_id one.
const findEngagementByLineItemQuery = `
	SELECT ce.id::text
	FROM customer_engagement ce
	LEFT JOIN sf_opportunity_product sop
	       ON sop.id = ce.line_item_id AND left(sop.line_item_sf_id, 15) = left($1, 15)
	WHERE sop.id IS NOT NULL OR left(ce.sf_id, 15) = left($1, 15)
	ORDER BY (sop.id IS NOT NULL) DESC, ce.created_on, ce.id
	LIMIT 1`

func (s *allocationEventStore) FindEngagementByLineItemSfID(ctx context.Context, lineItemSfID string) (*string, error) {
	return optionalID(s.q.QueryRow(ctx, findEngagementByLineItemQuery, lineItemSfID),
		"find engagement by line item")
}

// A 15-character id is the case-sensitive prefix of the 18-character form.
const findAccountBySfIDQuery = `
	SELECT a.id::text
	FROM account a
	WHERE a.sf_id = $1 OR (length($1) = 15 AND left(a.sf_id, 15) = $1)
	ORDER BY (a.deleted_on IS NULL) DESC, ` + accountReferencedOrder + `
	LIMIT 1`

func (s *allocationEventStore) FindAccountBySfID(ctx context.Context, sfID string) (*string, error) {
	return optionalID(s.q.QueryRow(ctx, findAccountBySfIDQuery, sfID), "find account by sf_id")
}

func (s *allocationEventStore) FindAccountsByName(ctx context.Context, name string) ([]domain.AccountCandidate, error) {
	rows, err := s.q.Query(ctx, `SELECT id::text, deleted_on IS NULL FROM account WHERE name = $1 ORDER BY id`, name)
	if err != nil {
		return nil, fmt.Errorf("find accounts by name: %w", err)
	}
	defer rows.Close()
	var out []domain.AccountCandidate
	for rows.Next() {
		var c domain.AccountCandidate
		if err := rows.Scan(&c.ID, &c.Live); err != nil {
			return nil, fmt.Errorf("find accounts by name: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find accounts by name: %w", err)
	}
	return out, nil
}

// Email may repeat across users, so prefer an active, internal, older row.
const findUserByEmailOrUserNameQuery = `
	SELECT u.id::text
	FROM "user" u
	WHERE lower(u.email) = lower($1) OR u.user_name = $1
	ORDER BY COALESCE(lower(u.email) = lower($1), false) DESC, u.is_active DESC NULLS LAST,
	         (u.user_type::text = 'INTERNAL') DESC, u.created_on, u.id
	LIMIT 1`

func (s *allocationEventStore) FindUserByEmailOrUserName(ctx context.Context, email string) (*string, error) {
	return optionalID(s.q.QueryRow(ctx, findUserByEmailOrUserNameQuery, email), "find user by email")
}

const insertEngagementQuery = `
	INSERT INTO customer_engagement (
		id, created_on, updated_on, created_by, updated_by, name, engagement_id, engagement_code,
		state, delivery_mode, is_paid, account_id, engagement_type, planned_start_date, planned_end_date)
	VALUES (gen_random_uuid(), now(), now(), $1, $1, $2, $3, $4,
		'NEW', $5::customer_engagement_delivery_mode_enum, $6, $7::uuid, $8::customer_engagement_type_enum, $9::date, $10::date)
	ON CONFLICT (engagement_id) WHERE engagement_id IS NOT NULL DO NOTHING
	RETURNING id::text`

func (s *allocationEventStore) InsertEngagement(ctx context.Context, e domain.NewCustomerEngagement) (string, bool, error) {
	id, err := optionalID(s.q.QueryRow(ctx, insertEngagementQuery, allocationSyncActor, e.Name, e.EngagementID,
		e.EngagementCode, e.DeliveryMode, e.IsPaid, e.AccountID, e.EngagementType,
		e.PlannedStartDate, e.PlannedEndDate), "insert engagement")
	if err != nil {
		return "", false, err
	}
	if id != nil {
		return *id, true, nil
	}
	// A concurrent insert won; it has committed, so this statement's snapshot sees it.
	existing, err := s.FindEngagementByEngagementID(ctx, e.EngagementID)
	if err != nil {
		return "", false, err
	}
	if existing == nil {
		return "", false, fmt.Errorf("insert engagement: conflict on %q but no row found", e.EngagementID)
	}
	return *existing, false, nil
}

// COALESCE keeps the stored state when the event's clearance status has no enum value.
const updateAllocationResourceQuery = `
	UPDATE customer_engagement_allocation_resource
	SET start_date = $3::date, end_date = $4::date, start_time = $5, end_time = $6, timezone = $7,
	    state = COALESCE($8::engagement_allocation_state_enum, state),
	    updated_on = now(), updated_by = $9
	WHERE engagement_id = $1::uuid AND allocation_id = $2
	RETURNING id::text`

func (s *allocationEventStore) UpdateAllocationResource(ctx context.Context, f domain.AllocationResourceFields) (*string, error) {
	return optionalID(s.q.QueryRow(ctx, updateAllocationResourceQuery, f.EngagementID, f.AllocationID,
		f.StartDate, f.EndDate, f.StartTime, f.EndTime, f.TimeZone, f.State, allocationSyncActor),
		"update allocation resource")
}

const upsertAllocationResourceQuery = `
	INSERT INTO customer_engagement_allocation_resource (
		id, created_on, updated_on, created_by, updated_by, allocation_id, engagement_id, resource_id,
		state, start_date, end_date, start_time, end_time, timezone)
	VALUES (gen_random_uuid(), now(), now(), $9, $9, $2, $1::uuid, $10::uuid,
		$8::engagement_allocation_state_enum, $3::date, $4::date, $5, $6, $7)
	ON CONFLICT (engagement_id, allocation_id) WHERE engagement_id IS NOT NULL AND allocation_id IS NOT NULL
	DO UPDATE SET start_date = EXCLUDED.start_date, end_date = EXCLUDED.end_date,
		start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time, timezone = EXCLUDED.timezone,
		state = COALESCE(EXCLUDED.state, customer_engagement_allocation_resource.state),
		updated_on = now(), updated_by = EXCLUDED.updated_by
	RETURNING id::text, (xmax = 0)`

func (s *allocationEventStore) UpsertAllocationResource(ctx context.Context, f domain.AllocationResourceFields, resourceID string) (string, bool, error) {
	var id string
	var created bool
	err := s.q.QueryRow(ctx, upsertAllocationResourceQuery, f.EngagementID, f.AllocationID,
		f.StartDate, f.EndDate, f.StartTime, f.EndTime, f.TimeZone, f.State, allocationSyncActor, resourceID).
		Scan(&id, &created)
	if err != nil {
		return "", false, fmt.Errorf("upsert allocation resource: %w", err)
	}
	return id, created, nil
}
