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
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	commandBatchSize      = 10
	commandCallTimeout    = 2 * time.Minute
	commandBackoffBase    = 30 * time.Second
	commandBackoffMax     = time.Hour
	commandGiveUpAfter    = 24 * time.Hour
	commandItemPages      = 50
	commandItemPageSize   = 100
	commandCommentPages   = 5
	commandCommentPerPage = 100
	// commandRecordTimeout bounds writing an outcome, which must happen even when
	// the run's own deadline has just passed.
	commandRecordTimeout = 10 * time.Second
	// commandClockSkew widens the "comments since" lookup window backwards.
	commandClockSkew = 2 * time.Minute

	reasonLeftWOC       = "issue left WOC before delivery"
	reasonLinkInactive  = "the GitHub issue is no longer linked"
	reasonNotOnBoard    = "not on the tracking board"
	reasonNoBoard       = "no tracking board for the repository"
	reasonNoWOWOption   = "the board has no WOW status option"
	reasonNoStatusField = "the board has no Status field"
	reasonBadPayload    = "the queued request is malformed"
	reasonGaveUp        = "gave up after 24 h"
)

// ProductIssueCommandEngineering is the engineering entity the worker calls.
type ProductIssueCommandEngineering interface {
	AddIssueComment(ctx context.Context, org, owner, repo string, number int, body string) (engineeringentity.IssueComment, error)
	CloseIssue(ctx context.Context, org, owner, repo string, number int, stateReason string) (engineeringentity.IssueState, error)
	ListIssueComments(ctx context.Context, org, owner, repo string, number int, q engineeringentity.ListCommentsQuery) ([]engineeringentity.IssueComment, error)
	ListProjectFields(ctx context.Context, org string, projectNumber int) ([]engineeringentity.ProjectField, error)
	ListProjectItemsWithContent(ctx context.Context, org string, projectNumber int, after string, perPage int) (engineeringentity.ProjectItemsPage, error)
	UpdateProjectItemStatus(ctx context.Context, org string, projectNumber int, itemID, fieldID int64, optionID string) error
}

type boardKey struct {
	org   string
	board int
}

type boardStatusField struct {
	fieldID int64
	options map[string]string // normalised option name -> option id
}

// ProductIssueCommandWorker delivers the queued portal actions of product issues
// (github_command) to GitHub through the engineering entity: a comment, the board
// move WOC to WOW, and closing the issue. It never changes the issue's stored
// board or GitHub state; GitHub's own webhook (or the reconciliation) does, once
// the change has happened.
type ProductIssueCommandWorker struct {
	repo     repository.GitHubCommandRepository
	eng      ProductIssueCommandEngineering
	interval time.Duration
	now      func() time.Time
	// callTimeout bounds one command's run (calls included); recording its outcome
	// is bounded separately and survives this deadline.
	callTimeout time.Duration

	mu     sync.Mutex
	boards map[boardKey]*boardStatusField
}

// NewProductIssueCommandWorker constructs the worker; a non-positive interval is 10s.
func NewProductIssueCommandWorker(repo repository.GitHubCommandRepository, eng ProductIssueCommandEngineering, interval time.Duration) *ProductIssueCommandWorker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &ProductIssueCommandWorker{repo: repo, eng: eng, interval: interval, now: time.Now, callTimeout: commandCallTimeout, boards: map[boardKey]*boardStatusField{}}
}

// Run delivers until ctx is cancelled. It runs under the system identity.
func (w *ProductIssueCommandWorker) Run(ctx context.Context) {
	ctx = repository.WithSystemIdentity(ctx)
	slog.InfoContext(ctx, "git internals: write-back worker started", "interval", w.interval)
	for {
		n, err := w.RunOnce(ctx)
		if ctx.Err() != nil {
			slog.InfoContext(ctx, "git internals: write-back worker stopped")
			return
		}
		if err != nil {
			slog.ErrorContext(ctx, "git internals: write-back claim failed", "err", err)
		}
		if n == commandBatchSize {
			continue
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "git internals: write-back worker stopped")
			return
		case <-time.After(w.interval):
		}
	}
}

// RunOnce claims the due commands and delivers each; it returns how many it took.
func (w *ProductIssueCommandWorker) RunOnce(ctx context.Context) (int, error) {
	ctx = repository.WithSystemIdentity(ctx)
	cmds, err := w.repo.ClaimDue(ctx, commandBatchSize)
	if err != nil {
		return 0, err
	}
	for _, c := range cmds {
		cctx, cancel := context.WithTimeout(ctx, w.callTimeout)
		w.process(cctx, c)
		cancel()
		if ctx.Err() != nil {
			break
		}
	}
	return len(cmds), nil
}

