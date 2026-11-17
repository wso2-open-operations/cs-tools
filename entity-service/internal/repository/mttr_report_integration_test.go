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

// Integration test for the MTTR report's SeverityStats query. Runs against a
// real, migrated Postgres, locally: no CI workflow runs the entity-service
// Go tests. Skipped without ENTITY_TEST_DATABASE_URL.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestMTTRReportIntegration ./internal/repository/

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	mttrAccountID        = "6d300000-0000-4000-8000-000000000001"
	mttrTeamAccountID    = "6d300000-0000-4000-8000-000000000002"
	mttrTeamGroupID      = "6d300000-0000-4000-8000-000000000010"
	mttrOtherTeamGroupID = "6d300000-0000-4000-8000-000000000011"
)

func mttrCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM work_item_activity WHERE work_item_id::text LIKE '6d3%'`,
		`DELETE FROM "case" WHERE id::text LIKE '6d3%'`,
		`DELETE FROM work_item WHERE id::text LIKE '6d3%'`,
		`DELETE FROM account WHERE id::text LIKE '6d3%'`,
		`DELETE FROM "group" WHERE id::text LIKE '6d3%'`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
}

func TestMTTRReportIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	mttrCleanup(t, pool)
	t.Cleanup(func() { mttrCleanup(t, pool) })
	ctx := repository.WithSystemIdentity(context.Background())
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}

	now := time.Now().UTC()
	ago := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }

	exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active)
		VALUES ($1, now(), now(), 't', 't', 'MTTR Team', true), ($2, now(), now(), 't', 't', 'MTTR Other Team', true)`,
		mttrTeamGroupID, mttrOtherTeamGroupID)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, cre_team_id) VALUES
		($1, now(), now(), 't', 't', 'MTTR Account', 'ACC-MTTR-1', $3),
		($2, now(), now(), 't', 't', 'MTTR Team Account', 'ACC-MTTR-2', $4)`,
		mttrAccountID, mttrTeamAccountID, mttrOtherTeamGroupID, mttrTeamGroupID)

	seq := 0
	// caseRow seeds a work item plus its case row under accountID, and
	// returns its id.
	caseRow := func(accountID string, createdOn time.Time, severity string) string {
		seq++
		id := fmt.Sprintf("6d300000-0000-4000-8000-0000%08d", seq)
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id)
			VALUES ($1, $2, $2, 't', 't', $3, $4, 'mttr integration test fixture', 'CASE', $5)`,
			id, createdOn, fmt.Sprintf("CASEMTTR%02d", seq), fmt.Sprintf("MTTR-WSO2-%02d", seq), accountID)
		exec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'CLOSED', $2::case_severity_enum)`, id, severity)
		return id
	}
	// setState moves a seeded case to its current state (default CLOSED).
	setState := func(id, state string) {
		exec(`UPDATE "case" SET state = $2::case_state_enum WHERE id = $1`, id, state)
	}
	entered := func(workItemID, newValue string, at time.Time) {
		exec(`INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value)
			VALUES (gen_random_uuid(), $1, 't', $2, 'state', 'Work In Progress', $3)`, at, workItemID, newValue)
	}

	// S1, 10 days to resolve, uppercase enum label, no team.
	s1a := caseRow(mttrAccountID, ago(40), "S1")
	entered(s1a, "SOLUTION_PROPOSED", ago(30))
	// S1, 5 days to resolve, title-case label, on the team-filtered account.
	s1b := caseRow(mttrTeamAccountID, ago(15), "S1")
	entered(s1b, "Solution Proposed", ago(10))
	// S2, 5 days to resolve, no team.
	s2a := caseRow(mttrAccountID, ago(25), "S2")
	entered(s2a, "solution_proposed", ago(20)) // lower case too: still matched
	// S3, bounced in and out (a rejected proposal, then a later one that
	// stuck): the LAST entry (50 days after creation) is what must be
	// counted, not the earlier, rejected one.
	s3Reenter := caseRow(mttrAccountID, ago(55), "S3")
	entered(s3Reenter, "SOLUTION_PROPOSED", ago(50))
	entered(s3Reenter, "SOLUTION_PROPOSED", ago(5))
	// S4, entered well outside the window: must not be counted at all.
	s4Outside := caseRow(mttrAccountID, ago(210), "S4")
	entered(s4Outside, "SOLUTION_PROPOSED", ago(200))
	// S1, never entered Solution Proposed: only a Closed entry.
	s1NeverProposed := caseRow(mttrAccountID, ago(10), "S1")
	entered(s1NeverProposed, "CLOSED", ago(5))
	// S0 (catastrophic), inside the window: reported as its own severity.
	s0 := caseRow(mttrAccountID, ago(15), "S0")
	entered(s0, "SOLUTION_PROPOSED", ago(10))
	// S2 whose proposal was rejected: it went back to Work In Progress and
	// has not been proposed again, so it is not resolved now and must not be
	// counted although its Solution Proposed entry is inside the window.
	s2Rejected := caseRow(mttrAccountID, ago(20), "S2")
	entered(s2Rejected, "SOLUTION_PROPOSED", ago(8))
	setState(s2Rejected, "WORK_IN_PROGRESS")
	// S2 currently in Solution Proposed (not closed yet): counted.
	s2Proposed := caseRow(mttrAccountID, ago(20), "S2")
	entered(s2Proposed, "SOLUTION_PROPOSED", ago(15))
	setState(s2Proposed, "SOLUTION_PROPOSED")
	// S2 with no activity at all: must not be counted.
	caseRow(mttrAccountID, ago(10), "S2")

	repo := repository.NewMTTRReportRepository(repository.NewScoped(pool))
	windowStart, windowEnd := now.Add(-60*24*time.Hour), now

	statsBySeverity := func(stats []domain.MTTRSeverityStat) map[string]domain.MTTRSeverityStat {
		m := make(map[string]domain.MTTRSeverityStat, len(stats))
		for _, s := range stats {
			m[s.Severity] = s
		}
		return m
	}

	stats, err := repo.SeverityStats(ctx, windowStart, windowEnd, nil)
	if err != nil {
		t.Fatalf("SeverityStats: %v", err)
	}
	bySev := statsBySeverity(stats)

	if got := bySev["S1"]; got.CaseCount != 2 || !closeTo(got.TotalResolutionSec, 10*86400+5*86400) {
		t.Errorf("S1 = %+v, want count 2, total ~%d seconds", got, 10*86400+5*86400)
	}
	// Closed (5 days), currently proposed (5 days); the rejected one, back in
	// Work In Progress, is left out.
	if got := bySev["S2"]; got.CaseCount != 2 || !closeTo(got.TotalResolutionSec, 10*86400) {
		t.Errorf("S2 = %+v, want count 2, total ~%d seconds", got, 10*86400)
	}
	if got := bySev["S3"]; got.CaseCount != 1 || !closeTo(got.TotalResolutionSec, 50*86400) {
		t.Errorf("S3 (bounced in and out) = %+v, want count 1 (the LAST entry only), total ~%d seconds", got, 50*86400)
	}
	if _, ok := bySev["S4"]; ok {
		t.Errorf("S4 entered outside the window, want it absent from the result, got %+v", bySev["S4"])
	}
	if got := bySev["S0"]; got.CaseCount != 1 || !closeTo(got.TotalResolutionSec, 5*86400) {
		t.Errorf("S0 = %+v, want count 1, total ~%d seconds", got, 5*86400)
	}

	// Team filter: only the team-account's S1 case counts.
	teamStats, err := repo.SeverityStats(ctx, windowStart, windowEnd, []string{mttrTeamGroupID})
	if err != nil {
		t.Fatalf("SeverityStats (team filter): %v", err)
	}
	teamBySev := statsBySeverity(teamStats)
	if len(teamBySev) != 1 {
		t.Fatalf("team-filtered severities = %v, want exactly S1", teamBySev)
	}
	if got := teamBySev["S1"]; got.CaseCount != 1 || !closeTo(got.TotalResolutionSec, 5*86400) {
		t.Errorf("team-filtered S1 = %+v, want count 1 (only the team account's case), total ~%d seconds", got, 5*86400)
	}

	// The window's start is inclusive, its end exclusive.
	exec(`UPDATE work_item_activity SET created_on = $1 WHERE work_item_id = $2`, windowStart, s4Outside)
	if atStart, err := repo.SeverityStats(ctx, windowStart, windowEnd, nil); err != nil || statsBySeverity(atStart)["S4"].CaseCount != 1 {
		t.Errorf("an entry exactly at windowStart must be included: err=%v stats=%v", err, atStart)
	}
	exec(`UPDATE work_item_activity SET created_on = $1 WHERE work_item_id = $2`, windowEnd, s4Outside)
	if atEnd, err := repo.SeverityStats(ctx, windowStart, windowEnd, nil); err != nil {
		t.Fatalf("SeverityStats (at windowEnd): %v", err)
	} else if _, ok := statsBySeverity(atEnd)["S4"]; ok {
		t.Errorf("an entry exactly at windowEnd must be excluded (exclusive upper bound), got %+v", statsBySeverity(atEnd)["S4"])
	}
}

// closeTo allows a small tolerance for floating-point accumulation across
// several EXTRACT(EPOCH ...) sums.
func closeTo(got, want float64) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff < 1.0
}
