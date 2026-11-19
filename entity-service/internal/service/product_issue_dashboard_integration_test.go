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

package service

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The product issue dashboard end to end against PostgreSQL: seeded issues, clocks,
// status events, links and cases through the real repository, service and the shared
// definitions. Every assertion filters to the test's own products, so rows other tests
// leave in the database do not count. Skipped without ENTITY_TEST_DATABASE_URL.

const (
	pdbA1       = "77000000-0000-4000-8000-0000000000e1"
	pdbA2       = "77000000-0000-4000-8000-0000000000e2"
	pdbA3       = "77000000-0000-4000-8000-0000000000e3"
	pdbA4       = "77000000-0000-4000-8000-0000000000e4"
	pdbA5       = "77000000-0000-4000-8000-0000000000e5"
	pdbA6       = "77000000-0000-4000-8000-0000000000e6"
	pdbB1       = "77000000-0000-4000-8000-0000000000e7"
	pdbB2       = "77000000-0000-4000-8000-0000000000e8"
	pdbCaseShut = "77000000-0000-4000-8000-0000000000ea"
	pdbProdA    = "PDB Product A"
	pdbProdB    = "PDB Product B"
)

func TestProductIssueDashboardIntegration(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	issues := []string{pdbA1, pdbA2, pdbA3, pdbA4, pdbA5, pdbA6, pdbB1, pdbB2}
	cleanup := func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM work_item WHERE id = ANY($1::text[]::uuid[]) OR id = $2::uuid`, issues, pdbCaseShut)
		_, _ = pool.Exec(bg, `DELETE FROM product_repo_mapping WHERE product_name IN ($1, $2)`, pdbProdA, pdbProdB)
	}
	cleanup()
	t.Cleanup(cleanup)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec (%.70s): %v", sql, err)
		}
	}
	ist := time.FixedZone("IST", 5*3600+30*60)
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, ist)
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, ist) }

	// Two products share one repository; the board number tells them apart.
	exec(`INSERT INTO product_repo_mapping (created_by, updated_by, product_name, owner, repository, github_label, board_project_number, lead_emails)
	      VALUES ('test', 'test', $1, 'pdb-org', 'pdb-repo', 'pdb/a', 11, ARRAY['Lead.A@WSO2.com', ' ', 'shared@wso2.com']),
	             ('test', 'test', $2, 'PDB-org', 'pdb-REPO', 'pdb/b', 12, ARRAY['shared@wso2.com'])`, pdbProdA, pdbProdB)
	// A second case that is closed.
	exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id)
	      VALUES ($1::uuid, $2, $2, 'test', 'test', 'PDB-CASE-SHUT', 'PDB-WSO2-SHUT', 'closed case', 'CASE', $3, $4)`, pdbCaseShut, now, pwiAccountID, pwiProjectID)
	exec(`INSERT INTO "case" (id, autoclose_enabled, state) VALUES ($1::uuid, false, 'CLOSED')`, pdbCaseShut)

	issue := func(id, num, priority, board string, board11 bool, ghState string, openedOn time.Time, closedOn *time.Time, gh int) {
		t.Helper()
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		      VALUES ($1::uuid, $2, $2, 'test', 'test', $3, $3, 'PRODUCT_ISSUE')`, id, openedOn, num)
		exec(`INSERT INTO product_issue (id, priority, opened_by_email) VALUES ($1::uuid, $2, 'opener@wso2.com')`, id, priority)
		bn := 12
		if board11 {
			bn = 11
		}
		exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, github_created_on, github_state, github_closed_on,
		        board_status, board_status_raw, board_project_number, last_synced_on)
		      VALUES ($1::uuid, 'pdb-org', 'pdb-repo', $2, $3, $4, $5, NULLIF($6, ''), NULLIF($6, ''), $7, $8)`,
			id, gh, openedOn, ghState, closedOn, board, bn, now.Add(-time.Hour))
	}
	link := func(from, to string, active bool) {
		t.Helper()
		exec(`INSERT INTO work_item_link (from_id, to_id, kind, source, is_active, created_by, updated_by)
		      VALUES ($1::uuid, $2::uuid, 'AFFECTS', 'MANUAL', $3, 'test', 'test')`, from, to, active)
	}
	event := func(id, dedupe, to, waits string, raw *string, occurred time.Time) {
		t.Helper()
		exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
		      VALUES ($1::uuid, $2, $3, $4, 'WEBHOOK', $5, $6)`, id, to, waits, occurred, dedupe, raw)
	}
	clock := func(id, policy, stage string, start time.Time, paused, end *time.Time, breached bool) {
		t.Helper()
		exec(`INSERT INTO sla (created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, schedule, is_active, stage, has_breached,
		        start_on, duration, end_on, paused_on, business_pause_duration, business_elapsed_percentage)
		      SELECT $1, $1, 'test', 'test', $2::uuid, sp.id, sp.schedule, $4::text NOT IN ('ACHIEVED', 'COMPLETED'), $4::sla_stage_enum, $8,
		             $5, sp.duration, $7, $6, '0'::interval, 7
		        FROM sla_policy sp WHERE sp.name = $3 AND sp.applies_to = 'PRODUCT_ISSUE'`, start, id, policy, stage, start, paused, end, breached)
	}
	str := func(s string) *string { return &s }
	tm := func(t time.Time) *time.Time { return &t }

	// A1: open, in progress, P1 clock running 10h; blocks the open case.
	issue(pdbA1, "PDB-PI-1", "P1", "IN_PROGRESS", true, "OPEN", at(10, 10), nil, 9201)
	link(pdbA1, pwiCaseID, true)
	clock(pdbA1, "Product Issue P1", "IN_PROGRESS", now.Add(-10*time.Hour), nil, nil, false)
	// A2: open, waiting on CS for 5 days, no case (unmatched), clock paused.
	issue(pdbA2, "PDB-PI-2", "P1", "WAITING_ON_CS", true, "OPEN", at(2, 10), nil, 9202)
	event(pdbA2, "pdb-a2-woc", "WAITING_ON_CS", "CS", str("WOC"), now.Add(-5*24*time.Hour))
	clock(pdbA2, "Product Issue P1", "PAUSED", now.Add(-5*24*time.Hour-2*time.Hour), tm(now.Add(-5*24*time.Hour)), nil, false)
	// A3: closed on 12 Nov, clock met.
	issue(pdbA3, "PDB-PI-3", "P1", "RESOLVED", true, "CLOSED", at(5, 10), tm(at(12, 10)), 9203)
	clock(pdbA3, "Product Issue P1", "ACHIEVED", at(12, 0), nil, tm(at(12, 6)), false)
	// A4: closed on 13 Nov, clock breached; links the closed case.
	issue(pdbA4, "PDB-PI-4", "P1", "RESOLVED", true, "CLOSED", at(8, 10), tm(at(13, 10)), 9204)
	link(pdbA4, pdbCaseShut, true)
	clock(pdbA4, "Product Issue P1", "COMPLETED", at(8, 10), nil, tm(at(13, 10)), true)
	// A5: resolved on the board on 10 Nov, reopened on GitHub on 11 Nov: open again; links the closed case.
	issue(pdbA5, "PDB-PI-5", "P1", "RESOLVED", true, "OPEN", at(9, 10), nil, 9205)
	event(pdbA5, "pdb-a5-resolved", "RESOLVED", "NOBODY", str("Resolved"), at(10, 12))
	event(pdbA5, "pdb-a5-reopen", "REOPENED", "PRODUCT_TEAM", nil, at(11, 12))
	link(pdbA5, pdbCaseShut, true)
	// A6: open, waits on the product team, its link to the open case is inactive.
	issue(pdbA6, "PDB-PI-6", "P1", "OPEN", true, "OPEN", at(12, 10), nil, 9206)
	link(pdbA6, pwiCaseID, false)
	// B1: product B (board 12 on the shared repository), open.
	issue(pdbB1, "PDB-PI-7", "P1", "OPEN", false, "OPEN", at(11, 10), nil, 9207)

	// B2: GitHub still open, card in the board's Closed column since 14 Nov: resolved, not open.
	issue(pdbB2, "PDB-PI-8", "P1", "CLOSED", false, "OPEN", at(6, 10), nil, 9208)
	event(pdbB2, "pdb-b2-closed", "CLOSED", "NOBODY", str("Closed"), at(14, 12))

	svc := NewProductIssueDashboardService(repository.NewProductIssueDashboardRepository(repository.NewScoped(pool)), func() time.Time { return now })
	got, err := svc.Dashboard(ctx, ProductIssueDashboardRequest{From: "2026-11-09", To: "2026-11-16", Products: []string{"pdb product a"}})
	if err != nil {
		t.Fatal(err)
	}
	tot := got.Totals
	if tot.Open != 4 || tot.OpenedInPeriod != 3 || tot.ResolvedInPeriod != 2 || tot.WaitingOnCs != 1 || tot.WaitingOnProductTeam != 3 || tot.UnmatchedOpen != 2 {
		t.Errorf("product A totals = %+v", tot)
	}
	if tot.SLAMet != 1 || tot.SLABreached != 1 || tot.CompliancePercentage == nil || *tot.CompliancePercentage != 50 {
		t.Errorf("product A SLA outcomes = %+v", tot)
	}
	if tot.OpenBySLA.OK != 1 || tot.OpenBySLA.Paused != 1 || tot.OpenBySLA.None != 2 {
		t.Errorf("open by SLA = %+v, want ok 1 (A1), paused 1 (A2), none 2", tot.OpenBySLA)
	}
	if len(got.ByProduct) != 1 || got.ByProduct[0].ProductName == nil || *got.ByProduct[0].ProductName != pdbProdA {
		t.Fatalf("byProduct = %+v, want only product A (the shared repository resolved by board number)", got.ByProduct)
	}
	if got.BlockedCases.Total != 1 || len(got.BlockedCases.Cases) != 1 || got.BlockedCases.Cases[0].CaseNumber != "PWI-CASE-1" ||
		len(got.BlockedCases.Cases[0].Issues) != 1 || got.BlockedCases.Cases[0].Issues[0].Number != "PDB-PI-1" {
		t.Errorf("blocked cases = %+v, want the open case only (closed case and inactive link excluded)", got.BlockedCases)
	}
	if len(got.WocAgeing.Oldest) != 1 || got.WocAgeing.Oldest[0].Number != "PDB-PI-2" || got.WocAgeing.Oldest[0].WocSince == nil ||
		!got.WocAgeing.Oldest[0].WocSince.Equal(now.Add(-5*24*time.Hour)) {
		t.Errorf("woc oldest = %+v", got.WocAgeing.Oldest)
	}
	for _, b := range got.WocAgeing.Buckets {
		if (b.Label == "3-7d") != (b.Count == 1) {
			t.Errorf("ageing buckets = %+v, want the 5-day wait in 3-7d", got.WocAgeing.Buckets)
		}
	}
	if len(got.Trend) != 1 || got.Trend[0].Opened != 3 || got.Trend[0].Resolved != 2 {
		t.Errorf("trend = %+v", got.Trend)
	}
	if got.PeriodStart.Format(time.RFC3339) != "2026-11-09T00:00:00+05:30" {
		t.Errorf("periodStart = %s", got.PeriodStart.Format(time.RFC3339))
	}

	b, err := svc.Dashboard(ctx, ProductIssueDashboardRequest{From: "2026-11-09", To: "2026-11-16", Products: []string{pdbProdB}})
	if err != nil || b.Totals.Open != 1 || b.Totals.OpenedInPeriod != 1 || b.Totals.ResolvedInPeriod != 1 || len(b.ByProduct) != 1 || *b.ByProduct[0].ProductName != pdbProdB {
		t.Errorf("product B = %+v, %v", b.Totals, err)
	}

	// The weekly report over the same data: Monday 16 Nov run reports Nov 9-16.
	svc2 := NewProductIssueDashboardService(repository.NewProductIssueDashboardRepository(repository.NewScoped(pool)), func() time.Time { return at(18, 12) })
	rep, err := svc2.Report(ctx, ProductIssueReportRequest{Period: "week", Date: "2026-11-12", Products: []string{pdbProdA}})
	if err != nil || rep.Totals.OpenedInPeriod != 3 || rep.Totals.ResolvedInPeriod != 2 || rep.Bucket != "week" {
		t.Errorf("report = %+v, %v", rep.Totals, err)
	}
	if _, err := svc.Report(ctx, ProductIssueReportRequest{Period: "year"}); err == nil {
		t.Error("an unknown period must be refused")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Errorf("unknown period error = %T, want a validation error", err)
	}

	// The unfiltered answer carries the Unmapped group of an unmapped repository.
	exec(`UPDATE github_issue_link SET repository = 'pdb-unmapped' WHERE work_item_id = $1::uuid`, pdbA6)
	all, err := svc.Dashboard(ctx, ProductIssueDashboardRequest{From: "2026-11-09", To: "2026-11-16"})
	if err != nil {
		t.Fatal(err)
	}
	var unmapped *domain.ProductIssueDashboardProduct
	for i, p := range all.ByProduct {
		if p.ProductName == nil && p.Repository == "pdb-unmapped" {
			unmapped = &all.ByProduct[i]
		}
	}
	if unmapped == nil || unmapped.Owner != "pdb-org" || unmapped.Open != 1 {
		t.Errorf("unmapped group = %+v", unmapped)
	}
	if all.ByProduct[len(all.ByProduct)-1].ProductName != nil {
		t.Errorf("unmapped groups must sort last: %+v", all.ByProduct[len(all.ByProduct)-1])
	}

	// Lead emails: lower-cased, blank ignored, one entry per email with its products.
	rec, err := svc.Recipients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var leadA, shared *domain.ProductIssueRecipient
	for i, l := range rec.Leads {
		switch l.Email {
		case "lead.a@wso2.com":
			leadA = &rec.Leads[i]
		case "shared@wso2.com":
			shared = &rec.Leads[i]
		}
	}
	if leadA == nil || len(leadA.ProductNames) != 1 || leadA.ProductNames[0] != pdbProdA ||
		shared == nil || len(shared.ProductNames) != 2 || shared.ProductNames[0] != pdbProdA || shared.ProductNames[1] != pdbProdB {
		t.Errorf("recipients = %+v", rec.Leads)
	}
	// An inactive row's leads are not recipients.
	exec(`UPDATE product_repo_mapping SET is_active = false WHERE product_name = $1`, pdbProdB)
	rec, _ = svc.Recipients(ctx)
	for _, l := range rec.Leads {
		if l.Email == "shared@wso2.com" && len(l.ProductNames) != 1 {
			t.Errorf("inactive product still listed: %+v", l)
		}
	}
}
