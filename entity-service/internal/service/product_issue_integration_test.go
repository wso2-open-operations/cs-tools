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
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The product issue reads, manual link and unlink end to end against PostgreSQL
// (the fixtures of the product work item integration test). Skipped without
// ENTITY_TEST_DATABASE_URL; it leaves no rows (the fixture cleanup cascades from
// the fixture account's work items).
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run ProductIssueIntegration

const pwiCase2ID = "77000000-0000-4000-8000-00000000000a"

func TestProductIssueIntegration(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, q := range []string{
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id)
		   VALUES ('` + pwiCase2ID + `', NOW(), NOW(), 'test', 'test', 'PWI-CASE-2', 'PWI-WSO2-2', 'pwi case 2', 'CASE', '` + pwiAccountID + `', '` + pwiProjectID + `')`,
		`INSERT INTO "case" (id, autoclose_enabled, state) VALUES ('` + pwiCase2ID + `', false, 'OPEN')`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("seed second case: %v", err)
		}
	}

	body := "Details.\n\nProduct : pwi-1.2.3\nPublic Git Issue : https://github.com/o/r/issues/7\nABT Team : PWI CRE Team\nOpened by : " + pwEmail
	eng := &fakePWEng{}
	setIssue := func(number int, title string) {
		eng.detail = engineeringentity.IssueDetail{
			NodeID: "I_pii" + title[:1] + string(rune('0'+number%10)), Number: number, Title: title, Body: &body, State: "open",
			Labels:    []string{"Origin/CS", "product/pwi", "Type/Discussion", "Priority/High(P2)"},
			HTMLURL:   "https://github.com/pwi-org/pwi-repo/issues/" + string(rune('0'+number%10)),
			CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z",
		}
	}
	svc := NewProductIssueService(eng, repository.NewProductIssueRepository(repository.NewScoped(pool)), internalAccess{})
	c := pwCtx()
	link := func(caseID string, req domain.LinkProductIssueRequest) (domain.ProductIssueSummary, bool, error) {
		req.CaseID = caseID
		return svc.Link(c, req)
	}
	ref := func(n int) domain.LinkProductIssueRequest {
		return domain.LinkProductIssueRequest{Owner: "PWI-ORG", Repository: "pwi-Repo", IssueNumber: n}
	}

	// --- register by GitHub reference (201), then idempotent (200) ---
	setIssue(301, "Alpha_broken 100%")
	a, created, err := link(pwiCaseID, ref(301))
	if err != nil || !created {
		t.Fatalf("register: created=%v err=%v", created, err)
	}
	if !regexp.MustCompile(`^PI\d{7}$`).MatchString(a.Number) || a.Title != "Alpha_broken 100%" || a.GitHub.Owner != "pwi-org" || a.GitHub.Repository != "pwi-repo" ||
		a.GitHub.IssueNumber != 301 || a.GitHub.State != "OPEN" || len(a.GitHub.Labels) != 4 || a.Priority == nil || *a.Priority != "P2" ||
		a.Category == nil || *a.Category != "Type/Discussion" || a.LinkSource == nil || *a.LinkSource != "MANUAL" || a.LinkedOn == nil ||
		a.Case == nil || a.Case.ID != pwiCaseID || a.Case.Number != "PWI-CASE-1" || a.LinkState != "ACTIVE" || a.LastActivityOn == nil {
		t.Fatalf("summary = %+v", a)
	}
	var openedBy, abt, pub, ghOwner *string
	if err := pool.QueryRow(ctx, `SELECT pi.opened_by_email, pi.abt_team, pi.public_issue_url, wi.github_owner FROM product_issue pi JOIN work_item wi ON wi.id = pi.id WHERE pi.id = $1::uuid`, a.ID).
		Scan(&openedBy, &abt, &pub, &ghOwner); err != nil {
		t.Fatal(err)
	}
	if openedBy == nil || *openedBy != pwEmail || abt == nil || *abt != "PWI CRE Team" || pub == nil || *pub != "https://github.com/o/r/issues/7" {
		t.Errorf("footer fields = %v %v %v", openedBy, abt, pub)
	}
	if ghOwner != nil {
		t.Error("work_item.github_owner must stay NULL (trigger 0206)")
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM comment WHERE work_item_id IN ($1::uuid, $2::uuid)`, pwiCaseID, a.ID); n != 0 {
		t.Errorf("a manual link wrote %d comments, want none", n)
	}
	again, created, err := link(pwiCaseID, ref(301))
	if err != nil || created || again.ID != a.ID || eng.detailCalls != 1 {
		t.Fatalf("idempotent: created=%v err=%v id=%s github calls=%d", created, err, again.ID, eng.detailCalls)
	}
	// By work item id too.
	if _, created, err := link(pwiCaseID, domain.LinkProductIssueRequest{WorkItemID: a.ID}); err != nil || created {
		t.Fatalf("by id again: created=%v err=%v", created, err)
	}

	// --- linked elsewhere: 409 naming the case; a case is not a product issue ---
	_, _, err = link(pwiCase2ID, domain.LinkProductIssueRequest{WorkItemID: a.ID})
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || ce.Code != apierror.CodeAlreadyLinked || !strings.Contains(ce.Msg, "PWI-CASE-1") {
		t.Fatalf("other case: %v", err)
	}
	var nfe *apierror.NotFoundError
	if _, _, err := link(pwiCase2ID, domain.LinkProductIssueRequest{WorkItemID: pwiCaseID}); !errors.As(err, &nfe) {
		t.Errorf("a case id as workItemId: %v", err)
	}
	// Patch and unmapped repository registered nothing.
	eng.detail.Labels = []string{"Type/Patch"}
	var ve *apierror.ValidationError
	if _, _, err := link(pwiCase2ID, ref(399)); !errors.As(err, &ve) {
		t.Errorf("patch: %v", err)
	}
	if _, _, err := link(pwiCase2ID, domain.LinkProductIssueRequest{Owner: "nobody", Repository: "nothing", IssueNumber: 1}); !errors.As(err, &ve) {
		t.Errorf("unmapped: %v", err)
	}

	// --- the case list ---
	list, err := svc.ListForCase(c, pwiCaseID)
	if err != nil || len(list.Items) != 1 || list.Items[0].ID != a.ID {
		t.Fatalf("case list: %+v %v", list, err)
	}
	if l2, err := svc.ListForCase(c, pwiCase2ID); err != nil || len(l2.Items) != 0 || l2.Items == nil {
		t.Errorf("second case list: %+v %v", l2, err)
	}
	if _, err := svc.ListForCase(c, a.ID); !errors.As(err, &nfe) {
		t.Errorf("a product issue is not a case: %v", err)
	}

	// --- detail: conversation and status history from the database ---
	seed := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	t1, t2, t3 := now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-1*time.Hour)
	seed(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content) VALUES
		('77000000-0000-4000-8000-0000000000c1', $2, 'bob@wso2.com', 'COMMENT', $1::uuid, 'second'),
		('77000000-0000-4000-8000-0000000000c2', $3, 'gh-user', 'COMMENT', $1::uuid, 'first')`, a.ID, t2, t1)
	seed(`INSERT INTO comment_github_ref (comment_id, github_comment_id, origin, author_login) VALUES ('77000000-0000-4000-8000-0000000000c2', 9001, 'GITHUB', 'octocat')`)
	seed(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content, deleted_at) VALUES
		('77000000-0000-4000-8000-0000000000c3', $2, 'bob@wso2.com', 'COMMENT', $1::uuid, 'deleted', NOW())`, a.ID, t3)
	seed(`INSERT INTO work_item_status_event (work_item_id, from_status, to_status, waits_on, occurred_at, actor, source, dedupe_key, raw_status) VALUES
		($1::uuid, 'OPEN', 'IN_PROGRESS', 'PRODUCT_TEAM', $3, 'octocat', 'WEBHOOK', 'pii-evt-2', 'In Progress'),
		($1::uuid, NULL, 'OPEN', 'PRODUCT_TEAM', $2, NULL, 'PORTAL', 'pii-evt-1', 'Open')`, a.ID, t1, t2)
	d, err := svc.Get(c, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Conversation) != 2 || d.Conversation[0].Body != "first" || d.Conversation[1].Body != "second" || d.ConversationTruncated {
		t.Fatalf("conversation = %+v truncated=%v", d.Conversation, d.ConversationTruncated)
	}
	if o := d.Conversation[0].Origin; o == nil || *o != "GITHUB" || d.Conversation[0].GitHubCommentID == nil || *d.Conversation[0].GitHubCommentID != 9001 ||
		d.Conversation[0].AuthorLogin == nil || *d.Conversation[0].AuthorLogin != "octocat" || d.Conversation[1].Origin != nil {
		t.Errorf("comment refs = %+v", d.Conversation)
	}
	if len(d.StatusHistory) != 2 || d.StatusHistory[0].ToStatus != "OPEN" || d.StatusHistory[1].ToStatus != "IN_PROGRESS" || d.StatusHistory[0].FromStatus != nil {
		t.Errorf("status history = %+v", d.StatusHistory)
	}
	if d.OpenedByEmail == nil || *d.OpenedByEmail != pwEmail || d.ABTTeam == nil || d.Case == nil || d.Case.Number != "PWI-CASE-1" {
		t.Errorf("detail = %+v", d)
	}
	// Last activity is the newest of GitHub update (2026-11-02), the live comment and the status events (the latter two are recent).
	if d.LastActivityOn == nil || d.LastActivityOn.Before(t2.Add(-time.Second)) || d.LastActivityOn.After(t2.Add(time.Second)) {
		t.Errorf("lastActivityOn = %v, want about %v (the newest live comment; a deleted one does not count)", d.LastActivityOn, t2)
	}
	if _, err := svc.Get(c, pwiCaseID); !errors.As(err, &nfe) {
		t.Errorf("detail of a case: %v", err)
	}
	if _, err := svc.Get(c, "77000000-0000-4000-8000-0000000000ff"); !errors.As(err, &nfe) {
		t.Errorf("detail of nothing: %v", err)
	}
	// The conversation is capped at the latest 200.
	seed(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
	      SELECT gen_random_uuid(), $2::timestamptz - (g || ' seconds')::interval, 'bulk', 'COMMENT', $1::uuid, 'bulk ' || g FROM generate_series(1, 200) g`, a.ID, now)
	if d, err = svc.Get(c, a.ID); err != nil || len(d.Conversation) != 200 || !d.ConversationTruncated {
		t.Fatalf("capped conversation: %d truncated=%v err=%v", len(d.Conversation), d.ConversationTruncated, err)
	}
	if d.Conversation[199].Body != "bulk 1" {
		t.Errorf("newest comment = %q, want the latest kept and last", d.Conversation[199].Body)
	}

	// --- search ---
	setIssue(302, "Beta plain title")
	eng.detail.Labels = []string{"Origin/CS", "product/pwi", "Type/Query"}
	b, created, err := link(pwiCase2ID, ref(302))
	if err != nil || !created || b.Priority != nil {
		t.Fatalf("second issue: created=%v err=%v %+v", created, err, b)
	}
	search := func(f domain.SearchProductIssuesFilters, p domain.Pagination) domain.SearchProductIssuesResponse {
		t.Helper()
		f.Owner = "PWI-ORG" // only this test's issues, case-insensitively
		r, err := svc.Search(c, domain.SearchProductIssuesRequest{Filters: f, Pagination: p})
		if err != nil {
			t.Fatalf("search %+v: %v", f, err)
		}
		return r
	}
	ids := func(r domain.SearchProductIssuesResponse) string {
		var s []string
		for _, it := range r.Items {
			s = append(s, it.ID)
		}
		return strings.Join(s, ",")
	}
	if r := search(domain.SearchProductIssuesFilters{}, domain.Pagination{}); r.Total != 2 || ids(r) != b.ID+","+a.ID || r.Limit != 20 {
		t.Errorf("all (newest first): total=%d ids=%s want %s,%s", r.Total, ids(r), b.ID, a.ID)
	}
	if r := search(domain.SearchProductIssuesFilters{}, domain.Pagination{Limit: 1, Offset: 1}); r.Total != 2 || ids(r) != a.ID || r.Offset != 1 {
		t.Errorf("page 2: %+v", r)
	}
	if r := search(domain.SearchProductIssuesFilters{CaseID: pwiCase2ID}, domain.Pagination{}); ids(r) != b.ID {
		t.Errorf("caseId: %s", ids(r))
	}
	if r := search(domain.SearchProductIssuesFilters{GitHubState: "closed"}, domain.Pagination{}); r.Total != 0 {
		t.Errorf("githubState CLOSED: %d", r.Total)
	}
	if r := search(domain.SearchProductIssuesFilters{GitHubState: "OPEN", Repository: "PWI-REPO"}, domain.Pagination{}); r.Total != 2 {
		t.Errorf("githubState OPEN + repository: %d", r.Total)
	}
	if r := search(domain.SearchProductIssuesFilters{Repository: "other"}, domain.Pagination{}); r.Total != 0 {
		t.Errorf("repository: %d", r.Total)
	}
	seed(`UPDATE github_issue_link SET board_status = 'WAITING_ON_CS', board_status_raw = 'WOC' WHERE work_item_id = $1::uuid`, a.ID)
	if r := search(domain.SearchProductIssuesFilters{BoardStatuses: []string{"WAITING_ON_CS", "RESOLVED"}}, domain.Pagination{}); ids(r) != a.ID {
		t.Errorf("boardStatuses: %s", ids(r))
	}
	if r := search(domain.SearchProductIssuesFilters{BoardStatuses: []string{"OPEN"}}, domain.Pagination{}); r.Total != 0 {
		t.Errorf("boardStatuses OPEN: %d", r.Total)
	}
	// The query is a substring of the number or the title; % and _ are literal.
	for _, tc := range []struct {
		q    string
		want string
	}{
		{"alpha_broken", a.ID}, {"ALPHA", a.ID}, {"100%", a.ID}, {"plain", b.ID}, {a.Number, a.ID}, {"301", a.ID}, {"302", b.ID},
		{"a_b", a.ID},   // an unescaped _ would also match "a p" of "Beta plain"
		{"a%e", ""},     // an unescaped % would match both titles
		{"%", a.ID},     // only the title with a literal percent sign
		{"_", a.ID},     // only the title with a literal underscore
		{"nothing", ""}, // no match
	} {
		if r := search(domain.SearchProductIssuesFilters{Query: tc.q}, domain.Pagination{}); ids(r) != tc.want {
			t.Errorf("query %q: got %s want %s", tc.q, ids(r), tc.want)
		}
	}

	// --- unlink: the issue is unmatched, then links to the second case ---
	if err := svc.Unlink(c, pwiCaseID, a.ID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := svc.Unlink(c, pwiCaseID, a.ID); !errors.As(err, &nfe) {
		t.Errorf("second unlink: %v", err)
	}
	if err := svc.Unlink(c, pwiCase2ID, a.ID); !errors.As(err, &nfe) {
		t.Errorf("unlink from a case it was never linked to: %v", err)
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid AND NOT is_active`, a.ID); n != 1 {
		t.Errorf("inactive links = %d, want the row kept", n)
	}
	if l, _ := svc.ListForCase(c, pwiCaseID); len(l.Items) != 0 {
		t.Errorf("case list after unlink: %d", len(l.Items))
	}
	if r := search(domain.SearchProductIssuesFilters{Unmatched: true}, domain.Pagination{}); ids(r) != a.ID || r.Items[0].Case != nil || r.Items[0].LinkSource != nil {
		t.Errorf("unmatched: %+v", r)
	}
	if r := search(domain.SearchProductIssuesFilters{Unmatched: false, CaseID: pwiCaseID}, domain.Pagination{}); r.Total != 0 {
		t.Errorf("caseId after unlink: %d", r.Total)
	}
	if d, err := svc.Get(c, a.ID); err != nil || d.Case != nil || d.LinkSource != nil {
		t.Errorf("detail of an unmatched issue: %+v %v", d.Case, err)
	}
	relinked, created, err := link(pwiCase2ID, domain.LinkProductIssueRequest{WorkItemID: a.ID})
	if err != nil || !created || relinked.Case == nil || relinked.Case.Number != "PWI-CASE-2" {
		t.Fatalf("link to the second case: created=%v err=%v %+v", created, err, relinked)
	}
	if r := search(domain.SearchProductIssuesFilters{Unmatched: true}, domain.Pagination{}); r.Total != 0 {
		t.Errorf("unmatched after relink: %d", r.Total)
	}
	// Relinking to the first case after all is refused while the second holds it.
	if _, _, err := link(pwiCaseID, domain.LinkProductIssueRequest{WorkItemID: a.ID}); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "PWI-CASE-2") {
		t.Errorf("relink while linked: %v", err)
	}

	// --- a closed case takes no link ---
	seed(`UPDATE "case" SET state = 'CLOSED' WHERE id = $1::uuid`, pwiCaseID)
	if _, _, err := link(pwiCaseID, ref(302)); !errors.As(err, &ce) || ce.Code != "" {
		t.Errorf("closed case: %v", err)
	}

	// --- a customer session sees none of it ---
	piiCustomerSeesNone(t, pool)
}

