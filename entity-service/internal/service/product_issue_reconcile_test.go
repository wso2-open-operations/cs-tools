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
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Unit tests of the reconciliation and backfill service over an in-memory
// repository and a scripted engineering entity. The database effects are tested
// in product_issue_reconcile_integration_test.go.

type fakeSyncClock struct{ t time.Time }

func (c *fakeSyncClock) now() time.Time { return c.t }

// fakeSyncEng scripts the engineering entity; every call advances the clock.
type fakeSyncEng struct {
	clock *fakeSyncClock
	step  time.Duration

	items    func(org string, board int, after string) (engineeringentity.ProjectItemsPage, error)
	detail   func(owner, repo string, n int) (engineeringentity.IssueDetail, error)
	list     func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error)
	comments func(n, page int) ([]engineeringentity.IssueComment, error)
	history  func(n int) ([]engineeringentity.ProjectStatusHistory, error)

	calls []string
}

func (f *fakeSyncEng) tick(what string) {
	f.calls = append(f.calls, what)
	if f.clock != nil {
		f.clock.t = f.clock.t.Add(f.step)
	}
}

func (f *fakeSyncEng) GetIssueDetail(_ context.Context, _, owner, repo string, n int) (engineeringentity.IssueDetail, error) {
	f.tick("detail")
	if f.detail == nil {
		return engineeringentity.IssueDetail{}, &engineeringentity.Error{StatusCode: 500}
	}
	return f.detail(owner, repo, n)
}
func (f *fakeSyncEng) ListIssueDetails(_ context.Context, _, _, _ string, q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
	f.tick("list")
	if f.list == nil {
		return nil, nil
	}
	return f.list(q)
}
func (f *fakeSyncEng) ListIssueComments(_ context.Context, _, _, _ string, n int, q engineeringentity.ListCommentsQuery) ([]engineeringentity.IssueComment, error) {
	f.tick("comments")
	if f.comments == nil {
		return nil, nil
	}
	return f.comments(n, q.Page)
}
func (f *fakeSyncEng) GetProjectStatusHistory(_ context.Context, _, _, _ string, n int) ([]engineeringentity.ProjectStatusHistory, error) {
	f.tick("history")
	if f.history == nil {
		return nil, nil
	}
	return f.history(n)
}
func (f *fakeSyncEng) ListProjectItemsWithContent(_ context.Context, org string, board int, after string, _ int) (engineeringentity.ProjectItemsPage, error) {
	f.tick("items")
	if f.items == nil {
		return engineeringentity.ProjectItemsPage{}, nil
	}
	return f.items(org, board, after)
}

// fakeSyncRepo is an in-memory repository.ProductIssueReconcileRepository.
type fakeSyncRepo struct {
	boards   []repository.ReconcileBoard
	byNode   map[string]*repository.LinkedIssue
	byIssue  map[string]*repository.LinkedIssue
	tracking bool
	notSeen  []repository.LinkedIssue

	mapped         bool
	repoBoards     []int
	caseByWSO2     map[string]string
	caseByNumber   map[string]string
	reconciled     []repository.ReconcileParams
	setStates      map[string]string
	created        []repository.BackfillCreateParams
	filled         []string
	createdAlready bool
	linked         map[string]bool // work item id -> has an active AFFECTS link
	attached       map[string]string
}

func newFakeSyncRepo() *fakeSyncRepo {
	return &fakeSyncRepo{byNode: map[string]*repository.LinkedIssue{}, byIssue: map[string]*repository.LinkedIssue{},
		tracking: true, mapped: true, caseByWSO2: map[string]string{}, caseByNumber: map[string]string{}, setStates: map[string]string{},
		linked: map[string]bool{}, attached: map[string]string{}}
}

