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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// githubIssueCreator is the one GitHub call this service makes.
type githubIssueCreator interface {
	CreateIssue(ctx context.Context, owner, repository, title, body string, labels []string) (*github.CreatedIssue, error)
}

// caseStateReader is the slice of CaseService this service needs. Narrow on
// purpose: the state gate is the only thing it asks the case for, and
// depending on the whole CaseService would make this untestable without
// standing up an implementation of forty methods.
type caseStateReader interface {
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
}

// caseGithubIssueRepo is the slice of GithubSyncRepository this service needs.
type caseGithubIssueRepo interface {
	AccountForCase(ctx context.Context, caseID string) (string, error)
	RepoForAccount(ctx context.Context, accountID string) (*repository.RepoMapping, error)
	SetCaseGithubIssueNumber(ctx context.Context, caseID string, issueNumber int) (bool, error)
}

// caseGithubIssueService files a GitHub issue for a case natively, in place of
// the ServiceNow scripted API that snCaseGithubIssueService proxies to.
//
// FILING THE ISSUE IS ALSO WHAT SWITCHES ON SYNC. The outbound triggers need
// two things before they enqueue anything: an active account_github_repo row,
// and an issue number on the case. This writes the second, so there is no
// separate "enable sync for this case" step -- it falls out of the gate model
// (see migration 000069).
//
// The validation and the state gate below are lifted unchanged from
// snCaseGithubIssueService. Only the delivery mechanism differs, and a caller
// must not be able to tell which implementation answered.
type caseGithubIssueService struct {
	gh      githubIssueCreator
	repo    caseGithubIssueRepo
	caseSvc caseStateReader
	labels  GithubLabels
}

// NewCaseGithubIssueService constructs the native implementation.
func NewCaseGithubIssueService(gh githubIssueCreator, repo caseGithubIssueRepo, caseSvc caseStateReader, labels GithubLabels) CaseGithubIssueService {
	return &caseGithubIssueService{gh: gh, repo: repo, caseSvc: caseSvc, labels: labels}
}

// CreateCaseGithubIssue implements CaseGithubIssueService.
func (s *caseGithubIssueService) CreateCaseGithubIssue(ctx context.Context, req domain.CreateCaseGithubIssueRequest) (domain.CreateCaseGithubIssueResponse, error) {
	if err := validateCaseGithubIssueRequest(req); err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}

	caseView, err := s.caseSvc.GetCaseByID(ctx, req.CaseID)
	if err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}
	if _, ok := caseGithubIssueActionableStates[derefState(caseView.State)]; !ok {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ConflictError{
			Msg: "Case is not in a state that allows filing a GitHub issue",
		}
	}

	owner, repoName, err := s.resolveRepository(ctx, req)
	if err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}

	issue, err := s.gh.CreateIssue(ctx, owner, repoName, titleFor(req.Title), req.Description, s.labelsFor(req))
	if err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}

	// THE ISSUE NOW EXISTS AND NOTHING POINTS AT IT YET.
	//
	// If this write fails the issue is real but orphaned: the case is not
	// linked, so no sync will ever reach it and a second attempt would file a
	// duplicate. There is no transaction spanning GitHub and Postgres, so the
	// choice is what to tell the caller.
	//
	// The error carries the issue's URL and number. A silent failure would
	// leave someone hunting for an issue they were never told about, and a
	// bare "could not link" would have them retry and file a second one. With
	// the URL in hand the link can be repaired by re-filing against the
	// existing issue, or by hand, and the duplicate is avoided.
	//
	// Not retried inline: the write is a single UPDATE by primary key, so a
	// failure here is the database being unreachable rather than contention,
	// and an immediate retry against an unreachable database buys nothing.
	if _, err := s.repo.SetCaseGithubIssueNumber(ctx, req.CaseID, issue.Number); err != nil {
		slog.ErrorContext(ctx, "github: issue filed but the case could not be linked",
			"caseId", req.CaseID, "issueNumber", issue.Number, "issueUrl", issue.HTMLURL, "err", err)
		return domain.CreateCaseGithubIssueResponse{}, fmt.Errorf(
			"GitHub issue %s (#%d) was created but the case could not be linked to it; "+
				"link it before filing another, or a duplicate issue will be created: %w",
			issue.HTMLURL, issue.Number, err)
	}

	return domain.CreateCaseGithubIssueResponse{
		Message: "GitHub issue created successfully",
		Issue: domain.CaseGithubIssueDetail{
			URL:    issue.HTMLURL,
			Number: issue.Number,
			Repo:   owner + "/" + repoName,
		},
	}, nil
}

