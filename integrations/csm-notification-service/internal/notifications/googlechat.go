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
	"sync"
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
	http *http.Client
	// staticAudienceSpaces are the config-sourced spaces (GOOGLE_CHAT_SPACES,
	// INCIDENT_CHAT_SPACES, ...) — immutable after construction, same as
	// before this type gained a second, dynamic source. Meant for the
	// standing audiences that aren't teams at all ("Incident Monitor",
	// "Onboarding", "Americas", "Evaluation") — see teamAudienceSpaces below
	// for where a real team's own space now comes from instead.
	staticAudienceSpaces map[string]string
	// teamAudienceSpaces holds each team's own Chat webhook URL, resolved
	// from entity-service's "team" table (team.chat_webhook_url) rather than
	// hand-listed in GOOGLE_CHAT_SPACES — see SetTeamAudienceSpaces. Guarded
	// by mu: cmd/server/main.go refreshes this on a ticker from a background
	// goroutine while request-handling goroutines concurrently read it via
	// HasAudienceSpace/sendCardToAudience.
	mu                 sync.RWMutex
	teamAudienceSpaces map[string]string
	// routedIncidentSpaces are incident audiences (IncidentAudience, and
	// IncidentFallbackAudience) taken from the Alert Settings mapping: an
	// assignment group -> its SRE team's saved room. Set once the paging
	// configuration is loaded (SetIncidentRouteSpaces), guarded by mu, and
	// checked before staticAudienceSpaces, so the mapping wins over an
	// INCIDENT_CHAT_SPACES entry for the same group.
	routedIncidentSpaces map[string]string
}

// NewGoogleChatClient constructs a GoogleChatClient that routes alerts to the
// webhook configured for each audience in cfg.AudienceSpaces. Starts with no
// team-sourced spaces at all — SetTeamAudienceSpaces populates those once
// entity-service has been reached (see that method's own doc comment); until
// then, a team with no entry in cfg.AudienceSpaces simply has nowhere to
// route to yet, same as any other unconfigured audience.
func NewGoogleChatClient(cfg GoogleChatConfig) *GoogleChatClient {
	staticAudienceSpaces := make(map[string]string, len(cfg.AudienceSpaces))
	for _, space := range cfg.AudienceSpaces {
		audience := strings.TrimSpace(space.Audience)
		if audience == "" || strings.TrimSpace(space.WebhookURL) == "" {
			continue
		}
		if _, exists := staticAudienceSpaces[audience]; exists {
			staticAudienceSpaces[audience] = ""
			continue
		}
		staticAudienceSpaces[audience] = space.WebhookURL
	}
	return &GoogleChatClient{
		http:                 &http.Client{Timeout: 10 * time.Second},
		staticAudienceSpaces: staticAudienceSpaces,
		teamAudienceSpaces:   map[string]string{},
	}
}

// SetTeamAudienceSpaces atomically replaces the team-sourced audience spaces
// with spaces (team name -> Chat webhook URL, as entity.CustomerEntityClient.
// GetTeamChatWebhooks returns), so this client always routes by whatever the
// "team" table currently holds rather than a snapshot from process start.
// Called periodically (not just once) from cmd/server/main.go, since an
// operator can set a team's chat_webhook_url at any time without restarting
// this service. A nil or empty map clears every team-sourced space — the
// caller is expected to pass the full current set each call, not a diff.
func (c *GoogleChatClient) SetTeamAudienceSpaces(spaces map[string]string) {
	c.mu.Lock()
	c.teamAudienceSpaces = spaces
	c.mu.Unlock()
}

// webhookURLFor resolves audience to its configured Chat webhook URL, if
// any. A routed incident audience (the Alert Settings mapping) wins first;
// then staticAudienceSpaces (the standing, config-driven audiences) is
// checked first and wins on a name collision — in practice there should
// never be one, since a real team's own space now comes from
// teamAudienceSpaces exclusively, but checking the static map first keeps a
// deployment safe during the migration window where a team might still be
// listed in both.
func (c *GoogleChatClient) webhookURLFor(audience string) (string, bool) {
	audience = strings.TrimSpace(audience)
	if strings.HasPrefix(audience, IncidentAudiencePrefix) {
		c.mu.RLock()
		url := c.routedIncidentSpaces[audience]
		c.mu.RUnlock()
		if url != "" {
			return url, true
		}
	}
	if url, ok := c.staticAudienceSpaces[audience]; ok && url != "" {
		return url, true
	}
	c.mu.RLock()
	url, ok := c.teamAudienceSpaces[audience]
	c.mu.RUnlock()
	return url, ok && url != ""
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
	_, ok := c.webhookURLFor(audience)
	return ok
}

