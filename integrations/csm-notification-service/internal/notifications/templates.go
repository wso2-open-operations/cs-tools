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
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
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

// safeLinkSchemes is the allow-list of URL schemes sanitizeRichText permits
// on an <a href>. Everything else (javascript:, data:, vbscript:, a bare
// relative path with no scheme, ...) drops the tag — the link's own text
// still renders, just not as a clickable link — since none of those are
// both meaningful and safe to click from inside an email.
var safeLinkSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
	"tel":    true,
}

// safeImageDataURI matches a self-contained base64-encoded image data URI —
// the ONLY form of <img src> sanitizeRichText allows through. Never an
// http(s) URL: an externally-hosted image would have the recipient's mail
// client fetch it the moment the email is opened, a classic tracking-pixel/
// read-receipt leak (their IP, mail client, and open time, all revealed to
// whoever controls that URL) that a comment's own author could embed
// without the recipient ever knowing. The portal's own rich-text editor
// only ever produces a data: URI for an inserted image in the first place
// (apps/customer-portal/webapp's richTextEditor.tsx), so this loses no real
// functionality.
var safeImageDataURI = regexp.MustCompile(`(?i)^data:image/[a-z0-9.+-]+;base64,[a-z0-9+/]+=*$`)

// isSafeLinkHref reports whether href is safe to render as a clickable
// <a href> in an email — see safeLinkSchemes' own doc comment. An
// unparseable value is never safe.
func isSafeLinkHref(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	return safeLinkSchemes[strings.ToLower(u.Scheme)]
}

// openTag is one entry of sanitizeRichText's stack — name is the source
// tag this entry was pushed for (used to match against a later close tag),
// out is what to write when that close tag arrives ("" for a tag that was
// dropped, so its close is silently dropped too).
type openTag struct {
	name string
	out  string
}

// drainAttrs consumes every attribute of the current tag token without
// using any of them — the tokenizer requires every attribute to be read
// before the next Next() call can advance past this tag.
func drainAttrs(z *xhtml.Tokenizer, hasAttr bool) {
	for hasAttr {
		_, _, hasAttr = z.TagAttr()
	}
}

// sanitizeRichText converts rich-text HTML (as the portal's comment/
// description editors produce it, or ServiceNow returns it — e.g.
// `<p><span style="white-space: pre-wrap;">some text</span></p>`) into a
// safe HTML fragment for embedding in an email body: structure (paragraphs,
// line breaks, lists), basic formatting (bold/italic/underline), hyperlinks,
// and inline images are preserved; everything else is dropped down to its
// own inner text.
//
// Uses a real HTML tokenizer (golang.org/x/net/html), not a regex — a
// hand-rolled regex sanitizer can't reliably reject malformed/adversarial
// markup the way a real parser does, and deciding what's safe to let
// through is this function's whole job: comment/description text is
// caller-supplied (a customer's own case comment), not trusted input.
//
// Allow-list, deliberately narrow — widen it only for a tag/attribute this
// pipeline actually needs to render, never speculatively:
//   - p, div, h1..h6: no output on open; their close renders as "<br>" —
//     real HTML email clients render nested block tags inconsistently, so
//     this stays intentionally flat rather than attempting real block
//     layout
//   - br: "<br>", handled directly as a void element (see the tokenizer
//     dispatch below for why this can't go through the generic open/close
//     stack the way p/div does)
//   - ul, ol, li: real <ul>/<ol>/<li> tags, so an inserted bullet/numbered
//     list actually renders as one instead of flattening to plain lines
//   - b, strong, i, em, u: preserved as themselves
//   - a: only when href resolves to a safeLinkSchemes scheme — every other
//     attribute is dropped, and an unsafe/unparseable href drops the tag
//     but keeps the link's own visible text
//   - img: only when src is a safeImageDataURI — see that var's own doc
//     comment for why http(s) is never allowed. alt is preserved if present
//
// Every other tag (span, font, table, script, ...) is dropped, keeping its
// inner text as plain (escaped) content — the same "no allow-list to get
// wrong" reasoning this function's stripped-down predecessor
// (plainTextFromHTML) always had for anything not explicitly listed above.
// A tag's own raw-text content (e.g. a <script> body) is never interpreted
// — the tokenizer hands it back as an ordinary text token, which is
// HTML-escaped like any other text, so it can only ever render as inert,
// visible text, never execute.
//
// A close tag is only honored when it matches the stack's own top entry —
// deliberately conservative: adversarial/malformed markup (a stray
// mismatched close tag) is only ever a cosmetic risk this way (e.g. a
// dangling unclosed <b> leaving the rest of the message bold), never a
// safety one, since every tag this function itself emits is one of the
// fixed, hardcoded strings above with properly escaped attribute values.
// trimBoundaryBreaks removes leading/trailing "<br>" runs (and any
// surrounding whitespace) from a sanitizeRichText result — the source's
// own leading/trailing paragraph or line break otherwise survives as a
// visible blank line at the very start/end of the rendered comment, the
// same leading/trailing blank line plainTextFromHTML's own TrimSpace used
// to absorb back when it operated on plain "\n" instead of "<br>".
func trimBoundaryBreaks(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasSuffix(s, "<br>") {
		s = strings.TrimSpace(s[:len(s)-len("<br>")])
	}
	for strings.HasPrefix(s, "<br>") {
		s = strings.TrimSpace(s[len("<br>"):])
	}
	return s
}

