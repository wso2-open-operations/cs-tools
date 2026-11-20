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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Run against a real Postgres with migrations 0309 and 100031 applied; skipped
// without ENTITY_TEST_DATABASE_URL. Each test makes its own product issue and
// deletes it (and, by cascade, its commands and comments) afterwards.

type gcEnv struct {
	ctx  context.Context
	db   *repository.Scoped
	repo repository.GitHubCommandRepository
}

func gcSetup(t *testing.T) gcEnv {
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
	db := repository.NewScoped(pool)
	return gcEnv{ctx: repository.WithSystemIdentity(context.Background()), db: db, repo: repository.NewGitHubCommandRepository(db)}
}

func (e gcEnv) scalar(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var v string
	if err := e.db.QueryRow(e.ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return v
}

func (e gcEnv) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(e.ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// productIssue makes a PRODUCT_ISSUE work item with a link, removed at cleanup.
func (e gcEnv) productIssue(t *testing.T) string {
	t.Helper()
	id := e.scalar(t, `
		INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number('PRODUCT_ISSUE'), 'gc test', 'PRODUCT_ISSUE')
		RETURNING id::text`)
	t.Cleanup(func() {
		_, _ = e.db.Exec(e.ctx, `DELETE FROM github_command WHERE work_item_id = $1::uuid`, id)
		_, _ = e.db.Exec(e.ctx, `DELETE FROM comment WHERE work_item_id = $1::uuid`, id)
		_, _ = e.db.Exec(e.ctx, `DELETE FROM github_issue_link WHERE work_item_id = $1::uuid`, id)
		_, _ = e.db.Exec(e.ctx, `DELETE FROM work_item WHERE id = $1::uuid`, id)
	})
	return id
}

func gcPayload(kind string) json.RawMessage {
	switch kind {
	case repository.CommandSetBoardStatus:
		return json.RawMessage(`{"status":"WAITING_ON_PRODUCT_TEAM","rawStatus":"WOW"}`)
	case repository.CommandSetState:
		return json.RawMessage(`{"state":"closed","stateReason":"completed"}`)
	}
	return json.RawMessage(`{"body":"x"}`)
}

func TestGitHubCommandRepo_InsertIdempotencyAndPendingConflict(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	other := e.productIssue(t)

	in := repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetState, Payload: gcPayload(repository.CommandSetState), IdempotencyKey: "k1", RequestedBy: "a@example.test"}
	first, existed, err := e.repo.InsertCommand(e.ctx, in)
	if err != nil || existed || first.Status != "PENDING" || first.Attempts != 0 || first.RequestedBy != "a@example.test" {
		t.Fatalf("first = %+v existed=%v err=%v", first, existed, err)
	}
	again, existed, err := e.repo.InsertCommand(e.ctx, in)
	if err != nil || !existed || again.ID != first.ID {
		t.Fatalf("repeat = %+v existed=%v err=%v", again, existed, err)
	}
	// Same key on another item is another command.
	in2 := in
	in2.WorkItemID = other
	if c, existed, err := e.repo.InsertCommand(e.ctx, in2); err != nil || existed || c.ID == first.ID {
		t.Fatalf("other item: %+v existed=%v err=%v", c, existed, err)
	}

	// A second open close (new key) is refused with the typed error.
	in.IdempotencyKey = "k2"
	if _, _, err := e.repo.InsertCommand(e.ctx, in); !errors.Is(err, repository.ErrCommandPending) {
		t.Fatalf("second pending close: err = %v, want ErrCommandPending", err)
	}
	// A different kind is fine, and comments queue freely.
	if _, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetBoardStatus, Payload: gcPayload(repository.CommandSetBoardStatus), RequestedBy: "a@example.test"}); err != nil {
		t.Fatalf("move beside close: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandAddComment, Payload: gcPayload(repository.CommandAddComment), RequestedBy: "a@example.test"}); err != nil {
			t.Fatalf("comment %d: %v", i, err)
		}
	}
	// Once the close is no longer pending, a new one may be queued.
	if err := e.repo.MarkCancelled(e.ctx, first.ID, "no longer needed"); err != nil {
		t.Fatal(err)
	}
	in.IdempotencyKey = "k3"
	if _, _, err := e.repo.InsertCommand(e.ctx, in); err != nil {
		t.Fatalf("close after cancel: %v", err)
	}
	// The transaction helper leaves the caller's tx usable after the conflict.
	err = e.db.InTx(e.ctx, func(tx pgx.Tx) error {
		in.IdempotencyKey = "k4"
		if _, _, err := repository.InsertGitHubCommandTx(e.ctx, tx, in); !errors.Is(err, repository.ErrCommandPending) {
			t.Errorf("tx insert: err = %v", err)
		}
		var n int
		return tx.QueryRow(e.ctx, `SELECT count(*) FROM github_command WHERE work_item_id = $1::uuid`, wi).Scan(&n)
	})
	if err != nil {
		t.Fatalf("tx unusable after the conflict: %v", err)
	}
}

