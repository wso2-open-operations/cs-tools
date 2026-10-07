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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// GoogleChatAudienceSpace maps a single audience key (e.g. a CRE team name
// like "Castor", or one of the fixed standing audiences — "Incident
// Monitor"/"Onboarding"/"Americas"/"Evaluation") to the Google Chat space
// that should receive alerts for it. This is now the *only* Google Chat
// routing mechanism in this service — there is no product-based
// alternative any more (removed once it became clear this deployment has
// no real per-product Chat space need). case.created/case.acknowledged/
// case.severity_changed all resolve to the single fixed
// chataudience.IncidentMonitor audience (see internal/dispatch); SLA
// breach alerts (internal/slaengine.Engine) resolve a real per-team
// audience via internal/chataudience.Resolve. incident.created has no
// Chat reaction at all (see SendIncidentAlert's own history — removed,
// see git history for the prior product-routed card).
type GoogleChatAudienceSpace struct {
	// Audience identifies the key this space is dedicated to. Matched
	// whitespace-trimmed but *case-sensitively* — an audience key is either
	// a real team's proper-cased display name (entity-service's own Team
	// value, verbatim) or one of this service's own fixed constants,
	// neither of which benefits from case-folding, and case-folding two
	// distinct real team names into one by accident would be a worse
	// failure mode than requiring an exact match.
	Audience string `json:"audience"`
	// WebhookURL is that space's incoming webhook URL (Space settings > Apps
	// & integrations > Webhooks). It already carries its own key/token query
	// parameters, so no separate auth flow is needed.
	WebhookURL string `json:"webhookUrl"`
}

// GoogleChatConfig holds the configuration for the Google Chat notification
// channel: one space per audience (see GoogleChatAudienceSpace's own doc
// comment for what "audience" covers, now that this is the only routing
// mechanism).
type GoogleChatConfig struct {
	AudienceSpaces []GoogleChatAudienceSpace
}

// GoogleChatClient posts messages to a Google Chat space via an incoming
// webhook, routing each alert to the space configured for its audience.
// Unlike the OAuth2-authenticated clients in this package, a webhook URL is
// the only credential required.
//
// NewGoogleChatClient never fails, so it is safe to construct with a
// zero-value GoogleChatConfig (e.g. when this channel is not yet configured
// for a given deployment) — an unconfigured audience only surfaces as a
// logged no-op the first time it's needed (see sendCardToAudience).
type GoogleChatClient struct {
	http                  *http.Client
	webhookURLsByAudience map[string]string
}

// NewGoogleChatClient constructs a GoogleChatClient that routes alerts to the
// webhook configured for each audience in cfg.AudienceSpaces.
func NewGoogleChatClient(cfg GoogleChatConfig) *GoogleChatClient {
	webhookURLsByAudience := make(map[string]string, len(cfg.AudienceSpaces))
	for _, space := range cfg.AudienceSpaces {
		audience := strings.TrimSpace(space.Audience)
		if audience == "" || strings.TrimSpace(space.WebhookURL) == "" {
			continue
		}
		if _, exists := webhookURLsByAudience[audience]; exists {
			webhookURLsByAudience[audience] = ""
			continue
		}
		webhookURLsByAudience[audience] = space.WebhookURL
	}
	return &GoogleChatClient{
		http:                  &http.Client{Timeout: 10 * time.Second},
		webhookURLsByAudience: webhookURLsByAudience,
	}
}

// HasAudienceSpace reports whether audience has a real, configured Chat
// webhook — used by internal/chataudience.Resolve (called from
// internal/slaengine.Engine) to decide whether a case's own CreTeam name is
// a recognized routing target (add it as its own audience) or not (fall
// back to the shared "Incident Monitor" audience instead). Deliberately a
// separate, exported query rather than folding this into
// sendCardToAudience's own resolution: the caller needs the answer
// *before* building the final audience list, not just when it's time to
// send.
func (c *GoogleChatClient) HasAudienceSpace(audience string) bool {
	url, ok := c.webhookURLsByAudience[strings.TrimSpace(audience)]
	return ok && url != ""
}

