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
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	msgProductIssueNotFound = "product issue not found"
	msgLinkRepoNotMapped    = "This repository is not mapped to a product."
	msgGitHubIssueNotFound  = "GitHub issue not found."
	msgLinkPatchRefused     = "Patch issues are not tracked as product issues."
	msgLinkPullRequest      = "A pull request cannot be linked to a case."
	msgLinkCaseClosed       = "A closed case cannot be linked to a product issue."
	msgLinkBeingCreated     = "This GitHub issue is being registered from a case right now. Please retry shortly."
	maxWorkItemSubjectChars = 255
	// productLinkDetailTimeout bounds the GitHub read made to register an issue.
	productLinkDetailTimeout = 30 * time.Second
)

// ProductIssueService reads product issues and links them to cases. Every method
// is for internal callers only.
type ProductIssueService interface {
	// ListForCase returns the product issues with an active AFFECTS link to the case.
	ListForCase(ctx context.Context, caseID string) (domain.ProductIssueListResponse, error)
	Get(ctx context.Context, id string) (domain.ProductIssueDetail, error)
	Search(ctx context.Context, req domain.SearchProductIssuesRequest) (domain.SearchProductIssuesResponse, error)
	// Link links a product issue to the case, registering it from GitHub first when
	// it is given by repository and issue number and is not known yet. created is
	// false when the issue already had this link.
	Link(ctx context.Context, req domain.LinkProductIssueRequest) (summary domain.ProductIssueSummary, created bool, err error)
	// Unlink deactivates the link; 404 when there is no active one.
	Unlink(ctx context.Context, caseID, workItemID string) error
}

// productIssueEngineering is the engineering entity call the link makes.
type productIssueEngineering interface {
	GetIssueDetail(ctx context.Context, org, owner, repo string, number int) (engineeringentity.IssueDetail, error)
}

type productIssueService struct {
	eng     productIssueEngineering
	repo    repository.ProductIssueRepository
	access  AccessService
	tagRule ProductIssueCaseTagRule // nil: s_dip is not maintained (WithProductIssueCaseTagRule)
}

// NewProductIssueService constructs a ProductIssueService.
func NewProductIssueService(eng productIssueEngineering, repo repository.ProductIssueRepository, access AccessService) ProductIssueService {
	return &productIssueService{eng: eng, repo: repo, access: access}
}

func (s *productIssueService) internalCaller(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, productWorkItemForbidden)
}

func (s *productIssueService) ListForCase(ctx context.Context, caseID string) (domain.ProductIssueListResponse, error) {
	if err := s.internalCaller(ctx); err != nil {
		return domain.ProductIssueListResponse{}, err
	}
	if !validate.IsUUID(caseID) {
		return domain.ProductIssueListResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", caseID)}
	}
	if _, err := s.repo.LoadCase(ctx, caseID); err != nil { // 404 unless case-like
		return domain.ProductIssueListResponse{}, err
	}
	items, err := s.repo.ListForCase(ctx, caseID)
	return domain.ProductIssueListResponse{Items: items}, err
}

func (s *productIssueService) Get(ctx context.Context, id string) (domain.ProductIssueDetail, error) {
	if err := s.internalCaller(ctx); err != nil {
		return domain.ProductIssueDetail{}, err
	}
	if !validate.IsUUID(id) {
		return domain.ProductIssueDetail{}, &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", id)}
	}
	return s.repo.GetDetail(ctx, id)
}

