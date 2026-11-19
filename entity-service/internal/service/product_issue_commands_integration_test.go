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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The write-back worker over the real command repository and the real
// engineering entity client, against a fake engineering entity (httptest).
// Skipped without ENTITY_TEST_DATABASE_URL.

type pwcRequest struct {
	method, path, query string
	body                string
}

// pwcEng is a fake engineering entity: routes are "METHOD /path" (exact path), the
// answer of each is a function; an unrouted call is a 500.
type pwcEng struct {
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	calls  []pwcRequest
	srv    *httptest.Server
}

func newPwcEng(t *testing.T) *pwcEng {
	t.Helper()
	e := &pwcEng{routes: map[string]http.HandlerFunc{}}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		e.mu.Lock()
		e.calls = append(e.calls, pwcRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		h := e.routes[r.Method+" "+r.URL.Path]
		e.mu.Unlock()
		if h == nil {
			http.Error(w, "unrouted", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		h(w, r)
	}))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *pwcEng) on(method, path string, h http.HandlerFunc) { e.routes[method+" "+path] = h }

func (e *pwcEng) count(method, path string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.calls {
		if c.method == method && c.path == path {
			n++
		}
	}
	return n
}

func (e *pwcEng) bodyOf(method, path string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.calls {
		if c.method == method && c.path == path {
			return c.body
		}
	}
	return ""
}

func (e *pwcEng) total() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func answer(status int, body string, headers ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for i := 0; i+1 < len(headers); i += 2 {
			w.Header().Set(headers[i], headers[i+1])
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

const (
	pwcIssue    = "/orgs/example-org/repos/example-org/example-repo/issues/1"
	pwcComments = pwcIssue + "/comments"
	pwcFields   = "/orgs/example-org/projects/1/fields"
	pwcItems    = "/orgs/example-org/projects/1/items-with-content"
)

type pwcEnv struct {
	*pwhEnv
	eng    *pwcEng
	repo   repository.GitHubCommandRepository
	worker *ProductIssueCommandWorker
}

func pwcSetup(t *testing.T) *pwcEnv {
	t.Helper()
	h := pwhSeed(t)
	eng := newPwcEng(t)
	repo := repository.NewGitHubCommandRepository(repository.NewScoped(h.pool))
	w := NewProductIssueCommandWorker(repo, engineeringentity.New(eng.srv.URL, engineeringentity.ClientCredentialsConfig{}), time.Hour)
	return &pwcEnv{pwhEnv: h, eng: eng, repo: repo, worker: w}
}

func (e *pwcEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec %.60s: %v", sql, err)
	}
}

func (e *pwcEnv) waitingOnCS() {
	e.exec(`UPDATE github_issue_link SET board_status = 'WAITING_ON_CS', board_status_raw = 'WOC' WHERE work_item_id = $1::uuid`, e.wi1)
}

func (e *pwcEnv) enqueue(kind string) repository.GitHubCommand {
	e.t.Helper()
	payload := map[string]string{
		repository.CommandSetBoardStatus: `{"status":"WAITING_ON_PRODUCT_TEAM","rawStatus":"WOW"}`,
		repository.CommandSetState:       `{"state":"closed","stateReason":"completed"}`,
	}[kind]
	c, _, err := e.repo.InsertCommand(e.ctx, repository.NewGitHubCommand{WorkItemID: e.wi1, Kind: kind, Payload: json.RawMessage(payload), RequestedBy: "a@wso2.com"})
	if err != nil {
		e.t.Fatal(err)
	}
	return c
}

func (e *pwcEnv) comment(text string) repository.PortalCommentResult {
	e.t.Helper()
	res, err := e.repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{WorkItemID: e.wi1, Content: text, GitHubBody: "**A** (CS) wrote:\n\n" + text, RequestedBy: "a@wso2.com"})
	if err != nil {
		e.t.Fatal(err)
	}
	return res
}