func (r *fakeSyncRepo) LinkByIssue(_ context.Context, owner, repo string, n int) (*repository.LinkedIssue, error) {
	return r.byIssue[fmt.Sprintf("%s/%s#%d", strings.ToLower(owner), strings.ToLower(repo), n)], nil
}
func (r *fakeSyncRepo) LinkByNodeID(_ context.Context, id string) (*repository.LinkedIssue, error) {
	return r.byNode[id], nil
}
func (r *fakeSyncRepo) IsTrackingBoard(context.Context, string, string, int) (bool, error) {
	return r.tracking, nil
}
func (r *fakeSyncRepo) SetLinkState(_ context.Context, id, state string) error {
	r.setStates[id] = state
	return nil
}
func (r *fakeSyncRepo) DatabaseNow(context.Context) (time.Time, error) {
	return time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC), nil
}
func (r *fakeSyncRepo) ListBoards(context.Context) ([]repository.ReconcileBoard, error) {
	return r.boards, nil
}
func (r *fakeSyncRepo) ListLinksNotSeen(_ context.Context, _ string, _ int, _ time.Time, afterID string, limit int) ([]repository.LinkedIssue, error) {
	var out []repository.LinkedIssue
	for _, l := range r.notSeen {
		if l.WorkItemID > afterID && len(out) < limit {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *fakeSyncRepo) ReconcileLink(_ context.Context, p repository.ReconcileParams) (repository.ReconcileResult, error) {
	r.reconciled = append(r.reconciled, p)
	return repository.ReconcileResult{Changed: true}, nil
}
func (r *fakeSyncRepo) MappedRepository(_ context.Context, owner, repo string) (string, string, bool, error) {
	return owner, repo, r.mapped, nil
}
func (r *fakeSyncRepo) BoardsForRepository(context.Context, string, string) ([]int, error) {
	return r.repoBoards, nil
}
func (r *fakeSyncRepo) CaseIDByWSO2ID(_ context.Context, v string) (string, error) {
	return r.caseByWSO2[v], nil
}
func (r *fakeSyncRepo) CaseIDByNumber(_ context.Context, v string) (string, error) {
	return r.caseByNumber[v], nil
}
func (r *fakeSyncRepo) BackfillCreate(_ context.Context, p repository.BackfillCreateParams) (repository.BackfillCreateResult, error) {
	r.created = append(r.created, p)
	return repository.BackfillCreateResult{WorkItemID: "wi", AlreadyLinked: r.createdAlready, CommentsInserted: len(p.Comments)}, nil
}
func (r *fakeSyncRepo) HasActiveCaseLink(_ context.Context, id string) (bool, error) {
	return r.linked[id], nil
}
func (r *fakeSyncRepo) BackfillAttachCase(_ context.Context, id, caseID, _ string) (bool, error) {
	r.attached[id] = caseID
	return true, nil
}
func (r *fakeSyncRepo) BackfillFill(_ context.Context, id string, _ []repository.GitHubCommentInput, _ *repository.BoardSync) (repository.BackfillFillResult, error) {
	r.filled = append(r.filled, id)
	return repository.BackfillFillResult{}, nil
}

func (r *fakeSyncRepo) addLink(l repository.LinkedIssue, node string) *repository.LinkedIssue {
	if l.LinkState == "" {
		l.LinkState = repository.LinkStateActive
	}
	p := &l
	r.byNode[node] = p
	r.byIssue[fmt.Sprintf("%s/%s#%d", strings.ToLower(l.Owner), strings.ToLower(l.Repository), l.IssueNumber)] = p
	return p
}

func newTestSync(eng *fakeSyncEng, repo *fakeSyncRepo, clock *fakeSyncClock) *productIssueSync {
	return &productIssueSync{eng: eng, repo: repo, now: clock.now, reserve: 15 * time.Second, defaultBudget: 30 * time.Second}
}

func boardItem(node string, n int, status string) engineeringentity.ProjectItem {
	it := engineeringentity.ProjectItem{NodeID: "PVTI_" + node, Type: "ISSUE", Content: &engineeringentity.ProjectItemContent{
		NodeID: node, Repository: "example-org/example-repo", Number: n, Title: "t" + node, State: "OPEN",
		Labels: []string{"Origin/CS"}, UpdatedAt: "2026-11-10T10:00:00Z"}}
	if status != "" {
		it.FieldValues = []engineeringentity.ProjectItemFieldValue{{Name: "Status", DataType: "SINGLE_SELECT", Value: []byte(`"` + status + `"`)}}
	}
	return it
}

func linkFor(id string, n int) repository.LinkedIssue {
	return repository.LinkedIssue{WorkItemID: id, Owner: "example-org", Repository: "example-repo", IssueNumber: n, GitHubState: "OPEN"}
}

func rateLimit(secs int) error {
	return &engineeringentity.Error{StatusCode: http.StatusTooManyRequests, RetryAfter: time.Duration(secs) * time.Second}
}

func TestSyncCursor_RoundTripAndInvalid(t *testing.T) {
	in := reconcileCursor{Owner: "example-org", Board: 7, Phase: phaseItems, After: "abc", Offset: 3, Started: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	var out reconcileCursor
	if err := decodeSyncCursor(encodeSyncCursor(in), &out); err != nil || out != in {
		t.Fatalf("round trip: %+v %v", out, err)
	}
	var ve *apierror.ValidationError
	for _, bad := range []string{"%%%", "bm90IGpzb24"} {
		if err := decodeSyncCursor(bad, &out); !errors.As(err, &ve) {
			t.Errorf("cursor %q: %v", bad, err)
		}
	}
	svc := newTestSync(&fakeSyncEng{}, newFakeSyncRepo(), &fakeSyncClock{t: time.Now()})
	if _, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Cursor: "garbage!"}); !errors.As(err, &ve) {
		t.Errorf("reconcile with a bad cursor: %v", err)
	}
	if _, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Limit: -1}); !errors.As(err, &ve) {
		t.Errorf("reconcile with a negative limit: %v", err)
	}
	// A well-formed JSON cursor with no start time is not one this service made.
	if _, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Cursor: encodeSyncCursor(map[string]any{"o": "x"})}); !errors.As(err, &ve) {
		t.Errorf("reconcile with a cursor without a start: %v", err)
	}
}

