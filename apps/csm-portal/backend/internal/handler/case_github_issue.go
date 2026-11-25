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
	"slices"
	"strings"
	"time"
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
	errMsgGitHubBodyTooLong   = "The issue is too long."

	errMsgGitHubHotfixReasonInvalid = "Choose a hotfix reason from the list."

	regressionLabel          = "regression"
	originLabel              = "Origin/CS"
	patchIssueTypeLabel      = "Type/Patch"
	featureIssueTypeLabel    = "Type/Feature"
	patchExtraLabel          = "patch"
	discussionIssueTypeLabel = "Type/Discussion"
	incidentIssueTypeLabel   = "Type/Incident"
	hotfixLabel              = "Require/Hotfix"
	migrationLabel           = "Affected/Migration"
	onboardingLabel          = "Onboarding/affected"

	// The priorities of the Internal Issue Handling Policy, as GitHub labels.
	priorityImmediateLabel = "Priority/Immediate(P0)"
	priorityHighLabel      = "Priority/High(P1)"
	priorityMediumLabel    = "Priority/Medium(P2)"
	priorityLowLabel       = "Priority/Low(P3)"
)

const (
	deployedProductLookupPageSize = 50
	// A deployment rarely holds more than a handful of deployed products; the
	// page bound only guards a response that never reports the end.
	deployedProductLookupMaxPages = 4
)

// gitHubPrioritiesWithJustification are the priorities that carry a Priority
// Justification: P0 to P2 (P0 and P1 also need CS leadership's review first).
var gitHubPrioritiesWithJustification = map[string]bool{
	priorityImmediateLabel: true,
	priorityHighLabel:      true,
	priorityMediumLabel:    true,
}

// gitHubHotfixReasons are the reasons the dialog offers when a hotfix is required.
//
// It must match the webapp's src/features/csm-cases/utils/githubIssueOptions.json,
// which the dialog reads: a choice missing here is a 400 for the engineer.
// TestGitHubIssueChoiceListsMatchTheWebapp fails when the two differ.
var gitHubHotfixReasons = []string{
	"Customer-Requested Hotfix",
	"Critical Production Bug / Downtime",
	"Defect Blocking Operations / Development",
	"Unable to Upgrade to Latest U2",
}

// deployedProductLookupTimeout bounds the deployed-product lookup so a slow
// entity service can delay filing a Git issue by at most this long, never block
// it. A variable so a test can shorten it.
var deployedProductLookupTimeout = 5 * time.Second

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
	// The fields below only add to the issue body; none of them is a label.
	PriorityJustification string `json:"priorityJustification"`
	ReproducingSteps      string `json:"reproducingSteps"`
	HotFixReason          string `json:"hotFixReason"`
	RelatedInternalIssues string `json:"relatedInternalIssues"`
	BusinessUseCase       string `json:"businessUseCase"`
	RelatedGitIssues      string `json:"relatedGitIssues"`
}

// priorityJustification is the request's free-text justification when its
// priority is P0, P1 or P2, "" otherwise: a P3 or an unknown priority carries
// none, so one sent with it is not written. The dialog's field is one line, so
// any line breaks are collapsed and the body keeps it on its own line.
func (req caseGitHubIssueRequest) priorityJustification() string {
	if !gitHubPrioritiesWithJustification[githubPriorityLabel(req.PriorityLevel)] {
		return ""
	}
	return strings.Join(strings.Fields(req.PriorityJustification), " ")
}

// hotFixReason is the request's hotfix reason when a hotfix is required, "" otherwise.
func (req caseGitHubIssueRequest) hotFixReason() string {
	if !req.HotFixRequired {
		return ""
	}
	return strings.TrimSpace(req.HotFixReason)
}

// scopedToIssueType is the request with the fields of the other issue type
// cleared, so they reach neither the body nor the labels: a Feature Request
// carries no priority, justification, reproducing steps, hotfix, regression or
// related internal issues, and a Discussion no business use case or related
// Git issues. The dialog never sends them; this covers any other caller. Any
// other type (the legacy Type/Patch, or none) is left as sent.
func (req caseGitHubIssueRequest) scopedToIssueType() caseGitHubIssueRequest {
	switch strings.TrimSpace(req.IssueTypeLabel) {
	case featureIssueTypeLabel:
		req.PriorityLevel = ""
		req.PriorityJustification = ""
		req.ReproducingSteps = ""
		req.HotFixRequired = false
		req.HotFixReason = ""
		req.Regression = false
		req.RelatedInternalIssues = ""
	case discussionIssueTypeLabel:
		req.BusinessUseCase = ""
		req.RelatedGitIssues = ""
	}
	return req
}

