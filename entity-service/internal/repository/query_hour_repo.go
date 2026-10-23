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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Query-hour utilisation, read from the tables that already hold it.
//
// No migration accompanies this. The entitlement lives in the Salesforce
// mirror (sf_opportunity, sf_opportunity_link, sf_opportunity_product),
// the consumption in time_card, the owners on account, and the state in
// sf_opportunity.query_hour_state -- an enum whose three values are
// already exactly ServiceNow's 1/2/3.

// QueryHourLineRow is one active product line on one opportunity.
type QueryHourLineRow struct {
	OpportunityID       string
	SalesforceID        string
	Name                string
	AccountID           string
	AccountName         string
	AccountSFID         string
	AccountManagerEmail string
	TechnicalOwnerEmail string
	AccountManagerName  string
	ProductName         string
	Quantity            float64
	ServiceStart        time.Time
	ServiceEnd          time.Time
}

// QueryHourProjectRow links a project to an opportunity, with the
// consumption already aggregated over that opportunity's service window.
type QueryHourProjectRow struct {
	OpportunityID      string
	ProjectID          string
	ProjectKey         string
	ProjectName        string
	ProjectSFID        string
	BillableMinutes    int
	NonBillableMinutes int
	OnboardingMinutes  int
}

// QueryHourRepository reads what the utilisation calculation needs.
type QueryHourRepository interface {
	// ActiveLines returns every product line currently in service, with
	// its opportunity and that opportunity's account.
	ActiveLines(ctx context.Context) ([]QueryHourLineRow, error)
	// ProjectConsumption returns each opportunity's linked projects with
	// their approved time summed over [from, to].
	ProjectConsumption(ctx context.Context, opportunityID string, from, to time.Time) ([]QueryHourProjectRow, error)
	// OpportunityStates returns the state currently stored against each
	// opportunity, so a recompute can tell a transition from a repeat.
	OpportunityStates(ctx context.Context) (map[string]*domain.QueryHourState, error)
	// SetOpportunityState writes one opportunity's state, or clears it.
	SetOpportunityState(ctx context.Context, opportunityID string, state *domain.QueryHourState) error
	// SetProjectDurations writes the three figures the notification
	// prints back onto the project row.
	SetProjectDurations(ctx context.Context, projectID string, totalMinutes, consumedMinutes, remainingMinutes int) error
}

type queryHourRepo struct {
	db *Scoped
}

// NewQueryHourRepository constructs a QueryHourRepository. Takes a
// *Scoped because time_card and work_item carry row-level security and
// this runs as the system identity.
func NewQueryHourRepository(db *Scoped) QueryHourRepository {
	return &queryHourRepo{db: db}
}

// A line is in service when today falls inside its dates, which is
// ServiceNow's
// `u_service_end_date>=beginningOfToday^u_service_start_date<=endOfToday`.
// The product filter is applied in Go rather than here, so a line whose
// SKU has been renamed is COUNTED as unmatched instead of vanishing --
// a renamed product would otherwise silently reduce an entitlement to
// zero with nothing to notice.
const queryHourActiveLinesSQL = `
SELECT o.id::text,
       COALESCE(o.sf_id, ''),
       COALESCE(o.name, ''),
       COALESCE(a.id::text, ''),
       COALESCE(a.name, ''),
       COALESCE(a.sf_id, ''),
       COALESCE(am.email, ''),
       COALESCE(te.email, ''),
       TRIM(COALESCE(am.name, CONCAT_WS(' ', am.first_name, am.last_name))),
       COALESCE(pr.product_name, ''),
       COALESCE(pr.quantity, 0),
       pr.service_start_date,
       pr.service_end_date
  FROM sf_opportunity_product pr
  JOIN sf_opportunity o ON o.id = pr.opportunity_id
  LEFT JOIN account a ON a.id = o.account_id
  LEFT JOIN "user" am ON am.id = a.account_manager_id
  LEFT JOIN "user" te ON te.id = a.technical_owner_id
 WHERE pr.service_start_date <= CURRENT_DATE
   AND pr.service_end_date   >= CURRENT_DATE
 ORDER BY o.id, pr.product_name`