// withThreadReplyOption adds the reply option a threaded webhook message
// needs: post into the thread named by threadKey, and start it when it does
// not exist yet. The alternative option would drop a message whose thread has
// not been created, which is every ladder's first rung.
//
// The URL already carries its own key and token, so this preserves the whole
// query rather than rebuilding it, and never logs the result.
func withThreadReplyOption(webhookURL string) (string, error) {
	u, err := url.Parse(webhookURL)
	if err != nil {
		// Deliberately not wrapping err: a parse failure echoes the URL,
		// which carries the space's credentials.
		return "", fmt.Errorf("notifications: google chat webhook URL is not parseable")
	}
	q := u.Query()
	q.Set("messageReplyOption", chatThreadReplyOption)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// redactURLError strips the request URL — which carries the webhook's secret
// key/token query parameters — out of a *url.Error before it's wrapped and
// potentially logged, keeping only the underlying (safe) failure reason.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// chatCardMessage is the wire shape Google Chat's webhook API expects for a
// single card message: https://developers.google.com/chat/api/guides/message-formats/cards
type chatCardMessage struct {
	CardsV2 []chatCardWrapper `json:"cardsV2"`
	// Thread groups this message into an existing Chat thread instead of
	// posting a new top-level message -- see chatThreadKey's own doc
	// comment. nil (omitempty) for every card that doesn't opt into
	// threading.
	Thread *chatThread `json:"thread,omitempty"`
}

// chatThread carries Google Chat's threadKey, which groups every message
// sharing the same key into one conversation thread within the space.
// SendCaseCreatedAlert/SendCaseAcknowledgedAlert set it so a case's
// acknowledgment lands as a reply under its own case.created alert instead
// of as a new top-level message -- explicit product request, since the two
// are about the same case and read better grouped together. The escalation
// ladder sets it too, for a different reason: its rungs are one unfolding
// story rather than separate events, so every rung of an incident's ladder
// belongs under the first one.
type chatThread struct {
	ThreadKey string `json:"threadKey,omitempty"`
}

// chatThreadReplyOption must be appended as a query parameter (not a body
// field) whenever a webhook POST sets chatThread.ThreadKey: Google Chat's
// webhook endpoint otherwise ignores threadKey and always starts a new
// thread. REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD replies into the thread if
// one with this key already exists (the case.acknowledged alert, posted
// after case.created), or starts one if this is the first message with
// that key (case.created itself) -- see
// https://developers.google.com/workspace/chat/format-structure-send-message#thread_a_message.
const chatThreadReplyOption = "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD"

// chatThreadKey derives a stable Google Chat threadKey from a case's own
// number -- already required/unique on every card that sets it, so no
// separate case-id parameter is needed just for this.
func chatThreadKey(caseNumber string) string {
	return "case-" + caseNumber
}

type chatCardWrapper struct {
	CardID string   `json:"cardId"`
	Card   chatCard `json:"card"`
}

// Header is a pointer, unlike every other field on this type — every
// case.*/incident.created card sets it today, but a pointer keeps a
// header-less card representable (omitempty) rather than requiring an
// empty title on the wire, should a future card need one.
type chatCard struct {
	Header   *chatCardHeader   `json:"header,omitempty"`
	Sections []chatCardSection `json:"sections"`
}

type chatCardHeader struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

type chatCardSection struct {
	Header string `json:"header,omitempty"`
	// Collapsible folds the section behind a "Show more" toggle, leaving its
	// first UncollapsibleWidgetsCount widgets visible (cardsV2 Section
	// schema). Only the SR-created card's description uses it, matching
	// ServiceNow's card.
	Collapsible               bool             `json:"collapsible,omitempty"`
	UncollapsibleWidgetsCount int              `json:"uncollapsibleWidgetsCount,omitempty"`
	Widgets                   []chatCardWidget `json:"widgets"`
}

// chatCardWidget is a union type: exactly one of TextParagraph or ButtonList
// is set per widget, matching Google Chat's widget schema.
type chatCardWidget struct {
	TextParagraph *chatTextParagraph `json:"textParagraph,omitempty"`
	ButtonList    *chatButtonList    `json:"buttonList,omitempty"`
}

type chatTextParagraph struct {
	Text string `json:"text"`
}

type chatButtonList struct {
	Buttons []chatButton `json:"buttons"`
}

type chatButton struct {
	Text    string      `json:"text"`
	OnClick chatOnClick `json:"onClick"`
}

type chatOnClick struct {
	OpenLink chatOpenLink `json:"openLink"`
}

type chatOpenLink struct {
	URL string `json:"url"`
}

// caseAlertLine builds one <br>-joined line of a case.created/
// case.acknowledged Chat card's single TextParagraph, HTML-escaping label
// (the dynamic value) but not the markup surrounding it — mirrors
// internal/notifications' own escapeHTML reasoning for email templates:
// Google Chat's card text interprets a limited HTML subset (<b>, <font
// color="...">, <a href="...">), so a dynamic value that happened to
// contain "<" or "&" must not be allowed to break out of the tag it's
// placed in.
//
// Every arg is converted to a string (fmt.Sprint) before escaping, so
// format must only ever use %s for a dynamic value — a numeric verb like
// %.2f against the now-stringified arg fails with a Go fmt verb mismatch
// (%!f(string=...)) instead of formatting the number. Format a non-string
// value with fmt.Sprintf yourself first, then pass the resulting string in
// through %s.
func caseAlertLine(format string, args ...any) string {
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = html.EscapeString(fmt.Sprint(a))
	}
	return fmt.Sprintf(format, escaped...)
}