func TestDecideCaseMatch(t *testing.T) {
	cases := []struct {
		name, byID, byNumber, want string
	}{
		{"both lines name the same case", "c1", "c1", "c1"},
		{"the lines name different cases", "c1", "c2", ""},
		{"only the id line resolves", "c1", "", ""},
		{"only the number line resolves", "", "c2", ""},
		{"neither resolves", "", "", ""},
	}
	for _, tc := range cases {
		if got := decideCaseMatch(tc.byID, tc.byNumber); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBoardSyncFromHistory(t *testing.T) {
	s := func(v string) *string { return &v }
	hist := []engineeringentity.ProjectStatusHistory{
		{ProjectNumber: 9, CurrentStatus: s("Open")}, // another board
		{ProjectNumber: 5, IsArchived: true, CurrentStatus: s("Resolved")},
		{ProjectNumber: 6, CurrentStatus: s("WOC"), Changes: []engineeringentity.ProjectStatusChange{
			{EventNodeID: "e2", PreviousStatus: s("Open"), NewStatus: s("WOC"), OccurredAt: "2026-11-02T10:00:00Z", ActorLogin: s("octo")},
			{EventNodeID: "e1", PreviousStatus: nil, NewStatus: s("Open"), OccurredAt: "2026-11-01T10:00:00Z"},
			{EventNodeID: "e3", NewStatus: nil, OccurredAt: "2026-11-03T10:00:00Z"}, // a cleared Status is not a change to a column
			{EventNodeID: "e4", NewStatus: s("WOW"), OccurredAt: "not a time"},      // unreadable time
		}},
	}
	got := boardSyncFromHistory(hist, []int{5, 6}, "BACKFILL")
	if got == nil || got.ProjectNumber != 6 || got.CurrentRaw != "WOC" || got.Source != "BACKFILL" || len(got.Changes) != 2 {
		t.Fatalf("got %+v: an unarchived board wins over an archived one", got)
	}
	byKey := map[string]repository.BoardHistoryChange{}
	for _, c := range got.Changes {
		byKey[c.EventKey] = c
	}
	if c := byKey["e2"]; c.FromRaw != "Open" || c.ToRaw != "WOC" || c.Actor != "octo" {
		t.Errorf("e2 = %+v", c)
	}
	if c := byKey["e1"]; c.FromRaw != "" || c.ToRaw != "Open" {
		t.Errorf("e1 = %+v", c)
	}
	if got := boardSyncFromHistory(hist, []int{5}, "x"); got == nil || got.ProjectNumber != 5 || got.CurrentRaw != "Resolved" {
		t.Errorf("an archived board is still read: %+v", got)
	}
	if boardSyncFromHistory(hist, []int{42}, "x") != nil {
		t.Error("an issue on none of the boards has no history")
	}
}

func TestReconcile_RateLimitStopsWithTheCursorAndResumes(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}, {Owner: "example-org", ProjectNumber: 2}}
	limited := true
	eng := &fakeSyncEng{clock: clock}
	eng.items = func(_ string, board int, _ string) (engineeringentity.ProjectItemsPage, error) {
		if board == 2 && limited {
			return engineeringentity.ProjectItemsPage{}, rateLimit(90)
		}
		return engineeringentity.ProjectItemsPage{}, nil
	}
	svc := newTestSync(eng, repo, clock)
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil || !resp.More || resp.RetryAfterSeconds != 90 || resp.NextCursor == "" || resp.Boards != 2 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	var cur reconcileCursor
	if err := decodeSyncCursor(resp.NextCursor, &cur); err != nil || cur.Board != 2 || cur.Phase != phaseItems {
		t.Fatalf("cursor = %+v: it must point at the board that was rate limited", cur)
	}
	limited = false
	resp, err = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Cursor: resp.NextCursor})
	if err != nil || resp.More || resp.NextCursor != "" || resp.RetryAfterSeconds != 0 || resp.Boards != 1 {
		t.Fatalf("resumed resp = %+v err=%v", resp, err)
	}
	// Without a Retry-After the caller is still told to wait.
	eng.items = func(string, int, string) (engineeringentity.ProjectItemsPage, error) {
		return engineeringentity.ProjectItemsPage{}, &engineeringentity.Error{StatusCode: 429}
	}
	if resp, _ = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{}); resp.RetryAfterSeconds != int(defaultRetryAfter.Seconds()) {
		t.Errorf("default wait = %d", resp.RetryAfterSeconds)
	}
}

