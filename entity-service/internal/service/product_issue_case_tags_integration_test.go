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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The s_dip case tag rule against PostgreSQL, over the real case service and the
// fixtures of the product work item integration test. Skipped without
// ENTITY_TEST_DATABASE_URL.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run CaseTagRule

const (
	ctrCase2ID   = "77000000-0000-4000-8000-0000000000b2"
	ctrCase3ID   = "77000000-0000-4000-8000-0000000000b3"
	ctrCase4ID   = "77000000-0000-4000-8000-0000000000b4"
	ctrPerson    = "person@test.local"
	ctrSDIP      = "s_dip"
	ctrIssueBase = "77000000-0000-4000-8000-0000000000c"
)

type ctrEnv struct {
	t     *testing.T
	pool  *pgxpool.Pool
	cases CaseService
	rule  ProductIssueCaseTagRule
	ctx   context.Context
	next  int
}

func newCTREnv(t *testing.T) *ctrEnv {
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := context.Background()
	for _, id := range []string{ctrCase2ID, ctrCase3ID, ctrCase4ID} {
		if _, err := pool.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, account_id, project_id)
			VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'CTR-' || right($4::text, 2), 'CTR-W-' || right($4::text, 2), 'ctr case', 'CASE', $2, $3)`, id, pwiAccountID, pwiProjectID, id); err != nil {
			t.Fatalf("seed case: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO "case" (id, autoclose_enabled, state) VALUES ($1, false, 'OPEN')`, id); err != nil {
			t.Fatalf("seed case row: %v", err)
		}
	}
	cases := NewCaseService(repository.NewCaseRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool), nil, internalAccess{}, repository.NewProjectContactRepository(pool))
	return &ctrEnv{t: t, pool: pool, cases: cases, ctx: repository.WithSystemIdentity(pwCtx()),
		rule: NewProductIssueCaseTagRule(repository.NewProductIssueCaseTagRepository(repository.NewScoped(pool)), cases)}
}

func (e *ctrEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec (%.70s): %v", sql, err)
	}
}

// issue inserts a product issue with a GitHub link and returns its id. board is
// the board status ("" = not on a board); state the GitHub state.
func (e *ctrEnv) issue(number, board, state string) string {
	e.t.Helper()
	e.next++
	id := ctrIssueBase + string(rune('0'+e.next))
	e.exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id)
		VALUES ($1, NOW(), NOW(), 'test', 'test', $2, 'ctr issue', 'PRODUCT_ISSUE', $3, $4)`, id, number, pwiAccountID, pwiProjectID)
	e.exec(`INSERT INTO product_issue (id) VALUES ($1)`, id)
	var b any
	if board != "" {
		b = board
	}
	e.exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, github_state, board_status)
		VALUES ($1, 'ctr-org', 'ctr-repo', $2, $3, $4)`, id, 900+e.next, state, b)
	return id
}

func (e *ctrEnv) link(issueID, caseID string, active bool) {
	e.t.Helper()
	e.exec(`INSERT INTO work_item_link (from_id, to_id, kind, source, is_active, created_by, updated_by)
		VALUES ($1, $2, 'AFFECTS', 'MANUAL', $3, 'test', 'test')`, issueID, caseID, active)
}

func (e *ctrEnv) board(issueID, board string) {
	e.exec(`UPDATE github_issue_link SET board_status = $2 WHERE work_item_id = $1`, issueID, board)
}
func (e *ctrEnv) ghState(issueID, state string) {
	e.exec(`UPDATE github_issue_link SET github_state = $2 WHERE work_item_id = $1`, issueID, state)
}

func (e *ctrEnv) count(sql string, args ...any) int {
	e.t.Helper()
	return pwiCount(e.t, e.pool, sql, args...)
}

func (e *ctrEnv) tagCount(caseID, label string) int {
	return e.count(`SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = lower($2)`, caseID, label)
}

func (e *ctrEnv) recorded(caseID string) bool {
	return e.count(`SELECT count(*) FROM case_tag_automation WHERE case_id = $1::uuid AND tag_label = 's_dip'`, caseID) == 1
}

