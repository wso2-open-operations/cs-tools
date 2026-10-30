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
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

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

//go:embed templates/project_contact_registered.html
var projectContactRegisteredTemplate string

//go:embed templates/project_reactivated.html
var projectReactivatedTemplateRaw string

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

	projectReactivatedTemplate = bakeLogo(projectReactivatedTemplateRaw)
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
// the ONLY form of <img src> sanitizeRichText accepts as a source image.
// Never an http(s) URL: an externally-hosted image would have the
// recipient's mail client fetch it the moment the email is opened, a
// classic tracking-pixel/read-receipt leak (their IP, mail client, and open
// time, all revealed to whoever controls that URL) that a comment's own
// author could embed without the recipient ever knowing. The portal's own
// rich-text editor only ever produces a data: URI for an inserted image in
// the first place (apps/customer-portal/webapp's richTextEditor.tsx), so
// this loses no real functionality. A matching image is never re-embedded
// as a data: URI in the output HTML, though — see InlineImage below.
//
// The media-type component is a fixed allow-list of raster formats
// (png/jpeg/jpg/gif/webp), deliberately not a general "image/*" wildcard —
// image/svg+xml is XML, not a raster format, and can carry a <script> tag
// or an onload= event handler that some mail clients execute when they
// render an inline image; a wildcard would have let a comment's own author
// smuggle active content in as an "image." inlineImageExtensions
// (dispatch.go) mirrors this exact list for its own reason (a file
// extension for the resulting EmailAttachment's ContentName) — keep both
// lists in sync if this one ever changes.
var safeImageDataURI = regexp.MustCompile(`(?i)^data:(image/(?:png|jpe?g|gif|webp));base64,([a-z0-9+/]+=*)$`)

// maxInlineImageBytes bounds one inline image's decoded size — without a
// cap, a single comment could embed an image large enough to bloat the
// outgoing email past email-service's own request-body limit (10MB
// default, README.md there) or meaningfully inflate this process's memory
// use while rendering. 5MB comfortably covers a real pasted screenshot
// (typically well under 1MB) with headroom to spare.
const maxInlineImageBytes = 5 * 1024 * 1024

// maxTotalInlineImageBytes bounds the combined decoded size of every image
// extracted toward ONE rendered email — see inlineImageBudget, which is
// what actually enforces this across however many sanitizeRichText calls
// that email's Render* function makes. maxInlineImageBytes alone doesn't
// prevent several images that are each individually within budget from
// still summing past email-service's own 10MB request-body limit once
// base64-re-encoded for the JSON attachments array (base64 inflates size
// by roughly 4/3 — two untouched 5MB images alone would already exceed
// it). 6MB of combined raw image bytes encodes to about 8MB, leaving
// headroom in that 10MB budget for the HTML body and JSON structure
// overhead.
const maxTotalInlineImageBytes = 6 * 1024 * 1024

// maxInlineImagesPerComment bounds how many images toward ONE rendered
// email inlineImageBudget will allow across however many sanitizeRichText
// calls that email's Render* function makes — a real comment realistically
// embeds one or two pasted screenshots, not dozens; this caps the worst
// case (a comment crafted to embed many images) rather than trusting input
// size alone. Once reached, every further <img> is dropped exactly like an
// unsafe one (logged nowhere, same as any other rejected tag) — silently,
// not an error, since a truncated comment still rendering is better than
// the whole email failing to send over one over-decorated comment.
const maxInlineImagesPerComment = 10

// inlineImageBudget tracks how much of maxTotalInlineImageBytes/
// maxInlineImagesPerComment has already been spent toward ONE rendered
// email. A caller that renders more than one rich-text field into the same
// email (RenderCaseCreatedEmail: Description + IncidentImpactDescription;
// RenderCRPlanDateNoticeEmail: ShortDescription + Description) must share a
// single *inlineImageBudget across both sanitizeRichText calls — a fresh
// budget per call would let each field independently max out, and the two
// fields' attachments still land on the same outgoing email/request. A
// caller with only one rich-text field just constructs one and uses it
// once.
type inlineImageBudget struct {
	totalBytes int
	count      int
}

// allow reports whether one more image of size decoded bytes still fits
// within this budget, and — only if so — reserves the space by updating
// the running totals. Checking and reserving in one call keeps this
// correct even though sanitizeRichText's own <img> branch calls it once
// per candidate image with no other synchronization.
func (b *inlineImageBudget) allow(size int) bool {
	if b.count >= maxInlineImagesPerComment || size > maxInlineImageBytes || b.totalBytes+size > maxTotalInlineImageBytes {
		return false
	}
	b.count++
	b.totalBytes += size
	return true
}