func TestGitHubCommandRepo_ClaimLease(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	c, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetState, Payload: gcPayload(repository.CommandSetState), RequestedBy: "a"})
	if err != nil {
		t.Fatal(err)
	}
	has := func(list []repository.GitHubCommand) bool {
		for _, x := range list {
			if x.ID == c.ID {
				return true
			}
		}
		return false
	}
	claimed, err := e.repo.ClaimDue(e.ctx, 1000)
	if err != nil || !has(claimed) {
		t.Fatalf("first claim: has=%v err=%v", has(claimed), err)
	}
	if again, err := e.repo.ClaimDue(e.ctx, 1000); err != nil || has(again) {
		t.Fatalf("second claim returned the leased command (err=%v)", err)
	}
	got, err := e.repo.Get(e.ctx, c.ID)
	if err != nil || got == nil || got.Status != "PENDING" {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	lease := time.Until(got.NextAttemptOn)
	if lease < 4*time.Minute || lease > 6*time.Minute {
		t.Errorf("lease = %v, want about 5 minutes", lease)
	}
	// A dead worker's lease runs out on its own.
	e.exec(t, `UPDATE github_command SET next_attempt_on = NOW() - INTERVAL '1 second' WHERE id = $1::uuid`, c.ID)
	if again, err := e.repo.ClaimDue(e.ctx, 1000); err != nil || !has(again) {
		t.Fatalf("expired lease not claimable (err=%v)", err)
	}
	// Not yet due: not claimed.
	e.exec(t, `UPDATE github_command SET next_attempt_on = NOW() + INTERVAL '1 hour' WHERE id = $1::uuid`, c.ID)
	if again, err := e.repo.ClaimDue(e.ctx, 1000); err != nil || has(again) {
		t.Fatalf("future command claimed (err=%v)", err)
	}
	if none, err := e.repo.Get(e.ctx, "00000000-0000-4000-8000-000000000000"); err != nil || none != nil {
		t.Fatalf("get unknown = %+v err=%v", none, err)
	}
}

