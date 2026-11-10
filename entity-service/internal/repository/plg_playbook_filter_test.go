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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// seedPlaybookAnswers seeds, once, registrations in July 2018 (the
// registration filter keeps every other test's rows out), each answering the
// playbook's outcome task differently:
//
//   - Playbook Org A's IAM registration: Monitor, and two ticks on the
//     reasons checklist; its API Platform registration has no run.
//   - Playbook Org B (API Platform): Excluded, on two playbooks that both
//     ask the outcome task.
//   - Playbook Org C: a run with the outcome not answered yet.
//   - Playbook Org D: no run.
//   - Playbook Org E: Eligible, on a run since detached.
func seedPlaybookAnswers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var seeded bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM plg_organization WHERE organization_name = 'Playbook Org A')`).Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	if seeded {
		return
	}
	july := time.Date(2018, 7, 10, 12, 0, 0, 0, time.UTC)
	for i, o := range []struct{ name, product string }{
		{"Playbook Org A", "IAM"}, {"Playbook Org B", "API_PLATFORM"}, {"Playbook Org C", "IAM"},
		{"Playbook Org D", "IAM"}, {"Playbook Org E", "IAM"},
	} {
		seedRegistration(t, pool, o.name, o.product, july.AddDate(0, 0, i), false)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO plg_org_platform (organization_id, product_id, registered_on)
		SELECT id, (SELECT id FROM plg_product WHERE code = 'API_PLATFORM'), $1
		FROM plg_organization WHERE organization_name = 'Playbook Org A'`, july.AddDate(0, 0, 6)); err != nil {
		t.Fatalf("seed Playbook Org A's API Platform registration: %v", err)
	}
	seedPlaybookRun(t, pool, "Playbook filter test", "Playbook Org A", "IAM", false, "MONITOR", []string{"DISPOSABLE_EMAIL", "NO_ACTIVITY"})
	seedPlaybookRun(t, pool, "Playbook filter test", "Playbook Org B", "API_PLATFORM", false, "EXCLUDED", nil)
	seedPlaybookRun(t, pool, "Playbook filter test, again", "Playbook Org B", "API_PLATFORM", false, "EXCLUDED", nil)
	seedPlaybookRun(t, pool, "Playbook filter test", "Playbook Org C", "IAM", false, "", nil)
	seedPlaybookRun(t, pool, "Playbook filter test", "Playbook Org E", "IAM", true, "ELIGIBLE", nil)
}

