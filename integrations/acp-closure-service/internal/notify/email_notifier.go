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

package notify

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
)

// wso2LogoURL points at WSO2's own public CDN copy of the logo (circular
// pulse icon + wordmark) used in real received notice examples — confirmed
// pixel-identical to the file Chamara provided directly. A hosted URL, not
// an embedded data: URI, deliberately: most email clients (confirmed
// against a real Gmail inbox — the data: URI approach rendered as a broken
// image icon there) refuse to render inline base64 images in received
// mail, so the logo has to be fetched from a real reachable address like
// any other web image.
const wso2LogoURL = "https://wso2.cachefly.net/wso2/sites/all/image_resources/logos/WSO2-Logo-Black.png"

// emailHTMLTemplate is the branded shell the customer-facing notice is
// wrapped in — peach page background, logo header, orange accent border
// around the message body, and the standard disclaimer footer — matching
// real customer-facing examples. The internal notice uses a different,
// distinct template (internalEmailHTMLTemplate) — see Send. Two
// placeholders: the logo URL, then the notice body already converted to
// simple HTML by plainTextToHTML.
const emailHTMLTemplate = `<div style="background-color:#fdece2;padding:32px 16px;font-family:Arial,Helvetica,sans-serif;">
  <div style="max-width:600px;margin:0 auto;background-color:#ffffff;border-radius:4px;overflow:hidden;">
    <div style="padding:24px 32px 16px 32px;">
      <img src="%s" alt="WSO2" height="28" style="display:block;">
    </div>
    <div style="padding:8px 32px 24px 32px;">
      <div style="border-left:4px solid #ff7300;padding:4px 0 4px 16px;color:#333333;font-size:14px;line-height:1.6;">
        %s
      </div>
    </div>
    <div style="padding:16px 32px;border-top:1px solid #eeeeee;color:#888888;font-size:11px;">
      This automated message was sent by WSO2's support system. Please do not reply to this email.
    </div>
  </div>
</div>`

// internalEmailHTMLTemplate is the internal notice's own distinct branded
// shell — no peach background, no logo header, a simple light-bordered
// white card instead, framed by a light ash-gray page background (the
// same #f2f2f2 tone as the detail box below, matching the real reference).
// Confirmed against a real received internal notice example ("Dear Nisha
// Farook..."): the greeting and intro sentence render in blue and larger
// than the body text, the five project/account fields sit in their own
// bordered detail box, and the closing sentence plus sign-off are plain
// black text. Five placeholders in order: greeting (HTML), intro sentence
// (HTML), the detail box's inner HTML (pre-built field rows), closing
// sentence (HTML), sign-off (HTML). Built by renderInternalEmailHTML,
// which is the only thing that knows this template expects exactly that
// shape.
const internalEmailHTMLTemplate = `<div style="background-color:#f2f2f2;padding:24px 16px;font-family:Arial,Helvetica,sans-serif;">
  <div style="background-color:#ffffff;border:1px solid #dadce0;border-radius:8px;padding:24px 32px;box-sizing:border-box;">
    <p style="color:#1a56db;font-size:16px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <p style="color:#1a56db;font-size:16px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <div style="border:1px solid #e0e0e0;border-radius:4px;background-color:#f2f2f2;padding:16px 20px;margin:0 0 16px 0;">
      %s
    </div>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0;">%s</p>
  </div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width:100%%;margin-top:8px;">
    <tr>
      <td style="padding:0 0 0 32px;color:#888888;font-size:11px;text-align:left;vertical-align:middle;">This automated message was sent by WSO2's support system. Please do not reply to this email.</td>
      <td style="padding:0 32px 0 8px;text-align:right;vertical-align:middle;white-space:nowrap;"><img src="%s" alt="WSO2" height="16" style="display:block;"></td>
    </tr>
  </table>
</div>`

