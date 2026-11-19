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
	"math"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The one place that defines the product issue dashboard's metrics (phase 4
// contract section 1). GET /product-work-items/dashboard and GET
// /reports/product-issues both call BuildProductIssueDashboard; it reads no
// database and no clock, so every definition is unit-tested.

const (
	// DashboardBucketWeek and DashboardBucketMonth are the trend bucket sizes.
	DashboardBucketWeek  = "week"
	DashboardBucketMonth = "month"
	// ProductIssueUnmappedName selects (in the products filter) issues with no product.
	ProductIssueUnmappedName = "Unmapped"

	dashboardOldestLimit   = 20
	dashboardBreachedLimit = 20
	dashboardBlockedLimit  = 50
)

// ProductIssueDashboardInput is everything BuildProductIssueDashboard reads.
type ProductIssueDashboardInput struct {
	Rows     []domain.ProductIssueDashboardRow
	Mappings []domain.ProductIssueDashboardMapping
	// Now is the "as of" instant of every non-historical figure.
	Now time.Time
	// Loc is the calendar of week and month buckets (Asia/Kolkata).
	Loc *time.Location
	// Start and End bound the period [Start, End).
	Start, End time.Time
	// Bucket is DashboardBucketWeek or DashboardBucketMonth.
	Bucket string
	// Products is the product filter as given: names, or "Unmapped"; empty selects all.
	Products []string
}

// dashIssue is one issue with every derived value computed once.
type dashIssue struct {
	row         domain.ProductIssueDashboardRow
	mapping     *domain.ProductIssueDashboardMapping
	productName *string
	owner       string
	repository  string
	state       string
	open        bool
	waitsOn     string
	resolvedOn  *time.Time
	wocSince    *time.Time
	sla         domain.ProductIssueSLA
}

// dashProductKey groups issues: one group per mapped product, one per unmapped repository.
type dashProductKey struct {
	mapped bool
	name   string // lower-cased product name, or "owner/repository" lower-cased
}

