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
	"fmt"
	"log/slog"
	"sort"
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
// (STANDARD_CHANGE_SUGGESTION_*).
var DefaultStandardChangeSuggestionCriteria = domain.StandardChangeSuggestionCriteria{
	WindowMonths:          12,
	MinClosed:             3,
	MinSuccessPercent:     90,
	PlanSimilarityPercent: 50,
	CoveredPercent:        80,
	ClosePercent:          50,
}

const maxSuggestionDismissalReason = 1000

// Refresh paging: how many customers one call computes unless it asks for
// fewer, the most it may ask for, and how long a call keeps starting customers
// before it hands back (it always finishes the customer it is on, and computes
// at least one). The budget leaves room inside the 60s REQUEST_TIMEOUT.
const (
	defaultSuggestionRefreshLimit = 50
	maxSuggestionRefreshLimit     = 500
)

var suggestionRefreshBudget = 40 * time.Second

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
		CoveredPercent:        min(pick(criteria.CoveredPercent, d.CoveredPercent), 100),
		ClosePercent:          min(pick(criteria.ClosePercent, d.ClosePercent), 100),
	}
	if criteria.ClosePercent > criteria.CoveredPercent {
		criteria.ClosePercent = criteria.CoveredPercent
	}
	return &standardChangeSuggestionService{repo: repo, access: access, criteria: criteria, now: time.Now}
}

const standardChangeSuggestionForbidden = "standard change suggestions are available to WSO2 staff only"

// GetSuggestions implements StandardChangeSuggestionService: the last published
// snapshot, filtered as asked. It never computes. The open proposals and the
// dismissals are read live, so a suggestion proposed or dismissed since the
// snapshot says so at once.
func (s *standardChangeSuggestionService) GetSuggestions(ctx context.Context, req domain.StandardChangeSuggestionsRequest) (domain.StandardChangeSuggestionsResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeSuggestionForbidden); err != nil {
		return domain.StandardChangeSuggestionsResponse{}, err
	}
	account := strings.ToLower(strings.TrimSpace(req.AccountID))
	if account != "" && !validate.IsUUID(account) {
		return domain.StandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "accountId must be a UUID"}
	}
	resp := domain.StandardChangeSuggestionsResponse{Suggestions: []domain.StandardChangeSuggestion{}, Criteria: s.criteria}
	run, err := s.repo.LatestRun(ctx)
	if err != nil || run == nil {
		return resp, err // nothing computed yet: an empty list, no computedOn
	}
	resp.Criteria, resp.ComputedOn = run.Criteria, run.CompletedOn
	resp.Scanned = run.Counts
	if account != "" {
		// The run's totals are every customer's: one customer's are its own.
		if resp.Scanned, err = s.repo.CustomerCounts(ctx, run.ID, account); err != nil {
			return resp, err
		}
	}

	rows, err := s.repo.SnapshotRows(ctx, run.ID, account, req.IncludeIneligible)
	if err != nil {
		return resp, err
	}
	var accounts []string // nil: every customer's dismissals
	if account != "" {
		accounts = []string{account}
	}
	dismissals, err := s.repo.Dismissals(ctx, accounts)
	if err != nil {
		return resp, err
	}
	var memberIDs []string
	for _, row := range rows {
		memberIDs = append(memberIDs, row.MemberIDs...)
	}
	proposals, err := s.repo.OpenProposalsOf(ctx, memberIDs)
	if err != nil {
		return resp, err
	}

	for _, row := range rows {
		sg := row.Suggestion
		sg.Dismissal, sg.OpenProposals = nil, []domain.EntityRef{}
		seen := map[string]bool{}
		for _, id := range row.MemberIDs {
			for _, p := range proposals[id] {
				if !seen[p.ID] {
					seen[p.ID] = true
					sg.OpenProposals = append(sg.OpenProposals, p)
				}
			}
		}
		if d, ok := dismissals[row.AccountID][row.Key]; ok {
			resp.Scanned.Dismissed++
			if !req.IncludeDismissed {
				continue
			}
			dd := d
			sg.Dismissal = &dd
		}
		resp.Suggestions = append(resp.Suggestions, sg)
	}
	sortSuggestions(resp.Suggestions)
	return resp, nil
}

// sortSuggestions: eligible first, then the most often closed, then the most recent.
func sortSuggestions(list []domain.StandardChangeSuggestion) {
	sort.SliceStable(list, func(a, b int) bool {
		x, y := list[a], list[b]
		if x.Eligible != y.Eligible {
			return x.Eligible
		}
		if x.ClosedCount != y.ClosedCount {
			return x.ClosedCount > y.ClosedCount
		}
		if !x.LastCompletedOn.Equal(y.LastCompletedOn) {
			return x.LastCompletedOn.After(y.LastCompletedOn)
		}
		if x.Customer.Name != y.Customer.Name {
			return x.Customer.Name < y.Customer.Name
		}
		return x.Key < y.Key
	})
}