// internalInvoiceEmailHTMLTemplate is the invoice-based internal notice's
// shell — identical to internalEmailHTMLTemplate (same card, same
// greeting/intro/closing/signoff styling, same footer) except the detail
// box has a second, nested box inside it for the invoice-specific fields
// (Invoice Id/Opportunity/Due Date), confirmed against real reference
// examples (actual_90_days_invoice_email.png, actual_0_days_invoice_email.html)
// — those three fields render visually nested under the project fields, not
// as three more flat rows in the same box. Seven placeholders in order:
// greeting, intro, project fields' inner HTML, invoice fields' inner HTML,
// closing, sign-off, wso2LogoURL. Built by renderInternalInvoiceEmailHTML.
const internalInvoiceEmailHTMLTemplate = `<div style="background-color:#f2f2f2;padding:24px 16px;font-family:Arial,Helvetica,sans-serif;">
  <div style="background-color:#ffffff;border:1px solid #dadce0;border-radius:8px;padding:24px 32px;box-sizing:border-box;">
    <p style="color:#1a56db;font-size:16px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <p style="color:#1a56db;font-size:16px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <div style="border:1px solid #e0e0e0;border-radius:4px;background-color:#f2f2f2;padding:16px 20px;margin:0 0 16px 0;">
      %s
      <div style="border:1px solid #e0e0e0;border-radius:4px;background-color:#ffffff;padding:12px 16px;margin-top:8px;">
        %s
      </div>
    </div>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0;">%s</p>
  </div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width:100%%;margin-top:8px;">
    <tr>
      <td style="padding:0 0 0 32px;color:#888888;font-size:11px;text-align:left;vertical-align:middle;">This automated message was sent by WSO2's support system. Please do not reply to this email.</td>
      <td style="padding:0 32px 0 8px;text-align:right;vertical-align:middle;white-space:nowrap;"><img src="%s" alt="WSO2" height="16" style="display:block;"></td>
    </tr>
  </table>
</div>`

// wso2BusinessContactDocURL is the fixed link target for the no-business-
// contact notice's "Update Business Contact" hyperlink — a static doc, not
// something that varies per project, so it's a constant rather than a
// per-notice parameter.
const wso2BusinessContactDocURL = "https://docs.google.com/document/d/1xv-n6XK7E60QZdEb7DrbpDW0UmXs-SqLceLGaqPx68k/edit?tab=t.0#heading=h.58epxafkaoe"

// noBusinessContactEmailHTMLTemplate is the no-business-contact notice's
// own dedicated shell, confirmed against the real reference
// (business_contact_email.png) — a bold red heading sits above the card
// (not inside it, unlike every other internal template here), a bold blue
// intro line, then a paragraph with fixed boilerplate wording (matching
// sweep.go's noBusinessContactBodyTemplate verbatim) with the project name
// bolded and two warning clauses in bold red italic, a real hyperlink to
// the business contact doc, and the same nested field-detail box style
// used elsewhere. Five placeholders in order: intro (HTML), project name
// (HTML-escaped, for the bolded mention), doc URL, fields' inner HTML,
// wso2LogoURL.
const noBusinessContactEmailHTMLTemplate = `<div style="background-color:#f2f2f2;padding:24px 16px;font-family:Arial,Helvetica,sans-serif;">
  <p style="color:#c62828;font-size:20px;font-weight:bold;margin:0 0 16px 0;">Internal - Customer Project without Business Contacts</p>
  <div style="background-color:#ffffff;border:1px solid #dadce0;border-radius:8px;padding:24px 32px;box-sizing:border-box;">
    <p style="color:#1a56db;font-size:16px;font-weight:bold;line-height:1.6;margin:0 0 16px 0;">%s</p>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0 0 16px 0;">Please note that no Business Contacts was found for the project <strong>%s</strong>. Immediate action is required to address this issue, as without a business contact, the <strong style="color:#c62828;font-style:italic;">customers will not receive essential notifications regarding their project suspension status</strong>. Additionally, this will lead to <strong style="color:#c62828;font-style:italic;">failures in further automated actions related to project suspension</strong>.</p>
    <p style="color:#333333;font-size:14px;line-height:1.6;margin:0 0 16px 0;">Please refer this document to <a href="%s" style="color:#1a56db;">Update Business Contact</a></p>
    <div style="border:1px solid #e0e0e0;border-radius:4px;background-color:#f2f2f2;padding:16px 20px;">
      %s
    </div>
  </div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width:100%%;margin-top:8px;">
    <tr>
      <td style="padding:0 0 0 32px;color:#888888;font-size:11px;text-align:left;vertical-align:middle;">This automated message was sent by WSO2's support system. Please do not reply to this email.</td>
      <td style="padding:0 32px 0 8px;text-align:right;vertical-align:middle;white-space:nowrap;"><img src="%s" alt="WSO2" height="16" style="display:block;"></td>
    </tr>
  </table>
</div>`