// chatHeaderCaseRef builds a case.*-card header's Title: the case's own
// identifiers, which read more prominently than a case title (a Roboto Mono
// -styled 15px header, per the redesign this matches) — "<caseNumber> ·
// <wso2CaseID>", falling back to caseNumber alone when the publisher didn't
// send a WSO2 case id.
func chatHeaderCaseRef(caseNumber, wso2CaseID string) string {
	if wso2CaseID == "" {
		return caseNumber
	}
	return caseNumber + " · " + wso2CaseID
}

// teamPart formats team (muted gray) as a case.*-card's own body line —
// visually distinct from severity's color and product's bold without
// italicizing it (tried and dropped: it read as de-emphasized rather than
// just differently colored). Returns "" when team is empty, so callers
// can skip appending it (not a blank line).
func teamPart(team string) string {
	if team == "" {
		return ""
	}
	return caseAlertLine(`<font color="#5F6368">%s</font>`, team)
}

// SendCaseCreatedAlert posts a card message announcing a newly created
// case, to the Google Chat space configured for audience — dispatch.go
// always resolves this to the fixed chataudience.IncidentMonitor audience
// for this event type today (see handleCaseCreated), not a per-team space.
// The case's own
// identifiers (caseNumber/wso2CaseID) lead the card as the header title,
// prefixed with a "🆕" marker — the one thing that distinguishes this
// alert from SendCaseAcknowledgedAlert/SendSeverityChangedAlert's cards,
// which don't carry it. It's folded into the header rather than its own
// body line (an earlier version had one) specifically to cost no extra
// height: Google Chat's card schema has no free-floating corner-badge
// widget the way arbitrary CSS can — a header only has a left-aligned
// title/subtitle plus an optional icon image, which would need a hosted
// asset this service doesn't have. The header subtitle is title (the case
// subject) alone, unstyled — Chat header fields don't render HTML, so
// putting team here (tried and reverted) couldn't get its own color the
// way it does in the body, and read as just another word in the subtitle
// rather than its own thing. team instead leads the body as its own
// first line — right below the header, the next most prominent position
// available — omitted entirely (no blank line) when empty. The rest of
// the body is up to two more plain-text lines, deliberately with no
// leading icon or glyph on any of them — a decorative icon here (Chat's
// native decoratedText icon, or an inline emoji, both tried and dropped)
// only ate into the width each line needs to stay readable on a narrow
// (mobile) screen, without adding any information the 🆕 header marker
// doesn't already carry: severity (colored) alone on its own line;
// productName (bold) alone on the next, omitted entirely (not a blank
// line) when empty; then a single visible "View case" link on its own
// line. "View case" is a real `<a href>` text, not a button, since
// opening the case is navigation, not a genuine one-click action. An
// earlier version of this card had two separate links here,
// "Acknowledge" and "Open in CSM" (there's no interactive card action
// wired up yet to actually acknowledge from Chat — see
// dispatch.handleCaseCreated — so "Acknowledge" pointed at the exact
// same destination as "Open in CSM" anyway); collapsed to one consistent
// "View case" link, matching SendCaseAcknowledgedAlert's/
// SendSeverityChangedAlert's own wording. There is deliberately no
// team/codename line above the header —
// an earlier version of this alert had one, discarded per explicit
// product decision; the case reference now leads instead.
func (c *GoogleChatClient) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var lines []string
	if team != "" {
		lines = append(lines, teamPart(team))
	}
	if severityLabel != "" {
		lines = append(lines, caseAlertLine(`<font color="%s"><b>%s</b></font>`, severityColor, severityLabel))
	}
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, productName))
	}
	lines = append(lines, caseAlertLine(`<a href="%s">View case</a>`, caseLink))
	text := strings.Join(lines, "<br>")
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-created-alert",
				Card: chatCard{
					Header:   &chatCardHeader{Title: "🆕 " + chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: title},
					Sections: []chatCardSection{{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}}},
				},
			},
		},
		Thread: &chatThread{ThreadKey: chatThreadKey(caseNumber)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendSecurityReportAnalysisAlert posts a card message announcing a newly
// created case of type "security_report_analysis", to the Google Chat
// space configured for audience. A dedicated card rather than a variant of
// SendCaseCreatedAlert: that card's severity line only makes sense for
// type=="case" (severity is only ever set for that type — see
// entity-service's own validateCreateCaseRequest/sla_policy.go), so this
// shows a fixed case-type label instead of a severity line. Same header
// convention as SendCaseCreatedAlert (case ref + "🆕" marker as the title,
// case subject as the subtitle) and the same single "View case" link, not
// a button and not an "acknowledge" action — matching the product
// decision behind SendCaseCreatedAlert's own single consistent link (see
// that function's own doc comment).
func (c *GoogleChatClient) SendSecurityReportAnalysisAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var lines []string
	if team != "" {
		lines = append(lines, teamPart(team))
	}
	lines = append(lines, "<b>Security Report Analysis</b>")
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, productName))
	}
	lines = append(lines, caseAlertLine(`<a href="%s">View case</a>`, caseLink))
	text := strings.Join(lines, "<br>")
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "security-report-analysis-created-alert",
				Card: chatCard{
					Header:   &chatCardHeader{Title: "🆕 " + chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: title},
					Sections: []chatCardSection{{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}}},
				},
			},
		},
		// A security_report_analysis case can still be acknowledged (see
		// SendCaseAcknowledgedAlert's own doc comment) -- without this, its
		// case.acknowledged alert would fall back to a new thread instead
		// of replying to this creation alert, the same reasoning
		// SendCaseCreatedAlert's own Thread field documents.
		Thread: &chatThread{ThreadKey: chatThreadKey(caseNumber)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendCaseAcknowledgedAlert posts a three-line card message announcing
// that a case was acknowledged, to the same Google Chat space as its
// case.created alert — no header, no button, and deliberately no leading
// icon/glyph on any line (see SendCaseCreatedAlert's own doc comment for
// why): severity alone on its own line; "<caseNumber> · <wso2CaseID>" on
// the next — the same "·" separator chatHeaderCaseRef uses for the case
// reference on the other two cards, for a consistent look; then
// "Ack by <name> · View case" on the final line. caseNumber is plain
// text, not a link — an earlier version linked it directly, which read
// inconsistently next to SendCaseCreatedAlert's/SendSeverityChangedAlert's
// own explicit "View case" link text; "View case" now plays that same
// role here too, this card's only navigation affordance
// (there's no genuine action to take from an acknowledgment — see
// SendCaseCreatedAlert's own doc comment for the fuller link-vs-button
// reasoning). wso2CaseID is dropped from its line entirely when the
// publisher didn't send one. Unlike SendCaseCreatedAlert/
// SendSeverityChangedAlert, this card deliberately doesn't show team at
// all — kept to exactly these three lines per explicit product direction.
func (c *GoogleChatClient) SendCaseAcknowledgedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error {
	if caseNumber == "" || acknowledgerName == "" {
		return fmt.Errorf("notifications: caseNumber and acknowledgerName are required")
	}
	sevLine := caseAlertLine(`<font color="%s"><b>%s</b></font>`, severityColor, severityLabel)
	caseRefLine := caseAlertLine(`%s`, caseNumber)
	if wso2CaseID != "" {
		caseRefLine += " · " + caseAlertLine(`%s`, wso2CaseID)
	}
	closeLine := caseAlertLine(`Ack by %s`, acknowledgerName) + " · " + caseAlertLine(`<a href="%s">View case</a>`, caseLink)
	text := strings.Join([]string{sevLine, caseRefLine, closeLine}, "<br>")

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-acknowledged-alert",
				Card: chatCard{
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}},
					},
				},
			},
		},
		Thread: &chatThread{ThreadKey: chatThreadKey(caseNumber)},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendSeverityChangedAlert posts a card message announcing a case's
