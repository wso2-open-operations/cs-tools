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

package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These run against a real Postgres with every migration applied. Skipped
// without CASE_STATS_TEST_DSN, so an ordinary `go test ./...` stays hermetic.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseStatsIntegration
//
// Every aggregation here COALESCEs state across five extension tables and
// casts into real enum types, so the queries can only be verified against the
// actual schema -- a fake pool would prove nothing about whether they parse.
func caseStatsPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CASE_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_STATS_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

const caseStatsProjectID = "11111111-1111-1111-1111-111111111111"

// seedCaseStats builds a small fixture: four case-like work items in one
// project (two cases, one engagement, one service request) plus one completed
// RESPONSE SLA. Everything is removed afterwards, so the test is re-runnable.
func seedCaseStats(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		// work_item children cascade; sla/sla_policy do not reference it by
		// a cascading FK in both directions, so drop them explicitly first.
		_, _ = pool.Exec(ctx, `DELETE FROM sla WHERE created_by = 'case-stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM sla_policy WHERE created_by = 'case-stats-test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE created_by IN ('owner@wso2.com', 'other@wso2.com')`)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, caseStatsProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'CASESTATS', 'sf-casestats')`, caseStatsProjectID)

	// id, type, created_by, created_on offset, state, severity
	items := []struct {
		id        string
		wiType    string
		createdBy string
		createdAt string
		state     string
		severity  string
		closedOn  string
	}{
		{"22222222-0000-0000-0000-000000000001", "CASE", "owner@wso2.com", "now()", "OPEN", "S1", "NULL"},
		{"22222222-0000-0000-0000-000000000002", "CASE", "owner@wso2.com", "now() - INTERVAL '5 days'", "CLOSED", "S2", "now() - INTERVAL '2 days'"},
		{"22222222-0000-0000-0000-000000000003", "ENGAGEMENT", "other@wso2.com", "now() - INTERVAL '45 days'", "CLOSED", "", "now() - INTERVAL '40 days'"},
		{"22222222-0000-0000-0000-000000000004", "SERVICE_REQUEST", "owner@wso2.com", "now()", "AWAITING_INFO", "", "NULL"},
	}

	for i, it := range items {
		// wso2_id is required by the work_item_wso2_id_required_by_type CHECK
		// for every case-like type (migration 000016).
		suffix := string(rune('1' + i))
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		          VALUES ($1, `+it.createdAt+`, now(), $2, $2, $3, $4, 'seeded', $5::work_item_type_enum, $6)`,
			it.id, it.createdBy, "CS999000"+suffix, "CASESTATS-"+suffix, it.wiType, caseStatsProjectID)

		switch it.wiType {
		case "CASE":
			mustExec(`INSERT INTO "case" (id, state, severity, closed_on)
			          VALUES ($1, $2::case_state_enum, $3::case_severity_enum, `+it.closedOn+`)`,
				it.id, it.state, it.severity)
		case "ENGAGEMENT":
			mustExec(`INSERT INTO engagement (id, state, type, closed_on)
			          VALUES ($1, $2::engagement_state_enum, 'MIGRATION'::engagement_type_enum, `+it.closedOn+`)`,
				it.id, it.state)
		case "SERVICE_REQUEST":
			mustExec(`INSERT INTO service_request (id, state) VALUES ($1, $2::service_request_state_enum)`,
				it.id, it.state)
		}
	}

	policyID := "33333333-0000-0000-0000-000000000001"
	mustExec(`INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, target)
	          VALUES ($1, now(), now(), 'case-stats-test', 'case-stats-test', 'Response', 'RESPONSE'::sla_policy_target_enum)`, policyID)
	mustExec(`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, stage, business_duration)
	          VALUES ($1, now(), now(), 'case-stats-test', 'case-stats-test', $2, $3, 'COMPLETED'::sla_stage_enum, INTERVAL '2 hours')`,
		"44444444-0000-0000-0000-000000000001", items[0].id, policyID)
}