func (e *pwcEnv) run() {
	e.t.Helper()
	if _, err := e.worker.RunOnce(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

type pwcRow struct {
	status, lastError string
	attempts          int
	ghStatus          *int
	next              time.Time
}

func (e *pwcEnv) row(id string) pwcRow {
	e.t.Helper()
	var r pwcRow
	var le *string
	err := e.pool.QueryRow(context.Background(), `SELECT status, COALESCE(last_error, ''), attempts, github_status, next_attempt_on FROM github_command WHERE id = $1::uuid`, id).
		Scan(&r.status, &le, &r.attempts, &r.ghStatus, &r.next)
	if err != nil {
		e.t.Fatal(err)
	}
	r.lastError = *le
	return r
}

func (e *pwcEnv) makeDue(id string) {
	e.exec(`UPDATE github_command SET next_attempt_on = NOW() - interval '1 second' WHERE id = $1::uuid`, id)
}

func (e *pwcEnv) refGitHubID(commentID string) *int64 {
	e.t.Helper()
	var id *int64
	if err := e.pool.QueryRow(context.Background(), `SELECT github_comment_id FROM comment_github_ref WHERE comment_id = $1::uuid`, commentID).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestProductIssueCommands_AddCommentSuccess(t *testing.T) {
	e := pwcSetup(t)
	queueBefore := e.count(`SELECT count(*) FROM github_outbound_queue`)
	res := e.comment("hello")
	// A portal comment never reaches the customer-repository outbound queue.
	if n := e.count(`SELECT count(*) FROM github_outbound_queue`); n != queueBefore {
		t.Fatalf("inserting a portal comment queued %d rows in github_outbound_queue", n-queueBefore)
	}
	e.eng.on("POST", pwcComments, answer(201, `{"id":777,"nodeId":"IC_1","body":"x","createdAt":"2026-11-19T10:00:00Z","updatedAt":"2026-11-19T10:00:01Z"}`))
	e.run()
	r := e.row(res.Command.ID)
	if r.status != "DELIVERED" || r.attempts != 1 {
		t.Fatalf("row = %+v", r)
	}
	if id := e.refGitHubID(res.Comment.ID); id == nil || *id != 777 {
		t.Errorf("ref id = %v", id)
	}
	if got := e.eng.bodyOf("POST", pwcComments); !strings.Contains(got, `**A** (CS) wrote:\n\nhello`) {
		t.Errorf("posted body = %s", got)
	}
	if n := e.count(`SELECT count(*) FROM github_outbound_queue`); n != queueBefore {
		t.Errorf("delivery queued outbound rows")
	}
	// A second run finds nothing due.
	e.run()
	if e.eng.count("POST", pwcComments) != 1 {
		t.Error("a delivered comment was posted again")
	}
	// The detail read shows delivery and the command.
	d, err := repository.NewProductIssueRepository(repository.NewScoped(e.pool)).GetDetail(e.ctx, e.wi1)
	if err != nil || len(d.Conversation) != 1 || d.Conversation[0].Delivery == nil || *d.Conversation[0].Delivery != "DELIVERED" ||
		d.Conversation[0].CommandID == nil || *d.Conversation[0].CommandID != res.Command.ID || len(d.Commands) != 1 || d.Commands[0].Status != "DELIVERED" {
		t.Fatalf("detail: %+v %+v %v", d.Conversation, d.Commands, err)
	}
}

func TestProductIssueCommands_AddCommentAlreadyAdoptedIsNotPosted(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("echoed first")
	if set, err := e.repo.SetPortalCommentGitHubID(e.ctx, res.Comment.ID, 555, "someone", nil); err != nil || !set {
		t.Fatalf("set: %v %v", set, err)
	}
	e.run()
	if r := e.row(res.Command.ID); r.status != "DELIVERED" || r.attempts != 0 {
		t.Errorf("row = %+v", r)
	}
	if e.eng.total() != 0 {
		t.Errorf("%d calls made for an adopted comment", e.eng.total())
	}
}

func TestProductIssueCommands_AmbiguousFailureThenLookupPreventsADuplicate(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("maybe posted")
	e.eng.on("POST", pwcComments, answer(500, "boom"))
	e.run()
	r := e.row(res.Command.ID)
	if r.status != "PENDING" || r.attempts != 1 || !r.next.After(time.Now().Add(20*time.Second)) || r.lastError == "" || strings.Contains(r.lastError, "boom") {
		t.Fatalf("after the 500: %+v", r)
	}
	// The comment did reach GitHub: the retry finds it and does not post again.
	e.eng.on("GET", pwcComments, answer(200, `[
		{"id":1,"body":"someone else's","updatedAt":"2026-11-19T10:00:00Z"},
		{"id":888,"body":"**A** (CS) wrote:\n\nmaybe posted","authorLogin":"portal-bot","updatedAt":"2026-11-19T10:00:05Z"}]`))
	e.makeDue(res.Command.ID)
	e.run()
	if r := e.row(res.Command.ID); r.status != "DELIVERED" || r.attempts != 2 {
		t.Fatalf("after the lookup: %+v", r)
	}
	if n := e.eng.count("POST", pwcComments); n != 1 {
		t.Errorf("posted %d times, want exactly the one failed attempt", n)
	}
	if id := e.refGitHubID(res.Comment.ID); id == nil || *id != 888 {
		t.Errorf("ref id = %v", id)
	}
	if q := e.eng.calls[len(e.eng.calls)-1].query; !strings.Contains(q, "since=") {
		t.Errorf("lookup query = %q", q)
	}

	// With no such comment on GitHub, the retry posts.
	res2 := e.comment("never arrived")
	e.eng.on("POST", pwcComments, answer(500, ""))
	e.run()
	e.eng.on("GET", pwcComments, answer(200, `[]`))
	e.eng.on("POST", pwcComments, answer(201, `{"id":999,"updatedAt":"2026-11-19T10:00:09Z"}`))
	e.makeDue(res2.Command.ID)
	e.run()
	if r := e.row(res2.Command.ID); r.status != "DELIVERED" {
		t.Errorf("second comment: %+v", r)
	}
	if id := e.refGitHubID(res2.Comment.ID); id == nil || *id != 999 {
		t.Errorf("second ref id = %v", id)
	}
}

func TestProductIssueCommands_PermanentAndRetryableFailures(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("perm")
	e.eng.on("POST", pwcComments, answer(502, `{"message":"secret github words"}`, "X-GitHub-Status", "404"))
	e.run()
	r := e.row(res.Command.ID)
	if r.status != "FAILED" || r.attempts != 1 || r.ghStatus == nil || *r.ghStatus != 404 || strings.Contains(r.lastError, "secret") || len(r.lastError) > 300 {
		t.Fatalf("permanent: %+v", r)
	}

	// 429 with Retry-After: rescheduled no sooner than GitHub said.
	res = e.comment("limited")
	e.eng.on("POST", pwcComments, answer(429, "", "Retry-After", "600"))
	e.run()
	r = e.row(res.Command.ID)
	if r.status != "PENDING" || r.attempts != 1 || r.next.Before(time.Now().Add(590*time.Second)) || r.ghStatus == nil || *r.ghStatus != 429 {
		t.Fatalf("rate limited: %+v", r)
	}

	// A 403 from GitHub with a rate limit is retried, one without is permanent.
	res = e.comment("403 limited")
	e.eng.on("POST", pwcComments, answer(502, "", "X-GitHub-Status", "403", "Retry-After", "60"))
	e.run()
	if r := e.row(res.Command.ID); r.status != "PENDING" {
		t.Errorf("403 with Retry-After: %+v", r)
	}
	res = e.comment("403 plain")
	e.eng.on("POST", pwcComments, answer(502, "", "X-GitHub-Status", "403"))
	e.run()
	if r := e.row(res.Command.ID); r.status != "FAILED" {
		t.Errorf("403 plain: %+v", r)
	}
	// The engineering entity's own 400.
	res = e.comment("eng 400")
	e.eng.on("POST", pwcComments, answer(400, ""))
	e.run()
	if r := e.row(res.Command.ID); r.status != "FAILED" {
		t.Errorf("eng 400: %+v", r)
	}
}

func TestProductIssueCommands_GivesUpAfter24Hours(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("stuck")
	e.exec(`UPDATE github_command SET window_started_on = NOW() - interval '25 hours' WHERE id = $1::uuid`, res.Command.ID)
	e.eng.on("POST", pwcComments, answer(503, ""))
	e.run()
	if r := e.row(res.Command.ID); r.status != "FAILED" || r.lastError != "gave up after 24 h" || r.attempts != 1 {
		t.Fatalf("row = %+v", r)
	}
	// Retry gives a fresh window: the next failure is only rescheduled.
	cmd, err := e.repo.Retry(e.ctx, e.wi1, res.Command.ID)
	if err != nil || cmd.Status != "PENDING" {
		t.Fatal(err)
	}
	e.run()
	if r := e.row(res.Command.ID); r.status != "PENDING" || r.attempts != 1 {
		t.Fatalf("after retry: %+v", r)
	}
}

func TestProductIssueCommands_BoardMoveCancelledWhenTheIssueLeftWOC(t *testing.T) {
	e := pwcSetup(t)
	c := e.enqueue(repository.CommandSetBoardStatus) // board_status is NULL
	e.run()
	if r := e.row(c.ID); r.status != "CANCELLED" || r.lastError != "issue left WOC before delivery" || e.eng.total() != 0 {
		t.Fatalf("row = %+v calls=%d", r, e.eng.total())
	}
	// A link that is no longer active.
	e.waitingOnCS()
	e.exec(`UPDATE github_issue_link SET link_state = 'DELETED' WHERE work_item_id = $1::uuid`, e.wi1)
	c = e.enqueue(repository.CommandSetBoardStatus)
	e.run()
	if r := e.row(c.ID); r.status != "CANCELLED" {
		t.Fatalf("inactive link: %+v", r)
	}
}

const pwcFieldsAnswer = `[{"id":5,"name":"Title","dataType":"title"},
	{"id":11,"nodeId":"F_1","name":"Status","dataType":"single_select","options":[{"id":"o1","name":"WOC"},{"id":"o2","name":" wow "}]}]`

func TestProductIssueCommands_BoardMoveFindsItemAndCachesTheOption(t *testing.T) {
	e := pwcSetup(t)
	e.waitingOnCS()
	e.exec(`UPDATE github_issue_link SET board_item_id = NULL WHERE work_item_id = $1::uuid`, e.wi1)
	e.eng.on("GET", pwcFields, answer(200, pwcFieldsAnswer))
	e.eng.on("GET", pwcItems, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after") == "" {
			_, _ = w.Write([]byte(`{"items":[{"itemId":1,"nodeId":"PVTI_a","content":{"nodeId":"I_other"}}],"nextCursor":"c2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"itemId":4242,"nodeId":"PVTI_b","content":{"nodeId":"` + pwhNode1 + `"}}],"nextCursor":null}`))
	})
	patch := "/orgs/example-org/projects/1/items/4242"
	e.eng.on("PATCH", patch, answer(200, `{}`))

	c := e.enqueue(repository.CommandSetBoardStatus)
	e.run()
	if r := e.row(c.ID); r.status != "DELIVERED" || r.attempts != 1 {
		t.Fatalf("row = %+v", r)
	}
	if got := e.eng.bodyOf("PATCH", patch); got != `{"fields":[{"id":11,"value":"o2"}]}` {
		t.Errorf("patch body = %s", got)
	}
	if got := e.count(`SELECT count(*) FROM github_issue_link WHERE work_item_id = $1::uuid AND board_item_id = '4242'`, e.wi1); got != 1 {
		t.Error("the item id was not stored on the link")
	}
	// The stored link state is untouched until GitHub confirms.
	if got := e.count(`SELECT count(*) FROM github_issue_link WHERE work_item_id = $1::uuid AND board_status = 'WAITING_ON_CS'`, e.wi1); got != 1 {
		t.Error("the stored board status changed")
	}

	// Second move: item id from the link, option from the cache.
	c2 := e.enqueue(repository.CommandSetBoardStatus)
	e.run()
	if r := e.row(c2.ID); r.status != "DELIVERED" {
		t.Fatalf("second: %+v", r)
	}
	if e.eng.count("GET", pwcItems) != 2 || e.eng.count("GET", pwcFields) != 1 {
		t.Errorf("items calls = %d (want 2: two pages, once), fields calls = %d (want 1)", e.eng.count("GET", pwcItems), e.eng.count("GET", pwcFields))
	}
}

func TestProductIssueCommands_BoardMoveRefreshesTheOptionOnAMissOnce(t *testing.T) {
	e := pwcSetup(t)
	e.waitingOnCS()
	e.exec(`UPDATE github_issue_link SET board_item_id = '77' WHERE work_item_id = $1::uuid`, e.wi1)
	n := 0
	e.eng.on("GET", pwcFields, func(w http.ResponseWriter, _ *http.Request) {
		n++
		if n == 1 { // a stale answer without WOW
			_, _ = w.Write([]byte(`[{"id":11,"name":"Status","dataType":"single_select","options":[{"id":"o1","name":"WOC"}]}]`))
			return
		}
		_, _ = w.Write([]byte(pwcFieldsAnswer))
	})
	e.eng.on("PATCH", "/orgs/example-org/projects/1/items/77", answer(200, `{}`))
	c := e.enqueue(repository.CommandSetBoardStatus)
	e.run()
	if r := e.row(c.ID); r.status != "DELIVERED" || e.eng.count("GET", pwcFields) != 2 {
		t.Fatalf("row = %+v fields calls = %d", r, e.eng.count("GET", pwcFields))
	}

	// A board that never has the option: refreshed once, then FAILED.
	e2 := pwcSetup(t)
	e2.waitingOnCS()
	e2.exec(`UPDATE github_issue_link SET board_item_id = '77' WHERE work_item_id = $1::uuid`, e2.wi1)
	e2.eng.on("GET", pwcFields, answer(200, `[{"id":11,"name":"Status","dataType":"single_select","options":[{"id":"o1","name":"WOC"}]}]`))
	c2 := e2.enqueue(repository.CommandSetBoardStatus)
	e2.run()
	if r := e2.row(c2.ID); r.status != "FAILED" || e2.eng.count("GET", pwcFields) != 2 || e2.eng.count("PATCH", "/orgs/example-org/projects/1/items/77") != 0 {
		t.Fatalf("no option: %+v fields calls = %d", r, e2.eng.count("GET", pwcFields))
	}
}

func TestProductIssueCommands_BoardMoveItemNotOnTheBoard(t *testing.T) {
	for name, items := range map[string]string{
		"absent":           `{"items":[{"itemId":1,"content":{"nodeId":"I_other"}}],"nextCursor":null}`,
		"no itemId field":  `{"items":[{"nodeId":"PVTI_b","content":{"nodeId":"` + pwhNode1 + `"}}],"nextCursor":null}`,
		"draft item first": `{"items":[{"itemId":3,"content":null}],"nextCursor":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := pwcSetup(t)
			e.waitingOnCS()
			e.eng.on("GET", pwcFields, answer(200, pwcFieldsAnswer))
			e.eng.on("GET", pwcItems, answer(200, items))
			c := e.enqueue(repository.CommandSetBoardStatus)
			e.run()
			if r := e.row(c.ID); r.status != "PENDING" || r.lastError != "not on the tracking board" || r.attempts != 1 {
				t.Fatalf("row = %+v", r)
			}
		})
	}
}

func TestProductIssueCommands_Close(t *testing.T) {
	e := pwcSetup(t)
	e.eng.on("PATCH", pwcIssue, answer(200, `{"id":1,"nodeId":"I","number":1,"state":"closed","stateReason":"completed"}`))
	c := e.enqueue(repository.CommandSetState)
	e.run()
	if r := e.row(c.ID); r.status != "DELIVERED" || r.attempts != 1 {
		t.Fatalf("close: %+v", r)
	}
	if got := e.eng.bodyOf("PATCH", pwcIssue); !strings.Contains(got, `"state":"closed"`) || !strings.Contains(got, `"stateReason":"completed"`) {
		t.Errorf("body = %s", got)
	}
	// The stored GitHub state does not change.
	if e.count(`SELECT count(*) FROM github_issue_link WHERE work_item_id = $1::uuid AND github_state = 'OPEN'`, e.wi1) != 1 {
		t.Error("the stored GitHub state changed")
	}

	// An answer showing it was already closed is delivered too (same answer shape).
	c = e.enqueue(repository.CommandSetState)
	e.eng.on("PATCH", pwcIssue, answer(200, `{"state":"closed","stateReason":"not_planned"}`))
	e.run()
	if r := e.row(c.ID); r.status != "DELIVERED" {
		t.Errorf("already closed: %+v", r)
	}

	// 422: permanent. Link inactive: cancelled.
	c = e.enqueue(repository.CommandSetState)
	e.eng.on("PATCH", pwcIssue, answer(422, "", "X-GitHub-Status", "422"))
	e.run()
	if r := e.row(c.ID); r.status != "FAILED" {
		t.Errorf("422: %+v", r)
	}
	e.exec(`UPDATE github_issue_link SET link_state = 'TRANSFERRED' WHERE work_item_id = $1::uuid`, e.wi1)
	c = e.enqueue(repository.CommandSetState)
	e.run()
	if r := e.row(c.ID); r.status != "CANCELLED" {
		t.Errorf("inactive: %+v", r)
	}
}

func TestProductIssueCommands_OptionNameNormalisation(t *testing.T) {
	for in, want := range map[string]string{"WOW": "wow", "  Wow ": "wow", "A &amp; B": "a & b"} {
		if got := normaliseOptionName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestProductIssueCommands_RetriedCommentLooksBeforePostingAgain(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("posted then retried")
	e.eng.on("POST", pwcComments, answer(500, ""))
	e.run() // a 500: it may have been posted
	e.exec(`UPDATE github_command SET status = 'FAILED' WHERE id = $1::uuid`, res.Command.ID)
	if _, err := e.repo.Retry(e.ctx, e.wi1, res.Command.ID); err != nil {
		t.Fatal(err)
	}
	if r := e.row(res.Command.ID); r.attempts != 0 {
		t.Fatalf("retry left attempts = %d", r.attempts)
	}
	e.eng.on("GET", pwcComments, answer(200, `[{"id":4711,"body":"**A** (CS) wrote:\n\nposted then retried","updatedAt":"2026-11-19T10:00:05Z"}]`))
	e.run()
	if r := e.row(res.Command.ID); r.status != "DELIVERED" {
		t.Fatalf("row = %+v", r)
	}
	if n := e.eng.count("POST", pwcComments); n != 1 {
		t.Errorf("posted %d times: a retried comment was duplicated", n)
	}
	if id := e.refGitHubID(res.Comment.ID); id == nil || *id != 4711 {
		t.Errorf("ref id = %v", id)
	}
}

func TestProductIssueCommands_LookupWindowIsWidenedForClockSkew(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("skew")
	e.eng.on("POST", pwcComments, answer(500, ""))
	e.run()
	e.eng.on("GET", pwcComments, answer(200, `[]`))
	e.eng.on("POST", pwcComments, answer(201, `{"id":1,"updatedAt":"2026-11-19T10:00:05Z"}`))
	e.makeDue(res.Command.ID)
	e.run()
	var since string
	for _, c := range e.eng.calls {
		if c.method == "GET" && c.path == pwcComments {
			since = c.query
		}
	}
	var created time.Time
	if err := e.pool.QueryRow(context.Background(), `SELECT created_on FROM github_command WHERE id = $1::uuid`, res.Command.ID).Scan(&created); err != nil {
		t.Fatal(err)
	}
	want := created.Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	if !strings.Contains(since, "since="+strings.ReplaceAll(strings.ReplaceAll(want, ":", "%3A"), "+", "%2B")) {
		t.Errorf("query %q does not carry since = created_on - 2 min (%s)", since, want)
	}
}

func TestProductIssueCommands_ExpiredClaimLeaseIsNotRun(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("late")
	claimed, err := e.repo.ClaimDue(e.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	var cmd repository.GitHubCommand
	for _, c := range claimed {
		if c.ID == res.Command.ID {
			cmd = c
		}
	}
	if cmd.ID == "" {
		t.Fatal("not claimed")
	}
	// The batch ran long: the claim's lease is gone (another replica may have it).
	e.exec(`UPDATE github_command SET next_attempt_on = NOW() - interval '1 second' WHERE id = $1::uuid`, cmd.ID)
	e.eng.on("POST", pwcComments, answer(201, `{"id":1}`))
	e.worker.process(e.ctx, cmd)
	if e.eng.total() != 0 {
		t.Errorf("%d calls made on an expired lease", e.eng.total())
	}
	if r := e.row(cmd.ID); r.status != "PENDING" || r.attempts != 0 {
		t.Errorf("row = %+v, want untouched", r)
	}
	// Claimed fresh, it runs, and the lease is renewed to a full five minutes by begin.
	e.makeDue(cmd.ID)
	e.run()
	if r := e.row(cmd.ID); r.status != "DELIVERED" {
		t.Errorf("fresh claim: %+v", r)
	}
}

func TestProductIssueCommands_OutcomeIsRecordedAfterTheRunTimedOut(t *testing.T) {
	e := pwcSetup(t)
	e.worker.callTimeout = 150 * time.Millisecond
	res := e.comment("slow")
	e.eng.on("POST", pwcComments, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(201)
	})
	e.run()
	r := e.row(res.Command.ID)
	if r.status != "PENDING" || r.attempts != 1 || r.lastError == "" || !r.next.After(time.Now().Add(20*time.Second)) {
		t.Fatalf("a timed-out call was not recorded with a backoff: %+v", r)
	}
	// And the 24 h give-up still applies to it.
	e.exec(`UPDATE github_command SET window_started_on = NOW() - interval '25 hours' WHERE id = $1::uuid`, res.Command.ID)
	e.makeDue(res.Command.ID)
	e.run()
	if r := e.row(res.Command.ID); r.status != "FAILED" || r.lastError != "gave up after 24 h" {
		t.Errorf("after the window: %+v", r)
	}
}

func TestProductIssueCommands_CancelledCommentShowsDelivery(t *testing.T) {
	e := pwcSetup(t)
	res := e.comment("never sent")
	e.exec(`UPDATE github_issue_link SET link_state = 'DELETED' WHERE work_item_id = $1::uuid`, e.wi1)
	e.run()
	if r := e.row(res.Command.ID); r.status != "CANCELLED" {
		t.Fatalf("row = %+v", r)
	}
	d, err := repository.NewProductIssueRepository(repository.NewScoped(e.pool)).GetDetail(e.ctx, e.wi1)
	if err != nil || len(d.Conversation) != 1 || d.Conversation[0].Delivery == nil || *d.Conversation[0].Delivery != "CANCELLED" {
		t.Fatalf("conversation = %+v %v", d.Conversation, err)
	}
	if len(d.Commands) != 1 || d.Commands[0].LastError == nil || *d.Commands[0].LastError == "" {
		t.Errorf("commands = %+v", d.Commands)
	}
}

func TestCleanDisplayName(t *testing.T) {
	for in, want := range map[string]string{
		"Jane  Doe":            "Jane Doe",
		"**Jane**\nDoe":        "Jane Doe",
		"@octocat":             "octocat",
		"[x](http://evil)":     "x(http://evil)",
		"<b>Mary_Ann</b> `rm`": "bMaryAnn/b rm",
	} {
		if got := cleanDisplayName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"@", "[", "]", "<", ">", "`", "_", "*"} {
		if got := cleanDisplayName("a" + bad + "b"); got != "ab" {
			t.Errorf("%q left %q", bad, got)
		}
	}
}
