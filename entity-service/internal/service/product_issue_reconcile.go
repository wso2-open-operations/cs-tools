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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Reconciliation and backfill of product issues (git internals), called by the
// operator or the scheduled tasks component with a service credential. Both
// reuse the webhook branch's repository functions with the sources RECONCILE and
// BACKFILL. A call works against a time budget (the request's own deadline, minus
// a reserve) and always makes progress: it returns a cursor and more=true instead
// of running into the deadline, and stops at once on an engineering 429.
//
// Known limits:
//   - A board item whose history read failed is counted as failed and its link is
//     rechecked in the missing phase of the same run, where it counts as not on
//     the board for that run.
//   - The board listing is paged by an offset, so items added to or removed from a
//     board between two calls can shift the page: an item may be skipped by this
//     run, and its status correction then waits for the next day's run.
//   - The REST board listing does not return archived items (an archived item is
//     omitted, also when asking for archived ones), so an archived item is covered
//     only by the missing phase, which refreshes its issue fields but not a board
//     status.
const (
	reconcileDefaultLimit = 200
	reconcileMaxLimit     = 500
	// reconcilePageSize is the board items read per engineering call (its maximum).
	reconcilePageSize = 100
	// reconcileMissingBatch is how many not-seen links are read from the database at once.
	reconcileMissingBatch = 50

	backfillDefaultMaxIssues = 25
	backfillMaxIssues        = 100
	backfillDefaultSinceDays = 180
	backfillMaxSinceDays     = 3650
	backfillPageSize         = 100
	backfillCommentPageSize  = 100
	backfillMaxCommentPages  = 200
	backfillActor            = "github-backfill"
	maxFailedIssueNumbers    = 100

	// syncTimeReserve is the most of the request's deadline that is kept back: a
	// call starts no further issue once less is left, so the one in hand can
	// finish. A short deadline keeps back only a quarter of what is left at the
	// start of the call (see newBudget), so it still makes several units.
	syncTimeReserve = 15 * time.Second
	// syncDefaultBudget bounds a call whose context has no deadline.
	syncDefaultBudget = 30 * time.Second
	// defaultRetryAfter is told to the caller when a 429 carried no Retry-After.
	defaultRetryAfter = 60 * time.Second

	phaseItems   = "items"
	phaseMissing = "missing"
	phaseOpen    = "open"
	phaseClosed  = "closed"
)

// ProductIssueSyncService reconciles linked product issues with GitHub and
// backfills the issues of a repository. Service credential only (the route's
// guard); neither method checks a caller itself.
type ProductIssueSyncService interface {
	Reconcile(ctx context.Context, req domain.ProductIssueReconcileRequest) (domain.ProductIssueReconcileResponse, error)
	Backfill(ctx context.Context, req domain.ProductIssueBackfillRequest) (domain.ProductIssueBackfillResponse, error)
}

// productIssueSyncEngineering is what the engineering entity client provides.
type productIssueSyncEngineering interface {
	GetIssueDetail(ctx context.Context, org, owner, repo string, number int) (engineeringentity.IssueDetail, error)
	ListIssueDetails(ctx context.Context, org, owner, repo string, q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error)
	ListIssueComments(ctx context.Context, org, owner, repo string, number int, q engineeringentity.ListCommentsQuery) ([]engineeringentity.IssueComment, error)
	GetProjectStatusHistory(ctx context.Context, org, owner, repo string, number int) ([]engineeringentity.ProjectStatusHistory, error)
	ListProjectItemsWithContent(ctx context.Context, org string, projectNumber int, after string, perPage int) (engineeringentity.ProjectItemsPage, error)
}

type productIssueSync struct {
	eng  productIssueSyncEngineering
	repo repository.ProductIssueReconcileRepository
	// bots are the portal's own GitHub logins (lower case): backfill does not
	// mirror their comments, as the webhook path does not.
	bots map[string]bool

	// Overridable in tests.
	now           func() time.Time
	reserve       time.Duration
	defaultBudget time.Duration
}

// NewProductIssueSyncService constructs the service. botLogins are the portal's
// own GitHub accounts (GIT_INTERNALS_BOT_LOGINS), read the way the webhook branch
// reads them.
func NewProductIssueSyncService(eng productIssueSyncEngineering, repo repository.ProductIssueReconcileRepository, botLogins []string) ProductIssueSyncService {
	return &productIssueSync{eng: eng, repo: repo, bots: botLoginSet(botLogins), now: time.Now, reserve: syncTimeReserve, defaultBudget: syncDefaultBudget}
}