// SetIncidentRouteSpaces replaces the incident audiences taken from the Alert
// Settings mapping (incident audience -> webhook URL). Only keys under
// IncidentAudiencePrefix are kept; nil or empty clears them.
func (c *GoogleChatClient) SetIncidentRouteSpaces(spaces map[string]string) {
	routed := make(map[string]string, len(spaces))
	for audience, url := range spaces {
		audience = strings.TrimSpace(audience)
		if strings.HasPrefix(audience, IncidentAudiencePrefix) && strings.TrimSpace(url) != "" {
			routed[audience] = url
		}
	}
	c.mu.Lock()
	c.routedIncidentSpaces = routed
	c.mu.Unlock()
}

// AudienceWebhookURL is the webhook URL audience resolves to, if any, so the
// paging configuration can point an incident audience at the room an SRE
// team's audience-only chat section names. The URL is a credential and is
// never logged.
func (c *GoogleChatClient) AudienceWebhookURL(audience string) (string, bool) {
	return c.webhookURLFor(audience)
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

// chatCardWidget is a union type: exactly one of TextParagraph, ButtonList,
// Divider or DecoratedText is set per widget, matching Google Chat's widget schema.
type chatCardWidget struct {
	TextParagraph *chatTextParagraph `json:"textParagraph,omitempty"`
	ButtonList    *chatButtonList    `json:"buttonList,omitempty"`
	// Divider draws a horizontal line between the widgets around it. A
	// non-nil pointer to the empty struct marshals to `{}`, which is what
	// cardsV2 wants.
	Divider *chatDivider `json:"divider,omitempty"`
	// DecoratedText is cardsV2's DecoratedText widget. The case cards use it
	// only for its BottomLabel — the one place Chat draws smaller text, since
	// card text markup has no font-size control. See caseRefWidget.
	DecoratedText *chatDecoratedText `json:"decoratedText,omitempty"`
}

// chatDecoratedText is the subset of cardsV2's DecoratedText widget the case
// cards use. Text supports the same simple HTML as a text paragraph.
// BottomLabel is a plain string drawn below it in Chat's smaller, muted label
// style: it takes no markup, so it must not be HTML-escaped, and Chat's
// reference says it always wraps. WrapText lets Text wrap instead of
// truncating, as it would by default.
type chatDecoratedText struct {
	// TopLabel is a plain string drawn above Text in the same muted label style as BottomLabel.
	TopLabel    string `json:"topLabel,omitempty"`
	Text        string `json:"text"`
	BottomLabel string `json:"bottomLabel,omitempty"`
	WrapText    bool   `json:"wrapText,omitempty"`
}

// chatDivider is cardsV2's Divider widget: it has no fields.
type chatDivider struct{}

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

// teamTextColor is the one color every ABT team name is drawn in: the muted
// gray (#5F6368) team names have always had on these cards. It is a single
// constant on purpose: a per-team palette (Sirius blue, Atlas green, ...) and
// then a single green were both built and dropped on request, the first as
// more upkeep than it was worth (every new team would need a color) and the
// second in favor of the established gray. The team line is told apart from
// the rest of the card by being bold and first, not by hue.
const teamTextColor = "#5F6368"

// teamPart formats team as a case.*-card's first body line — bold, in
// teamTextColor — so it leads the card the way ServiceNow's alert leads with
// the team. Returns "" when team is empty, so callers can skip
// appending it (not a blank line).
func teamPart(team string) string {
	if team == "" {
		return ""
	}
	return caseAlertLine(`<font color="%s"><b>%s</b></font>`, teamTextColor, team)
}

// caseRefWidget builds the widget that identifies a case at the foot of a
// case.* card: the case reference as the main text, and the case subject
// beneath it as the smaller label.
//
// The reference is the card's link to the case — a real `<a href>` with the
// reference as its text, replacing the separate "View case" line these cards
// used to end on — so the one thing a reader taps is also the thing that says
// which case it is. It is plain link text, not bold as well: nesting `<a>`
// inside `<b>` is not something Chat's card markup documents, and the link
// styling already sets it apart. With no caseLink (none of today's callers)
// it falls back to the bold reference alone rather than an `<a href="">` that
// goes nowhere.
//
// The subject is a DecoratedText BottomLabel because that is the only way a
// card can draw text smaller than the rest: the markup Chat accepts in card
// text (b, i, u, s, font color, a, br, code, lists) has no size, and a label
// is rendered in Chat's smaller, muted style. A label is a plain string, so
// the subject is passed through verbatim, not HTML-escaped (escaping would
// show a literal "&amp;"), and takes no bold or color. A label always wraps,
// so a long subject is not cut off.
//
// These cards carry no header (a Chat card header is always drawn above
// everything else, so the team could never lead while there was one), so the
// reference and subject that used to be the header's Title/Subtitle live in
// the body instead.
func caseRefWidget(caseNumber, wso2CaseID, title, caseLink string) chatCardWidget {
	ref := chatHeaderCaseRef(caseNumber, wso2CaseID)
	text := caseAlertLine(`<b>%s</b>`, ref)
	if caseLink != "" {
		text = caseAlertLine(`<a href="%s">%s</a>`, caseLink, ref)
	}
	return chatCardWidget{DecoratedText: &chatDecoratedText{Text: text, BottomLabel: title, WrapText: true}}
}

// caseCardWidgets lays out a case.* card as two blocks with a divider between
// them: the facts about the case on top (team, then severity, then product —
// each omitted when empty, as one text paragraph), and which case it is below
// (ref, from caseRefWidget). The divider is the line the card's header used
// to draw; it is dropped, not left dangling, when there is nothing on top,
// and an empty text widget is never sent.
func caseCardWidgets(top []string, ref chatCardWidget) []chatCardWidget {
	var widgets []chatCardWidget
	if len(top) > 0 {
		widgets = append(widgets,
			chatCardWidget{TextParagraph: &chatTextParagraph{Text: strings.Join(top, "<br>")}},
			chatCardWidget{Divider: &chatDivider{}})
	}
	return append(widgets, ref)
}

// SendCaseCreatedAlert posts a card message announcing a newly created
// case, to the Google Chat space configured for audience — dispatch.go
// always resolves this to the fixed chataudience.IncidentMonitor audience
// for this event type today (see handleCaseCreated), not a per-team space.
//
// The card has no header: a Chat card header is always drawn above every
// widget, so with one the team could never be the first thing a reader
// sees — and it can't carry a color or bold either (header fields don't
// render HTML). Everything is body text instead, in two blocks with a
// divider between them (the line the header used to draw). Above it: team
// (bold, in teamTextColor), severity (colored) and productName (bold),
// each omitted entirely (no blank line) when empty. Below it: the case
// reference "<caseNumber> · <wso2CaseID>" as a link to the case, with the case
// subject beneath it in Chat's smaller label style (see caseRefWidget). There
// is no separate "View case" line: that link moved onto the reference on
// request. There is
// deliberately no leading icon or glyph on any line, and no "new" marker (a
// 🆕 on the reference was tried and removed on request: the card's place in
// the thread already says it is the first one for the case, and a severity
// change posts its own differently-shaped card). Chat's native decoratedText
// icon, or an inline emoji, only ate into the width each line needs on a
// narrow (mobile) screen. The link is a real `<a href>` text, not a button,
// since opening the case is navigation, not a genuine one-click action (an
// earlier version had "Acknowledge" and "Open in CSM" links that both pointed
// at the same place — there's no interactive card action wired up to
// actually acknowledge from Chat; see dispatch.handleCaseCreated).
func (c *GoogleChatClient) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var top []string
	if team != "" {
		top = append(top, teamPart(team))
	}
	if severityLabel != "" {
		top = append(top, caseAlertLine(`<font color="%s"><b>%s</b></font>`, severityColor, severityLabel))
	}
	if productName != "" {
		top = append(top, caseAlertLine(`<b>%s</b>`, productName))
	}
	ref := caseRefWidget(caseNumber, wso2CaseID, title, caseLink)
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-created-alert",
				Card: chatCard{
					Sections: []chatCardSection{{Widgets: caseCardWidgets(top, ref)}},
				},
			},
		},
		Thread: &chatThread{ThreadKey: chatThreadKey(caseNumber)},
	}
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
}

