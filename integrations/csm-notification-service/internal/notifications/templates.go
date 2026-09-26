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

package notifications

import (
	_ "embed"
	"html"
	"regexp"
	"strconv"
	"strings"
)

//go:embed templates/comment_added.html
var commentAddedTemplateRaw string

//go:embed templates/status_changed.html
var statusChangedTemplateRaw string

//go:embed templates/case_assigned.html
var caseAssignedTemplateRaw string

//go:embed templates/case_created.html
var caseCreatedTemplateRaw string

//go:embed templates/internal_note.html
var internalNoteTemplateRaw string

//go:embed templates/severity_changed.html
var severityChangedTemplateRaw string

//go:embed templates/cr_approval_requested.html
var crApprovalRequestedTemplateRaw string

//go:embed templates/cr_plan_date_notice.html
var crPlanDateNoticeTemplateRaw string

//go:embed templates/project_contact_invited_new.html
var projectContactInvitedNewTemplateRaw string

//go:embed templates/project_contact_invited_existing.html
var projectContactInvitedExistingTemplateRaw string

//go:embed templates/project_contact_invited_reminder.html
var projectContactInvitedReminderTemplateRaw string

// wso2LogoURL is WSO2's own official logo asset, served from wso2.cachefly.net
// (WSO2's public CDN for site assets — not third-party hosting). An earlier
// version embedded the logo as an inline base64 data: URI instead, avoiding
// any external fetch — but Gmail (and most major webmail clients) strip
// data: URI images from received HTML mail as a security measure, which is
// why the logo showed as a broken image regardless of how it was encoded.
// A cid:-referenced inline MIME attachment would avoid the external fetch
// too, but the internal email-sending service this client calls only
// supports plain Content-Disposition: attachment, not inline/Content-ID —
// so a real, fetchable URL is the only option that actually renders today.
// This CDN URL, not a self-hosted endpoint, was the explicit choice made
// over hosting the same bytes from this service's own endpoint, which
// isn't reachable from outside WSO2's network.
const wso2LogoURL = "https://wso2.cachefly.net/wso2/sites/all/image_resources/logos/WSO2-Logo-Black.png"

// bakeLogo substitutes wso2LogoURL into raw's <!-- [LOGO_SRC] --> placeholder.
// Done once per template at package init rather than on every Render* call,
// since the logo never varies between emails.
func bakeLogo(raw string) string {
	return strings.Replace(raw, "<!-- [LOGO_SRC] -->", wso2LogoURL, 1)
}

var (
	commentAddedTemplate        = bakeLogo(commentAddedTemplateRaw)
	statusChangedTemplate       = bakeLogo(statusChangedTemplateRaw)
	crApprovalRequestedTemplate = bakeLogo(crApprovalRequestedTemplateRaw)
	crPlanDateNoticeTemplate    = bakeLogo(crPlanDateNoticeTemplateRaw)
	caseAssignedTemplate        = bakeLogo(caseAssignedTemplateRaw)
	caseCreatedTemplate         = bakeLogo(caseCreatedTemplateRaw)
	internalNoteTemplate        = bakeLogo(internalNoteTemplateRaw)
	severityChangedTemplate     = bakeLogo(severityChangedTemplateRaw)

	projectContactInvitedNewTemplate      = bakeLogo(projectContactInvitedNewTemplateRaw)
	projectContactInvitedExistingTemplate = bakeLogo(projectContactInvitedExistingTemplateRaw)
	projectContactInvitedReminderTemplate = bakeLogo(projectContactInvitedReminderTemplateRaw)
)

// htmlBlockBoundary matches the tags plainTextFromHTML treats as line
// breaks — everything else just disappears when stripped, which would
// otherwise run a rich-text source's separate paragraphs together into one
// unreadable line.
var htmlBlockBoundary = regexp.MustCompile(`(?i)</p>|</h[1-6]>|<br\s*/?>|</div>|</li>`)