func TestGitHubCommandRepo_Outcomes(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	other := e.productIssue(t)
	mk := func(kind string) repository.GitHubCommand {
		c, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: kind, Payload: gcPayload(kind), RequestedBy: "a"})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	get := func(id string) repository.GitHubCommand {
		c, err := e.repo.Get(e.ctx, id)
		if err != nil || c == nil {
			t.Fatalf("get %s: %v", id, err)
		}
		return *c
	}
	st := 429
	long := strings.Repeat("é", 400)

	// Reschedule keeps it PENDING with the new time and the cut reason.
	c := mk(repository.CommandSetState)
	next := time.Now().Add(90 * time.Second).Truncate(time.Microsecond)
	if err := e.repo.Reschedule(e.ctx, c.ID, 2, next, long, &st); err != nil {
		t.Fatal(err)
	}
	g := get(c.ID)
	if g.Status != "PENDING" || g.Attempts != 2 || !g.NextAttemptOn.Equal(next) || g.LastError == nil || len([]rune(*g.LastError)) != 300 || g.GitHubStatus == nil || *g.GitHubStatus != 429 {
		t.Fatalf("after reschedule: %+v", g)
	}

	// MarkFailed, then Retry puts it back with a fresh window; the error stays.
	st = 422
	if err := e.repo.MarkFailed(e.ctx, c.ID, 3, "rejected by GitHub", &st); err != nil {
		t.Fatal(err)
	}
	g = get(c.ID)
	if g.Status != "FAILED" || g.Attempts != 3 || *g.LastError != "rejected by GitHub" {
		t.Fatalf("after fail: %+v", g)
	}
	if err := e.repo.Reschedule(e.ctx, c.ID, 9, time.Now(), "late", nil); err != nil || get(c.ID).Attempts != 3 {
		t.Fatalf("a FAILED command must not be rescheduled (err=%v)", err)
	}
	e.exec(t, `UPDATE github_command SET window_started_on = NOW() - INTERVAL '2 days' WHERE id = $1::uuid`, c.ID)
	if _, err := e.repo.Retry(e.ctx, other, c.ID); !errors.Is(err, repository.ErrCommandNotFound) {
		t.Fatalf("retry on another item: %v", err)
	}
	r, err := e.repo.Retry(e.ctx, wi, c.ID)
	if err != nil || r.Status != "PENDING" || r.Attempts != 0 || r.LastError == nil || time.Since(r.WindowStartedOn) > time.Minute || time.Since(r.NextAttemptOn) > time.Minute {
		t.Fatalf("retry = %+v err=%v", r, err)
	}
	if _, err := e.repo.Retry(e.ctx, wi, c.ID); !errors.Is(err, repository.ErrCommandNotFailed) {
		t.Fatalf("retry of a pending command: %v", err)
	}

	// Retry of a failed close while another close is pending: ErrCommandPending.
	if err := e.repo.MarkFailed(e.ctx, c.ID, 1, "x", nil); err != nil {
		t.Fatal(err)
	}
	second := mk(repository.CommandSetState)
	if _, err := e.repo.Retry(e.ctx, wi, c.ID); !errors.Is(err, repository.ErrCommandPending) {
		t.Fatalf("retry beside a pending close: %v", err)
	}

	// Cancel, deliver.
	if err := e.repo.MarkCancelled(e.ctx, second.ID, "issue left WOC before delivery"); err != nil {
		t.Fatal(err)
	}
	if g := get(second.ID); g.Status != "CANCELLED" || *g.LastError != "issue left WOC before delivery" {
		t.Fatalf("after cancel: %+v", g)
	}
	cm := mk(repository.CommandAddComment)
	ok := 201
	if err := e.repo.MarkDelivered(e.ctx, cm.ID, 1, &ok); err != nil {
		t.Fatal(err)
	}
	if g := get(cm.ID); g.Status != "DELIVERED" || g.DeliveredOn == nil || g.LastError != nil || g.Attempts != 1 {
		t.Fatalf("after deliver: %+v", g)
	}
	// A delivered or cancelled command stays as it is.
	_ = e.repo.MarkFailed(e.ctx, cm.ID, 5, "late", nil)
	_ = e.repo.MarkCancelled(e.ctx, cm.ID, "late")
	if get(cm.ID).Status != "DELIVERED" {
		t.Fatal("a DELIVERED command was overwritten")
	}

	// ListForWorkItem: every PENDING/FAILED plus the 10 newest others, newest first.
	for i := 0; i < 12; i++ {
		d := mk(repository.CommandAddComment)
		if err := e.repo.MarkDelivered(e.ctx, d.ID, 1, nil); err != nil {
			t.Fatal(err)
		}
	}
	pend := mk(repository.CommandAddComment)
	list, err := e.repo.ListForWorkItem(e.ctx, wi)
	if err != nil {
		t.Fatal(err)
	}
	var open, done int
	for i, x := range list {
		if x.Status == "PENDING" || x.Status == "FAILED" {
			open++
		} else {
			done++
		}
		if i > 0 && list[i-1].CreatedOn.Before(x.CreatedOn) {
			t.Errorf("not newest first at %d", i)
		}
	}
	// open: the failed close (c) and pend; done: 10 of the 14 delivered/cancelled.
	if open != 2 || done != 10 || list[0].ID != pend.ID {
		t.Fatalf("list: open=%d done=%d first=%s want 2/10/%s (len %d)", open, done, list[0].ID, pend.ID, len(list))
	}
	if l, err := e.repo.ListForWorkItem(e.ctx, other); err != nil || l == nil || len(l) != 0 {
		t.Fatalf("empty list = %v err=%v (want a non-nil empty slice)", l, err)
	}
}

