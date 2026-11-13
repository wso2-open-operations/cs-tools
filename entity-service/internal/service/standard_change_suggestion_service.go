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
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// DefaultStandardChangeSuggestionCriteria are the starting thresholds, to be
// measured against production data and set through configuration
// (STANDARD_CHANGE_SUGGESTION_*, read by config.Load with these same defaults).
var DefaultStandardChangeSuggestionCriteria = domain.StandardChangeSuggestionCriteria{
	WindowMonths:          12,
	MinClosed:             5,
	MinSuccessPercent:     90,
	PlanSimilarityPercent: 50,
}

const maxSuggestionDismissalReason = 1000

// Refresh paging: how many customers one call computes unless it asks for
// fewer, and the most it may ask for.
const (
	defaultSuggestionRefreshLimit = 50
	maxSuggestionRefreshLimit     = 500
)

// Refresh timing. Everything one call does runs under suggestionPageTimeout
// (45s), so a call answers 15s inside both the 60s REQUEST_TIMEOUT and the
// scheduled task's 60s per-call timeout, whatever its customers and writes do.
// Inside it, each customer's compute has suggestionCustomerTimeout (20s) and
// each write (its save, or its skip) suggestionWriteTimeout (5s); a customer is
// started only while the page has time for its compute and two writes (a save
// that fails, then the skip that records it: 30s), so a customer that starts
// always gets its whole deadline and the page's own deadline is never what
// stops it. The publish (with the error check before it) gets
// suggestionPublishTimeout (10s); a page that finishes its customers with less
// than that left hands the publish to the next call. Worst case: the last
// customer starts at 15s, its compute ends by 35s, its save by 40s, its skip
// by 45s.
var (
	suggestionPageTimeout     = 45 * time.Second
	suggestionCustomerTimeout = 20 * time.Second
	suggestionWriteTimeout    = 5 * time.Second
	suggestionPublishTimeout  = 10 * time.Second
)

// The reasons a skipped customer is recorded under (skipped_reason, returned as
// skippedCustomers[].reason; the webapp matches these exact strings, so change
// them together).
const (
	// A customer that overran its own deadline.
	suggestionSkippedTooLarge = "too large to compute within the refresh's per-customer deadline"
	// A customer whose compute or save failed for any other reason (a bad row,
	// a decode failure, a failed write, a panic); the cause is logged at ERROR.
	suggestionSkippedError = "an error stopped its computation (the refresh logged the cause)"
)

// suggestionMaxErrorShare: a run in which more than this share of its customers
// were skipped for an error is not published. One customer's error is that
// customer's problem and is skipped like an overrun; most customers failing is
// a fault of the system (a query, the schema, the database) and publishing
// would replace every customer's suggestions with "not computed", so the run
// fails instead and readers keep the previous snapshot. Overruns do not count:
// they are bounded by each customer's own size.
const suggestionMaxErrorShare = 0.5

type standardChangeSuggestionService struct {
	repo     repository.StandardChangeSuggestionRepository
	access   AccessService
	criteria domain.StandardChangeSuggestionCriteria
	now      func() time.Time
}

// NewStandardChangeSuggestionService constructs a StandardChangeSuggestionService
// that computes by the criteria given (a zero field takes the default).
func NewStandardChangeSuggestionService(repo repository.StandardChangeSuggestionRepository, access AccessService, criteria domain.StandardChangeSuggestionCriteria) StandardChangeSuggestionService {
	d := DefaultStandardChangeSuggestionCriteria
	pick := func(v, def int) int {
		if v <= 0 {
			return def
		}
		return v
	}
	criteria = domain.StandardChangeSuggestionCriteria{
		WindowMonths:          pick(criteria.WindowMonths, d.WindowMonths),
		MinClosed:             pick(criteria.MinClosed, d.MinClosed),
		MinSuccessPercent:     min(pick(criteria.MinSuccessPercent, d.MinSuccessPercent), 100),
		PlanSimilarityPercent: min(pick(criteria.PlanSimilarityPercent, d.PlanSimilarityPercent), 100),
	}
	return &standardChangeSuggestionService{repo: repo, access: access, criteria: criteria, now: time.Now}
}