func (s *productIssueService) Search(ctx context.Context, req domain.SearchProductIssuesRequest) (domain.SearchProductIssuesResponse, error) {
	if err := s.internalCaller(ctx); err != nil {
		return domain.SearchProductIssuesResponse{}, err
	}
	f := &req.Filters
	if f.CaseID = strings.TrimSpace(f.CaseID); f.CaseID != "" && !validate.IsUUID(f.CaseID) {
		return domain.SearchProductIssuesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("caseId contains invalid UUID: %q", f.CaseID)}
	}
	if f.CaseID != "" && f.Unmatched {
		return domain.SearchProductIssuesResponse{}, &apierror.ValidationError{Msg: "caseId and unmatched cannot be combined."}
	}
	f.GitHubState = strings.ToUpper(strings.TrimSpace(f.GitHubState))
	if f.GitHubState != "" && f.GitHubState != domain.ProductIssueStateOpen && f.GitHubState != domain.ProductIssueStateClosed {
		return domain.SearchProductIssuesResponse{}, &apierror.ValidationError{Msg: "githubState must be OPEN or CLOSED."}
	}
	for i, b := range f.BoardStatuses {
		b = strings.TrimSpace(b)
		if !slices.Contains(domain.ProductIssueBoardStatuses, b) {
			return domain.SearchProductIssuesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("boardStatuses contains an unknown status: %q", b)}
		}
		f.BoardStatuses[i] = b
	}
	f.Owner, f.Repository = strings.TrimSpace(f.Owner), strings.TrimSpace(f.Repository)
	f.Query = strings.TrimSpace(f.Query)
	if err := validateSearchQuery(f.Query); err != nil {
		return domain.SearchProductIssuesResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProductIssuesResponse{}, err
	}
	items, total, err := s.repo.Search(ctx, *f, req.Pagination)
	if err != nil {
		return domain.SearchProductIssuesResponse{}, err
	}
	return domain.SearchProductIssuesResponse{Items: items, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

func (s *productIssueService) Unlink(ctx context.Context, caseID, workItemID string) error {
	if err := s.internalCaller(ctx); err != nil {
		return err
	}
	email := strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail)
	if email == "" {
		return &apierror.ValidationError{Msg: "A signed-in user is required to unlink a product issue."}
	}
	ok, err := s.repo.Unlink(ctx, caseID, workItemID, email)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.NotFoundError{Msg: "no active link between this product issue and the case"}
	}
	applyCaseTagRule(ctx, s.tagRule, caseID)
	return nil
}

// Link implements ProductIssueService.
func (s *productIssueService) Link(ctx context.Context, req domain.LinkProductIssueRequest) (domain.ProductIssueSummary, bool, error) {
	var zero domain.ProductIssueSummary
	if err := s.internalCaller(ctx); err != nil {
		return zero, false, err
	}
	email := strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail)
	if email == "" {
		return zero, false, &apierror.ValidationError{Msg: "A signed-in user is required to link a product issue."}
	}
	if !validate.IsUUID(req.CaseID) {
		return zero, false, &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", req.CaseID)}
	}
	req.WorkItemID = strings.TrimSpace(req.WorkItemID)
	req.Owner, req.Repository = strings.TrimSpace(req.Owner), strings.TrimSpace(req.Repository)
	byID := req.WorkItemID != ""
	byIssue := req.Owner != "" || req.Repository != "" || req.IssueNumber != 0
	switch {
	case byID == byIssue:
		return zero, false, &apierror.ValidationError{Msg: "Give either workItemId or owner, repository and issueNumber."}
	case byID && !validate.IsUUID(req.WorkItemID):
		return zero, false, &apierror.ValidationError{Msg: fmt.Sprintf("workItemId contains invalid UUID: %q", req.WorkItemID)}
	case byIssue && (req.Owner == "" || req.Repository == "" || req.IssueNumber < 1):
		return zero, false, &apierror.ValidationError{Msg: "owner, repository and a positive issueNumber are all required."}
	}

	kase, err := s.repo.LoadCase(ctx, req.CaseID)
	if err != nil {
		return zero, false, err
	}
	if kase.State == domain.CaseStateClosed {
		return zero, false, &apierror.ConflictError{Msg: msgLinkCaseClosed}
	}

	workItemID, registered := req.WorkItemID, false
	if byIssue {
		workItemID, err = s.repo.FindByGitHubIssue(ctx, req.Owner, req.Repository, req.IssueNumber)
		if err != nil {
			return zero, false, err
		}
		if workItemID == "" {
			workItemID, err = s.registerFromGitHub(ctx, req, kase, email)
			if err != nil {
				return zero, false, err
			}
			if workItemID != "" {
				registered = true
			} else if workItemID, err = s.repo.FindByGitHubIssue(ctx, req.Owner, req.Repository, req.IssueNumber); err != nil {
				// Another request registered the issue first: treat it as known.
				return zero, false, err
			}
		}
	}
	if registered {
		applyCaseTagRule(ctx, s.tagRule, req.CaseID)
		sum, err := s.repo.GetSummary(ctx, workItemID)
		return sum, true, err
	}
	if workItemID == "" {
		// The unique index refused the insert for a link the lookup cannot see
		// (the issue's node id is on another row).
		return zero, false, &apierror.ConflictError{Msg: "This GitHub issue is already linked to another work item."}
	}
	created, err := s.repo.LinkExisting(ctx, workItemID, req.CaseID, email)
	if err != nil {
		return zero, false, err
	}
	applyCaseTagRule(ctx, s.tagRule, req.CaseID)
	sum, err := s.repo.GetSummary(ctx, workItemID)
	return sum, created, err
}

