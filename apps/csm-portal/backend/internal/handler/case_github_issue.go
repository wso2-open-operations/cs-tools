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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	// GitHub's own limits: a title is at most 256 characters and a body at most
	// 65,536. Checked here so the caller gets a specific 400 instead of an
	// opaque upstream failure.
	maxGitHubIssueTitleChars = 256
	maxGitHubIssueBodyChars  = 65536

	errMsgGitHubRepoNotMapped = "No GitHub repository is mapped for this product."
	errMsgGitHubTitleInvalid  = "A title of up to 256 characters is required."
	errMsgGitHubBodyTooLong   = "The issue description is too long."

	regressionLabel          = "regression"
	originLabel              = "Origin/CS"
	patchIssueTypeLabel      = "Type/Patch"
	patchExtraLabel          = "patch"
	discussionIssueTypeLabel = "Type/Discussion"
	hotfixLabel              = "Require/Hotfix"
	migrationLabel           = "Affected/Migration"
	onboardingLabel          = "Onboarding/affected"
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
// body this path reads. The repository comes from the case product, not from
// repoOverride. reason "migration" adds Affected/Migration.
type caseGitHubIssueRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Reason       string `json:"reason"`
	RepoOverride *struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"repoOverride"`
	UpdateLevel          string `json:"updateLevel"`
	PublicIssueURL       string `json:"publicIssueUrl"`
	Regression           bool   `json:"regression"`
	HotFixRequired       bool   `json:"hotFixRequired"`
	IssueTypeLabel       string `json:"issueTypeLabel"`
	PriorityLevel        string `json:"priorityLevel"`
	OnboardingInProgress bool   `json:"onboardingInProgress"`
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

// buildGitHubIssueLabels applies the sheet's rules. Origin/CS and the product
// label always go on. Patch adds Type/Patch and patch. Discussion adds the
// priority label. The switches add Require/Hotfix, regression, and
// Affected/Migration. An in-progress project adds Onboarding/affected.
func buildGitHubIssueLabels(productLabel string, req caseGitHubIssueRequest) []string {
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
	add(originLabel)
	// The update level is free text from the case. It is a version label only
	// when it is not one of the labels this function assigns itself, so a
	// value such as Priority/Critical cannot land on a Patch issue.
	if !reservedIssueLabel(req.UpdateLevel) {
		add(req.UpdateLevel)
	}
	add(productLabel)
	issueType := strings.TrimSpace(req.IssueTypeLabel)
	switch issueType {
	case patchIssueTypeLabel:
		add(patchIssueTypeLabel)
		add(patchExtraLabel)
	case discussionIssueTypeLabel:
		add(req.PriorityLevel)
	}
	if req.HotFixRequired {
		add(hotfixLabel)
	}
	if req.Regression {
		add(regressionLabel)
	}
	if strings.EqualFold(strings.TrimSpace(req.Reason), "migration") {
		add(migrationLabel)
	}
	if req.OnboardingInProgress {
		add(onboardingLabel)
	}
	return labels
}

// reservedIssueLabel reports whether s is a label this builder assigns for a
// reason other than the product version.
//
// The three priority strings are the values of SEVERITY_OPTIONS in
// CreateGithubIssueDialog.tsx. A new severity there has to be added here too,
// or an update level with that text would be filed as a label.
func reservedIssueLabel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case strings.ToLower(originLabel),
		strings.ToLower(patchIssueTypeLabel),
		strings.ToLower(patchExtraLabel),
		strings.ToLower(discussionIssueTypeLabel),
		strings.ToLower(hotfixLabel),
		strings.ToLower(migrationLabel),
		strings.ToLower(onboardingLabel),
		strings.ToLower(regressionLabel),
		"priority/critical",
		"priority/high",
		"priority/medium":
		return true
	default:
		return false
	}
}

// productNameFromCase prefers the catalogue product name, then the versioned
// display name. The entity lookup treats a trailing version as a prefix match.
func productNameFromCase(raw []byte) string {
	var view struct {
		DeployedProduct *struct {
			DisplayName *string `json:"displayName"`
			Product     *struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"deployedProduct"`
	}
	if err := json.Unmarshal(raw, &view); err != nil || view.DeployedProduct == nil {
		return ""
	}
	if view.DeployedProduct.Product != nil {
		if name := strings.TrimSpace(view.DeployedProduct.Product.Name); name != "" {
			return name
		}
	}
	if view.DeployedProduct.DisplayName != nil {
		return strings.TrimSpace(*view.DeployedProduct.DisplayName)
	}
	return ""
}

