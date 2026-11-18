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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CaseEscalationAccessHandler serves who may de-escalate any case: listing the
// people an admin has named, granting and removing. The service decides who
// may; grants are PUT and removals DELETE, both idempotent.
type CaseEscalationAccessHandler struct {
	svc service.CaseEscalationAccessService
}

// NewCaseEscalationAccessHandler constructs a CaseEscalationAccessHandler.
func NewCaseEscalationAccessHandler(svc service.CaseEscalationAccessService) *CaseEscalationAccessHandler {
	return &CaseEscalationAccessHandler{svc: svc}
}

// ListDeescalators handles GET /case-escalation-access/deescalators.
func (h *CaseEscalationAccessHandler) ListDeescalators(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListDeescalators(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GrantDeescalator handles PUT /case-escalation-access/deescalators/{userId}.
func (h *CaseEscalationAccessHandler) GrantDeescalator(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.GrantDeescalator(r.Context(), r.PathValue("userId")))
}

// RevokeDeescalator handles DELETE /case-escalation-access/deescalators/{userId}.
func (h *CaseEscalationAccessHandler) RevokeDeescalator(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.RevokeDeescalator(r.Context(), r.PathValue("userId")))
}

func (h *CaseEscalationAccessHandler) done(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
