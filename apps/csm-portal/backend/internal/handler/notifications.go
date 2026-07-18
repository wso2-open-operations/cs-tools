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
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// googleChatAlertSender abstracts the Google Chat notification channel used
// by NotificationHandler, allowing the handler to be tested independently of
// the real webhook client.
type googleChatAlertSender interface {
	SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error
}

// NotificationHandler handles HTTP requests for outbound notification channels.
type NotificationHandler struct {
	googleChat googleChatAlertSender
	// portalBaseURL is the CSM portal webapp's base URL, used to build the
	// "Open in CSM Portal" link (/operations/incidents/{caseId}) included in
	// Google Chat alerts.
	portalBaseURL string
}

// NewNotificationHandler creates a NotificationHandler backed by the given
// Google Chat client and portal base URL.
func NewNotificationHandler(googleChat googleChatAlertSender, portalBaseURL string) *NotificationHandler {
	return &NotificationHandler{
		googleChat:    googleChat,
		portalBaseURL: strings.TrimRight(portalBaseURL, "/"),
	}
}

// googleChatAlertRequest is the body accepted by PostGoogleChatAlert.
type googleChatAlertRequest struct {
	// Product selects which configured Google Chat space receives the alert
	// (e.g. "api-manager", "identity-server").
	Product          string `json:"product"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	CaseID           string `json:"caseId"`
}

// PostGoogleChatAlert handles POST /notifications/google-chat/alerts.
// It is called manually today (not yet wired into case/incident creation).
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

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	var req googleChatAlertRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Product == "" || req.Title == "" || req.ShortDescription == "" || req.CaseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	portalURL := h.portalBaseURL + "/operations/incidents/" + url.PathEscape(req.CaseID)
	if err := h.googleChat.SendIncidentAlert(r.Context(), req.Product, req.Title, req.ShortDescription, portalURL); err != nil {
		slog.ErrorContext(r.Context(), "google chat SendIncidentAlert failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to send Google Chat alert.")
		return
	}

	writeJSONValue(w, http.StatusOK, map[string]string{"message": "alert sent"})
}
