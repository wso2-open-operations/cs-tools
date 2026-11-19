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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	pwWI  = "0190a4b0-0000-7000-8000-0000000000c1"
	pwCmd = "0190a4b0-0000-7000-8000-0000000000c2"
)

// fakeCommandRepo is an in-memory GitHubCommandRepository for the service tests.
type fakeCommandRepo struct {
	link    *repository.CommandLink
	linkErr error
	name    string
	cmds    map[string]*repository.GitHubCommand
	byKey   map[string]string
	inserts int
	pendErr error
}

func newFakeCommandRepo() *fakeCommandRepo {
	b := "WAITING_ON_CS"
	board := 3
	return &fakeCommandRepo{
		link: &repository.CommandLink{WorkItemID: pwWI, Owner: "o", Repository: "r", IssueNumber: 5, BoardStatus: &b,
			GitHubState: "OPEN", LinkState: "ACTIVE", Org: "o", BoardProjectNumber: &board},
		cmds: map[string]*repository.GitHubCommand{}, byKey: map[string]string{},
	}
}

func (f *fakeCommandRepo) add(kind, status, key string) *repository.GitHubCommand {
	c := &repository.GitHubCommand{ID: fmt.Sprintf("0190a4b0-0000-7000-8000-%012d", 100+len(f.cmds)), WorkItemID: pwWI, Kind: kind, Status: status,
		RequestedBy: "a@wso2.com", CreatedOn: time.Now(), NextAttemptOn: time.Now(), Payload: []byte(`{"status":"WAITING_ON_PRODUCT_TEAM","rawStatus":"WOW"}`)}
	f.cmds[c.ID] = c
	if key != "" {
		f.byKey[key] = c.ID
		c.IdempotencyKey = &key
	}
	return c
}

func (f *fakeCommandRepo) InsertCommand(_ context.Context, in repository.NewGitHubCommand) (repository.GitHubCommand, bool, error) {
	if id, ok := f.byKey[in.IdempotencyKey]; ok && in.IdempotencyKey != "" {
		return *f.cmds[id], true, nil
	}
	if f.pendErr != nil {
		return repository.GitHubCommand{}, false, f.pendErr
	}
	f.inserts++
	c := f.add(in.Kind, "PENDING", in.IdempotencyKey)
	c.Payload, c.RequestedBy = in.Payload, in.RequestedBy
	return *c, false, nil
}
func (f *fakeCommandRepo) InsertPortalComment(_ context.Context, in repository.PortalCommentInput) (repository.PortalCommentResult, error) {
	if id, ok := f.byKey[in.IdempotencyKey]; ok && in.IdempotencyKey != "" {
		return repository.PortalCommentResult{Command: *f.cmds[id], Comment: repository.PortalComment{ID: "cm1", Author: in.RequestedBy, Body: "first"}, Existed: true}, nil
	}
	f.inserts++
	c := f.add(repository.CommandAddComment, "PENDING", in.IdempotencyKey)
	c.Payload = []byte(`{"body":` + jsonString(in.GitHubBody) + `}`)
	c.RequestedBy = in.RequestedBy
	return repository.PortalCommentResult{Command: *c, Comment: repository.PortalComment{ID: "cm1", Author: in.RequestedBy, Body: in.Content, CreatedOn: time.Now()}}, nil
}
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
func (f *fakeCommandRepo) FindCommandByKey(_ context.Context, _, key string) (*repository.GitHubCommand, error) {
	if id, ok := f.byKey[key]; ok && key != "" {
		return f.cmds[id], nil
	}
	return nil, nil
}
func (f *fakeCommandRepo) ClaimDue(context.Context, int) ([]repository.GitHubCommand, error) {
	return nil, nil
}
func (f *fakeCommandRepo) MarkDelivered(context.Context, string, int, *int) error { return nil }
func (f *fakeCommandRepo) MarkFailed(context.Context, string, int, string, *int) error {
	return nil
}
func (f *fakeCommandRepo) MarkCancelled(context.Context, string, string) error { return nil }
func (f *fakeCommandRepo) Reschedule(context.Context, string, int, time.Time, string, *int) error {
	return nil
}
func (f *fakeCommandRepo) Retry(_ context.Context, _, id string) (repository.GitHubCommand, error) {
	c := f.cmds[id]
	if c == nil {
		return repository.GitHubCommand{}, repository.ErrCommandNotFound
	}
	if f.pendErr != nil {
		return repository.GitHubCommand{}, f.pendErr
	}
	c.Status, c.Attempts = "PENDING", 0
	return *c, nil
}
func (f *fakeCommandRepo) Get(_ context.Context, id string) (*repository.GitHubCommand, error) {
	return f.cmds[id], nil
}
func (f *fakeCommandRepo) ListForWorkItem(context.Context, string) ([]repository.GitHubCommand, error) {
	return nil, nil
}
func (f *fakeCommandRepo) SetPortalCommentGitHubID(context.Context, string, int64, string, *time.Time) (bool, error) {
	return true, nil
}
func (f *fakeCommandRepo) PortalCommentGitHubID(context.Context, string) (*int64, error) {
	return nil, nil
}
func (f *fakeCommandRepo) GitHubCommentRecorded(context.Context, int64) (bool, error) {
	return false, nil
}
func (f *fakeCommandRepo) UserDisplayName(context.Context, string) (string, error) {
	return f.name, nil
}
func (f *fakeCommandRepo) LinkForCommand(context.Context, string) (*repository.CommandLink, error) {
	if f.linkErr != nil {
		return nil, f.linkErr
	}
	l := *f.link
	return &l, nil
}
func (f *fakeCommandRepo) SetBoardItemID(context.Context, string, string) error { return nil }
func (f *fakeCommandRepo) BeginAttempt(context.Context, string, int) (bool, error) {
	return true, nil
}