// SendSecurityReportAnalysisAlert posts a card message announcing a newly
// created case of type "security_report_analysis", to the Google Chat
// space configured for audience. A dedicated card rather than a variant of
// SendCaseCreatedAlert: that card's severity line only makes sense for
// type=="case" (severity is only ever set for that type — see
// entity-service's own validateCreateCaseRequest/sla_policy.go), so this
// shows a fixed case-type label instead of a severity line. Same headerless,
// two-block layout as SendCaseCreatedAlert (team, the fixed "Security Report
// Analysis" label where a severity would sit, product — divider — linked
// case ref, subject), with the same single link (on the case reference), not
// a button and not an "acknowledge" action — matching the product decision
// behind SendCaseCreatedAlert's own single consistent link (see that
// function's own doc comment).
func (c *GoogleChatClient) SendSecurityReportAnalysisAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var top []string
	if team != "" {
		top = append(top, teamPart(team))
	}
	top = append(top, "<b>Security Report Analysis</b>")
	if productName != "" {
		top = append(top, caseAlertLine(`<b>%s</b>`, productName))
	}
	ref := caseRefWidget(caseNumber, wso2CaseID, title, caseLink)
	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "security-report-analysis-created-alert",
				Card: chatCard{
					Sections: []chatCardSection{{Widgets: caseCardWidgets(top, ref)}},
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
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
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
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
}