func TestReconcile_TimeBudgetStopsAfterAtLeastOneUnitAndResumes(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	for i := 1; i <= 3; i++ {
		repo.addLink(linkFor(fmt.Sprintf("wi%d", i), i), fmt.Sprintf("I_%d", i))
	}
	next := "page2"
	eng := &fakeSyncEng{clock: clock, step: 10 * time.Second}
	eng.items = func(_ string, _ int, after string) (engineeringentity.ProjectItemsPage, error) {
		if after == "" {
			return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{boardItem("I_1", 1, "WOC"), boardItem("I_2", 2, "WOC"), boardItem("I_3", 3, "WOC")}, NextCursor: &next}, nil
		}
		return engineeringentity.ProjectItemsPage{}, nil
	}
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) { return nil, nil }
	svc := newTestSync(eng, repo, clock)

	// 30 s budget, 15 s reserve, 10 s per engineering call: the page (10 s) and
	// the first item's history (10 s) fit; the second item must wait.
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil || !resp.More || resp.ItemsChecked != 1 {
		t.Fatalf("first call = %+v err=%v", resp, err)
	}
	var cur reconcileCursor
	if err := decodeSyncCursor(resp.NextCursor, &cur); err != nil || cur.Offset != 1 || cur.After != "" || cur.Phase != phaseItems {
		t.Fatalf("cursor = %+v", cur)
	}
	total := resp.ItemsChecked
	for calls := 0; resp.More; calls++ {
		if calls > 10 {
			t.Fatal("the run does not end")
		}
		if resp, err = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Cursor: resp.NextCursor}); err != nil {
			t.Fatal(err)
		}
		if resp.ItemsChecked == 0 && resp.More {
			t.Fatalf("a call made no progress: %+v", resp)
		}
		total += resp.ItemsChecked
	}
	if total != 3 || len(repo.reconciled) != 3 {
		t.Errorf("items checked = %d, reconciled = %d, want 3 and 3", total, len(repo.reconciled))
	}
}

func TestReconcile_LimitBoundsAUnitCountAcrossBoards(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	repo.addLink(linkFor("wi1", 1), "I_1")
	repo.addLink(linkFor("wi2", 2), "I_2")
	eng := &fakeSyncEng{clock: clock}
	eng.items = func(string, int, string) (engineeringentity.ProjectItemsPage, error) {
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{boardItem("I_1", 1, ""), boardItem("I_2", 2, "")}}, nil
	}
	svc := newTestSync(eng, repo, clock)
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Limit: 1})
	if err != nil || !resp.More || resp.ItemsChecked != 1 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	resp, err = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Limit: 1, Cursor: resp.NextCursor})
	if err != nil || resp.ItemsChecked != 1 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
}

func TestReconcile_ItemRules(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	repo.addLink(linkFor("wi-active", 1), "I_active")
	repo.addLink(repository.LinkedIssue{WorkItemID: "wi-gone", Owner: "example-org", Repository: "example-repo", IssueNumber: 2, LinkState: repository.LinkStateDeleted}, "I_gone")
	other := linkFor("wi-other-org", 3)
	other.Owner = "another-org"
	repo.addLink(other, "I_other")
	onOtherBoard := linkFor("wi-other-board", 4)
	nine := 9
	raw := "In Progress"
	onOtherBoard.BoardProjectNumber, onOtherBoard.BoardStatusRaw = &nine, &raw
	repo.addLink(onOtherBoard, "I_ob")
	pr := boardItem("I_pr", 5, "Open")
	pr.Content.IsPullRequest = true
	draft := engineeringentity.ProjectItem{NodeID: "PVTI_d", Type: "DRAFT_ISSUE"}
	eng := &fakeSyncEng{clock: clock}
	eng.items = func(string, int, string) (engineeringentity.ProjectItemsPage, error) {
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{
			boardItem("I_active", 1, "Open"), boardItem("I_gone", 2, "Open"), boardItem("I_other", 3, "Open"),
			boardItem("I_ob", 4, "Open"), boardItem("I_unlinked", 6, "Open"), pr, draft}}, nil
	}
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) { return nil, nil }
	svc := newTestSync(eng, repo, clock)
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil || resp.ItemsChecked != 2 || len(repo.reconciled) != 2 {
		t.Fatalf("resp = %+v reconciled=%d err=%v: only an active link of the board's organisation counts", resp, len(repo.reconciled), err)
	}
	var ob repository.ReconcileParams
	for _, p := range repo.reconciled {
		if p.WorkItemID == "wi-other-board" {
			ob = p
		}
	}
	if ob.Board != nil || ob.Issue == nil {
		t.Errorf("an issue on another board of a shared repository keeps that board's status: %+v", ob)
	}
	if n := countCalls(eng.calls, "history"); n != 1 {
		t.Errorf("history reads = %d, want 1 (only the link whose status differs and is on this board)", n)
	}
	// An unchanged status needs no history read.
	repo.reconciled, eng.calls = nil, nil
	open := "Open"
	one := 1
	repo.byNode["I_active"].BoardStatusRaw, repo.byNode["I_active"].BoardProjectNumber = &open, &one
	if _, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{}); err != nil {
		t.Fatal(err)
	}
	if n := countCalls(eng.calls, "history"); n != 0 {
		t.Errorf("history reads = %d for an unchanged status", n)
	}
	for _, p := range repo.reconciled {
		if p.WorkItemID == "wi-active" && p.Board != nil {
			t.Error("an unchanged status must not be sent as a board change")
		}
	}
}