// TestPlaybookAnswerFilters filters the registrations seedPlaybookAnswers
// seeds by their answers.
func TestPlaybookAnswerFilters(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	seedPlaybookAnswers(t, pool)

	from, to := time.Date(2018, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2018, 7, 31, 23, 59, 59, 0, time.UTC)
	yes := true
	const outcome, reasons = "PB_TEST_OUTCOME", "PB_TEST_REASONS"
	orgs := NewOrganizationRepository(pool)
	regs := NewOrgPlatformRepository(pool, nil)
	for _, c := range []struct {
		name                 string
		filters              domain.OrganizationSearchFilters
		organizations, count int // count: registrations, and the trend's total
	}{
		{"an answer", domain.OrganizationSearchFilters{PlaybookTask: outcome, PlaybookAnswers: []string{"MONITOR"}}, 1, 1},
		{"any of two answers, trimmed", domain.OrganizationSearchFilters{PlaybookTask: " " + outcome, PlaybookAnswers: []string{"MONITOR", " EXCLUDED "}}, 2, 2},
		{"a detached run's answer", domain.OrganizationSearchFilters{PlaybookTask: outcome, PlaybookAnswers: []string{"ELIGIBLE"}}, 0, 0},
		{"a checklist tick", domain.OrganizationSearchFilters{PlaybookTask: reasons, PlaybookAnswers: []string{"NO_ACTIVITY"}}, 1, 1},
		{"another task's answer", domain.OrganizationSearchFilters{PlaybookTask: reasons, PlaybookAnswers: []string{"MONITOR"}}, 0, 0},
		{"a code as written", domain.OrganizationSearchFilters{PlaybookTask: outcome, PlaybookAnswers: []string{"monitor"}}, 0, 0},
		// Org A's API Platform registration, C, D and E (detached).
		{"unanswered", domain.OrganizationSearchFilters{PlaybookTask: outcome, PlaybookUnanswered: &yes}, 4, 4},
		// An organisation matches when any registration has the product and
		// any has no answer: Org A, through two different registrations.
		{"unanswered, on IAM", domain.OrganizationSearchFilters{PlaybookTask: outcome, PlaybookUnanswered: &yes, ProductCodes: []string{"IAM"}}, 4, 3},
		{"a task on its own", domain.OrganizationSearchFilters{PlaybookTask: outcome}, 5, 6},
	} {
		f := c.filters
		f.RegisteredFrom, f.RegisteredTo = &from, &to
		_, orgTotal, err := orgs.Search(ctx, domain.SearchOrganizationsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_, regTotal, err := regs.SearchRegistrations(ctx, domain.SearchRegistrationsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		days, err := regs.RegistrationVolumeTrend(ctx, domain.SearchRegistrationsRequest{Filters: c.filters}, from, to)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		trendTotal := 0
		for _, n := range days {
			trendTotal += n
		}
		if orgTotal != c.organizations || regTotal != c.count || trendTotal != c.count {
			t.Errorf("%s: organisations %d, registrations %d, trend %d; want %d, %d, %d",
				c.name, orgTotal, regTotal, trendTotal, c.organizations, c.count, c.count)
		}

		// The aggregates count what the searches list.
		byOwner, err := orgs.AggregateByOwner(ctx, f, 0)
		if err != nil {
			t.Fatalf("%s, by owner: %v", c.name, err)
		}
		byCountry, err := regs.AggregateRegistrationsByCountry(ctx, domain.SearchRegistrationsRequest{Filters: f}, 0)
		if err != nil {
			t.Fatalf("%s, by country: %v", c.name, err)
		}
		if byOwner.TotalRecords != c.organizations || byCountry.TotalRecords != c.count {
			t.Errorf("%s: aggregates %d organisations, %d registrations; want %d, %d",
				c.name, byOwner.TotalRecords, byCountry.TotalRecords, c.organizations, c.count)
		}
	}
}

// seedPlaybookRun runs the playbook named playbook, of the test's two tasks,
// on an organisation's registration of productCode: PB_TEST_OUTCOME, a single
// choice answered outcome ("" for not answered), and PB_TEST_REASONS, a
// checklist with ticked ticked. The playbook is the product's own, made on
// first use.
func seedPlaybookRun(t *testing.T, pool *pgxpool.Pool, playbook, org, productCode string, detached bool, outcome string, ticked []string) {
	t.Helper()
	ctx := context.Background()
	var playbookID string
	if err := pool.QueryRow(ctx, `
		SELECT pb.id::TEXT FROM plg_playbook pb JOIN plg_product pr ON pr.id = pb.product_id
		WHERE pr.code = $1 AND pb.name = $2`, productCode, playbook).Scan(&playbookID); err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO plg_playbook (product_id, name, lifecycle_stage, playbook_type)
			VALUES ((SELECT id FROM plg_product WHERE code = $1), $2, 'REGISTRATION', 'PROGRESSIVE')
			RETURNING id::TEXT`, productCode, playbook).Scan(&playbookID); err != nil {
			t.Fatalf("seed a playbook: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO plg_playbook_task (playbook_id, code, name, sequence_no, value_type, options) VALUES
			($1::UUID, 'PB_TEST_OUTCOME', 'Outcome', 1, 'SINGLE_SELECT',
			 '[{"code":"ELIGIBLE","label":"Eligible"},{"code":"MONITOR","label":"Monitor"},{"code":"EXCLUDED","label":"Excluded"}]'),
			($1::UUID, 'PB_TEST_REASONS', 'Reasons', 2, 'CHECKLIST',
			 '[{"code":"DISPOSABLE_EMAIL","label":"Disposable email"},{"code":"NO_ACTIVITY","label":"No activity"}]')`,
			playbookID); err != nil {
			t.Fatalf("seed the playbook's tasks: %v", err)
		}
	}
	var detachedOn any
	if detached {
		detachedOn = time.Now()
	}
	var runID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO plg_playbook_run (org_platform_id, playbook_id, detached_on)
		SELECT op.id, $3::UUID, $4 FROM plg_org_platform op
		JOIN plg_organization o ON o.id = op.organization_id
		JOIN plg_product pr ON pr.id = op.product_id
		WHERE o.organization_name = $1 AND pr.code = $2
		RETURNING id::TEXT`, org, productCode, playbookID, detachedOn).Scan(&runID); err != nil {
		t.Fatalf("seed a run for %s: %v", org, err)
	}
	var text any
	if outcome != "" {
		text = outcome
	}
	var checked any
	if len(ticked) > 0 {
		checked = ticked
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO plg_playbook_run_task (playbook_run_id, playbook_task_id, code, name, sequence_no, value_type, options, value_text, value_checked)
		SELECT $1::UUID, t.id, t.code, t.name, t.sequence_no, t.value_type, t.options,
		       CASE WHEN t.code = 'PB_TEST_OUTCOME' THEN $3::TEXT END,
		       CASE WHEN t.code = 'PB_TEST_REASONS' THEN $4::TEXT[] END
		FROM plg_playbook_task t WHERE t.playbook_id = $2::UUID`, runID, playbookID, text, checked); err != nil {
		t.Fatalf("seed the run's tasks for %s: %v", org, err)
	}
}