// syncBudget is the time one call may still use.
type syncBudget struct {
	deadline time.Time
	reserve  time.Duration
	now      func() time.Time
}

func (s *productIssueSync) newBudget(ctx context.Context) syncBudget {
	deadline, reserve := s.now().Add(s.defaultBudget), s.reserve
	if d, ok := ctx.Deadline(); ok {
		// The request's own deadline: keep back min(reserve, a quarter of the time
		// the call starts with), so a short REQUEST_TIMEOUT still makes several units.
		deadline = d
		if q := max(d.Sub(s.now())/4, 0); q < reserve {
			reserve = q
		}
	}
	return syncBudget{deadline: deadline, reserve: reserve, now: s.now}
}

// low reports whether the call should start no further work.
func (b syncBudget) low(ctx context.Context) bool {
	return ctx.Err() != nil || b.deadline.Sub(b.now()) < b.reserve
}

// rateLimited reports an engineering 429 and how long to wait.
func rateLimited(err error) (time.Duration, bool) {
	var ee *engineeringentity.Error
	if errors.As(err, &ee) && ee.StatusCode == http.StatusTooManyRequests {
		if ee.RetryAfter > 0 {
			return ee.RetryAfter, true
		}
		return defaultRetryAfter, true
	}
	return 0, false
}

func isNotFound(err error) bool {
	var ee *engineeringentity.Error
	return errors.As(err, &ee) && ee.StatusCode == http.StatusNotFound
}

func retrySeconds(d time.Duration) int { return int(math.Ceil(d.Seconds())) }