func normRepoPart(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// resolveDashboardProduct is the product of an issue: the product_repo_mapping row of
// its owner/repository (case-insensitive, trimmed), preferring a row whose
// board_project_number equals the link's, then one whose github_label is among the
// link's labels (case-insensitive), then an active one, then the smallest product
// name. nil when no row has the repository.
func resolveDashboardProduct(row domain.ProductIssueDashboardRow, mappings []domain.ProductIssueDashboardMapping) *domain.ProductIssueDashboardMapping {
	owner, repo := normRepoPart(row.Owner), normRepoPart(row.Repository)
	labels := map[string]bool{}
	for _, l := range row.Labels {
		labels[strings.ToLower(strings.TrimSpace(l))] = true
	}
	score := func(m *domain.ProductIssueDashboardMapping) (board, label, active bool) {
		board = row.BoardProjectNumber != nil && m.BoardProjectNumber != nil && *row.BoardProjectNumber == *m.BoardProjectNumber
		label = labels[strings.ToLower(strings.TrimSpace(m.GithubLabel))]
		return board, label, m.IsActive
	}
	var best *domain.ProductIssueDashboardMapping
	for i := range mappings {
		m := &mappings[i]
		if normRepoPart(m.Owner) != owner || normRepoPart(m.Repository) != repo {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		mb, ml, ma := score(m)
		bb, bl, ba := score(best)
		switch {
		case mb != bb:
			if mb {
				best = m
			}
		case ml != bl:
			if ml {
				best = m
			}
		case ma != ba:
			if ma {
				best = m
			}
		case m.ProductName < best.ProductName:
			best = m
		}
	}
	return best
}

// dashboardResolvedOn is when a not-open issue was resolved: the earliest of
// github_closed_on (GitHub state CLOSED) and the newest RESOLVED status event (board
// status RESOLVED). nil when neither is known.
func dashboardResolvedOn(r domain.ProductIssueDashboardRow) *time.Time {
	var out *time.Time
	consider := func(t *time.Time) {
		if t != nil && (out == nil || t.Before(*out)) {
			c := *t
			out = &c
		}
	}
	if r.GitHubState == piStateClosed {
		consider(r.GitHubClosedOn)
	}
	if r.BoardStatus != nil && *r.BoardStatus == piStateResolved {
		consider(r.LatestResolvedEventOn)
	}
	if r.BoardStatus != nil && *r.BoardStatus == piStateClosed {
		consider(r.LatestClosedEventOn)
	}
	return out
}

// dashboardWocSince is when an open issue began waiting on CS: the newest
// WAITING_ON_CS status event, else the clock's paused_on, else the link's last sync.
func dashboardWocSince(r domain.ProductIssueDashboardRow) *time.Time {
	switch {
	case r.LatestWOCEventOn != nil:
		return r.LatestWOCEventOn
	case r.Clock != nil && r.Clock.PausedOn != nil:
		return r.Clock.PausedOn
	default:
		return r.LastSyncedOn
	}
}

func evaluateDashboardIssue(r domain.ProductIssueDashboardRow, mappings []domain.ProductIssueDashboardMapping, now time.Time) dashIssue {
	d := dashIssue{row: r, owner: r.Owner, repository: r.Repository}
	if m := resolveDashboardProduct(r, mappings); m != nil {
		d.mapping = m
		name := m.ProductName
		d.productName = &name
		d.owner, d.repository = m.Owner, m.Repository
	}
	board := nonNilString(r.BoardStatus)
	d.state = productIssueState(r.GitHubState, board, r.LatestBoardEventOn, r.LatestReopenEventOn)
	d.open = d.state != piStateResolved && d.state != piStateClosed
	boardWaits := ""
	if d.state == board {
		boardWaits = r.BoardWaitsOn
	}
	d.waitsOn = productIssueWaitsOn(d.state, boardWaits)
	if d.open {
		if d.waitsOn == piWaitsCS && board == piStateWOC {
			d.wocSince = dashboardWocSince(r)
		}
	} else {
		d.resolvedOn = dashboardResolvedOn(r)
	}
	d.sla = computeProductIssueSLA(r.Clock, r.Priority, now)
	return d
}

func (d dashIssue) key() dashProductKey {
	if d.productName != nil {
		return dashProductKey{mapped: true, name: strings.ToLower(*d.productName)}
	}
	return dashProductKey{name: normRepoPart(d.row.Owner) + "/" + normRepoPart(d.row.Repository)}
}

func dashboardProductFilter(products []string) func(dashIssue) bool {
	want := map[string]bool{}
	for _, p := range products {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			want[p] = true
		}
	}
	if len(want) == 0 {
		return func(dashIssue) bool { return true }
	}
	return func(d dashIssue) bool {
		if d.productName == nil {
			return want[strings.ToLower(ProductIssueUnmappedName)]
		}
		return want[strings.ToLower(*d.productName)]
	}
}

// compliancePercentage is met / (met + breached) as a percentage with one decimal; nil
// when there is no outcome.
func compliancePercentage(met, breached int) *float64 {
	if met+breached == 0 {
		return nil
	}
	v := math.Round(float64(met)/float64(met+breached)*1000) / 10
	return &v
}

// dashboardOutcome is the SLA outcome of an issue in [start, end): its newest clock
// finished (ACHIEVED or COMPLETED) with end_on in the period. "" when it has none.
func dashboardOutcome(d dashIssue, start, end time.Time) string {
	c := d.row.Clock
	if c == nil || (c.Stage != "ACHIEVED" && c.Stage != "COMPLETED") || c.EndOn == nil {
		return ""
	}
	if c.EndOn.Before(start) || !c.EndOn.Before(end) {
		return ""
	}
	if d.sla.State == domain.ProductIssueSLAMet || d.sla.State == domain.ProductIssueSLABreached {
		return d.sla.State
	}
	return ""
}

func inPeriod(t *time.Time, start, end time.Time) bool {
	return t != nil && !t.Before(start) && t.Before(end)
}

// dashboardBucketStart is the first instant of the week (Monday) or month containing t, in loc.
func dashboardBucketStart(t time.Time, bucket string, loc *time.Location) time.Time {
	d := t.In(loc)
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	if bucket == DashboardBucketMonth {
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, loc)
	}
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

func dashboardBucketNext(start time.Time, bucket string) time.Time {
	if bucket == DashboardBucketMonth {
		return start.AddDate(0, 1, 0)
	}
	return start.AddDate(0, 0, 7)
}

// dashboardBuckets splits [start, end) into week or month buckets, the first and last
// clipped to the period.
func dashboardBuckets(start, end time.Time, bucket string, loc *time.Location) [][2]time.Time {
	out := [][2]time.Time{}
	for b := dashboardBucketStart(start, bucket, loc); b.Before(end); b = dashboardBucketNext(b, bucket) {
		s, e := b, dashboardBucketNext(b, bucket)
		if s.Before(start) {
			s = start
		}
		if e.After(end) {
			e = end
		}
		out = append(out, [2]time.Time{s, e})
	}
	return out
}

var dashboardAgeLabels = []string{"<1d", "1-3d", "3-7d", "7-14d", ">14d"}

// dashboardAgeBucket is the index in dashboardAgeLabels of an age in calendar time.
func dashboardAgeBucket(age time.Duration) int {
	day := 24 * time.Hour
	switch {
	case age < day:
		return 0
	case age < 3*day:
		return 1
	case age < 7*day:
		return 2
	case age < 14*day:
		return 3
	default:
		return 4
	}
}

func lessNameNullsLast(an *string, ao, ar string, bn *string, bo, br string) bool {
	if (an == nil) != (bn == nil) {
		return an != nil
	}
	if an != nil && !strings.EqualFold(*an, *bn) {
		return strings.ToLower(*an) < strings.ToLower(*bn)
	}
	if an != nil && *an != *bn {
		return *an < *bn
	}
	if !strings.EqualFold(ao, bo) {
		return strings.ToLower(ao) < strings.ToLower(bo)
	}
	if !strings.EqualFold(ar, br) {
		return strings.ToLower(ar) < strings.ToLower(br)
	}
	return ao+"/"+ar < bo+"/"+br
}

func (d dashIssue) item() domain.ProductIssueDashboardItem {
	it := domain.ProductIssueDashboardItem{
		ID: d.row.ID, Number: d.row.Number, Title: d.row.Title,
		ProductName: d.productName, Owner: d.owner, Repository: d.repository,
		IssueNumber: d.row.IssueNumber, HTMLURL: d.row.HTMLURL, BoardStatus: d.row.BoardStatus,
		Priority: d.row.Priority, OpenedOn: d.row.OpenedOn, WocSince: d.wocSince,
		OpenedByEmail: d.row.OpenedByEmail, SLA: d.sla,
	}
	if d.row.Case != nil {
		it.Case = &domain.ProductIssueCaseRef{ID: d.row.Case.ID, Number: d.row.Case.Number}
	}
	return it
}

type dashGroup struct {
	product domain.ProductIssueDashboardProduct
	cases   map[string]bool
}

// BuildProductIssueDashboard computes the dashboard from rows (contract section 1).
// Counts are over the filtered products; open, waiting-on, SLA state, ageing and
// blocked cases are as of Now, the rest use [Start, End).
func BuildProductIssueDashboard(in ProductIssueDashboardInput) domain.ProductIssueDashboard {
	loc := in.Loc
	if loc == nil {
		loc = time.UTC
	}
	keep := dashboardProductFilter(in.Products)
	var issues []dashIssue
	for _, r := range in.Rows {
		d := evaluateDashboardIssue(r, in.Mappings, in.Now)
		if keep(d) {
			issues = append(issues, d)
		}
	}

	buckets := dashboardBuckets(in.Start, in.End, in.Bucket, loc)
	out := domain.ProductIssueDashboard{
		PeriodStart: in.Start.In(loc), PeriodEnd: in.End.In(loc), GeneratedOn: in.Now.In(loc),
		Bucket: in.Bucket, Products: append([]string{}, in.Products...),
		Trend:     make([]domain.ProductIssueDashboardTrendPoint, len(buckets)),
		ByProduct: []domain.ProductIssueDashboardProduct{},
	}
	for i, b := range buckets {
		out.Trend[i] = domain.ProductIssueDashboardTrendPoint{Start: b[0].In(loc), End: b[1].In(loc)}
	}

	groups := map[dashProductKey]*dashGroup{}
	ageCounts := make([]int, len(dashboardAgeLabels))
	var woc, breached []dashIssue
	type blocked struct {
		caseRef *domain.ProductIssueDashboardCase
		issues  []dashIssue
	}
	blockedByCase := map[string]*blocked{}
	var blockedOrder []string

	for _, d := range issues {
		g := groups[d.key()]
		if g == nil {
			g = &dashGroup{cases: map[string]bool{}}
			g.product.ProductName, g.product.Owner, g.product.Repository = d.productName, d.owner, d.repository
			groups[d.key()] = g
		}
		t, p := &out.Totals, &g.product

		if inPeriod(&d.row.OpenedOn, in.Start, in.End) {
			t.OpenedInPeriod++
			p.OpenedInPeriod++
			for i, b := range buckets {
				if inPeriod(&d.row.OpenedOn, b[0], b[1]) {
					out.Trend[i].Opened++
					break
				}
			}
		}
		if inPeriod(d.resolvedOn, in.Start, in.End) {
			t.ResolvedInPeriod++
			p.ResolvedInPeriod++
			for i, b := range buckets {
				if inPeriod(d.resolvedOn, b[0], b[1]) {
					out.Trend[i].Resolved++
					break
				}
			}
		}
		switch dashboardOutcome(d, in.Start, in.End) {
		case domain.ProductIssueSLAMet:
			t.SLAMet++
			p.SLAMet++
		case domain.ProductIssueSLABreached:
			t.SLABreached++
			p.SLABreached++
		}

		if !d.open {
			continue
		}
		t.Open++
		p.Open++
		if d.row.Case == nil {
			t.UnmatchedOpen++
		}
		if d.row.BoardStatus != nil && *d.row.BoardStatus == "OTHER" {
			t.NonStandardColumnOpen++
		}
		// Open issues use the open-clock state only: a finished newest clock is "none".
		openState := d.sla.State
		if c := d.row.Clock; c != nil && (c.Stage == "ACHIEVED" || c.Stage == "COMPLETED") {
			openState = domain.ProductIssueSLANone
		}
		for _, c := range []*domain.ProductIssueDashboardSLACounts{&t.OpenBySLA, &p.OpenBySLA} {
			switch openState {
			case domain.ProductIssueSLAOK:
				c.OK++
			case domain.ProductIssueSLAAtRisk:
				c.AtRisk++
			case domain.ProductIssueSLABreached:
				c.Breached++
			case domain.ProductIssueSLAPaused:
				c.Paused++
			default:
				c.None++
			}
		}
		switch d.waitsOn {
		case piWaitsCS:
			t.WaitingOnCs++
			p.WaitingOnCs++
		case piWaitsProduct:
			t.WaitingOnProductTeam++
			p.WaitingOnProductTeam++
			if d.row.Case != nil && d.row.Case.Open {
				b := blockedByCase[d.row.Case.ID]
				if b == nil {
					b = &blocked{caseRef: d.row.Case}
					blockedByCase[d.row.Case.ID] = b
					blockedOrder = append(blockedOrder, d.row.Case.ID)
				}
				b.issues = append(b.issues, d)
				g.cases[d.row.Case.ID] = true
			}
		}
		if d.wocSince != nil {
			woc = append(woc, d)
			age := in.Now.Sub(*d.wocSince)
			ageCounts[dashboardAgeBucket(age)]++
		}
		if openState == domain.ProductIssueSLABreached {
			breached = append(breached, d)
		}
	}

	out.Totals.CompliancePercentage = compliancePercentage(out.Totals.SLAMet, out.Totals.SLABreached)
	for _, g := range groups {
		g.product.CompliancePercentage = compliancePercentage(g.product.SLAMet, g.product.SLABreached)
		out.ByProduct = append(out.ByProduct, g.product)
	}
	sort.Slice(out.ByProduct, func(i, j int) bool {
		a, b := out.ByProduct[i], out.ByProduct[j]
		return lessNameNullsLast(a.ProductName, a.Owner, a.Repository, b.ProductName, b.Owner, b.Repository)
	})

	out.WocAgeing.Buckets = make([]domain.ProductIssueDashboardAgeBucket, len(dashboardAgeLabels))
	for i, l := range dashboardAgeLabels {
		out.WocAgeing.Buckets[i] = domain.ProductIssueDashboardAgeBucket{Label: l, Count: ageCounts[i]}
	}
	sort.SliceStable(woc, func(i, j int) bool {
		if !woc[i].wocSince.Equal(*woc[j].wocSince) {
			return woc[i].wocSince.Before(*woc[j].wocSince)
		}
		return woc[i].row.Number < woc[j].row.Number
	})
	out.WocAgeing.Oldest = []domain.ProductIssueDashboardItem{}
	for i := 0; i < len(woc) && i < dashboardOldestLimit; i++ {
		out.WocAgeing.Oldest = append(out.WocAgeing.Oldest, woc[i].item())
	}

	sort.SliceStable(breached, func(i, j int) bool {
		if breached[i].sla.Percentage != breached[j].sla.Percentage {
			return breached[i].sla.Percentage > breached[j].sla.Percentage
		}
		return breached[i].row.Number < breached[j].row.Number
	})
	out.BreachedOpen = []domain.ProductIssueDashboardItem{}
	for i := 0; i < len(breached) && i < dashboardBreachedLimit; i++ {
		out.BreachedOpen = append(out.BreachedOpen, breached[i].item())
	}

	out.BlockedCases = buildDashboardBlocked(groups, blockedOrder, func(id string) ([]dashIssue, *domain.ProductIssueDashboardCase) {
		b := blockedByCase[id]
		return b.issues, b.caseRef
	})
	return out
}

func buildDashboardBlocked(groups map[dashProductKey]*dashGroup, order []string, lookup func(string) ([]dashIssue, *domain.ProductIssueDashboardCase)) domain.ProductIssueDashboardBlocked {
	res := domain.ProductIssueDashboardBlocked{
		Total: len(order), ByProduct: []domain.ProductIssueDashboardBlockedProduct{}, Cases: []domain.ProductIssueDashboardBlockedCase{},
	}
	for _, g := range groups {
		if len(g.cases) > 0 {
			res.ByProduct = append(res.ByProduct, domain.ProductIssueDashboardBlockedProduct{
				ProductName: g.product.ProductName, Owner: g.product.Owner, Repository: g.product.Repository, Cases: len(g.cases),
			})
		}
	}
	sort.Slice(res.ByProduct, func(i, j int) bool {
		a, b := res.ByProduct[i], res.ByProduct[j]
		return lessNameNullsLast(a.ProductName, a.Owner, a.Repository, b.ProductName, b.Owner, b.Repository)
	})
	for _, id := range order {
		issues, c := lookup(id)
		sort.SliceStable(issues, func(i, j int) bool {
			if !issues[i].row.OpenedOn.Equal(issues[j].row.OpenedOn) {
				return issues[i].row.OpenedOn.Before(issues[j].row.OpenedOn)
			}
			return issues[i].row.Number < issues[j].row.Number
		})
		bc := domain.ProductIssueDashboardBlockedCase{
			CaseID: c.ID, CaseNumber: c.Number, Subject: c.Subject,
			Issues: make([]domain.ProductIssueDashboardBlockingIssue, len(issues)), OldestOpenedOn: issues[0].row.OpenedOn,
		}
		for i, d := range issues {
			bc.Issues[i] = domain.ProductIssueDashboardBlockingIssue{
				ID: d.row.ID, Number: d.row.Number, Title: d.row.Title, ProductName: d.productName,
				BoardStatus: d.row.BoardStatus, OpenedOn: d.row.OpenedOn,
			}
		}
		res.Cases = append(res.Cases, bc)
	}
	sort.SliceStable(res.Cases, func(i, j int) bool {
		a, b := res.Cases[i], res.Cases[j]
		if !a.OldestOpenedOn.Equal(b.OldestOpenedOn) {
			return a.OldestOpenedOn.Before(b.OldestOpenedOn)
		}
		if a.CaseNumber != b.CaseNumber {
			return a.CaseNumber < b.CaseNumber
		}
		return a.CaseID < b.CaseID
	})
	if len(res.Cases) > dashboardBlockedLimit {
		res.Cases = res.Cases[:dashboardBlockedLimit]
	}
	return res
}

// BuildProductIssueRecipients groups lead emails: lower-cased, trimmed, blanks ignored,
// one entry per distinct email with its distinct product names, sorted by email, then name.
func BuildProductIssueRecipients(leads []domain.ProductIssueLead) domain.ProductIssueRecipients {
	byEmail := map[string]map[string]bool{}
	for _, l := range leads {
		e := strings.ToLower(strings.TrimSpace(l.Email))
		if e == "" {
			continue
		}
		if byEmail[e] == nil {
			byEmail[e] = map[string]bool{}
		}
		byEmail[e][l.ProductName] = true
	}
	out := domain.ProductIssueRecipients{Leads: []domain.ProductIssueRecipient{}}
	for e, names := range byEmail {
		r := domain.ProductIssueRecipient{Email: e, ProductNames: []string{}}
		for n := range names {
			r.ProductNames = append(r.ProductNames, n)
		}
		sort.Strings(r.ProductNames)
		out.Leads = append(out.Leads, r)
	}
	sort.Slice(out.Leads, func(i, j int) bool { return out.Leads[i].Email < out.Leads[j].Email })
	return out
}
