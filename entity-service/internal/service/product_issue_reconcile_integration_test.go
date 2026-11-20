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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Reconciliation and backfill end to end against PostgreSQL: the real
// repositories, a scripted engineering entity. Skipped without
// ENTITY_TEST_DATABASE_URL; the rows it leaves are removed.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run ProductIssueReconcile

const (
	rcOwner   = "rcn-org"
	rcRepo    = "rcn-repo"
	rcBoard   = 77
	rcProduct = "RCN Test Product"
)

type rcEnv struct {
	*pwhEnv
	repo repository.ProductIssueReconcileRepository
}

func rcSeed(t *testing.T) *rcEnv {
	t.Helper()
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	clean := func() {
		bg := context.Background()
		// Unmatched product issues have no account, so pwiSeed's cleanup does not reach them.
		_, _ = pool.Exec(bg, `DELETE FROM work_item WHERE id IN (SELECT work_item_id FROM github_issue_link WHERE lower(owner) = $1)`, rcOwner)
		_, _ = pool.Exec(bg, `DELETE FROM product_repo_mapping WHERE product_name = $1`, rcProduct)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO product_repo_mapping (created_by, updated_by, product_name, abbreviation, owner, repository, github_label, board_project_number)
		   VALUES ('test', 'test', '` + rcProduct + `', 'rcn-abbr', '` + rcOwner + `', '` + rcRepo + `', 'product/rcn', ` + fmt.Sprint(rcBoard) + `)`,
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id)
		   VALUES ('` + pwiCase2ID + `', NOW(), NOW(), 'test', 'test', 'PWI-CASE-2', 'PWI-WSO2-2', 'pwi case 2', 'CASE', '` + pwiAccountID + `', '` + pwiProjectID + `')`,
		`INSERT INTO "case" (id, autoclose_enabled, state) VALUES ('` + pwiCase2ID + `', false, 'OPEN')`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("seed (%.60s): %v", q, err)
		}
	}
	return &rcEnv{
		pwhEnv: &pwhEnv{t: t, pool: pool, ctx: ctx},
		repo:   repository.NewProductIssueReconcileRepository(repository.NewScoped(pool)),
	}
}

func (e *rcEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatalf("exec (%.70s): %v", sql, err)
	}
}

// product issue linked to the first case, as a webhook or the create path leaves it.
func (e *rcEnv) mkLinked(n int, title, rawStatus string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(e.ctx, `
		INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id)
		VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number('PRODUCT_ISSUE'), $1, 'PRODUCT_ISSUE', $2, $3) RETURNING id::TEXT`,
		title, pwiAccountID, pwiProjectID).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO product_issue (id) VALUES ($1::uuid)`, id)
	e.exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id, github_updated_on) VALUES ($1::uuid, $2, $3, $4, $5, '2026-11-01T10:00:00Z')`,
		id, rcOwner, rcRepo, n, fmt.Sprintf("I_rc%d", n))
	e.exec(`INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by) VALUES ($1::uuid, $2::uuid, 'AFFECTS', 'MANUAL', 'test', 'test')`, id, pwiCaseID)
	if rawStatus != "" {
		e.exec(`UPDATE github_issue_link SET board_status_raw = $2, board_status = 'OPEN', board_project_number = $3 WHERE work_item_id = $1::uuid`, id, rawStatus, rcBoard)
	}
	return id
}

