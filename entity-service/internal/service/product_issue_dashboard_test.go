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
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var (
	dashLoc = time.FixedZone("Asia/Kolkata", 5*3600+30*60)
	// Wednesday 18 Nov 2026, 12:00 IST.
	dashNow = time.Date(2026, 11, 18, 12, 0, 0, 0, dashLoc)
	// The period of most tests: the week of Mon 9 Nov to Mon 16 Nov.
	dashStart = time.Date(2026, 11, 9, 0, 0, 0, 0, dashLoc)
	dashEnd   = time.Date(2026, 11, 16, 0, 0, 0, 0, dashLoc)
)

func sp(s string) *string       { return &s }
func tp(t time.Time) *time.Time { return &t }
func ip(i int) *int             { return &i }

func dashMap(name, owner, repo, label string, active bool, board *int) domain.ProductIssueDashboardMapping {
	return domain.ProductIssueDashboardMapping{ProductName: name, Owner: owner, Repository: repo, GithubLabel: label, IsActive: active, BoardProjectNumber: board}
}

// dashRow is an open issue of org/apim opened two days ago, on the board column OPEN.
func dashRow(n int) domain.ProductIssueDashboardRow {
	return domain.ProductIssueDashboardRow{
		ID: fmt.Sprintf("id-%03d", n), Number: fmt.Sprintf("PI%07d", n), Title: fmt.Sprintf("issue %d", n),
		Owner: "org", Repository: "apim", IssueNumber: n, GitHubState: "OPEN", BoardStatus: sp("OPEN"),
		OpenedOn: dashNow.Add(-48 * time.Hour), Labels: []string{}, BoardWaitsOn: "PRODUCT_TEAM",
	}
}

func dashClock(stage string, start time.Time, end *time.Time, budgetH float64, breached bool) *domain.ProductIssueClock {
	return &domain.ProductIssueClock{
		PolicyName: "p", Schedule: "", Stage: stage, StartOn: start, EndOn: end,
		BudgetSeconds: budgetH * 3600, HasBreached: breached,
	}
}

func dashBuild(rows []domain.ProductIssueDashboardRow, maps []domain.ProductIssueDashboardMapping, products ...string) domain.ProductIssueDashboard {
	return BuildProductIssueDashboard(ProductIssueDashboardInput{
		Rows: rows, Mappings: maps, Now: dashNow, Loc: dashLoc, Start: dashStart, End: dashEnd, Bucket: DashboardBucketWeek, Products: products,
	})
}

