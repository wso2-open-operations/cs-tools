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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityAlertMuteRuleClient abstracts the entity service alert-mute-rule operations
// used by AlertMuteRuleHandler.
type entityAlertMuteRuleClient interface {
	SearchAlertMuteRules(ctx context.Context, body []byte) ([]byte, error)
	GetAlertMuteRule(ctx context.Context, id string) ([]byte, error)
	CreateAlertMuteRule(ctx context.Context, body []byte) ([]byte, error)
	PatchAlertMuteRule(ctx context.Context, id string, body []byte) ([]byte, error)
	ExtendAlertMuteRule(ctx context.Context, id string, body []byte) ([]byte, error)
	EndAlertMuteRule(ctx context.Context, id string) ([]byte, error)
	DeleteAlertMuteRule(ctx context.Context, id string) error
}

// AlertMuteRuleHandler proxies the alert-mute-rule routes 1:1 to entity-service.
//
// Entity-service owns every rule (name and window limits, the condition
// fields, which state allows which action). The writes go through
// mapUpstreamError so its 400 ("endOn must be after startOn") and 409 ("only a
// scheduled alert mute rule can be edited") reasons reach the engineer; the reads
// use mapUpstreamErrorGeneric like every other read.
type AlertMuteRuleHandler struct {
	entity entityAlertMuteRuleClient
}

// NewAlertMuteRuleHandler creates an AlertMuteRuleHandler backed by the given entity client.
func NewAlertMuteRuleHandler(entity entityAlertMuteRuleClient) *AlertMuteRuleHandler {
	return &AlertMuteRuleHandler{entity: entity}
}

// alertMuteRulePathID returns the {id} path value when it is a UUID, else writes 400.
func alertMuteRulePathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", false
	}
	return id, true
}

// readAlertMuteRuleObject reads a JSON body and requires it to be an object.
func readAlertMuteRuleObject(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, ok := readJSONBody(w, r)
	if !ok {
		return nil, false
	}
	var obj map[string]json.RawMessage
	// A JSON null unmarshals into a nil map without error, so check for nil too.
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// SearchAlertMuteRules handles POST /alert-mute-rules/search.
func (h *AlertMuteRuleHandler) SearchAlertMuteRules(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAlertMuteRuleObject(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchAlertMuteRules(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAlertMuteRules failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search alert mute rules.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetAlertMuteRule handles GET /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) GetAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	result, err := h.entity.GetAlertMuteRule(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAlertMuteRule failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve alert mute rule.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CreateAlertMuteRule handles POST /alert-mute-rules.
func (h *AlertMuteRuleHandler) CreateAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAlertMuteRuleObject(w, r)
	if !ok {
		return
	}
	result, err := h.entity.CreateAlertMuteRule(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAlertMuteRule failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to create alert mute rule.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// PatchAlertMuteRule handles PATCH /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) PatchAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	body, ok := readAlertMuteRuleObject(w, r)
	if !ok {
		return
	}
	result, err := h.entity.PatchAlertMuteRule(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchAlertMuteRule failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update alert mute rule.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ExtendAlertMuteRule handles POST /alert-mute-rules/{id}/extend. The body must carry
// a non-blank endOn; entity-service checks it is a later, in-limit RFC3339 time.
func (h *AlertMuteRuleHandler) ExtendAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req struct {
		EndOn string `json:"endOn"`
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.EndOn) == "" {
		writeError(w, http.StatusBadRequest, "endOn is required.")
		return
	}
	result, err := h.entity.ExtendAlertMuteRule(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ExtendAlertMuteRule failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to extend alert mute rule.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// EndAlertMuteRule handles POST /alert-mute-rules/{id}/end. It takes no body.
func (h *AlertMuteRuleHandler) EndAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	result, err := h.entity.EndAlertMuteRule(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity EndAlertMuteRule failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to end alert mute rule.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// DeleteAlertMuteRule handles DELETE /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) DeleteAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	if err := h.entity.DeleteAlertMuteRule(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteAlertMuteRule failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to delete alert mute rule.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
