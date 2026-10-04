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

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// googleChatAlertSender abstracts the Google Chat notification channel used
// by NotificationHandler, allowing the handler to be tested independently of
// the real webhook client.
type googleChatAlertSender interface {
	SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error
}

// alertIncidentClient is the incident lookup the alert content is built from.
type alertIncidentClient interface {
	GetIncident(ctx context.Context, id string) ([]byte, error)
}

// Alerts per caller allowed within alertRateWindow.
const (
	alertRateLimit  = 5
	alertRateWindow = 10 * time.Minute
)

// ErrMsgTooManyAlerts is returned with 429 when a caller exceeds the alert rate.
const ErrMsgTooManyAlerts = "Too many alerts sent. Please try again later."

// NotificationHandler handles HTTP requests for outbound notification channels.
type NotificationHandler struct {
	googleChat googleChatAlertSender
	incidents  alertIncidentClient
	// portalBaseURL is the CSM portal webapp's base URL, used to build the
	// "Open in CSM Portal" link (/operations/incidents/{caseId}) included in
	// Google Chat alerts.
	portalBaseURL string
	limiter       *alertRateLimiter
}

// NewNotificationHandler creates a NotificationHandler backed by the given
// Google Chat client, the incident lookup the alert content is built from,
// and the portal base URL.
func NewNotificationHandler(googleChat googleChatAlertSender, incidents alertIncidentClient, portalBaseURL string) *NotificationHandler {
	return &NotificationHandler{
		googleChat:    googleChat,
		incidents:     incidents,
		portalBaseURL: strings.TrimRight(portalBaseURL, "/"),
		limiter:       newAlertRateLimiter(alertRateLimit, alertRateWindow, time.Now),
	}
}

// googleChatAlertRequest is the body accepted by PostGoogleChatAlert. Title
// and ShortDescription are still accepted for compatibility but no longer
// used: the card is built from the incident record named by CaseID.
type googleChatAlertRequest struct {
	// Product selects which configured Google Chat space receives the alert
	// (e.g. "api-manager", "identity-server").
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	CaseID           string `json:"caseId"`
}

// alertIncident is the subset of an incident record an alert card shows.
type alertIncident struct {
	Number   *string `json:"number"`
	Subject  *string `json:"subject"`
	Priority *string `json:"priority"`
}

// PostGoogleChatAlert handles POST /notifications/google-chat/alerts.
//
// The card's title and text come from the incident record caseId names (its
// number, priority and subject), HTML-escaped, never from the request body,
// so a caller can only announce an incident that exists, with its real
// content. Each caller may send at most alertRateLimit alerts per
// alertRateWindow; beyond that the response is 429.
func (h *NotificationHandler) PostGoogleChatAlert(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var req googleChatAlertRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req.Product = strings.TrimSpace(req.Product)
	req.CaseID = strings.TrimSpace(req.CaseID)
	if req.Product == "" || !uuidRe.MatchString(req.CaseID) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.limiter.allow(user.UserID) {
		writeError(w, http.StatusTooManyRequests, ErrMsgTooManyAlerts)
		return
	}

	raw, err := h.incidents.GetIncident(r.Context(), req.CaseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetIncident failed", "userID", user.UserID, "caseID", req.CaseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to send Google Chat alert.")
		return
	}
	var inc alertIncident
	if err := json.Unmarshal(raw, &inc); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to send Google Chat alert.")
		return
	}
	title, text := alertCardContent(inc)

	portalURL := h.portalBaseURL + "/operations/incidents/" + url.PathEscape(req.CaseID)
	if err := h.googleChat.SendIncidentAlert(r.Context(), req.Product, title, text, portalURL); err != nil {
		// err's text is safe to log: SendIncidentAlert never wraps a URL
		// (which would carry the webhook's key/token) into its error text.
		slog.ErrorContext(r.Context(), "google chat SendIncidentAlert failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to send Google Chat alert.")
		return
	}

	writeJSONValue(w, http.StatusOK, map[string]string{"message": "alert sent"})
}

// alertCardContent builds the HTML-escaped card title ("<PRIORITY> Incident -
// <number>") and text (the subject) from an incident record.
func alertCardContent(inc alertIncident) (title, text string) {
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	number, priority, subject := deref(inc.Number), deref(inc.Priority), deref(inc.Subject)
	switch {
	case priority != "" && number != "":
		title = priority + " Incident - " + number
	case number != "":
		title = "Incident - " + number
	default:
		title = "Incident"
	}
	if subject == "" {
		subject = "(no subject)"
	}
	return html.EscapeString(title), html.EscapeString(subject)
}

// alertRateLimiter is a per-key sliding-window limiter: at most limit events
// per window. Keys with no events inside the window are dropped on access,
// so memory is bounded by the number of recently active callers.
type alertRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	events map[string][]time.Time
}

func newAlertRateLimiter(limit int, window time.Duration, now func() time.Time) *alertRateLimiter {
	return &alertRateLimiter{limit: limit, window: window, now: now, events: map[string][]time.Time{}}
}

// allow records an event for key and reports whether it is within the limit.
func (l *alertRateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-l.window)
	for k, ts := range l.events {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.events, k)
		} else {
			l.events[k] = kept
		}
	}
	if len(l.events[key]) >= l.limit {
		return false
	}
	l.events[key] = append(l.events[key], now)
	return true
}
