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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// EngagementAllocationViewRepository reads customer engagement allocations
// and the status updates written against them, and writes new updates.
type EngagementAllocationViewRepository interface {
	// SearchAllocations returns one page of allocations and the total that match.
	SearchAllocations(ctx context.Context, req domain.SearchEngagementAllocationsRequest) ([]domain.EngagementAllocationView, int, error)
	// AllocationByID returns one allocation, or a NotFoundError.
	AllocationByID(ctx context.Context, id string) (domain.EngagementAllocationView, error)
	// ListUpdates returns one page of an allocation's updates, newest first, and the total.
	ListUpdates(ctx context.Context, allocationID string, limit, offset int) ([]domain.EngagementAllocationUpdate, int, error)
	// CreateUpdate stores one update and returns it.
	CreateUpdate(ctx context.Context, w EngagementAllocationUpdateWrite) (domain.EngagementAllocationUpdate, error)
	// UserByEmail resolves a caller's email to the platform user record, or nil
	// when there is none.
	UserByEmail(ctx context.Context, email string) (*domain.EngagementAllocationPerson, error)
}

// EngagementAllocationUpdateWrite is everything a new update row is made of.
// The service decides all of it; the repository only stores it.
type EngagementAllocationUpdateWrite struct {
	AllocationID   string
	EngagementID   string
	AuthorID       string
	Subject        string
	Content        string
	State          string
	Frequency      string
	Scope          string
	Visibility     string
	CycleStartDate string
	ActorEmail     string
}

type engagementAllocationViewRepo struct{ db *pgxpool.Pool }

// NewEngagementAllocationViewRepository constructs the repository over db.
func NewEngagementAllocationViewRepository(db *pgxpool.Pool) EngagementAllocationViewRepository {
	return &engagementAllocationViewRepo{db: db}
}

const allocationViewSelect = `
	SELECT ar.id::text, ar.allocation_id, ar.state::text,
	       to_char(ar.start_date, 'YYYY-MM-DD'), to_char(ar.end_date, 'YYYY-MM-DD'),
	       ar.start_time, ar.end_time, ar.timezone, ar.utilization::float8,
	       ce.id::text, ce.engagement_id, ce.name, ce.engagement_type::text, ce.delivery_mode::text,
	       ce.state::text, ce.business_unit::text, ce.is_paid, ce.project_id::text,
	       to_char(ce.planned_start_date, 'YYYY-MM-DD'), to_char(ce.planned_end_date, 'YYYY-MM-DD'),
	       to_char(ce.actual_start_date, 'YYYY-MM-DD'), to_char(ce.actual_end_date, 'YYYY-MM-DD'),
	       a.id::text, a.name,
	       u.id::text, ` + allocationUserName + `, u.email,
	       (SELECT max(su.published_date) FROM customer_engagement_status_update su
	         WHERE su.allocation_id = ar.id AND su.state = 'PUBLISHED')
	FROM customer_engagement_allocation_resource ar
	JOIN customer_engagement ce ON ce.id = ar.engagement_id
	LEFT JOIN account a ON a.id = ce.account_id
	LEFT JOIN "user" u ON u.id = ar.resource_id`

// allocationUserName is a person's display name: the name column, else first
// and last name together.
const allocationUserName = `COALESCE(NULLIF(u.name, ''), NULLIF(btrim(concat_ws(' ', u.first_name, u.last_name)), ''), u.user_name)`

// allocationAuthorName is the same expression over the author's alias.
var allocationAuthorName = strings.ReplaceAll(allocationUserName, "u.", "au.")

// allocationSortColumns maps a validated sort field to its SQL expression.
var allocationSortColumns = map[string]string{
	"startDate":    "ar.start_date",
	"endDate":      "ar.end_date",
	"state":        "ar.state::text",
	"allocationId": "ar.allocation_id",
	"accountName":  "lower(a.name)",
	"engineerName": "lower(" + allocationUserName + ")",
	"utilization":  "ar.utilization",
}

// IsEngagementAllocationSortField reports whether field is a sortable column.
func IsEngagementAllocationSortField(field string) bool {
	_, ok := allocationSortColumns[field]
	return ok
}

func allocationOrderBy(s domain.EngagementAllocationSortBy) string {
	col, ok := allocationSortColumns[s.Field]
	if !ok {
		return "ar.start_date DESC NULLS LAST, ar.id"
	}
	dir := "ASC"
	if strings.EqualFold(s.Order, "desc") {
		dir = "DESC"
	}
	return col + " " + dir + " NULLS LAST, ar.id"
}

