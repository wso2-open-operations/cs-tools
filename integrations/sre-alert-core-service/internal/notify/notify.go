// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package notify sends incidents to CSM, falling back to Chat webhooks when CSM does not confirm.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"
	"golang.org/x/sync/singleflight"

	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
)

// Notifier targets CSM first, falling back to Google Chat webhooks when CSM does not confirm.
type Notifier struct {
	logger           *slog.Logger
	client           *http.Client
	csm              *csm.Client
	callerID         string
	unknownServiceID string
	// defaultAssignmentGroupID assigns an incident no other signal routes, so it still reaches a team.
	defaultAssignmentGroupID string
	// groupRoutes maps "group:<name>", "topic:<source topic>" and "account:<source account>" (lower-cased) to a CSM group id.
	groupRoutes map[string]string
	services    *serviceCache
	// serviceResolveGroup collapses concurrent cache misses for the same unresolved label into one CSM search.
	serviceResolveGroup     singleflight.Group
	fallbackChatWebhookURLs []string
	maxAttempts             int
	retryBaseDelay          time.Duration
	// chatThreadingEnabled threads each incident's fallback card and Duplicate/OK replies into one Google Chat thread keyed by chatThreadKey.
	chatThreadingEnabled bool
}

// Config groups New's dependencies to avoid a growing positional-argument list.
type Config struct {
	CallerID string
	// UnknownServiceID is used when a Service label has no CMDB match.
	UnknownServiceID string
	// DefaultAssignmentGroupID assigns an incident that no other signal routes; empty leaves it unassigned.
	DefaultAssignmentGroupID string
	// AssignmentGroupRoutes maps routing keys to CSM group ids: "group:<name>" (a group an alert names
	// for itself), "topic:<source topic>" and "account:<source account>".
	// Keys are matched ignoring case.
	AssignmentGroupRoutes map[string]string
	// ServiceCacheTTL bounds reuse of a resolved label->serviceId mapping.
	ServiceCacheTTL time.Duration
	MaxAttempts     int
	RetryBaseDelay  time.Duration
	HTTPTimeout     time.Duration
	// ChatThreadingEnabled threads Chat fallback messages per incident; see Notifier.chatThreadingEnabled.
	ChatThreadingEnabled bool
}

// New wires the notifier; a nil csm client disables CSM delivery so incidents only reach Chat.
func New(logger *slog.Logger, csm *csm.Client, cfg Config) *Notifier {
	n := &Notifier{
		logger:                   logger,
		client:                   &http.Client{Timeout: cfg.HTTPTimeout},
		csm:                      csm,
		callerID:                 cfg.CallerID,
		unknownServiceID:         cfg.UnknownServiceID,
		defaultAssignmentGroupID: cfg.DefaultAssignmentGroupID,
		groupRoutes:              normaliseRoutes(cfg.AssignmentGroupRoutes),
		services:                 newServiceCache(cfg.ServiceCacheTTL),
		fallbackChatWebhookURLs:  splitURLs(os.Getenv("FALLBACK_CHAT_WEBHOOK_URLS")),
		maxAttempts:              cfg.MaxAttempts,
		retryBaseDelay:           cfg.RetryBaseDelay,
		chatThreadingEnabled:     cfg.ChatThreadingEnabled,
	}
	if len(n.fallbackChatWebhookURLs) == 0 {
		logger.Warn("FALLBACK_CHAT_WEBHOOK_URLS not set; incidents will not reach Chat if CSM fails")
	}
	return n
}

