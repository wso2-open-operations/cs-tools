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

// deriveProjectOnboarding applies the rule to one project. Run it inside the
// ingest transaction, after the project, link or product line it follows has
// been written. A blank id is ignored.
func deriveProjectOnboarding(ctx context.Context, q querier, projectID string) error {
	if projectID == "" {
		return nil
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
	if _, err := q.Exec(ctx, deriveOnboardingForOpportunityQuery, domain.SalesforceSyncActor, opportunityID); err != nil {
		return fmt.Errorf("derive project onboarding: %w", err)
	}
	return nil
}