func TestDashboardProductResolution(t *testing.T) {
	apim := dashMap("API Manager", "org", "apim", "product/apim", true, ip(5))
	// Two products share one repository (open banking / open healthcare style).
	shareA := dashMap("Open Banking", "Org ", " Shared", "ob", true, ip(1))
	shareB := dashMap("Open Healthcare", "org", "shared", "ohc", true, ip(2))
	inactive := dashMap("Aaa Retired", "org", "shared", "none", false, nil)

	row := func(owner, repo string, labels []string, board *int) domain.ProductIssueDashboardRow {
		r := dashRow(1)
		r.Owner, r.Repository, r.Labels, r.BoardProjectNumber = owner, repo, labels, board
		return r
	}
	cases := []struct {
		name string
		row  domain.ProductIssueDashboardRow
		maps []domain.ProductIssueDashboardMapping
		want string // "" = unmapped
	}{
		{"single mapping, case and spaces ignored", row(" ORG", "APIM ", nil, nil), []domain.ProductIssueDashboardMapping{apim}, "API Manager"},
		{"no mapping for the repository", row("org", "other", nil, nil), []domain.ProductIssueDashboardMapping{apim}, ""},
		{"shared: board match beats label", row("org", "shared", []string{"ohc"}, ip(1)), []domain.ProductIssueDashboardMapping{shareA, shareB}, "Open Banking"},
		{"shared: label when no board match", row("org", "shared", []string{"OHC"}, ip(9)), []domain.ProductIssueDashboardMapping{shareA, shareB}, "Open Healthcare"},
		{"shared: active beats inactive when nothing else matches", row("org", "shared", nil, nil), []domain.ProductIssueDashboardMapping{inactive, shareB}, "Open Healthcare"},
		{"shared: name ascending as the last resort", row("org", "shared", nil, nil), []domain.ProductIssueDashboardMapping{shareB, shareA}, "Open Banking"},
		{"shared: board match wins even over an inactive row", row("org", "shared", nil, ip(7)), []domain.ProductIssueDashboardMapping{shareA, dashMap("Zed", "org", "shared", "z", false, ip(7))}, "Zed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := resolveDashboardProduct(c.row, c.maps)
			got := ""
			if m != nil {
				got = m.ProductName
			}
			if got != c.want {
				t.Fatalf("product = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDashboardEffectiveStateOpenAndResolved(t *testing.T) {
	boardResolved := dashRow(1)
	boardResolved.BoardStatus = sp("RESOLVED")
	boardResolved.LatestBoardEventOn = tp(dashStart.Add(24 * time.Hour))
	boardResolved.LatestResolvedEventOn = tp(dashStart.Add(24 * time.Hour))

	// Resolved on the board, then a GitHub reopen after it: open again, not resolved.
	reopened := boardResolved
	reopened.ID, reopened.Number = "id-002", "PI0000002"
	reopened.LatestReopenEventOn = tp(dashNow.Add(-2 * time.Hour))

	// A reopen older than the board's last move does not reopen it.
	staleReopen := boardResolved
	staleReopen.ID, staleReopen.Number = "id-003", "PI0000003"
	staleReopen.LatestReopenEventOn = tp(dashStart.Add(time.Hour))

	// GitHub closed, board still on a working column: closed, resolved on the close time.
	closed := dashRow(4)
	closed.GitHubState, closed.GitHubClosedOn = "CLOSED", tp(dashStart.Add(3*time.Hour))

	// Both known: the earliest of the two.
	both := dashRow(5)
	both.GitHubState, both.GitHubClosedOn = "CLOSED", tp(dashStart.Add(10*time.Hour))
	both.BoardStatus, both.LatestResolvedEventOn = sp("RESOLVED"), tp(dashStart.Add(5*time.Hour))

	// Board status moved to Reopened: open.
	boardReopened := dashRow(6)
	boardReopened.BoardStatus = sp("REOPENED")

	// Not on a board: open.
	noBoard := dashRow(7)
	noBoard.BoardStatus = nil

	d := dashBuild([]domain.ProductIssueDashboardRow{boardResolved, reopened, staleReopen, closed, both, boardReopened, noBoard}, nil)
	if d.Totals.Open != 3 {
		t.Fatalf("open = %d, want 3 (resolved-then-reopened, board REOPENED, not on a board)", d.Totals.Open)
	}
	if d.Totals.ResolvedInPeriod != 4 {
		t.Fatalf("resolvedInPeriod = %d, want 4 (1, 3, 4, 5)", d.Totals.ResolvedInPeriod)
	}
	// Issue 5 resolved at the earlier RESOLVED event, not the later GitHub close.
	got := dashboardResolvedOn(both)
	if got == nil || !got.Equal(dashStart.Add(5*time.Hour)) {
		t.Fatalf("resolvedOn(both) = %v, want the earlier instant", got)
	}
	// A CLOSED state with neither source is resolved at an unknown time: not counted.
	unknown := dashRow(8)
	unknown.GitHubState = "CLOSED"
	if dashboardResolvedOn(unknown) != nil {
		t.Fatal("resolvedOn with no close time and no RESOLVED event must be nil")
	}
}

func TestDashboardWaitsOnAndWocSince(t *testing.T) {
	woc := func(n int) domain.ProductIssueDashboardRow {
		r := dashRow(n)
		r.BoardStatus, r.BoardWaitsOn = sp("WAITING_ON_CS"), "CS"
		return r
	}
	withEvent := woc(1)
	withEvent.LatestWOCEventOn = tp(dashNow.Add(-5 * 24 * time.Hour))
	withEvent.Clock = dashClock("PAUSED", dashNow.Add(-9*24*time.Hour), nil, 40, false)
	withEvent.Clock.PausedOn = tp(dashNow.Add(-4 * 24 * time.Hour))
	withEvent.LastSyncedOn = tp(dashNow.Add(-1 * time.Hour))

	withPause := woc(2)
	withPause.Clock = dashClock("PAUSED", dashNow.Add(-9*24*time.Hour), nil, 40, false)
	withPause.Clock.PausedOn = tp(dashNow.Add(-2 * 24 * time.Hour))
	withPause.LastSyncedOn = tp(dashNow.Add(-1 * time.Hour))

	withSync := woc(3)
	withSync.LastSyncedOn = tp(dashNow.Add(-20 * 24 * time.Hour))

	// Waits on the product team and on nobody: not in the ageing.
	prod := dashRow(4)
	prod.BoardStatus = sp("WAITING_ON_PRODUCT_TEAM")
	custom := dashRow(5)
	custom.BoardWaitsOn = "CUSTOMER"
	closed := woc(6)
	closed.GitHubState = "CLOSED"
	closed.GitHubClosedOn = tp(dashStart.Add(24 * time.Hour))

	d := dashBuild([]domain.ProductIssueDashboardRow{withEvent, withPause, withSync, prod, custom, closed}, nil)
	if d.Totals.WaitingOnCs != 3 || d.Totals.WaitingOnProductTeam != 1 || d.Totals.Open != 5 {
		t.Fatalf("totals = %+v, want waitingOnCs 3, waitingOnProductTeam 1 (the CUSTOMER wait is neither), open 5", d.Totals)
	}
	if d.Totals.Open != 5 || d.Totals.ResolvedInPeriod != 1 {
		t.Fatalf("closed issue must not be open, and resolves in the period: %+v", d.Totals)
	}
	wantBuckets := map[string]int{"<1d": 0, "1-3d": 1, "3-7d": 1, "7-14d": 0, ">14d": 1}
	if len(d.WocAgeing.Buckets) != 5 {
		t.Fatalf("buckets = %+v", d.WocAgeing.Buckets)
	}
	for _, b := range d.WocAgeing.Buckets {
		if b.Count != wantBuckets[b.Label] {
			t.Errorf("bucket %s = %d, want %d", b.Label, b.Count, wantBuckets[b.Label])
		}
	}
	if len(d.WocAgeing.Oldest) != 3 || d.WocAgeing.Oldest[0].Number != "PI0000003" || d.WocAgeing.Oldest[2].Number != "PI0000002" {
		t.Fatalf("oldest order = %+v, want PI3 (20d), PI1 (5d), PI2 (2d)", d.WocAgeing.Oldest)
	}
	if d.WocAgeing.Oldest[1].WocSince == nil || !d.WocAgeing.Oldest[1].WocSince.Equal(*withEvent.LatestWOCEventOn) {
		t.Fatal("WOC since must prefer the newest WAITING_ON_CS event over the clock's paused_on")
	}
}

func TestDashboardAgeBucketEdges(t *testing.T) {
	h := time.Hour
	cases := map[time.Duration]int{
		-h: 0, 0: 0, 24*h - 1: 0, 24 * h: 1, 72*h - 1: 1, 72 * h: 2, 7*24*h - 1: 2, 7 * 24 * h: 3, 14*24*h - 1: 3, 14 * 24 * h: 4, 100 * 24 * h: 4,
	}
	for age, want := range cases {
		if got := dashboardAgeBucket(age); got != want {
			t.Errorf("age %v -> bucket %d, want %d", age, got, want)
		}
	}
}

func TestDashboardSLAOutcomeAndCompliance(t *testing.T) {
	start := dashNow.Add(-10 * 24 * time.Hour)
	met := dashRow(1)
	met.BoardStatus, met.GitHubState, met.GitHubClosedOn = sp("RESOLVED"), "CLOSED", tp(dashStart.Add(24*time.Hour))
	met.Clock = dashClock("ACHIEVED", start, tp(dashStart.Add(24*time.Hour)), 1000, false)

	breached := dashRow(2)
	breached.GitHubState, breached.GitHubClosedOn = "CLOSED", tp(dashStart.Add(48*time.Hour))
	breached.Clock = dashClock("COMPLETED", start, tp(dashStart.Add(48*time.Hour)), 1, true)

	// Finished before the period, and exactly at the end instant: both outside [start, end).
	before := dashRow(3)
	before.GitHubState, before.GitHubClosedOn = "CLOSED", tp(dashStart.Add(-time.Hour))
	before.Clock = dashClock("ACHIEVED", start, tp(dashStart.Add(-time.Hour)), 1000, false)
	atEnd := dashRow(4)
	atEnd.GitHubState, atEnd.GitHubClosedOn = "CLOSED", tp(dashEnd)
	atEnd.Clock = dashClock("ACHIEVED", start, tp(dashEnd), 1000, false)
	// Exactly the start instant is inside.
	atStart := dashRow(5)
	atStart.GitHubState, atStart.GitHubClosedOn = "CLOSED", tp(dashStart)
	atStart.Clock = dashClock("ACHIEVED", start, tp(dashStart), 1000, false)

	// A running clock is not an outcome.
	running := dashRow(6)
	running.Clock = dashClock("IN_PROGRESS", start, nil, 1000, false)

	d := dashBuild([]domain.ProductIssueDashboardRow{met, breached, before, atEnd, atStart, running}, nil)
	if d.Totals.SLAMet != 2 || d.Totals.SLABreached != 1 {
		t.Fatalf("met/breached = %d/%d, want 2/1", d.Totals.SLAMet, d.Totals.SLABreached)
	}
	if c := d.Totals.CompliancePercentage; c == nil || *c != 66.7 {
		t.Fatalf("compliance = %v, want 66.7", c)
	}

	empty := dashBuild([]domain.ProductIssueDashboardRow{running}, nil)
	if empty.Totals.CompliancePercentage != nil {
		t.Fatalf("compliance with no outcome = %v, want nil", *empty.Totals.CompliancePercentage)
	}
	if compliancePercentage(1, 0) == nil || *compliancePercentage(1, 0) != 100 || *compliancePercentage(0, 3) != 0 {
		t.Fatal("compliance 1/0 must be 100 and 0/3 must be 0")
	}
}

func TestDashboardOpenBySLAAndBreachedList(t *testing.T) {
	// Budgets are in calendar hours on the default (24x7) schedule of an empty Schedule.
	running := func(n int, startedAgoH, budgetH float64, breached bool) domain.ProductIssueDashboardRow {
		r := dashRow(n)
		r.Clock = dashClock("IN_PROGRESS", dashNow.Add(-time.Duration(startedAgoH*float64(time.Hour))), nil, budgetH, breached)
		return r
	}
	ok := running(1, 10, 100, false)
	atRisk := running(2, 80, 100, false)
	late := running(3, 120, 100, false)  // consumed >= budget while running: breached
	flagged := running(4, 10, 100, true) // stored flag
	none := dashRow(5)                   // no clock
	paused := dashRow(6)
	paused.Clock = dashClock("PAUSED", dashNow.Add(-50*time.Hour), nil, 100, false)
	paused.Clock.PausedOn = tp(dashNow.Add(-20 * time.Hour))
	finishedOpen := dashRow(7) // open issue whose clock already finished: counted under none
	finishedOpen.Clock = dashClock("ACHIEVED", dashNow.Add(-30*time.Hour), tp(dashNow.Add(-20*time.Hour)), 100, false)

	d := dashBuild([]domain.ProductIssueDashboardRow{ok, atRisk, late, flagged, none, paused, finishedOpen}, nil)
	c := d.Totals.OpenBySLA
	if c.OK != 1 || c.AtRisk != 1 || c.Breached != 2 || c.Paused != 1 || c.None != 2 {
		t.Fatalf("openBySla = %+v, want ok 1, atRisk 1, breached 2, paused 1, none 2", c)
	}
	if len(d.BreachedOpen) != 2 || d.BreachedOpen[0].Number != "PI0000003" {
		t.Fatalf("breachedOpen = %+v, want the issue at the highest percentage first", d.BreachedOpen)
	}
	if d.BreachedOpen[0].SLA.State != domain.ProductIssueSLABreached {
		t.Fatalf("breached item sla = %+v", d.BreachedOpen[0].SLA)
	}
}

func TestDashboardListCapsAndOrdering(t *testing.T) {
	var rows []domain.ProductIssueDashboardRow
	for i := 1; i <= 30; i++ {
		r := dashRow(i)
		r.BoardStatus, r.BoardWaitsOn = sp("WAITING_ON_CS"), "CS"
		r.LatestWOCEventOn = tp(dashNow.Add(-time.Duration(i) * time.Hour)) // issue 30 waited longest
		r.Clock = dashClock("IN_PROGRESS", dashNow.Add(-time.Duration(1000+i)*time.Hour), nil, 10, false)
		rows = append(rows, r)
	}
	d := dashBuild(rows, nil)
	if len(d.WocAgeing.Oldest) != 20 || d.WocAgeing.Oldest[0].Number != "PI0000030" || d.WocAgeing.Oldest[19].Number != "PI0000011" {
		t.Fatalf("oldest = %d items, first %s last %s; want 20, PI30 .. PI11", len(d.WocAgeing.Oldest), d.WocAgeing.Oldest[0].Number, d.WocAgeing.Oldest[19].Number)
	}
	if len(d.BreachedOpen) != 20 {
		t.Fatalf("breachedOpen = %d, want the cap of 20", len(d.BreachedOpen))
	}
	total := 0
	for _, b := range d.WocAgeing.Buckets {
		total += b.Count
	}
	if total != 30 || d.Totals.WaitingOnCs != 30 {
		t.Fatalf("ageing counts every waiting issue, not only the listed ones: %d, %d", total, d.Totals.WaitingOnCs)
	}
}

func TestDashboardBlockedCases(t *testing.T) {
	openCase := func(id string) *domain.ProductIssueDashboardCase {
		return &domain.ProductIssueDashboardCase{ID: id, Number: "CS-" + id, Subject: "subject " + id, Open: true}
	}
	a1 := dashRow(1) // blocks case A, opened 5 days ago
	a1.Case, a1.OpenedOn = openCase("A"), dashNow.Add(-5*24*time.Hour)
	a2 := dashRow(2) // second blocker of case A, opened later
	a2.Case, a2.OpenedOn = openCase("A"), dashNow.Add(-1*24*time.Hour)
	b1 := dashRow(3) // case B, opened 9 days ago, other product
	b1.Owner, b1.Repository = "org", "is"
	b1.Case, b1.OpenedOn = openCase("B"), dashNow.Add(-9*24*time.Hour)
	closedCase := dashRow(4) // the case is closed: excluded
	closedCase.Case = &domain.ProductIssueDashboardCase{ID: "C", Number: "CS-C", Subject: "closed", Open: false}
	wocIssue := dashRow(5) // waits on CS: does not block
	wocIssue.BoardStatus, wocIssue.BoardWaitsOn = sp("WAITING_ON_CS"), "CS"
	wocIssue.Case = openCase("D")
	resolved := dashRow(6) // resolved: does not block
	resolved.BoardStatus, resolved.GitHubState, resolved.GitHubClosedOn = sp("RESOLVED"), "CLOSED", tp(dashNow.Add(-time.Hour))
	resolved.Case = openCase("E")
	inactiveLink := dashRow(7) // an inactive link is not loaded: Case is nil, the issue is unmatched
	maps := []domain.ProductIssueDashboardMapping{
		dashMap("API Manager", "org", "apim", "x", true, nil), dashMap("Identity Server", "org", "is", "y", true, nil),
	}

	d := dashBuild([]domain.ProductIssueDashboardRow{a1, a2, b1, closedCase, wocIssue, resolved, inactiveLink}, maps)
	bc := d.BlockedCases
	if bc.Total != 2 || len(bc.Cases) != 2 {
		t.Fatalf("blocked total/len = %d/%d, want 2/2 (A and B)", bc.Total, len(bc.Cases))
	}
	if bc.Cases[0].CaseID != "B" || bc.Cases[1].CaseID != "A" {
		t.Fatalf("order = %s, %s; want B (9d) before A (5d)", bc.Cases[0].CaseID, bc.Cases[1].CaseID)
	}
	if len(bc.Cases[1].Issues) != 2 || bc.Cases[1].Issues[0].Number != "PI0000001" || !bc.Cases[1].OldestOpenedOn.Equal(a1.OpenedOn) {
		t.Fatalf("case A = %+v", bc.Cases[1])
	}
	if len(bc.ByProduct) != 2 || *bc.ByProduct[0].ProductName != "API Manager" || bc.ByProduct[0].Cases != 1 || *bc.ByProduct[1].ProductName != "Identity Server" {
		t.Fatalf("byProduct = %+v", bc.ByProduct)
	}
	if d.Totals.UnmatchedOpen != 1 {
		t.Fatalf("unmatchedOpen = %d, want 1 (the issue with no active link)", d.Totals.UnmatchedOpen)
	}

	// A case blocked by issues of two products counts once in total, once per product.
	x := dashRow(8)
	x.Case = openCase("X")
	y := dashRow(9)
	y.Owner, y.Repository, y.Case = "org", "is", openCase("X")
	d = dashBuild([]domain.ProductIssueDashboardRow{x, y}, maps)
	if d.BlockedCases.Total != 1 || len(d.BlockedCases.ByProduct) != 2 || len(d.BlockedCases.Cases[0].Issues) != 2 {
		t.Fatalf("shared case = %+v", d.BlockedCases)
	}
	// Filtered to one product, only that product's blocker is listed.
	d = dashBuild([]domain.ProductIssueDashboardRow{x, y}, maps, "Identity Server")
	if d.BlockedCases.Total != 1 || len(d.BlockedCases.Cases[0].Issues) != 1 || d.BlockedCases.Cases[0].Issues[0].Number != "PI0000009" {
		t.Fatalf("filtered case = %+v", d.BlockedCases)
	}
}

func TestDashboardBlockedCasesCapAt50(t *testing.T) {
	var rows []domain.ProductIssueDashboardRow
	for i := 1; i <= 55; i++ {
		r := dashRow(i)
		r.OpenedOn = dashNow.Add(-time.Duration(i) * time.Hour)
		r.Case = &domain.ProductIssueDashboardCase{ID: fmt.Sprintf("c%02d", i), Number: fmt.Sprintf("CS%02d", i), Open: true}
		rows = append(rows, r)
	}
	d := dashBuild(rows, nil)
	if d.BlockedCases.Total != 55 || len(d.BlockedCases.Cases) != 50 || d.BlockedCases.Cases[0].CaseID != "c55" {
		t.Fatalf("total %d, listed %d, first %s; want 55, 50, c55 (oldest first)", d.BlockedCases.Total, len(d.BlockedCases.Cases), d.BlockedCases.Cases[0].CaseID)
	}
}

func TestDashboardPeriodEdgesInKolkata(t *testing.T) {
	// 2026-11-08 18:29:59 UTC is 23:59:59 IST on the Sunday: before the period.
	// 2026-11-08 18:30:00 UTC is Monday 00:00:00 IST: the first instant.
	before := dashRow(1)
	before.OpenedOn = time.Date(2026, 11, 8, 18, 29, 59, 0, time.UTC)
	first := dashRow(2)
	first.OpenedOn = time.Date(2026, 11, 8, 18, 30, 0, 0, time.UTC)
	last := dashRow(3)
	last.OpenedOn = time.Date(2026, 11, 15, 18, 29, 59, 0, time.UTC) // Sunday 23:59:59 IST
	next := dashRow(4)
	next.OpenedOn = time.Date(2026, 11, 15, 18, 30, 0, 0, time.UTC) // Monday 00:00 IST: outside

	d := dashBuild([]domain.ProductIssueDashboardRow{before, first, last, next}, nil)
	if d.Totals.OpenedInPeriod != 2 {
		t.Fatalf("openedInPeriod = %d, want 2", d.Totals.OpenedInPeriod)
	}
	if len(d.Trend) != 1 || d.Trend[0].Opened != 2 {
		t.Fatalf("trend = %+v", d.Trend)
	}
	if got := d.PeriodStart.Format(time.RFC3339); got != "2026-11-09T00:00:00+05:30" {
		t.Fatalf("periodStart = %s", got)
	}
}

func TestDashboardTrendBuckets(t *testing.T) {
	// A mid-week start is clipped to the period and a month spans five calendar weeks.
	b := dashboardBuckets(time.Date(2026, 11, 11, 0, 0, 0, 0, dashLoc), time.Date(2026, 11, 24, 0, 0, 0, 0, dashLoc), DashboardBucketWeek, dashLoc)
	if len(b) != 3 || !b[0][0].Equal(time.Date(2026, 11, 11, 0, 0, 0, 0, dashLoc)) || !b[0][1].Equal(time.Date(2026, 11, 16, 0, 0, 0, 0, dashLoc)) ||
		!b[2][1].Equal(time.Date(2026, 11, 24, 0, 0, 0, 0, dashLoc)) {
		t.Fatalf("weeks = %v", b)
	}
	b = dashboardBuckets(time.Date(2026, 11, 1, 0, 0, 0, 0, dashLoc), time.Date(2026, 12, 1, 0, 0, 0, 0, dashLoc), DashboardBucketWeek, dashLoc)
	// 1 Nov 2026 is a Sunday: a one-day first week, four full weeks, a one-day last week.
	if len(b) != 6 || !b[0][1].Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, dashLoc)) || !b[5][0].Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, dashLoc)) {
		t.Fatalf("november weeks = %v", b)
	}
	b = dashboardBuckets(time.Date(2026, 9, 15, 0, 0, 0, 0, dashLoc), time.Date(2026, 12, 2, 0, 0, 0, 0, dashLoc), DashboardBucketMonth, dashLoc)
	if len(b) != 4 || !b[0][0].Equal(time.Date(2026, 9, 15, 0, 0, 0, 0, dashLoc)) || !b[0][1].Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, dashLoc)) ||
		!b[3][0].Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, dashLoc)) || !b[3][1].Equal(time.Date(2026, 12, 2, 0, 0, 0, 0, dashLoc)) {
		t.Fatalf("months = %v", b)
	}

	// The trend sums to the period totals, and a resolved issue lands in its bucket.
	in := BuildProductIssueDashboard(ProductIssueDashboardInput{
		Rows: func() []domain.ProductIssueDashboardRow {
			a := dashRow(1)
			a.OpenedOn = time.Date(2026, 11, 3, 10, 0, 0, 0, dashLoc)
			c := dashRow(2)
			c.OpenedOn = time.Date(2026, 11, 20, 10, 0, 0, 0, dashLoc)
			c.GitHubState, c.GitHubClosedOn = "CLOSED", tp(time.Date(2026, 11, 25, 10, 0, 0, 0, dashLoc))
			return []domain.ProductIssueDashboardRow{a, c}
		}(),
		Now: dashNow, Loc: dashLoc, Start: time.Date(2026, 11, 1, 0, 0, 0, 0, dashLoc), End: time.Date(2026, 12, 1, 0, 0, 0, 0, dashLoc), Bucket: DashboardBucketWeek,
	})
	opened, resolved := 0, 0
	for _, p := range in.Trend {
		opened += p.Opened
		resolved += p.Resolved
	}
	if opened != 2 || resolved != 1 || in.Totals.OpenedInPeriod != 2 || in.Totals.ResolvedInPeriod != 1 {
		t.Fatalf("trend %+v totals %+v", in.Trend, in.Totals)
	}
	if in.Trend[1].Opened != 1 || in.Trend[3].Opened != 1 || in.Trend[4].Resolved != 1 {
		t.Fatalf("buckets = %+v", in.Trend)
	}
}