func TestGitHubCommandRepo_PortalCommentAdoptAndSetID(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	other := e.productIssue(t)

	post := func(workItem, content, key string) repository.PortalCommentResult {
		t.Helper()
		res, err := e.repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{
			WorkItemID: workItem, Content: content, GitHubBody: "**Ann** (CS) wrote:\n\n" + content, IdempotencyKey: key, RequestedBy: "ann@example.test"})
		if err != nil {
			t.Fatalf("insert portal comment: %v", err)
		}
		return res
	}
	a := post(wi, "first", "key-a")
	if a.Existed || a.Comment.Author != "ann@example.test" || a.Comment.Body != "first" || a.Command.Kind != "ADD_COMMENT" ||
		a.Command.CommentID == nil || *a.Command.CommentID != a.Comment.ID {
		t.Fatalf("comment = %+v", a)
	}
	if typ := e.scalar(t, `SELECT type::text FROM comment WHERE id = $1::uuid`, a.Comment.ID); typ != "WORK_NOTE" {
		t.Errorf("comment type = %s", typ)
	}
	if o := e.scalar(t, `SELECT origin || '/' || COALESCE(github_comment_id::text, 'null') FROM comment_github_ref WHERE comment_id = $1::uuid`, a.Comment.ID); o != "PORTAL/null" {
		t.Errorf("ref = %s", o)
	}
	var payload map[string]string
	if err := json.Unmarshal(a.Command.Payload, &payload); err != nil || payload["body"] != "**Ann** (CS) wrote:\n\nfirst" {
		t.Errorf("payload = %s (%v)", a.Command.Payload, err)
	}
	// Idempotent repeat: same comment and command, nothing new written.
	again := post(wi, "first", "key-a")
	if !again.Existed || again.Comment.ID != a.Comment.ID || again.Command.ID != a.Command.ID {
		t.Fatalf("repeat = %+v", again)
	}
	if n := e.scalar(t, `SELECT count(*)::text FROM comment WHERE work_item_id = $1::uuid`, wi); n != "1" {
		t.Errorf("%s comments after a repeat, want 1", n)
	}

	// Two identical comments, oldest adopted first; a different body and another
	// item match nothing; the same GitHub comment is not adopted twice.
	b := post(wi, "first", "key-b")
	adopt := func(workItem, body string, id int64) string {
		t.Helper()
		var got string
		err := e.db.InTx(e.ctx, func(tx pgx.Tx) error {
			var err error
			got, err = repository.AdoptPendingPortalComment(e.ctx, tx, workItem, body, id, "ann-gh", nil)
			return err
		})
		if err != nil {
			t.Fatalf("adopt: %v", err)
		}
		return got
	}
	if got := adopt(wi, "**Ann** (CS) wrote:\n\nother text", 9001); got != "" {
		t.Errorf("a different body adopted %s", got)
	}
	if got := adopt(other, "**Ann** (CS) wrote:\n\nfirst", 9002); got != "" {
		t.Errorf("another work item adopted %s", got)
	}
	body := "**Ann** (CS) wrote:\n\nfirst"
	if got := adopt(wi, body, 9003); got != a.Comment.ID {
		t.Fatalf("first adopt = %q, want the oldest %s", got, a.Comment.ID)
	}
	if o := e.scalar(t, `SELECT github_comment_id::text || '/' || author_login FROM comment_github_ref WHERE comment_id = $1::uuid`, a.Comment.ID); o != "9003/ann-gh" {
		t.Errorf("adopted ref = %s", o)
	}
	if st := e.scalar(t, `SELECT status FROM github_command WHERE comment_id = $1::uuid`, a.Comment.ID); st != "DELIVERED" {
		t.Errorf("an adopted comment's command is %s, want DELIVERED", st)
	}
	if got := adopt(wi, body, 9004); got != b.Comment.ID {
		t.Fatalf("second adopt = %q, want %s", got, b.Comment.ID)
	}
	if got := adopt(wi, body, 9005); got != "" {
		t.Errorf("third adopt = %q, want none left", got)
	}

	// SetPortalCommentGitHubID: only while NULL.
	c := post(wi, "third", "key-c")
	upd := time.Now().UTC().Truncate(time.Microsecond)
	set, err := e.repo.SetPortalCommentGitHubID(e.ctx, c.Comment.ID, 7001, "bot", &upd)
	if err != nil || !set {
		t.Fatalf("set = %v err=%v", set, err)
	}
	set, err = e.repo.SetPortalCommentGitHubID(e.ctx, c.Comment.ID, 7002, "someone", nil)
	if err != nil || set {
		t.Fatalf("second set = %v err=%v, want false", set, err)
	}
	if o := e.scalar(t, `SELECT github_comment_id::text || '/' || author_login FROM comment_github_ref WHERE comment_id = $1::uuid`, c.Comment.ID); o != "7001/bot" {
		t.Errorf("ref after second set = %s", o)
	}
	// An id already on another comment is reported, not swallowed.
	d := post(wi, "fourth", "key-d")
	if _, err := e.repo.SetPortalCommentGitHubID(e.ctx, d.Comment.ID, 7001, "", nil); !errors.Is(err, repository.ErrGitHubCommentIDTaken) {
		t.Errorf("duplicate id: err = %v", err)
	}
	// A GitHub-origin comment is never touched.
	e.exec(t, `UPDATE comment_github_ref SET github_comment_id = NULL WHERE comment_id = $1::uuid`, a.Comment.ID)
	e.exec(t, `UPDATE comment_github_ref SET origin = 'GITHUB' WHERE comment_id = $1::uuid`, a.Comment.ID)
	if set, err := e.repo.SetPortalCommentGitHubID(e.ctx, a.Comment.ID, 7003, "", nil); err != nil || set {
		t.Errorf("set on a GITHUB ref = %v err=%v", set, err)
	}
}