func writeCtx(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

func newWriteSvc(f *fakeCommandRepo) ProductIssueWriteService {
	return NewProductIssueWriteService(f, internalAccess{})
}

func wantPIConflict(t *testing.T, err error, code string) {
	t.Helper()
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || ce.Code != code {
		t.Fatalf("err = %v, want 409 %s", err, code)
	}
}

func TestProductIssueWrite_AddComment(t *testing.T) {
	f := newFakeCommandRepo()
	f.name = "Jane  *Doe*"
	svc := newWriteSvc(f)
	ctx := writeCtx("jane@wso2.com")

	resp, replay, err := svc.AddComment(ctx, pwWI, "k1", domain.AddProductIssueCommentRequest{Body: "  hello\nworld  "})
	if err != nil || replay {
		t.Fatalf("add: %v replay=%v", err, replay)
	}
	if resp.Comment.Body != "hello\nworld" || resp.Comment.Origin == nil || *resp.Comment.Origin != "PORTAL" ||
		resp.Comment.Delivery == nil || *resp.Comment.Delivery != "PENDING" || resp.Comment.CommandID == nil || *resp.Comment.CommandID != resp.Command.ID {
		t.Errorf("comment = %+v", resp.Comment)
	}
	if resp.Command.Kind != "ADD_COMMENT" || resp.Command.Status != "PENDING" {
		t.Errorf("command = %+v", resp.Command)
	}
	// P6 attribution: bold name (emphasis markers removed), (CS) wrote:, blank line, text.
	if got := string(f.cmds[resp.Command.ID].Payload); !strings.Contains(got, `**Jane Doe** (CS) wrote:\n\nhello\nworld`) {
		t.Errorf("payload = %s", got)
	}

	// Replay: same key returns the first command, queues nothing (even after the link goes inactive).
	f.link.LinkState = "DELETED"
	again, replay, err := svc.AddComment(ctx, pwWI, "k1", domain.AddProductIssueCommentRequest{Body: "hello"})
	if err != nil || !replay || again.Command.ID != resp.Command.ID || f.inserts != 1 {
		t.Fatalf("replay: %v %v %+v inserts=%d", err, replay, again.Command, f.inserts)
	}
	// A new request with the link inactive is refused.
	_, _, err = svc.AddComment(ctx, pwWI, "k2", domain.AddProductIssueCommentRequest{Body: "x"})
	wantPIConflict(t, err, apierror.CodeLinkInactive)
}

func TestProductIssueWrite_CommentFallsBackToEmailAndValidates(t *testing.T) {
	f := newFakeCommandRepo()
	svc := newWriteSvc(f)
	resp, _, err := svc.AddComment(writeCtx("jane@wso2.com"), pwWI, "", domain.AddProductIssueCommentRequest{Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(f.cmds[resp.Command.ID].Payload); !strings.Contains(got, `**jane@wso2.com** (CS) wrote:`) {
		t.Errorf("payload = %s", got)
	}
	for name, body := range map[string]string{"empty": "", "blank": " \n\t", "too long": strings.Repeat("a", 60001)} {
		_, _, err := svc.AddComment(writeCtx("jane@wso2.com"), pwWI, "", domain.AddProductIssueCommentRequest{Body: body})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want 400", name, err)
		}
	}
	if _, _, err := svc.AddComment(writeCtx("jane@wso2.com"), pwWI, strings.Repeat("k", 201), domain.AddProductIssueCommentRequest{Body: "x"}); err == nil {
		t.Error("an over-long Idempotency-Key was accepted")
	}
	if _, _, err := svc.AddComment(writeCtx(""), pwWI, "", domain.AddProductIssueCommentRequest{Body: "x"}); err == nil {
		t.Error("no signed-in user was accepted")
	}
	if _, _, err := svc.AddComment(writeCtx("a@wso2.com"), "not-a-uuid", "", domain.AddProductIssueCommentRequest{Body: "x"}); err == nil {
		t.Error("a malformed id was accepted")
	}
	f.linkErr = repository.ErrProductIssueLinkNotFound
	_, _, err = svc.AddComment(writeCtx("a@wso2.com"), pwWI, "", domain.AddProductIssueCommentRequest{Body: "x"})
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("not a product issue: %v, want 404", err)
	}
}

func TestProductIssueWrite_NonInternalCallerRefused(t *testing.T) {
	svc := NewProductIssueWriteService(newFakeCommandRepo(), customerAccess{})
	_, _, err := svc.AddComment(writeCtx("c@example.com"), pwWI, "", domain.AddProductIssueCommentRequest{Body: "x"})
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want 403", err)
	}
}