// severity changed, to the same Google Chat space as its case.created
// alert. The case reference leads the header, same as every other case.*
// card; the subtitle is title (the case subject) alone, unstyled — team
// is not part of the header (see SendCaseCreatedAlert's own doc comment
// for why: header fields can't carry team's own color, so it leads the
// body as its own first line instead — the next most prominent position
// — omitted entirely when empty). The rest of the body is up to two more
// plain-text lines, deliberately with no leading icon/glyph on either
// (see SendCaseCreatedAlert's own doc comment for why): old severity and
// new severity — each colored by its own resolved color, not just the
// new one, so a severity reads the same way here as it does on the other
// two cards — separated by an arrow, on their own line; then a visible
// "View case" link alone on the next. No button: there's no genuine
// action to take from this card, only navigation, which the link already
// covers — see SendCaseCreatedAlert's own doc comment for the fuller
// button-vs-link reasoning.
func (c *GoogleChatClient) SendSeverityChangedAlert(ctx context.Context, audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var lines []string
	if team != "" {
		lines = append(lines, teamPart(team))
	}
	if oldSeverityLabel != "" && newSeverityLabel != "" {
		lines = append(lines, caseAlertLine(`<font color="%s"><b>%s</b></font> → <font color="%s"><b>%s</b></font>`, oldSeverityColor, oldSeverityLabel, newSeverityColor, newSeverityLabel))
	}
	lines = append(lines, caseAlertLine(`<a href="%s">View case</a>`, caseLink))
	text := strings.Join(lines, "<br>")

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-severity-changed-alert",
				Card: chatCard{
					Header: &chatCardHeader{Title: chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: title},
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}},
					},
				},
			},
		},
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// SendFrustrationAlert posts a card message for a comment
// ai-escalate-comment-detector (internal/escalation) flagged as
// escalation-worthy, to the Google Chat space configured for audience —
// dispatch.checkFrustration resolves audience via chataudience.Resolve, the
// same team-first/Incident-Monitor-fallback routing (plus the Evaluation/
// Onboarding/Americas/weekend overlays) an SLA breach alert uses, using the
// case.comment_added payload's own Team/IsEvaluationAccount/
// ProjectOnboardingStatus fields — not the fixed-IncidentMonitor-only
// posture SendCaseCreatedAlert's own doc comment describes for that event
// type. Same header/body convention as the other case.* cards above: case
// ref as the header title, a "🚨" marker (distinct from SendCaseCreatedAlert's
// "🆕"), product (bold) and the frustration score on their own lines, the
// model's own reason as plain text, then a single "View case" link.
func (c *GoogleChatClient) SendFrustrationAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, reason string, frustrationLevel float64, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var lines []string
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, productName))
	}
	// caseAlertLine stringifies every arg via fmt.Sprint before escaping it
	// (see its own doc comment) -- %.2f against the already-stringified arg
	// would fail with a Go fmt verb mismatch (%!f(string=...)), so the float
	// is formatted here, before caseAlertLine ever sees it, and handed in
	// through %s like every other caseAlertLine call in this file.
	lines = append(lines, caseAlertLine(`Frustration level: <b>%s</b>`, fmt.Sprintf("%.2f", frustrationLevel)))
	if reason != "" {
		lines = append(lines, caseAlertLine(`%s`, reason))
	}
	lines = append(lines, caseAlertLine(`<a href="%s">View case</a>`, caseLink))
	text := strings.Join(lines, "<br>")

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "frustration-alert",
				Card: chatCard{
					Header:   &chatCardHeader{Title: "🚨 " + chatHeaderCaseRef(caseNumber, wso2CaseID), Subtitle: "Frustration detected"},
					Sections: []chatCardSection{{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}}},
				},
			},
		},
		// Deliberately NOT threaded (see chatCardMessage.Thread's own doc
		// comment) -- unlike case.created/case.acknowledged, which group
		// together because they're genuinely the same gesture on the same
		// case, a frustration alert needs to stand out as its own visible
		// message. Threading it under chatThreadKey(caseNumber) (an earlier
		// version of this did, matching the other case.* cards' own
		// ThreadKey by copying their shape without this one's different
		// reasoning) buried every alert as a reply under that case's
		// original case.created message -- easy to miss if that thread is
		// already old/scrolled past, exactly the opposite of what a
		// frustration alert is for.
	}
	return c.sendCardToAudience(ctx, audience, msg)
}

