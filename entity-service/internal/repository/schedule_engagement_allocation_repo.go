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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SearchEngagementAllocations returns the confirmed allocations of the given
// people whose [start_date, end_date] overlaps [from, to], both ends inclusive.
//
// Only CONFIRMED rows are returned: tentative, cancelled and rejected
// allocations are not work anyone is committed to. A row without a resource,
// an engagement or both dates cannot be drawn on a person's row and is left out.
func (r *scheduleRepository) SearchEngagementAllocations(ctx context.Context, req domain.SearchScheduleEngagementAllocationsRequest) ([]domain.ScheduleEngagementAllocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ar.id::text, ar.allocation_id, ar.resource_id::text, ar.engagement_id::text,
		       ce.engagement_id, a.name, ce.engagement_type::text, ce.delivery_mode::text,
		       ar.utilization::float8, ar.start_date, ar.end_date,
		       COALESCE((SELECT array_agg(to_char(x.rota_date, 'YYYY-MM-DD') ORDER BY x.rota_date)
		                   FROM team_schedule_allocation_dismissal x
		                  WHERE x.allocation_resource_id = ar.id
		                    AND x.rota_date BETWEEN $2::date AND $3::date), '{}')
		FROM customer_engagement_allocation_resource ar
		JOIN customer_engagement ce ON ce.id = ar.engagement_id
		LEFT JOIN account a ON a.id = ce.account_id
		WHERE ar.resource_id = ANY($1::uuid[])
		  AND ar.state = 'CONFIRMED'
		  AND ar.start_date IS NOT NULL AND ar.end_date IS NOT NULL
		  AND ar.start_date <= $3::date AND ar.end_date >= $2::date
		ORDER BY ar.resource_id, ar.start_date, ar.end_date, ar.id`,
		req.UserIDs, req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("query schedule engagement allocations: %w", err)
	}
	defer rows.Close()

	out := []domain.ScheduleEngagementAllocation{}
	for rows.Next() {
		var al domain.ScheduleEngagementAllocation
		var start, end time.Time
		if err := rows.Scan(&al.ID, &al.AllocationID, &al.UserID, &al.EngagementID,
			&al.EngagementDisplayID, &al.AccountName, &al.EngagementType, &al.ModeOfDelivery,
			&al.Utilization, &start, &end, &al.DismissedDates); err != nil {
			return nil, fmt.Errorf("scan schedule engagement allocation: %w", err)
		}
		al.StartDate = start.Format("2006-01-02")
		al.EndDate = end.Format("2006-01-02")
		if len(al.DismissedDates) == 0 {
			al.DismissedDates = nil
		}
		out = append(out, al)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule engagement allocations: %w", err)
	}
	return out, nil
}

// EngagementAllocationOnRoster reads one confirmed allocation by its id, for
// taking it off a roster or putting it back. A row that is not confirmed, has
// no person or no dates is not on any roster, so it is not found.
func (r *scheduleRepository) EngagementAllocationOnRoster(ctx context.Context, id string) (domain.EngagementAllocationOnRoster, error) {
	var out domain.EngagementAllocationOnRoster
	var start, end time.Time
	var code *string
	err := r.db.QueryRow(ctx, `
		SELECT ar.resource_id::text, ar.start_date, ar.end_date, ce.engagement_id
		  FROM customer_engagement_allocation_resource ar
		  JOIN customer_engagement ce ON ce.id = ar.engagement_id
		 WHERE ar.id = $1::uuid
		   AND ar.state = 'CONFIRMED'
		   AND ar.resource_id IS NOT NULL
		   AND ar.start_date IS NOT NULL AND ar.end_date IS NOT NULL`, id).Scan(&out.UserID, &start, &end, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, &apierror.NotFoundError{Msg: "that customer engagement allocation is not on the roster"}
	}
	if err != nil {
		return out, fmt.Errorf("read engagement allocation: %w", err)
	}
	out.StartDate = start.Format("2006-01-02")
	out.EndDate = end.Format("2006-01-02")
	if code != nil {
		out.Code = strings.TrimSpace(*code)
	}
	return out, nil
}

// DismissEngagementAllocation takes the allocation off the person's roster on
// every weekday of from..to (or, with restore, puts those days back), and
// records the change in the roster's history with the actor. It returns how
// many days changed; a day already in the asked state is left alone.
func (r *scheduleRepository) DismissEngagementAllocation(ctx context.Context, al domain.EngagementAllocationOnRoster,
	req domain.DismissScheduleEngagementAllocationRequest, actorEmail string) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin allocation removal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tag pgconn.CommandTag
	if req.Restore {
		tag, err = tx.Exec(ctx, `
			DELETE FROM team_schedule_allocation_dismissal
			 WHERE allocation_resource_id = $1::uuid
			   AND rota_date BETWEEN $2::date AND $3::date`, req.ID, req.From, req.To)
	} else {
		tag, err = tx.Exec(ctx, `
			INSERT INTO team_schedule_allocation_dismissal
			  (allocation_resource_id, user_id, team_key, rota_date, created_by)
			SELECT $1::uuid, $2::uuid, $3, d::date, $6
			  FROM generate_series($4::date, $5::date, interval '1 day') d
			 WHERE EXTRACT(ISODOW FROM d) < 6
			ON CONFLICT ON CONSTRAINT team_schedule_allocation_dismissal_day_unique DO NOTHING`,
			req.ID, al.UserID, req.TeamKey, req.From, req.To, actorEmail)
	}
	if err != nil {
		return 0, fmt.Errorf("change allocation days: %w", err)
	}
	days := int(tag.RowsAffected())
	if days > 0 {
		code := "Allo-" + al.Code
		if al.Code == "" {
			code = "Allo-ENG"
		}
		if len(code) > 64 {
			code = code[:64]
		}
		action, note := "DELETED", code+" (allocation app) taken off the roster"
		if req.Restore {
			action, note = "CREATED", code+" (allocation app) put back on the roster"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO team_schedule_absence_activity
			  (created_on, created_by, absence_id, user_id, team_key, kind_code, starts_on, ends_on, action, actor_email, note)
			VALUES (now(), $1, $2::uuid, $3::uuid, $4, $5, $6::date, $7::date, $8, $1, $9)`,
			actorEmail, req.ID, al.UserID, req.TeamKey, code, req.From, req.To, action, note); err != nil {
			return 0, fmt.Errorf("record allocation change: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit allocation change: %w", err)
	}
	return days, nil
}