const standardChangeSuggestionForbidden = "standard change suggestions are available to WSO2 staff only"

// GetSuggestions implements StandardChangeSuggestionService: one page of the
// last published snapshot, filtered as asked. It never computes. The open
// proposals and the dismissals are read live, so a suggestion proposed or
// dismissed since the snapshot says so at once. The list is sorted and paged in
// the query (repository.SuggestionPageQuery), across customers when none is
// asked for: one suggestion can hold 100 members and 40 CAB entries (about
// 45 KB, up to about 140 KB), so a snapshot of every customer is never sent whole.
func (s *standardChangeSuggestionService) GetSuggestions(ctx context.Context, req domain.StandardChangeSuggestionsRequest) (domain.StandardChangeSuggestionsResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeSuggestionForbidden); err != nil {
		return domain.StandardChangeSuggestionsResponse{}, err
	}
	account := strings.ToLower(strings.TrimSpace(req.AccountID))
	if account != "" && !validate.IsUUID(account) {
		return domain.StandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "accountId must be a UUID"}
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.StandardChangeSuggestionsResponse{}, err
	}
	// The run, its counts and its rows are separate reads: a refresh publishing
	// between them (which deletes the run read first) would answer one run's
	// computedOn and totals with no rows. So the latest run is looked up again
	// after the rows, and when it moved the whole read is done once more.
	var resp domain.StandardChangeSuggestionsResponse
	for attempt := 0; ; attempt++ {
		var runID string
		var err error
		resp, runID, err = s.readSuggestions(ctx, account, req)
		if err != nil || runID == "" || attempt == 1 {
			return resp, err
		}
		latest, err := s.repo.LatestRun(ctx)
		if err != nil {
			return resp, err
		}
		if latest != nil && latest.ID == runID {
			return resp, nil
		}
		slog.InfoContext(ctx, "standard change suggestions: a refresh published during the read; reading again", "runId", runID)
	}
}

// readSuggestions is one read of GetSuggestions (see there), with the run it read.
func (s *standardChangeSuggestionService) readSuggestions(ctx context.Context, account string, req domain.StandardChangeSuggestionsRequest) (domain.StandardChangeSuggestionsResponse, string, error) {
	resp := domain.StandardChangeSuggestionsResponse{
		Suggestions: []domain.StandardChangeSuggestion{}, Criteria: s.criteria, SkippedCustomers: []domain.StandardChangeSuggestionSkipped{},
		Limit: req.Pagination.Limit, Offset: req.Pagination.Offset,
	}
	run, err := s.repo.LatestRun(ctx)
	if err != nil || run == nil {
		return resp, "", err // nothing computed yet: an empty list, no computedOn
	}
	resp.Criteria, resp.ComputedOn = run.Criteria, run.CompletedOn
	resp.Scanned = run.Counts
	if account != "" {
		// The run's totals are every customer's: one customer's are its own.
		if resp.Scanned, err = s.repo.CustomerCounts(ctx, run.ID, account); err != nil {
			return resp, "", err
		}
	}
	if resp.SkippedCustomers, err = s.repo.SkippedCustomers(ctx, run.ID, account); err != nil {
		return resp, "", err
	}

	page, err := s.repo.SuggestionPage(ctx, run.ID, repository.SuggestionPageQuery{
		AccountID:         account,
		IncludeIneligible: req.IncludeIneligible,
		IncludeDismissed:  req.IncludeDismissed,
		Limit:             req.Pagination.Limit,
		Offset:            req.Pagination.Offset,
	})
	if err != nil {
		return resp, "", err
	}
	resp.Total, resp.Scanned.Dismissed = page.Total, page.Dismissed
	var memberIDs []string
	for _, row := range page.Rows {
		memberIDs = append(memberIDs, row.MemberIDs...)
	}
	proposals, err := s.repo.OpenProposalsOf(ctx, memberIDs)
	if err != nil {
		return resp, "", err
	}

	for _, row := range page.Rows {
		sg := row.Suggestion
		sg.Dismissal, sg.OpenProposals = row.Dismissal, []domain.EntityRef{}
		// A snapshot computed before the CAB's pushback was recorded has none.
		if sg.CABRejections == nil {
			sg.CABRejections = []domain.StandardChangeCABRejection{}
		}
		if sg.CABComments == nil {
			sg.CABComments = []domain.StandardChangeCABComment{}
		}
		// One computed before DeviationCount was kept decodes it as 0 (the
		// field has no "absent"): count the listed members that deviated, which
		// is every one that did unless the cluster lists only its newest 100.
		// A count the refresh kept is never below the listed ones.
		if listed := listedDeviations(sg.Members); sg.DeviationCount < listed {
			sg.DeviationCount = listed
		}
		seen := map[string]bool{}
		for _, id := range row.MemberIDs {
			for _, p := range proposals[id] {
				if !seen[p.ID] {
					seen[p.ID] = true
					sg.OpenProposals = append(sg.OpenProposals, p)
				}
			}
		}
		resp.Suggestions = append(resp.Suggestions, sg)
	}
	return resp, run.ID, nil
}