func (e *rcEnv) seedEvent(wi, raw, key string, at time.Time) {
	e.t.Helper()
	e.exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
		VALUES ($1::uuid, 'OPEN', 'PRODUCT_TEAM', $2, 'WEBHOOK', $3, $4)`, wi, at, key, raw)
}

func (e *rcEnv) linkSnapshot(wi string) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(e.ctx, `
		SELECT concat_ws('|', l.github_state, l.state_reason, l.labels::text, l.board_status_raw, l.board_status, l.link_state,
		                 l.updated_on::text, l.github_updated_on::text, wi.subject, wi.updated_on::text, coalesce(pi.priority, ''), coalesce(pi.category, ''))
		  FROM github_issue_link l JOIN work_item wi ON wi.id = l.work_item_id JOIN product_issue pi ON pi.id = l.work_item_id
		 WHERE l.work_item_id = $1::uuid`, wi).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *rcEnv) service(eng *fakeSyncEng) *productIssueSync {
	clock := &fakeSyncClock{t: time.Now()}
	if eng.clock == nil {
		eng.clock = clock
	}
	return &productIssueSync{eng: eng, repo: e.repo, now: time.Now, reserve: 15 * time.Second, defaultBudget: 30 * time.Second}
}

func TestProductIssueReconcileIntegration(t *testing.T) {
	e := rcSeed(t)
	a := e.mkLinked(1, "seed A", "Open")
	b := e.mkLinked(2, "seed B", "")
	c := e.mkLinked(3, "seed C", "")
	d := e.mkLinked(4, "seed D", "Open")
	ee := e.mkLinked(5, "seed E", "Open")
	f := e.mkLinked(6, "seed F", "Open")
	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	e.seedEvent(a, "Open", "wh-a0", t0)
	tE := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	e.seedEvent(ee, "WOC", "wh-e1", tE)
	e.exec(`UPDATE github_issue_link SET labels = '["Origin/CS"]'::jsonb WHERE work_item_id = $1::uuid`, d)

	item := func(n int, title, status string, labels ...string) engineeringentity.ProjectItem {
		it := boardItem(fmt.Sprintf("I_rc%d", n), n, status)
		it.Content.Repository, it.Content.Title, it.Content.Labels = rcOwner+"/"+rcRepo, title, labels
		it.Content.UpdatedAt = "2026-11-10T10:00:00Z"
		return it
	}
	dItem := item(4, "seed D", "Open", "Origin/CS")
	dItem.Content.UpdatedAt = "2026-11-01T10:00:00Z" // what the link already holds
	var itemCalls []string
	eng := &fakeSyncEng{}
	eng.items = func(org string, board int, after string) (engineeringentity.ProjectItemsPage, error) {
		itemCalls = append(itemCalls, fmt.Sprintf("%s/%d", org, board))
		if board != rcBoard {
			return engineeringentity.ProjectItemsPage{}, nil // a board left by another fixture
		}
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{
			item(1, "renamed A", "WOC", "Origin/CS", "Priority/High(P2)", "Type/Incident"), dItem,
			item(5, "seed E", "WOC", "Origin/CS"), item(6, "seed F", "Resolved", "Origin/CS"),
			item(99, "not linked", "Open"),
		}}, nil
	}
	var historyFor []int
	eng.history = func(n int) ([]engineeringentity.ProjectStatusHistory, error) {
		historyFor = append(historyFor, n)
		switch n {
		case 1:
			return []engineeringentity.ProjectStatusHistory{{ProjectNumber: rcBoard, CurrentStatus: strp("WOC"), Changes: []engineeringentity.ProjectStatusChange{
				{EventNodeID: "e-a0", NewStatus: strp("Open"), OccurredAt: "2026-11-01T10:00:01Z"}, // the webhook's event, a second later
				{EventNodeID: "e-a1", PreviousStatus: strp("Open"), NewStatus: strp("WOC"), OccurredAt: "2026-11-05T10:00:00Z", ActorLogin: strp("octo")},
			}}}, nil
		case 5:
			return []engineeringentity.ProjectStatusHistory{{ProjectNumber: rcBoard, CurrentStatus: strp("WOC"), Changes: []engineeringentity.ProjectStatusChange{
				{EventNodeID: "e-e1", PreviousStatus: strp("Open"), NewStatus: strp("WOC"), OccurredAt: tE.Add(time.Second).Format(time.RFC3339)},
			}}}, nil
		case 6:
			return []engineeringentity.ProjectStatusHistory{{ProjectNumber: rcBoard, CurrentStatus: strp("Resolved")}}, nil // the history lacks the change
		}
		return nil, nil
	}
	eng.detail = func(_, _ string, n int) (engineeringentity.IssueDetail, error) {
		switch n {
		case 2:
			return engineeringentity.IssueDetail{Number: 2, Title: "B closed", State: "closed", StateReason: strp("not_planned"),
				Labels: []string{"Origin/CS", "Priority/Medium"}, UpdatedAt: "2026-11-09T10:00:00Z", ClosedAt: strp("2026-11-09T10:00:00Z")}, nil
		case 3:
			return engineeringentity.IssueDetail{}, &engineeringentity.Error{StatusCode: 404}
		}
		return engineeringentity.IssueDetail{}, &engineeringentity.Error{StatusCode: 500}
	}
	svc := e.service(eng)

	beforeD := e.linkSnapshot(d)
	resp, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ItemsChecked != 4 || resp.LinksUpdated != 5 || resp.StatusEventsWritten != 2 || resp.NotOnBoard != 2 || resp.Failed != 0 || resp.More || resp.NextCursor != "" {
		t.Fatalf("run 1 = %+v", resp)
	}
	if fmt.Sprint(historyFor) != "[1 5 6]" {
		t.Errorf("board history was read for %v: only a link whose status differs needs it", historyFor)
	}

	// A: the missing move recorded once (the webhook's own event is not duplicated).
	evA := e.events(a)
	if len(evA) != 2 || evA[1].Key != "e-a1" || evA[1].Source != "RECONCILE" || evA[1].From != "OPEN" || evA[1].To != "WAITING_ON_CS" ||
		evA[1].Raw != "WOC" || evA[1].WaitsOn != "CS" || !evA[1].At.Equal(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("A events = %+v", evA)
	}
	la := e.link(a)
	if la.Raw != "WOC" || la.Status != "WAITING_ON_CS" || la.Board == nil || *la.Board != rcBoard || e.subject(a) != "renamed A" {
		t.Errorf("A link = %+v subject=%q", la, e.subject(a))
	}
	if p, cat := e.product(a); p != "P2" || cat != "Type/Incident" {
		t.Errorf("A priority/category = %q %q", p, cat)
	}
	// E: the same move under a webhook key and a timeline key a second apart is one event.
	if ev := e.events(ee); len(ev) != 1 || ev[0].Key != "wh-e1" {
		t.Errorf("E events = %+v", ev)
	}
	if l := e.link(ee); l.Raw != "WOC" || l.Status != "WAITING_ON_CS" {
		t.Errorf("E link = %+v", l)
	}
	// F: the history lacks the move: one synthetic event stamped now.
	evF := e.events(f)
	wantKey := fmt.Sprintf("reconcile:%s:RESOLVED:%s", f, time.Now().UTC().Format("2006-01-02"))
	if len(evF) != 1 || evF[0].Key != wantKey || evF[0].Source != "RECONCILE" || evF[0].From != "OPEN" || evF[0].To != "RESOLVED" || evF[0].Raw != "Resolved" ||
		time.Since(evF[0].At) > time.Minute {
		t.Errorf("F events = %+v (want key %s)", evF, wantKey)
	}
	// D: nothing to change on the issue; only the marker moved.
	if e.linkSnapshot(d) != beforeD {
		t.Errorf("D changed: %q -> %q", beforeD, e.linkSnapshot(d))
	}
	if n := e.count(`SELECT count(*) FROM github_issue_link WHERE work_item_id = $1::uuid AND last_synced_on IS NOT NULL`, d); n != 1 {
		t.Error("a link seen on the board is marked synced")
	}
	// B was missing from the board: refreshed from the issue. C is gone: inaccessible.
	if l := e.link(b); l.State != "CLOSED" || l.Reason != "not_planned" || l.LinkState != "ACTIVE" || l.Closed == nil || !strings.Contains(l.Labels, "Priority/Medium") {
		t.Errorf("B link = %+v", l)
	}
	if p, _ := e.product(b); p != "P3" {
		t.Errorf("B priority = %q", p)
	}
	if e.link(c).LinkState != "INACCESSIBLE" {
		t.Errorf("C link = %+v", e.link(c))
	}
	if n := e.count(`SELECT count(*) FROM work_item_status_event WHERE work_item_id = ANY($1::uuid[])`, []string{b, c, d}); n != 0 {
		t.Errorf("B, C and D got %d status events", n)
	}

	// Second run, nothing changed upstream: writes nothing.
	snaps := map[string]string{}
	for _, id := range []string{a, b, c, d, ee, f} {
		snaps[id] = e.linkSnapshot(id)
	}
	events := e.count(`SELECT count(*) FROM work_item_status_event WHERE work_item_id = ANY($1::uuid[])`, []string{a, b, c, d, ee, f})
	historyFor = nil
	resp, err = svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ItemsChecked != 4 || resp.LinksUpdated != 0 || resp.StatusEventsWritten != 0 || resp.NotOnBoard != 0 || resp.Failed != 0 || resp.More {
		t.Fatalf("run 2 = %+v", resp)
	}
	if len(historyFor) != 0 {
		t.Errorf("run 2 read history for %v", historyFor)
	}
	for id, was := range snaps {
		if now := e.linkSnapshot(id); now != was {
			t.Errorf("run 2 changed %s: %q -> %q", id, was, now)
		}
	}
	if n := e.count(`SELECT count(*) FROM work_item_status_event WHERE work_item_id = ANY($1::uuid[])`, []string{a, b, c, d, ee, f}); n != events {
		t.Errorf("run 2 wrote %d events", n-events)
	}
}

func TestProductIssueReconcileIntegration_OpenLinkMissingFromTheBoardIsCheckedEveryRun(t *testing.T) {
	e := rcSeed(t)
	b := e.mkLinked(2, "seed B", "")
	eng := &fakeSyncEng{}
	details := 0
	eng.detail = func(_, _ string, n int) (engineeringentity.IssueDetail, error) {
		details++
		return engineeringentity.IssueDetail{Number: n, Title: "seed B", State: "open", Labels: []string{"Origin/CS"}, UpdatedAt: "2026-11-09T10:00:00Z"}, nil
	}
	svc := e.service(eng)
	for run := 1; run <= 2; run++ {
		resp, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{})
		if err != nil || resp.NotOnBoard != 1 || resp.More {
			t.Fatalf("run %d = %+v err=%v", run, resp, err)
		}
		if run == 2 && resp.LinksUpdated != 0 {
			t.Errorf("run 2 updated %d links", resp.LinksUpdated)
		}
	}
	if details != 2 {
		t.Errorf("issue reads = %d", details)
	}
	// An older read of the issue than the stored one does not overwrite it.
	eng.detail = func(_, _ string, n int) (engineeringentity.IssueDetail, error) {
		return engineeringentity.IssueDetail{Number: n, Title: "stale title", State: "closed", UpdatedAt: "2026-10-01T10:00:00Z"}, nil
	}
	if _, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{}); err != nil {
		t.Fatal(err)
	}
	if e.subject(b) != "seed B" || e.link(b).State != "OPEN" {
		t.Errorf("a stale read overwrote the issue: %q %+v", e.subject(b), e.link(b))
	}
}

// --- backfill ---

func rcBackfillEng(comments map[int][]engineeringentity.IssueComment) *fakeSyncEng {
	eng := &fakeSyncEng{}
	pr := issue(109)
	pr.HTMLURL = "https://github.com/" + rcOwner + "/" + rcRepo + "/pull/109"
	closedAt := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	oldAt := time.Now().UTC().AddDate(0, 0, -400).Format(time.RFC3339)
	closedRecent, closedOld := issue(107), issue(108)
	closedRecent.State, closedRecent.ClosedAt, closedRecent.UpdatedAt = "closed", &closedAt, closedAt
	closedRecent.StateReason = strp("completed")
	closedOld.State, closedOld.ClosedAt, closedOld.UpdatedAt = "closed", &oldAt, oldAt
	closedRecent = withBody(closedRecent, "WSO2 case id : PWI-WSO2-1\nCase number : PWI-CASE-1")
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.Page > 1 {
			return nil, nil
		}
		if q.State == "closed" {
			return []engineeringentity.IssueDetail{closedRecent, closedOld}, nil
		}
		return []engineeringentity.IssueDetail{
			withBody(issue(101, "Priority/High(P2)", "Type/Query"), "Details\n\nWSO2 case id : PWI-WSO2-1\nCase number : PWI-CASE-1\nABT Team : Team A\nOpened by : eng@example.test"),
			withBody(issue(102), "WSO2 case id : PWI-WSO2-1\nCase number : PWI-CASE-2"),
			withBody(issue(103), "WSO2 case id : PWI-WSO2-2\nCase number : PWI-CASE-2"),
			withBody(issue(104), "Case number : PWI-CASE-2"), // one footer line: unmatched
			issue(105, "Type/Patch"), issue(106, "patch"), pr,
		}, nil
	}
	eng.comments = func(n, page int) ([]engineeringentity.IssueComment, error) {
		if page > 1 {
			return nil, nil
		}
		return comments[n], nil
	}
	eng.history = func(n int) ([]engineeringentity.ProjectStatusHistory, error) {
		if n != 101 {
			return nil, nil
		}
		return []engineeringentity.ProjectStatusHistory{
			{ProjectNumber: 5, CurrentStatus: strp("Open")}, // another board of another product
			{ProjectNumber: rcBoard, CurrentStatus: strp("WOC"), Changes: []engineeringentity.ProjectStatusChange{
				{EventNodeID: "bf-e1", NewStatus: strp("Open"), OccurredAt: "2026-11-01T10:00:01Z", ActorLogin: strp("github-project-automation")},
				{EventNodeID: "bf-e2", PreviousStatus: strp("Open"), NewStatus: strp("WOC"), OccurredAt: "2026-11-03T10:00:00Z", ActorLogin: strp("octo")},
			}}}, nil
	}
	return eng
}

func TestProductIssueBackfillIntegration(t *testing.T) {
	e := rcSeed(t)
	comments := map[int][]engineeringentity.IssueComment{101: {
		{ID: 880001, Body: "first", AuthorLogin: strp("octo"), CreatedAt: "2026-11-02T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z"},
		{ID: 880002, Body: "second", AuthorLogin: strp("hubot"), CreatedAt: "2026-11-02T11:00:00Z", UpdatedAt: "2026-11-02T11:00:00Z"},
	}}
	eng := rcBackfillEng(comments)
	svc := e.service(eng)
	req := domain.ProductIssueBackfillRequest{Owner: strings.ToUpper(rcOwner), Repository: rcRepo, DryRun: true, MaxIssues: 100}
	links := func() int { return e.count(`SELECT count(*) FROM github_issue_link WHERE lower(owner) = $1`, rcOwner) }

	// Dry run: counts only.
	resp, err := svc.Backfill(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Considered != 8 || resp.WouldCreateMatched != 3 || resp.WouldCreateUnmatched != 2 || resp.AlreadyKnown != 0 || resp.SkippedPatch != 2 || resp.SkippedPullRequest != 1 || resp.More {
		t.Fatalf("dry run = %+v", resp)
	}
	if links() != 0 || e.count(`SELECT count(*) FROM work_item WHERE created_by = $1`, backfillActor) != 0 {
		t.Fatal("a dry run wrote")
	}
	for _, c := range eng.calls {
		if c != "list" {
			t.Fatalf("a dry run called the engineering entity for %q", c)
		}
	}

	// A real run in small calls.
	req.DryRun, req.MaxIssues = false, 3
	var total domain.ProductIssueBackfillResponse
	calls := 0
	for {
		resp, err = svc.Backfill(e.ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		calls++
		total.Considered += resp.Considered
		total.CreatedMatched += resp.CreatedMatched
		total.CreatedUnmatched += resp.CreatedUnmatched
		total.CommentsWritten += resp.CommentsWritten
		total.StatusEventsWritten += resp.StatusEventsWritten
		total.SkippedPatch += resp.SkippedPatch
		total.SkippedPullRequest += resp.SkippedPullRequest
		total.Failed += resp.Failed
		if !resp.More {
			break
		}
		if calls > 10 {
			t.Fatal("the run does not end")
		}
		req.Cursor = resp.NextCursor
	}
	if total.Considered != 8 || total.CreatedMatched != 3 || total.CreatedUnmatched != 2 || total.CommentsWritten != 2 || total.StatusEventsWritten != 2 ||
		total.SkippedPatch != 2 || total.SkippedPullRequest != 1 || total.Failed != 0 || calls < 3 {
		t.Fatalf("real run = %+v in %d calls", total, calls)
	}

	byIssue := func(n int) string {
		var id string
		if err := e.pool.QueryRow(e.ctx, `SELECT work_item_id::TEXT FROM github_issue_link WHERE lower(owner) = $1 AND issue_number = $2`, rcOwner, n).Scan(&id); err != nil {
			t.Fatalf("issue %d: %v", n, err)
		}
		return id
	}
	if links() != 5 {
		t.Fatalf("links = %d, want 5 (patches, the pull request and the old closed issue are not registered)", links())
	}
	// Matched: the case's account and project, an AFFECTS link with source BACKFILL.
	for n, wantCase := range map[int]string{101: pwiCaseID, 103: pwiCase2ID, 107: pwiCaseID} {
		id := byIssue(n)
		var typ string
		var acc, proj *string
		if err := e.pool.QueryRow(e.ctx, `SELECT type::TEXT, account_id::TEXT, project_id::TEXT FROM work_item WHERE id = $1::uuid`, id).Scan(&typ, &acc, &proj); err != nil {
			t.Fatal(err)
		}
		if typ != "PRODUCT_ISSUE" || acc == nil || *acc != pwiAccountID || proj == nil || *proj != pwiProjectID {
			t.Errorf("#%d work item = %s %v %v", n, typ, acc, proj)
		}
		var to, source string
		if err := e.pool.QueryRow(e.ctx, `SELECT to_id::TEXT, source FROM work_item_link WHERE from_id = $1::uuid AND is_active AND kind = 'AFFECTS'`, id).Scan(&to, &source); err != nil || to != wantCase || source != "BACKFILL" {
			t.Errorf("#%d AFFECTS link = %s %s %v", n, to, source, err)
		}
	}
	// Unmatched: no account, no project, no case link; listed as unmatched.
	for _, n := range []int{102, 104} {
		id := byIssue(n)
		var acc, proj *string
		if err := e.pool.QueryRow(e.ctx, `SELECT account_id::TEXT, project_id::TEXT FROM work_item WHERE id = $1::uuid`, id).Scan(&acc, &proj); err != nil || acc != nil || proj != nil {
			t.Errorf("#%d account/project = %v %v %v", n, acc, proj, err)
		}
		if k := e.count(`SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid`, id); k != 0 {
			t.Errorf("#%d has %d case links", n, k)
		}
	}
	found, total2, err := repository.NewProductIssueRepository(repository.NewScoped(e.pool)).Search(e.ctx,
		domain.SearchProductIssuesFilters{Unmatched: true, Owner: rcOwner, Repository: rcRepo}, domain.Pagination{Limit: 50})
	if err != nil || total2 != 2 || len(found) != 2 {
		t.Errorf("unmatched search = %d/%d %v", len(found), total2, err)
	}

	// The first issue in full.
	id := byIssue(101)
	var email, abt *string
	var userID *string
	if err := e.pool.QueryRow(e.ctx, `SELECT opened_by_email, abt_team, opened_by_user_id::TEXT FROM product_issue WHERE id = $1::uuid`, id).Scan(&email, &abt, &userID); err != nil {
		t.Fatal(err)
	}
	if email == nil || *email != "eng@example.test" || abt == nil || *abt != "Team A" || userID != nil {
		t.Errorf("footer fields = %v %v %v", email, abt, userID)
	}
	if p, cat := e.product(id); p != "P2" || cat != "Type/Query" {
		t.Errorf("priority/category = %q %q", p, cat)
	}
	l := e.link(id)
	if l.State != "OPEN" || l.LinkState != "ACTIVE" || l.Raw != "WOC" || l.Status != "WAITING_ON_CS" || l.Board == nil || *l.Board != rcBoard || !strings.Contains(l.Labels, "Type/Query") {
		t.Errorf("link = %+v", l)
	}
	if ev := e.events(id); len(ev) != 2 || ev[0].Key != "bf-e1" || ev[0].Source != "BACKFILL" || ev[0].From != "" || ev[0].To != "OPEN" ||
		ev[1].Key != "bf-e2" || ev[1].From != "OPEN" || ev[1].To != "WAITING_ON_CS" {
		t.Errorf("events = %+v", ev)
	}
	rows, err := e.pool.Query(e.ctx, `SELECT c.content, c.created_by, c.type::TEXT, r.origin, r.github_comment_id FROM comment c JOIN comment_github_ref r ON r.comment_id = c.id
		WHERE c.work_item_id = $1::uuid ORDER BY c.created_on`, id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var content, by, typ, origin string
		var gid int64
		if err := rows.Scan(&content, &by, &typ, &origin, &gid); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s|%s|%s|%s|%d", content, by, typ, origin, gid))
	}
	rows.Close()
	if fmt.Sprint(got) != "[first|octo|WORK_NOTE|GITHUB|880001 second|hubot|WORK_NOTE|GITHUB|880002]" {
		t.Errorf("comments = %v", got)
	}
	// The closed one carries its state.
	if l := e.link(byIssue(107)); l.State != "CLOSED" || l.Reason != "completed" || l.Closed == nil {
		t.Errorf("closed issue link = %+v", l)
	}
	if n := e.count(`SELECT count(*) FROM work_item WHERE id = $1::uuid AND github_issue_number IS NULL AND github_owner IS NULL`, id); n != 1 {
		t.Error("work_item.github_* must stay NULL (it would arm the comment trigger)")
	}

	// Second run, everything there: writes nothing.
	rowCount := func() string {
		return fmt.Sprint(links(),
			e.count(`SELECT count(*) FROM work_item WHERE id IN (SELECT work_item_id FROM github_issue_link WHERE lower(owner) = $1)`, rcOwner),
			e.count(`SELECT count(*) FROM comment WHERE work_item_id IN (SELECT work_item_id FROM github_issue_link WHERE lower(owner) = $1)`, rcOwner),
			e.count(`SELECT count(*) FROM work_item_status_event WHERE work_item_id IN (SELECT work_item_id FROM github_issue_link WHERE lower(owner) = $1)`, rcOwner),
			e.count(`SELECT count(*) FROM work_item_link WHERE from_id IN (SELECT work_item_id FROM github_issue_link WHERE lower(owner) = $1)`, rcOwner))
	}
	before := rowCount()
	linkBefore := e.linkSnapshot(id)
	again := domain.ProductIssueBackfillRequest{Owner: rcOwner, Repository: rcRepo, MaxIssues: 100}
	resp, err = svc.Backfill(e.ctx, again)
	if err != nil {
		t.Fatal(err)
	}
	if resp.AlreadyKnown != 5 || resp.CreatedMatched != 0 || resp.CreatedUnmatched != 0 || resp.CommentsWritten != 0 || resp.StatusEventsWritten != 0 || resp.SkippedPatch != 2 || resp.More {
		t.Fatalf("second run = %+v", resp)
	}
	if rowCount() != before || e.linkSnapshot(id) != linkBefore {
		t.Errorf("the second run changed rows: %s -> %s", before, rowCount())
	}

	// Gaps are filled, and only gaps.
	e.exec(`DELETE FROM comment WHERE id = (SELECT comment_id FROM comment_github_ref WHERE github_comment_id = 880002)`)
	e.exec(`DELETE FROM work_item_status_event WHERE dedupe_key = 'bf-e1'`)
	e.exec(`UPDATE github_issue_link SET board_status_raw = 'In Progress', board_status = 'IN_PROGRESS' WHERE work_item_id = $1::uuid`, id)
	resp, err = svc.Backfill(e.ctx, again)
	if err != nil || resp.AlreadyKnown != 5 || resp.CommentsWritten != 1 || resp.StatusEventsWritten != 1 {
		t.Fatalf("gap run = %+v err=%v", resp, err)
	}
	if e.link(id).Raw != "In Progress" {
		t.Error("a fill must not override the board status a webhook or reconciliation wrote")
	}
	if rowCount() != before {
		t.Errorf("after the fill: %s, want %s", rowCount(), before)
	}
	resp, err = svc.Backfill(e.ctx, again)
	if err != nil || resp.CommentsWritten != 0 || resp.StatusEventsWritten != 0 {
		t.Fatalf("fourth run = %+v err=%v", resp, err)
	}

	// A repository that is not mapped is refused.
	if _, err := svc.Backfill(e.ctx, domain.ProductIssueBackfillRequest{Owner: rcOwner, Repository: "nothing"}); err == nil {
		t.Error("an unmapped repository must be refused")
	}
}

// The repository's board listing and its not-seen selection.
func TestProductIssueReconcileIntegration_ListBoardsAndNotSeen(t *testing.T) {
	e := rcSeed(t)
	e.exec(`INSERT INTO product_repo_mapping (created_by, updated_by, product_name, abbreviation, owner, repository, github_label, board_project_number)
	        VALUES ('test', 'test', $1, 'rcn-two', $2, 'rcn-repo-two', 'product/rcn2', $3)`, rcProduct+" Two", rcOwner, rcBoard)
	e.exec(`INSERT INTO product_repo_mapping (created_by, updated_by, product_name, abbreviation, owner, repository, github_label)
	        VALUES ('test', 'test', $1, 'rcn-three', $2, 'rcn-repo-three', 'product/rcn3')`, rcProduct+" Three", rcOwner)
	t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM product_repo_mapping WHERE product_name LIKE $1`, rcProduct+"%")
	})
	boards, err := e.repo.ListBoards(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range boards {
		if strings.EqualFold(b.Owner, rcOwner) {
			n++
			if b.ProjectNumber != rcBoard {
				t.Errorf("board = %+v", b)
			}
		}
	}
	if n != 1 {
		t.Errorf("two repositories on one board and one with no board give %d boards, want 1", n)
	}
	if nums, err := e.repo.BoardsForRepository(e.ctx, strings.ToUpper(rcOwner), "RCN-REPO"); err != nil || fmt.Sprint(nums) != fmt.Sprint([]int{rcBoard}) {
		t.Errorf("boards of the repository = %v %v", nums, err)
	}

	open := e.mkLinked(1, "open", "")
	closed := e.mkLinked(2, "closed", "")
	other := e.mkLinked(3, "on another board", "")
	e.exec(`UPDATE github_issue_link SET github_state = 'CLOSED', last_synced_on = NOW() - interval '2 days' WHERE work_item_id = $1::uuid`, closed)
	e.exec(`UPDATE github_issue_link SET board_project_number = 5 WHERE work_item_id = $1::uuid`, other)
	started := time.Now()
	got, err := e.repo.ListLinksNotSeen(e.ctx, rcOwner, rcBoard, started, "", 10)
	if err != nil || len(got) != 1 || got[0].WorkItemID != open {
		t.Fatalf("not seen = %+v %v: a recently synced closed link and a link on another board are left out", got, err)
	}
	e.exec(`UPDATE github_issue_link SET last_synced_on = NOW() - interval '9 days' WHERE work_item_id = $1::uuid`, closed)
	if got, _ = e.repo.ListLinksNotSeen(e.ctx, rcOwner, rcBoard, started, "", 10); len(got) != 2 {
		t.Errorf("a closed link last synced over a week ago is checked again: %+v", got)
	}
	if got, _ = e.repo.ListLinksNotSeen(e.ctx, rcOwner, rcBoard, started, open, 10); len(got) != 1 && open < closed {
		t.Errorf("paging after an id: %+v", got)
	}
	e.exec(`UPDATE github_issue_link SET last_synced_on = NOW() + interval '1 minute' WHERE work_item_id = $1::uuid`, open)
	if got, _ = e.repo.ListLinksNotSeen(e.ctx, rcOwner, rcBoard, started, "", 10); len(got) != 1 || got[0].WorkItemID != closed {
		t.Errorf("a link synced by this run is not listed: %+v", got)
	}
}