func TestReconcile_HistoryFailureAndRateLimit(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	repo.addLink(linkFor("wi1", 1), "I_1")
	eng := &fakeSyncEng{clock: clock}
	eng.items = func(string, int, string) (engineeringentity.ProjectItemsPage, error) {
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{boardItem("I_1", 1, "WOC")}}, nil
	}
	svc := newTestSync(eng, repo, clock)

	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) {
		return nil, &engineeringentity.Error{StatusCode: 500}
	}
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil || resp.Failed != 1 || resp.More || len(repo.reconciled) != 1 || repo.reconciled[0].Board != nil {
		t.Fatalf("a failed history read counts and still refreshes the issue fields: %+v err=%v params=%+v", resp, err, repo.reconciled)
	}

	repo.reconciled = nil
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) { return nil, rateLimit(30) }
	resp, err = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil || !resp.More || resp.RetryAfterSeconds != 30 || len(repo.reconciled) != 0 {
		t.Fatalf("a 429 on the history stops at once: %+v err=%v", resp, err)
	}
	var cur reconcileCursor
	if err := decodeSyncCursor(resp.NextCursor, &cur); err != nil || cur.Offset != 0 {
		t.Fatalf("cursor = %+v: the rate limited item must be retried", cur)
	}
}

func TestReconcile_NotOnBoardRefreshesInaccessibleAndRateLimited(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	repo.notSeen = []repository.LinkedIssue{linkFor("wi-a", 1), linkFor("wi-b", 2), linkFor("wi-c", 3), linkFor("wi-d", 4)}
	eng := &fakeSyncEng{clock: clock}
	limited := true
	eng.detail = func(_, _ string, n int) (engineeringentity.IssueDetail, error) {
		switch n {
		case 2:
			return engineeringentity.IssueDetail{}, &engineeringentity.Error{StatusCode: http.StatusNotFound}
		case 3:
			return engineeringentity.IssueDetail{}, &engineeringentity.Error{StatusCode: 500}
		case 4:
			if limited {
				return engineeringentity.IssueDetail{}, rateLimit(45)
			}
		}
		closed := "2026-11-05T10:00:00Z"
		reason := "COMPLETED"
		return engineeringentity.IssueDetail{Number: n, Title: "T", State: "closed", StateReason: &reason, ClosedAt: &closed,
			UpdatedAt: "2026-11-05T10:00:00Z", Labels: []string{"Origin/CS", "Priority/High(P2)"}}, nil
	}
	svc := newTestSync(eng, repo, clock)
	resp, err := svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// a refreshed, b 404 -> INACCESSIBLE, c failed, d 429 -> stop.
	if resp.NotOnBoard != 2 || resp.Failed != 1 || !resp.More || resp.RetryAfterSeconds != 45 || resp.LinksUpdated != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	if repo.setStates["wi-b"] != repository.LinkStateInaccessible || len(repo.setStates) != 1 {
		t.Errorf("states = %v", repo.setStates)
	}
	if len(repo.reconciled) != 1 {
		t.Fatalf("reconciled = %+v", repo.reconciled)
	}
	p := repo.reconciled[0]
	if p.WorkItemID != "wi-a" || p.Board != nil || p.Issue == nil || p.Issue.State != "CLOSED" || p.Issue.StateReason != "completed" ||
		p.Issue.Priority != "P2" || p.Issue.ClosedOn == nil {
		t.Errorf("refresh = %+v issue=%+v", p, p.Issue)
	}
	var cur reconcileCursor
	if err := decodeSyncCursor(resp.NextCursor, &cur); err != nil || cur.Phase != phaseMissing || cur.LastID != "wi-c" {
		t.Fatalf("cursor = %+v: it must resume at the rate limited link", cur)
	}
	limited = false
	repo.reconciled = nil
	resp, err = svc.Reconcile(context.Background(), domain.ProductIssueReconcileRequest{Cursor: resp.NextCursor})
	if err != nil || resp.More || resp.NotOnBoard != 1 || len(repo.reconciled) != 1 || repo.reconciled[0].WorkItemID != "wi-d" {
		t.Fatalf("resumed = %+v err=%v reconciled=%+v", resp, err, repo.reconciled)
	}
}

func countCalls(calls []string, what string) int {
	n := 0
	for _, c := range calls {
		if c == what {
			n++
		}
	}
	return n
}

// --- backfill ---

func issue(n int, labels ...string) engineeringentity.IssueDetail {
	body := ""
	return engineeringentity.IssueDetail{NodeID: fmt.Sprintf("I_%d", n), Number: n, Title: fmt.Sprintf("Issue %d", n), Body: &body, State: "open",
		Labels: append([]string{"Origin/CS"}, labels...), HTMLURL: fmt.Sprintf("https://github.com/example-org/example-repo/issues/%d", n),
		CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z"}
}

func withBody(i engineeringentity.IssueDetail, body string) engineeringentity.IssueDetail {
	i.Body = &body
	return i
}

func backfillFixture() (*fakeSyncEng, *fakeSyncRepo, *productIssueSync) {
	clock := &fakeSyncClock{t: time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)}
	repo := newFakeSyncRepo()
	repo.repoBoards = []int{7}
	repo.caseByWSO2["W1"], repo.caseByNumber["CS1"] = "case-1", "case-1"
	repo.caseByWSO2["W2"], repo.caseByNumber["CS2"] = "case-2", "case-2"
	eng := &fakeSyncEng{clock: clock}
	return eng, repo, newTestSync(eng, repo, clock)
}

