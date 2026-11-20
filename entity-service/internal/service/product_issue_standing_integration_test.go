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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The product issue SLA standing and the "mine" / board status search filters against
// PostgreSQL, using the policies migration 0307 seeds. Skipped without
// ENTITY_TEST_DATABASE_URL.

const (
	wocUserID = "77000000-0000-4000-8000-0000000000d0"
	wocA      = "77000000-0000-4000-8000-0000000000d1" // created by the caller, P1 paused
	wocB      = "77000000-0000-4000-8000-0000000000d2" // opened_by_email is the caller, P2 running
	wocC      = "77000000-0000-4000-8000-0000000000d3" // linked case assigned to the caller, no clock
	wocD      = "77000000-0000-4000-8000-0000000000d4" // nobody's, P1 met
	wocE      = "77000000-0000-4000-8000-0000000000d5" // unmatched, caller's, OTHER column
	wocMe     = "woc-me@wso2.com"
)

func TestProductIssueStandingIntegration(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	cleanup := func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM work_item WHERE id IN ($1,$2,$3,$4,$5)`, wocA, wocB, wocC, wocD, wocE)
		_, _ = pool.Exec(bg, `UPDATE work_item SET assigned_to_id = NULL WHERE id = $1`, pwiCaseID)
		_, _ = pool.Exec(bg, `DELETE FROM "user" WHERE id = $1`, wocUserID)
	}
	cleanup()
	t.Cleanup(cleanup)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec (%.70s): %v", sql, err)
		}
	}
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC) // a Wednesday
	issue := func(id, num, createdBy, openedBy, priority, board string, gh int, linkCase bool) {
		t.Helper()
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		      VALUES ($1::uuid, $2, $2, $3, $3, $4, $4, 'PRODUCT_ISSUE')`, id, now.Add(-24*time.Hour), createdBy, num)
		var pr, ob, bs *string
		if priority != "" {
			pr = &priority
		}
		if openedBy != "" {
			ob = &openedBy
		}
		if board != "" {
			bs = &board
		}
		exec(`INSERT INTO product_issue (id, priority, opened_by_email) VALUES ($1::uuid, $2, $3)`, id, pr, ob)
		exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, github_created_on, board_status, board_status_raw)
		      VALUES ($1::uuid, 'woc-org', 'woc-repo', $2, $3, $4::text, CASE WHEN $4::text = 'OTHER' THEN 'Triage?' ELSE NULL END)`, id, gh, now.Add(-24*time.Hour), bs)
		if linkCase {
			exec(`INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
			      VALUES ($1::uuid, $2::uuid, 'AFFECTS', 'MANUAL', 'test', 'test')`, id, pwiCaseID)
		}
	}
	clock := func(id, policy, stage string, start time.Time, paused, end *time.Time, bpause string, breached bool, created time.Time) {
		t.Helper()
		exec(`INSERT INTO sla (created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, schedule, is_active, stage, has_breached,
		        start_on, duration, end_on, paused_on, business_pause_duration, business_elapsed_percentage)
		      SELECT $9, $9, 'test', 'test', $1::uuid, sp.id, sp.schedule, $3 <> 'ACHIEVED', $3::sla_stage_enum, $8, $4, sp.duration, $6, $5, $7::interval, 7
		        FROM sla_policy sp WHERE sp.name = $2 AND sp.applies_to = 'PRODUCT_ISSUE'`, id, policy, stage, start, paused, end, bpause, breached, created)
	}
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active) VALUES ($1::uuid, NOW(), NOW(), 'woc-me', $2, true)`, wocUserID, strings.ToUpper(wocMe))
	exec(`UPDATE work_item SET assigned_to_id = $1::uuid WHERE id = $2::uuid`, wocUserID, pwiCaseID)

	issue(wocA, "WOC-PI-A", wocMe, "", "P1", "WAITING_ON_CS", 9101, false)
	issue(wocB, "WOC-PI-B", "someone@wso2.com", "WOC-Me@wso2.com", "P2", "IN_PROGRESS", 9102, false)
	issue(wocC, "WOC-PI-C", "someone@wso2.com", "", "", "WAITING_ON_CS", 9103, true) // case assigned to the caller
	issue(wocD, "WOC-PI-D", "other@wso2.com", "other@wso2.com", "P1", "WAITING_ON_CS", 9104, false)
	issue(wocE, "WOC-PI-E", wocMe, "", "P3", "OTHER", 9105, false)

	// A: P1 24x7 paused since 10:00 after 10h; an older ACHIEVED row must lose to the open one.
	start := now.Add(-30 * time.Hour) // Tue 06:00Z
	pausedOn := now.Add(-4 * time.Hour)
	clock(wocA, "Product Issue P1", "ACHIEVED", start.Add(-72*time.Hour), nil, &start, "0", false, now.Add(-100*time.Hour))
	clock(wocA, "Product Issue P1", "PAUSED", start, &pausedOn, nil, "2 hours", false, now.Add(-29*time.Hour))
	// B: P2 12x5 IST running since Wed 03:30Z (09:00 IST), now 12:00Z = 17:30 IST: 8.5h of 24h.
	bStart := time.Date(2026, 11, 18, 3, 30, 0, 0, time.UTC)
	clock(wocB, "Product Issue P2", "IN_PROGRESS", bStart, nil, nil, "0", false, bStart)
	// D: P1 met after 6h.
	dEnd := now.Add(-20 * time.Hour)
	clock(wocD, "Product Issue P1", "ACHIEVED", now.Add(-26*time.Hour), nil, &dEnd, "0", false, now.Add(-26*time.Hour))
	// A newer CANCELLED row (a priority change) never wins over the met clock.
	clock(wocD, "Product Issue P2", "CANCELLED", now.Add(-20*time.Hour), nil, &dEnd, "0", false, now.Add(-20*time.Hour))
	// The budget is the policy's duration: a wrong stored sla.duration (migration 0272) is ignored.
	exec(`UPDATE sla SET duration = interval '1 hour' WHERE work_item_id = $1::uuid`, wocB)

	svc := NewProductIssueService(&fakePWEng{}, repository.NewProductIssueRepository(repository.NewScoped(pool)), internalAccess{}).(*productIssueService)
	svc.now = func() time.Time { return now }
	sctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: wocMe})

	// --- single read ---
	d, err := svc.Get(sctx, wocA)
	if err != nil {
		t.Fatal(err)
	}
	// 24x7: start - 30h before now; paused 4h before now: 26h wall - 2h paused = 24h? budget 24h -> breached by consumption? No: paused clock stops at pausedOn.
	// start..pausedOn = 26h; minus 2h business pause = 24h = budget -> BREACHED by computation.
	if d.SLA.State != "BREACHED" || d.SLA.ConsumedSeconds != 24*3600 || d.SLA.RemainingSeconds != 0 || d.SLA.Running ||
		d.SLA.PausedSince == nil || !d.SLA.PausedSince.Equal(pausedOn) || d.SLA.PolicyName == nil || *d.SLA.PolicyName != "Product Issue P1" ||
		d.SLA.Priority == nil || *d.SLA.Priority != "P1" || d.SLA.BudgetSeconds != 24*3600 {
		t.Errorf("A standing = %+v", d.SLA)
	}
	if d.NonStandardColumn {
		t.Error("A is not on a non-standard column")
	}
	e, err := svc.Get(sctx, wocE)
	if err != nil || !e.NonStandardColumn || e.SLA.State != "NONE" || e.BoardStatusRaw == nil || *e.BoardStatusRaw != "Triage?" {
		t.Errorf("E = %+v %v", e, err)
	}

	// --- case list: one case holds C only; link B and D to it for the list read ---
	exec(`INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by) VALUES ($1::uuid, $3::uuid, 'AFFECTS', 'MANUAL', 'test', 'test'), ($2::uuid, $3::uuid, 'AFFECTS', 'MANUAL', 'test', 'test')`, wocB, wocD, pwiCaseID)
	list, err := svc.ListForCase(sctx, pwiCaseID)
	if err != nil || len(list.Items) != 3 {
		t.Fatalf("list: %+v %v", list, err)
	}
	by := map[string]domain.ProductIssueSummary{}
	for _, it := range list.Items {
		by[it.ID] = it
	}
	if b := by[wocB].SLA; b.State != "OK" || b.ConsumedSeconds != 8*3600+30*60 || !b.Running || b.PausedSince != nil || b.Percentage != 35.42 ||
		b.Schedule == nil || *b.Schedule != "CS 12x5 IST" {
		t.Errorf("B standing = %+v", b)
	}
	if c := by[wocC].SLA; c.State != "NONE" || c.Priority != nil {
		t.Errorf("C standing = %+v", c)
	}
	if dd := by[wocD].SLA; dd.State != "MET" || dd.ConsumedSeconds != 6*3600 || dd.Running {
		t.Errorf("D standing = %+v", dd)
	}
	exec(`UPDATE work_item_link SET is_active = false WHERE from_id IN ($1::uuid, $2::uuid)`, wocB, wocD)

	// --- search: mine + boardStatuses ---
	search := func(ctx context.Context, f domain.SearchProductIssuesFilters) (domain.SearchProductIssuesResponse, error) {
		f.Owner = "woc-org"
		return svc.Search(ctx, domain.SearchProductIssuesRequest{Filters: f})
	}
	ids := func(r domain.SearchProductIssuesResponse) string {
		var s []string
		for _, it := range r.Items {
			s = append(s, strings.TrimPrefix(it.Number, "WOC-PI-"))
		}
		return strings.Join(s, ",")
	}
	r, err := search(sctx, domain.SearchProductIssuesFilters{Mine: true})
	if err != nil || r.Total != 4 {
		t.Fatalf("mine: %v total=%d ids=%s", err, r.Total, ids(r))
	}
	// default order for mine: oldest paused first, unpaused after (then newest created).
	if got := ids(r); !strings.HasPrefix(got, "A,") {
		t.Errorf("mine order = %s, want the paused issue first", got)
	}
	if r, err = search(sctx, domain.SearchProductIssuesFilters{Mine: true, BoardStatuses: []string{"WAITING_ON_CS"}}); err != nil || ids(r) != "A,C" {
		t.Errorf("mine + WAITING_ON_CS = %s %v (creator A, case assignee C; B opened-by is IN_PROGRESS)", ids(r), err)
	}
	if r, err = search(sctx, domain.SearchProductIssuesFilters{BoardStatuses: []string{"WAITING_ON_CS"}}); err != nil || r.Total != 3 {
		t.Errorf("WAITING_ON_CS without mine = %d %v", r.Total, err)
	}
	// opened_by_email match alone (case-insensitive) and the assignee match alone.
	if r, err = search(sctx, domain.SearchProductIssuesFilters{Mine: true, BoardStatuses: []string{"IN_PROGRESS"}}); err != nil || ids(r) != "B" {
		t.Errorf("opened_by match = %s %v", ids(r), err)
	}
	// The "nobody's" issue D never matches; a stranger gets nothing.
	other := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "stranger@wso2.com"})
	if r, err = search(other, domain.SearchProductIssuesFilters{Mine: true}); err != nil || r.Total != 0 {
		t.Errorf("stranger = %d %v", r.Total, err)
	}
	// An inactive link stops the assignee match (C's link stays active; B/D were deactivated above).
	exec(`UPDATE work_item_link SET is_active = false WHERE from_id = $1::uuid`, wocC)
	if r, err = search(sctx, domain.SearchProductIssuesFilters{Mine: true, BoardStatuses: []string{"WAITING_ON_CS"}}); err != nil || ids(r) != "A" {
		t.Errorf("inactive link = %s %v", ids(r), err)
	}
	// Standing in search results; sort by percentage.
	if r, err = search(sctx, domain.SearchProductIssuesFilters{Sort: "slaPercentage"}); err != nil || r.Total != 5 || r.Items[0].SLA.State == "" {
		t.Errorf("sorted by percentage: %v total=%d", err, r.Total)
	}
	for _, it := range r.Items {
		if it.SLA.State == "" {
			t.Errorf("%s has no standing state", it.Number)
		}
	}
	// No user: 400.
	var ve *apierror.ValidationError
	anon := auth.WithIdentity(context.Background(), auth.Identity{Validated: true})
	if _, err = search(anon, domain.SearchProductIssuesFilters{Mine: true}); !errors.As(err, &ve) {
		t.Errorf("mine without a user: %v", err)
	}
	// Without mine the request needs no user and keeps the newest-first order.
	if r, err = search(anon, domain.SearchProductIssuesFilters{}); err != nil || r.Total != 5 {
		t.Errorf("no filters = %d %v", r.Total, err)
	}
}