// resolveRepository decides where the issue is filed: the explicit override if
// one was given, otherwise the account's mapping.
//
// An override is NOT checked against account_github_repo. ServiceNow's own API
// took repoOverride as an instruction, and an agent filing to a repository the
// account does not normally use is the case the field exists for.
func (s *caseGithubIssueService) resolveRepository(ctx context.Context, req domain.CreateCaseGithubIssueRequest) (string, string, error) {
	if req.RepoOverride != nil {
		return req.RepoOverride.Owner, req.RepoOverride.Repo, nil
	}

	accountID, err := s.repo.AccountForCase(ctx, req.CaseID)
	if err != nil {
		return "", "", err
	}
	if accountID == "" {
		return "", "", &apierror.ConflictError{
			Msg: "Case has no account, so there is no repository to file the issue in",
		}
	}

	mapping, err := s.repo.RepoForAccount(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if mapping == nil {
		// Nothing to serve this on. A conflict rather than a not-found: the
		// case exists and is otherwise fileable, it is the routing that is
		// absent, and that is fixed by seeding account_github_repo.
		return "", "", &apierror.ConflictError{
			Msg: "This account has no active GitHub repository mapping; add one before filing an issue",
		}
	}
	return mapping.Owner, mapping.Repository, nil
}

// titleFor prefixes the title so the issue is recognisable as a change
// request. A change request carries no type label -- the prefix is the only
// signal, per issue_servicenow.yml -- so an issue filed without it would be
// invisible to our own inbound sync.
func titleFor(title string) string {
	if IsChangeRequestTitle(title) {
		return title
	}
	return TitlePrefixChangeRequest + " " + title
}

// labelsFor is the label set the new issue carries. The class label is what
// the inbound gate needs alongside the title prefix.
func (s *caseGithubIssueService) labelsFor(req domain.CreateCaseGithubIssueRequest) []string {
	var labels []string
	for label := range s.labels.ClassByLabel {
		// Default to a normal change: the caller files from a case, and the
		// class is refined on the issue if it turns out to be anything else.
		if s.labels.ClassByLabel[label] == "Normal Change" {
			labels = append(labels, label)
			break
		}
	}
	if req.IssueTypeLabel != nil && strings.TrimSpace(*req.IssueTypeLabel) != "" {
		labels = append(labels, strings.TrimSpace(*req.IssueTypeLabel))
	}
	if req.PriorityLevel != nil && strings.TrimSpace(*req.PriorityLevel) != "" {
		labels = append(labels, strings.TrimSpace(*req.PriorityLevel))
	}
	return labels
}

// validateCaseGithubIssueRequest is the shared gate, so the native and
// ServiceNow implementations cannot drift apart on what they accept.
func validateCaseGithubIssueRequest(req domain.CreateCaseGithubIssueRequest) error {
	if req.CaseID == "" {
		return &apierror.ValidationError{Msg: "case ID is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return err
	}
	if _, ok := validCaseGithubIssueReasons[req.Reason]; !ok {
		return &apierror.ValidationError{Msg: "reason contains invalid value: " + string(req.Reason)}
	}
	if req.Title == "" {
		return &apierror.ValidationError{Msg: "title is required"}
	}
	if req.Description == "" {
		return &apierror.ValidationError{Msg: "description is required"}
	}
	if req.RepoOverride != nil && (req.RepoOverride.Owner == "" || req.RepoOverride.Repo == "") {
		return &apierror.ValidationError{Msg: "repoOverride requires both owner and repo"}
	}
	return nil
}