func TestDashboardProductFilterAndGrouping(t *testing.T) {
	apim1, apim2, isr, unm := dashRow(1), dashRow(2), dashRow(3), dashRow(4)
	isr.Repository = "is"
	unm.Owner, unm.Repository = "Other", "Thing"
	maps := []domain.ProductIssueDashboardMapping{
		dashMap("API Manager", "org", "apim", "x", true, nil), dashMap("Identity Server", "org", "is", "y", true, nil),
	}
	rows := []domain.ProductIssueDashboardRow{apim1, apim2, isr, unm}

	all := dashBuild(rows, maps)
	if all.Totals.Open != 4 || len(all.ByProduct) != 3 {
		t.Fatalf("all = %d open, %d groups", all.Totals.Open, len(all.ByProduct))
	}
	if all.ByProduct[0].ProductName == nil || *all.ByProduct[0].ProductName != "API Manager" || all.ByProduct[0].Open != 2 ||
		*all.ByProduct[1].ProductName != "Identity Server" || all.ByProduct[2].ProductName != nil || all.ByProduct[2].Owner != "Other" {
		t.Fatalf("byProduct = %+v (want sorted by name, unmapped last)", all.ByProduct)
	}
	if got := dashBuild(rows, maps, "api manager"); got.Totals.Open != 2 || len(got.ByProduct) != 1 {
		t.Fatalf("case-insensitive filter: %+v", got.Totals)
	}
	if got := dashBuild(rows, maps, " Unmapped ", "identity server"); got.Totals.Open != 2 || len(got.ByProduct) != 2 || len(got.Products) != 2 {
		t.Fatalf("unmapped + named: %+v", got.Totals)
	}
	if got := dashBuild(rows, maps, "nonexistent"); got.Totals.Open != 0 || len(got.ByProduct) != 0 {
		t.Fatalf("unknown product selects nothing: %+v", got.Totals)
	}
	// Two products on one repository: two groups only if issues resolve to both.
	shared := []domain.ProductIssueDashboardMapping{
		dashMap("Open Banking", "org", "apim", "ob", true, nil), dashMap("Open Healthcare", "org", "apim", "ohc", true, nil),
	}
	a, b := dashRow(1), dashRow(2)
	a.Labels, b.Labels = []string{"ob"}, []string{"ohc"}
	g := dashBuild([]domain.ProductIssueDashboardRow{a, b}, shared)
	if len(g.ByProduct) != 2 || g.ByProduct[0].Open != 1 || g.ByProduct[1].Open != 1 {
		t.Fatalf("shared repo groups = %+v", g.ByProduct)
	}
}