// buildAllocationWhere renders the WHERE clause of an allocation search and its
// arguments. The service has already validated every id, date and state.
func buildAllocationWhere(f domain.EngagementAllocationFilters) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	add := func(clause string, v any) {
		args = append(args, v)
		where += " AND " + fmt.Sprintf(clause, len(args))
	}
	if f.SearchQuery != "" {
		add(`(ar.allocation_id ILIKE $%[1]d ESCAPE '\' OR ce.name ILIKE $%[1]d ESCAPE '\' OR ce.engagement_id ILIKE $%[1]d ESCAPE '\')`,
			"%"+escapeLikeWildcards(f.SearchQuery)+"%")
	}
	if f.AccountID != "" {
		add("ce.account_id = $%d::uuid", f.AccountID)
	}
	if f.AccountName != "" {
		add(`a.name ILIKE $%d ESCAPE '\'`, "%"+escapeLikeWildcards(f.AccountName)+"%")
	}
	if f.EngineerID != "" {
		add("ar.resource_id = $%d::uuid", f.EngineerID)
	}
	if f.EngagementID != "" {
		add("ar.engagement_id = $%d::uuid", f.EngagementID)
	}
	if len(f.States) > 0 {
		add("ar.state::text = ANY($%d::text[])", f.States)
	}
	if f.From != "" {
		add("ar.end_date >= $%d::date", f.From)
	}
	if f.To != "" {
		add("ar.start_date <= $%d::date", f.To)
	}
	return where, args
}

// SearchAllocations implements EngagementAllocationViewRepository.
func (r *engagementAllocationViewRepo) SearchAllocations(ctx context.Context, req domain.SearchEngagementAllocationsRequest) ([]domain.EngagementAllocationView, int, error) {
	where, args := buildAllocationWhere(req.Filters)

	var total int
	countSQL := `SELECT count(*) FROM customer_engagement_allocation_resource ar
		JOIN customer_engagement ce ON ce.id = ar.engagement_id
		LEFT JOIN account a ON a.id = ce.account_id
		LEFT JOIN "user" u ON u.id = ar.resource_id ` + where
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count engagement allocations: %w", err)
	}

	pageArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)
	rows, err := r.db.Query(ctx, allocationViewSelect+" "+where+
		" ORDER BY "+allocationOrderBy(req.SortBy)+
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2), pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query engagement allocations: %w", err)
	}
	defer rows.Close()

	out := []domain.EngagementAllocationView{}
	for rows.Next() {
		v, err := scanAllocationView(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate engagement allocations: %w", err)
	}
	return out, total, nil
}

// AllocationByID implements EngagementAllocationViewRepository.
func (r *engagementAllocationViewRepo) AllocationByID(ctx context.Context, id string) (domain.EngagementAllocationView, error) {
	rows, err := r.db.Query(ctx, allocationViewSelect+` WHERE ar.id = $1::uuid`, id)
	if err != nil {
		return domain.EngagementAllocationView{}, fmt.Errorf("query engagement allocation: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.EngagementAllocationView{}, fmt.Errorf("query engagement allocation: %w", err)
		}
		return domain.EngagementAllocationView{}, &apierror.NotFoundError{Msg: "engagement allocation not found"}
	}
	return scanAllocationView(rows)
}

func scanAllocationView(rows pgx.Rows) (domain.EngagementAllocationView, error) {
	var v domain.EngagementAllocationView
	var accID, accName, engID, engName, engEmail *string
	if err := rows.Scan(&v.ID, &v.AllocationID, &v.State,
		&v.StartDate, &v.EndDate, &v.StartTime, &v.EndTime, &v.Timezone, &v.Utilization,
		&v.Engagement.ID, &v.Engagement.DisplayID, &v.Engagement.Name, &v.Engagement.Type, &v.Engagement.ModeOfDelivery,
		&v.Engagement.State, &v.Engagement.BusinessUnit, &v.Engagement.IsPaid, &v.Engagement.ProjectID,
		&v.Engagement.PlannedStartDate, &v.Engagement.PlannedEndDate, &v.Engagement.ActualStartDate, &v.Engagement.ActualEndDate,
		&accID, &accName, &engID, &engName, &engEmail, &v.LatestUpdateDate); err != nil {
		return domain.EngagementAllocationView{}, fmt.Errorf("scan engagement allocation: %w", err)
	}
	if accID != nil {
		v.Account = &domain.EngagementAllocationAccount{ID: *accID, Name: stringOrEmpty(accName)}
	}
	if engID != nil {
		v.Engineer = &domain.EngagementAllocationPerson{ID: *engID, Name: stringOrEmpty(engName), Email: engEmail}
	}
	return v, nil
}