func TestCaseStatsIntegration_Aggregations(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseStats(t, pool)

	ctx := context.Background()
	repo := repository.NewProjectCaseStatsRepository(pool)
	all := repository.ProjectCaseStatsFilter{ProjectID: caseStatsProjectID}

	t.Run("StateSeverityCounts", func(t *testing.T) {
		rows, err := repo.StateSeverityCounts(ctx, all)
		if err != nil {
			t.Fatalf("StateSeverityCounts: %v", err)
		}
		total, withSeverity := 0, 0
		for _, r := range rows {
			total += r.Count
			if r.Severity != "" {
				withSeverity += r.Count
			}
		}
		if total != 4 {
			t.Errorf("total across groups = %d, want 4", total)
		}
		// Only the two CASE rows carry a severity; engagement and
		// service_request have no severity column at all.
		if withSeverity != 2 {
			t.Errorf("rows with a severity = %d, want 2", withSeverity)
		}
	})

	t.Run("TypeFilterNarrows", func(t *testing.T) {
		rows, err := repo.StateSeverityCounts(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: caseStatsProjectID, Types: []string{"case"},
		})
		if err != nil {
			t.Fatalf("StateSeverityCounts(types=case): %v", err)
		}
		total := 0
		for _, r := range rows {
			total += r.Count
		}
		if total != 2 {
			t.Errorf("total with types=[case] = %d, want 2", total)
		}
	})

	t.Run("CreatedByFilterIsCaseInsensitive", func(t *testing.T) {
		rows, err := repo.StateSeverityCounts(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: caseStatsProjectID, CreatedBy: "OWNER@WSO2.COM",
		})
		if err != nil {
			t.Fatalf("StateSeverityCounts(createdBy): %v", err)
		}
		total := 0
		for _, r := range rows {
			total += r.Count
		}
		if total != 3 {
			t.Errorf("total for owner@wso2.com = %d, want 3", total)
		}
	})

	t.Run("StateEngagementTypeCounts", func(t *testing.T) {
		rows, err := repo.StateEngagementTypeCounts(ctx, all)
		if err != nil {
			t.Fatalf("StateEngagementTypeCounts: %v", err)
		}
		if len(rows) != 1 || rows[0].EngagementType != "MIGRATION" || rows[0].Count != 1 {
			t.Errorf("rows = %+v, want one MIGRATION row with count 1", rows)
		}
	})

	// ServiceNow's engagement-type aggregate (engAgg) never applies the
	// createdBy filter its sibling aggregations do, so neither does this --
	// the omission lives in the SQL, not in the caller.
	t.Run("EngagementCountsIgnoreCreatedBy", func(t *testing.T) {
		rows, err := repo.StateEngagementTypeCounts(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: caseStatsProjectID, CreatedBy: "nobody@example.com",
		})
		if err != nil {
			t.Fatalf("StateEngagementTypeCounts(createdBy): %v", err)
		}
		// The seeded engagement was created by other@wso2.com, so a filter
		// would exclude it. It must still be counted.
		if len(rows) != 1 || rows[0].Count != 1 {
			t.Errorf("rows = %+v, want the engagement still counted despite an unmatched createdBy", rows)
		}
	})

	t.Run("ResolvedBuckets", func(t *testing.T) {
		_, pastThirty, err := repo.ResolvedBuckets(ctx, all, []string{"CLOSED", "SOLUTION_PROPOSED"})
		if err != nil {
			t.Fatalf("ResolvedBuckets: %v", err)
		}
		// One case closed 2 days ago; the engagement closed 40 days ago is out.
		if pastThirty != 1 {
			t.Errorf("pastThirtyDays = %d, want 1", pastThirty)
		}
	})

	t.Run("ClosedByCreatedWindow", func(t *testing.T) {
		current, previous, err := repo.ClosedByCreatedWindow(ctx, all, "CLOSED")
		if err != nil {
			t.Fatalf("ClosedByCreatedWindow: %v", err)
		}
		// Closed case created 5 days ago -> current; closed engagement created
		// 45 days ago -> previous window.
		if current != 1 || previous != 1 {
			t.Errorf("current/previous = %d/%d, want 1/1", current, previous)
		}
	})

	t.Run("AverageResponseSeconds", func(t *testing.T) {
		avg, count, err := repo.AverageResponseSeconds(ctx, caseStatsProjectID)
		if err != nil {
			t.Fatalf("AverageResponseSeconds: %v", err)
		}
		if count != 1 || avg != 7200 {
			t.Errorf("avg/count = %v/%d, want 7200/1", avg, count)
		}
	})

	t.Run("CaseTypeCounts", func(t *testing.T) {
		counts, err := repo.CaseTypeCounts(ctx, all)
		if err != nil {
			t.Fatalf("CaseTypeCounts: %v", err)
		}
		if counts["case"] != 2 || counts["engagement"] != 1 || counts["service_request"] != 1 {
			t.Errorf("counts = %v, want case=2 engagement=1 service_request=1", counts)
		}
	})

	t.Run("CaseTypeCountsIgnoresTypeFilter", func(t *testing.T) {
		// Mirrors ServiceNow: the case-type aggregate never applies the
		// requested caseTypes filter, so the breakdown always covers them all.
		counts, err := repo.CaseTypeCounts(ctx, repository.ProjectCaseStatsFilter{
			ProjectID: caseStatsProjectID, Types: []string{"case"},
		})
		if err != nil {
			t.Fatalf("CaseTypeCounts(types=case): %v", err)
		}
		if counts["engagement"] != 1 {
			t.Errorf("engagement = %d, want 1 -- the type filter must not narrow this aggregate", counts["engagement"])
		}
	})
}