// InlineImage is one image sanitizeRichText extracted out of a data: URI
// <img> tag. The caller (dispatch, via notifications.Render*) is
// responsible for sending it as a real MIME attachment with
// Content-Disposition: inline and this exact ContentID (EmailClient's
// EmailAttachment carries both) — the returned HTML only ever contains a
// short <img src="cid:<contentId>"> reference, never the original data:
// URI. This exists because Gmail (and most major webmail clients) strip a
// data: image src from received HTML on render, regardless of how
// correctly it's encoded — the same reason internal/notifications' own
// wso2LogoURL switched the WSO2 logo off a baked-in data: URI. A proper
// cid:-referenced MIME part is the universally-supported way any mail
// client renders an inline image; entity-service has no way to produce
// one, hence this extraction step. See ServiceNow's own outbound mail (a
// separate, native pipeline unrelated to this one) for a real example of
// exactly this MIME shape — multipart/related, an inline part with a
// Content-ID, and an <img src="cid:..."> reference.
type InlineImage struct {
	// ContentID is this image's Content-ID, without the RFC 2392 angle
	// brackets (EmailClient adds those, matching how it wraps the id when
	// building the actual header) — a value unique within THIS rendered
	// email, matching the cid: reference sanitizeRichText wrote in its own
	// HTML output. Generated by nextInlineImageContentID, never
	// caller-supplied.
	ContentID string
	// ContentType is the image's MIME type (e.g. "image/png"), read
	// straight out of the data: URI's own media-type component.
	ContentType string
	// Data is the decoded (no longer base64) image bytes.
	Data []byte
}

// inlineImageContentIDDomain is the domain component nextInlineImageContentID
// appends to every Content-ID it generates — not a real, resolvable
// hostname, just a fixed placeholder identifying this service as the
// minting system. email-service's own handler requires an inline
// attachment's contentId to be shaped like RFC 2392's addr-spec
// (local-part@domain) and rejects a bare token outright, so this can't be
// left off.
const inlineImageContentIDDomain = "csm-notification-service.internal"

// inlineImageSeq is a process-wide counter backing nextInlineImageContentID
// — atomic since sanitizeRichText can run concurrently across dispatch's
// own concurrent Handle calls (see dispatch.go's own concurrency notes).
// Only cross-call uniqueness matters (a Content-ID only has to be unique
// within the one email it's attached to), but a shared counter is the
// simplest way to guarantee that without adding a randomness source purely
// for this.
var inlineImageSeq int64

// nextInlineImageContentID returns a new, process-wide-unique Content-ID
// for one extracted InlineImage, already shaped as RFC 2392's addr-spec —
// see inlineImageContentIDDomain's own doc comment for why. This exact
// string is used both as the value written into the returned HTML's own
// cid: reference and as the EmailAttachment.ContentID eventually sent to
// email-service — the two must always match verbatim, so this is the one
// and only place a Content-ID is minted.
func nextInlineImageContentID() string {
	return fmt.Sprintf("inline-image-%d@%s", atomic.AddInt64(&inlineImageSeq, 1), inlineImageContentIDDomain)
}

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

	// cell marks a table cell (td/th); cellStart is the output offset just
	// past its open tag, so the cell's close can trim the content written
	// since — see closeTopTag in sanitizeRichText.
	cell      bool
	cellStart int
}

// The fixed tags sanitizeRichText emits for a table. The source's own table
// attributes and inline styles are never carried over (same as every other
// tag here) — these hardcoded strings are the whole output, so there is
// nothing attacker-supplied in them. Borders, padding and the header
// background are spelled out inline per cell, not in a <style> block: mail
// clients (Gmail especially) drop or ignore a fragment-level stylesheet, and
// Outlook only honors padding/borders set on the cell itself.
const (
	emailTableOpen = `<table cellspacing="0" cellpadding="0" style="border-collapse:collapse;margin:8px 0;">`
	emailTHOpen    = `<th align="left" valign="top" style="border:1px solid #d4d4d8;padding:6px 10px;background-color:#f4f4f5;">`
	emailTDOpen    = `<td valign="top" style="border:1px solid #d4d4d8;padding:6px 10px;">`

	// The look of the editor's own code/quote blocks (see its stylesheet in
	// apps/csm-portal/webapp/src/components/rich-text-editor/Editor.tsx),
	// translated to what mail clients honor: inline styles, px units (Outlook
	// ignores rem), a font stack rather than "monospace" alone (which some
	// clients render a size too small).
	emailCodeOpen  = `<code style="font-family:Consolas,Menlo,'Courier New',monospace;font-size:0.9em;background-color:#f4f4f5;padding:1px 4px;border-radius:3px;">`
	emailPreOpen   = `<pre style="margin:8px 0;padding:8px 16px;font-family:Consolas,Menlo,'Courier New',monospace;font-size:13px;line-height:1.5;background-color:#f4f4f5;border:1px solid #d4d4d8;border-radius:6px;white-space:pre-wrap;">`
	emailQuoteOpen = `<blockquote style="margin:8px 0;padding:4px 12px;border-left:3px solid #d4d4d8;color:#52525b;">`
)