// SendSecurityReportAnalysisAcknowledgedAlert is SendCaseAcknowledgedAlert's
// counterpart for a security_report_analysis case — a dedicated card rather
// than a variant of that one, same reasoning as SendSecurityReportAnalysisAlert's
// own doc comment: severity is never set for this case type, so
// SendCaseAcknowledgedAlert's own severity line would render "Unknown"
// instead of just not being there. Two lines instead of three — the case
// reference, then "Ack by <name> · View case" — everything
// SendCaseAcknowledgedAlert has except its leading severity line.
func (c *GoogleChatClient) SendSecurityReportAnalysisAcknowledgedAlert(ctx context.Context, audience, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error {
	if caseNumber == "" || acknowledgerName == "" {
		return fmt.Errorf("notifications: caseNumber and acknowledgerName are required")
	}
	caseRefLine := caseAlertLine(`%s`, caseNumber)
	if wso2CaseID != "" {
		caseRefLine += " · " + caseAlertLine(`%s`, wso2CaseID)
	}
	closeLine := caseAlertLine(`Ack by %s`, acknowledgerName) + " · " + caseAlertLine(`<a href="%s">View case</a>`, caseLink)
	text := strings.Join([]string{caseRefLine, closeLine}, "<br>")

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "security-report-analysis-acknowledged-alert",
				Card: chatCard{
					Sections: []chatCardSection{
						{Widgets: []chatCardWidget{{TextParagraph: &chatTextParagraph{Text: text}}}},
					},
				},
			},
		},
		// Same thread as the case's own security-report-analysis-created-alert
		// card — see SendSecurityReportAnalysisAlert's own Thread field doc
		// comment.
		Thread: &chatThread{ThreadKey: chatThreadKey(caseNumber)},
	}
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
}