func TestBackfill_Validation(t *testing.T) {
	_, repo, svc := backfillFixture()
	var ve *apierror.ValidationError
	for name, req := range map[string]domain.ProductIssueBackfillRequest{
		"no repository":     {Owner: "example-org"},
		"negative max":      {Owner: "example-org", Repository: "example-repo", MaxIssues: -1},
		"negative since":    {Owner: "example-org", Repository: "example-repo", SinceDays: -3},
		"a broken cursor":   {Owner: "example-org", Repository: "example-repo", Cursor: "zzz"},
		"a cursor of other": {Owner: "example-org", Repository: "example-repo", Cursor: encodeSyncCursor(map[string]any{"p": "sideways", "g": 1})},
	} {
		if _, err := svc.Backfill(context.Background(), req); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	repo.mapped = false
	if _, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "unknown-repo"}); !errors.As(err, &ve) || ve.Msg != msgLinkRepoNotMapped {
		t.Errorf("an unmapped repository: %v", err)
	}
}

func TestBackfill_DryRunCountsAndWritesNothing(t *testing.T) {
	eng, repo, svc := backfillFixture()
	repo.addLink(linkFor("wi-known", 10), "I_10")
	closedAt := "2026-11-10T10:00:00Z"
	old := "2026-01-01T10:00:00Z"
	closedRecent, closedOld := issue(8), issue(9)
	closedRecent.State, closedRecent.ClosedAt = "closed", &closedAt
	closedOld.State, closedOld.ClosedAt, closedOld.UpdatedAt = "closed", &old, old
	pr := issue(7)
	pr.HTMLURL = "https://github.com/example-org/example-repo/pull/7"
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.Page > 1 {
			return nil, nil
		}
		if q.State == "open" {
			return []engineeringentity.IssueDetail{
				withBody(issue(1), "WSO2 case id : W1\nCase number : CS1"), // both agree: matched
				withBody(issue(2), "WSO2 case id : W1\nCase number : CS2"), // disagree: unmatched
				withBody(issue(3), "Case number : CS2"),                    // one line: unmatched
				issue(4),                                                   // none: unmatched
				issue(5, "Type/Patch"), issue(6, "patch"), pr, issue(10),
			}, nil
		}
		return []engineeringentity.IssueDetail{withBody(closedRecent, "WSO2 case id : W2"), closedOld}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo", DryRun: true, MaxIssues: 100})
	if err != nil {
		t.Fatal(err)
	}
	want := domain.ProductIssueBackfillResponse{DryRun: true, Considered: 9, WouldCreateMatched: 1, WouldCreateUnmatched: 4, AlreadyKnown: 1,
		SkippedPatch: 2, SkippedPullRequest: 1, FailedIssueNumbers: []int{}}
	if fmt.Sprintf("%+v", resp) != fmt.Sprintf("%+v", want) {
		t.Fatalf("resp  = %+v\nwant = %+v", resp, want)
	}
	if len(repo.created) != 0 || len(repo.filled) != 0 {
		t.Errorf("a dry run wrote: created=%d filled=%d", len(repo.created), len(repo.filled))
	}
	for _, c := range eng.calls {
		if c != "list" {
			t.Errorf("a dry run called %q: only the listing is needed", c)
		}
	}
}

func TestBackfill_CreatesMatchedAndUnmatchedFromTheFooter(t *testing.T) {
	eng, repo, svc := backfillFixture()
	body := "Text\n\nProduct : x\nWSO2 case id : W1\nCase number : CS1\nPublic Git Issue : undefined\nABT Team : Team A\nOpened by : eng@example.test"
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{withBody(issue(1, "Priority/High(P2)", "Type/Query"), body), issue(2)}, nil
	}
	eng.comments = func(n, _ int) ([]engineeringentity.IssueComment, error) {
		if n != 1 {
			return nil, nil
		}
		login := "octo"
		return []engineeringentity.IssueComment{{ID: 11, Body: "hello", AuthorLogin: &login, CreatedAt: "2026-11-01T11:00:00Z", UpdatedAt: "2026-11-01T11:00:00Z"}}, nil
	}
	s := func(v string) *string { return &v }
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) {
		return []engineeringentity.ProjectStatusHistory{{ProjectNumber: 7, CurrentStatus: s("Open"), Changes: []engineeringentity.ProjectStatusChange{
			{EventNodeID: "e1", NewStatus: s("Open"), OccurredAt: "2026-11-01T10:00:01Z"}}}}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || resp.CreatedMatched != 1 || resp.CreatedUnmatched != 1 || resp.Considered != 2 || resp.CommentsWritten != 1 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	if len(repo.created) != 2 {
		t.Fatalf("created = %d", len(repo.created))
	}
	m, u := repo.created[0], repo.created[1]
	if m.CaseID != "case-1" || m.OpenedByEmail != "eng@example.test" || m.ABTTeam != "Team A" || m.PublicIssueURL != "" || m.Priority != "P2" ||
		m.Category != "Type/Query" || m.State != "OPEN" || len(m.Comments) != 1 || m.Comments[0].Author != "octo" || m.Comments[0].GitHubCommentID != 11 ||
		m.Board == nil || m.Board.ProjectNumber != 7 || len(m.Board.Changes) != 1 || m.Actor != backfillActor {
		t.Errorf("matched = %+v", m)
	}
	if m.Unmatched || !u.Unmatched || u.CaseID != "" || len(u.Comments) != 0 {
		t.Errorf("unmatched = %+v", u)
	}
}