func TestGitHubCommandRepo_LinkForCommand(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	owner, repo := "gc-org-"+wi[:8], "gc-repo"
	if _, err := e.repo.LinkForCommand(e.ctx, wi); !errors.Is(err, repository.ErrProductIssueLinkNotFound) {
		t.Fatalf("no link: err = %v", err)
	}
	e.exec(t, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id, board_status)
		VALUES ($1::uuid, $2, $3, 5, $4, 'WAITING_ON_CS')`, wi, owner, repo, "I_gc_"+wi)
	l, err := e.repo.LinkForCommand(e.ctx, wi)
	if err != nil || l.BoardProjectNumber != nil || l.Org != owner || l.IssueNumber != 5 || l.BoardItemID != nil || *l.BoardStatus != "WAITING_ON_CS" || l.LinkState != "ACTIVE" || l.GitHubState != "OPEN" {
		t.Fatalf("link = %+v err=%v", l, err)
	}
	mapping := func(name string, board int, active bool) {
		e.exec(t, `INSERT INTO product_repo_mapping (created_by, updated_by, product_name, owner, repository, github_label, is_active, board_project_number)
			VALUES ('t', 't', $1, upper($2), $3, 'l', $4, $5)`, name+wi, owner, repo, active, board)
		t.Cleanup(func() { _, _ = e.db.Exec(e.ctx, `DELETE FROM product_repo_mapping WHERE product_name LIKE $1`, "%"+wi) })
	}
	mapping("inactive", 3, false)
	mapping("active-high", 9, true)
	mapping("active-low", 7, true)
	if l, _ := e.repo.LinkForCommand(e.ctx, wi); l.BoardProjectNumber == nil || *l.BoardProjectNumber != 7 {
		t.Fatalf("board = %v, want the lowest active (7)", l.BoardProjectNumber)
	}
	if err := e.repo.SetBoardItemID(e.ctx, wi, "307799589"); err != nil {
		t.Fatal(err)
	}
	l, _ = e.repo.LinkForCommand(e.ctx, wi)
	if l.BoardItemID == nil || *l.BoardItemID != "307799589" || *l.BoardProjectNumber != 7 {
		t.Fatalf("after set: item=%v board=%v (only the item id is written; the board is still the mapping's)", l.BoardItemID, l.BoardProjectNumber)
	}
	if n := e.scalar(t, `SELECT (board_project_number IS NULL)::text FROM github_issue_link WHERE work_item_id = $1::uuid`, wi); n != "true" {
		t.Error("SetBoardItemID wrote board_project_number")
	}
}

func TestGitHubCommandRLS_InternalOnly(t *testing.T) {
	ctx, tx := piTx(t)
	if piScalar[int](t, ctx, tx, `SELECT count(*) FROM pg_roles WHERE rolname = 'csm_app'`) == 0 {
		t.Skip("role csm_app does not exist")
	}
	if piScalar[bool](t, ctx, tx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = 'csm_app'`) {
		t.Skip("csm_app bypasses row-level security")
	}
	project := piScalar[string](t, ctx, tx, `INSERT INTO project (created_on, updated_on, created_by, updated_by, key)
		VALUES (NOW(), NOW(), 'test', 'test', 'GCRLSKEY') RETURNING id::text`)
	pi := piNewWorkItem(t, ctx, tx, "PRODUCT_ISSUE", "gc rls", project)
	piExec(t, ctx, tx, `INSERT INTO github_command (work_item_id, kind, payload, requested_by) VALUES ($1, 'ADD_COMMENT', '{"body":"x"}', 't')`, pi)
	piExec(t, ctx, tx, `GRANT SELECT, INSERT, UPDATE, DELETE ON github_command TO csm_app`)

	piAsApp(t, ctx, tx, false, "gc-rls-customer@example.test", "{"+project+"}")
	if n := piScalar[int](t, ctx, tx, `SELECT count(*) FROM github_command`); n != 0 {
		t.Errorf("customer reads %d github_command rows, want 0", n)
	}
	e := piExpectError(ctx, tx, `INSERT INTO github_command (work_item_id, kind, payload, requested_by) VALUES ($1, 'SET_STATE', '{}', 't')`, pi)
	if e == nil || e.Code != "42501" {
		t.Errorf("customer insert: %v, want 42501", e)
	}
	piBackToSuper(t, ctx, tx)

	piAsApp(t, ctx, tx, true, "", "")
	if n := piScalar[int](t, ctx, tx, `SELECT count(*) FROM github_command WHERE work_item_id = $1`, pi); n != 1 {
		t.Errorf("internal session reads %d rows, want 1", n)
	}
}