func (e *ctrEnv) autoNotes(caseID string) int {
	return e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid AND type = 'WORK_NOTE' AND content LIKE 's_dip % automatically:%'`, caseID)
}

func (e *ctrEnv) lastNote(caseID string) string {
	var c string
	if err := e.pool.QueryRow(context.Background(), `SELECT content FROM comment WHERE work_item_id = $1::uuid AND content LIKE 's_dip % automatically:%' ORDER BY created_on DESC, id DESC LIMIT 1`, caseID).Scan(&c); err != nil {
		e.t.Fatalf("last note: %v", err)
	}
	return c
}

func (e *ctrEnv) applyCase(caseID string) {
	e.t.Helper()
	if err := e.rule.ApplyCaseTagRuleForCase(e.ctx, caseID); err != nil {
		e.t.Fatalf("rule for case: %v", err)
	}
}

func (e *ctrEnv) applyIssue(issueID string) {
	e.t.Helper()
	if err := e.rule.ApplyCaseTagRuleForWorkItem(e.ctx, issueID); err != nil {
		e.t.Fatalf("rule for work item: %v", err)
	}
}

// want asserts the tag count, the record and the number of automatic notes.
func (e *ctrEnv) want(name, caseID string, tags int, recorded bool, notes int) {
	e.t.Helper()
	if got := e.tagCount(caseID, ctrSDIP); got != tags {
		e.t.Errorf("%s: s_dip attachments = %d, want %d", name, got, tags)
	}
	if got := e.recorded(caseID); got != recorded {
		e.t.Errorf("%s: recorded = %v, want %v", name, got, recorded)
	}
	if got := e.autoNotes(caseID); got != notes {
		e.t.Errorf("%s: automatic notes = %d, want %d", name, got, notes)
	}
}

func (e *ctrEnv) personRemoves(caseID string) {
	e.t.Helper()
	var tagID string
	if err := e.pool.QueryRow(context.Background(), `SELECT tag.id::text FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip' LIMIT 1`, caseID).Scan(&tagID); err != nil {
		e.t.Fatalf("find tag: %v", err)
	}
	if err := e.cases.RemoveCaseTagAs(e.ctx, caseID, tagID, ctrPerson); err != nil {
		e.t.Fatalf("person removes: %v", err)
	}
}

func (e *ctrEnv) personAdds(caseID string) {
	e.t.Helper()
	if _, err := e.cases.AddCaseTagAs(e.ctx, caseID, ctrSDIP, ctrPerson); err != nil {
		e.t.Fatalf("person adds: %v", err)
	}
}