func TestDashboardNonStandardAndUnmatched(t *testing.T) {
	other := dashRow(1)
	other.BoardStatus = sp("OTHER")
	other.Case = &domain.ProductIssueDashboardCase{ID: "c", Number: "CS1", Open: true}
	resolvedOther := dashRow(2)
	resolvedOther.BoardStatus, resolvedOther.GitHubState = sp("OTHER"), "CLOSED"
	d := dashBuild([]domain.ProductIssueDashboardRow{other, resolvedOther}, nil)
	if d.Totals.NonStandardColumnOpen != 1 || d.Totals.UnmatchedOpen != 0 {
		t.Fatalf("totals = %+v (OTHER open counts; a closed issue is neither)", d.Totals)
	}
}

func TestDashboardEmptyData(t *testing.T) {
	d := dashBuild(nil, nil)
	if d.Totals.Open != 0 || d.Totals.CompliancePercentage != nil {
		t.Fatalf("totals = %+v", d.Totals)
	}
	if d.Trend == nil || len(d.Trend) != 1 || d.ByProduct == nil || d.WocAgeing.Oldest == nil || d.BreachedOpen == nil ||
		d.BlockedCases.Cases == nil || d.BlockedCases.ByProduct == nil || d.Products == nil || len(d.WocAgeing.Buckets) != 5 {
		t.Fatalf("empty slices must be non-nil (JSON []): %+v", d)
	}
}