// SendSeverityChangedAlert posts a card message announcing a case's
// severity changed, to the same Google Chat space as its case.created
// alert. Same headerless, two-block layout as SendCaseCreatedAlert, so the
// two read alike and only the severity line differs: above the divider, team
// (bold, colored), the old→new severity transition — each colored by its own
// resolved color, not just the new one, so a severity reads the same way here
// as it does on the other two cards — separated by an arrow, and productName
// (bold); below it, the case reference as a link to the case, and the case
// subject. Team, transition and product are each omitted when empty. No
// button: there's no genuine action to take from this card, only navigation,
// which the link already covers.
func (c *GoogleChatClient) SendSeverityChangedAlert(ctx context.Context, audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	var top []string
	if team != "" {
		top = append(top, teamPart(team))
	}
	if oldSeverityLabel != "" && newSeverityLabel != "" {
		top = append(top, caseAlertLine(`<font color="%s"><b>%s</b></font> → <font color="%s"><b>%s</b></font>`, oldSeverityColor, oldSeverityLabel, newSeverityColor, newSeverityLabel))
	}
	if productName != "" {
		top = append(top, caseAlertLine(`<b>%s</b>`, productName))
	}
	ref := caseRefWidget(caseNumber, wso2CaseID, title, caseLink)

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "case-severity-changed-alert",
				Card: chatCard{
					Sections: []chatCardSection{
						{Widgets: caseCardWidgets(top, ref)},
					},
				},
			},
		},
	}
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
}

// frustrationBands maps a frustration score (ai-escalate-comment-detector's
// own 0.0-1.0 scale) to a label/color pair for SendFrustrationAlert's score
// row, highest band first (frustrationLabelAndColor stops at the first
// min it meets or exceeds). High and Critical deliberately reuse
// severityDisplay's exact orange/red (dispatch.go) rather than inventing a
// new palette, so a frustration alert's color reads as the same visual
// system as a case severity; Mild/Moderate have no case-severity color to
// borrow (severity has no "this is fine" band), so they're new and
// deliberately more muted than the two borrowed colors.
var frustrationBands = []struct {
	min          float64
	label, color string
}{
	{0.90, "Critical", "#DC2626"},
	{0.70, "High", "#F97316"},
	{0.40, "Moderate", "#D97706"},
	{0, "Mild", "#16A34A"},
}

