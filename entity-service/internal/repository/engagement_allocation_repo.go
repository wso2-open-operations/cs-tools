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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// EngagementAllocationRepository reads the customer-engagement allocation
// tables — see domain.StatusUpdateReminderRecipient's doc comment for what
// they are and why this service reads them.
type EngagementAllocationRepository interface {
	// StatusUpdateReminderRecipients returns everyone who has a live
	// allocation covering cycleStart and has not published their own status
	// update for that cycle. cycleStart is assumed already validated by the
	// service layer.
	StatusUpdateReminderRecipients(ctx context.Context, cycleStart time.Time) ([]domain.StatusUpdateReminderRecipient, error)
}

type engagementAllocationRepo struct {
	db *pgxpool.Pool
}

// NewEngagementAllocationRepository constructs an EngagementAllocationRepository
// over the given pool.
func NewEngagementAllocationRepository(db *pgxpool.Pool) EngagementAllocationRepository {
	return &engagementAllocationRepo{db: db}
}

// statusUpdateReminderQuery is the whole weekly-reminder rule in one statement.
//
// It is the Go port of ServiceNow's WeeklyAllocationStatusUpdateReminderEmailFlow,
// and it deliberately differs from that flow in three places. Each difference
// is a defect the port fixes, each was confirmed against the sys_dictionary
// dump of 2026-09-21, and each is reproducible offline against
// csm-sync-service's testdata/0079_customer_engagement_seed.sql:
//
//  1. AUTHOR, NOT ENGAGEMENT. The original's per-author filter is applied to
//     the wrong GlideRecord — and after that record's query() has already run,
//     so it does nothing at all. The live flow therefore asks "has ANYONE on
//     this engagement posted?", which means one person's update silences the
//     reminder for every colleague on it. The NOT EXISTS below matches on
//     author_id, which is what the original's own commented-out line intended.
//
//  2. PUBLISHED UPDATES ONLY. The original counts any status_update row
//     regardless of state, so starting a draft and abandoning it suppresses
//     the reminder exactly as well as publishing one does.
//
//  3. LIVE ALLOCATIONS ONLY. The original never looks at the allocation's own
//     state. Three of its nine values are cancellations or rejections, so a
//     consultant who rejected an allocation still gets chased for updates on
//     it.
//
// What is NOT changed: the engagement filter stays IN_PROGRESS only, matching
// the original's raw u_state == "1". NEW, REQUESTED and ON_HOLD engagements
// are still skipped — widening that is a product decision, not a defect fix.
//
// The one bound added beyond those three: the original tests
// cycle_start_date >= lastMonday with no upper bound, so an update filed for a
// LATER cycle silences a reminder about an earlier one. The half-open week
// window below keeps the original's tolerance for an update dated mid-cycle
// while refusing to let next week's update answer for last week's.
//
// a.state IS NULL passes deliberately: the column is nullable, and an
// allocation with no recorded state is not evidence of cancellation.
const statusUpdateReminderQuery = `
	SELECT DISTINCT u.id, u.email, u.name
	FROM customer_engagement_allocation_resource a
	JOIN customer_engagement e ON e.id = a.engagement_id
	JOIN "user" u              ON u.id = a.resource_id
	WHERE e.state = 'IN_PROGRESS'
	  AND a.start_date <= $1
	  AND a.end_date   >= $1
	  AND (
	        a.state IS NULL
	     OR a.state NOT IN ('ALLOCATION_CANCELLED', 'REJECTED_BY_CONSULTANT', 'REJECTED_OTHER')
	      )
	  AND u.email IS NOT NULL
	  AND u.email <> ''
	  AND NOT EXISTS (
	        SELECT 1
	        FROM customer_engagement_status_update s
	        WHERE s.engagement_id      = a.engagement_id
	          AND s.author_id          = a.resource_id
	          AND s.state              = 'PUBLISHED'
	          AND s.cycle_start_date  >= $1
	          AND s.cycle_start_date   < $1::date + 7
	      )
	ORDER BY u.email`

func (r *engagementAllocationRepo) StatusUpdateReminderRecipients(ctx context.Context, cycleStart time.Time) ([]domain.StatusUpdateReminderRecipient, error) {
	rows, err := r.db.Query(ctx, statusUpdateReminderQuery, cycleStart)
	if err != nil {
		return nil, fmt.Errorf("status update reminder recipients: %w", err)
	}
	defer rows.Close()

	recipients := []domain.StatusUpdateReminderRecipient{}
	for rows.Next() {
		var rec domain.StatusUpdateReminderRecipient
		// name is nullable upstream; email cannot be, because the query
		// already excludes rows without one.
		var name *string
		if err := rows.Scan(&rec.UserID, &rec.Email, &name); err != nil {
			return nil, fmt.Errorf("status update reminder recipients: scan: %w", err)
		}
		rec.Name = name
		recipients = append(recipients, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("status update reminder recipients: rows: %w", err)
	}
	return recipients, nil
}
