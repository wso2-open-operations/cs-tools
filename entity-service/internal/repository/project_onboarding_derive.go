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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// onboardingProductName is the exact opportunity product name that funds a
// project's onboarding.
const onboardingProductName = "WSO2 Onboarding Services"

// deriveOnboardingBody is the shared UPDATE. The previous system derived a
// project's onboarding window from its linked onboarding product lines, and
// the ingest now does the same:
//
//   - a project qualifies through an sf_opportunity_link to an opportunity
//     holding a line named onboardingProductName whose service end date is
//     today or later (no start-date filter: a line that starts in the future
//     still opens the window);
//   - when several lines qualify, the one with the earliest service start date
//     wins (ties by id), so the outcome is deterministic and does not depend
//     on ingest order;
//   - onboarding_start_date, onboarding_expiry_date and the total hours (a
//     fixed 200, the line's quantity is ignored) always follow the line;
//   - onboarding_status and remaining hours are only set, to NOT_STARTED and
//     200 hours, while the status is NULL or NOT_APPLICABLE. Every other
//     status is a person's or a later process's decision and is never
//     overwritten.
//
// The trailing predicate makes a re-run a no-op once the row is current, so
// replays do not churn updated_on. The caller appends the project scope.
const deriveOnboardingBody = `
	UPDATE project p SET
		onboarding_start_date = l.service_start_date,
		onboarding_expiry_date = l.service_end_date,
		total_onboarding_duration = interval '200 hours',
		onboarding_status = CASE WHEN p.onboarding_status IS NULL OR p.onboarding_status = 'NOT_APPLICABLE'
			THEN 'NOT_STARTED'::onboarding_status_enum ELSE p.onboarding_status END,
		remaining_onboarding_duration = CASE WHEN p.onboarding_status IS NULL OR p.onboarding_status = 'NOT_APPLICABLE'
			THEN interval '200 hours' ELSE p.remaining_onboarding_duration END,
		updated_on = now(),
		updated_by = $1
	FROM (
		SELECT DISTINCT ON (k.project_id) k.project_id, op.service_start_date, op.service_end_date
		FROM sf_opportunity_link k
		JOIN sf_opportunity_product op ON op.opportunity_id = k.opportunity_id
		WHERE op.product_name = '` + onboardingProductName + `'
		  AND op.service_end_date >= CURRENT_DATE
		  AND k.project_id IS NOT NULL
		ORDER BY k.project_id, op.service_start_date ASC NULLS LAST, op.id
	) l
	WHERE p.id = l.project_id
	  AND (p.onboarding_start_date IS DISTINCT FROM l.service_start_date
	    OR p.onboarding_expiry_date IS DISTINCT FROM l.service_end_date
	    OR p.total_onboarding_duration IS DISTINCT FROM interval '200 hours'
	    OR p.onboarding_status IS NULL OR p.onboarding_status = 'NOT_APPLICABLE')`

// deriveOnboardingForProjectQuery scopes the rule to one project ($2).
const deriveOnboardingForProjectQuery = deriveOnboardingBody + `
	  AND p.id = $2::uuid`

// deriveOnboardingForOpportunityQuery scopes the rule to every project linked
// to one opportunity ($2).
const deriveOnboardingForOpportunityQuery = deriveOnboardingBody + `
	  AND p.id IN (SELECT project_id FROM sf_opportunity_link WHERE opportunity_id = $2::uuid)`

// onboardingDeriveLockKey is the ONE advisory-lock key every derivation takes.
// The writers that feed the rule (project, link, opportunity, line item) each
// serialise on their own key, so a line item and its link can commit in
// concurrent transactions. Under READ COMMITTED each derive statement then
// sees only committed rows, and both transactions could derive without the
// other's row, leaving the project NOT_APPLICABLE. Taking this one lock after
// the caller's own write and immediately before the derive statement orders
// the two: the later transaction blocks until the earlier one commits, and
// its derive statement, which starts after the lock is granted, takes a fresh
// snapshot that includes the earlier row. A single global key is acceptable
// because the lock is held only to the end of an ingest transaction that is
// otherwise a few indexed writes. The project writer takes it before its
// own project UPDATE (lockOnboardingDerive) so it never holds a project row
// lock while waiting for a derivation that wants that same row.
const onboardingDeriveLockKey = "onboarding-derive"

