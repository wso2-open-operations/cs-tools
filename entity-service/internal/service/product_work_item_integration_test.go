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
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The product work item create end to end against PostgreSQL: the real service
// over the real repository, with a fake engineering entity. Skipped without
// ENTITY_TEST_DATABASE_URL (a superuser; the customer-session check switches to
// the non-superuser csm_app role per connection and skips if it is not there).
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run ProductWorkItemIntegration

const (
	pwiAccountID = "77000000-0000-4000-8000-000000000001"
	pwiProjectID = "77000000-0000-4000-8000-000000000002"
	pwiContactID = "77000000-0000-4000-8000-000000000003"
	pwiCaseID    = "77000000-0000-4000-8000-000000000004"
	pwiGroupID   = "77000000-0000-4000-8000-000000000005"
	pwiProductID = "77000000-0000-4000-8000-000000000006"
	pwiVersionID = "77000000-0000-4000-8000-000000000007"
	pwiDeployID  = "77000000-0000-4000-8000-000000000008"
	pwiDPID      = "77000000-0000-4000-8000-000000000009"
	pwiMember    = "pwi-member@test.local"
)

func pwiPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func pwiCleanup(ctx context.Context, pool *pgxpool.Pool) {
	for _, q := range []string{
		`DELETE FROM work_item_creation_request WHERE case_id = '` + pwiCaseID + `'`,
		`DELETE FROM work_item WHERE account_id = '` + pwiAccountID + `'`, // cascades to the rest
		`DELETE FROM deployed_product WHERE id = '` + pwiDPID + `'`,
		`DELETE FROM deployment WHERE id = '` + pwiDeployID + `'`,
		`DELETE FROM product_version WHERE id = '` + pwiVersionID + `'`,
		`DELETE FROM product WHERE id = '` + pwiProductID + `'`,
		`DELETE FROM product_repo_mapping WHERE product_name = 'PWI Test Product'`,
		`DELETE FROM project_contact WHERE project_id = '` + pwiProjectID + `'`,
		`DELETE FROM project WHERE id = '` + pwiProjectID + `'`,
		`DELETE FROM account_contact WHERE id = '` + pwiContactID + `'`,
		`DELETE FROM account WHERE id = '` + pwiAccountID + `'`,
		`DELETE FROM "group" WHERE id = '` + pwiGroupID + `'`,
	} {
		_, _ = pool.Exec(ctx, q)
	}
}