func splitURLs(raw string) []string {
	var urls []string
	for u := range strings.SplitSeq(raw, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

// CSMEnabled reports whether a CSM client is configured.
func (n *Notifier) CSMEnabled() bool {
	return n.csm != nil
}

// DedupTag includes FirstSeen for uniqueness; millisecond precision ensures same-second recurrences get distinct tags.
func DedupTag(fingerprint string, firstSeen time.Time) string {
	return fmt.Sprintf("[fp:%s:%d]", fingerprint[:12], firstSeen.UnixMilli())
}

// NotifyCSM returns permanent=true for non-retryable rejections (non-429 4xx). CSMAttempts >= 1 already counts current attempt; only first attempts fail open on search errors.
func (n *Notifier) NotifyCSM(ctx context.Context, inc model.Incident, creationNote string) (incidentID, incidentNumber string, ok bool, permanent bool) {
	tag := DedupTag(inc.Fingerprint, inc.FirstSeen)
	if id, number, found, err := n.csm.SearchIncidentByCorrelationID(ctx, tag); err != nil {
		if inc.CSMAttempts > 1 {
			n.logger.Warn("csm dedup search failed on retry, deferring to avoid a duplicate create", "incident_number", inc.IncidentNumber, "error", err)
			return "", "", false, false
		}
		// Fail open: first attempt, so no prior create possible; search error doesn't prove no incident exists.
		n.logger.Warn("csm dedup search failed, proceeding to create", "incident_number", inc.IncidentNumber, "error", err)
	} else if found {
		n.logger.Info("found existing csm incident via dedup search, reusing", "incident_id", id, "incident_number", number)
		return id, number, true, false
	}

	svc, err := n.resolveService(ctx, inc.Service)
	if err != nil {
		n.logger.Error("service id resolution failed, will retry", "incident_number", inc.IncidentNumber, "service", inc.Service, "error", err)
		return "", "", false, false
	}
	// The routing chain still runs so its answer is visible, but nothing sends it: entity-service assigns the
	// incident to its service's support group and rejects a create that names a group.
	routedGroup, routedBy := n.assignmentGroup(inc, svc.groupID)
	n.logger.Info("assignment group resolved, not sent", "incident_number", inc.IncidentNumber, "by", routedBy, "assignment_group_id", routedGroup)

	req := n.createRequest(inc, svc, tag, creationNote)

	res, err := n.createIncidentWithRetry(ctx, tag, req)
	if err != nil {
		var apiErr *csm.Error
		perm := errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusTooManyRequests
		n.logger.Error("csm create incident failed", "incident_number", inc.IncidentNumber, "permanent", perm, "error", err)
		return "", "", false, perm
	}

	n.logger.Info("notified", "target", "csm", "incident_id", res.IncidentID, "incident_number", res.IncidentNumber)
	return res.IncidentID, res.IncidentNumber, true, false
}

// PushWorkNote is best-effort; callers must not fail the overall Outcome on error.
func (n *Notifier) PushWorkNote(ctx context.Context, incidentID, note string) error {
	if incidentID == "" {
		return fmt.Errorf("notify: cannot push work note, incident has no csm incident id yet")
	}
	return n.csm.UpdateIncident(ctx, incidentID, note)
}

// IncidentState returns found=false when CSM has no matching incident yet.
func (n *Notifier) IncidentState(ctx context.Context, incidentNumber string) (open bool, found bool, err error) {
	return n.csm.IncidentState(ctx, incidentNumber)
}

// createIncidentWithRetry re-checks dedup on each retry, since a lost response could mean CSM already created it.
func (n *Notifier) createIncidentWithRetry(ctx context.Context, tag string, req csm.CreateIncidentRequest) (*csm.CreateIncidentResult, error) {
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = n.retryBaseDelay

	attempt := 0
	return backoff.Retry(ctx, func() (*csm.CreateIncidentResult, error) {
		attempt++
		if attempt > 1 {
			// Recheck dedup on retry: prior attempt may have succeeded but lost response; CreateIncident isn't idempotent.
			id, number, found, err := n.csm.SearchIncidentByCorrelationID(ctx, tag)
			if err != nil {
				return nil, fmt.Errorf("dedup search before retry: %w", err)
			}
			if found {
				n.logger.Info("found existing csm incident via dedup search on retry, reusing", "incident_id", id, "incident_number", number)
				return &csm.CreateIncidentResult{IncidentID: id, IncidentNumber: number}, nil
			}
		}
		res, err := n.csm.CreateIncident(ctx, req)
		if err == nil {
			return res, nil
		}
		var apiErr *csm.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusTooManyRequests {
			return nil, backoff.Permanent(err)
		}
		return nil, err
	}, backoff.WithBackOff(eb), backoff.WithMaxTries(uint(n.maxAttempts)))
}

// resolvedService is a CMDB service and the group that supports it ("" when it has none).
type resolvedService struct {
	id      string
	groupID string
}

// createRequest builds the POST /incidents body. It never carries an assignment group: entity-service
// assigns the service's support group and rejects a create that names one. The contact type is sent, so a
// monitoring-raised incident still matches the SRE escalation ladder's routing.
func (n *Notifier) createRequest(inc model.Incident, svc resolvedService, tag, creationNote string) csm.CreateIncidentRequest {
	req := csm.CreateIncidentRequest{
		CallerID:      n.callerID,
		Category:      csmCategory(inc.Category),
		ServiceID:     svc.id,
		Impact:        inc.Impact,
		Urgency:       inc.Urgency,
		Subject:       incidentSubject(inc),
		CorrelationID: &tag,
	}
	if creationNote != "" {
		req.WorkNotes = &creationNote
	}
	if ct := contactTypeForSource(inc.Source); ct != "" {
		req.ContactType = &ct
	}
	return req
}

// assignmentGroup picks the incident's group from the most specific signal it has, in order: the group the
// alert named for itself, its service's CMDB support group, the topic it was sent from, the account it was sent from, then the configured default. by names the signal that decided ("none" when nothing did), for the log.
func (n *Notifier) assignmentGroup(inc model.Incident, serviceGroupID string) (id, by string) {
	if named := strings.TrimSpace(inc.AssignmentGroup); named != "" {
		if id := n.groupRoutes[routeKey("group", named)]; id != "" {
			return id, "alert"
		}
		// A UUID named in the alarm is used as-is, on purpose: a team can route its own alarms to its
		// group without a route being added here first. Editing an alarm's description is limited to the
		// account's own operators, and entity-service still rejects an id that is not a real group.
		if looksLikeGroupID(named) {
			return named, "alert"
		}
		n.logger.Warn("alert names an assignment group with no route, ignoring it", "incident_number", inc.IncidentNumber, "assignment_group", named)
	}
	if serviceGroupID != "" {
		return serviceGroupID, "service"
	}
	if id := n.groupRoutes[routeKey("topic", inc.SourceTopic)]; inc.SourceTopic != "" && id != "" {
		return id, "topic"
	}
	if id := n.groupRoutes[routeKey("account", inc.SourceAccount)]; inc.SourceAccount != "" && id != "" {
		return id, "account"
	}
	if n.defaultAssignmentGroupID != "" {
		return n.defaultAssignmentGroupID, "default"
	}
	return "", "none"
}

// routeKey is the lookup key for one routing signal.
func routeKey(kind, value string) string {
	return kind + ":" + strings.ToLower(strings.TrimSpace(value))
}

// normaliseRoutes lower-cases and trims every key ("Group: SRE - Apollo" and "group:sre - apollo" are one route).
func normaliseRoutes(routes map[string]string) map[string]string {
	out := make(map[string]string, len(routes))
	for k, v := range routes {
		kind, value, ok := strings.Cut(k, ":")
		if !ok {
			continue
		}
		out[routeKey(strings.ToLower(strings.TrimSpace(kind)), value)] = strings.TrimSpace(v)
	}
	return out
}

// looksLikeGroupID reports whether v is already a group id rather than a name. Only the dashed UUID form
// counts: entity-service rejects any other assignmentGroupId with a 400, which alert-core treats as permanent,
// so a bare ServiceNow sys_id sent as-is would stop the incident being created at all.
func looksLikeGroupID(v string) bool {
	return uuidPattern.MatchString(v)
}

// uuidPattern is entity-service's own validate.UUIDPattern (a separate Go module, so copied by hand).
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// routeKinds are the routing signals a CSM_ASSIGNMENT_GROUP_ROUTES key may name.
var routeKinds = map[string]bool{"group": true, "topic": true, "account": true}

// ValidateGroupIDs checks the configured default group and every route's group id are UUIDs, the only
// assignmentGroupId entity-service accepts; a bad one would make every incident it routes fail to create.
// It also refuses a key whose kind is not group, topic or account (it would never match anything), and two
// keys that are the same route once normalised but name different groups (which one won would depend on
// map order).
func ValidateGroupIDs(cfg Config) error {
	if v := strings.TrimSpace(cfg.DefaultAssignmentGroupID); v != "" && !looksLikeGroupID(v) {
		return fmt.Errorf("CSM_DEFAULT_ASSIGNMENT_GROUP_ID %q is not a UUID", v)
	}
	seen := make(map[string]string, len(cfg.AssignmentGroupRoutes)) // normalised key -> original key
	for k, v := range cfg.AssignmentGroupRoutes {
		kind, value, ok := strings.Cut(k, ":")
		kind = strings.ToLower(strings.TrimSpace(kind))
		if !ok || !routeKinds[kind] {
			return fmt.Errorf("CSM_ASSIGNMENT_GROUP_ROUTES key %q must start with group:, topic: or account:", k)
		}
		if !looksLikeGroupID(strings.TrimSpace(v)) {
			return fmt.Errorf("CSM_ASSIGNMENT_GROUP_ROUTES[%q] = %q is not a UUID", k, v)
		}
		key := routeKey(kind, value)
		if other, dup := seen[key]; dup && !strings.EqualFold(strings.TrimSpace(cfg.AssignmentGroupRoutes[other]), strings.TrimSpace(v)) {
			return fmt.Errorf("CSM_ASSIGNMENT_GROUP_ROUTES keys %q and %q are the same route but name different groups", other, k)
		}
		seen[key] = k
	}
	return nil
}

// contactTypes maps an alert's Source (normalised by normaliseSource) to entity-service's IncidentContactType.
// Only sources that enum names are listed; any other source (AWS, Grafana, ...) sends no contact type and
// relies on the assignment group alone.
var contactTypes = map[string]string{
	"azure":             "AZURE",
	"azuremonitor":      "AZURE",
	"site24x7":          "SITE_247",
	"site247":           "SITE_247",
	"sentinel":          "SENTINEL",
	"azuresentinel":     "SENTINEL",
	"microsoftsentinel": "SENTINEL",
}

// contactTypeForSource returns the contact type for an alert source, or "" when the enum has none for it.
func contactTypeForSource(source string) string {
	return contactTypes[normaliseSource(source)]
}

// normaliseSource lower-cases a source and drops everything but letters and digits, so "Site 24x7" and "site24x7" match.
func normaliseSource(source string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(source) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolveService returns a search error as-is, never falling back to UnknownServiceID, so callers retry.
func (n *Notifier) resolveService(ctx context.Context, label string) (resolvedService, error) {
	if label == "" {
		return resolvedService{id: n.unknownServiceID}, nil
	}
	if svc, ok := n.services.get(label, time.Now()); ok {
		return svc, nil
	}
	// Collapses concurrent same-label lookups into one CSM search on its own context (not any single caller's), so one caller's cancellation can't fail it for the others still waiting.
	resultCh := n.serviceResolveGroup.DoChan(label, func() (any, error) {
		hit, found, err := n.csm.SearchService(context.WithoutCancel(ctx), label)
		if err != nil {
			return resolvedService{}, err
		}
		if !found {
			return resolvedService{}, nil
		}
		svc := resolvedService{id: hit.ID, groupID: hit.SupportGroupID()}
		n.services.set(label, svc, time.Now())
		return svc, nil
	})
	select {
	case <-ctx.Done():
		return resolvedService{}, ctx.Err()
	case res := <-resultCh:
		if res.Err != nil {
			return resolvedService{}, res.Err
		}
		svc := res.Val.(resolvedService)
		if svc.id == "" {
			return resolvedService{id: n.unknownServiceID}, nil
		}
		return svc, nil
	}
}

var csmCategoryMap = map[string]string{
	"security": "SECURITY",
	"inquiry":  "INQUIRY",
}

// csmCategory defaults to SERVICE_INTERRUPTION since most alerts represent something breaking.
func csmCategory(category string) string {
	if v, ok := csmCategoryMap[strings.ToLower(strings.TrimSpace(category))]; ok {
		return v
	}
	return "SERVICE_INTERRUPTION"
}

// incidentSubject is the metric name alone; the dedup tag and other alert context live in WorkNotes instead of the title.
func incidentSubject(inc model.Incident) string {
	subject := inc.MetricName
	if subject == "" {
		subject = inc.Service
	}
	if inc.Fallback {
		// Chat already fired before CSM confirmed, so this create call is a delayed catch-up, not a fresh occurrence.
		subject = "[DELAYED-CSM] " + subject
	}
	return subject
}

// NotifyChat returns true only if every configured target confirms, or if none are configured.
func (n *Notifier) NotifyChat(ctx context.Context, inc model.Incident) (ok bool) {
	return n.postCardToChat(ctx, inc.IncidentNumber, fallbackGoogleChatCard(inc, n.chatThreadingEnabled))
}

// NotifyChatAnnotation threads a Duplicate/OK digest into the incident's Chat thread, so it reads as an update rather than a repeat of the "Priority Incident Reported" card.
func (n *Notifier) NotifyChatAnnotation(ctx context.Context, inc model.Incident, text string) (ok bool) {
	return n.postCardToChat(ctx, inc.IncidentNumber, annotationGoogleChatCard(inc, text, n.chatThreadingEnabled))
}

// postCardToChat posts card to every configured webhook, threading it when enabled, and returns true only if every target confirms, or if none are configured.
func (n *Notifier) postCardToChat(ctx context.Context, incidentNumber string, card map[string]any) (ok bool) {
	if len(n.fallbackChatWebhookURLs) == 0 {
		n.logger.Warn("no chat target for incident: FALLBACK_CHAT_WEBHOOK_URLS not configured", "incident_number", incidentNumber)
		return true
	}
	var wg sync.WaitGroup
	var failures atomic.Int32
	for _, chatURL := range n.fallbackChatWebhookURLs {
		wg.Add(1)
		go func(chatURL string) {
			defer wg.Done()
			spaceID := chatSpaceID(chatURL)
			target := chatURL
			if n.chatThreadingEnabled {
				target = withThreadReplyOption(chatURL)
			}
			if _, err := n.postWithRetry(ctx, target, card); err != nil {
				n.logger.Error("notify failed after retries", "target", "google_chat", "chat_space_id", spaceID, "incident_number", incidentNumber, "error", err)
				failures.Add(1)
				return
			}
			n.logger.Info("notified", "target", "google_chat", "chat_space_id", spaceID, "incident_number", incidentNumber)
		}(chatURL)
	}
	wg.Wait()
	return failures.Load() == 0
}

// chatSpaceID avoids logging the webhook URL's embedded bearer credential.
func chatSpaceID(webhookURL string) string {
	_, rest, ok := strings.Cut(webhookURL, "/spaces/")
	if !ok {
		return "unknown"
	}
	if id, _, ok := strings.Cut(rest, "/"); ok {
		return id
	}
	return "unknown"
}

// withThreadReplyOption appends messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD so Chat replies into the thread named by the card's thread.threadKey instead of always starting a new one; falls back to the original URL if it won't parse.
func withThreadReplyOption(webhookURL string) string {
	u, err := url.Parse(webhookURL)
	if err != nil {
		return webhookURL
	}
	q := u.Query()
	q.Set("messageReplyOption", "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD")
	u.RawQuery = q.Encode()
	return u.String()
}

// postWithRetry keeps 429 retryable since Chat webhooks rate-limit bursty concurrent incidents.
func (n *Notifier) postWithRetry(ctx context.Context, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = n.retryBaseDelay

	return backoff.Retry(ctx, func() ([]byte, error) {
		status, rb, err := n.post(ctx, url, body)
		if err == nil {
			return rb, nil
		}
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return nil, backoff.Permanent(err)
		}
		return nil, err
	}, backoff.WithBackOff(eb), backoff.WithMaxTries(uint(n.maxAttempts)))
}

// post returns status 0 when the request never got a response.
func (n *Notifier) post(ctx context.Context, target string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			// Strip url.Error's embedded URL so the webhook's key/token query params don't reach logs.
			return 0, nil, fmt.Errorf("%s request failed: %w", uerr.Op, uerr.Err)
		}
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return resp.StatusCode, nil, fmt.Errorf("status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// severityWord mirrors model.SeverityToNumeric's scale in reverse, title-cased for card display.
var severityWord = map[int]string{
	1: "Critical",
	2: "Major",
	3: "Minor",
	4: "Warning",
	5: "OK",
	0: "Clear",
}

// severityLabel falls back to "SEVERITY <N>" outside the known 1-5 scale.
func severityLabel(severity int) string {
	if word, ok := severityWord[severity]; ok {
		return word
	}
	return fmt.Sprintf("SEVERITY %d", severity)
}

func priorityLabel(severity int) string {
	return fmt.Sprintf("P%d - %s", severity, severityLabel(severity))
}

// chatThreadKey is unique per incident generation, so a recurrence after the dedup window starts a new thread while its Duplicate/OK replies join it.
func chatThreadKey(inc model.Incident) string {
	return fmt.Sprintf("%s-%d", inc.Fingerprint, inc.FirstSeen.UnixMilli())
}

// fallbackGoogleChatCard is titled FALLBACK since this path has no team-specific routing info; when threaded, the card carries chatThreadKey so the incident's annotations reply into it.
func fallbackGoogleChatCard(inc model.Incident, threaded bool) map[string]any {
	word := severityLabel(inc.Severity)
	subtitle := "#" + inc.IncidentNumber + " | " + inc.Service
	if inc.Environment != "" {
		subtitle += " | " + inc.Environment
	}
	shortDescription := inc.MetricName
	if shortDescription == "" {
		shortDescription = "No description provided."
	}
	category := inc.Category
	if category == "" {
		category = "Uncategorized"
	}
	card := map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": inc.IncidentNumber,
				"card": map[string]any{
					"header": map[string]any{
						"title":    "<font color='#f70707'><b>FALLBACK | " + word + " Priority Incident Reported</b></font>",
						"subtitle": subtitle,
					},
					"sections": []map[string]any{
						{
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": "<b>Short Description:</b><br>" + shortDescription}},
							},
						},
						{
							"header":                    "Incident Details",
							"collapsible":               true,
							"uncollapsibleWidgetsCount": 0,
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": "<b>Category:</b> " + category + "<br>" +
									"<b>Priority:</b> " + priorityLabel(inc.Severity) + "<br>" +
									"<b>State:</b> " + inc.Status}},
							},
						},
					},
				},
			},
		},
	}
	if threaded {
		card["thread"] = map[string]any{"threadKey": chatThreadKey(inc)}
	}
	return card
}

// annotationGoogleChatCard renders a Duplicate/OK digest as a reply without fallbackGoogleChatCard's header, so it doesn't look like a new page; note is model.BuildChatDigest's HTML.
func annotationGoogleChatCard(inc model.Incident, note string, threaded bool) map[string]any {
	card := map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": inc.IncidentNumber,
				"card": map[string]any{
					"sections": []map[string]any{
						{
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": note}},
							},
						},
					},
				},
			},
		},
	}
	if threaded {
		card["thread"] = map[string]any{"threadKey": chatThreadKey(inc)}
	}
	return card
}