// validateGitHubIssueChoices checks that the hotfix reason, when it will be
// written, is one of the dialog's own options. It is not required here: the
// dialog requires it, and refusing a request without one would break a webapp
// one release behind this backend. The priority justification is free text.
func validateGitHubIssueChoices(req caseGitHubIssueRequest) string {
	if r := req.hotFixReason(); r != "" && !slices.Contains(gitHubHotfixReasons, r) {
		return errMsgGitHubHotfixReasonInvalid
	}
	return ""
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

// caseGitHubIssueContext is the subset of a case this handler needs beyond
// what the request body itself carries, to fill the fixed info block at the
// foot of every filed issue's body. A field is left "" when the case has none
// (e.g. an account with no CRE team) -- buildGitHubIssueBody omits its line
// rather than rendering it empty. OpenedBy is not a case field: it is the
// engineer filing the issue, set by the caller from the signed-in user.
type caseGitHubIssueContext struct {
	// Product is the deployed product the way the earlier Git issues wrote it,
	// "<abbreviation>-<version>" (e.g. "wso2is-6.1.0"), when deployedProductSlug
	// found both. Otherwise it is the deployed product's own label, which carries
	// the version (e.g. "WSO2 Identity Server 6.1.0"), then the product catalogue
	// name when the case links no deployed product.
	Product    string
	Number     string
	InternalID string
	// Team is the case's ABT team: its account's CRE team, the same team the
	// case page's "CRE / SRE team" cell, the de-escalation lead check and the
	// case-event payloads call the case's team. The case's own assignedTeam is
	// only the fallback for an account with no CRE team: entity-service leaves
	// it unpopulated on synced data, so it alone left the line missing.
	Team     string
	OpenedBy string
}

// caseGitHubIssueContextFromCase reads the same raw GetCase response
// productNameFromCase already decodes a different projection of. It
// deliberately does not read the case's createdBy: that is the customer who
// raised the case, not the WSO2 engineer opening the Git issue.
func caseGitHubIssueContextFromCase(raw []byte) caseGitHubIssueContext {
	var view struct {
		Number          string `json:"number"`
		InternalID      string `json:"internalId"`
		DeployedProduct *struct {
			DisplayName *string `json:"displayName"`
			Product     *struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"deployedProduct"`
		AssignedTeam *struct {
			Name string `json:"name"`
		} `json:"assignedTeam"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		return caseGitHubIssueContext{}
	}
	ctx := caseGitHubIssueContext{Number: view.Number, InternalID: view.InternalID}
	if dp := view.DeployedProduct; dp != nil {
		if dp.DisplayName != nil {
			ctx.Product = strings.TrimSpace(*dp.DisplayName)
		}
		if ctx.Product == "" && dp.Product != nil {
			ctx.Product = strings.TrimSpace(dp.Product.Name)
		}
	}
	if ctx.Team = creTeamNameFromCase(raw); ctx.Team == "" && view.AssignedTeam != nil {
		ctx.Team = strings.TrimSpace(view.AssignedTeam.Name)
	}
	return ctx
}

// creTeamNameFromCase is the case's account's CRE team name, "" when the case
// has no account or the account no CRE team. It decodes the account on its own
// so that an account object of an unexpected shape costs only this line, never
// the case number or case id the caller has already read.
func creTeamNameFromCase(raw []byte) string {
	var view struct {
		Account *struct {
			CreTeam *struct {
				Name string `json:"name"`
			} `json:"creTeam"`
		} `json:"account"`
	}
	if err := json.Unmarshal(raw, &view); err != nil || view.Account == nil || view.Account.CreTeam == nil {
		return ""
	}
	return strings.TrimSpace(view.Account.CreTeam.Name)
}

// deployedProductSlug is the case's deployed product as "<abbreviation>-<version>"
// (e.g. "wso2is-6.1.0"), the form the earlier Git issues carried and the one the
// product-updates catalogue keys on. The case itself holds only the display name
// ("WSO2 Identity Server 6.1.0"), so it reads the case's deployment's deployed
// products and picks the one the case names. It is best effort: "" when the case
// names no deployment or deployed product, the product is not found, either part
// is missing, or the lookup fails or runs past deployedProductLookupTimeout, and
// the caller then keeps the deployed product's label. It never fails the filing.
func (h *CaseHandler) deployedProductSlug(ctx context.Context, caseRaw []byte, userID, caseID string) string {
	var view struct {
		Deployment *struct {
			ID string `json:"id"`
		} `json:"deployment"`
		DeployedProduct *struct {
			ID *string `json:"id"`
		} `json:"deployedProduct"`
	}
	if err := json.Unmarshal(caseRaw, &view); err != nil || view.Deployment == nil || view.DeployedProduct == nil || view.DeployedProduct.ID == nil {
		return ""
	}
	deploymentID := strings.TrimSpace(view.Deployment.ID)
	wantID := idKey(*view.DeployedProduct.ID)
	if deploymentID == "" || wantID == "" {
		return ""
	}

	// Every way the lookup can come back empty is logged with why (ids only), so a
	// Product that fell back to the display name can be explained afterwards.
	fallback := func(reason string, err error) string {
		args := []any{"reason", reason, "userID", userID, "caseID", caseID}
		if err != nil {
			args = append(args, "err", err)
		}
		slog.WarnContext(ctx, "deployed product lookup found no abbreviation and version while filing a GitHub issue; using the product label", args...)
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, deployedProductLookupTimeout)
	defer cancel()
	for page := 0; page < deployedProductLookupMaxPages; page++ {
		body, err := json.Marshal(map[string]any{
			"deploymentIds": []string{deploymentID},
			"pagination":    map[string]int{"limit": deployedProductLookupPageSize, "offset": page * deployedProductLookupPageSize},
		})
		if err != nil {
			return fallback("request not built", err)
		}
		raw, err := h.entity.SearchDeployedProducts(ctx, body)
		if err != nil {
			return fallback("search failed", err)
		}
		var resp struct {
			DeployedProducts []struct {
				ID      string `json:"id"`
				Product *struct {
					Abbreviation *string `json:"abbreviation"`
				} `json:"product"`
				Version *struct {
					Name string `json:"name"`
				} `json:"version"`
			} `json:"deployedProducts"`
			HasMore bool `json:"hasMore"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return fallback("response not understood", err)
		}
		for _, dp := range resp.DeployedProducts {
			if idKey(dp.ID) != wantID {
				continue
			}
			if dp.Product == nil || dp.Product.Abbreviation == nil || dp.Version == nil {
				return fallback("deployed product has no abbreviation or version", nil)
			}
			abbreviation, version := strings.TrimSpace(*dp.Product.Abbreviation), strings.TrimSpace(dp.Version.Name)
			if abbreviation == "" || version == "" {
				return fallback("deployed product has a blank abbreviation or version", nil)
			}
			return abbreviation + "-" + version
		}
		// An empty page ends the walk even when hasMore says otherwise, the same
		// guard the entity service's own deployed-product paging uses.
		if !resp.HasMore || len(resp.DeployedProducts) == 0 {
			return fallback("deployed product not listed on the deployment", nil)
		}
	}
	return fallback("deployed product not found within the page limit", nil)
}

// idKey makes two spellings of one id comparable: the entity service writes the
// same id with or without dashes depending on the data source it was read from.
func idKey(id string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(id), "-", ""))
}

// issueInfoBlankValue stands in for a footer value that is not known: the word the
// earlier Git issues carry there, so the block reads the same as theirs.
const issueInfoBlankValue = "undefined"

// buildGitHubIssueBody is the caller's own description, then the optional
// sections the engineer filled in, in gitHubIssueSections' order (Reproducing
// Steps, Business Use Case, Priority Justification, the Hotfix Required line
// with its Hotfix Reason, Related Internal Issue(s), Related Git Issue(s)),
// then the info block as the footer: always the same
// seven lines (Product, Update level, WSO2 case id, Case number, Public Git
// Issue, ABT Team, Opened by), in this order and spelling, as the issues filed
// before the portal moved to the engineering service carry it. A value that is
// not known is written as "undefined", never dropped, so the block keeps one shape
// whatever the engineer filled in and whatever the case holds. The block goes
// last so the issue opens with what the engineer wrote, and so the Git
// Internals dashboard, which reads the last "ABT Team"/"Opened by" line of a
// body as the automated footer, is not overridden by a line quoted inside the
// description.
func buildGitHubIssueBody(req caseGitHubIssueRequest, productLabel string, caseCtx caseGitHubIssueContext) string {
	var info strings.Builder
	line := func(label, value string) {
		// Fields rather than TrimSpace: the block is parsed line by line downstream, so a
		// value holding a newline must not start a line of its own.
		if value = strings.Join(strings.Fields(value), " "); value == "" {
			value = issueInfoBlankValue
		}
		if info.Len() > 0 {
			info.WriteString("\n")
		}
		info.WriteString(label + " : " + value)
	}
	// The deployed product's own label carries the version; the repository
	// mapping's label (e.g. "product/is") is a GitHub label, not a product name,
	// and is only the last resort when the case has no product label at all.
	product := caseCtx.Product
	if product == "" {
		product = productLabel
	}
	line("Product", product)
	line("Update level", req.UpdateLevel)
	line("WSO2 case id", caseCtx.InternalID)
	line("Case number", caseCtx.Number)
	line("Public Git Issue", req.PublicIssueURL)
	line("ABT Team", caseCtx.Team)
	line("Opened by", caseCtx.OpenedBy)

	sections := append([]string{strings.TrimSpace(req.Description)}, gitHubIssueSections(req)...)
	sections = append(sections, info.String())
	return joinGitHubIssueSections(sections)
}

// gitHubIssueSections are the optional sections the engineer filled in, in body
// order: Reproducing Steps, Business Use Case, Priority Justification, the
// Hotfix Required line with its Hotfix Reason, Related Internal Issue(s),
// Related Git Issue(s). A section left empty is
// not in the list.
func gitHubIssueSections(req caseGitHubIssueRequest) []string {
	var sections []string
	// A free-text section keeps its own line breaks: it is read by people, and
	// only the footer is parsed line by line.
	labelled := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			sections = append(sections, label+" :\n"+value)
		}
	}
	labelled("Reproducing Steps", req.ReproducingSteps)
	labelled("Business Use Case", req.BusinessUseCase)
	if j := req.priorityJustification(); j != "" {
		sections = append(sections, "Priority Justification : "+j)
	}
	if req.HotFixRequired {
		// "Hotfix Required : Yes" stays a line of its own, spelled as before.
		hotfix := "Hotfix Required : Yes"
		if r := req.hotFixReason(); r != "" {
			hotfix += "\nHotfix Reason : " + r
		}
		sections = append(sections, hotfix)
	}
	labelled("Related Internal Issue(s)", req.RelatedInternalIssues)
	labelled("Related Git Issue(s)", req.RelatedGitIssues)
	return sections
}

// joinGitHubIssueSections joins the non-empty sections with a blank line.
func joinGitHubIssueSections(sections []string) string {
	return strings.TrimSpace(strings.Join(slices.DeleteFunc(sections, func(s string) bool { return s == "" }), "\n\n"))
}

// gitHubIssueBodyOnlyFields are the request fields only this backend knows. The
// entity service's POST /cases/{id}/github-issues decodes with unknown fields
// refused, and matches a key whatever its case, so these are removed from the
// forwarded request in any spelling.
var gitHubIssueBodyOnlyFields = []string{"reproducingSteps", "priorityJustification", "hotFixReason", "relatedInternalIssues", "businessUseCase", "relatedGitIssues"}

// entityGitHubIssueRequest prepares the request for the entity service, the
// path used when no engineering client is configured. The fields the entity
// service does not know are written into the description as the same sections
// the engineering path writes, and removed from the request, so a P0 to P2 or
// hotfix filing keeps its details instead of being refused. A request without
// any of them is forwarded byte for byte. The second result is a message for a
// 400 when the request cannot be used.
func entityGitHubIssueRequest(body []byte) ([]byte, string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, ErrMsgBadRequest
	}
	stripped := false
	for key := range fields {
		if slices.ContainsFunc(gitHubIssueBodyOnlyFields, func(f string) bool { return strings.EqualFold(f, key) }) {
			delete(fields, key)
			stripped = true
		}
	}
	if !stripped {
		return body, ""
	}
	var req caseGitHubIssueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, ErrMsgBadRequest
	}
	// Only the sections written into the description are scoped here: the
	// fields the entity service knows (priorityLevel, hotFixRequired,
	// regression) are forwarded as sent.
	req = req.scopedToIssueType()
	if msg := validateGitHubIssueChoices(req); msg != "" {
		return nil, msg
	}
	if sections := gitHubIssueSections(req); len(sections) > 0 {
		description, err := json.Marshal(joinGitHubIssueSections(append([]string{strings.TrimSpace(req.Description)}, sections...)))
		if err != nil {
			return nil, ErrMsgBadRequest
		}
		for key := range fields {
			if strings.EqualFold(key, "description") {
				delete(fields, key)
			}
		}
		fields["description"] = description
	}
	forward, err := json.Marshal(fields)
	if err != nil {
		return nil, ErrMsgBadRequest
	}
	return forward, ""
}

// githubPriorityLabel maps the dialog's priority onto the GitHub label names
// of the Internal Issue Handling Policy (P0 to P3). Bare Priority/Immediate
// (etc.) is rewritten to the (Pn) form; anything else is left as written, so a
// webapp one release behind still files the label it sent.
func githubPriorityLabel(level string) string {
	switch strings.TrimSpace(level) {
	case "Priority/Immediate", priorityImmediateLabel:
		return priorityImmediateLabel
	case "Priority/High", priorityHighLabel:
		return priorityHighLabel
	case "Priority/Medium", priorityMediumLabel:
		return priorityMediumLabel
	case "Priority/Low", priorityLowLabel:
		return priorityLowLabel
	default:
		return strings.TrimSpace(level)
	}
}

// buildGitHubIssueLabels applies the sheet's rules. Origin/CS and the product
// label always go on. Patch adds Type/Patch and patch. Discussion adds
// Type/Incident and the (Pn) priority label. Feature adds Type/Feature only,
// no priority label. The switches add Require/Hotfix,
// regression, and Affected/Migration. An in-progress project adds
// Onboarding/affected.
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
	add(productLabel)
	issueType := strings.TrimSpace(req.IssueTypeLabel)
	switch issueType {
	case patchIssueTypeLabel:
		add(patchIssueTypeLabel)
		add(patchExtraLabel)
	case featureIssueTypeLabel:
		add(featureIssueTypeLabel)
	case discussionIssueTypeLabel:
		add(incidentIssueTypeLabel)
		add(githubPriorityLabel(req.PriorityLevel))
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
	req = req.scopedToIssueType()

	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxGitHubIssueTitleChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubTitleInvalid)
		return
	}
	// A cheap early guard on the caller's own description -- the real body
	// (built below, once the case and its product mapping are in hand) can
	// only be longer, so a description already over the limit is rejected
	// before any upstream call.
	if utf8.RuneCountInString(strings.TrimSpace(req.Description)) > maxGitHubIssueBodyChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubBodyTooLong)
		return
	}

	if msg := validateGitHubIssueChoices(req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	caseRaw, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed before creating a GitHub issue", "userID", user.UserID, "caseID", caseID, "err", err)
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
		slog.ErrorContext(r.Context(), "entity GetProductRepoMapping failed", "userID", user.UserID, "caseID", caseID, "err", err)
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

	// github_label is NOT NULL but not constrained against being blank, and
	// this table is known to carry occasional data-quality gaps -- fall back
	// to the case's own product name (already validated non-empty above)
	// rather than silently dropping the Product line.
	productLabel := strings.TrimSpace(mapping.GithubLabel)
	if productLabel == "" {
		productLabel = productName
	}
	caseCtx := caseGitHubIssueContextFromCase(caseRaw)
	if slug := h.deployedProductSlug(r.Context(), caseRaw, user.UserID, caseID); slug != "" {
		caseCtx.Product = slug
	}
	caseCtx.OpenedBy = strings.TrimSpace(user.Email)
	issueBody := buildGitHubIssueBody(req, productLabel, caseCtx)
	if utf8.RuneCountInString(issueBody) > maxGitHubIssueBodyChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubBodyTooLong)
		return
	}

	// The mapping's owner is the GitHub organisation, so it is passed as both
	// the organisation and the owner the engineering service asks for. The
	// service picks its GitHub access token by that organisation name, so it
	// must be one it is configured with.
	issue, err := h.engineering.CreateGitIssue(r.Context(), owner, owner, repo, title, issueBody, buildGitHubIssueLabels(mapping.GithubLabel, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "engineering CreateGitIssue failed", "userID", user.UserID, "caseID", caseID, "repo", owner+"/"+repo, "err", err)
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
		slog.WarnContext(ctx, "failed to record GitHub issue work note", "userID", user.UserID, "caseID", caseID, "err", err)
	}
}