// listedDeviations counts the listed members that deviated from their plan.
func listedDeviations(members []domain.StandardChangeSuggestionMember) int {
	n := 0
	for _, m := range members {
		if m.HasDeviation {
			n++
		}
	}
	return n
}

// Refresh implements StandardChangeSuggestionService: one page of a refresh run
// (see RefreshStandardChangeSuggestionsRequest). Customers are computed one at
// a time, so a page never holds more than one customer's change requests and
// plans; the last page publishes the run. A customer that overruns its deadline
// or fails is recorded as skipped and the page carries on, so one customer
// cannot keep every other one's snapshot from being published; see
// suggestionPageTimeout for the timing and suggestionMaxErrorShare for when
// errors fail the run instead.
func (s *standardChangeSuggestionService) Refresh(ctx context.Context, req domain.RefreshStandardChangeSuggestionsRequest) (domain.RefreshStandardChangeSuggestionsResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeSuggestionForbidden); err != nil {
		return domain.RefreshStandardChangeSuggestionsResponse{}, err
	}
	limit := req.Limit
	switch {
	case limit == 0:
		limit = defaultSuggestionRefreshLimit
	case limit < 0 || limit > maxSuggestionRefreshLimit:
		return domain.RefreshStandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("limit must be between 1 and %d", maxSuggestionRefreshLimit)}
	}
	runID := strings.ToLower(strings.TrimSpace(req.RunID))
	cursor := strings.ToLower(strings.TrimSpace(req.Cursor))
	if cursor != "" && (runID == "" || !validate.IsUUID(cursor)) {
		return domain.RefreshStandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "cursor continues a run: send it with that run's runId, as the previous call returned it"}
	}
	if runID != "" && !validate.IsUUID(runID) {
		return domain.RefreshStandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "runId must be a UUID"}
	}

	// Everything from here, the run's start included, runs under the page's
	// own deadline (or the request's, when that is sooner).
	pageStart := s.now()
	pageCtx, cancelPage := context.WithTimeout(ctx, suggestionPageTimeout)
	defer cancelPage()
	pageTime := suggestionPageTimeout
	if d, ok := ctx.Deadline(); ok && time.Until(d) < pageTime {
		pageTime = time.Until(d)
	}
	pageDeadline := pageStart.Add(pageTime)
	remaining := func() time.Duration { return pageDeadline.Sub(s.now()) }

	var run repository.SuggestionRun
	if runID == "" {
		crit := s.criteria
		started, err := s.repo.StartRun(pageCtx, crit, func(startedOn time.Time) time.Time {
			return suggestionWindowStart(startedOn, crit.WindowMonths)
		})
		if err != nil {
			return domain.RefreshStandardChangeSuggestionsResponse{}, err
		}
		run = started
		slog.InfoContext(ctx, "standard change suggestions: refresh started", "runId", run.ID, "criteria", run.Criteria)
	} else {
		got, err := s.repo.GetRun(pageCtx, runID)
		if err != nil {
			return domain.RefreshStandardChangeSuggestionsResponse{}, err
		}
		if got == nil || got.CompletedOn != nil {
			return domain.RefreshStandardChangeSuggestionsResponse{}, &apierror.ConflictError{Msg: "this refresh run is no longer open (published, or replaced by another): start a new one"}
		}
		run = *got
	}

	// The window and the criteria are the run's own, so every page of one run
	// judges alike.
	crit := run.Criteria
	since := suggestionWindowStart(run.StartedOn, crit.WindowMonths)
	// The customers were listed once, when the run started.
	accounts, err := s.repo.NextCustomers(pageCtx, run.ID, cursor, limit+1)
	if err != nil {
		return domain.RefreshStandardChangeSuggestionsResponse{}, err
	}
	resp := domain.RefreshStandardChangeSuggestionsResponse{RunID: run.ID, NextCursor: cursor}
	// The previous snapshot, which each customer's clusters take their keys from.
	previous, err := s.repo.LatestRun(pageCtx)
	if err != nil {
		return resp, err
	}
	todo := accounts[:min(limit, len(accounts))]
	perCustomer := suggestionCustomerTimeout + 2*suggestionWriteTimeout
	done := 0
	for _, account := range todo {
		if remaining() < perCustomer {
			break
		}
		if err := pageCtx.Err(); err != nil {
			return resp, err
		}
		if err := s.refreshCustomer(pageCtx, run.ID, account, since, crit, previous, &resp); err != nil {
			return resp, err
		}
		done++
		resp.NextCursor = account
	}
	resp.More = done < len(accounts)
	if done == 0 && resp.More {
		// Not even one customer fits (a slow start of the run, or a request
		// deadline shorter than one customer needs). The next call starts
		// afresh from the same cursor; on a continuation page the caller sees
		// no progress and fails the run.
		slog.WarnContext(ctx, "standard change suggestions: no time left on this page for a customer",
			"runId", run.ID, "cursor", cursor, "remainingMs", remaining().Milliseconds(), "neededMs", perCustomer.Milliseconds())
	}
	if !resp.More && done > 0 && remaining() < suggestionPublishTimeout {
		// Every customer is done but the publish might not fit: the next call
		// finds no customer left and publishes with a page of its own.
		resp.More = true
	}
	if !resp.More {
		resp.NextCursor = ""
	}
	slog.InfoContext(ctx, "standard change suggestions: refresh page", "runId", run.ID,
		"customers", resp.Customers, "skipped", resp.Skipped, "changeRequests", resp.ChangeRequests, "suggestions", resp.Suggestions,
		"more", resp.More, "pageMs", s.now().Sub(pageStart).Milliseconds())
	if resp.More {
		return resp, nil
	}

	publishCtx, cancelPublish := context.WithTimeout(pageCtx, suggestionPublishTimeout)
	defer cancelPublish()
	if err := s.checkRunErrors(publishCtx, run.ID); err != nil {
		return resp, err
	}
	published, err := s.repo.PublishRun(publishCtx, run.ID)
	if err != nil {
		return resp, err
	}
	resp.ComputedOn = published.CompletedOn
	totals := published.Counts
	resp.Totals = &totals
	slog.InfoContext(ctx, "standard change suggestions: refresh published", "runId", run.ID,
		"customers", totals.Customers, "skipped", totals.Skipped, "changeRequests", totals.ChangeRequests, "clusters", totals.Clusters,
		"eligible", totals.Eligible, "durationMs", published.CompletedOn.Sub(published.StartedOn).Milliseconds())
	return resp, nil
}