// internalEmailFallbackTemplate wraps a body that doesn't match the
// day-count/suspension reminder's expected paragraph shape (currently:
// the no-business-contact notice) — same card styling as
// internalEmailHTMLTemplate, but without attempting the greeting/detail
// box/closing split, since that shape assumption doesn't hold for it.
const internalEmailFallbackTemplate = `<div style="background-color:#f2f2f2;padding:24px 16px;font-family:Arial,Helvetica,sans-serif;">
  <div style="background-color:#ffffff;border:1px solid #dadce0;border-radius:8px;padding:24px 32px;color:#333333;font-size:14px;line-height:1.6;box-sizing:border-box;">
    %s
  </div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width:100%%;margin-top:8px;">
    <tr>
      <td style="padding:0 0 0 32px;color:#888888;font-size:11px;text-align:left;vertical-align:middle;">This automated message was sent by WSO2's support system. Please do not reply to this email.</td>
      <td style="padding:0 32px 0 8px;text-align:right;vertical-align:middle;white-space:nowrap;"><img src="%s" alt="WSO2" height="16" style="display:block;"></td>
    </tr>
  </table>
</div>`

// emailSender is the minimal send surface EmailNotifier needs. Satisfied by
// *emailservice.Client — declared locally, not imported, so this package
// doesn't need to depend on emailservice's concrete type.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// EmailNotifier sends real emails via WSO2's internal email notification
// service (internal/emailservice), replacing LoggingNotifier once real
// sending is actually wanted. Recipients map onto the real API's "to"/"cc"
// fields based on which Recipients fields are populated: when Customer is
// present, the customer is the primary "to" recipient and the three
// internal people (Account Owner/Renewal Manager/Technical Owner) are
// copied via "cc"; otherwise (internal-only notices, and the
// no-business-contact notice) all populated internal recipients go in "to".
type EmailNotifier struct {
	Sender emailSender
	Logger *slog.Logger
	// AllowNonWSO2Recipients, when false (the safe default), filters out
	// any recipient whose address doesn't end in "@wso2.com" before
	// sending. This is a hard requirement from Rashmika's team (owners of
	// the email service): a staging/testing environment must never
	// actually email a real customer contact. Only set true in a genuine
	// production environment, once that's a deliberate decision — not
	// something to flip casually to "make a test work".
	AllowNonWSO2Recipients bool
}

// Send builds the to/cc recipient lists, converts Body to simple HTML, and
// calls the real email service. Reports whether this specific notice was
// actually delivered — false (not an error) when every recipient got
// filtered out (empty email, or a non-WSO2 address with
// AllowNonWSO2Recipients false) or there were never any recipients to
// begin with, rather than forcing a call the real API would reject anyway
// (it requires at least one "to" address); a project with nobody to notify
// is a legitimate, unremarkable state, not an error, matching the
// convention already established throughout this codebase for absent
// recipients. Callers (sweep.recordNoticeSent) use this per-call result,
// not a blanket "this notifier type sends for real" signal — a customer
// notice silently filtered out in staging must not be recorded as
// delivered just because EmailNotifier is, in general, the real-sending
// kind.
func (n *EmailNotifier) Send(ctx context.Context, notice Notice) (bool, error) {
	to, cc := recipientsToToCC(notice.Recipients)
	to = n.filterRecipients(to)
	cc = n.filterRecipients(cc)

	if len(to) == 0 {
		n.Logger.InfoContext(ctx, "email skipped: no valid recipients",
			"projectID", notice.ProjectID, "window", notice.Window, "subject", notice.Subject)
		return false, nil
	}

	// Every notice — internal and customer-facing alike — gets a branded
	// WSO2 shell, confirmed against real received examples of both — but
	// the two shells are genuinely different (not just a color swap): the
	// customer-facing one is renderEmailHTML (peach background, logo,
	// orange border); the internal one is its own distinct
	// renderInternalEmailHTML (light-bordered white card, blue
	// greeting/intro, a separate detail box for the project/account
	// fields). An earlier version of this code kept the internal notice
	// on the bare plainTextToHTML fragment alone, based on a different,
	// less complete reference; that was wrong, and sending an unwrapped
	// fragment with no real block-level container was also the likely
	// cause of a real symptom seen in a live test: the trailing "WSO2
	// Team" signature line visually missing in the received email.
	htmlBody := renderInternalEmailHTML(notice.Body)
	if notice.Recipients.Customer != nil {
		htmlBody = renderEmailHTML(notice.Body)
	}

	if err := n.Sender.SendEmail(ctx, to, cc, notice.Subject, htmlBody); err != nil {
		return false, fmt.Errorf("send email: %w", err)
	}

	n.Logger.InfoContext(ctx, "email sent",
		"projectID", notice.ProjectID, "window", notice.Window, "subject", notice.Subject,
		"toCount", len(to), "ccCount", len(cc))
	return true, nil
}