// emailHeadingSizes is the font size each heading level renders at. The body
// text is 14px, so every level is bold and steps down toward it.
var emailHeadingSizes = map[string]string{
	"h1": "26px", "h2": "22px", "h3": "19px", "h4": "17px", "h5": "15px", "h6": "14px",
}

// textAlignRe finds a non-default text-align in a block's style attribute.
// left/start is the default, so only the three values that change the layout
// are worth carrying over — and only the keyword, never the raw style string.
var textAlignRe = regexp.MustCompile(`(?i)(?:^|;)\s*text-align\s*:\s*(center|right|justify)\s*(?:;|$)`)

// blockAttrs consumes every attribute of the current block tag (see
// drainAttrs) and returns its alignment — "center", "right" or "justify" — or
// "" when it has none. The returned value is always one of those three fixed
// lowercase words, never source text.
func blockAttrs(z *xhtml.Tokenizer, hasAttr bool) string {
	align := ""
	for hasAttr {
		var key, val []byte
		key, val, hasAttr = z.TagAttr()
		if string(key) == "style" {
			if m := textAlignRe.FindSubmatch(val); m != nil {
				align = strings.ToLower(string(m[1]))
			}
		}
	}
	return align
}

// emailStyleProperties is the allow-list for an announcement's inline styles:
// presentation only, nothing that positions, hides or loads anything.
var emailStyleProperties = map[string]bool{
	"color": true, "background": true, "background-color": true,
	"border": true, "border-top": true, "border-right": true, "border-bottom": true, "border-left": true,
	"border-color": true, "border-style": true, "border-width": true, "border-radius": true,
	"padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true,
	"margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true,
	"font-weight": true, "font-style": true, "font-size": true, "line-height": true,
	"text-align": true, "text-decoration": true, "box-shadow": true,
	"width": true, "max-width": true,
}

// emailStyleValue is what a style value may be made of: colours, lengths,
// keywords and function arguments (rgba(...), linear-gradient(...)). No quotes,
// semicolons, colons, backslashes, braces, angle brackets or "@", so a value
// can neither leave its declaration nor its attribute.
var emailStyleValue = regexp.MustCompile(`^[#A-Za-z0-9\s.,%()+\-/!]{1,200}$`)

// emailStyleDenied are substrings that make a value unsafe even though it
// passes emailStyleValue: anything that fetches a resource or runs script.
var emailStyleDenied = []string{"url(", "expression", "javascript", "image-set", "var(", "attr(", "behavior", "-moz-binding"}

// safeInlineStyle reduces a style attribute to the declarations in
// emailStyleProperties whose values pass emailStyleValue, rebuilt as
// "name:value;" pairs. Anything else is dropped, never repaired.
func safeInlineStyle(raw string) string {
	var out strings.Builder
	for _, decl := range strings.Split(raw, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if !emailStyleProperties[name] || !emailStyleValue.MatchString(value) {
			continue
		}
		lower := strings.ToLower(value)
		denied := false
		for _, bad := range emailStyleDenied {
			if strings.Contains(lower, bad) {
				denied = true
				break
			}
		}
		if denied {
			continue
		}
		out.WriteString(name + ":" + value + ";")
	}
	return out.String()
}

// hasPresentationStyles reports whether s has a div, p, span or a whose style
// attribute keeps, after safeInlineStyle, anything beyond text-align (which the
// rich editor itself produces and the plain rules already handle).
func hasPresentationStyles(s string) bool {
	if !strings.Contains(s, "style") {
		return false
	}
	z := xhtml.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return false
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "div", "p", "span", "a":
			default:
				drainAttrs(z, hasAttr)
				continue
			}
			for hasAttr {
				var key, val []byte
				key, val, hasAttr = z.TagAttr()
				if string(key) != "style" {
					continue
				}
				for _, decl := range strings.Split(safeInlineStyle(string(val)), ";") {
					if prop, _, ok := strings.Cut(decl, ":"); ok && prop != "text-align" {
						return true
					}
				}
			}
		}
	}
}

// inlineStyleAttr consumes the current tag's attributes and returns its
// safeInlineStyle (every other attribute is dropped).
func inlineStyleAttr(z *xhtml.Tokenizer, hasAttr bool) string {
	style := ""
	for hasAttr {
		var key, val []byte
		key, val, hasAttr = z.TagAttr()
		if string(key) == "style" {
			style = safeInlineStyle(string(val))
		}
	}
	return style
}

