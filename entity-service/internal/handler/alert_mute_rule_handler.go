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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// AlertMuteRuleHandler serves alert mute rules (Operations > Alert mute rules) and the active list alert-core polls.
type AlertMuteRuleHandler struct {
	svc service.AlertMuteRuleService
}

// NewAlertMuteRuleHandler constructs an AlertMuteRuleHandler.
func NewAlertMuteRuleHandler(svc service.AlertMuteRuleService) *AlertMuteRuleHandler {
	return &AlertMuteRuleHandler{svc: svc}
}

func writeAlertMuteRuleJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// SearchAlertMuteRules handles POST /alert-mute-rules/search.
func (h *AlertMuteRuleHandler) SearchAlertMuteRules(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAlertMuteRulesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, resp)
}

// GetActiveAlertMuteRules handles GET /alert-mute-rules/active.
func (h *AlertMuteRuleHandler) GetActiveAlertMuteRules(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Active(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, resp)
}

// GetAlertMuteRule handles GET /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) GetAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, b)
}

// CreateAlertMuteRule handles POST /alert-mute-rules.
func (h *AlertMuteRuleHandler) CreateAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	var req domain.SaveAlertMuteRuleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	b, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusCreated, b)
}

// UpdateAlertMuteRule handles PATCH /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) UpdateAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchAlertMuteRuleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	b, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, b)
}

// ExtendAlertMuteRule handles POST /alert-mute-rules/{id}/extend.
func (h *AlertMuteRuleHandler) ExtendAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	var req domain.ExtendAlertMuteRuleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	b, err := h.svc.Extend(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, b)
}

// EndAlertMuteRule handles POST /alert-mute-rules/{id}/end.
func (h *AlertMuteRuleHandler) EndAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.End(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeAlertMuteRuleJSON(w, http.StatusOK, b)
}

// DeleteAlertMuteRule handles DELETE /alert-mute-rules/{id}.
func (h *AlertMuteRuleHandler) DeleteAlertMuteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