// htmlTag matches any remaining tag, stripped unconditionally — this is a
// blunt strip-everything approach, not a sanitizer with an allow-list, but
// that's what makes it safe: no tag ever survives to be interpreted, so
// there's no allow-list to get wrong.
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// plainTextFromHTML converts s from rich-text HTML (as ServiceNow returns
// case descriptions and comments — e.g.
// `<p><span style="white-space: pre-wrap;">some text</span></p>`) to plain
// text, so escapeMultiline below doesn't end up HTML-escaping the source's
// own tags into literal, visible "<p>" clutter in the rendered email. Runs
// before escaping, not instead of it: the result is still plain text that
// itself might contain "<"/"&" (e.g. someone literally typed "<3" in a
// comment), which escapeMultiline still needs to escape safely.
//
// s that was never HTML in the first place (no tags present) passes through
// unchanged other than entity-decoding, which is a no-op for plain text.
func plainTextFromHTML(s string) string {
	s = htmlBlockBoundary.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

// escapeHTML HTML-escapes s and additionally converts every non-ASCII rune
// to a numeric HTML character reference (e.g. "Ⓦ" -> "&#9424;"). A browser
// renders raw UTF-8 (e.g. a display name entity-service returns with a
// trailing "Ⓦ" marker) just fine, but the external email-sending service
// this client calls (a separate service, in another repo, reached over
// HTTP — see EmailClient) doesn't reliably preserve non-ASCII bytes through
// its own send path: a raw multi-byte character in the outgoing email
// arrives as "?" in the recipient's inbox. A numeric character reference is
// pure ASCII on the wire, so it survives regardless, and any HTML-capable
// mail client decodes it back to the original character on display.
func escapeHTML(s string) string {
	var b strings.Builder
	for _, r := range html.EscapeString(s) {
		if r > 127 {
			b.WriteString("&#")
			b.WriteString(strconv.Itoa(int(r)))
			b.WriteByte(';')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeMultiline HTML-escapes s (see escapeHTML) and converts its newlines
// to <br>, so free-text fields (a comment, a case description) keep their
// original line breaks when dropped into HTML.
func escapeMultiline(s string) string {
	return strings.ReplaceAll(escapeHTML(plainTextFromHTML(s)), "\n", "<br>")
}

// applyOptionalBlock handles a template section wrapped in
// "<!-- [BLOCK:<name>_START] -->"..."<!-- [BLOCK:<name>_END] -->": if value is
// empty, the whole section (markers included) is removed; otherwise only the
// markers are stripped, leaving the section's content in place for the
// caller's usual placeholder substitution. Returns tmpl unchanged if the
// markers aren't found.
func applyOptionalBlock(tmpl, name, value string) string {
	start := "<!-- [BLOCK:" + name + "_START] -->"
	end := "<!-- [BLOCK:" + name + "_END] -->"
	si := strings.Index(tmpl, start)
	ei := strings.Index(tmpl, end)
	if si == -1 || ei == -1 || ei < si {
		return tmpl
	}
	if strings.TrimSpace(value) == "" {
		return tmpl[:si] + tmpl[ei+len(end):]
	}
	return tmpl[:si] + tmpl[si+len(start):ei] + tmpl[ei+len(end):]
}

// RenderCommentAddedEmail fills in the "comment added" HTML email template.
// name and caseTitle are HTML-escaped as-is; caseComment is escaped via
// escapeMultiline, which also strips any rich-text HTML markup the source
// comment carries down to plain text first (see plainTextFromHTML) so raw
// tags never show up as literal clutter in the rendered email. commentLink
// is the "Add Comment" call-to-action target; caseLink is the "View Case"
// link and the case-title link target. caseNumber is the case's
// human-readable reference (e.g. "CS0023001") — display-only, distinct from
// the caseLink URL, which already carries whatever id the portal needs.
func RenderCommentAddedEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", escapeMultiline(caseComment),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(commentAddedTemplate)
}

// RenderInternalNoteEmail fills in the "internal note" HTML email
// template — used instead of RenderCommentAddedEmail for a work note (see
// events.CommentAddedPayload.IsInternalNote), matching an existing
// internal WSO2-support email format recipients (always wso2.com staff —
// see that field's own doc comment) are already used to: no "Re: <title>"
// strap (an internal note isn't "about" the case title the way a reply
// is), and caseNumber here is expected to be the case's WSO2CaseID
// (dispatch.handleCommentAdded's own concern which value to pass), not
// the ServiceNow CaseNumber every other template uses — the internal case
// reference is the one this audience actually recognizes.
func RenderInternalNoteEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", escapeMultiline(caseComment),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(internalNoteTemplate)
}

// RenderStatusChangedEmail fills in the "case status changed" HTML email
// template. caseLink is used both for the case-number link in the strap
// line and the "View Case" link; commentLink is the "Add Comment"
// call-to-action target. caseNumber — see RenderCommentAddedEmail's own doc
// comment.
func RenderStatusChangedEmail(caseNumber, newStatus, caseLink, commentLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [NEW_STATUS] -->", escapeHTML(newStatus),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(statusChangedTemplate)
}

// RenderSeverityChangedEmail fills in the "case severity changed" HTML
// email template — structurally identical to RenderStatusChangedEmail
// (same strap-line-above-a-mostly-empty-card layout, same Add
// Comment/View Case links), just with oldSeverity/newSeverity in place of
// a single newStatus. oldSeverity/newSeverity are expected to already be
// display-formatted (e.g. "High (P2)") — dispatch.severityLabelAndColor's
// concern, not this function's — matching the Chat card's own severity
// labels so an email and its matching Chat alert read consistently.
func RenderSeverityChangedEmail(caseNumber, oldSeverity, newSeverity, caseLink, commentLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [OLD_SEVERITY] -->", escapeHTML(oldSeverity),
		"<!-- [NEW_SEVERITY] -->", escapeHTML(newSeverity),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(severityChangedTemplate)
}

// RenderCaseAssignedEmail fills in the "case assigned" HTML email template.
// assigneeEmail is rendered both as a mailto: link and as plain text.
// caseNumber — see RenderCommentAddedEmail's own doc comment.
func RenderCaseAssignedEmail(assigneeName, assigneeEmail, caseNumber, caseLink, commentLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [ASSIGNEE_NAME] -->", escapeHTML(assigneeName),
		"<!-- [ASSIGNEE_EMAIL] -->", escapeHTML(assigneeEmail),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(caseAssignedTemplate)
}

// CaseCreatedEmailData holds every value substituted into the "case created"
// HTML email template. IncidentImpactDescription is optional: when empty,
// its whole section is omitted from the output rather than rendering a
// placeholder like "null" or "N/A" for cases that don't have one (e.g.
// non-Incident case types).
type CaseCreatedEmailData struct {
	ReporterName string
	ProjectName  string
	// CaseNumber is the case's human-readable reference (e.g. "CS0023001")
	// — display-only, distinct from CaseLink's URL.
	CaseNumber                string
	CaseTitle                 string
	CaseType                  string
	Priority                  string
	Product                   string
	CreatedAt                 string
	Description               string
	IncidentImpactDescription string
	CaseLink                  string
	CommentLink               string
}

// RenderCaseCreatedEmail fills in the "case created" HTML email template.
func RenderCaseCreatedEmail(data CaseCreatedEmailData) string {
	tmpl := applyOptionalBlock(caseCreatedTemplate, "IMPACT", data.IncidentImpactDescription)
	replacer := strings.NewReplacer(
		"<!-- [REPORTER_NAME] -->", escapeHTML(data.ReporterName),
		"<!-- [PROJECT_NAME] -->", escapeHTML(data.ProjectName),
		"<!-- [CASE_NUMBER] -->", escapeHTML(data.CaseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(data.CaseTitle),
		"<!-- [CASE_TYPE] -->", escapeHTML(data.CaseType),
		"<!-- [PRIORITY] -->", escapeHTML(data.Priority),
		"<!-- [PRODUCT] -->", escapeHTML(data.Product),
		"<!-- [CREATED_AT] -->", escapeHTML(data.CreatedAt),
		"<!-- [DESCRIPTION] -->", escapeMultiline(data.Description),
		"<!-- [INCIDENT_IMPACT_DESCRIPTION] -->", escapeMultiline(data.IncidentImpactDescription),
		"<!-- [CASE_LINK] -->", escapeHTML(data.CaseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(data.CommentLink),
	)
	return replacer.Replace(tmpl)
}

// crStateLabels turn the domain state into the words a reader recognises. The
// raw values are ServiceNow's own (ASSESS, CUSTOMER_APPROVAL, ...), which are
// right for a payload and wrong for an email.
var crStateLabels = map[string]string{
	"ASSESS":            "Assess",
	"AUTHORIZE":         "Authorize",
	"REVIEW":            "Review",
	"CUSTOMER_APPROVAL": "Customer Approval",
	"CUSTOMER_REVIEW":   "Customer Review",
}

// CRApprovalEmailData holds every value substituted into the change-request
// approval template.
type CRApprovalEmailData struct {
	Number        string
	State         string
	Audience      string
	Team          string
	GroupName     string
	RequesterName string
	ProjectName   string
	Link          string
}

// RenderCRApprovalRequestedEmail fills in the "a change request needs your
// approval" template.
//
// The SUBJECT is not built here — it arrives already rendered on the payload,
// because csm-flow-service reproduces ServiceNow's per-branch wording verbatim
// and keeping a second copy of that in step would guarantee they drift.
func RenderCRApprovalRequestedEmail(d CRApprovalEmailData) string {
	state := crStateLabels[d.State]
	if state == "" {
		// An unmapped state is still worth sending: better a slightly raw word
		// in one line than no notice at all to someone waiting to approve.
		state = d.State
	}

	audience := "your approval"
	if d.GroupName != "" {
		audience = d.GroupName
	} else if d.Audience == "customer" {
		audience = "customer approval"
	}

	var context string
	switch {
	case d.ProjectName != "" && d.Team != "":
		context = "Project " + escapeHTML(d.ProjectName) + " · owned by " + escapeHTML(d.Team) + "."
	case d.ProjectName != "":
		context = "Project " + escapeHTML(d.ProjectName) + "."
	case d.Team != "":
		context = "Owned by " + escapeHTML(d.Team) + "."
	default:
		context = "Open the change request to review and act on it."
	}

	requester := d.RequesterName
	if requester == "" {
		requester = "Someone"
	}

	replacer := strings.NewReplacer(
		"<!-- [CR_NUMBER] -->", escapeHTML(d.Number),
		"<!-- [STATE_LABEL] -->", escapeHTML(state),
		"<!-- [AUDIENCE_LABEL] -->", escapeHTML(audience),
		"<!-- [REQUESTER] -->", escapeHTML(requester),
		"<!-- [CR_LINK] -->", escapeHTML(d.Link),
		"<!-- [CONTEXT_LINE] -->", context,
	)
	return replacer.Replace(crApprovalRequestedTemplate)
}

// CRPlanDateEmailData is what the plan-start-date notice renders from.
type CRPlanDateEmailData struct {
	// Kind is "customer_proposed", "accepted" or "rejected" — it selects both
	// the headline and the closing line.
	Kind             string
	Number           string
	ActorName        string
	ProjectName      string
	ShortDescription string
	Description      string
	Link             string
}

// crPlanDateWording is the per-kind text, reproduced from the ServiceNow
// templates verbatim — including "Reject the proposed plan start date" as a
// past-tense sentence and "<name> customer has updated…", both of which read
// oddly and are what the original sends.
var crPlanDateWording = map[string]struct{ headlineSuffix, closing string }{
	"customer_proposed": {
		"customer has updated the <b>plan start date</b>",
		"Customer has updated the plan start date. Please review the change.",
	},
	"accepted": {
		"accepted the plan start date",
		"The proposed plan start date accepted by the WSO2 Team.",
	},
	"rejected": {
		"Reject the proposed plan start date",
		"WSO2 Team request to change the plan start date.",
	},
}

// RenderCRPlanDateNoticeEmail renders one plan-start-date notice.
func RenderCRPlanDateNoticeEmail(d CRPlanDateEmailData) string {
	w, ok := crPlanDateWording[d.Kind]
	if !ok {
		// An unmapped kind still sends: a plain statement beats no notice at
		// all to someone waiting on a date.
		w.headlineSuffix = "updated the plan start date"
		w.closing = "Open the change request to review the change."
	}

	headline := escapeHTML(d.ActorName) + " " + w.headlineSuffix
	if d.ActorName == "" {
		// No resolvable actor: drop the empty leading space rather than
		// rendering " customer has updated…".
		headline = strings.ToUpper(w.headlineSuffix[:1]) + w.headlineSuffix[1:]
	}

	projectAndNumber := escapeHTML(d.Number)
	if d.ProjectName != "" {
		projectAndNumber = escapeHTML(d.ProjectName) + " / " + escapeHTML(d.Number)
	}

	replacer := strings.NewReplacer(
		"<!-- [CR_NUMBER] -->", escapeHTML(d.Number),
		"<!-- [HEADLINE] -->", headline,
		"<!-- [PROJECT_AND_NUMBER] -->", projectAndNumber,
		"<!-- [SHORT_DESCRIPTION] -->", escapeMultiline(d.ShortDescription),
		"<!-- [DESCRIPTION] -->", escapeMultiline(d.Description),
		"<!-- [CLOSING_LINE] -->", escapeHTML(w.closing),
		"<!-- [CR_LINK] -->", escapeHTML(d.Link),
	)
	return replacer.Replace(crPlanDateNoticeTemplate)
}

// ProjectContactInvitedEmailData holds every value substituted into the
// three project-invitation templates (RenderProjectContactInvitedNewEmail /
// RenderProjectContactInvitedExistingEmail /
// RenderProjectContactInvitedReminderEmail). DisplayName is already
// resolved by the caller (given + family name, or the email's local part
// when Salesforce has neither — dispatch.inviteeDisplayName's concern, not
// this package's). Roles are the raw Salesforce project roles; when empty
// the whole "Your role" line is omitted rather than rendered blank.
// PortalURL is the sign-in link target (ONBOARD_PORTAL_URL).
type ProjectContactInvitedEmailData struct {
	DisplayName string
	Email       string
	ProjectName string
	ProjectKey  string
	Roles       []string
	PortalURL   string
	// AccountCreated is true only when the identity step ran for this
	// record and created the account. The "new" template then says so;
	// otherwise it only explains how to sign in, making no claim about
	// whether an account exists.
	AccountCreated bool
}

// RenderProjectContactInvitedNewEmail fills in the invitation for a contact
// not known to already have a WSO2 account: you've been given access to the
// project, sign in with your email, first sign-in asks for an email code.
// With d.AccountCreated (internal/scim reported existed=false) it also says
// the account was created; with identity provisioning disabled it makes no
// claim either way.
func RenderProjectContactInvitedNewEmail(d ProjectContactInvitedEmailData) string {
	return renderProjectContactInvited(projectContactInvitedNewTemplate, d)
}

// RenderProjectContactInvitedExistingEmail fills in the invitation for a
// contact who already had a WSO2 account (internal/scim reported
// existed=true): the project has been added, sign in as usual.
func RenderProjectContactInvitedExistingEmail(d ProjectContactInvitedEmailData) string {
	return renderProjectContactInvited(projectContactInvitedExistingTemplate, d)
}

// RenderProjectContactInvitedReminderEmail fills in the short reminder sent
// when an admin deliberately resends an invitation
// (events.ProjectContactInvitedPayload.IsResend): here is your invitation
// again, sign in here. Deliberately says nothing about the account — by the
// time a resend goes out the Asgardeo user exists, but the reader may never
// have seen the first email, so "you already have a WSO2 account" would read
// as nonsense and "an account has been created for you" would be a second
// welcome. AccountCreated is ignored by this variant.
func RenderProjectContactInvitedReminderEmail(d ProjectContactInvitedEmailData) string {
	return renderProjectContactInvited(projectContactInvitedReminderTemplate, d)
}

// renderProjectContactInvited is the shared substitution all three
// invitation variants use — they differ only in wording, not in
// placeholders (the reminder simply has no ACCOUNT_* blocks to fill).
func renderProjectContactInvited(tmpl string, d ProjectContactInvitedEmailData) string {
	roles := strings.Join(d.Roles, ", ")
	tmpl = applyOptionalBlock(tmpl, "ROLES", roles)
	created, unknown := "", "x"
	if d.AccountCreated {
		created, unknown = "x", ""
	}
	tmpl = applyOptionalBlock(tmpl, "ACCOUNT_CREATED", created)
	tmpl = applyOptionalBlock(tmpl, "ACCOUNT_UNKNOWN", unknown)
	replacer := strings.NewReplacer(
		"<!-- [DISPLAY_NAME] -->", escapeHTML(d.DisplayName),
		"<!-- [EMAIL] -->", escapeHTML(d.Email),
		"<!-- [PROJECT_NAME] -->", escapeHTML(d.ProjectName),
		"<!-- [PROJECT_KEY] -->", escapeHTML(d.ProjectKey),
		"<!-- [ROLES] -->", escapeHTML(roles),
		"<!-- [PORTAL_URL] -->", escapeHTML(d.PortalURL),
	)
	return replacer.Replace(tmpl)
}