// recipientsToToCC maps Recipients onto the real API's to/cc shape. Empty
// emails (a role with no address on file — a legitimate, unremarkable
// state per recipients.AccountManagerEmail's existing contract) are
// dropped rather than sent through as blank strings.
func recipientsToToCC(r Recipients) (to, cc []string) {
	if r.Customer != nil {
		to = appendIfNonEmpty(to, r.Customer.Email)
		cc = appendIfNonEmpty(cc, r.AccountOwner.Email, r.RenewalManager.Email, r.TechnicalOwner.Email)
		return to, cc
	}
	to = appendIfNonEmpty(to, r.AccountOwner.Email, r.RenewalManager.Email, r.TechnicalOwner.Email)
	return to, nil
}

func appendIfNonEmpty(list []string, emails ...string) []string {
	for _, e := range emails {
		if e != "" {
			list = append(list, e)
		}
	}
	return list
}

// filterRecipients applies the WSO2-only staging safeguard: when
// AllowNonWSO2Recipients is false, only addresses ending in "@wso2.com"
// (case-insensitive) survive.
func (n *EmailNotifier) filterRecipients(emails []string) []string {
	if n.AllowNonWSO2Recipients {
		return emails
	}
	var filtered []string
	for _, e := range emails {
		if strings.HasSuffix(strings.ToLower(e), "@wso2.com") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// renderEmailHTML wraps a notice's plain-text Body in the customer-facing
// branded WSO2 email shell (emailHTMLTemplate) — logo, orange accent
// border, footer disclaimer — matching real customer-facing notice
// examples. Used by Send only when notice.Recipients.Customer is non-nil.
func renderEmailHTML(body string) string {
	return fmt.Sprintf(emailHTMLTemplate, wso2LogoURL, plainTextToHTML(body))
}

// internalBodyParagraphCount is the exact number of blank-line-separated
// paragraphs the day-count/suspension reminder body templates always
// produce (sweep.go's internalReminderBodyTemplate /
// internalSuspensionBodyTemplate): greeting, intro sentence, 5 field
// lines (Project Name/Key/Account Owner/Start Date/End Date), closing
// sentence, sign-off. renderInternalEmailHTML only attempts the
// structured layout when a body has exactly this shape.
const internalBodyParagraphCount = 9

// internalInvoiceBodyParagraphCount is the exact paragraph count of the
// invoice-based internal notice's body shape: greeting, intro, the same 5
// project fields the subscription-based notice has, then 3 more
// invoice-specific fields (Invoice Id/Opportunity/Due Date), then closing,
// sign-off.
const internalInvoiceBodyParagraphCount = 12

// noBusinessContactBodyParagraphCount is the exact paragraph count of
// sweep.go's noBusinessContactBodyTemplate: heading, intro, warning
// paragraph, hyperlink line, then a final paragraph holding all 5 fields
// joined by single "\n" rather than one field per paragraph like the other
// notice shapes.
const noBusinessContactBodyParagraphCount = 5

// renderInternalEmailHTML wraps an internal notice's body in its own
// distinct branded shell. The day-count/suspension reminder bodies have one
// of two known, fixed paragraph shapes — subscription-based
// (internalBodyParagraphCount) or invoice-based
// (internalInvoiceBodyParagraphCount, with its extra nested invoice-fields
// box) — and the no-business-contact notice has its own fixed shape
// (noBusinessContactBodyParagraphCount) — each dispatched to its matching
// renderer below. Anything else falls back to internalEmailFallbackTemplate
// — same card styling, without assuming a shape that doesn't hold for it.
func renderInternalEmailHTML(body string) string {
	paragraphs := strings.Split(body, "\n\n")
	switch len(paragraphs) {
	case internalInvoiceBodyParagraphCount:
		return renderInternalInvoiceEmailHTML(paragraphs)
	case internalBodyParagraphCount:
		return renderInternalSubscriptionEmailHTML(paragraphs)
	case noBusinessContactBodyParagraphCount:
		return renderNoBusinessContactEmailHTML(paragraphs)
	default:
		return fmt.Sprintf(internalEmailFallbackTemplate, plainTextToHTML(body), wso2LogoURL)
	}
}

// renderInternalSubscriptionEmailHTML builds the subscription-based
// internal notice's HTML from its already-validated 9-paragraph body.
func renderInternalSubscriptionEmailHTML(paragraphs []string) string {
	greeting := plainTextToHTML(paragraphs[0])
	intro := plainTextToHTML(paragraphs[1])
	var fields strings.Builder
	for _, p := range paragraphs[2:7] {
		fields.WriteString(fieldRowHTML(p))
	}
	closing := plainTextToHTML(paragraphs[7])
	signoff := plainTextToHTML(paragraphs[8])

	return fmt.Sprintf(internalEmailHTMLTemplate, greeting, intro, fields.String(), closing, signoff, wso2LogoURL)
}

// renderInternalInvoiceEmailHTML builds the invoice-based internal notice's
// HTML from its already-validated 12-paragraph body — the 5 project fields
// render in the main detail box same as the subscription-based notice, the
// 3 invoice fields render in their own nested box inside it.
func renderInternalInvoiceEmailHTML(paragraphs []string) string {
	greeting := plainTextToHTML(paragraphs[0])
	intro := plainTextToHTML(paragraphs[1])
	var projectFields strings.Builder
	for _, p := range paragraphs[2:7] {
		projectFields.WriteString(fieldRowHTML(p))
	}
	var invoiceFields strings.Builder
	for _, p := range paragraphs[7:10] {
		invoiceFields.WriteString(fieldRowHTML(p))
	}
	closing := plainTextToHTML(paragraphs[10])
	signoff := plainTextToHTML(paragraphs[11])

	return fmt.Sprintf(internalInvoiceEmailHTMLTemplate, greeting, intro, projectFields.String(), invoiceFields.String(), closing, signoff, wso2LogoURL)
}

// renderNoBusinessContactEmailHTML builds the no-business-contact notice's
// HTML from its already-validated 5-paragraph body. The warning paragraph
// and hyperlink line's wording is fixed boilerplate baked directly into
// noBusinessContactEmailHTMLTemplate (matching sweep.go's
// noBusinessContactBodyTemplate verbatim) rather than parsed back out of
// the plain-text body's prose — only the project name (pulled from the
// structured "Project Name: X" field line, the same reliable source the
// field box itself uses) and the field values are genuinely dynamic here.
func renderNoBusinessContactEmailHTML(paragraphs []string) string {
	intro := plainTextToHTML(paragraphs[1])

	fieldLines := strings.Split(paragraphs[4], "\n")
	var fields strings.Builder
	var projectName string
	for _, line := range fieldLines {
		fields.WriteString(fieldRowHTML(line))
		if label, value, ok := strings.Cut(line, ": "); ok && label == "Project Name" {
			projectName = value
		}
	}

	return fmt.Sprintf(noBusinessContactEmailHTMLTemplate, intro, html.EscapeString(projectName), wso2BusinessContactDocURL, fields.String(), wso2LogoURL)
}

// fieldRowHTML renders one "Label: value" paragraph (e.g. "Project Name:
// X") as its own styled row inside the internal template's detail box,
// bolding the value. Falls back to a plain escaped line if a paragraph
// doesn't have the expected "Label: value" shape, rather than dropping it.
func fieldRowHTML(paragraph string) string {
	label, value, ok := strings.Cut(paragraph, ": ")
	if !ok {
		return fmt.Sprintf(`<p style="margin:0 0 8px 0;color:#333333;font-size:14px;">%s</p>`, plainTextToHTML(paragraph))
	}
	return fmt.Sprintf(`<p style="margin:0 0 8px 0;color:#333333;font-size:14px;">%s: <strong>%s</strong></p>`,
		html.EscapeString(label), html.EscapeString(value))
}

// plainTextToHTML converts a plain-text notice Body (every existing
// template uses blank-line paragraph breaks and single newlines, never
// HTML) into simple HTML: special characters are escaped first (so a
// project/account name containing "&", "<", etc. can never break the
// resulting markup), then every newline becomes a <br> line break. Used by
// renderEmailHTML to build the message content inside the branded shell.
func plainTextToHTML(s string) string {
	escaped := html.EscapeString(s)
	return strings.ReplaceAll(escaped, "\n", "<br>\n")
}