// sendCardToAudience posts msg to the webhook configured for audience — the
// only Chat-send path in this file now (see GoogleChatAudienceSpace's own
// doc comment for why). An audience with no configured webhook is treated
// as a known configuration gap, not a failure: logged at warn and reported
// as a no-op success — a not-yet-onboarded team must not block or retry
// the whole delivery (other audiences still go out regardless).
//
// Every real attempt (webhook configured) logs its own outcome — success or
// failure — at this single choke point, so "did a Chat alert actually go
// out, and to which audience's space" is answerable directly from this
// service's own logs instead of only inferring it from a caller's generic
// retry/dead-letter log further up the call stack, which doesn't say which
// channel failed or why. audience is always safe to log (a team name or
// fixed constant, never recipient data); the failure log deliberately logs
// only postCard's status code, not the error itself or its message — a
// google chat error response can echo back part of the submitted card
// (title/text) verbatim, and this service's own convention is to log ids
// and sanitised summaries only, never a raw upstream body that might carry
// case content (see publishCaseCreatedEvent's identical reasoning,
// entity-service's CLAUDE.md).
func (c *GoogleChatClient) sendCardToAudience(ctx context.Context, audience string, msg chatCardMessage) error {
	webhookURL, ok := c.webhookURLsByAudience[strings.TrimSpace(audience)]
	if !ok || webhookURL == "" {
		slog.WarnContext(ctx, "notifications: no google chat space configured for audience; alert was not posted to it", "audience", audience)
		return nil
	}
	if err := c.postCard(ctx, webhookURL, msg); err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			slog.ErrorContext(ctx, "notifications: google chat alert failed to send", "audience", audience, "statusCode", apiErr.StatusCode)
		} else {
			slog.ErrorContext(ctx, "notifications: google chat alert failed to send", "audience", audience, "errType", fmt.Sprintf("%T", err))
		}
		return err
	}
	slog.InfoContext(ctx, "notifications: google chat alert sent", "audience", audience)
	return nil
}