func TestProductIssueWrite_SetBoardStatus(t *testing.T) {
	ctx := writeCtx("a@wso2.com")
	req := domain.SetProductIssueBoardStatusRequest{Status: "WAITING_ON_PRODUCT_TEAM"}

	f := newFakeCommandRepo()
	svc := newWriteSvc(f)
	cmd, replay, err := svc.SetBoardStatus(ctx, pwWI, "k", req)
	if err != nil || replay || cmd.Kind != "SET_BOARD_STATUS" || cmd.TargetStatus == nil || *cmd.TargetStatus != "WAITING_ON_PRODUCT_TEAM" {
		t.Fatalf("move: %+v replay=%v err=%v", cmd, replay, err)
	}
	// Replay answers the first command even though a command is now pending.
	again, replay, err := svc.SetBoardStatus(ctx, pwWI, "k", req)
	if err != nil || !replay || again.ID != cmd.ID {
		t.Fatalf("replay: %+v %v %v", again, replay, err)
	}
	// The same key used for a close is refused.
	if _, _, err := svc.Close(ctx, pwWI, "k"); err == nil {
		t.Error("a key used for another kind was accepted")
	} else {
		wantPIConflict(t, err, apierror.CodeIdempotencyKeyReused)
	}

	_, _, err = svc.SetBoardStatus(ctx, pwWI, "", domain.SetProductIssueBoardStatusRequest{Status: "RESOLVED"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Code != apierror.CodeUnsupportedStatus {
		t.Errorf("unsupported status: %v", err)
	}

	refusals := map[string]struct {
		edit func(l *repository.CommandLink)
		code string
	}{
		"not waiting on CS": {func(l *repository.CommandLink) { s := "OPEN"; l.BoardStatus = &s }, apierror.CodeNotWaitingOnCS},
		"no board status":   {func(l *repository.CommandLink) { l.BoardStatus = nil }, apierror.CodeNotWaitingOnCS},
		"link inactive":     {func(l *repository.CommandLink) { l.LinkState = "TRANSFERRED" }, apierror.CodeLinkInactive},
		"no tracking board": {func(l *repository.CommandLink) { l.BoardProjectNumber = nil }, apierror.CodeNoTrackingBoard},
	}
	for name, tc := range refusals {
		g := newFakeCommandRepo()
		tc.edit(g.link)
		_, _, err := newWriteSvc(g).SetBoardStatus(ctx, pwWI, "", req)
		t.Run(name, func(t *testing.T) { wantPIConflict(t, err, tc.code) })
	}
	g := newFakeCommandRepo()
	g.pendErr = repository.ErrCommandPending
	_, _, err = newWriteSvc(g).SetBoardStatus(ctx, pwWI, "", req)
	wantPIConflict(t, err, apierror.CodeCommandPending)
}

func TestProductIssueWrite_Close(t *testing.T) {
	ctx := writeCtx("a@wso2.com")
	f := newFakeCommandRepo()
	svc := newWriteSvc(f)
	cmd, replay, err := svc.Close(ctx, pwWI, "k")
	if err != nil || replay || cmd.Kind != "SET_STATE" || !strings.Contains(string(f.cmds[cmd.ID].Payload), `"stateReason":"completed"`) {
		t.Fatalf("close: %+v %v %v", cmd, replay, err)
	}
	if again, replay, err := svc.Close(ctx, pwWI, "k"); err != nil || !replay || again.ID != cmd.ID {
		t.Fatalf("replay: %+v %v %v", again, replay, err)
	}
	f.link.GitHubState = "CLOSED"
	_, _, err = svc.Close(ctx, pwWI, "other")
	wantPIConflict(t, err, apierror.CodeAlreadyClosed)
	f.link.GitHubState, f.link.LinkState = "OPEN", "DELETED"
	_, _, err = svc.Close(ctx, pwWI, "other")
	wantPIConflict(t, err, apierror.CodeLinkInactive)
	g := newFakeCommandRepo()
	g.pendErr = repository.ErrCommandPending
	_, _, err = newWriteSvc(g).Close(ctx, pwWI, "")
	wantPIConflict(t, err, apierror.CodeCommandPending)
}

func TestProductIssueWrite_Retry(t *testing.T) {
	ctx := writeCtx("a@wso2.com")
	f := newFakeCommandRepo()
	svc := newWriteSvc(f)

	failed := f.add(repository.CommandSetBoardStatus, "FAILED", "")
	cmd, err := svc.Retry(ctx, pwWI, failed.ID)
	if err != nil || cmd.Status != "PENDING" || cmd.Attempts != 0 {
		t.Fatalf("retry: %+v %v", cmd, err)
	}
	// Now PENDING: not failed.
	_, err = svc.Retry(ctx, pwWI, failed.ID)
	wantPIConflict(t, err, apierror.CodeCommandNotFailed)

	// A failed move retried after the issue left WOC.
	mv := f.add(repository.CommandSetBoardStatus, "FAILED", "")
	s := "IN_PROGRESS"
	f.link.BoardStatus = &s
	_, err = svc.Retry(ctx, pwWI, mv.ID)
	wantPIConflict(t, err, apierror.CodeNotWaitingOnCS)
	// A failed close retried after the issue closed.
	cl := f.add(repository.CommandSetState, "FAILED", "")
	f.link.GitHubState = "CLOSED"
	_, err = svc.Retry(ctx, pwWI, cl.ID)
	wantPIConflict(t, err, apierror.CodeAlreadyClosed)
	// A failed comment retried with the link inactive.
	cm := f.add(repository.CommandAddComment, "FAILED", "")
	f.link.LinkState = "DELETED"
	_, err = svc.Retry(ctx, pwWI, cm.ID)
	wantPIConflict(t, err, apierror.CodeLinkInactive)
	f.link.LinkState = "ACTIVE"
	if _, err = svc.Retry(ctx, pwWI, cm.ID); err != nil {
		t.Errorf("retry of a comment: %v", err)
	}

	// Unknown command, another item's command, malformed id: 404.
	other := f.add(repository.CommandSetState, "FAILED", "")
	other.WorkItemID = "0190a4b0-0000-7000-8000-0000000000ff"
	for _, id := range []string{"0190a4b0-0000-7000-8000-0000000000aa", other.ID, "nope"} {
		_, err := svc.Retry(ctx, pwWI, id)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("retry %q: %v, want 404", id, err)
		}
	}
	g := newFakeCommandRepo()
	g.add(repository.CommandSetState, "FAILED", "")
	g.pendErr = repository.ErrCommandPending
	for id := range g.cmds {
		_, err = newWriteSvc(g).Retry(ctx, pwWI, id)
		wantPIConflict(t, err, apierror.CodeCommandPending)
	}
}

func TestClassifyCommandError(t *testing.T) {
	status := func(n int) *int { return &n }
	cases := []struct {
		name      string
		err       error
		permanent bool
		status    *int
		after     time.Duration
	}{
		{"no answer", &engineeringentity.Error{}, false, nil, 0},
		{"plain error", errors.New("x"), false, nil, 0},
		{"github 404", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 404}, true, status(404), 0},
		{"github 410", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 410}, true, status(410), 0},
		{"github 422", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 422}, true, status(422), 0},
		{"github 403 plain", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 403}, true, status(403), 0},
		{"github 403 rate limit", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 403, RetryAfter: 40 * time.Second}, false, status(403), 40 * time.Second},
		{"429", &engineeringentity.Error{StatusCode: 429, RetryAfter: time.Minute}, false, status(429), time.Minute},
		{"github 500", &engineeringentity.Error{StatusCode: 502, GitHubStatus: 500}, false, status(500), 0},
		{"eng 502 no header", &engineeringentity.Error{StatusCode: 502}, false, status(502), 0},
		{"eng 400", &engineeringentity.Error{StatusCode: 400}, true, status(400), 0},
		{"eng 404 no header", &engineeringentity.Error{StatusCode: 404}, false, status(404), 0},
		{"eng 403 no header", &engineeringentity.Error{StatusCode: 403}, false, status(403), 0},
		{"not on board", &retryableError{reason: "not on the tracking board"}, false, nil, 0},
		{"choreo 401", &engineeringentity.Error{StatusCode: 401}, false, status(401), 0},
		{"409", &engineeringentity.Error{StatusCode: 409}, false, status(409), 0},
	}
	for _, tc := range cases {
		got := classifyCommandError(tc.err)
		if got.permanent != tc.permanent || got.retryAfter != tc.after || (tc.status == nil) != (got.status == nil) ||
			(tc.status != nil && *tc.status != *got.status) || got.reason == "" {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}

func TestCommandBackoff(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 32 * time.Minute, time.Hour, time.Hour}
	for i, w := range want {
		if got := commandBackoff(i + 1); got != w {
			t.Errorf("attempt %d: %v, want %v", i+1, got, w)
		}
	}
	if commandBackoff(0) != 30*time.Second || commandBackoff(500) != time.Hour {
		t.Error("backoff bounds")
	}
}