// frustrationLabelAndColor resolves a frustration score to the label/color
// frustrationBands assigns it (Critical >= 0.90, High >= 0.70, Moderate >=
// 0.40, else Mild). A score below 0 (shouldn't happen, but this must never
// panic on one) falls through to the last band rather than returning
// nothing.
func frustrationLabelAndColor(level float64) (label, color string) {
	for _, b := range frustrationBands {
		if level >= b.min {
			return b.label, b.color
		}
	}
	last := frustrationBands[len(frustrationBands)-1]
	return last.label, last.color
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
// type.
//
// Unlike every other card in this file, the frustration score — not the
// case — is the headline, by explicit product request: the header's Title
// is "🚨 Frustration detected" and the case reference moves into the
// Subtitle. Chat always draws a header's Title larger/bolder than its
// Subtitle, with no way to reorder, resize or color either field (see
// SendCaseCreatedAlert's own doc comment on header fields not rendering
// HTML) — swapping which string occupies which field is the only lever
// available, and it's enough: the score needing attention reads first, which
// case it's about reads second, the reverse of every case.*/incident.* card
// here.
//
// Below the header: the score gets its own DecoratedText row (TopLabel
// "Frustration level", a bold colored Text "<score> · <band>" from
// frustrationLabelAndColor) rather than sharing a line with anything else —
// the one thing on the card that needs picking out at a glance gets its own
// line, not a shared one. No icon on that row: this file's established rule
// (SendCaseCreatedAlert's own doc comment) is that Chat's native
// decoratedText icon, or an inline emoji, costs a narrow screen's width with
// no added information — this card follows the same rule rather than
// carving out an exception.
//
// Then a divider, then product (bold) and the model's own reason as plain
// text, then a "View case" button (ButtonList) — a deliberate, explicitly
// requested exception to this file's own "navigation is a link, not a
// button" convention for case.* cards (see SendCaseCreatedAlert's own doc
// comment): this card gets a one-tap action the way the SR-created card's
// own button does.
func (c *GoogleChatClient) SendFrustrationAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, reason string, frustrationLevel float64, caseLink string) error {
	if caseNumber == "" {
		return fmt.Errorf("notifications: caseNumber is required")
	}
	label, color := frustrationLabelAndColor(frustrationLevel)
	// caseAlertLine stringifies every arg via fmt.Sprint before escaping it
	// (see its own doc comment) -- %.2f against the already-stringified arg
	// would fail with a Go fmt verb mismatch (%!f(string=...)), so the float
	// is formatted here, before caseAlertLine ever sees it, and handed in
	// through %s like every other caseAlertLine call in this file.
	scoreText := caseAlertLine(`<font color="%s"><b>%s</b></font> · <font color="%s"><b>%s</b></font>`,
		color, fmt.Sprintf("%.2f", frustrationLevel), color, label)

	var lines []string
	if productName != "" {
		lines = append(lines, caseAlertLine(`<b>%s</b>`, productName))
	}
	if reason != "" {
		lines = append(lines, caseAlertLine(`%s`, reason))
	}

	widgets := []chatCardWidget{
		{DecoratedText: &chatDecoratedText{TopLabel: "Frustration level", Text: scoreText, WrapText: true}},
		{Divider: &chatDivider{}},
	}
	if len(lines) > 0 {
		widgets = append(widgets, chatCardWidget{TextParagraph: &chatTextParagraph{Text: strings.Join(lines, "<br>")}})
	}
	widgets = append(widgets, chatCardWidget{ButtonList: &chatButtonList{Buttons: []chatButton{
		{Text: "View case", OnClick: chatOnClick{OpenLink: chatOpenLink{URL: caseLink}}},
	}}})

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "frustration-alert",
				Card: chatCard{
					Header:   &chatCardHeader{Title: "🚨 Frustration detected", Subtitle: chatHeaderCaseRef(caseNumber, wso2CaseID)},
					Sections: []chatCardSection{{Widgets: widgets}},
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
	return c.sendCardToAudience(ctx, audience, caseNumber, "", msg)
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
//
// number/id are the case/SR/incident's own human-readable number (e.g.
// "CS0379063") and raw id (a UUID), logged on every line here alongside
// audience — every caller already has both in hand (or, for the escalation
// ladder, just a human reference — see its own call sites), so it costs
// nothing to thread them through. Added because "google chat alert sent"
// with only an audience gave no way to confirm which specific case/incident
// a given send was actually for when investigating a report that an alert
// didn't arrive; a number/id is the same class of safe-to-log value
// audience already is, never recipient data or case content. Either may be
// empty when a caller genuinely has nothing better (e.g. a card with no
// natural id of its own) — logged as an empty string, not omitted, so its
// absence is itself visible rather than silently missing from the line.
//
// alertType (derived from msg's own CardsV2[0].CardID — "case-created-alert",
// "frustration-alert", "sla-breach-alert", ... — never a second parameter
// every caller would otherwise have to pass) says WHICH kind of alert this
// was: "google chat alert sent" alone couldn't tell a routine case-created
// post apart from a frustration-detection alert for the same case, which
// matters when investigating whether a *specific* alert (not just any alert
// for that case) actually went out.
func (c *GoogleChatClient) sendCardToAudience(ctx context.Context, audience, number, id string, msg chatCardMessage) error {
	var alertType string
	if len(msg.CardsV2) > 0 {
		alertType = msg.CardsV2[0].CardID
	}
	webhookURL, ok := c.webhookURLFor(audience)
	if !ok {
		slog.WarnContext(ctx, "notifications: no google chat space configured for audience; alert was not posted to it", "audience", audience, "number", number, "id", id, "alertType", alertType)
		return nil
	}
	if err := c.postCard(ctx, webhookURL, msg); err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			slog.ErrorContext(ctx, "notifications: google chat alert failed to send", "audience", audience, "number", number, "id", id, "alertType", alertType, "statusCode", apiErr.StatusCode)
		} else {
			slog.ErrorContext(ctx, "notifications: google chat alert failed to send", "audience", audience, "number", number, "id", id, "alertType", alertType, "errType", fmt.Sprintf("%T", err))
		}
		return err
	}
	slog.InfoContext(ctx, "notifications: google chat alert sent", "audience", audience, "number", number, "id", id, "alertType", alertType)
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
