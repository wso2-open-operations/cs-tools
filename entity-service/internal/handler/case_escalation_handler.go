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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CaseEscalationHandler handles HTTP requests for a single case's escalation
// sub-resource (GET/POST /cases/{id}/escalations) -- a case-scoped
// convenience over the generic /escalations resource EscalationHandler
// serves.
type CaseEscalationHandler struct {
	svc service.CaseEscalationService
}

// NewCaseEscalationHandler constructs a CaseEscalationHandler with the given service.
func NewCaseEscalationHandler(svc service.CaseEscalationService) *CaseEscalationHandler {
	return &CaseEscalationHandler{svc: svc}
}

// SearchCaseEscalations handles GET /cases/{id}/escalations.
func (h *CaseEscalationHandler) SearchCaseEscalations(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	resp, err := h.svc.SearchCaseEscalations(r.Context(), caseID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// createCaseEscalationRequestBody is the request body for
// POST /cases/{id}/escalations -- the same shape as domain.CreateEscalationRequest
// minus CaseID, which comes from the URL path here.
type createCaseEscalationRequestBody struct {
	Reason *string                  `json:"reason,omitempty"`
	Action *domain.EscalationAction `json:"action,omitempty"`
}

// CreateCaseEscalation handles POST /cases/{id}/escalations.
func (h *CaseEscalationHandler) CreateCaseEscalation(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req createCaseEscalationRequestBody
	if !decodeRequest(w, r, &req) {
		return
	}

	resp, err := h.svc.CreateCaseEscalation(r.Context(), caseID, req.Reason, req.Action)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