func encodeSyncCursor(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeSyncCursor(s string, dst any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		err = json.Unmarshal(b, dst)
	}
	if err != nil {
		return &apierror.ValidationError{Msg: "cursor is not valid."}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reconciliation

// reconcileCursor is where a reconciliation run stands: the board, the phase
// (paging its items, then the linked issues it did not list), and the position in
// it. Started is the run's first call: a link synced since is not "not seen".
type reconcileCursor struct {
	Owner   string    `json:"o,omitempty"`
	Board   int       `json:"b,omitempty"`
	Phase   string    `json:"p,omitempty"`
	After   string    `json:"a,omitempty"`
	Offset  int       `json:"n,omitempty"`
	LastID  string    `json:"w,omitempty"`
	Started time.Time `json:"s"`
}

type reconcileRun struct {
	resp   *domain.ProductIssueReconcileResponse
	budget syncBudget
	work   int
	limit  int
}

// stop reports whether the call must stop before starting more work. The first
// unit of a call is always done, so every call makes progress.
func (r *reconcileRun) stop(ctx context.Context) bool {
	return r.work > 0 && (r.work >= r.limit || r.budget.low(ctx))
}

// Reconcile implements ProductIssueSyncService.
func (s *productIssueSync) Reconcile(ctx context.Context, req domain.ProductIssueReconcileRequest) (domain.ProductIssueReconcileResponse, error) {
	var resp domain.ProductIssueReconcileResponse
	limit := req.Limit
	switch {
	case limit < 0:
		return resp, &apierror.ValidationError{Msg: "limit must not be negative."}
	case limit == 0:
		limit = reconcileDefaultLimit
	case limit > reconcileMaxLimit:
		limit = reconcileMaxLimit
	}
	var cur reconcileCursor
	if req.Cursor != "" {
		if err := decodeSyncCursor(req.Cursor, &cur); err != nil {
			return resp, err
		}
		if cur.Started.IsZero() {
			return resp, &apierror.ValidationError{Msg: "cursor is not valid."}
		}
	} else {
		started, err := s.repo.DatabaseNow(ctx)
		if err != nil {
			return resp, err
		}
		cur.Started = started
	}
	boards, err := s.repo.ListBoards(ctx)
	if err != nil {
		return resp, err
	}
	run := &reconcileRun{resp: &resp, budget: s.newBudget(ctx), limit: limit}

	start := 0
	if cur.Owner != "" || cur.Board != 0 {
		start = sort.Search(len(boards), func(i int) bool {
			o := strings.ToLower(boards[i].Owner)
			c := strings.ToLower(cur.Owner)
			return o > c || (o == c && boards[i].ProjectNumber >= cur.Board)
		})
	}
	for i := start; i < len(boards); i++ {
		b := boards[i]
		pos := reconcileCursor{Owner: b.Owner, Board: b.ProjectNumber, Phase: phaseItems, Started: cur.Started}
		if i == start && strings.EqualFold(b.Owner, cur.Owner) && b.ProjectNumber == cur.Board && cur.Phase != "" {
			pos = cur
			pos.Owner = b.Owner
		}
		if run.stop(ctx) {
			resp.NextCursor, resp.More = encodeSyncCursor(pos), true
			return resp, nil
		}
		resp.Boards++
		next, retry, err := s.reconcileBoard(ctx, run, b, pos)
		if err != nil {
			return resp, err
		}
		if next != nil {
			resp.NextCursor, resp.More = encodeSyncCursor(next), true
			if retry > 0 {
				resp.RetryAfterSeconds = retrySeconds(retry)
			}
			return resp, nil
		}
	}
	return resp, nil
}

// reconcileBoard works one board from pos. It returns where to resume (nil: the
// board is done) and, on an engineering 429, how long to wait.
func (s *productIssueSync) reconcileBoard(ctx context.Context, run *reconcileRun, b repository.ReconcileBoard, pos reconcileCursor) (*reconcileCursor, time.Duration, error) {
	resume := func(phase, after string, offset int, lastID string) *reconcileCursor {
		c := pos
		c.Phase, c.After, c.Offset, c.LastID = phase, after, offset, lastID
		return &c
	}
	if pos.Phase != phaseMissing {
		after, offset := pos.After, pos.Offset
	pages:
		for {
			if run.stop(ctx) {
				return resume(phaseItems, after, offset, ""), 0, nil
			}
			page, err := s.eng.ListProjectItemsWithContent(ctx, b.Owner, b.ProjectNumber, after, reconcilePageSize)
			if err != nil {
				if wait, ok := rateLimited(err); ok {
					return resume(phaseItems, after, offset, ""), wait, nil
				}
				if ctx.Err() != nil {
					return resume(phaseItems, after, offset, ""), 0, nil
				}
				slog.WarnContext(ctx, "product issue reconcile: listing a board failed; the board is skipped this run",
					"owner", b.Owner, "board", b.ProjectNumber, "err", err)
				run.resp.Failed++
				return nil, 0, nil
			}
			for i := offset; i < len(page.Items); i++ {
				if run.stop(ctx) {
					return resume(phaseItems, after, i, ""), 0, nil
				}
				run.work++
				wait, err := s.reconcileItem(ctx, run, b, page.Items[i])
				if err != nil {
					return nil, 0, err
				}
				if wait > 0 {
					return resume(phaseItems, after, i, ""), wait, nil
				}
			}
			offset = 0
			if page.NextCursor == nil || *page.NextCursor == "" || len(page.Items) == 0 {
				break pages
			}
			after = *page.NextCursor
		}
	}

	lastID := pos.LastID
	if pos.Phase != phaseMissing {
		lastID = ""
	}
	for {
		if run.stop(ctx) {
			return resume(phaseMissing, "", 0, lastID), 0, nil
		}
		links, err := s.repo.ListLinksNotSeen(ctx, b.Owner, b.ProjectNumber, pos.Started, lastID, reconcileMissingBatch)
		if err != nil {
			return nil, 0, err
		}
		if len(links) == 0 {
			return nil, 0, nil
		}
		for _, l := range links {
			if run.stop(ctx) {
				return resume(phaseMissing, "", 0, lastID), 0, nil
			}
			run.work++
			wait, err := s.refreshMissingLink(ctx, run, l)
			if err != nil {
				return nil, 0, err
			}
			if wait > 0 {
				return resume(phaseMissing, "", 0, lastID), wait, nil
			}
			lastID = l.WorkItemID
		}
	}
}

// reconcileItem compares one board item with the link it belongs to.
func (s *productIssueSync) reconcileItem(ctx context.Context, run *reconcileRun, b repository.ReconcileBoard, item engineeringentity.ProjectItem) (time.Duration, error) {
	c := item.Content
	if c == nil || item.Type != "ISSUE" || c.IsPullRequest {
		return 0, nil
	}
	link, err := s.repo.LinkByNodeID(ctx, c.NodeID)
	if err != nil {
		return 0, err
	}
	if link == nil {
		if link, err = s.repo.LinkByIssue(ctx, c.RepositoryOwner(), c.RepositoryName(), c.Number); err != nil {
			return 0, err
		}
	}
	if link == nil || link.LinkState != repository.LinkStateActive || !strings.EqualFold(link.Owner, b.Owner) {
		return 0, nil
	}
	tracked, err := s.repo.IsTrackingBoard(ctx, link.Owner, link.Repository, b.ProjectNumber)
	if err != nil {
		return 0, err
	}
	if !tracked {
		return 0, nil
	}
	run.resp.ItemsChecked++

	snap := issueSnapshotFromContent(link.WorkItemID, *c)
	params := repository.ReconcileParams{WorkItemID: link.WorkItemID, Issue: &snap}

	status, hasStatus := item.Status()
	status = strings.TrimSpace(status)
	// A link that sits on another board of a shared repository keeps that board's status.
	onThisBoard := link.BoardProjectNumber == nil || *link.BoardProjectNumber == b.ProjectNumber
	if hasStatus && status != "" && onThisBoard && boardStatusDiffers(link, status, b.ProjectNumber) {
		history, err := s.eng.GetProjectStatusHistory(ctx, link.Owner, link.Owner, link.Repository, link.IssueNumber)
		switch {
		case err == nil:
			sync := boardSyncFromHistory(history, []int{b.ProjectNumber}, repository.StatusSourceReconcile)
			if sync == nil {
				sync = &repository.BoardSync{ProjectNumber: b.ProjectNumber, Source: repository.StatusSourceReconcile}
			}
			if sync.CurrentRaw == "" {
				sync.CurrentRaw = status
			}
			sync.Mode, sync.Synthetic, sync.Now = repository.BoardCurrentIfDifferent, true, s.now().UTC()
			params.Board = sync
		default:
			if wait, ok := rateLimited(err); ok {
				return wait, nil
			}
			if ctx.Err() != nil {
				return 0, nil
			}
			slog.WarnContext(ctx, "product issue reconcile: reading the board history failed; the status is left for the next run",
				"workItemId", link.WorkItemID, "err", err)
			run.resp.Failed++
		}
	}
	res, err := s.repo.ReconcileLink(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "product issue reconcile: updating a link failed", "workItemId", link.WorkItemID, "err", err)
		run.resp.Failed++
		return 0, nil
	}
	run.resp.StatusEventsWritten += res.EventsWritten
	if res.Changed {
		run.resp.LinksUpdated++
	}
	return 0, nil
}

// refreshMissingLink refreshes a linked issue the board listing did not show
// (archived off the board, or never on it) from the issue itself.
func (s *productIssueSync) refreshMissingLink(ctx context.Context, run *reconcileRun, l repository.LinkedIssue) (time.Duration, error) {
	detail, err := s.eng.GetIssueDetail(ctx, l.Owner, l.Owner, l.Repository, l.IssueNumber)
	if err != nil {
		if wait, ok := rateLimited(err); ok {
			return wait, nil
		}
		if isNotFound(err) {
			run.resp.NotOnBoard++
			if err := s.repo.SetLinkState(ctx, l.WorkItemID, repository.LinkStateInaccessible); err != nil {
				slog.ErrorContext(ctx, "product issue reconcile: marking a link inaccessible failed", "workItemId", l.WorkItemID, "err", err)
				run.resp.Failed++
				return 0, nil
			}
			run.resp.LinksUpdated++
			return 0, nil
		}
		if ctx.Err() != nil {
			return 0, nil
		}
		slog.WarnContext(ctx, "product issue reconcile: reading an issue missing from the board failed", "workItemId", l.WorkItemID, "err", err)
		run.resp.Failed++
		return 0, nil
	}
	run.resp.NotOnBoard++
	snap := issueSnapshotFromDetail(l.WorkItemID, detail)
	res, err := s.repo.ReconcileLink(ctx, repository.ReconcileParams{WorkItemID: l.WorkItemID, Issue: &snap})
	if err != nil {
		slog.ErrorContext(ctx, "product issue reconcile: updating a link failed", "workItemId", l.WorkItemID, "err", err)
		run.resp.Failed++
		return 0, nil
	}
	if res.Changed {
		run.resp.LinksUpdated++
	}
	return 0, nil
}

func boardStatusDiffers(l *repository.LinkedIssue, status string, board int) bool {
	return l.BoardStatusRaw == nil || !strings.EqualFold(strings.TrimSpace(*l.BoardStatusRaw), status) ||
		l.BoardProjectNumber == nil || *l.BoardProjectNumber != board
}

// issueSnapshotFromContent reads a board item's issue the way a webhook's issue
// payload is read: state upper case, reason lower case (GitHub's REST spelling).
func issueSnapshotFromContent(workItemID string, c engineeringentity.ProjectItemContent) repository.IssueSnapshot {
	reason := ""
	if c.StateReason != nil {
		reason = strings.ToLower(*c.StateReason)
	}
	return newIssueSnapshot(workItemID, c.Title, c.State, reason, c.Labels, c.UpdatedAt, c.ClosedAt)
}

func issueSnapshotFromDetail(workItemID string, d engineeringentity.IssueDetail) repository.IssueSnapshot {
	reason := ""
	if d.StateReason != nil {
		reason = strings.ToLower(*d.StateReason)
	}
	return newIssueSnapshot(workItemID, d.Title, d.State, reason, d.Labels, d.UpdatedAt, d.ClosedAt)
}

func newIssueSnapshot(workItemID, title, state, reason string, labels []string, updatedAt string, closedAt *string) repository.IssueSnapshot {
	return repository.IssueSnapshot{
		WorkItemID: workItemID, Title: truncateRunes(strings.TrimSpace(title), maxIssueSubject),
		State: strings.ToUpper(state), StateReason: reason, Labels: labels,
		Priority: issuePriorityFromLabels(labels), Category: issueCategoryFromLabels(labels),
		UpdatedOn: parseRFC3339(updatedAt), ClosedOn: parseRFC3339Ptr(closedAt),
	}
}

// boardSyncFromHistory reads an issue's history on one of the boards: the entry
// for a board in boards (an unarchived one first, then the lowest number). nil
// when the issue is on none of them.
func boardSyncFromHistory(history []engineeringentity.ProjectStatusHistory, boards []int, source string) *repository.BoardSync {
	var best *engineeringentity.ProjectStatusHistory
	for i := range history {
		h := &history[i]
		on := false
		for _, n := range boards {
			if h.ProjectNumber == n {
				on = true
			}
		}
		if !on {
			continue
		}
		if best == nil || (best.IsArchived && !h.IsArchived) || (best.IsArchived == h.IsArchived && h.ProjectNumber < best.ProjectNumber) {
			best = h
		}
	}
	if best == nil {
		return nil
	}
	out := &repository.BoardSync{ProjectNumber: best.ProjectNumber, Source: source}
	if best.CurrentStatus != nil {
		out.CurrentRaw = strings.TrimSpace(*best.CurrentStatus)
	}
	for _, ch := range best.Changes {
		at := parseRFC3339(ch.OccurredAt)
		if at == nil || ch.NewStatus == nil {
			continue
		}
		c := repository.BoardHistoryChange{EventKey: ch.EventNodeID, ToRaw: *ch.NewStatus, OccurredAt: *at}
		if ch.PreviousStatus != nil {
			c.FromRaw = *ch.PreviousStatus
		}
		if ch.ActorLogin != nil {
			c.Actor = *ch.ActorLogin
		}
		out.Changes = append(out.Changes, c)
	}
	return out
}

// ---------------------------------------------------------------------------
// Backfill

// backfillCursor is where a backfill run stands: the phase (open issues, then
// closed ones), the 1-based page of the engineering entity's listing and the
// offset in it. Cutoff is the oldest close date that counts, fixed on the first call.
type backfillCursor struct {
	Phase  string    `json:"p"`
	Page   int       `json:"g"`
	Offset int       `json:"o,omitempty"`
	Cutoff time.Time `json:"c"`
}

type backfillRun struct {
	resp       *domain.ProductIssueBackfillResponse
	owner      string
	repository string
	boards     []int
	dry        bool
}

// Backfill implements ProductIssueSyncService.
func (s *productIssueSync) Backfill(ctx context.Context, req domain.ProductIssueBackfillRequest) (domain.ProductIssueBackfillResponse, error) {
	resp := domain.ProductIssueBackfillResponse{DryRun: req.DryRun, FailedIssueNumbers: []int{}}
	owner, repoName := strings.TrimSpace(req.Owner), strings.TrimSpace(req.Repository)
	if owner == "" || repoName == "" {
		return resp, &apierror.ValidationError{Msg: "owner and repository are required."}
	}
	maxIssues := req.MaxIssues
	switch {
	case maxIssues < 0:
		return resp, &apierror.ValidationError{Msg: "maxIssues must not be negative."}
	case maxIssues == 0:
		maxIssues = backfillDefaultMaxIssues
	case maxIssues > backfillMaxIssues:
		maxIssues = backfillMaxIssues
	}
	sinceDays := req.SinceDays
	switch {
	case sinceDays < 0:
		return resp, &apierror.ValidationError{Msg: "sinceDays must not be negative."}
	case sinceDays == 0:
		sinceDays = backfillDefaultSinceDays
	case sinceDays > backfillMaxSinceDays:
		sinceDays = backfillMaxSinceDays
	}
	mappedOwner, mappedRepo, ok, err := s.repo.MappedRepository(ctx, owner, repoName)
	if err != nil {
		return resp, err
	}
	if !ok {
		return resp, &apierror.ValidationError{Msg: msgLinkRepoNotMapped}
	}
	boards, err := s.repo.BoardsForRepository(ctx, mappedOwner, mappedRepo)
	if err != nil {
		return resp, err
	}
	var cur backfillCursor
	if req.Cursor != "" {
		if err := decodeSyncCursor(req.Cursor, &cur); err != nil {
			return resp, err
		}
		if (cur.Phase != phaseOpen && cur.Phase != phaseClosed) || cur.Page < 1 || cur.Offset < 0 {
			return resp, &apierror.ValidationError{Msg: "cursor is not valid."}
		}
	} else {
		cur = backfillCursor{Phase: phaseOpen, Page: 1, Cutoff: s.now().UTC().AddDate(0, 0, -sinceDays)}
	}

	run := &backfillRun{resp: &resp, owner: mappedOwner, repository: mappedRepo, boards: boards, dry: req.DryRun}
	budget := s.newBudget(ctx)
	considered := 0
	stopAt := func(phase string, page, offset int, wait time.Duration) {
		resp.NextCursor = encodeSyncCursor(backfillCursor{Phase: phase, Page: page, Offset: offset, Cutoff: cur.Cutoff})
		resp.More = true
		if wait > 0 {
			resp.RetryAfterSeconds = retrySeconds(wait)
		}
	}
	phase, page, offset := cur.Phase, cur.Page, cur.Offset
	for {
		if considered > 0 && (considered >= maxIssues || budget.low(ctx)) {
			stopAt(phase, page, offset, 0)
			return resp, nil
		}
		q := engineeringentity.ListIssueDetailsQuery{State: phase, Labels: []string{originLabel}, PerPage: backfillPageSize, Page: page}
		if phase == phaseClosed {
			q.Since = cur.Cutoff.Format(time.RFC3339)
		}
		issues, err := s.eng.ListIssueDetails(ctx, mappedOwner, mappedOwner, mappedRepo, q)
		if err != nil {
			if wait, ok := rateLimited(err); ok {
				stopAt(phase, page, offset, wait)
				return resp, nil
			}
			if ctx.Err() != nil {
				stopAt(phase, page, offset, 0)
				return resp, nil
			}
			slog.ErrorContext(ctx, "product issue backfill: listing issues failed", "owner", mappedOwner, "repo", mappedRepo, "phase", phase, "page", page, "err", err)
			return resp, upstreamFailure(err)
		}
		// Pull requests are left out of the listing, so a page may be short: only an
		// empty page ends the phase.
		if len(issues) == 0 {
			if phase == phaseOpen {
				phase, page, offset = phaseClosed, 1, 0
				continue
			}
			return resp, nil
		}
		for i := offset; i < len(issues); i++ {
			issue := issues[i]
			if !hasLabel(issue.Labels, originLabel) {
				continue
			}
			if phase == phaseClosed && closedBefore(issue, cur.Cutoff) {
				continue
			}
			if considered > 0 && (considered >= maxIssues || budget.low(ctx)) {
				stopAt(phase, page, i, 0)
				return resp, nil
			}
			considered++
			wait, err := s.backfillIssue(ctx, run, issue)
			if err != nil {
				return resp, err
			}
			if wait > 0 {
				stopAt(phase, page, i, wait)
				return resp, nil
			}
		}
		page, offset = page+1, 0
	}
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

// closedBefore reports whether a closed issue was closed before cutoff (its last
// update when GitHub gives no close date).
func closedBefore(i engineeringentity.IssueDetail, cutoff time.Time) bool {
	t := parseRFC3339Ptr(i.ClosedAt)
	if t == nil {
		t = parseRFC3339(i.UpdatedAt)
	}
	return t != nil && t.Before(cutoff)
}

// backfillIssue handles one issue. It returns a wait when the engineering entity
// said 429 (the issue is then retried by the next call), and an error only for a
// database failure that no issue-level handling covers.
func (s *productIssueSync) backfillIssue(ctx context.Context, run *backfillRun, issue engineeringentity.IssueDetail) (time.Duration, error) {
	resp := run.resp
	resp.Considered++
	if isPatchIssue(issue.Labels) {
		resp.SkippedPatch++
		return 0, nil
	}
	if strings.Contains(issue.HTMLURL, "/pull/") {
		resp.SkippedPullRequest++
		return 0, nil
	}
	link, err := s.repo.LinkByNodeID(ctx, issue.NodeID)
	if err != nil {
		return 0, err
	}
	if link == nil {
		if link, err = s.repo.LinkByIssue(ctx, run.owner, run.repository, issue.Number); err != nil {
			return 0, err
		}
	}
	if link != nil {
		resp.AlreadyKnown++
		if run.dry || link.LinkState != repository.LinkStateActive {
			return 0, nil
		}
		return s.fillKnownIssue(ctx, run, issue, link)
	}

	var footer ProductIssueFooterInfo
	if issue.Body != nil {
		footer = ParseProductIssueFooter(*issue.Body)
	}
	caseID, err := s.matchCase(ctx, footer)
	if err != nil {
		return 0, err
	}
	if run.dry {
		if caseID == "" {
			resp.WouldCreateUnmatched++
		} else {
			resp.WouldCreateMatched++
		}
		return 0, nil
	}

	comments, board, wait, ok := s.readIssueSide(ctx, run, issue)
	if wait > 0 {
		return wait, nil
	}
	if !ok {
		return 0, nil
	}
	title := truncateRunes(strings.TrimSpace(issue.Title), maxIssueSubject)
	if title == "" {
		title = fmt.Sprintf("%s/%s#%d", run.owner, run.repository, issue.Number)
	}
	state, reason := domain.ProductIssueStateOpen, ""
	if strings.EqualFold(issue.State, "closed") {
		state = domain.ProductIssueStateClosed
	}
	if issue.StateReason != nil {
		reason = strings.ToLower(*issue.StateReason)
	}
	htmlURL := issue.HTMLURL
	if htmlURL == "" {
		htmlURL = fmt.Sprintf("https://github.com/%s/%s/issues/%d", run.owner, run.repository, issue.Number)
	}
	res, err := s.repo.BackfillCreate(ctx, repository.BackfillCreateParams{
		CaseID: caseID, Unmatched: caseID == "", Actor: backfillActor, Title: title,
		Priority: issuePriorityFromLabels(issue.Labels), Category: issueCategoryFromLabels(issue.Labels),
		OpenedByEmail: footer.OpenedBy, ABTTeam: footer.ABTTeam, PublicIssueURL: footer.PublicIssueURL,
		Issue: repository.GitHubIssueRef{Owner: run.owner, Repository: run.repository, Number: issue.Number, NodeID: issue.NodeID, HTMLURL: htmlURL},
		State: state, StateReason: reason, Labels: issue.Labels,
		CreatedOn: parseRFC3339(issue.CreatedAt), UpdatedOn: parseRFC3339(issue.UpdatedAt), ClosedOn: parseRFC3339Ptr(issue.ClosedAt),
		Comments: comments, Board: board,
	})
	if err != nil {
		s.failIssue(ctx, run, issue.Number, "registering the issue failed", err)
		return 0, nil
	}
	if res.AlreadyLinked {
		resp.AlreadyKnown++
		return 0, nil
	}
	if caseID == "" {
		resp.CreatedUnmatched++
	} else {
		resp.CreatedMatched++
	}
	resp.CommentsWritten += res.CommentsInserted
	resp.StatusEventsWritten += res.EventsWritten
	return 0, nil
}

// fillKnownIssue adds the comments and board events a known issue lacks.
func (s *productIssueSync) fillKnownIssue(ctx context.Context, run *backfillRun, issue engineeringentity.IssueDetail, link *repository.LinkedIssue) (time.Duration, error) {
	if err := s.attachCaseIfUnmatched(ctx, run, issue, link.WorkItemID); err != nil {
		return 0, err
	}
	comments, board, wait, ok := s.readIssueSide(ctx, run, issue)
	if wait > 0 || !ok {
		return wait, nil
	}
	res, err := s.repo.BackfillFill(ctx, link.WorkItemID, comments, board)
	if err != nil {
		s.failIssue(ctx, run, issue.Number, "filling the issue's gaps failed", err)
		return 0, nil
	}
	run.resp.CommentsWritten += res.CommentsInserted
	run.resp.StatusEventsWritten += res.EventsWritten
	return 0, nil
}

// attachCaseIfUnmatched re-attempts the footer match for a known product issue
// that has no active AFFECTS link, with the rule a new issue is matched by, and
// links it to the case it names (source BACKFILL), copying the case's account and
// project onto the work item. A failed match leaves the issue as it is.
func (s *productIssueSync) attachCaseIfUnmatched(ctx context.Context, run *backfillRun, issue engineeringentity.IssueDetail, workItemID string) error {
	linked, err := s.repo.HasActiveCaseLink(ctx, workItemID)
	if err != nil || linked {
		return err
	}
	var footer ProductIssueFooterInfo
	if issue.Body != nil {
		footer = ParseProductIssueFooter(*issue.Body)
	}
	caseID, err := s.matchCase(ctx, footer)
	if err != nil || caseID == "" {
		return err
	}
	attached, err := s.repo.BackfillAttachCase(ctx, workItemID, caseID, backfillActor)
	if err != nil {
		s.failIssue(ctx, run, issue.Number, "linking the issue to its case failed", err)
		return nil
	}
	if attached {
		slog.InfoContext(ctx, "product issue backfill: linked a known issue to the case its footer names",
			"owner", run.owner, "repo", run.repository, "number", issue.Number)
	}
	return nil
}

// readIssueSide reads an issue's comments and its history on the product's
// tracking board. ok is false when a read failed (counted, the issue is retried by
// a later run); wait is set on a 429.
func (s *productIssueSync) readIssueSide(ctx context.Context, run *backfillRun, issue engineeringentity.IssueDetail) (comments []repository.GitHubCommentInput, board *repository.BoardSync, wait time.Duration, ok bool) {
	comments, err := s.readComments(ctx, run, issue.Number)
	if err != nil {
		if w, limited := rateLimited(err); limited {
			return nil, nil, w, false
		}
		s.failIssue(ctx, run, issue.Number, "reading the issue's comments failed", err)
		return nil, nil, 0, false
	}
	if len(run.boards) > 0 {
		history, err := s.eng.GetProjectStatusHistory(ctx, run.owner, run.owner, run.repository, issue.Number)
		if err != nil {
			if w, limited := rateLimited(err); limited {
				return nil, nil, w, false
			}
			s.failIssue(ctx, run, issue.Number, "reading the issue's board history failed", err)
			return nil, nil, 0, false
		}
		board = boardSyncFromHistory(history, run.boards, repository.StatusSourceBackfill)
	}
	return comments, board, 0, true
}

func (s *productIssueSync) failIssue(ctx context.Context, run *backfillRun, number int, what string, err error) {
	if ctx.Err() != nil {
		// The call itself ran out of time: not the issue's fault, and the next call retries it.
		return
	}
	slog.WarnContext(ctx, "product issue backfill: "+what, "owner", run.owner, "repo", run.repository, "number", number, "err", err)
	run.resp.Failed++
	if len(run.resp.FailedIssueNumbers) < maxFailedIssueNumbers {
		run.resp.FailedIssueNumbers = append(run.resp.FailedIssueNumbers, number)
	}
}

// readComments reads every comment of an issue, oldest first.
func (s *productIssueSync) readComments(ctx context.Context, run *backfillRun, number int) ([]repository.GitHubCommentInput, error) {
	var out []repository.GitHubCommentInput
	for page := 1; page <= backfillMaxCommentPages; page++ {
		batch, err := s.eng.ListIssueComments(ctx, run.owner, run.owner, run.repository, number,
			engineeringentity.ListCommentsQuery{PerPage: backfillCommentPageSize, Page: page})
		if err != nil {
			return nil, err
		}
		for _, c := range batch {
			if c.AuthorLogin != nil && s.bots[strings.ToLower(strings.TrimSpace(*c.AuthorLogin))] {
				continue // the portal's own comment, mirrored from the portal, not from GitHub
			}
			in := repository.GitHubCommentInput{GitHubCommentID: c.ID, Body: c.Body, UpdatedOn: parseRFC3339(c.UpdatedAt)}
			if c.AuthorLogin != nil {
				in.Author = *c.AuthorLogin
			}
			if t := parseRFC3339(c.CreatedAt); t != nil {
				in.CreatedOn = *t
			}
			out = append(out, in)
		}
		if len(batch) < backfillCommentPageSize {
			return out, nil
		}
	}
	return out, nil
}

// matchCase finds the case a footer names: "WSO2 case id" is a case's wso2_id and
// "Case number" its number; both lines are needed. See decideCaseMatch.
func (s *productIssueSync) matchCase(ctx context.Context, f ProductIssueFooterInfo) (string, error) {
	byID, err := s.repo.CaseIDByWSO2ID(ctx, f.CaseID)
	if err != nil {
		return "", err
	}
	byNumber, err := s.repo.CaseIDByNumber(ctx, f.CaseNumber)
	if err != nil {
		return "", err
	}
	return decideCaseMatch(byID, byNumber), nil
}

// decideCaseMatch combines what the two footer lines resolved to ("" when a line
// is missing or names no single case): an issue matches only when both lines are
// present, each resolves to exactly one case, and both name the same case.
// Anything else is unmatched.
func decideCaseMatch(byID, byNumber string) string {
	if byID != "" && byID == byNumber {
		return byID
	}
	return ""
}