// refreshCustomer computes one customer and saves it, or records it as skipped:
// too large when it overran its own deadline, an error when its compute or its
// save failed otherwise (the cause logged at ERROR). It returns an error only
// for what is not that customer's: the page's own deadline or cancellation, a
// run no longer open (409), or a skip that cannot be recorded (the database).
func (s *standardChangeSuggestionService) refreshCustomer(pageCtx context.Context, runID, account string, since time.Time, crit domain.StandardChangeSuggestionCriteria, previous *repository.SuggestionRun, resp *domain.RefreshStandardChangeSuggestionsResponse) error {
	customerStart := s.now()
	customerCtx, cancel := context.WithTimeout(pageCtx, suggestionCustomerTimeout)
	rows, candidates, err := s.computeCustomerRecovering(customerCtx, account, since, crit, previous)
	// Its own deadline, not the page's (a query cut short reports it in its own words).
	overran := errors.Is(customerCtx.Err(), context.DeadlineExceeded) && pageCtx.Err() == nil
	cancel()
	if err != nil && pageCtx.Err() != nil {
		return fmt.Errorf("standard change suggestions: customer %s: %w", account, err)
	}
	reason := ""
	switch {
	case err != nil && overran:
		slog.WarnContext(pageCtx, "standard change suggestions: customer skipped, too large to compute in time",
			"runId", runID, "accountId", account, "candidates", candidates,
			"deadlineMs", suggestionCustomerTimeout.Milliseconds(), "elapsedMs", s.now().Sub(customerStart).Milliseconds())
		reason = suggestionSkippedTooLarge
	case err != nil:
		slog.ErrorContext(pageCtx, "standard change suggestions: customer skipped, its computation failed",
			"runId", runID, "accountId", account, "candidates", candidates, "error", err)
		reason = suggestionSkippedError
	default:
		writeCtx, cancelWrite := context.WithTimeout(pageCtx, suggestionWriteTimeout)
		err := s.repo.SaveCustomer(writeCtx, runID, account, candidates, rows)
		cancelWrite()
		var conflict *apierror.ConflictError
		switch {
		case err == nil:
			resp.Customers++
			resp.ChangeRequests += candidates
			resp.Suggestions += len(rows)
			return nil
		case errors.As(err, &conflict) || pageCtx.Err() != nil:
			return err
		}
		slog.ErrorContext(pageCtx, "standard change suggestions: customer skipped, saving its suggestions failed",
			"runId", runID, "accountId", account, "candidates", candidates, "suggestions", len(rows), "error", err)
		reason = suggestionSkippedError
	}
	writeCtx, cancelWrite := context.WithTimeout(pageCtx, suggestionWriteTimeout)
	defer cancelWrite()
	if err := s.repo.SkipCustomer(writeCtx, runID, account, reason); err != nil {
		return err
	}
	resp.Skipped++
	return nil
}