// The create flow adds s_dip, then the status changes converge it.
func TestCaseTagRuleIntegration_CreateThenStatusChanges(t *testing.T) {
	e := newCTREnv(t)
	eng := &fakePWEng{
		createRef: engineeringentity.IssueRef{ID: 9, NodeID: "I_ctr1", Number: 501},
		detail: engineeringentity.IssueDetail{NodeID: "I_ctr1", Number: 501, HTMLURL: "https://github.com/pwi-org/pwi-repo/issues/501",
			CreatedAt: "2026-11-18T10:00:00Z"},
	}
	svc := WithProductCaseTagRule(NewProductWorkItemService(eng, repository.NewProductWorkItemRepository(repository.NewScoped(e.pool)),
		repository.NewProductRepoMappingRepository(e.pool), e.cases, internalAccess{}), e.rule)
	resp, _, err := svc.CreateProductWorkItem(e.ctx, domain.CreateProductWorkItemRequest{CaseID: pwiCaseID, IdempotencyKey: "ctr-1",
		Title: "Rule issue", IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High"})
	if err != nil || resp.WorkItem == nil {
		t.Fatalf("create: %v %+v", err, resp)
	}
	wi := resp.WorkItem.ID
	e.want("after create", pwiCaseID, 1, true, 1)
	if want := "s_dip added automatically: product issue " + resp.WorkItem.Number + " is open"; e.lastNote(pwiCaseID) != want {
		t.Errorf("note = %q, want %q", e.lastNote(pwiCaseID), want)
	}
	if n := e.count(`SELECT count(*) FROM work_item_tag WHERE work_item_id = $1::uuid AND created_by = $2`, pwiCaseID, productIssueTagActor); n != 1 {
		t.Errorf("tag attachments by the system actor = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid AND type = 'WORK_NOTE'`, pwiCaseID); n != 2 {
		t.Errorf("work notes = %d, want 2 (filed + added)", n)
	}

	e.applyCase(pwiCaseID)
	e.applyIssue(wi)
	e.want("second run", pwiCaseID, 1, true, 1)

	e.board(wi, "RESOLVED")
	e.applyIssue(wi)
	e.want("resolved", pwiCaseID, 0, false, 2)
	if want := "s_dip removed automatically: every linked product issue is resolved or closed"; e.lastNote(pwiCaseID) != want {
		t.Errorf("note = %q, want %q", e.lastNote(pwiCaseID), want)
	}
	e.applyIssue(wi)
	e.want("resolved, second run", pwiCaseID, 0, false, 2)

	e.board(wi, "REOPENED")
	e.applyIssue(wi)
	e.want("reopened", pwiCaseID, 1, true, 3)

	e.ghState(wi, "CLOSED")
	e.applyIssue(wi)
	e.want("github closed", pwiCaseID, 0, false, 4)

	e.ghState(wi, "OPEN")
	e.exec(`UPDATE github_issue_link SET board_status = NULL WHERE work_item_id = $1`, wi)
	e.applyIssue(wi)
	e.want("open, not on a board", pwiCaseID, 1, true, 5)
}

func TestCaseTagRuleIntegration_TwoIssuesOneCase(t *testing.T) {
	e := newCTREnv(t)
	a, b := e.issue("PICTR01", "IN_PROGRESS", "OPEN"), e.issue("PICTR02", "WAITING_ON_PRODUCT_TEAM", "OPEN")
	e.link(a, ctrCase2ID, true)
	e.link(b, ctrCase2ID, true)
	e.applyCase(ctrCase2ID)
	e.want("two open", ctrCase2ID, 1, true, 1)
	if want := "s_dip added automatically: product issues PICTR01, PICTR02 are open"; e.lastNote(ctrCase2ID) != want {
		t.Errorf("note = %q", e.lastNote(ctrCase2ID))
	}
	e.board(a, "RESOLVED")
	e.applyIssue(a)
	e.want("one resolved", ctrCase2ID, 1, true, 1)
	e.board(b, "RESOLVED")
	e.applyIssue(b)
	e.want("both resolved", ctrCase2ID, 0, false, 2)
}

func TestCaseTagRuleIntegration_ManualTagIsNeverRemoved(t *testing.T) {
	e := newCTREnv(t)
	e.personAdds(ctrCase2ID)
	a := e.issue("PICTR03", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)
	e.applyCase(ctrCase2ID)
	e.want("manual tag, issue open", ctrCase2ID, 1, false, 0)
	e.board(a, "RESOLVED")
	e.applyIssue(a)
	e.want("manual tag, issue resolved", ctrCase2ID, 1, false, 0)
	e.exec(`UPDATE work_item_link SET is_active = false WHERE from_id = $1`, a)
	e.applyIssue(a)
	e.want("manual tag, issue unlinked", ctrCase2ID, 1, false, 0)
}

func TestCaseTagRuleIntegration_ManualRemovalIsUndoneWhileOpen(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR04", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)
	e.applyCase(ctrCase2ID)
	e.want("added", ctrCase2ID, 1, true, 1)
	e.personRemoves(ctrCase2ID)
	e.applyCase(ctrCase2ID)
	e.want("re-added", ctrCase2ID, 1, true, 2)

	// A person removes it and no issue is open: nothing happens, and the stale record goes.
	e.personRemoves(ctrCase2ID)
	e.board(a, "CLOSED")
	e.applyCase(ctrCase2ID)
	e.want("removed by a person, issue closed", ctrCase2ID, 0, false, 2)
}

func TestCaseTagRuleIntegration_PersonTakesOverARecordedTag(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR05", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)
	e.applyCase(ctrCase2ID)
	// A person removes the system's tag and adds their own while the record still stands.
	e.personRemoves(ctrCase2ID)
	e.personAdds(ctrCase2ID)
	e.board(a, "RESOLVED")
	e.applyCase(ctrCase2ID)
	e.want("taken over", ctrCase2ID, 1, false, 1)
}

func TestCaseTagRuleIntegration_UnlinkAndUntouchedCases(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR06", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)

	// The manual link and unlink hooks, over the real services.
	svc := WithProductIssueCaseTagRule(NewProductIssueService(&fakePWEng{}, repository.NewProductIssueRepository(repository.NewScoped(e.pool)), internalAccess{}), e.rule)
	if _, _, err := svc.Link(e.ctx, domain.LinkProductIssueRequest{CaseID: ctrCase2ID, WorkItemID: a}); err != nil {
		t.Fatalf("link: %v", err)
	}
	e.want("after link", ctrCase2ID, 1, true, 1)
	if err := svc.Unlink(e.ctx, ctrCase2ID, a); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	e.want("after unlink", ctrCase2ID, 0, false, 2)
	if want := "s_dip removed automatically: no product issue is linked to the case any more"; e.lastNote(ctrCase2ID) != want {
		t.Errorf("note = %q", e.lastNote(ctrCase2ID))
	}
	if _, _, err := svc.Link(e.ctx, domain.LinkProductIssueRequest{CaseID: ctrCase2ID, WorkItemID: a}); err != nil {
		t.Fatalf("relink: %v", err)
	}
	e.want("after relink", ctrCase2ID, 1, true, 3)

	// A case with no AFFECTS link at all is never touched, tag or no tag, record or no record.
	e.personAdds(ctrCase4ID)
	e.exec(`INSERT INTO case_tag_automation (case_id, tag_label) VALUES ($1, 's_dip')`, ctrCase4ID)
	e.applyCase(ctrCase4ID)
	e.want("no link", ctrCase4ID, 1, true, 0)
	// A non-case id is not an error.
	e.applyCase("77000000-0000-4000-8000-0000000000ff")
}

func TestCaseTagRuleIntegration_PatchTagIsNeverRemoved(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR07", "OPEN", "OPEN")
	e.link(a, ctrCase3ID, true)
	if _, err := e.cases.AddCaseTagAs(e.ctx, ctrCase3ID, "patch", ctrPerson); err != nil {
		t.Fatal(err)
	}
	e.applyCase(ctrCase3ID)
	e.want("open", ctrCase3ID, 1, true, 1)
	e.board(a, "RESOLVED")
	e.applyIssue(a)
	e.want("resolved", ctrCase3ID, 0, false, 2)
	if n := e.tagCount(ctrCase3ID, "patch"); n != 1 {
		t.Errorf("patch tag count = %d, want 1", n)
	}
}

// Two runs at once for one case add the tag once and write one note.
func TestCaseTagRuleIntegration_ConcurrentRunsSerialise(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR08", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)
	errs := make(chan error, 6)
	for range 6 {
		go func() { errs <- e.rule.ApplyCaseTagRuleForCase(e.ctx, ctrCase2ID) }()
	}
	for range 6 {
		if err := <-errs; err != nil {
			t.Errorf("run: %v", err)
		}
	}
	e.want("six concurrent runs", ctrCase2ID, 1, true, 1)
}

// A customer-scoped session cannot read case_tag_automation.
func TestCaseTagRuleIntegration_CustomerCannotReadTheRecord(t *testing.T) {
	e := newCTREnv(t)
	a := e.issue("PICTR09", "OPEN", "OPEN")
	e.link(a, ctrCase2ID, true)
	e.applyCase(ctrCase2ID)
	if !e.recorded(ctrCase2ID) {
		t.Fatal("fixture: no record")
	}
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
	count := func(ctx context.Context, sql string) int {
		t.Helper()
		var n int
		if err := scoped.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Skipf("csm_app cannot run the check (role missing or ungranted): %v", err)
		}
		return n
	}
	member := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ProjectIDs: []string{pwiProjectID}, ViewerEmail: pwiMember})
	if n := count(member, `SELECT count(*) FROM work_item WHERE id = '`+ctrCase2ID+`'::uuid`); n != 1 {
		t.Fatalf("the member sees %d of the case, want 1 (the check is meaningless otherwise)", n)
	}
	if n := count(member, `SELECT count(*) FROM case_tag_automation`); n != 0 {
		t.Errorf("a customer session reads %d case_tag_automation rows", n)
	}
	internal := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true})
	if n := count(internal, `SELECT count(*) FROM case_tag_automation WHERE case_id = '`+ctrCase2ID+`'::uuid`); n != 1 {
		t.Errorf("an internal session reads %d rows, want 1", n)
	}
	// And it cannot write one.
	if err := scoped.InTx(member, func(tx pgx.Tx) error {
		_, err := tx.Exec(member, `INSERT INTO case_tag_automation (case_id, tag_label) VALUES ($1::uuid, 'x')`, ctrCase3ID)
		return err
	}); err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Errorf("a customer session inserted a record: %v", err)
	}
}
