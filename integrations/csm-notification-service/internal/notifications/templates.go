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

//go:embed templates/mention.html
var mentionTemplateRaw string

//go:embed templates/internal_mention.html
var internalMentionTemplateRaw string

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
	commentAddedTemplate    = bakeLogo(commentAddedTemplateRaw)
	statusChangedTemplate   = bakeLogo(statusChangedTemplateRaw)
	caseAssignedTemplate    = bakeLogo(caseAssignedTemplateRaw)
	caseCreatedTemplate     = bakeLogo(caseCreatedTemplateRaw)
	internalNoteTemplate    = bakeLogo(internalNoteTemplateRaw)
	severityChangedTemplate = bakeLogo(severityChangedTemplateRaw)
	mentionTemplate         = bakeLogo(mentionTemplateRaw)
	internalMentionTemplate = bakeLogo(internalMentionTemplateRaw)
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

// RenderMentionEmail fills in the "you were mentioned" HTML email template —
// the customer/CSM-facing layout for events.TypeCaseMentioned, used when the
// mention happened in a public comment (see events.CaseMentionedPayload.
// IsInternalNote). name is the mentioner (who @mentioned the recipient), not
// the recipient themselves — dispatch.handleCaseMentioned resolves one
// portal link per recipient the same way handleCommentAdded does, so this is
// called once per resolved link group, same as RenderCommentAddedEmail.
// Parameter order/meaning otherwise mirrors RenderCommentAddedEmail exactly.
func RenderMentionEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", escapeMultiline(caseComment),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(mentionTemplate)
}

// RenderInternalMentionEmail fills in the "you were mentioned" HTML email
// template's internal-note variant — used instead of RenderMentionEmail when
// the mention happened inside a work note (see events.CaseMentionedPayload.
// IsInternalNote), matching RenderInternalNoteEmail's own internal-note
// conventions: caseNumber here is expected to be the case's WSO2CaseID, not
// the ServiceNow CaseNumber, same reasoning as RenderInternalNoteEmail's own
// doc comment.
func RenderInternalMentionEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", escapeMultiline(caseComment),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(internalMentionTemplate)
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