// postCard marshals msg and posts it to webhookURL — the actual HTTP
// mechanics shared by sendCard/sendCardToAudience once each has resolved
// its own webhook URL.
func (c *GoogleChatClient) postCard(ctx context.Context, webhookURL string, msg chatCardMessage) error {
	// A threaded message needs chatThreadReplyOption as a query parameter,
	// not just msg.Thread's own body field -- see that constant's own doc
	// comment for why the webhook endpoint otherwise ignores threadKey.
	if msg.Thread != nil && msg.Thread.ThreadKey != "" {
		parsedURL, err := url.Parse(webhookURL)
		if err != nil {
			return fmt.Errorf("notifications: parse google chat webhook url: %w", redactURLError(err))
		}
		q := parsedURL.Query()
		q.Set("messageReplyOption", chatThreadReplyOption)
		parsedURL.RawQuery = q.Encode()
		webhookURL = parsedURL.String()
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notifications: encode google chat message: %w", err)
	}

	// A threaded message needs the space told what to do when the thread does
	// not exist yet, which is the case for a ladder's first rung. Without
	// this, Chat rejects the threadKey rather than starting the thread.
	if msg.Thread != nil {
		webhookURL, err = withThreadReplyOption(webhookURL)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifications: build google chat request: %w", redactURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: post google chat message: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notifications: read google chat response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return nil
}