// A known issue registered unmatched gets its case when a later backfill finds a
// footer that now matches: the AFFECTS link (BACKFILL) and the case's account and
// project; one that already has a case link, or whose footer does not match, is left.
func TestProductIssueBackfillIntegration_KnownUnmatchedIssueGetsItsCase(t *testing.T) {
	e := rcSeed(t)
	mk := func(n int) string {
		var id string
		if err := e.pool.QueryRow(e.ctx, `
			INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number('PRODUCT_ISSUE'), 'unmatched', 'PRODUCT_ISSUE') RETURNING id::TEXT`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		e.exec(`INSERT INTO product_issue (id) VALUES ($1::uuid)`, id)
		e.exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id) VALUES ($1::uuid, $2, $3, $4, $5)`,
			id, rcOwner, rcRepo, n, fmt.Sprintf("I_rc%d", n))
		return id
	}
	matching, partial := mk(201), mk(202)
	linked := e.mkLinked(203, "already linked", "")
	eng := &fakeSyncEng{}
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{
			withBody(issue(201), "WSO2 case id : PWI-WSO2-1\nCase number : PWI-CASE-1"),
			withBody(issue(202), "WSO2 case id : PWI-WSO2-1"),
			withBody(issue(203), "WSO2 case id : PWI-WSO2-2\nCase number : PWI-CASE-2"),
		}, nil
	}
	resp, err := e.service(eng).Backfill(e.ctx, domain.ProductIssueBackfillRequest{Owner: rcOwner, Repository: rcRepo, MaxIssues: 100})
	if err != nil || resp.AlreadyKnown != 3 || resp.Failed != 0 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	var to, source string
	var acc, proj *string
	if err := e.pool.QueryRow(e.ctx, `SELECT to_id::TEXT, source FROM work_item_link WHERE from_id = $1::uuid AND kind = 'AFFECTS' AND is_active`, matching).Scan(&to, &source); err != nil || to != pwiCaseID || source != "BACKFILL" {
		t.Errorf("matched issue link = %s %s %v", to, source, err)
	}
	if err := e.pool.QueryRow(e.ctx, `SELECT account_id::TEXT, project_id::TEXT FROM work_item WHERE id = $1::uuid`, matching).Scan(&acc, &proj); err != nil || acc == nil || *acc != pwiAccountID || proj == nil || *proj != pwiProjectID {
		t.Errorf("matched issue account/project = %v %v %v", acc, proj, err)
	}
	if k := e.count(`SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid`, partial); k != 0 {
		t.Errorf("a footer with one line matched: %d links", k)
	}
	if err := e.pool.QueryRow(e.ctx, `SELECT to_id::TEXT FROM work_item_link WHERE from_id = $1::uuid AND is_active`, linked).Scan(&to); err != nil || to != pwiCaseID {
		t.Errorf("an issue with a case link was re-pointed: %s %v", to, err)
	}
	// Run again: nothing changes.
	if _, err := e.service(eng).Backfill(e.ctx, domain.ProductIssueBackfillRequest{Owner: rcOwner, Repository: rcRepo, MaxIssues: 100}); err != nil {
		t.Fatal(err)
	}
	if k := e.count(`SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid`, matching); k != 1 {
		t.Errorf("links after the second run = %d", k)
	}
}

// The delivery of a board move that a reconciliation already recorded (a
// synthetic event stamped with the reconciliation's time) corrects that event's
// time instead of adding a second event; a synthetic event more than a day later,
// or for another column, is not taken for the move.
func TestProductIssueReconcileIntegration_WebhookMoveAdoptsTheSyntheticEvent(t *testing.T) {
	e := rcSeed(t)
	sync := repository.NewProductIssueSyncRepository(repository.NewScoped(e.pool))
	tSyn := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	real := tSyn.Add(-3 * time.Hour)
	seedSyn := func(wi, raw string, at time.Time) {
		e.exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
			VALUES ($1::uuid, 'WAITING_ON_CS', 'CS', $2, 'RECONCILE', $3, $4)`, wi, at, "reconcile:"+wi+":WAITING_ON_CS:"+at.Format("2006-01-02"), raw)
	}
	move := func(wi, key string, at time.Time) {
		t.Helper()
		if _, err := sync.RecordBoardStatus(e.ctx, repository.BoardStatusParams{WorkItemID: wi, ProjectNumber: rcBoard, RawStatus: "WOC",
			DedupeKey: key, Source: repository.StatusSourceWebhook, Actor: "octo", OccurredAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	type ev struct {
		At   time.Time
		From *string
	}
	events := func(wi string) []ev {
		rows, err := e.pool.Query(e.ctx, `SELECT occurred_at, from_status FROM work_item_status_event WHERE work_item_id = $1::uuid AND to_status = 'WAITING_ON_CS' ORDER BY occurred_at`, wi)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []ev
		for rows.Next() {
			var x ev
			if err := rows.Scan(&x.At, &x.From); err != nil {
				t.Fatal(err)
			}
			out = append(out, x)
		}
		return out
	}

	a := e.mkLinked(301, "synthetic", "WOC")
	e.seedEvent(a, "Open", "wh-a0", tSyn.Add(-48*time.Hour))
	seedSyn(a, "WOC", tSyn)
	move(a, "wh-a1", real)
	got := events(a)
	if len(got) != 1 || !got[0].At.Equal(real) || got[0].From == nil || *got[0].From != "OPEN" {
		t.Fatalf("events after the delivery = %+v, want one at %v from OPEN", got, real)
	}

	// More than 24 h apart: a different move, recorded as its own event.
	b := e.mkLinked(302, "far apart", "WOC")
	seedSyn(b, "WOC", tSyn)
	move(b, "wh-b1", tSyn.Add(-25*time.Hour))
	if got := events(b); len(got) != 2 {
		t.Errorf("a move 25 h before the synthetic event: %d events, want 2", len(got))
	}
}