// styleAttrHTML is the ` style="..."` attribute for an already-sanitised style,
// or nothing when there is none.
func styleAttrHTML(style string) string {
	if style == "" {
		return ""
	}
	return ` style="` + escapeHTML(style) + `"`
}

// voidElements are the HTML elements that never have a close tag. The
// tokenizer reports them as plain start tags, so without an explicit case a
// <col>/<hr>/... would be pushed on sanitizeRichText's stack and never
// popped — leaving it on top, where every later close tag (</table> after a
// <colgroup><col>, the editors' own table output) fails the top-of-stack
// match and is dropped. br and img are handled before the stack entirely
// (they emit output), so they are not listed here.
var voidElements = map[string]bool{
	"area": true, "base": true, "col": true, "embed": true, "hr": true,
	"input": true, "link": true, "meta": true, "param": true, "source": true,
	"track": true, "wbr": true,
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
// own inner text. Also returns every InlineImage it extracted along the
// way — the caller (a Render* function, ultimately dispatch) is
// responsible for sending each one back to EmailClient.SendEmail as an
// inline EmailAttachment, since the returned HTML only ever contains a
// short cid: reference, never the original data: URI. budget tracks how
// much of maxTotalInlineImageBytes/maxInlineImagesPerComment remains for
// the email this call's result will end up part of — see
// inlineImageBudget's own doc comment for why a caller rendering more than
// one rich-text field into the same email must share a single budget
// across every sanitizeRichText call it makes, rather than passing a fresh
// one each time.
//
// Uses a real HTML tokenizer (golang.org/x/net/html), not a regex — a
// hand-rolled regex sanitizer can't reliably reject malformed/adversarial
// markup the way a real parser does, and deciding what's safe to let
// through is this function's whole job: comment/description text is
// caller-supplied (a customer's own case comment), not trusted input.
//
// Allow-list, deliberately narrow — widen it only for a tag/attribute this
// pipeline actually needs to render, never speculatively:
//   - p, div: no output on open; their close renders as "<br>" — real HTML
//     email clients render nested block tags inconsistently, so this stays
//     intentionally flat rather than attempting real block layout. The one
//     exception is a center/right/justify text-align, which needs a real
//     block to carry it (see blockAttrs)
//   - h1..h6: a real heading tag with a fixed size/weight/margin
//     (emailHeadingSizes), plus the same text-align keyword — without this a
//     "Impact"/"Solution" heading arrives as an ordinary line of text
//   - blockquote, pre, code: fixed-style block/inline tags
//     (emailQuoteOpen/emailPreOpen/emailCodeOpen) mirroring the editor's own
//     look; without them a code block loses its monospace and spacing
//   - br: "<br>", handled directly as a void element (see the tokenizer
//     dispatch below for why this can't go through the generic open/close
//     stack the way p/div does)
//   - ul, ol, li: real <ul>/<ol>/<li> tags, so an inserted bullet/numbered
//     list actually renders as one instead of flattening to plain lines
//   - b, strong, i, em, u, s: preserved as themselves (s is strikethrough)
//   - table, tr, th, td: real table tags with fixed inline styling
//     (emailTableOpen/emailTHOpen/emailTDOpen) — the source's own
//     attributes and styles are dropped like everywhere else. A product/
//     version matrix flattened into one value per line is unreadable, and
//     the announcement flow's editors produce exactly that. thead/tbody/
//     tfoot/caption/colgroup are dropped (their rows and text still render);
//     each cell's own leading/trailing paragraph breaks are trimmed so a
//     row doesn't grow an empty line
//   - a: only when href resolves to a safeLinkSchemes scheme — every other
//     attribute is dropped, and an unsafe/unparseable href drops the tag
//     but keeps the link's own visible text
//   - img: only when src is a safeImageDataURI — see that var's own doc
//     comment for why http(s) is never allowed. alt is preserved if
//     present; src itself is never kept as a data: URI — see InlineImage
//
// Anything the editor can't produce and the pipeline doesn't need — span
// styling (font size, colors), font, script, ... — is dropped, keeping its
// inner text as plain (escaped) content — the same "no allow-list to get
// wrong" reasoning this function's stripped-down predecessor
// (plainTextFromHTML) always had for anything not explicitly listed above.
// A tag's own raw-text content (e.g. a <script> body) is never interpreted
// — the tokenizer hands it back as an ordinary text token, which is
// HTML-escaped like any other text, so it can only ever render as inert,
// visible text, never execute.
//
// HTML void elements (col, hr, ...) never touch the stack — see voidElements.
// Tags still open when the input ends are closed in reverse order, so an
// unclosed <table> can't swallow the rest of the template.
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

func sanitizeRichText(s string, budget *inlineImageBudget) (string, []InlineImage) {
	return sanitizeRichTextMode(s, budget, false)
}

// sanitizeAnnouncementRichText is sanitizeRichText for an announcement's body,
// which is authored as styled HTML (a bordered, shaded advisory box, coloured
// links, spaced paragraphs) rather than typed into the comment editor. It adds
// to the allow-list above, and only for announcements:
//   - p, div, span and a keep a style attribute reduced to safeInlineStyle's
//     allow-list of presentation properties (no url(), no positioning)
//   - p and div become real blocks (<p> with a bottom margin, <div>) instead
//     of closing as "<br>", so the box and the spacing between paragraphs show
//
// Everything else, including every security rule above (link schemes, inline
// images, unknown tags reduced to their text), is unchanged.
func sanitizeAnnouncementRichText(s string, budget *inlineImageBudget) (string, []InlineImage) {
	return sanitizeRichTextMode(s, budget, true)
}

func sanitizeRichTextMode(s string, budget *inlineImageBudget, keepStyles bool) (string, []InlineImage) {
	z := xhtml.NewTokenizer(strings.NewReader(s))
	var b bytes.Buffer
	var stack []openTag
	var images []InlineImage

	// closeTopTag pops the stack's top entry and writes its close. A table
	// cell first drops the line breaks its own paragraphs left at either
	// edge (every <p> closes as "<br>", so a cell like <td><p>13</p></td>
	// would otherwise render with an empty line under the value). Only the
	// cell's own content is copied, so this stays linear however many cells
	// the table has.
	closeTopTag := func() {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if top.cell {
			cell := trimBoundaryBreaks(string(b.Bytes()[top.cellStart:]))
			b.Truncate(top.cellStart)
			b.WriteString(cell)
		}
		b.WriteString(top.out)
	}

	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			// Close whatever the source left open (an unclosed <table>, <b>,
			// <a>, ...). Every open tag this function emitted must get its
			// close, or it would swallow the rest of the email template the
			// fragment is embedded in — the footer's "Add Comment"/"View
			// Case" links would end up inside the unclosed table.
			for len(stack) > 0 {
				closeTopTag()
			}
			return trimBoundaryBreaks(b.String()), images

		case xhtml.TextToken:
			b.WriteString(escapeHTML(string(z.Text())))

		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)

			// Void elements never get a matching close tag from any real
			// source, self-closing slash or not — handling them here,
			// before the stack push below, is what keeps the stack in
			// sync with the tokenizer's own actual nesting depth.
			if voidElements[tag] {
				drainAttrs(z, hasAttr)
				continue
			}

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
				// A base64 payload that fails to decode, or one budget.allow
				// rejects (too big on its own, or would push this email's
				// shared running total/count past its caps), is dropped
				// silently, same as any other rejected <img> — the regex
				// already rejected anything not shaped like valid base64,
				// so a decode failure here only ever catches an edge case
				// (e.g. non-canonical padding) the regex alone can't.
				if m := safeImageDataURI.FindStringSubmatch(src); m != nil {
					if data, err := base64.StdEncoding.DecodeString(m[2]); err == nil && budget.allow(len(data)) {
						contentID := nextInlineImageContentID()
						images = append(images, InlineImage{ContentID: contentID, ContentType: m[1], Data: data})
						// The surrounding <br>s plus display:block take the image
						// out of the inline line box it would otherwise share with
						// adjacent text spans. Left inline, some renderers
						// (confirmed: Outlook web/desktop) visually reorder a tall
						// inline image ahead of the text it was inserted after,
						// even though the underlying HTML keeps the original
						// text-then-image source order — a real reported bug.
						b.WriteString(`<br><img src="cid:` + contentID + `" alt="` + escapeHTML(alt) + `" style="display:block;max-width:100%;height:auto;margin:8px 0;"><br>`)
					}
				}
				continue
			}

			switch tag {
			case "p", "div":
				// A table cell keeps the plain rule: its own paragraph breaks are
				// trimmed so a row does not grow an empty line (see closeTopTag).
				inCell := false
				for _, open := range stack {
					inCell = inCell || open.cell
				}
				if keepStyles && !inCell {
					style := inlineStyleAttr(z, hasAttr)
					if tag == "p" {
						b.WriteString(`<p style="margin:0 0 12px;` + style + `">`)
						stack = append(stack, openTag{name: tag, out: "</p>"})
					} else {
						b.WriteString(`<div` + styleAttrHTML(style) + `>`)
						stack = append(stack, openTag{name: tag, out: "</div>"})
					}
				} else if align := blockAttrs(z, hasAttr); align != "" {
					// An aligned paragraph needs a real block to carry the
					// alignment; its trailing <br> inside the block adds no
					// extra line, so spacing matches an unaligned one.
					b.WriteString(`<div style="text-align:` + align + `;">`)
					stack = append(stack, openTag{name: tag, out: "<br></div>"})
				} else {
					stack = append(stack, openTag{name: tag, out: "<br>"})
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				align := blockAttrs(z, hasAttr)
				open := `<` + tag + ` style="margin:12px 0 4px;font-size:` + emailHeadingSizes[tag] + `;font-weight:600;line-height:1.3;`
				if align != "" {
					open += "text-align:" + align + ";"
				}
				b.WriteString(open + `">`)
				stack = append(stack, openTag{name: tag, out: "</" + tag + ">"})
			case "blockquote":
				drainAttrs(z, hasAttr)
				b.WriteString(emailQuoteOpen)
				stack = append(stack, openTag{name: tag, out: "</blockquote>"})
			case "pre":
				drainAttrs(z, hasAttr)
				b.WriteString(emailPreOpen)
				stack = append(stack, openTag{name: tag, out: "</pre>"})
			case "code":
				drainAttrs(z, hasAttr)
				b.WriteString(emailCodeOpen)
				stack = append(stack, openTag{name: tag, out: "</code>"})
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
			case "b", "strong", "i", "em", "u", "s":
				drainAttrs(z, hasAttr)
				b.WriteString("<" + tag + ">")
				stack = append(stack, openTag{name: tag, out: "</" + tag + ">"})
			case "table":
				drainAttrs(z, hasAttr)
				b.WriteString(emailTableOpen)
				stack = append(stack, openTag{name: tag, out: "</table>"})
			case "tr":
				drainAttrs(z, hasAttr)
				b.WriteString("<tr>")
				stack = append(stack, openTag{name: tag, out: "</tr>"})
			case "th", "td":
				drainAttrs(z, hasAttr)
				open := emailTDOpen
				if tag == "th" {
					open = emailTHOpen
				}
				b.WriteString(open)
				stack = append(stack, openTag{name: tag, out: "</" + tag + ">", cell: true, cellStart: b.Len()})
			case "span":
				style := ""
				if keepStyles {
					style = inlineStyleAttr(z, hasAttr)
				} else {
					drainAttrs(z, hasAttr)
				}
				if style != "" {
					b.WriteString(`<span` + styleAttrHTML(style) + `>`)
					stack = append(stack, openTag{name: tag, out: "</span>"})
				} else {
					stack = append(stack, openTag{name: tag})
				}
			case "a":
				var href, style string
				for hasAttr {
					var key, val []byte
					key, val, hasAttr = z.TagAttr()
					switch string(key) {
					case "href":
						href = string(val)
					case "style":
						if keepStyles {
							style = safeInlineStyle(string(val))
						}
					}
				}
				if isSafeLinkHref(href) {
					b.WriteString(`<a href="` + escapeHTML(href) + `"` + styleAttrHTML(style) + `>`)
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
				closeTopTag()
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
// portal needs. intendedFor, when non-empty, shows a "Sent to: <value>" row
// — the real recipient(s), for a debug-redirected send (see dispatch.go's
// own EMAIL_DEBUG_MODE handling); empty omits the row entirely rather than
// rendering it blank, so a real (non-debug) send never shows it.
func RenderCommentAddedEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink, intendedFor string) (string, []InlineImage) {
	tmpl := applyOptionalBlock(commentAddedTemplate, "INTENDED_FOR", intendedFor)
	comment, images := sanitizeRichText(caseComment, &inlineImageBudget{})
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(intendedFor),
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", comment,
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(tmpl), images
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
func RenderInternalNoteEmail(name, caseNumber, caseTitle, caseComment, commentLink, caseLink, intendedFor string) (string, []InlineImage) {
	tmpl := applyOptionalBlock(internalNoteTemplate, "INTENDED_FOR", intendedFor)
	comment, images := sanitizeRichText(caseComment, &inlineImageBudget{})
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(intendedFor),
		"<!-- [NAME] -->", escapeHTML(name),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(caseTitle),
		"<!-- [CASE_COMMENT] -->", comment,
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
	)
	return replacer.Replace(tmpl), images
}

// RenderStatusChangedEmail fills in the "case status changed" HTML email
// template. caseLink is used both for the case-number link in the strap
// line and the "View Case" link; commentLink is the "Add Comment"
// call-to-action target. caseNumber — see RenderCommentAddedEmail's own doc
// comment.
func RenderStatusChangedEmail(caseNumber, newStatus, caseLink, commentLink, intendedFor string) string {
	tmpl := applyOptionalBlock(statusChangedTemplate, "INTENDED_FOR", intendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(intendedFor),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [NEW_STATUS] -->", escapeHTML(newStatus),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(tmpl)
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
func RenderSeverityChangedEmail(caseNumber, oldSeverity, newSeverity, caseLink, commentLink, intendedFor string) string {
	tmpl := applyOptionalBlock(severityChangedTemplate, "INTENDED_FOR", intendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(intendedFor),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [OLD_SEVERITY] -->", escapeHTML(oldSeverity),
		"<!-- [NEW_SEVERITY] -->", escapeHTML(newSeverity),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(tmpl)
}

// RenderCaseAssignedEmail fills in the "case assigned" HTML email template.
// assigneeEmail is rendered both as a mailto: link and as plain text.
// caseNumber — see RenderCommentAddedEmail's own doc comment.
func RenderCaseAssignedEmail(assigneeName, assigneeEmail, caseNumber, caseLink, commentLink, intendedFor string) string {
	tmpl := applyOptionalBlock(caseAssignedTemplate, "INTENDED_FOR", intendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(intendedFor),
		"<!-- [ASSIGNEE_NAME] -->", escapeHTML(assigneeName),
		"<!-- [ASSIGNEE_EMAIL] -->", escapeHTML(assigneeEmail),
		"<!-- [CASE_NUMBER] -->", escapeHTML(caseNumber),
		"<!-- [CASE_LINK] -->", escapeHTML(caseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(commentLink),
	)
	return replacer.Replace(tmpl)
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
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
}

// RenderCaseCreatedEmail fills in the "case created" HTML email template.
// Priority and Product each drop their whole row (not just render blank)
// when unset — every case type but "case" has no Priority, and
// "announcement" has neither, since neither concept applies to those types
// (see entity-service's own validateCreateCaseRequest/publishCaseCreatedEvent).
func RenderCaseCreatedEmail(data CaseCreatedEmailData) (string, []InlineImage) {
	tmpl := applyOptionalBlock(caseCreatedTemplate, "IMPACT", data.IncidentImpactDescription)
	tmpl = applyOptionalBlock(tmpl, "PRIORITY", data.Priority)
	tmpl = applyOptionalBlock(tmpl, "PRODUCT", data.Product)
	tmpl = applyOptionalBlock(tmpl, "INTENDED_FOR", data.IntendedFor)
	// Description and IncidentImpactDescription both end up as attachments
	// on this same outgoing email, so they must share one budget — see
	// inlineImageBudget's own doc comment for why a fresh one per call
	// would let each field independently max out.
	budget := &inlineImageBudget{}
	// An announcement whose body carries real styling (a box, colours, spacing)
	// keeps it; every other email, and an announcement typed in the rich editor
	// (no styling beyond alignment), goes through the plain rules unchanged.
	sanitizeDescription := sanitizeRichText
	if strings.EqualFold(strings.TrimSpace(data.CaseType), "Announcement") && hasPresentationStyles(data.Description) {
		sanitizeDescription = sanitizeAnnouncementRichText
	}
	description, descImages := sanitizeDescription(data.Description, budget)
	impact, impactImages := sanitizeRichText(data.IncidentImpactDescription, budget)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(data.IntendedFor),
		"<!-- [REPORTER_NAME] -->", escapeHTML(data.ReporterName),
		"<!-- [PROJECT_NAME] -->", escapeHTML(data.ProjectName),
		"<!-- [CASE_NUMBER] -->", escapeHTML(data.CaseNumber),
		"<!-- [CASE_TITLE] -->", escapeHTML(data.CaseTitle),
		"<!-- [CASE_TYPE] -->", escapeHTML(data.CaseType),
		"<!-- [PRIORITY] -->", escapeHTML(data.Priority),
		"<!-- [PRODUCT] -->", escapeHTML(data.Product),
		"<!-- [CREATED_AT] -->", escapeHTML(data.CreatedAt),
		"<!-- [DESCRIPTION] -->", description,
		"<!-- [INCIDENT_IMPACT_DESCRIPTION] -->", impact,
		"<!-- [CASE_LINK] -->", escapeHTML(data.CaseLink),
		"<!-- [COMMENT_LINK] -->", escapeHTML(data.CommentLink),
	)
	return replacer.Replace(tmpl), append(descImages, impactImages...)
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
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
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

	tmpl := applyOptionalBlock(crApprovalRequestedTemplate, "INTENDED_FOR", d.IntendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [CR_NUMBER] -->", escapeHTML(d.Number),
		"<!-- [STATE_LABEL] -->", escapeHTML(state),
		"<!-- [AUDIENCE_LABEL] -->", escapeHTML(audience),
		"<!-- [REQUESTER] -->", escapeHTML(requester),
		"<!-- [CR_LINK] -->", escapeHTML(d.Link),
		"<!-- [CONTEXT_LINE] -->", context,
	)
	return replacer.Replace(tmpl)
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
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
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
func RenderCRPlanDateNoticeEmail(d CRPlanDateEmailData) (string, []InlineImage) {
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

	// ShortDescription and Description both end up as attachments on this
	// same outgoing email — see inlineImageBudget's own doc comment for
	// why they must share one budget rather than each getting a fresh one.
	budget := &inlineImageBudget{}
	shortDescription, shortDescImages := sanitizeRichText(d.ShortDescription, budget)
	description, descImages := sanitizeRichText(d.Description, budget)
	tmpl := applyOptionalBlock(crPlanDateNoticeTemplate, "INTENDED_FOR", d.IntendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [CR_NUMBER] -->", escapeHTML(d.Number),
		"<!-- [HEADLINE] -->", headline,
		"<!-- [PROJECT_AND_NUMBER] -->", projectAndNumber,
		"<!-- [SHORT_DESCRIPTION] -->", shortDescription,
		"<!-- [DESCRIPTION] -->", description,
		"<!-- [CLOSING_LINE] -->", escapeHTML(w.closing),
		"<!-- [CR_LINK] -->", escapeHTML(d.Link),
	)
	return replacer.Replace(tmpl), append(shortDescImages, descImages...)
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
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
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
	tmpl = applyOptionalBlock(tmpl, "INTENDED_FOR", d.IntendedFor)
	replacer := strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [DISPLAY_NAME] -->", escapeHTML(d.DisplayName),
		"<!-- [EMAIL] -->", escapeHTML(d.Email),
		"<!-- [PROJECT_NAME] -->", escapeHTML(d.ProjectName),
		"<!-- [PROJECT_KEY] -->", escapeHTML(d.ProjectKey),
		"<!-- [ROLES] -->", escapeHTML(roles),
		"<!-- [PORTAL_URL] -->", escapeHTML(d.PortalURL),
	)
	return replacer.Replace(tmpl)
}

// ProjectContactRegisteredEmailData fills the Welcome email sent after a
// contact's first sign-in moves their membership to REGISTERED.
type ProjectContactRegisteredEmailData struct {
	DisplayName string
	ProjectName string
	ProjectKey  string
	PortalURL   string
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
}

// RenderProjectContactRegisteredEmail fills in the Welcome email.
func RenderProjectContactRegisteredEmail(d ProjectContactRegisteredEmailData) string {
	tmpl := applyOptionalBlock(projectContactRegisteredTemplate, "INTENDED_FOR", d.IntendedFor)
	return strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [DISPLAY_NAME] -->", escapeHTML(d.DisplayName),
		"<!-- [PROJECT_NAME] -->", escapeHTML(d.ProjectName),
		"<!-- [PROJECT_KEY] -->", escapeHTML(d.ProjectKey),
		"<!-- [PORTAL_URL] -->", escapeHTML(d.PortalURL),
	).Replace(tmpl)
}

// ProjectReactivatedEmailData fills the "Back to Open state" email, sent when
// a project the Account Closure Process suspended is back to Open.
type ProjectReactivatedEmailData struct {
	// GreetingName follows "Dear": the account owner's first name.
	GreetingName string
	// ReasonLine is the sentence saying why the project reopened.
	ReasonLine   string
	ProjectName  string
	ProjectKey   string
	ProjectLink  string
	AccountOwner string
	StartDate    string
	EndDate      string
	// IntendedFor, when non-empty, shows a "Sent to: <value>" row — see
	// RenderCommentAddedEmail's own doc comment for the convention.
	IntendedFor string
}

// RenderProjectReactivatedEmail fills in the "Back to Open state" email. Every
// value is escaped; an empty one shows as "-".
func RenderProjectReactivatedEmail(d ProjectReactivatedEmailData) string {
	tmpl := applyOptionalBlock(projectReactivatedTemplate, "INTENDED_FOR", d.IntendedFor)
	orDash := func(v string) string {
		if strings.TrimSpace(v) == "" {
			return "-"
		}
		return escapeHTML(v)
	}
	return strings.NewReplacer(
		"<!-- [INTENDED_FOR] -->", escapeHTML(d.IntendedFor),
		"<!-- [GREETING_NAME] -->", escapeHTML(d.GreetingName),
		"<!-- [REASON_LINE] -->", escapeHTML(d.ReasonLine),
		"<!-- [PROJECT_NAME] -->", orDash(d.ProjectName),
		"<!-- [PROJECT_KEY] -->", orDash(d.ProjectKey),
		"<!-- [PROJECT_LINK] -->", escapeHTML(d.ProjectLink),
		"<!-- [ACCOUNT_OWNER] -->", orDash(d.AccountOwner),
		"<!-- [START_DATE] -->", orDash(d.StartDate),
		"<!-- [END_DATE] -->", orDash(d.EndDate),
	).Replace(tmpl)
}