var allocationUpdateSelect = `
	SELECT su.id::text, su.subject, su.content, su.state::text, su.visibility::text,
	       to_char(su.cycle_start_date, 'YYYY-MM-DD'), su.published_date, su.created_on,
	       au.id::text, ` + allocationAuthorName + `, au.email
	FROM customer_engagement_status_update su
	LEFT JOIN "user" au ON au.id = su.author_id`

func scanAllocationUpdate(rows pgx.Rows) (domain.EngagementAllocationUpdate, error) {
	var up domain.EngagementAllocationUpdate
	var authorID, authorName, authorEmail *string
	if err := rows.Scan(&up.ID, &up.Subject, &up.Content, &up.State, &up.Visibility,
		&up.CycleStartDate, &up.PublishedDate, &up.CreatedOn, &authorID, &authorName, &authorEmail); err != nil {
		return domain.EngagementAllocationUpdate{}, fmt.Errorf("scan engagement allocation update: %w", err)
	}
	if authorID != nil {
		up.Author = &domain.EngagementAllocationPerson{ID: *authorID, Name: stringOrEmpty(authorName), Email: authorEmail}
	}
	return up, nil
}

// ListUpdates implements EngagementAllocationViewRepository.
func (r *engagementAllocationViewRepo) ListUpdates(ctx context.Context, allocationID string, limit, offset int) ([]domain.EngagementAllocationUpdate, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM customer_engagement_status_update WHERE allocation_id = $1::uuid`, allocationID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count engagement allocation updates: %w", err)
	}
	rows, err := r.db.Query(ctx, allocationUpdateSelect+`
		WHERE su.allocation_id = $1::uuid
		ORDER BY COALESCE(su.published_date, su.created_on) DESC, su.id DESC
		LIMIT $2 OFFSET $3`, allocationID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query engagement allocation updates: %w", err)
	}
	defer rows.Close()
	out := []domain.EngagementAllocationUpdate{}
	for rows.Next() {
		up, err := scanAllocationUpdate(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, up)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate engagement allocation updates: %w", err)
	}
	return out, total, nil
}

// CreateUpdate implements EngagementAllocationViewRepository.
func (r *engagementAllocationViewRepo) CreateUpdate(ctx context.Context, w EngagementAllocationUpdateWrite) (domain.EngagementAllocationUpdate, error) {
	var id string
	var createdOn time.Time
	var published *time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO customer_engagement_status_update
		  (created_on, updated_on, created_by, updated_by, engagement_id, allocation_id, author_id,
		   subject, content, state, frequency, scope, visibility, cycle_start_date, published_date, mailing_list)
		VALUES (now(), now(), $1, $1, $2::uuid, $3::uuid, $4::uuid,
		        $5, $6, $7::customer_engagement_status_update_state_enum,
		        $8::customer_engagement_status_update_frequency_enum,
		        $9::customer_engagement_status_update_scope_enum,
		        $10::customer_engagement_status_update_visibility_enum,
		        $11::date, now(), NULL)
		RETURNING id::text, created_on, published_date`,
		w.ActorEmail, w.EngagementID, w.AllocationID, w.AuthorID,
		w.Subject, w.Content, w.State, w.Frequency, w.Scope, w.Visibility, w.CycleStartDate).
		Scan(&id, &createdOn, &published)
	if err != nil {
		return domain.EngagementAllocationUpdate{}, fmt.Errorf("insert engagement allocation update: %w", err)
	}
	return domain.EngagementAllocationUpdate{
		ID: id, Subject: &w.Subject, Content: &w.Content, State: &w.State, Visibility: &w.Visibility,
		CycleStartDate: &w.CycleStartDate, PublishedDate: published, CreatedOn: createdOn,
	}, nil
}

// UserByEmail implements EngagementAllocationViewRepository.
func (r *engagementAllocationViewRepo) UserByEmail(ctx context.Context, email string) (*domain.EngagementAllocationPerson, error) {
	var p domain.EngagementAllocationPerson
	var em *string
	err := r.db.QueryRow(ctx, `SELECT u.id::text, `+allocationUserName+`, u.email FROM "user" u
		WHERE lower(u.email) = lower($1) ORDER BY (u.email = $1) DESC LIMIT 1`, email).Scan(&p.ID, &p.Name, &em)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user by email: %w", err)
	}
	p.Email = em
	return &p, nil
}