func TestBackfill_RateLimitStopsBeforeAnythingIsCreatedForThatIssue(t *testing.T) {
	eng, repo, svc := backfillFixture()
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{issue(1), issue(2), issue(3)}, nil
	}
	eng.comments = func(n, _ int) ([]engineeringentity.IssueComment, error) {
		if n == 2 {
			return nil, rateLimit(120)
		}
		return nil, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || !resp.More || resp.RetryAfterSeconds != 120 || resp.CreatedUnmatched != 1 || len(repo.created) != 1 {
		t.Fatalf("resp = %+v err=%v created=%d", resp, err, len(repo.created))
	}
	var cur backfillCursor
	if err := decodeSyncCursor(resp.NextCursor, &cur); err != nil || cur.Phase != phaseOpen || cur.Page != 1 || cur.Offset != 1 {
		t.Fatalf("cursor = %+v: it must resume at the second issue", cur)
	}
	eng.comments = nil
	resp, err = svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo", Cursor: resp.NextCursor})
	if err != nil || resp.More || resp.CreatedUnmatched != 2 || len(repo.created) != 3 {
		t.Fatalf("resumed = %+v err=%v", resp, err)
	}
}

func TestBackfill_MaxIssuesPagesThroughOpenThenClosed(t *testing.T) {
	eng, repo, svc := backfillFixture()
	closedAt := "2026-11-10T10:00:00Z"
	var sinceSeen string
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State == "closed" {
			sinceSeen = q.Since
			if q.Page > 1 {
				return nil, nil
			}
			c := issue(30)
			c.State, c.ClosedAt = "closed", &closedAt
			return []engineeringentity.IssueDetail{c}, nil
		}
		// Pull requests are left out of a listing, so pages are short: only an empty page ends the phase.
		switch q.Page {
		case 1:
			return []engineeringentity.IssueDetail{issue(1), issue(2), issue(3)}, nil
		case 2:
			return []engineeringentity.IssueDetail{issue(4)}, nil
		}
		return nil, nil
	}
	req := domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo", MaxIssues: 2, SinceDays: 30}
	calls, created := 0, 0
	for {
		resp, err := svc.Backfill(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		calls++
		created += resp.CreatedUnmatched
		if resp.Considered > 2 {
			t.Fatalf("a call considered %d issues, want at most 2", resp.Considered)
		}
		if !resp.More {
			break
		}
		if calls > 10 {
			t.Fatal("the run does not end")
		}
		req.Cursor = resp.NextCursor
	}
	if created != 5 || len(repo.created) != 5 || calls != 3 {
		t.Errorf("created=%d (rows %d) in %d calls, want 5 in 3", created, len(repo.created), calls)
	}
	if want := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC).Format(time.RFC3339); sinceSeen != want {
		t.Errorf("closed issues were asked for since %q, want %q", sinceSeen, want)
	}
	nums := []int{}
	for _, c := range repo.created {
		nums = append(nums, c.Issue.Number)
	}
	sort.Ints(nums)
	if fmt.Sprint(nums) != "[1 2 3 4 30]" {
		t.Errorf("issues created = %v", nums)
	}
}

func TestBackfill_KnownIssueOnlyFillsAndSkipsInactiveLinks(t *testing.T) {
	eng, repo, svc := backfillFixture()
	repo.addLink(linkFor("wi-active", 1), "I_1")
	repo.addLink(repository.LinkedIssue{WorkItemID: "wi-deleted", Owner: "example-org", Repository: "example-repo", IssueNumber: 2, LinkState: repository.LinkStateDeleted}, "I_2")
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{issue(1), issue(2)}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || resp.AlreadyKnown != 2 || resp.CreatedMatched+resp.CreatedUnmatched != 0 || len(repo.created) != 0 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	if len(repo.filled) != 1 || repo.filled[0] != "wi-active" {
		t.Errorf("filled = %v: only an active link is filled", repo.filled)
	}
}

func TestBackfill_FailedReadsAreCountedAndTheIssueIsNotCreated(t *testing.T) {
	eng, repo, svc := backfillFixture()
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{issue(1), issue(2)}, nil
	}
	eng.history = func(n int) ([]engineeringentity.ProjectStatusHistory, error) {
		if n == 1 {
			return nil, &engineeringentity.Error{StatusCode: 500}
		}
		return nil, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || resp.Failed != 1 || len(resp.FailedIssueNumbers) != 1 || resp.FailedIssueNumbers[0] != 1 || resp.CreatedUnmatched != 1 || len(repo.created) != 1 || repo.created[0].Issue.Number != 2 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	// A listing that fails is the call's failure.
	eng.list = func(engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		return nil, &engineeringentity.Error{StatusCode: 500}
	}
	if _, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"}); err == nil {
		t.Error("a failed listing must fail the call")
	}
}