func TestResolveDashboardPeriod(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, dashLoc) }
	// Defaults: to = tomorrow, from = the Monday 12 weeks before this week's Monday.
	s, e, err := resolveDashboardPeriod("", "", dashNow, dashLoc)
	if err != nil || !s.Equal(day(2026, 8, 24)) || !e.Equal(day(2026, 11, 19)) {
		t.Fatalf("defaults = %v..%v, %v; want 2026-08-24..2026-11-19", s, e, err)
	}
	// A Monday 00:00:30 IST now: same week, and a Sunday 23:59 UTC instant is already Monday in IST.
	mon := time.Date(2026, 11, 15, 18, 45, 0, 0, time.UTC) // Monday 16 Nov 00:15 IST
	s, e, err = resolveDashboardPeriod("", "", mon, dashLoc)
	if err != nil || !s.Equal(day(2026, 8, 24)) || !e.Equal(day(2026, 11, 17)) {
		t.Fatalf("monday defaults = %v..%v, %v", s, e, err)
	}
	s, e, err = resolveDashboardPeriod("2026-01-01", "2026-02-01", dashNow, dashLoc)
	if err != nil || !s.Equal(day(2026, 1, 1)) || !e.Equal(day(2026, 2, 1)) {
		t.Fatalf("explicit = %v..%v, %v", s, e, err)
	}
	// 400 days is accepted, 401 is not.
	if _, _, err = resolveDashboardPeriod("2025-01-01", "2026-02-05", dashNow, dashLoc); err != nil {
		t.Fatalf("400 days refused: %v", err)
	}
	bad := []struct{ from, to string }{
		{"2025-01-01", "2026-02-06"}, // 401 days
		{"2026-02-01", "2026-02-01"}, // empty
		{"2026-02-02", "2026-02-01"}, // reversed
		{"2026-13-01", ""}, {"nope", ""}, {"", "2026-1-1x"},
		{"2026-12-01", ""}, // default to (tomorrow) is not after from
	}
	for _, c := range bad {
		_, _, err := resolveDashboardPeriod(c.from, c.to, dashNow, dashLoc)
		var ve *apierror.ValidationError
		if err == nil || !dashAsValidation(err, &ve) {
			t.Errorf("from=%q to=%q: err = %v, want a validation error", c.from, c.to, err)
		}
	}
}