// outcome is what one delivery attempt came to. Exactly one of delivered, cancel,
// fail and err is set.
type outcome struct {
	delivered bool
	cancel    string // CANCELLED with this reason
	fail      string // FAILED with this reason, no retry
	err       error  // a call failed: classified
	skip      bool   // not run (the claim lease was gone): left for the next claim
}

// retryableError is a failure that is not an answer from the engineering entity
// but should be retried within the 24 h window.
type retryableError struct{ reason string }

func (e *retryableError) Error() string { return e.reason }

// attemptState tracks the attempt counter of one run: attempts counts every run
// that reached GitHub, and is written before the first call, so a crash between
// the call and its record still leaves attempts > 0 (the comment lookup then runs).
type attemptState struct {
	attempts int
	counted  bool
}

// begin records the attempt (and that the action may now be on GitHub) and renews
// the lease for a full five minutes, before the first call of the run. proceed is
// false when the claim's lease has already run out (a long batch): another worker
// may have the command, so this run must not call out and records nothing.
func (w *ProductIssueCommandWorker) begin(ctx context.Context, cmd repository.GitHubCommand, st *attemptState) (out outcome, proceed bool) {
	if st.counted {
		return outcome{}, true
	}
	ok, err := w.repo.BeginAttempt(ctx, cmd.ID, st.attempts+1)
	if err != nil {
		return outcome{err: err}, false
	}
	if !ok {
		slog.WarnContext(ctx, "git internals: command skipped, its claim lease ran out or it changed", "command", cmd.ID, "kind", cmd.Kind)
		return outcome{skip: true}, false
	}
	st.attempts++
	st.counted = true
	return outcome{}, true
}

func (w *ProductIssueCommandWorker) process(ctx context.Context, cmd repository.GitHubCommand) {
	st := &attemptState{attempts: cmd.Attempts}
	var out outcome
	switch cmd.Kind {
	case repository.CommandAddComment:
		out = w.addComment(ctx, cmd, st)
	case repository.CommandSetBoardStatus:
		out = w.setBoardStatus(ctx, cmd, st)
	case repository.CommandSetState:
		out = w.setState(ctx, cmd, st)
	default:
		out = outcome{fail: reasonBadPayload}
	}
	// The run's context may have just timed out; the outcome must still be written,
	// or a timed-out call would neither back off nor ever give up.
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commandRecordTimeout)
	defer cancel()
	w.record(rctx, cmd, st, out)
}

func (w *ProductIssueCommandWorker) record(ctx context.Context, cmd repository.GitHubCommand, st *attemptState, out outcome) {
	var err error
	switch {
	case out.skip:
		return
	case out.delivered:
		err = w.repo.MarkDelivered(ctx, cmd.ID, st.attempts, nil)
	case out.cancel != "":
		err = w.repo.MarkCancelled(ctx, cmd.ID, out.cancel)
	case out.fail != "":
		err = w.repo.MarkFailed(ctx, cmd.ID, st.attempts, out.fail, nil)
	default:
		err = w.recordError(ctx, cmd, st, out.err)
	}
	if err != nil {
		// The row stays PENDING and is due again when its lease ends.
		slog.ErrorContext(ctx, "git internals: recording a command outcome failed", "command", cmd.ID, "kind", cmd.Kind, "err", err)
	}
}

func (w *ProductIssueCommandWorker) recordError(ctx context.Context, cmd repository.GitHubCommand, st *attemptState, callErr error) error {
	cl := classifyCommandError(callErr)
	attempts := st.attempts
	if !st.counted {
		attempts++ // a failure with no earlier begin (a database error before any call)
	}
	if cl.permanent {
		slog.WarnContext(ctx, "git internals: command failed permanently", "command", cmd.ID, "kind", cmd.Kind, "reason", cl.reason)
		return w.repo.MarkFailed(ctx, cmd.ID, attempts, cl.reason, cl.status)
	}
	now := w.now()
	if now.Sub(cmd.WindowStartedOn) > commandGiveUpAfter {
		slog.WarnContext(ctx, "git internals: command gave up", "command", cmd.ID, "kind", cmd.Kind, "attempts", attempts)
		return w.repo.MarkFailed(ctx, cmd.ID, attempts, reasonGaveUp, cl.status)
	}
	wait := commandBackoff(attempts)
	if cl.retryAfter > wait {
		wait = cl.retryAfter
	}
	slog.InfoContext(ctx, "git internals: command will be retried", "command", cmd.ID, "kind", cmd.Kind, "attempts", attempts, "reason", cl.reason, "in", wait)
	return w.repo.Reschedule(ctx, cmd.ID, attempts, now.Add(wait), cl.reason, cl.status)
}