func TestBackfill_TimeBudgetStopsAfterOneIssue(t *testing.T) {
	eng, repo, svc := backfillFixture()
	eng.step = 20 * time.Second
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{issue(1), issue(2)}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || !resp.More || resp.CreatedUnmatched != 1 || len(repo.created) != 1 || resp.RetryAfterSeconds != 0 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
}

func TestBackfill_SkipsCommentsOfTheConfiguredBotsButNotProjectAutomation(t *testing.T) {
	eng, repo, svc := backfillFixture()
	svc.bots = botLoginSet([]string{"Portal-Bot", "github-project-automation[bot]"})
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{issue(1)}, nil
	}
	eng.comments = func(int, int) ([]engineeringentity.IssueComment, error) {
		l := func(v string) *string { return &v }
		return []engineeringentity.IssueComment{
			{ID: 1, Body: "from the portal", AuthorLogin: l("portal-bot")},
			{ID: 2, Body: "from the board automation", AuthorLogin: l("github-project-automation[bot]")},
			{ID: 3, Body: "from a person", AuthorLogin: l("octo")},
			{ID: 4, Body: "no author"},
		}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || len(repo.created) != 1 {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	var ids []int64
	for _, c := range repo.created[0].Comments {
		ids = append(ids, c.GitHubCommentID)
	}
	if fmt.Sprint(ids) != "[2 3 4]" {
		t.Errorf("comments kept = %v, want [2 3 4]: the bot's is skipped, project automation's is not", ids)
	}
}

func TestBackfill_KnownUnmatchedIssueGetsItsCaseWhenTheFooterNowMatches(t *testing.T) {
	eng, repo, svc := backfillFixture()
	repo.addLink(linkFor("wi-a", 1), "I_1") // unmatched, footer matches
	repo.addLink(linkFor("wi-b", 2), "I_2") // unmatched, one footer line only
	repo.addLink(linkFor("wi-c", 3), "I_3") // already linked to a case
	repo.linked["wi-c"] = true
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.State != "open" || q.Page > 1 {
			return nil, nil
		}
		return []engineeringentity.IssueDetail{
			withBody(issue(1), "WSO2 case id : W1\nCase number : CS1"),
			withBody(issue(2), "WSO2 case id : W1"),
			withBody(issue(3), "WSO2 case id : W2\nCase number : CS2"),
		}, nil
	}
	resp, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo"})
	if err != nil || resp.AlreadyKnown != 3 || len(repo.filled) != 3 {
		t.Fatalf("resp = %+v err=%v filled=%v", resp, err, repo.filled)
	}
	if len(repo.attached) != 1 || repo.attached["wi-a"] != "case-1" {
		t.Errorf("attached = %v, want only wi-a -> case-1", repo.attached)
	}
	// A dry run attaches nothing.
	repo.attached = map[string]string{}
	if _, err := svc.Backfill(context.Background(), domain.ProductIssueBackfillRequest{Owner: "example-org", Repository: "example-repo", DryRun: true}); err != nil || len(repo.attached) != 0 {
		t.Errorf("dry run attached %v err=%v", repo.attached, err)
	}
}

func TestReconcile_ShortDeadlineKeepsBackOnlyAQuarter(t *testing.T) {
	clock := &fakeSyncClock{t: time.Now()}
	repo := newFakeSyncRepo()
	repo.boards = []repository.ReconcileBoard{{Owner: "example-org", ProjectNumber: 1}}
	var items []engineeringentity.ProjectItem
	for i := 1; i <= 8; i++ {
		repo.addLink(linkFor(fmt.Sprintf("wi%d", i), i), fmt.Sprintf("I_%d", i))
		items = append(items, boardItem(fmt.Sprintf("I_%d", i), i, "WOC"))
	}
	eng := &fakeSyncEng{clock: clock, step: time.Second}
	eng.items = func(string, int, string) (engineeringentity.ProjectItemsPage, error) {
		return engineeringentity.ProjectItemsPage{Items: items}, nil
	}
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) { return nil, nil }
	svc := newTestSync(eng, repo, clock)
	// A 20 s deadline: the reserve is 5 s (a quarter), not 15 s, so the call keeps
	// working until 5 s are left. With the old fixed reserve it would stop at once.
	ctx, cancel := context.WithDeadline(context.Background(), clock.t.Add(20*time.Second))
	defer cancel()
	b := svc.newBudget(ctx)
	if b.reserve != 5*time.Second {
		t.Fatalf("reserve = %v, want 5s", b.reserve)
	}
	resp, err := svc.Reconcile(ctx, domain.ProductIssueReconcileRequest{})
	if err != nil || resp.ItemsChecked != 8 || resp.More {
		t.Fatalf("resp = %+v err=%v: a short deadline must still do several units", resp, err)
	}
	// A long deadline keeps back the full 15 s.
	long, cancel2 := context.WithDeadline(context.Background(), clock.t.Add(10*time.Minute))
	defer cancel2()
	if r := svc.newBudget(long).reserve; r != 15*time.Second {
		t.Errorf("reserve with a long deadline = %v, want 15s", r)
	}
}