func dashAsValidation(err error, target **apierror.ValidationError) bool {
	ve, ok := err.(*apierror.ValidationError)
	*target = ve
	return ok
}

func TestResolveReportPeriod(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, dashLoc) }
	cases := []struct {
		period, date string
		now          time.Time
		start, end   time.Time
	}{
		{"week", "", dashNow, day(2026, 11, 9), day(2026, 11, 16)},
		{"week", "2026-11-18", dashNow, day(2026, 11, 16), day(2026, 11, 23)},
		{"week", "2026-11-15", dashNow, day(2026, 11, 9), day(2026, 11, 16)}, // a Sunday belongs to the week before
		{"week", "2026-11-16", dashNow, day(2026, 11, 16), day(2026, 11, 23)},
		{"month", "", dashNow, day(2026, 10, 1), day(2026, 11, 1)},
		{"month", "", time.Date(2027, 1, 5, 9, 0, 0, 0, dashLoc), day(2026, 12, 1), day(2027, 1, 1)},
		{"month", "2026-11-30", dashNow, day(2026, 11, 1), day(2026, 12, 1)},
		{"month", "2026-02-10", dashNow, day(2026, 2, 1), day(2026, 3, 1)},
		// 2026-11-01 20:00 UTC is already 02:30 IST on the 2nd: the previous month is October.
		{"month", "", time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC), day(2026, 10, 1), day(2026, 11, 1)},
		// 2026-10-31 19:00 UTC is 00:30 IST on 1 Nov: the previous month is already October.
		{"month", "", time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC), day(2026, 10, 1), day(2026, 11, 1)},
		// 2026-10-31 17:00 UTC is 22:30 IST on 31 Oct: still October, so the previous month is September.
		{"month", "", time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC), day(2026, 9, 1), day(2026, 10, 1)},
	}
	for _, c := range cases {
		s, e, err := resolveReportPeriod(c.period, c.date, c.now, dashLoc)
		if err != nil || !s.Equal(c.start) || !e.Equal(c.end) {
			t.Errorf("%s %q at %v = %v..%v (%v), want %v..%v", c.period, c.date, c.now, s, e, err, c.start, c.end)
		}
	}
	for _, c := range [][2]string{{"", ""}, {"year", ""}, {"week", "11/18/2026"}, {"month", "x"}} {
		if _, _, err := resolveReportPeriod(c[0], c[1], dashNow, dashLoc); err == nil {
			t.Errorf("period %q date %q must be refused", c[0], c[1])
		}
	}
}