func TestGitHubCommandRepo_WriteBackReadsAndAdoptionDeliversFailed(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	e.exec(t, `INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id) VALUES ($1::uuid, 'o', 'r', 9, 'I_x')`, wi)

	res, err := e.repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{WorkItemID: wi, Content: "t", GitHubBody: "B", IdempotencyKey: "kk", RequestedBy: "u@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if c, err := e.repo.FindCommandByKey(e.ctx, wi, "kk"); err != nil || c == nil || c.ID != res.Command.ID {
		t.Fatalf("find by key: %+v %v", c, err)
	}
	if c, err := e.repo.FindCommandByKey(e.ctx, wi, "nope"); err != nil || c != nil {
		t.Fatalf("find unknown key: %+v %v", c, err)
	}
	if id, err := e.repo.PortalCommentGitHubID(e.ctx, res.Comment.ID); err != nil || id != nil {
		t.Fatalf("id before = %v %v", id, err)
	}
	if ok, err := e.repo.GitHubCommentRecorded(e.ctx, 31337); err != nil || ok {
		t.Fatalf("recorded before = %v %v", ok, err)
	}

	// A FAILED command is delivered by adoption (GitHub has the comment).
	e.exec(t, `UPDATE github_command SET status = 'FAILED', last_error = 'x' WHERE id = $1::uuid`, res.Command.ID)
	err = e.db.InTx(e.ctx, func(tx pgx.Tx) error {
		got, err := repository.AdoptPendingPortalComment(e.ctx, tx, wi, "B", 31337, "bot", nil)
		if got != res.Comment.ID {
			t.Errorf("adopted %q", got)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if st := e.scalar(t, `SELECT status || '/' || COALESCE(last_error, '-') FROM github_command WHERE id = $1::uuid`, res.Command.ID); st != "DELIVERED/-" {
		t.Errorf("command = %s", st)
	}
	if id, err := e.repo.PortalCommentGitHubID(e.ctx, res.Comment.ID); err != nil || id == nil || *id != 31337 {
		t.Fatalf("id after = %v %v", id, err)
	}
	if ok, err := e.repo.GitHubCommentRecorded(e.ctx, 31337); err != nil || !ok {
		t.Fatalf("recorded after = %v %v", ok, err)
	}

	// LinkForCommand answers for a product issue only.
	if l, err := e.repo.LinkForCommand(e.ctx, wi); err != nil || l.Org != "o" || l.IssueNumber != 9 {
		t.Fatalf("link: %+v %v", l, err)
	}

	// Domain shape: next attempt only while pending; the move's target status.
	mv, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetBoardStatus, Payload: gcPayload(repository.CommandSetBoardStatus), RequestedBy: "u@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	d := mv.Domain()
	if d.NextAttemptOn == nil || d.TargetStatus == nil || *d.TargetStatus != "WAITING_ON_PRODUCT_TEAM" || d.CommentID != nil {
		t.Errorf("domain = %+v", d)
	}
}

func TestGitHubCommandRepo_UserDisplayName(t *testing.T) {
	e := gcSetup(t)
	email := "gc-name-test@example.test"
	e.exec(t, `DELETE FROM "user" WHERE user_name = 'gc-name-test'`)
	e.exec(t, `INSERT INTO "user" (created_on, updated_on, created_by, updated_by, user_name, email, first_name, last_name, is_active) VALUES (NOW(), NOW(), 't', 't', 'gc-name-test', $1, 'Ann', 'Lee', true)`, email)
	t.Cleanup(func() { _, _ = e.db.Exec(e.ctx, `DELETE FROM "user" WHERE user_name = 'gc-name-test'`) })
	if n, err := e.repo.UserDisplayName(e.ctx, "GC-Name-Test@Example.Test"); err != nil || n != "Ann Lee" {
		t.Fatalf("name = %q %v", n, err)
	}
	e.exec(t, `UPDATE "user" SET name = 'Annabel Lee' WHERE user_name = 'gc-name-test'`)
	if n, _ := e.repo.UserDisplayName(e.ctx, email); n != "Annabel Lee" {
		t.Errorf("name = %q", n)
	}
	if n, err := e.repo.UserDisplayName(e.ctx, "nobody-gc@example.test"); err != nil || n != "" {
		t.Errorf("unknown = %q %v", n, err)
	}
}

func TestGitHubCommandRepo_IdempotencyKeyReusedAcrossKinds(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	if _, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetState, Payload: gcPayload(repository.CommandSetState), IdempotencyKey: "same", RequestedBy: "u"}); err != nil {
		t.Fatal(err)
	}
	// Another kind with the key: refused, not replayed.
	if _, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: wi, Kind: repository.CommandSetBoardStatus, Payload: gcPayload(repository.CommandSetBoardStatus), IdempotencyKey: "same", RequestedBy: "u"}); !errors.Is(err, repository.ErrIdempotencyKeyReused) {
		t.Errorf("insert with another kind: %v", err)
	}
	if _, err := e.repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{WorkItemID: wi, Content: "c", GitHubBody: "c", IdempotencyKey: "same", RequestedBy: "u"}); !errors.Is(err, repository.ErrIdempotencyKeyReused) {
		t.Errorf("comment with a close's key: %v", err)
	}
	if n := e.scalar(t, `SELECT count(*)::text FROM comment WHERE work_item_id = $1::uuid`, wi); n != "0" {
		t.Errorf("a refused comment left %s comment rows", n)
	}
}