// commandBackoff is min(30s * 2^(attempts-1), 1h).
func commandBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 8 {
		return commandBackoffMax
	}
	d := commandBackoffBase << (attempts - 1)
	if d > commandBackoffMax {
		return commandBackoffMax
	}
	return d
}

type commandClass struct {
	permanent  bool
	reason     string // a classification, never a GitHub body
	status     *int
	retryAfter time.Duration
}

// classifyCommandError decides permanent versus retryable. GitHub's own status (X-GitHub-Status): 404, 410
// and 422, and 403 without a rate limit, are permanent, as is an engineering entity
// 400. Everything else (no answer, 5xx, 429, 403 with Retry-After, 401, 409, and any
// status that is not GitHub's) is retried.
func classifyCommandError(err error) commandClass {
	var re *retryableError
	if errors.As(err, &re) {
		return commandClass{reason: re.reason}
	}
	var ee *engineeringentity.Error
	if !errors.As(err, &ee) {
		return commandClass{reason: "request failed before an answer"}
	}
	code := ee.GitHubStatus
	if code == 0 {
		code = ee.StatusCode
	}
	// 404, 410, 422 and 403 are permanent only as GitHub's own answer (the header is
	// present). Without it the status is the engineering entity's or the gateway's
	// (scope, route, rollout problems) and says nothing about the action.
	fromGitHub := ee.GitHubStatus != 0
	cl := commandClass{retryAfter: ee.RetryAfter}
	if code > 0 {
		c := code
		cl.status = &c
	}
	switch {
	case ee.StatusCode == http.StatusBadRequest:
		cl.permanent, cl.reason = true, "the engineering entity rejected the request (400)"
	case fromGitHub && code == http.StatusNotFound:
		cl.permanent, cl.reason = true, "not found on GitHub (404)"
	case fromGitHub && code == http.StatusGone:
		cl.permanent, cl.reason = true, "gone on GitHub (410)"
	case fromGitHub && code == http.StatusUnprocessableEntity:
		cl.permanent, cl.reason = true, "GitHub refused the request (422)"
	case fromGitHub && code == http.StatusForbidden && ee.RetryAfter == 0:
		cl.permanent, cl.reason = true, "GitHub refused the request (403)"
	case code == 0:
		cl.reason = "no answer from the engineering entity"
	case code == http.StatusForbidden || code == http.StatusTooManyRequests:
		cl.reason = "rate limited by GitHub (" + strconv.Itoa(code) + ")"
	case code >= 500:
		cl.reason = "server error (" + strconv.Itoa(code) + ")"
	default:
		cl.reason = "unexpected answer (" + strconv.Itoa(code) + ")"
	}
	return cl
}

// link reads the issue's link; ok false means the outcome is already decided.
func (w *ProductIssueCommandWorker) link(ctx context.Context, cmd repository.GitHubCommand) (*repository.CommandLink, outcome, bool) {
	l, err := w.repo.LinkForCommand(ctx, cmd.WorkItemID)
	if errors.Is(err, repository.ErrProductIssueLinkNotFound) {
		return nil, outcome{cancel: reasonLinkInactive}, false
	}
	if err != nil {
		return nil, outcome{err: err}, false
	}
	return l, outcome{}, true
}

func (w *ProductIssueCommandWorker) addComment(ctx context.Context, cmd repository.GitHubCommand, st *attemptState) outcome {
	var p struct {
		Body string `json:"body"`
	}
	if json.Unmarshal(cmd.Payload, &p) != nil || p.Body == "" || cmd.CommentID == nil {
		return outcome{fail: reasonBadPayload}
	}
	// The echo may have been adopted already: GitHub has the comment.
	if id, err := w.repo.PortalCommentGitHubID(ctx, *cmd.CommentID); err != nil {
		return outcome{err: err}
	} else if id != nil {
		return outcome{delivered: true}
	}
	l, out, ok := w.link(ctx, cmd)
	if !ok {
		return out
	}
	if l.LinkState != "ACTIVE" {
		return outcome{cancel: reasonLinkInactive}
	}
	if out, proceed := w.begin(ctx, cmd, st); !proceed {
		return out
	}
	// An earlier attempt may have posted it without our learning so (the flag is
	// never reset, so a retried command looks too).
	if cmd.DeliveryAttempted {
		found, err := w.findPostedComment(ctx, cmd, l, p.Body)
		if err != nil {
			return outcome{err: err}
		}
		if found != nil {
			w.recordCommentID(ctx, cmd, *found)
			return outcome{delivered: true}
		}
	}
	posted, err := w.eng.AddIssueComment(ctx, l.Org, l.Owner, l.Repository, l.IssueNumber, p.Body)
	if err != nil {
		return outcome{err: err}
	}
	w.recordCommentID(ctx, cmd, posted)
	return outcome{delivered: true}
}