// computeCustomerRecovering is computeCustomer with a panic turned into that
// customer's error, so a row that trips a bug skips the customer rather than
// failing the page (and every page after it, at the same customer).
func (s *standardChangeSuggestionService) computeCustomerRecovering(ctx context.Context, account string, since time.Time, crit domain.StandardChangeSuggestionCriteria, previous *repository.SuggestionRun) (rows []repository.SuggestionSnapshotRow, candidates int, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "standard change suggestions: computing a customer panicked", "accountId", account, "panic", r, "stack", string(debug.Stack()))
			rows, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	return s.computeCustomer(ctx, account, since, crit, previous)
}

// checkRunErrors refuses to publish a run in which more than
// suggestionMaxErrorShare of its customers were skipped for an error.
func (s *standardChangeSuggestionService) checkRunErrors(ctx context.Context, runID string) error {
	skipped, err := s.repo.SkippedCustomers(ctx, runID, "")
	if err != nil {
		return err
	}
	errored := 0
	for _, sk := range skipped {
		if sk.Reason == suggestionSkippedError {
			errored++
		}
	}
	if errored == 0 {
		return nil
	}
	listed, err := s.repo.RunCustomerCount(ctx, runID)
	if err != nil {
		return err
	}
	if float64(errored) <= suggestionMaxErrorShare*float64(listed) {
		return nil
	}
	slog.ErrorContext(ctx, "standard change suggestions: most customers failed; the run is not published and readers keep the previous snapshot",
		"runId", runID, "errored", errored, "customers", listed)
	return fmt.Errorf("standard change suggestions: %d of the run's %d customers failed to compute (each logged with its cause), which points at a fault of the system rather than of one customer: the run is not published", errored, listed)
}