// piiCustomerSeesNone runs the read model's select as a member of the case's
// project on the non-superuser csm_app role (row-level security applies), the
// way pwiCustomerCannotRead does, and the member must see no product issue row.
func piiCustomerSeesNone(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("ENTITY_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET ROLE csm_app")
		return err
	}
	app, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Skipf("csm_app pool: %v", err)
	}
	defer app.Close()
	scoped := repository.NewScoped(app)
	member := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ProjectIDs: []string{pwiProjectID}, ViewerEmail: pwiMember})
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := scoped.QueryRow(member, sql, args...).Scan(&n); err != nil {
			t.Skipf("csm_app cannot run the check (role missing or ungranted): %v", err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM work_item WHERE id = $1::uuid`, pwiCase2ID); n != 1 {
		t.Fatalf("the member sees %d of the second case, want 1 (the check is meaningless otherwise)", n)
	}
	// The statement every list, detail and search reads through.
	if n := count(`SELECT count(*) FROM (` + repository.ProductIssueSelectSQL + ` WHERE wi.type = 'PRODUCT_ISSUE') x`); n != 0 {
		t.Errorf("a customer session sees %d product issue rows", n)
	}
	for name, sql := range map[string]string{
		"status events":               `SELECT count(*) FROM work_item_status_event e JOIN github_issue_link g ON g.work_item_id = e.work_item_id`,
		"github refs":                 `SELECT count(*) FROM comment_github_ref`,
		"case links":                  `SELECT count(*) FROM work_item_link`,
		"product rows":                `SELECT count(*) FROM product_issue`,
		"comments of a product issue": `SELECT count(*) FROM comment c JOIN work_item w ON w.id = c.work_item_id WHERE w.type = 'PRODUCT_ISSUE'`,
	} {
		if n := count(sql); n != 0 {
			t.Errorf("a customer session sees %s (%d rows)", name, n)
		}
	}
}

// The footer's "Opened by" email of an issue registered from GitHub is
// user-editable text: it is stored but never resolved to opened_by_user_id. The
// create path's email is the authenticated caller's, so it is resolved.
func TestProductIssueIntegration_OpenedByUserIsNotResolvedFromTheFooter(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := context.Background()
	const userID = "77000000-0000-4000-8000-0000000000f1"
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES ($1, NOW(), NOW(), 'pwi-opener', $2)`, userID, pwEmail); err != nil {
		t.Fatal(err)
	}
	openedByUser := func(workItemID string) (*string, *string) {
		var uid, email *string
		if err := pool.QueryRow(ctx, `SELECT opened_by_user_id::text, opened_by_email FROM product_issue WHERE id = $1::uuid`, workItemID).Scan(&uid, &email); err != nil {
			t.Fatal(err)
		}
		return uid, email
	}

	// Registered from GitHub: the email is kept, the user id is not set.
	body := "Details.\n\nProduct : pwi-1.2.3\nOpened by : " + pwEmail
	eng := &fakePWEng{detail: engineeringentity.IssueDetail{NodeID: "I_pwi_ob1", Number: 401, Title: "Footer email", Body: &body, State: "open",
		Labels: []string{"Origin/CS", "product/pwi"}, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/401", CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z"}}
	svc := NewProductIssueService(eng, repository.NewProductIssueRepository(repository.NewScoped(pool)), internalAccess{})
	sum, _, err := svc.Link(pwCtx(), domain.LinkProductIssueRequest{CaseID: pwiCaseID, Owner: "pwi-org", Repository: "pwi-repo", IssueNumber: 401})
	if err != nil {
		t.Fatal(err)
	}
	if uid, email := openedByUser(sum.ID); uid != nil || email == nil || *email != pwEmail {
		t.Errorf("registered from GitHub: opened_by_user_id=%v opened_by_email=%v, want NULL and the footer email", uid, email)
	}

	// Created from the case: the caller's email resolves.
	ceng := &fakePWEng{createRef: engineeringentity.IssueRef{ID: 9, NodeID: "I_pwi_ob2", Number: 402},
		detail: engineeringentity.IssueDetail{NodeID: "I_pwi_ob2", Number: 402, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/402", CreatedAt: "2026-11-18T10:00:00Z"}}
	csvc := NewProductWorkItemService(ceng, repository.NewProductWorkItemRepository(repository.NewScoped(pool)), repository.NewProductRepoMappingRepository(pool),
		NewCaseService(repository.NewCaseRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool), nil, internalAccess{}, repository.NewProjectContactRepository(pool)), internalAccess{})
	resp, _, err := csvc.CreateProductWorkItem(repository.WithSystemIdentity(pwCtx()), domain.CreateProductWorkItemRequest{CaseID: pwiCaseID, IdempotencyKey: "ob-1", Title: "Created", IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High"})
	if err != nil || resp.WorkItem == nil {
		t.Fatalf("create: %+v %v", resp, err)
	}
	if uid, _ := openedByUser(resp.WorkItem.ID); uid == nil || *uid != userID {
		t.Errorf("created from the case: opened_by_user_id=%v, want %s", uid, userID)
	}
}

// IssueBeingCreated sees PENDING and GITHUB_CREATED requests only.
func TestProductIssueIntegration_IssueBeingCreated(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := context.Background()
	repo := repository.NewProductIssueRepository(repository.NewScoped(pool))
	if _, err := pool.Exec(ctx, `INSERT INTO work_item_creation_request (idempotency_key, case_id, request_hash, kind, status, github_owner, github_repository, github_issue_number, created_by, updated_by)
		VALUES ('ibc-1', $1::uuid, 'h', 'PRODUCT_ISSUE', 'GITHUB_CREATED', 'PWI-Org', 'pwi-repo', 555, 'test', 'test')`, pwiCaseID); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.IssueBeingCreated(ctx, "pwi-org", "PWI-REPO", 555); err != nil || !got {
		t.Fatalf("GITHUB_CREATED: got=%v err=%v", got, err)
	}
	if got, _ := repo.IssueBeingCreated(ctx, "pwi-org", "pwi-repo", 556); got {
		t.Error("another number must not match")
	}
	if _, err := pool.Exec(ctx, `UPDATE work_item_creation_request SET status = 'COMPLETED' WHERE idempotency_key = 'ibc-1' AND case_id = $1::uuid`, pwiCaseID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.IssueBeingCreated(ctx, "pwi-org", "pwi-repo", 555); got {
		t.Error("a COMPLETED request no longer owns the issue (its link row does)")
	}
}