// registerFromGitHub reads the issue and registers it as a product issue linked
// to the case (source MANUAL). It returns the new work item's id, or "" with a
// nil error when another request registered the issue first: the caller then
// re-reads it by its issue key.
func (s *productIssueService) registerFromGitHub(ctx context.Context, req domain.LinkProductIssueRequest, kase repository.ProductWorkItemCase, email string) (string, error) {
	owner, repoName, mapped, err := s.repo.MappedRepository(ctx, req.Owner, req.Repository)
	if err != nil {
		return "", err
	}
	if !mapped {
		return "", &apierror.ValidationError{Msg: msgLinkRepoNotMapped}
	}
	// A case's create request that has recorded this issue but not finished
	// registering it owns it: registering it here first would steal it.
	if inFlight, err := s.repo.IssueBeingCreated(ctx, owner, repoName, req.IssueNumber); err != nil {
		return "", err
	} else if inFlight {
		return "", &apierror.ConflictError{Msg: msgLinkBeingCreated, Code: apierror.CodeRequestInProgress}
	}
	detailCtx, cancel := context.WithTimeout(ctx, productLinkDetailTimeout)
	defer cancel()
	// As the create path does, the mapping's owner is both the organisation and the owner.
	detail, err := s.eng.GetIssueDetail(detailCtx, owner, owner, repoName, req.IssueNumber)
	if err != nil {
		var ee *engineeringentity.Error
		switch {
		case errors.As(err, &ee) && ee.StatusCode == http.StatusNotFound:
			return "", &apierror.NotFoundError{Msg: msgGitHubIssueNotFound}
		case errors.As(err, &ee) && ee.StatusCode == http.StatusTooManyRequests:
			return "", &UpstreamFailureError{Status: http.StatusServiceUnavailable, Msg: msgProductRateLimited, RetryAfter: ee.RetryAfter}
		}
		slog.ErrorContext(ctx, "product issue link: reading the GitHub issue failed", "repo", owner+"/"+repoName, "number", req.IssueNumber, "err", err)
		return "", upstreamFailure(err)
	}
	if isPatchIssue(detail.Labels) {
		return "", &apierror.ValidationError{Msg: msgLinkPatchRefused}
	}
	// GitHub's issues API also lists pull requests, whose html_url has /pull/.
	if strings.Contains(detail.HTMLURL, "/pull/") {
		return "", &apierror.ValidationError{Msg: msgLinkPullRequest}
	}

	title := strings.TrimSpace(detail.Title)
	if title == "" {
		title = fmt.Sprintf("%s/%s#%d", owner, repoName, req.IssueNumber)
	}
	if utf8.RuneCountInString(title) > maxWorkItemSubjectChars { // work_item.subject is VARCHAR(255)
		title = string([]rune(title)[:maxWorkItemSubjectChars])
	}
	var footer ProductIssueFooterInfo
	if detail.Body != nil {
		footer = ParseProductIssueFooter(*detail.Body)
	}
	state := domain.ProductIssueStateOpen
	if strings.EqualFold(detail.State, "closed") {
		state = domain.ProductIssueStateClosed
	}
	ref := repository.GitHubIssueRef{Owner: owner, Repository: repoName, Number: req.IssueNumber, NodeID: detail.NodeID, HTMLURL: detail.HTMLURL}
	if ref.HTMLURL == "" {
		ref.HTMLURL = fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repoName, req.IssueNumber)
	}
	var stateReason string
	if detail.StateReason != nil {
		stateReason = *detail.StateReason
	}
	id, err := s.repo.RegisterManualProductIssue(ctx, repository.ManualRegisterParams{
		CaseID: req.CaseID, Actor: email, Title: title,
		Priority: issuePriorityFromLabels(detail.Labels), Category: issueCategoryFromLabels(detail.Labels),
		OpenedByEmail: footer.OpenedBy, ABTTeam: footer.ABTTeam, PublicIssueURL: footer.PublicIssueURL,
		Issue: ref, State: state, StateReason: stateReason, Labels: detail.Labels,
		GitHubCreatedOn: parseRFC3339(detail.CreatedAt), GitHubUpdatedOn: parseRFC3339(detail.UpdatedAt), GitHubClosedOn: parseRFC3339Ptr(detail.ClosedAt),
	})
	if errors.Is(err, repository.ErrGitHubIssueAlreadyLinked) {
		return "", nil // another request registered it first: re-read
	}
	return id, err
}

func parseRFC3339(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func parseRFC3339Ptr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	return parseRFC3339(*s)
}