func (r *queryHourRepo) ActiveLines(ctx context.Context) ([]QueryHourLineRow, error) {
	rows, err := r.db.Query(ctx, queryHourActiveLinesSQL)
	if err != nil {
		return nil, fmt.Errorf("query active query-hour lines: %w", err)
	}
	defer rows.Close()

	var out []QueryHourLineRow
	for rows.Next() {
		var x QueryHourLineRow
		if err := rows.Scan(&x.OpportunityID, &x.SalesforceID, &x.Name,
			&x.AccountID, &x.AccountName, &x.AccountSFID,
			&x.AccountManagerEmail, &x.TechnicalOwnerEmail, &x.AccountManagerName,
			&x.ProductName, &x.Quantity, &x.ServiceStart, &x.ServiceEnd); err != nil {
			return nil, fmt.Errorf("scan active query-hour line: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active query-hour lines: %w", err)
	}
	return out, nil
}

// Consumption is BILLABLE approved time only. ServiceNow sums
// `total_billable` into its running total and computes the non-billable
// figure purely to display, so a non-billable hour draws nothing down.
//
// The window is the opportunity's own service period, not a rolling one:
// an allowance is bounded by the contract that granted it.
const queryHourProjectConsumptionSQL = `
SELECT p.id::text,
       COALESCE(p.key, ''),
       COALESCE(p.name, ''),
       COALESCE(p.sf_id, ''),
       COALESCE(SUM(
           CASE WHEN tc.is_billable IS TRUE THEN
               COALESCE(tc.analyzing_minutes, 0) + COALESCE(tc.setting_up_minutes, 0)
             + COALESCE(tc.reproducing_debugging_minutes, 0)
             + COALESCE(tc.providing_solution_minutes, 0)
             + COALESCE(tc.patching_minutes, 0)
           ELSE 0 END), 0)::int,
       COALESCE(SUM(
           CASE WHEN tc.is_billable IS NOT TRUE THEN
               COALESCE(tc.analyzing_minutes, 0) + COALESCE(tc.setting_up_minutes, 0)
             + COALESCE(tc.reproducing_debugging_minutes, 0)
             + COALESCE(tc.providing_solution_minutes, 0)
             + COALESCE(tc.patching_minutes, 0)
           ELSE 0 END), 0)::int,
       CASE
           WHEN COALESCE(p.onboarding_status::text, 'NOT_APPLICABLE') = 'NOT_APPLICABLE'
               THEN 0
           ELSE LEAST(
               COALESCE(FLOOR(EXTRACT(EPOCH FROM p.consumed_onboarding_duration) / 60), 0),
               COALESCE(FLOOR(EXTRACT(EPOCH FROM p.total_onboarding_duration) / 60), 0)
           )
       END::int
  FROM sf_opportunity_link l
  JOIN project p ON p.id = l.project_id
  LEFT JOIN time_card tc
         ON tc.customer_project_id = p.id
        AND tc.state = 'APPROVED'
        AND tc.work_date >= $2
        AND tc.work_date <= $3
 WHERE l.opportunity_id = $1
 GROUP BY p.id, p.key, p.name, p.sf_id,
          p.onboarding_status, p.consumed_onboarding_duration, p.total_onboarding_duration
 ORDER BY p.key`

func (r *queryHourRepo) ProjectConsumption(ctx context.Context, opportunityID string, from, to time.Time) ([]QueryHourProjectRow, error) {
	rows, err := r.db.Query(ctx, queryHourProjectConsumptionSQL, opportunityID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query query-hour consumption: %w", err)
	}
	defer rows.Close()

	var out []QueryHourProjectRow
	for rows.Next() {
		x := QueryHourProjectRow{OpportunityID: opportunityID}
		if err := rows.Scan(&x.ProjectID, &x.ProjectKey, &x.ProjectName, &x.ProjectSFID,
			&x.BillableMinutes, &x.NonBillableMinutes, &x.OnboardingMinutes); err != nil {
			return nil, fmt.Errorf("scan query-hour consumption: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate query-hour consumption: %w", err)
	}
	return out, nil
}

func (r *queryHourRepo) OpportunityStates(ctx context.Context) (map[string]*domain.QueryHourState, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, query_hour_state::text FROM sf_opportunity`)
	if err != nil {
		return nil, fmt.Errorf("query opportunity states: %w", err)
	}
	defer rows.Close()

	out := map[string]*domain.QueryHourState{}
	for rows.Next() {
		var id string
		var state *string
		if err := rows.Scan(&id, &state); err != nil {
			return nil, fmt.Errorf("scan opportunity state: %w", err)
		}
		if state == nil {
			out[id] = nil
			continue
		}
		s := domain.QueryHourState(*state)
		out[id] = &s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate opportunity states: %w", err)
	}
	return out, nil
}

func (r *queryHourRepo) SetOpportunityState(ctx context.Context, opportunityID string, state *domain.QueryHourState) error {
	var v *string
	if state != nil {
		s := string(*state)
		v = &s
	}
	if _, err := r.db.Exec(ctx,
		`UPDATE sf_opportunity
		    SET query_hour_state = $2::query_hour_state_enum
		  WHERE id = $1`, opportunityID, v); err != nil {
		return fmt.Errorf("set opportunity %s query-hour state: %w", opportunityID, err)
	}
	return nil
}

// Minutes are stored back as INTERVAL, which is what the three columns
// already are -- they hold ServiceNow's mirrored figures today.
func (r *queryHourRepo) SetProjectDurations(ctx context.Context, projectID string, totalMinutes, consumedMinutes, remainingMinutes int) error {
	if _, err := r.db.Exec(ctx,
		`UPDATE project
		    SET total_query_duration     = make_interval(mins => $2),
		        consumed_duration        = make_interval(mins => $3),
		        remaining_query_duration = make_interval(mins => $4)
		  WHERE id = $1`,
		projectID, totalMinutes, consumedMinutes, remainingMinutes); err != nil {
		return fmt.Errorf("set project %s query durations: %w", projectID, err)
	}
	return nil
}