func TestBuildProductIssueRecipients(t *testing.T) {
	got := BuildProductIssueRecipients([]domain.ProductIssueLead{
		{ProductName: "Identity Server", Email: " Bob@WSO2.com "},
		{ProductName: "API Manager", Email: "bob@wso2.com"},
		{ProductName: "API Manager", Email: "bob@wso2.com"},
		{ProductName: "API Manager", Email: "alice@wso2.com"},
		{ProductName: "API Manager", Email: "   "},
		{ProductName: "API Manager", Email: ""},
	})
	if len(got.Leads) != 2 || got.Leads[0].Email != "alice@wso2.com" || got.Leads[1].Email != "bob@wso2.com" {
		t.Fatalf("leads = %+v", got.Leads)
	}
	if n := got.Leads[1].ProductNames; len(n) != 2 || n[0] != "API Manager" || n[1] != "Identity Server" {
		t.Fatalf("bob's products = %v", n)
	}
	if empty := BuildProductIssueRecipients(nil); empty.Leads == nil || len(empty.Leads) != 0 {
		t.Fatalf("empty = %+v, want a non-nil empty list", empty)
	}
}

func TestDashboardBoardClosedIsResolved(t *testing.T) {
	// GitHub still open, card in the board's Closed column: not open, resolved at the board move.
	closed := dashRow(1)
	closed.BoardStatus, closed.LatestClosedEventOn = sp("CLOSED"), tp(dashStart.Add(30*time.Hour))
	// The CLOSED event only counts while the board status is CLOSED.
	moved := dashRow(2)
	moved.LatestClosedEventOn = tp(dashStart.Add(30 * time.Hour))
	noEvent := dashRow(3)
	noEvent.BoardStatus = sp("CLOSED")
	// With GitHub closed earlier, the earliest candidate wins.
	both := closed
	both.ID, both.Number = "id-004", "PI0000004"
	both.GitHubState, both.GitHubClosedOn = "CLOSED", tp(dashStart.Add(10*time.Hour))

	d := dashBuild([]domain.ProductIssueDashboardRow{closed, moved, noEvent, both}, nil)
	if d.Totals.Open != 1 || d.Totals.ResolvedInPeriod != 2 {
		t.Fatalf("totals = %+v, want open 1 (the issue not in the Closed column), resolved 2", d.Totals)
	}
	if got := dashboardResolvedOn(both); got == nil || !got.Equal(dashStart.Add(10*time.Hour)) {
		t.Fatalf("resolvedOn(both) = %v", got)
	}
}

