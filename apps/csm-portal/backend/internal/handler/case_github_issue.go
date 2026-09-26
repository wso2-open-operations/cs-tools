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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/githubissue"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	// GitHub's own limits: a title is at most 256 characters and a body at most
	// 65,536. Checked here so the caller gets a specific 400 instead of an
	// opaque upstream failure.
	maxGitHubIssueTitleChars = 256
	maxGitHubIssueBodyChars  = 65536

	errMsgGitHubRepoRequired   = "A target repository is required."
	errMsgGitHubRepoNotAllowed = "The selected repository is not available."
	errMsgGitHubTitleInvalid   = "A title of up to 256 characters is required."
	errMsgGitHubBodyTooLong    = "The issue description is too long."

	// regressionLabel is applied when the caller marks the issue a regression.
	regressionLabel = "regression"
	// incidentIssueTypeLabel is the one issue type a priority label applies to.
	incidentIssueTypeLabel = "Type/Incident"
)

// engineeringGitIssueClient is the engineering entity service call used to file
// GitHub issues. When it is set on a CaseHandler, POST /cases/{id}/github-issues
// creates the issue through it instead of forwarding to the entity service.
type engineeringGitIssueClient interface {
	CreateGitIssue(ctx context.Context, orgName, owner, repoName, title, body string, labels []string) (entity.GitHubIssue, error)
}

// WithEngineeringClient makes POST /cases/{id}/github-issues create the issue
// through the engineering entity service. Left unset (the default), the request
// is forwarded to the entity service exactly as before. Returns h for chaining
// at the construction site.
func (h *CaseHandler) WithEngineeringClient(c engineeringGitIssueClient) *CaseHandler {
	h.engineering = c
	return h
}

// caseGitHubIssueRequest is the subset of the POST /cases/{id}/github-issues
// body this path reads. reason only steers the entity service's own repo
// routing, so it plays no part here: the target is always repoOverride.
type caseGitHubIssueRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	RepoOverride *struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"repoOverride"`
	UpdateLevel    string `json:"updateLevel"`
	PublicIssueURL string `json:"publicIssueUrl"`
	Regression     bool   `json:"regression"`
	HotFixRequired bool   `json:"hotFixRequired"`
	IssueTypeLabel string `json:"issueTypeLabel"`
	PriorityLevel  string `json:"priorityLevel"`
}

type caseGitHubIssueResponse struct {
	Message string                `json:"message"`
	Issue   caseGitHubIssueResult `json:"issue"`
}

type caseGitHubIssueResult struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
	Repo   string `json:"repo"`
}

// findGitHubRepoOption returns the configured catalogue entry for owner/repo.
// GitHub names are case-insensitive, so the match is too. Only catalogue repos
// may be targeted: without this, any caller with write access could have the
// service account file issues in an arbitrary repository.
func findGitHubRepoOption(owner, repo string) (githubissue.RepoOption, bool) {
	for _, o := range githubissue.Active() {
		if strings.EqualFold(o.Owner, owner) && strings.EqualFold(o.Repo, repo) {
			return o, true
		}
	}
	return githubissue.RepoOption{}, false
}

// buildGitHubIssueBody is the caller's description followed by the optional
// context lines, one per line.
func buildGitHubIssueBody(req caseGitHubIssueRequest) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(req.Description))
	extra := func(line string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	if v := strings.TrimSpace(req.UpdateLevel); v != "" {
		extra("\nUpdate Level : " + v)
	}
	if v := strings.TrimSpace(req.PublicIssueURL); v != "" {
		extra("\nPublic Issue : " + v)
	}
	if req.HotFixRequired {
		extra("\nHotfix Required : Yes")
	}
	return strings.TrimSpace(b.String())
}

// buildGitHubIssueLabels is the repo option's own label, then the issue-type
// label, the priority (only for an incident), and "regression" when flagged,
// without duplicates and without blanks.
func buildGitHubIssueLabels(option githubissue.RepoOption, req caseGitHubIssueRequest) []string {
	var labels []string
	seen := make(map[string]bool)
	add := func(l string) {
		l = strings.TrimSpace(l)
		if l == "" || seen[strings.ToLower(l)] {
			return
		}
		seen[strings.ToLower(l)] = true
		labels = append(labels, l)
	}
	add(option.GithubLabel)
	issueType := strings.TrimSpace(req.IssueTypeLabel)
	add(issueType)
	if issueType == incidentIssueTypeLabel {
		add(req.PriorityLevel)
	}
	if req.Regression {
		add(regressionLabel)
	}
	return labels
}

// createGitHubIssueViaEngineering files the issue in the requested catalogue
// repository through the engineering entity service. The case must exist and be
// visible to the caller (the entity service enforces that on GetCase).
//
// Unlike the entity service's own implementation, this does not write the issue
// URL back into the case's work notes or tag the case as a regression.
func (h *CaseHandler) createGitHubIssueViaEngineering(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, caseID string, body []byte) {
	var req caseGitHubIssueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxGitHubIssueTitleChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubTitleInvalid)
		return
	}
	if req.RepoOverride == nil || strings.TrimSpace(req.RepoOverride.Owner) == "" || strings.TrimSpace(req.RepoOverride.Repo) == "" {
		writeError(w, http.StatusBadRequest, errMsgGitHubRepoRequired)
		return
	}
	option, ok := findGitHubRepoOption(strings.TrimSpace(req.RepoOverride.Owner), strings.TrimSpace(req.RepoOverride.Repo))
	if !ok {
		writeError(w, http.StatusBadRequest, errMsgGitHubRepoNotAllowed)
		return
	}
	issueBody := buildGitHubIssueBody(req)
	if utf8.RuneCountInString(issueBody) > maxGitHubIssueBodyChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubBodyTooLong)
		return
	}

	if _, err := h.entity.GetCase(r.Context(), caseID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed before creating a GitHub issue", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}

	// The catalogue's owner is the GitHub organisation, so it is passed as both
	// the organisation and the owner the engineering service asks for. The
	// service picks its GitHub access token by that organisation name, so it
	// must be one it is configured with.
	issue, err := h.engineering.CreateGitIssue(r.Context(), option.Owner, option.Owner, option.Repo, title, issueBody, buildGitHubIssueLabels(option, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "engineering CreateGitIssue failed", "userID", user.UserID, "caseID", caseID, "repo", option.Owner+"/"+option.Repo, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}

	// The engineering service returns the issue's id, number, state, title, body
	// and labels but no URL, and files issues on github.com, so the URL is built
	// from the repo and number.
	slog.InfoContext(r.Context(), "GitHub issue created from case", "userID", user.UserID, "caseID", caseID, "repo", option.Owner+"/"+option.Repo, "number", issue.Number)
	writeJSONValue(w, http.StatusCreated, caseGitHubIssueResponse{
		Message: "GitHub issue created.",
		Issue: caseGitHubIssueResult{
			URL:    fmt.Sprintf("https://github.com/%s/%s/issues/%d", option.Owner, option.Repo, issue.Number),
			Number: issue.Number,
			Repo:   option.Owner + "/" + option.Repo,
		},
	})
}
