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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The GitHub issue body and labels of a case's product issue. Moved (copied; the
// portal backend keeps its own until its flag-off path is removed) from
// apps/csm-portal/backend/internal/handler/case_github_issue.go. Two label
// changes: Patch adds Type/Patch only (the extra "patch" label is gone) and
// onboarding adds Affected/Onboarding, the label names the product teams use.
const (
	regressionLabel          = "regression"
	originLabel              = "Origin/CS"
	patchIssueTypeLabel      = "Type/Patch"
	discussionIssueTypeLabel = "Type/Discussion"
	incidentIssueTypeLabel   = "Type/Incident"
	hotfixLabel              = "Require/Hotfix"
	migrationLabel           = "Affected/Migration"
	onboardingLabel          = "Affected/Onboarding"

	priorityCriticalLabel = "Priority/Critical(P1)"
	priorityHighLabel     = "Priority/High(P2)"
	priorityMediumLabel   = "Priority/Medium(P3)"
)

// issueInfoBlankValue stands in for a footer value that is not known: the word the
// earlier Git issues carry there, so the block reads the same as theirs.
const issueInfoBlankValue = "undefined"

// productIssueFooterContext is what the issue body's info block needs beyond the
// request itself. A field left "" is written as "undefined", never dropped.
type productIssueFooterContext struct {
	// Product is "<abbreviation>-<version>" of the case's deployed product,
	// else its display name, else the catalogue name; the builder falls back to
	// the repository mapping's label when it is empty.
	Product    string
	Number     string
	InternalID string
	// Team is the ABT team: the account's CRE team, else the case's assigned team.
	Team string
	// OpenedBy is the engineer filing the issue (the caller's email).
	OpenedBy string
}

// buildProductIssueBody is the caller's own description, then the optional
// Hotfix Required line, then the info block as the footer: always the same
// seven lines (Product, Update level, WSO2 case id, Case number, Public Git
// Issue, ABT Team, Opened by), in this order and spelling, as the issues filed
// before the portal moved to the engineering service carry it. A value that is
// not known is written as "undefined", never dropped, so the block keeps one shape
// whatever the engineer filled in and whatever the case holds. The block goes
// last so the issue opens with what the engineer wrote, and so the Git
// Internals dashboard, which reads the last "ABT Team"/"Opened by" line of a
// body as the automated footer, is not overridden by a line quoted inside the
// description. Workflows in the product repositories parse these lines, so
// their spelling and order must not change.
func buildProductIssueBody(req domain.CreateProductWorkItemRequest, productLabel string, caseCtx productIssueFooterContext) string {
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

	var b strings.Builder
	section := func(value string) {
		if value == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(value)
	}
	section(strings.TrimSpace(req.Description))
	if req.HotFixRequired {
		section("Hotfix Required : Yes")
	}
	section(info.String())
	return strings.TrimSpace(b.String())
}

// footerLine renders one info block line the way buildProductIssueBody does, so
// the adoption search matches exactly what a create wrote.
func footerLine(label, value string) string {
	if value = strings.Join(strings.Fields(value), " "); value == "" {
		value = issueInfoBlankValue
	}
	return label + " : " + value
}

// githubPriorityLabel maps the dialog's severity onto the GitHub label names
// the internals dashboards filter on. Bare Priority/Critical (etc.) is
// rewritten to the (Pn) form; the (Pn) form is left as-is.
func githubPriorityLabel(level string) string {
	switch strings.TrimSpace(level) {
	case "Priority/Critical", priorityCriticalLabel:
		return priorityCriticalLabel
	case "Priority/High", priorityHighLabel:
		return priorityHighLabel
	case "Priority/Medium", priorityMediumLabel:
		return priorityMediumLabel
	default:
		return strings.TrimSpace(level)
	}
}

// buildProductIssueLabels applies the sheet's rules. Origin/CS and the product
// label always go on. Patch adds Type/Patch only. Discussion adds
// Type/Incident and the (Pn) priority label. The switches add Require/Hotfix,
// regression, and Affected/Migration. An in-progress project adds
// Affected/Onboarding.
func buildProductIssueLabels(productLabel string, req domain.CreateProductWorkItemRequest) []string {
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
	switch strings.TrimSpace(req.IssueTypeLabel) {
	case patchIssueTypeLabel:
		add(patchIssueTypeLabel)
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

// productIssuePriority maps a priority label to the stored priority:
// Priority/Critical* and Priority/Highest* -> P1, Priority/High* -> P2,
// Priority/Medium* and Priority/Normal* -> P3, and a "(Pn)" suffix is taken at its
// word. An unknown label is recorded as sent (trimmed), and starts no clock; an
// empty one is "".
func productIssuePriority(level string) string {
	trimmed := strings.TrimSpace(level)
	l := strings.ToLower(strings.ReplaceAll(trimmed, " ", ""))
	switch {
	case strings.HasPrefix(l, "priority/critical"), strings.HasPrefix(l, "priority/highest"), strings.HasSuffix(l, "(p1)"):
		return "P1"
	case strings.HasPrefix(l, "priority/high"), strings.HasSuffix(l, "(p2)"):
		return "P2"
	case strings.HasPrefix(l, "priority/medium"), strings.HasPrefix(l, "priority/normal"), strings.HasSuffix(l, "(p3)"):
		return "P3"
	}
	return trimmed
}

// ProductIssueFooterInfo is what the footer block of an issue body says about
// the issue, as buildProductIssueBody writes it. A field is "" when the footer
// has no such line or its value is "undefined".
type ProductIssueFooterInfo struct {
	OpenedBy       string
	ABTTeam        string
	PublicIssueURL string
}

// ParseProductIssueFooter reads the "Label : value" footer lines of an issue
// body. Each label is taken from its LAST occurrence (the footer is at the end,
// and an earlier line may be the reporter's own text), so a last "undefined"
// means none even when an earlier line had a value. A line is split at its first
// " : ", so a value may contain one; CRLF bodies are handled. Other labels are
// ignored. It never fails: a body with no footer gives the zero value.
func ParseProductIssueFooter(body string) ProductIssueFooterInfo {
	var info ProductIssueFooterInfo
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")
		label, value, ok := strings.Cut(line, " : ")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.EqualFold(value, issueInfoBlankValue) {
			value = ""
		}
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "opened by":
			info.OpenedBy = value
		case "abt team":
			info.ABTTeam = value
		case "public git issue":
			info.PublicIssueURL = value
		}
	}
	return info
}

// issuePriorityFromLabels is the product issue priority (P1/P2/P3) named by an
// issue's Priority/... label, "" when there is none. An unrecognised
// priority label is kept as written, as productIssuePriority does.
func issuePriorityFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "priority/") {
			return productIssuePriority(l)
		}
	}
	return ""
}

// issueCategoryFromLabels is the issue's Type/... label ("Type/Discussion"), the
// category the create path stores; "" when it has none.
func issueCategoryFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "type/") {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// isPatchIssue reports whether the labels mark a patch issue, which is not
// tracked (patches have no work item yet): Type/Patch or the legacy "patch".
func isPatchIssue(labels []string) bool {
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if strings.EqualFold(l, patchIssueTypeLabel) || strings.EqualFold(l, "patch") {
			return true
		}
	}
	return false
}