func TestDashboardOpenIssueWithFinishedClockIsNone(t *testing.T) {
	start := dashNow.Add(-30 * time.Hour)
	end := dashNow.Add(-5 * time.Hour)
	finishedBreached := dashRow(1)
	finishedBreached.Clock = dashClock("COMPLETED", start, &end, 1000, true)
	finishedMet := dashRow(2)
	finishedMet.Clock = dashClock("ACHIEVED", start, &end, 1000, false)
	running := dashRow(3)
	running.Clock = dashClock("IN_PROGRESS", start, nil, 10, false) // 30h consumed of 10h: breached

	d := dashBuild([]domain.ProductIssueDashboardRow{finishedBreached, finishedMet, running}, nil)
	c := d.Totals.OpenBySLA
	if c.Breached != 1 || c.None != 2 {
		t.Fatalf("openBySla = %+v, want breached 1 (running), none 2", c)
	}
	if len(d.BreachedOpen) != 1 || d.BreachedOpen[0].Number != "PI0000003" {
		t.Fatalf("breachedOpen = %+v, want only the running one", d.BreachedOpen)
	}
	// The item's own sla still reports the clock's state.
	if len(d.ByProduct) != 1 || d.ByProduct[0].OpenBySLA.None != 2 {
		t.Fatalf("byProduct = %+v", d.ByProduct)
	}
}