// recordCommentID stores the GitHub comment's id on the portal comment. A failure
// does not undo the delivery: the comment is on GitHub either way.
func (w *ProductIssueCommandWorker) recordCommentID(ctx context.Context, cmd repository.GitHubCommand, c engineeringentity.IssueComment) {
	var updated *time.Time
	if t, err := time.Parse(time.RFC3339, c.UpdatedAt); err == nil {
		updated = &t
	}
	author := ""
	if c.AuthorLogin != nil {
		author = *c.AuthorLogin
	}
	if _, err := w.repo.SetPortalCommentGitHubID(ctx, *cmd.CommentID, c.ID, author, updated); err != nil {
		slog.WarnContext(ctx, "git internals: could not record the GitHub comment id", "command", cmd.ID, "err", err)
	}
}

func normaliseNewlines(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// findPostedComment looks for a comment of the issue made since the command was
// queued whose body is exactly the payload and whose id is not recorded yet.
func (w *ProductIssueCommandWorker) findPostedComment(ctx context.Context, cmd repository.GitHubCommand, l *repository.CommandLink, body string) (*engineeringentity.IssueComment, error) {
	want := normaliseNewlines(body)
	since := cmd.CreatedOn.Add(-commandClockSkew).UTC().Format(time.RFC3339)
	for page := 1; page <= commandCommentPages; page++ {
		comments, err := w.eng.ListIssueComments(ctx, l.Org, l.Owner, l.Repository, l.IssueNumber,
			engineeringentity.ListCommentsQuery{Since: since, PerPage: commandCommentPerPage, Page: page})
		if err != nil {
			return nil, err
		}
		for i := range comments {
			c := comments[i]
			if c.ID <= 0 || normaliseNewlines(c.Body) != want {
				continue
			}
			recorded, err := w.repo.GitHubCommentRecorded(ctx, c.ID)
			if err != nil {
				return nil, err
			}
			if !recorded {
				return &c, nil
			}
		}
		if len(comments) < commandCommentPerPage {
			return nil, nil
		}
	}
	slog.WarnContext(ctx, "git internals: the posted-comment lookup hit its page cap without an answer", "command", cmd.ID, "pages", commandCommentPages)
	return nil, nil
}

func normaliseOptionName(s string) string {
	return strings.ToLower(strings.TrimSpace(html.UnescapeString(s)))
}

// statusField returns the board's Status field with its options, from the cache or
// (refresh true, or never read) the engineering entity.
func (w *ProductIssueCommandWorker) statusField(ctx context.Context, key boardKey, refresh bool) (*boardStatusField, error) {
	w.mu.Lock()
	cached := w.boards[key]
	w.mu.Unlock()
	if cached != nil && !refresh {
		return cached, nil
	}
	fields, err := w.eng.ListProjectFields(ctx, key.org, key.board)
	if err != nil {
		return nil, err
	}
	var out *boardStatusField
	for _, f := range fields {
		if strings.EqualFold(strings.TrimSpace(f.Name), "Status") && strings.EqualFold(f.DataType, "single_select") {
			out = &boardStatusField{fieldID: f.ID, options: map[string]string{}}
			for _, o := range f.Options {
				out.options[normaliseOptionName(o.Name)] = o.ID
			}
			break
		}
	}
	w.mu.Lock()
	if out != nil {
		w.boards[key] = out
	} else {
		delete(w.boards, key)
	}
	w.mu.Unlock()
	return out, nil
}

func (w *ProductIssueCommandWorker) forgetBoard(key boardKey) {
	w.mu.Lock()
	delete(w.boards, key)
	w.mu.Unlock()
}

func (w *ProductIssueCommandWorker) setBoardStatus(ctx context.Context, cmd repository.GitHubCommand, st *attemptState) outcome {
	var p struct {
		RawStatus string `json:"rawStatus"`
	}
	if json.Unmarshal(cmd.Payload, &p) != nil || strings.TrimSpace(p.RawStatus) == "" {
		return outcome{fail: reasonBadPayload}
	}
	l, out, ok := w.link(ctx, cmd)
	if !ok {
		return out
	}
	if l.LinkState != "ACTIVE" || l.BoardStatus == nil || *l.BoardStatus != "WAITING_ON_CS" {
		return outcome{cancel: reasonLeftWOC}
	}
	if l.BoardProjectNumber == nil {
		return outcome{fail: reasonNoBoard}
	}
	board := *l.BoardProjectNumber
	key := boardKey{org: l.Org, board: board}
	if out, proceed := w.begin(ctx, cmd, st); !proceed {
		return out
	}

	name := normaliseOptionName(p.RawStatus)
	field, err := w.statusField(ctx, key, false)
	if err != nil {
		return outcome{err: err}
	}
	optionID := ""
	if field != nil {
		optionID = field.options[name]
	}
	if optionID == "" { // a miss: read the board's fields once more
		if field, err = w.statusField(ctx, key, true); err != nil {
			return outcome{err: err}
		}
		if field == nil {
			return outcome{fail: reasonNoStatusField}
		}
		if optionID = field.options[name]; optionID == "" {
			return outcome{fail: reasonNoWOWOption}
		}
	}

	itemID, out, ok := w.itemID(ctx, l, board)
	if !ok {
		return out
	}
	if err := w.eng.UpdateProjectItemStatus(ctx, l.Org, board, itemID, field.fieldID, optionID); err != nil {
		var ee *engineeringentity.Error
		if errors.As(err, &ee) && (ee.StatusCode == http.StatusNotFound || ee.StatusCode == http.StatusUnprocessableEntity) {
			w.forgetBoard(key) // the ids may be stale: read them again next time
		}
		return outcome{err: err}
	}
	return outcome{delivered: true}
}

// itemID is the issue's numeric item id on the board: stored on the link, else
// found by paging the board's items for the issue's node id.
func (w *ProductIssueCommandWorker) itemID(ctx context.Context, l *repository.CommandLink, board int) (int64, outcome, bool) {
	if l.BoardItemID != nil {
		if id, err := strconv.ParseInt(strings.TrimSpace(*l.BoardItemID), 10, 64); err == nil && id > 0 {
			return id, outcome{}, true
		}
	}
	if l.NodeID == nil || *l.NodeID == "" {
		return 0, outcome{err: &retryableError{reason: reasonNotOnBoard}}, false
	}
	after := ""
	for range commandItemPages {
		page, err := w.eng.ListProjectItemsWithContent(ctx, l.Org, board, after, commandItemPageSize)
		if err != nil {
			return 0, outcome{err: err}, false
		}
		for _, it := range page.Items {
			if it.Content == nil || it.Content.NodeID != *l.NodeID {
				continue
			}
			if it.ItemID <= 0 { // an engineering entity without the field
				return 0, outcome{err: &retryableError{reason: reasonNotOnBoard}}, false
			}
			if err := w.repo.SetBoardItemID(ctx, l.WorkItemID, strconv.FormatInt(it.ItemID, 10)); err != nil {
				slog.WarnContext(ctx, "git internals: could not store the board item id", "workItem", l.WorkItemID, "err", err)
			}
			return it.ItemID, outcome{}, true
		}
		if page.NextCursor == nil || *page.NextCursor == "" || len(page.Items) == 0 {
			break
		}
		after = *page.NextCursor
	}
	return 0, outcome{err: &retryableError{reason: reasonNotOnBoard}}, false
}

func (w *ProductIssueCommandWorker) setState(ctx context.Context, cmd repository.GitHubCommand, st *attemptState) outcome {
	l, out, ok := w.link(ctx, cmd)
	if !ok {
		return out
	}
	if l.LinkState != "ACTIVE" {
		return outcome{cancel: reasonLinkInactive}
	}
	if out, proceed := w.begin(ctx, cmd, st); !proceed {
		return out
	}
	res, err := w.eng.CloseIssue(ctx, l.Org, l.Owner, l.Repository, l.IssueNumber, "completed")
	if err != nil {
		return outcome{err: err}
	}
	// An answer showing the issue closed (including one that already was) is done.
	if res.State != "" && !strings.EqualFold(res.State, "closed") {
		return outcome{err: &retryableError{reason: "the answer shows the issue still open"}}
	}
	return outcome{delivered: true}
}