func sanitizeRichText(s string) string {
	z := xhtml.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	var stack []openTag

	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return trimBoundaryBreaks(b.String())

		case xhtml.TextToken:
			b.WriteString(escapeHTML(string(z.Text())))

		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)

			// Void elements never get a matching close tag from any real
			// source, self-closing slash or not — handling them here,
			// before the stack push below, is what keeps the stack in
			// sync with the tokenizer's own actual nesting depth.
			switch tag {
			case "br":
				drainAttrs(z, hasAttr)
				b.WriteString("<br>")
				continue
			case "img":
				var src, alt string
				for hasAttr {
					var key, val []byte
					key, val, hasAttr = z.TagAttr()
					switch string(key) {
					case "src":
						src = string(val)
					case "alt":
						alt = string(val)
					}
				}
				if safeImageDataURI.MatchString(src) {
					b.WriteString(`<img src="` + escapeHTML(src) + `" alt="` + escapeHTML(alt) + `" style="max-width:100%;height:auto;">`)
				}
				continue
			}

			switch tag {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6":
				drainAttrs(z, hasAttr)
				stack = append(stack, openTag{name: tag, out: "<br>"})
			case "li":
				drainAttrs(z, hasAttr)
				b.WriteString("<li>")
				stack = append(stack, openTag{name: tag, out: "</li>"})
			case "ul":
				drainAttrs(z, hasAttr)
				b.WriteString("<ul>")
				stack = append(stack, openTag{name: tag, out: "</ul>"})
			case "ol":
				drainAttrs(z, hasAttr)
				b.WriteString("<ol>")
				stack = append(stack, openTag{name: tag, out: "</ol>"})
			case "b", "strong", "i", "em", "u":
				drainAttrs(z, hasAttr)
				b.WriteString("<" + tag + ">")
				stack = append(stack, openTag{name: tag, out: "</" + tag + ">"})
			case "a":
				var href string
				for hasAttr {
					var key, val []byte
					key, val, hasAttr = z.TagAttr()
					if string(key) == "href" {
						href = string(val)
					}
				}
				if isSafeLinkHref(href) {
					b.WriteString(`<a href="` + escapeHTML(href) + `">`)
					stack = append(stack, openTag{name: tag, out: "</a>"})
				} else {
					stack = append(stack, openTag{name: tag})
				}
			default:
				drainAttrs(z, hasAttr)
				stack = append(stack, openTag{name: tag})
			}

		case xhtml.EndTagToken:
			name, _ := z.TagName()
			if len(stack) > 0 && stack[len(stack)-1].name == string(name) {
				out := stack[len(stack)-1].out
				stack = stack[:len(stack)-1]
				b.WriteString(out)
			}
		}
	}
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
// name and caseTitle are HTML-escaped as-is; caseComment goes through
// sanitizeRichText, which preserves the source comment's structure, basic
// formatting, links, and inline images (see that function's own doc
// comment for the exact allow-list) rather than stripping every tag.
// commentLink is the "Add Comment" call-to-action target; caseLink is the
// "View Case" link and the case-title link target. caseNumber is the
// case's human-readable reference (e.g. "CS0023001") — display-only,
// distinct from the caseLink URL, which already carries whatever id the
// portal needs.
func RenderCommentAddedEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink string) string {
	replacer := strings.NewReplacer(
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", sanitizeRichText(caseComment),
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
		"<!-- [CASE_COMMENT] -->", sanitizeRichText(caseComment),
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
// display-formatted (e.g. "High(S2)") — dispatch.emailSeverityLabel's
// concern, not this function's. This is deliberately a different label
// format from the matching Chat card (dispatch.severityLabelAndColor's
// "High (P2)") — email uses entity-service's own S0..S4 severity notation,
// Chat keeps its established P0..P4 convention; the two are not meant to
// match.
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
		"<!-- [DESCRIPTION] -->", sanitizeRichText(data.Description),
		"<!-- [INCIDENT_IMPACT_DESCRIPTION] -->", sanitizeRichText(data.IncidentImpactDescription),
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
		"<!-- [SHORT_DESCRIPTION] -->", sanitizeRichText(d.ShortDescription),
		"<!-- [DESCRIPTION] -->", sanitizeRichText(d.Description),
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