func TestGitHubCommandRepo_BeginAttemptLeaseAndRetryKeepsDeliveryAttempted(t *testing.T) {
	e := gcSetup(t)
	wi := e.productIssue(t)
	res, err := e.repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{WorkItemID: wi, Content: "c", GitHubBody: "c", RequestedBy: "u"})
	if err != nil {
		t.Fatal(err)
	}
	id := res.Command.ID
	claimed, err := e.repo.ClaimDue(e.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	var mine *repository.GitHubCommand
	for i := range claimed {
		if claimed[i].ID == id {
			mine = &claimed[i]
		}
	}
	if mine == nil || mine.DeliveryAttempted {
		t.Fatalf("claimed = %+v", mine)
	}
	ok, err := e.repo.BeginAttempt(e.ctx, id, 1)
	if err != nil || !ok {
		t.Fatalf("begin = %v %v", ok, err)
	}
	if v := e.scalar(t, `SELECT delivery_attempted::text || '/' || attempts::text || '/' || (next_attempt_on > NOW() + interval '4 minutes')::text FROM github_command WHERE id = $1::uuid`, id); v != "true/1/true" {
		t.Errorf("after begin = %s (lease must be renewed to a full 5 minutes)", v)
	}
	// The lease running out: another worker may own it, so begin refuses and writes nothing.
	e.exec(t, `UPDATE github_command SET next_attempt_on = NOW() - interval '1 second', attempts = 7 WHERE id = $1::uuid`, id)
	if ok, err := e.repo.BeginAttempt(e.ctx, id, 8); err != nil || ok {
		t.Fatalf("begin after the lease = %v %v, want refused", ok, err)
	}
	if v := e.scalar(t, `SELECT attempts::text FROM github_command WHERE id = $1::uuid`, id); v != "7" {
		t.Errorf("attempts = %s after a refused begin", v)
	}
	// Retry resets attempts but never delivery_attempted.
	e.exec(t, `UPDATE github_command SET status = 'FAILED' WHERE id = $1::uuid`, id)
	if _, err := e.repo.Retry(e.ctx, wi, id); err != nil {
		t.Fatal(err)
	}
	if v := e.scalar(t, `SELECT delivery_attempted::text || '/' || attempts::text FROM github_command WHERE id = $1::uuid`, id); v != "true/0" {
		t.Errorf("after retry = %s", v)
	}
}