// computeCustomer computes one customer's suggestions: its candidates and their
// clusters, each judged and keyed from the customer's rows of the previous
// snapshot (nil: none). It also returns how many candidates there were (also
// with an error, once they were read). It stops with ctx's error when ctx ends.
func (s *standardChangeSuggestionService) computeCustomer(ctx context.Context, account string, since time.Time, crit domain.StandardChangeSuggestionCriteria, previous *repository.SuggestionRun) ([]repository.SuggestionSnapshotRow, int, error) {
	cands, err := s.repo.Candidates(ctx, account, since)
	if err != nil {
		return nil, 0, err
	}
	clusters, err := clusterSuggestionCandidates(ctx, cands, float64(crit.PlanSimilarityPercent)/100)
	if err != nil {
		return nil, len(cands), err
	}
	if len(clusters) == 0 {
		return nil, len(cands), nil
	}
	var before []repository.SuggestionSnapshotRow
	if previous != nil {
		if before, err = s.repo.SnapshotKeys(ctx, previous.ID, account); err != nil {
			return nil, len(cands), err
		}
	}
	assignSuggestionIdentities(clusters, before)
	var memberIDs []string
	for _, cl := range clusters {
		for _, n := range cl.nodes {
			memberIDs = append(memberIDs, n.c.ID)
		}
	}
	incidents, err := s.repo.IncidentsOf(ctx, memberIDs)
	if err != nil {
		return nil, len(cands), err
	}
	// The CAB's pushback: the members' own decisions, and the change requests
	// the CAB rejected that look like a cluster (attachCABLookalikes).
	cab, err := s.repo.CABDecisionsOf(ctx, memberIDs)
	if err != nil {
		return nil, len(cands), err
	}
	lookalikes, err := s.repo.CABRejectedLookalikes(ctx, account, since)
	if err != nil {
		return nil, len(cands), err
	}
	attached := attachCABLookalikes(clusters, lookalikes)
	rows := make([]repository.SuggestionSnapshotRow, 0, len(clusters))
	for i, cl := range clusters {
		sg := judgeSuggestionCluster(cl, crit, incidents, nil, cab, attached[i])
		ids := make([]string, len(cl.nodes))
		for i, n := range cl.nodes {
			ids[i] = n.c.ID
		}
		rows = append(rows, repository.SuggestionSnapshotRow{AccountID: account, Key: sg.Key, Eligible: sg.Eligible, MemberIDs: ids, Suggestion: sg})
	}
	return rows, len(cands), nil
}

// suggestionActor is an internal caller who is a person: a dismissal records who.
func (s *standardChangeSuggestionService) suggestionActor(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeSuggestionForbidden); err != nil {
		return "", err
	}
	email := strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail)
	if email == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	return email, nil
}

// DismissSuggestion implements StandardChangeSuggestionService.
func (s *standardChangeSuggestionService) DismissSuggestion(ctx context.Context, req domain.DismissStandardChangeSuggestionRequest) (domain.StandardChangeSuggestionDismissal, error) {
	actor, err := s.suggestionActor(ctx)
	if err != nil {
		return domain.StandardChangeSuggestionDismissal{}, err
	}
	account := strings.ToLower(strings.TrimSpace(req.AccountID))
	if !validate.IsUUID(account) {
		return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: "accountId is required and must be a UUID"}
	}
	key := strings.TrimSpace(req.Key)
	if key == "" || len(key) > suggestionMaxKey {
		return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: fmt.Sprintf("key is required (the suggestion's key, at most %d bytes)", suggestionMaxKey)}
	}
	// A key that is not a cluster key never matches a suggestion, so it is
	// refused rather than stored.
	if !validSuggestionKey(key) {
		return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: "key is not a suggestion key: send the key the suggestion carries"}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: "reason is required: say why this is not a standard change"}
	}
	if utf8.RuneCountInString(reason) > maxSuggestionDismissalReason {
		return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: fmt.Sprintf("reason must be at most %d characters", maxSuggestionDismissalReason)}
	}
	return s.repo.Dismiss(ctx, account, key, reason, actor)
}

// RestoreSuggestion implements StandardChangeSuggestionService.
func (s *standardChangeSuggestionService) RestoreSuggestion(ctx context.Context, id string) error {
	if _, err := s.suggestionActor(ctx); err != nil {
		return err
	}
	id = strings.ToLower(strings.TrimSpace(id))
	if !validate.IsUUID(id) {
		return &apierror.ValidationError{Msg: "id must be a UUID"}
	}
	return s.repo.Restore(ctx, id)
}
