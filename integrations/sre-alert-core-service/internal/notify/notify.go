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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"

	"alert-core-service/internal/apierror"
	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
)

// Notifier targets CSM first, falling back to Google Chat webhooks when CSM does not confirm.
type Notifier struct {
	logger                  *slog.Logger
	client                  *http.Client
	csm                     *csm.Client
	callerID                string
	unknownServiceID        string
	services                *serviceCache
	fallbackChatWebhookURLs []string
	maxAttempts             int
	retryBaseDelay          time.Duration
}

// Config groups New's dependencies to avoid a growing positional-argument list.
type Config struct {
	CallerID string
	// UnknownServiceID is used when a Service label has no CMDB match.
	UnknownServiceID string
	// ServiceCacheTTL bounds reuse of a resolved label->serviceId mapping.
	ServiceCacheTTL time.Duration
	MaxAttempts     int
	RetryBaseDelay  time.Duration
	HTTPTimeout     time.Duration
}

func New(logger *slog.Logger, csm *csm.Client, cfg Config) *Notifier {
	n := &Notifier{
		logger:                  logger,
		client:                  &http.Client{Timeout: cfg.HTTPTimeout},
		csm:                     csm,
		callerID:                cfg.CallerID,
		unknownServiceID:        cfg.UnknownServiceID,
		services:                newServiceCache(cfg.ServiceCacheTTL),
		fallbackChatWebhookURLs: splitURLs(os.Getenv("FALLBACK_CHAT_WEBHOOK_URLS")),
		maxAttempts:             cfg.MaxAttempts,
		retryBaseDelay:          cfg.RetryBaseDelay,
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

// DedupTag includes FirstSeen for uniqueness; millisecond precision ensures same-second recurrences get distinct tags.
func DedupTag(fingerprint string, firstSeen time.Time) string {
	return fmt.Sprintf("[fp:%s:%d]", fingerprint[:12], firstSeen.UnixMilli())
}

// NotifyCSM returns permanent=true only for a 400 (invalid payload, will never succeed). CSMAttempts >= 1 already counts current attempt; only first attempts fail open on search errors.
func (n *Notifier) NotifyCSM(ctx context.Context, inc model.Incident) (incidentID, incidentNumber string, ok bool, permanent bool) {
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

	serviceID, err := n.resolveServiceID(ctx, inc.Service)
	if err != nil {
		n.logger.Error("service id resolution failed, will retry", "incident_number", inc.IncidentNumber, "service", inc.Service, "error", err)
		return "", "", false, false
	}

	req := csm.CreateIncidentRequest{
		CallerID:      n.callerID,
		Category:      csmCategory(inc.Category),
		ServiceID:     serviceID,
		Impact:        inc.Impact,
		Urgency:       inc.Urgency,
		Subject:       incidentSubject(inc),
		CorrelationID: &tag,
	}
	if inc.Description != "" {
		req.WorkNotes = &inc.Description
	}
	if inc.Environment != "" {
		env := truncateRunes(inc.Environment, maxEnvironmentLen)
		req.Environment = &env
	}

	res, err := n.createIncidentWithRetry(ctx, tag, req)
	if err != nil {
		// Only 400 means the payload itself is invalid and will never succeed; everything else
		// (401/403/404/409/429/5xx) is the kind of transient CSM-side condition maxCSMAttempts'
		// 6h retry budget exists to survive, and must not be given up on after a single attempt.
		var apiErr *apierror.Error
		perm := errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest
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
	b := backoff.WithContext(backoff.WithMaxRetries(eb, uint64(n.maxAttempts-1)), ctx)

	var result *csm.CreateIncidentResult
	attempt := 0
	err := backoff.Retry(func() error {
		attempt++
		if attempt > 1 {
			// Recheck dedup on retry: prior attempt may have succeeded but lost response; CreateIncident isn't idempotent.
			id, number, found, err := n.csm.SearchIncidentByCorrelationID(ctx, tag)
			if err != nil {
				return fmt.Errorf("dedup search before retry: %w", err)
			}
			if found {
				n.logger.Info("found existing csm incident via dedup search on retry, reusing", "incident_id", id, "incident_number", number)
				result = &csm.CreateIncidentResult{IncidentID: id, IncidentNumber: number}
				return nil
			}
		}
		res, err := n.csm.CreateIncident(ctx, req)
		if err == nil {
			result = res
			return nil
		}
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusTooManyRequests {
			return backoff.Permanent(err)
		}
		return err
	}, b)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveServiceID returns a search error as-is, never falling back to UnknownServiceID, so callers retry.
func (n *Notifier) resolveServiceID(ctx context.Context, label string) (string, error) {
	if label == "" {
		return n.unknownServiceID, nil
	}
	if id, ok := n.services.get(label, time.Now()); ok {
		return id, nil
	}
	id, err := n.csm.SearchServiceID(ctx, label)
	if err != nil {
		return "", err
	}
	if id == "" {
		return n.unknownServiceID, nil
	}
	n.services.set(label, id, time.Now())
	return id, nil
}

// maxEnvironmentLen matches ServiceNow's custom incident.u_enviroment field's max_length.
const maxEnvironmentLen = 40

// truncateRunes bounds s to at most n runes, so a caller-supplied value never
// overflows a downstream fixed-width field like ServiceNow's u_enviroment.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
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
	if len(n.fallbackChatWebhookURLs) == 0 {
		n.logger.Warn("no chat target for incident: FALLBACK_CHAT_WEBHOOK_URLS not configured", "incident_number", inc.IncidentNumber)
		return true
	}
	card := fallbackGoogleChatCard(inc)
	var wg sync.WaitGroup
	var failures atomic.Int32
	for _, chatURL := range n.fallbackChatWebhookURLs {
		wg.Add(1)
		go func(chatURL string) {
			defer wg.Done()
			spaceID := chatSpaceID(chatURL)
			if _, err := n.postWithRetry(ctx, chatURL, card); err != nil {
				n.logger.Error("notify failed after retries", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber, "error", err)
				failures.Add(1)
				return
			}
			n.logger.Info("notified", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber)
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

// postWithRetry keeps 429 retryable since Chat webhooks rate-limit bursty concurrent incidents.
func (n *Notifier) postWithRetry(ctx context.Context, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = n.retryBaseDelay
	b := backoff.WithContext(backoff.WithMaxRetries(eb, uint64(n.maxAttempts-1)), ctx)

	var respBody []byte
	err = backoff.Retry(func() error {
		status, rb, err := n.post(ctx, url, body)
		if err == nil {
			respBody = rb
			return nil
		}
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return backoff.Permanent(err)
		}
		return err
	}, b)
	if err != nil {
		return nil, err
	}
	return respBody, nil
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

// fallbackGoogleChatCard is titled FALLBACK since this path has no team-specific routing info.
func fallbackGoogleChatCard(inc model.Incident) map[string]any {
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
	return map[string]any{
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
}