func lockOnboardingDerive(ctx context.Context, q querier) error {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, onboardingDeriveLockKey); err != nil {
		return fmt.Errorf("derive project onboarding: lock: %w", err)
	}
	return nil
}

// previousLinkProjectsQuery and previousLineItemOpportunitiesQuery read, before
// a write, the owners the row has now, so a move can re-derive the old owner.
const previousLinkProjectsQuery = `
	SELECT COALESCE(array_agg(DISTINCT project_id::text) FILTER (WHERE project_id IS NOT NULL), '{}'::text[])
	FROM sf_opportunity_link WHERE link_sf_id = $1`

// previousLinkOpportunitiesQuery reads the opportunities a link joins now.
const previousLinkOpportunitiesQuery = `
	SELECT COALESCE(array_agg(DISTINCT opportunity_id::text) FILTER (WHERE opportunity_id IS NOT NULL), '{}'::text[])
	FROM sf_opportunity_link WHERE link_sf_id = $1`

// previousOpportunityProjectsQuery reads, by Salesforce id, the projects linked
// to an opportunity before it is deleted.
const previousOpportunityProjectsQuery = `
	SELECT COALESCE(array_agg(DISTINCT l.project_id::text) FILTER (WHERE l.project_id IS NOT NULL), '{}'::text[])
	FROM sf_opportunity o JOIN sf_opportunity_link l ON l.opportunity_id = o.id WHERE o.sf_id = $1`

// without returns ids minus the one given.
func without(ids []string, drop string) []string {
	var out []string
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}

const previousLineItemOpportunitiesQuery = `
	SELECT COALESCE(array_agg(DISTINCT opportunity_id::text) FILTER (WHERE opportunity_id IS NOT NULL), '{}'::text[])
	FROM sf_opportunity_product WHERE line_item_sf_id = $1`

func previousIDs(ctx context.Context, q querier, sql, sfID string) ([]string, error) {
	var ids []string
	if err := q.QueryRow(ctx, sql, sfID).Scan(&ids); err != nil {
		return nil, fmt.Errorf("read previous owners: %w", err)
	}
	return ids, nil
}

// deriveProjectOnboarding applies the rule to one project. Run it inside the
// ingest transaction, after the project, link or product line it follows has
// been written. A blank id is ignored.
func deriveProjectOnboarding(ctx context.Context, q querier, projectID string) error {
	if projectID == "" {
		return nil
	}
	if err := lockOnboardingDerive(ctx, q); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, deriveOnboardingForProjectQuery, domain.SalesforceSyncActor, projectID); err != nil {
		return fmt.Errorf("derive project onboarding: %w", err)
	}
	return nil
}

// deriveOpportunityOnboarding applies the rule to every project linked to
// opportunityID, for a product line change. A blank id is ignored.
func deriveOpportunityOnboarding(ctx context.Context, q querier, opportunityID string) error {
	if opportunityID == "" {
		return nil
	}
	if err := lockOnboardingDerive(ctx, q); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, deriveOnboardingForOpportunityQuery, domain.SalesforceSyncActor, opportunityID); err != nil {
		return fmt.Errorf("derive project onboarding: %w", err)
	}
	return nil
}

// deriveMovedLinkProjects re-derives the projects a link belonged to before a
// write that moved it to newProjectID, so the old owner picks its remaining
// earliest qualifying line.
func deriveMovedLinkProjects(ctx context.Context, q querier, previous []string, newProjectID string) error {
	for _, id := range previous {
		if id == newProjectID {
			continue
		}
		if err := deriveProjectOnboarding(ctx, q, id); err != nil {
			return err
		}
	}
	return nil
}

// deriveMovedLineItemOpportunities is the same for a line item that moved
// from one opportunity to another.
func deriveMovedLineItemOpportunities(ctx context.Context, q querier, previous []string, newOpportunityID string) error {
	for _, id := range previous {
		if id == newOpportunityID {
			continue
		}
		if err := deriveOpportunityOnboarding(ctx, q, id); err != nil {
			return err
		}
	}
	return nil
}