// createGitHubIssueViaEngineering files the issue through the engineering
// entity service. The repository and product label come from
// product_repo_mapping, looked up by the case product. The browser's
// repoOverride is ignored.
//
// When the engineering client is configured, every create uses this path.
// After GitHub accepts the issue, a work note with the issue URL is written
// on the case. That write is best-effort: a failure is logged and the create
// response is still success, because the issue already exists. Case tags
// stay on the portal, which already calls POST /cases/{id}/tags and can show
// a failure there.
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
	issueBody := buildGitHubIssueBody(req)
	if utf8.RuneCountInString(issueBody) > maxGitHubIssueBodyChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubBodyTooLong)
		return
	}

	caseRaw, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed before creating a GitHub issue", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}
	productName := productNameFromCase(caseRaw)
	if productName == "" {
		writeError(w, http.StatusBadRequest, errMsgGitHubRepoNotMapped)
		return
	}
	mappingRaw, err := h.entity.GetProductRepoMapping(r.Context(), productName)
	if err != nil {
		var upstream *apierror.Error
		if errors.As(err, &upstream) && upstream.StatusCode == http.StatusNotFound {
			writeError(w, http.StatusBadRequest, errMsgGitHubRepoNotMapped)
			return
		}
		slog.ErrorContext(r.Context(), "entity GetProductRepoMapping failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}
	var mapping struct {
		Owner       string `json:"owner"`
		Repository  string `json:"repository"`
		GithubLabel string `json:"githubLabel"`
	}
	if err := json.Unmarshal(mappingRaw, &mapping); err != nil || strings.TrimSpace(mapping.Owner) == "" || strings.TrimSpace(mapping.Repository) == "" {
		writeError(w, http.StatusBadRequest, errMsgGitHubRepoNotMapped)
		return
	}
	owner := strings.TrimSpace(mapping.Owner)
	repo := strings.TrimSpace(mapping.Repository)

	// The mapping's owner is the GitHub organisation, so it is passed as both
	// the organisation and the owner the engineering service asks for. The
	// service picks its GitHub access token by that organisation name, so it
	// must be one it is configured with.
	issue, err := h.engineering.CreateGitIssue(r.Context(), owner, owner, repo, title, issueBody, buildGitHubIssueLabels(mapping.GithubLabel, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "engineering CreateGitIssue failed", "userID", user.UserID, "caseID", caseID, "repo", owner+"/"+repo, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}

	// The engineering service returns the issue's id, number, state, title, body
	// and labels but no URL, and files issues on github.com, so the URL is built
	// from the repo and number.
	slog.InfoContext(r.Context(), "GitHub issue created from case", "userID", user.UserID, "caseID", caseID, "repo", owner+"/"+repo, "number", issue.Number)
	issueURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issue.Number)
	h.recordGitHubIssueWorkNote(r.Context(), user, caseID, issueURL)
	writeJSONValue(w, http.StatusCreated, caseGitHubIssueResponse{
		Message: "GitHub issue created.",
		Issue: caseGitHubIssueResult{
			URL:    issueURL,
			Number: issue.Number,
			Repo:   owner + "/" + repo,
		},
	})
}

// recordGitHubIssueWorkNote leaves the issue URL on the case. Best-effort:
// the GitHub issue already exists, so a failed note must not fail the create.
func (h *CaseHandler) recordGitHubIssueWorkNote(ctx context.Context, user *middleware.UserInfo, caseID, issueURL string) {
	body, err := json.Marshal(map[string]string{
		"type":    "work_note",
		"content": "GitHub issue filed: " + issueURL,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build GitHub issue work note", "userID", user.UserID, "caseID", caseID, "err", err)
		return
	}
	if _, err := h.entity.CreateCaseComment(ctx, caseID, body); err != nil {
		slog.WarnContext(ctx, "failed to record GitHub issue work note", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	}
}