func pwiSeed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pwiCleanup(ctx, pool)
	t.Cleanup(func() { pwiCleanup(context.Background(), pool) })
	now := time.Now().UTC()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', 'PWI CRE Team')`, []any{pwiGroupID, now}},
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, cre_team_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWI Account', 'PWI-ACC-1', 'sf-pwi-acc-1', $3)`, []any{pwiAccountID, now, pwiGroupID}},
		{`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWI Contact', $3)`, []any{pwiContactID, now, pwiAccountID}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, account_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWITEST', 'sf-pwitest', $3)`, []any{pwiProjectID, now, pwiAccountID}},
		{`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		   VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, []any{now, pwiMember, pwiContactID, pwiProjectID}},
		{`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, category, code, name)
		   VALUES ($1, $2, $2, 'test', 'test', (SELECT enum_range(NULL::product_category_enum))[1], 'pwi', 'PWI Test Product')`, []any{pwiProductID, now}},
		{`INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id, is_public, show_in_support_matrix, tags, legacy_status)
		   VALUES ($1, $2, $2, 'test', 'test', '1.2.3', $3, false, false, '{}', false)`, []any{pwiVersionID, now, pwiProductID}},
		{`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, air_gapped, project_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWI-DEP-1', 'PWI Deployment', true, false, $3)`, []any{pwiDeployID, now, pwiProjectID}},
		{`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, deployment_id, product_id, version_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWI-DP-1', $3, $4, $5)`, []any{pwiDPID, now, pwiDeployID, pwiProductID, pwiVersionID}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id, deployment_id, deployed_product_id)
		   VALUES ($1, $2, $2, 'test', 'test', 'PWI-CASE-1', 'PWI-WSO2-1', 'pwi case', 'CASE', $3, $4, $5, $6)`, []any{pwiCaseID, now, pwiAccountID, pwiProjectID, pwiDeployID, pwiDPID}},
		{`INSERT INTO "case" (id, autoclose_enabled, state) VALUES ($1, false, 'OPEN')`, []any{pwiCaseID}},
		{`INSERT INTO product_repo_mapping (created_by, updated_by, product_name, abbreviation, owner, repository, github_label)
		   VALUES ('test', 'test', 'PWI Test Product', 'pwi-abbr', 'pwi-org', 'pwi-repo', 'product/pwi')`, nil},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("seed (%.70s): %v", q.sql, err)
		}
	}
}

func pwiCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count (%.80s): %v", sql, err)
	}
	return n
}

func TestProductWorkItemIntegration(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	eng := &fakePWEng{
		createRef: engineeringentity.IssueRef{ID: 9, NodeID: "I_pwi1", Number: 101},
		detail: engineeringentity.IssueDetail{NodeID: "I_pwi1", Number: 101, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/101",
			CreatedAt: "2026-11-18T10:00:00Z"},
	}
	svc := NewProductWorkItemService(eng, repository.NewProductWorkItemRepository(repository.NewScoped(pool)),
		repository.NewProductRepoMappingRepository(pool),
		// The real case service, as routes.go wires it: the note and the tag are written by it.
		NewCaseService(repository.NewCaseRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool), nil, internalAccess{}, repository.NewProjectContactRepository(pool)),
		internalAccess{})
	pwCtx := func() context.Context { return repository.WithSystemIdentity(pwCtx()) }

	req := domain.CreateProductWorkItemRequest{CaseID: pwiCaseID, IdempotencyKey: "pwi-key-1", Title: "Integration issue", Description: "It fails.",
		IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High", UpdateLevel: "U7"}

	// --- the product issue ---
	resp, created, err := svc.CreateProductWorkItem(pwCtx(), req)
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	if resp.Kind != "PRODUCT_ISSUE" || resp.WorkItem == nil || !regexp.MustCompile(`^PI\d{7}$`).MatchString(resp.WorkItem.Number) {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Issue.URL != "https://github.com/pwi-org/pwi-repo/issues/101" || resp.Issue.Repo != "pwi-org/pwi-repo" || resp.Issue.Number != 101 {
		t.Errorf("issue = %+v", resp.Issue)
	}
	wi := resp.WorkItem.ID

	// What was sent to GitHub: the footer comes from the database.
	if len(eng.createCalls) != 1 {
		t.Fatalf("create calls = %d", len(eng.createCalls))
	}
	body := eng.createCalls[0].body
	for _, want := range []string{"Product : pwi-1.2.3", "Update level : U7", "WSO2 case id : PWI-WSO2-1", "Case number : PWI-CASE-1", "ABT Team : PWI CRE Team", "Opened by : " + pwEmail} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q: %q", want, body)
		}
	}
	if eng.createCalls[0].owner != "pwi-org" || eng.createCalls[0].repo != "pwi-repo" {
		t.Errorf("repository = %s/%s", eng.createCalls[0].owner, eng.createCalls[0].repo)
	}

	// The five rows, and no work_item.github_* column.
	var typ, subject string
	var account, project *string
	var ghNumber *int
	var ghOwner, ghRepo *string
	if err := pool.QueryRow(context.Background(), `SELECT type::text, subject, account_id::text, project_id::text, github_issue_number, github_owner, github_repository
		FROM work_item WHERE id = $1::uuid`, wi).Scan(&typ, &subject, &account, &project, &ghNumber, &ghOwner, &ghRepo); err != nil {
		t.Fatal(err)
	}
	if typ != "PRODUCT_ISSUE" || subject != "Integration issue" || account == nil || *account != pwiAccountID || project == nil || *project != pwiProjectID {
		t.Errorf("work_item = %s %q %v %v", typ, subject, account, project)
	}
	if ghNumber != nil || ghOwner != nil || ghRepo != nil {
		t.Errorf("work_item.github_* must stay NULL (trigger 0206): %v %v %v", ghNumber, ghOwner, ghRepo)
	}
	var priority, category, openedBy, abt *string
	if err := pool.QueryRow(context.Background(), `SELECT priority, category, opened_by_email, abt_team FROM product_issue WHERE id = $1::uuid`, wi).
		Scan(&priority, &category, &openedBy, &abt); err != nil {
		t.Fatal(err)
	}
	if *priority != "P2" || *category != "Type/Discussion" || *openedBy != pwEmail || *abt != "PWI CRE Team" {
		t.Errorf("product_issue = %v %v %v %v", *priority, *category, *openedBy, *abt)
	}
	var owner, repo, nodeID, htmlURL, state, labels string
	var number int
	if err := pool.QueryRow(context.Background(), `SELECT owner, repository, issue_number, node_id, html_url, github_state, labels::text FROM github_issue_link WHERE work_item_id = $1::uuid`, wi).
		Scan(&owner, &repo, &number, &nodeID, &htmlURL, &state, &labels); err != nil {
		t.Fatal(err)
	}
	if owner != "pwi-org" || repo != "pwi-repo" || number != 101 || nodeID != "I_pwi1" || htmlURL != resp.Issue.URL || state != "OPEN" ||
		!strings.Contains(labels, "Origin/CS") || !strings.Contains(labels, "Priority/High(P2)") {
		t.Errorf("github_issue_link = %s %s %d %s %s %s %s", owner, repo, number, nodeID, htmlURL, state, labels)
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid AND to_id = $2::uuid AND kind = 'AFFECTS' AND source = 'CREATION' AND is_active`, wi, pwiCaseID); n != 1 {
		t.Errorf("work_item_link rows = %d", n)
	}
	note := "GitHub issue filed: " + resp.Issue.URL
	if n := pwiCount(t, pool, `SELECT count(*) FROM comment WHERE work_item_id = $1::uuid AND type = 'WORK_NOTE' AND content = $2 AND created_by = $3`, pwiCaseID, note, pwEmail); n != 1 {
		t.Errorf("work notes = %d", n)
	}
	var status string
	var reqWI *string
	if err := pool.QueryRow(context.Background(), `SELECT status, work_item_id::text FROM work_item_creation_request WHERE case_id = $1::uuid AND idempotency_key = 'pwi-key-1'`, pwiCaseID).Scan(&status, &reqWI); err != nil {
		t.Fatal(err)
	}
	if status != "COMPLETED" || reqWI == nil || *reqWI != wi {
		t.Errorf("creation request = %s %v", status, reqWI)
	}

	// --- a replay creates nothing new ---
	rows := func() [4]int {
		return [4]int{
			pwiCount(t, pool, `SELECT count(*) FROM work_item WHERE account_id = $1::uuid AND type = 'PRODUCT_ISSUE'`, pwiAccountID),
			pwiCount(t, pool, `SELECT count(*) FROM github_issue_link gl JOIN work_item w ON w.id = gl.work_item_id WHERE w.account_id = $1::uuid`, pwiAccountID),
			pwiCount(t, pool, `SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, pwiCaseID),
			pwiCount(t, pool, `SELECT count(*) FROM work_item_creation_request WHERE case_id = $1::uuid`, pwiCaseID),
		}
	}
	before := rows()
	again, created, err := svc.CreateProductWorkItem(pwCtx(), req)
	if err != nil || created || again.WorkItem == nil || again.WorkItem.ID != wi || again.Issue != resp.Issue {
		t.Fatalf("replay: created=%v err=%v resp=%+v", created, err, again)
	}
	if rows() != before || len(eng.createCalls) != 1 {
		t.Errorf("a replay wrote rows or called GitHub: %v -> %v, creates %d", before, rows(), len(eng.createCalls))
	}
	// The same key with a different body is refused.
	changed := req
	changed.Title = "Another title"
	if _, _, err := svc.CreateProductWorkItem(pwCtx(), changed); err == nil || !strings.Contains(err.Error(), "idempotency key") {
		t.Errorf("reused key: %v", err)
	}

	// --- an untracked patch: note and tag, no work item; the tag once, even when the case has it ---
	patchTags := func() int {
		return pwiCount(t, pool, `SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 'patch'`, pwiCaseID)
	}
	if patchTags() != 0 {
		t.Fatal("fixture case already has the patch tag")
	}
	for i, key := range []string{"pwi-patch-1", "pwi-patch-2"} {
		eng.createRef.Number = 200 + i
		eng.detail.Number = 200 + i
		eng.detail.NodeID = "I_pwi_patch" + key[len(key)-1:]
		eng.detail.HTMLURL = "https://github.com/pwi-org/pwi-repo/issues/" + string(rune('0'+i)) + "00"
		p := domain.CreateProductWorkItemRequest{CaseID: pwiCaseID, IdempotencyKey: key, Title: "Patch " + key, IssueTypeLabel: "Type/Patch"}
		pr, created, err := svc.CreateProductWorkItem(pwCtx(), p)
		if err != nil || !created || pr.Kind != "UNTRACKED_PATCH" || pr.WorkItem != nil {
			t.Fatalf("patch %s: created=%v err=%v resp=%+v", key, created, err, pr)
		}
		if n := patchTags(); n != 1 {
			t.Errorf("after patch %d the case has %d patch tags, want 1", i+1, n)
		}
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM work_item WHERE account_id = $1::uuid AND type = 'PRODUCT_ISSUE'`, pwiAccountID); n != 1 {
		t.Errorf("product issues = %d: an untracked patch must register no work item", n)
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM comment WHERE work_item_id = $1::uuid AND type = 'WORK_NOTE'`, pwiCaseID); n != 3 {
		t.Errorf("work notes = %d, want 3 (one per filed issue)", n)
	}

	// --- a customer session cannot read the new work item ---
	pwiCustomerCannotRead(t, pool, wi)
}

// pwiCustomerCannotRead reads as a member of the case's project through the real
// Scoped identity, on connections switched to the non-superuser csm_app role so
// row-level security applies.
func pwiCustomerCannotRead(t *testing.T, owner *pgxpool.Pool, productIssueID string) {
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
	count := func(ctx context.Context, sql string, args ...any) int {
		t.Helper()
		var n int
		if err := scoped.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Skipf("csm_app cannot run the check (role missing or ungranted): %v", err)
		}
		return n
	}
	if n := count(member, `SELECT count(*) FROM work_item WHERE id = $1::uuid`, pwiCaseID); n != 1 {
		t.Fatalf("the member sees %d of the case, want 1 (the check is meaningless otherwise)", n)
	}
	for name, sql := range map[string]string{
		"the product issue work item": `SELECT count(*) FROM work_item WHERE id = $1::uuid`,
		"its product_issue row":       `SELECT count(*) FROM product_issue WHERE id = $1::uuid`,
		"its GitHub link":             `SELECT count(*) FROM github_issue_link WHERE work_item_id = $1::uuid`,
		"its case link":               `SELECT count(*) FROM work_item_link WHERE from_id = $1::uuid`,
		"its creation request":        `SELECT count(*) FROM work_item_creation_request WHERE work_item_id = $1::uuid`,
	} {
		if n := count(member, sql, productIssueID); n != 0 {
			t.Errorf("a customer session sees %s (%d rows)", name, n)
		}
	}
	// The case's own work note is internal too, but the case stays visible.
	if n := count(member, `SELECT count(*) FROM comment WHERE work_item_id = $1::uuid AND type = 'WORK_NOTE'`, pwiCaseID); n != 0 {
		t.Errorf("a customer session sees %d work notes", n)
	}
}

// A retry after the registration committed but the note or tag failed must not
// insert a second work item: the request already holds work_item_id.
func TestProductWorkItemIntegration_RegistrationResumesWithoutASecondWorkItem(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	repo := repository.NewProductWorkItemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	cr, err := repo.BeginCreationRequest(ctx, repository.BeginCreationParams{CaseID: pwiCaseID, Key: "resume-1", Hash: "h", Kind: "PRODUCT_ISSUE", Actor: pwEmail, Lease: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ref := repository.GitHubIssueRef{Owner: "pwi-org", Repository: "pwi-repo", Number: 300, NodeID: "I_resume"}
	if err := repo.RecordGitHubCreated(ctx, cr.ID, pwEmail, ref); err != nil {
		t.Fatal(err)
	}
	p := repository.RegisterParams{RequestID: cr.ID, CaseID: pwiCaseID, CaseKey: "resume-1", Actor: pwEmail, ActorEmail: pwEmail, Issue: ref, Title: "Resume",
		Respond: func(id, n string) any { return map[string]string{"id": id, "n": n} }}
	first, err := repo.RegisterProductIssue(ctx, p)
	if err != nil || first.WorkItemID == "" {
		t.Fatalf("first: %+v %v", first, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM work_item_creation_request WHERE id = $1::uuid`, cr.ID).Scan(&status); err != nil || status != "GITHUB_CREATED" {
		t.Fatalf("status after registration = %q %v, want GITHUB_CREATED", status, err)
	}
	second, err := repo.RegisterProductIssue(ctx, p)
	if err != nil || second.WorkItemID != first.WorkItemID || second.Number != first.Number {
		t.Fatalf("second: %+v %v", second, err)
	}
	if n := pwiCount(t, pool, `SELECT count(*) FROM work_item WHERE account_id = $1::uuid AND type = 'PRODUCT_ISSUE'`, pwiAccountID); n != 1 {
		t.Errorf("product issues = %d, want 1", n)
	}
}

// An ambiguous PENDING row (a note recorded) is leased too: another attempt may
// not resume it until productAmbiguousLease has passed since its last write.
func TestProductWorkItemIntegration_AmbiguousRowIsLeased(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	repo := repository.NewProductWorkItemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	begin := func() repository.CreationRequest {
		cr, err := repo.BeginCreationRequest(ctx, repository.BeginCreationParams{CaseID: pwiCaseID, Key: "amb-lease", Hash: "h", Kind: "PRODUCT_ISSUE", Actor: pwEmail,
			Lease: productRequestLease, AmbiguousLease: productAmbiguousLease})
		if err != nil {
			t.Fatal(err)
		}
		return cr
	}
	first := begin()
	if !first.Inserted {
		t.Fatal("first call should insert")
	}
	if err := repo.RecordOutcome(ctx, first.ID, pwEmail, "create ended without a clear answer", false); err != nil {
		t.Fatal(err)
	}
	if cr := begin(); !cr.InFlight {
		t.Fatalf("an ambiguous row touched just now must be in flight: %+v", cr)
	}
	if _, err := pool.Exec(ctx, `UPDATE work_item_creation_request SET updated_on = NOW() - interval '61 seconds' WHERE id = $1::uuid`, first.ID); err != nil {
		t.Fatal(err)
	}
	cr := begin()
	if cr.InFlight || cr.Inserted || cr.RetryOfFailed {
		t.Fatalf("after the ambiguous lease the row is resumed: %+v", cr)
	}
	// Claimed: now in flight under the 2-minute lease, whatever the note was.
	if again := begin(); !again.InFlight {
		t.Fatalf("a freshly claimed row is in flight: %+v", again)
	}
}

// timeout -> a retry refused because the case is closed -> the row is still
// PENDING -> a later retry adopts the earlier issue instead of creating another.
func TestProductWorkItemIntegration_InheritedAmbiguousRowSurvivesARefusalAndIsAdopted(t *testing.T) {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := context.Background()
	eng := &fakePWEng{
		createRef: engineeringentity.IssueRef{ID: 9, NodeID: "I_amb", Number: 501},
		detail:    engineeringentity.IssueDetail{NodeID: "I_amb", Number: 501, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/501", CreatedAt: "2026-11-18T10:00:00Z"},
		createErr: &engineeringentity.Error{StatusCode: 0, Message: "request failed: timeout"},
	}
	svc := NewProductWorkItemService(eng, repository.NewProductWorkItemRepository(repository.NewScoped(pool)),
		repository.NewProductRepoMappingRepository(pool),
		NewCaseService(repository.NewCaseRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool), nil, internalAccess{}, repository.NewProjectContactRepository(pool)),
		internalAccess{})
	sysCtx := repository.WithSystemIdentity(pwCtx())
	req := domain.CreateProductWorkItemRequest{CaseID: pwiCaseID, IdempotencyKey: "amb-1", Title: "Ambiguous issue", Description: "It fails.",
		IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High", UpdateLevel: "U7"}
	status := func() (string, *string) {
		var st string
		var note *string
		if err := pool.QueryRow(ctx, `SELECT status, error FROM work_item_creation_request WHERE case_id = $1::uuid AND idempotency_key = 'amb-1'`, pwiCaseID).Scan(&st, &note); err != nil {
			t.Fatal(err)
		}
		return st, note
	}
	age := func() {
		if _, err := pool.Exec(ctx, `UPDATE work_item_creation_request SET updated_on = NOW() - interval '3 minutes' WHERE case_id = $1::uuid`, pwiCaseID); err != nil {
			t.Fatal(err)
		}
	}

	// 1. The create times out: the issue may exist.
	if _, _, err := svc.CreateProductWorkItem(sysCtx, req); err == nil {
		t.Fatal("want the upstream failure")
	}
	if st, note := status(); st != "PENDING" || note == nil {
		t.Fatalf("after the timeout: %s %v", st, note)
	}
	sent := eng.createCalls[0].body

	// 2. A retry after the lease finds the case closed: refused, but the row stays PENDING with a note.
	age()
	if _, err := pool.Exec(ctx, `UPDATE "case" SET state = 'CLOSED' WHERE id = $1::uuid`, pwiCaseID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateProductWorkItem(sysCtx, req); err == nil {
		t.Fatal("want the closed-case refusal")
	}
	if st, note := status(); st != "PENDING" || note == nil {
		t.Fatalf("an inherited row must stay PENDING with its note after a refusal: %s %v", st, note)
	}

	// 3. The case is reopened and GitHub's list now shows the issue: it is adopted, not created again.
	age()
	if _, err := pool.Exec(ctx, `UPDATE "case" SET state = 'OPEN' WHERE id = $1::uuid`, pwiCaseID); err != nil {
		t.Fatal(err)
	}
	eng.createErr = nil
	earlier := sent
	eng.list = []engineeringentity.IssueDetail{{Number: 501, NodeID: "I_amb", Title: "Ambiguous issue", Body: &earlier, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/501"}}
	resp, _, err := svc.CreateProductWorkItem(sysCtx, req)
	if err != nil || resp.Issue.Number != 501 {
		t.Fatalf("adopt: resp=%+v err=%v", resp, err)
	}
	if len(eng.createCalls) != 1 {
		t.Fatalf("GitHub was asked to create %d times, want 1 (the earlier issue adopted)", len(eng.createCalls))
	}
	if st, _ := status(); st != "COMPLETED" {
		t.Errorf("status = %s", st)
	}
}
