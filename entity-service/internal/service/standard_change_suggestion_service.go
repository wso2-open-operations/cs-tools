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

type standardChangeSuggestionService struct {
	repo     repository.StandardChangeSuggestionRepository
	access   AccessService
	criteria domain.StandardChangeSuggestionCriteria
	now      func() time.Time
}

// NewStandardChangeSuggestionService constructs a StandardChangeSuggestionService
// judging by the criteria given (a zero field takes the default).
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

// effectiveCriteria applies a request's overrides, each held to its range (400).
func (s *standardChangeSuggestionService) effectiveCriteria(req domain.StandardChangeSuggestionsRequest) (domain.StandardChangeSuggestionCriteria, error) {
	c := s.criteria
	set := func(name string, v *int, lo, hi int, dst *int) error {
		if v == nil {
			return nil
		}
		if *v < lo || *v > hi {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s must be between %d and %d", name, lo, hi)}
		}
		*dst = *v
		return nil
	}
	for _, o := range []struct {
		name   string
		v      *int
		lo, hi int
		dst    *int
	}{
		{"windowMonths", req.WindowMonths, 1, 60, &c.WindowMonths},
		{"minClosed", req.MinClosed, 1, 1000, &c.MinClosed},
		{"minSuccessPercent", req.MinSuccessPercent, 0, 100, &c.MinSuccessPercent},
		{"planSimilarityPercent", req.PlanSimilarityPercent, 1, 100, &c.PlanSimilarityPercent},
	} {
		if err := set(o.name, o.v, o.lo, o.hi, o.dst); err != nil {
			return c, err
		}
	}
	return c, nil
}

// GetSuggestions implements StandardChangeSuggestionService.
func (s *standardChangeSuggestionService) GetSuggestions(ctx context.Context, req domain.StandardChangeSuggestionsRequest) (domain.StandardChangeSuggestionsResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeSuggestionForbidden); err != nil {
		return domain.StandardChangeSuggestionsResponse{}, err
	}
	account := strings.ToLower(strings.TrimSpace(req.AccountID))
	if account != "" && !validate.IsUUID(account) {
		return domain.StandardChangeSuggestionsResponse{}, &apierror.ValidationError{Msg: "accountId must be a UUID"}
	}
	crit, err := s.effectiveCriteria(req)
	if err != nil {
		return domain.StandardChangeSuggestionsResponse{}, err
	}
	resp := domain.StandardChangeSuggestionsResponse{Suggestions: []domain.StandardChangeSuggestion{}, Criteria: crit}

	cands, err := s.repo.Candidates(ctx, account, suggestionWindowStart(s.now(), crit.WindowMonths))
	if err != nil {
		return resp, err
	}
	resp.Scanned.ChangeRequests = len(cands)
	byAccount := map[string][]repository.SuggestionCandidate{}
	for _, c := range cands {
		byAccount[c.AccountID] = append(byAccount[c.AccountID], c)
	}
	type accountClusters struct {
		account  string
		clusters []suggestionCluster
	}
	var all []accountClusters
	var memberIDs, accountIDs []string
	for a, cs := range byAccount {
		clusters := clusterSuggestionCandidates(cs, float64(crit.PlanSimilarityPercent)/100)
		if len(clusters) == 0 {
			continue
		}
		all = append(all, accountClusters{account: a, clusters: clusters})
		accountIDs = append(accountIDs, a)
		for _, cl := range clusters {
			for _, n := range cl.nodes {
				memberIDs = append(memberIDs, n.c.ID)
			}
		}
	}
	if len(all) == 0 {
		return resp, nil
	}
	incidents, err := s.repo.IncidentsOf(ctx, memberIDs)
	if err != nil {
		return resp, err
	}
	proposals, err := s.repo.OpenProposalsOf(ctx, memberIDs)
	if err != nil {
		return resp, err
	}
	templates, err := s.repo.LiveTemplates(ctx, accountIDs)
	if err != nil {
		return resp, err
	}
	templatesOf := map[string][]repository.SuggestionTemplate{}
	for _, t := range templates {
		templatesOf[t.AccountID] = append(templatesOf[t.AccountID], t)
	}
	dismissals, err := s.repo.Dismissals(ctx, accountIDs)
	if err != nil {
		return resp, err
	}

	for _, ac := range all {
		for _, cl := range ac.clusters {
			resp.Scanned.Clusters++
			sg := judgeSuggestionCluster(cl, crit, incidents, proposals)
			sg.Match, sg.Template = matchSuggestionTemplate(cl, templatesOf[ac.account], crit)
			if sg.Eligible {
				resp.Scanned.Eligible++
			}
			if d, ok := dismissals[ac.account][cl.key]; ok {
				resp.Scanned.Dismissed++
				if !req.IncludeDismissed {
					continue
				}
				dd := d
				sg.Dismissal = &dd
			}
			if !sg.Eligible && !req.IncludeIneligible {
				continue
			}
			resp.Suggestions = append(resp.Suggestions, sg)
		}
	}
	// Eligible first, then the most often closed, then the most recent.
	sort.SliceStable(resp.Suggestions, func(a, b int) bool {
		x, y := resp.Suggestions[a], resp.Suggestions[b]
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
	return resp, nil
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