// Refresh implements StandardChangeSuggestionService: one page of a refresh run
// (see RefreshStandardChangeSuggestionsRequest). Customers are computed one at
// a time, so a page never holds more than one customer's change requests and
// plans; the last page publishes the run.
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
	var run repository.SuggestionRun
	if runID == "" {
		started, err := s.repo.StartRun(ctx, s.criteria)
		if err != nil {
			return domain.RefreshStandardChangeSuggestionsResponse{}, err
		}
		run = started
		slog.InfoContext(ctx, "standard change suggestions: refresh started", "runId", run.ID, "criteria", run.Criteria)
	} else {
		if !validate.IsUUID(runID) {
			return domain.RefreshStandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "runId must be a UUID"}
		}
		got, err := s.repo.GetRun(ctx, runID)
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
	pageStart := s.now()
	accounts, err := s.repo.CandidateAccounts(ctx, since, cursor, limit+1)
	if err != nil {
		return domain.RefreshStandardChangeSuggestionsResponse{}, err
	}
	resp := domain.RefreshStandardChangeSuggestionsResponse{RunID: run.ID}
	todo := accounts[:min(limit, len(accounts))]
	for i, account := range todo {
		if i > 0 && s.now().Sub(pageStart) > suggestionRefreshBudget {
			break
		}
		if err := ctx.Err(); err != nil {
			return resp, err
		}
		rows, candidates, err := s.computeCustomer(ctx, account, since, crit)
		if err != nil {
			return resp, fmt.Errorf("standard change suggestions: customer %s: %w", account, err)
		}
		if err := s.repo.SaveCustomer(ctx, run.ID, account, candidates, rows); err != nil {
			return resp, err
		}
		resp.Customers++
		resp.ChangeRequests += candidates
		resp.Suggestions += len(rows)
		resp.NextCursor = account
	}
	resp.More = resp.Customers < len(accounts)
	if !resp.More {
		resp.NextCursor = ""
	}
	slog.InfoContext(ctx, "standard change suggestions: refresh page", "runId", run.ID,
		"customers", resp.Customers, "changeRequests", resp.ChangeRequests, "suggestions", resp.Suggestions,
		"more", resp.More, "pageMs", s.now().Sub(pageStart).Milliseconds())
	if resp.More {
		return resp, nil
	}

	published, err := s.repo.PublishRun(ctx, run.ID)
	if err != nil {
		return resp, err
	}
	resp.ComputedOn = published.CompletedOn
	totals := published.Counts
	resp.Totals = &totals
	slog.InfoContext(ctx, "standard change suggestions: refresh published", "runId", run.ID,
		"customers", totals.Customers, "changeRequests", totals.ChangeRequests, "clusters", totals.Clusters,
		"eligible", totals.Eligible, "durationMs", published.CompletedOn.Sub(published.StartedOn).Milliseconds())
	return resp, nil
}

// computeCustomer computes one customer's suggestions: its candidates, their
// clusters, each judged and related to the customer's live templates. It also
// returns how many candidates there were.
func (s *standardChangeSuggestionService) computeCustomer(ctx context.Context, account string, since time.Time, crit domain.StandardChangeSuggestionCriteria) ([]repository.SuggestionSnapshotRow, int, error) {
	cands, err := s.repo.Candidates(ctx, account, since)
	if err != nil {
		return nil, 0, err
	}
	clusters := clusterSuggestionCandidates(cands, float64(crit.PlanSimilarityPercent)/100)
	if len(clusters) == 0 {
		return nil, len(cands), nil
	}
	var memberIDs []string
	for _, cl := range clusters {
		for _, n := range cl.nodes {
			memberIDs = append(memberIDs, n.c.ID)
		}
	}
	incidents, err := s.repo.IncidentsOf(ctx, memberIDs)
	if err != nil {
		return nil, 0, err
	}
	templates, err := s.repo.LiveTemplates(ctx, []string{account})
	if err != nil {
		return nil, 0, err
	}
	rows := make([]repository.SuggestionSnapshotRow, 0, len(clusters))
	for _, cl := range clusters {
		sg := judgeSuggestionCluster(cl, crit, incidents, nil)
		sg.Match, sg.Template = matchSuggestionTemplate(cl, templates, crit)
		ids := make([]string, len(cl.nodes))
		for i, n := range cl.nodes {
			ids[i] = n.c.ID
		}
		rows = append(rows, repository.SuggestionSnapshotRow{AccountID: account, Key: cl.key, Eligible: sg.Eligible, MemberIDs: ids, Suggestion: sg})
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
	// The key is the normalised subject: a key that is not one never matches a
	// suggestion, so it is refused rather than stored.
	if suggestionKey(key) != key {
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
