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

// StandardChangeHandler serves the standard change catalogue and its proposals.
type StandardChangeHandler struct {
	svc service.StandardChangeService
}

// NewStandardChangeHandler constructs a StandardChangeHandler.
func NewStandardChangeHandler(svc service.StandardChangeService) *StandardChangeHandler {
	return &StandardChangeHandler{svc: svc}
}

func writeStandardChangeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *StandardChangeHandler) answer(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStandardChangeJSON(w, status, v)
}

// SearchTemplates handles POST /standard-change-templates/search.
func (h *StandardChangeHandler) SearchTemplates(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchStandardChangeTemplatesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchTemplates(r.Context(), req)
	h.answer(w, r, http.StatusOK, resp, err)
}

// GetTemplate handles GET /standard-change-templates/{id}.
func (h *StandardChangeHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTemplate(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, t, err)
}

// GetTemplateVersion handles GET /standard-change-templates/{id}/versions/{versionId}.
func (h *StandardChangeHandler) GetTemplateVersion(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.GetTemplateVersion(r.Context(), r.PathValue("id"), r.PathValue("versionId"))
	h.answer(w, r, http.StatusOK, v, err)
}

// SuspendTemplate handles POST /standard-change-templates/{id}/suspend. The body
// {reason} is optional: an empty body suspends without a reason.
func (h *StandardChangeHandler) SuspendTemplate(w http.ResponseWriter, r *http.Request) {
	var req domain.SuspendStandardChangeTemplateRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	t, err := h.svc.SuspendTemplate(r.Context(), r.PathValue("id"), req)
	h.answer(w, r, http.StatusOK, t, err)
}

// ResumeTemplate handles POST /standard-change-templates/{id}/resume.
func (h *StandardChangeHandler) ResumeTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.ResumeTemplate(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, t, err)
}

// ClearTemplateReview handles POST /standard-change-templates/{id}/clear-review.
func (h *StandardChangeHandler) ClearTemplateReview(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.ClearTemplateReview(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, t, err)
}

// CreateProposal handles POST /standard-change-proposals.
func (h *StandardChangeHandler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateStandardChangeProposalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	p, err := h.svc.CreateProposal(r.Context(), req)
	h.answer(w, r, http.StatusCreated, p, err)
}

// SearchProposals handles POST /standard-change-proposals/search.
func (h *StandardChangeHandler) SearchProposals(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchStandardChangeProposalsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchProposals(r.Context(), req)
	h.answer(w, r, http.StatusOK, resp, err)
}

// GetProposal handles GET /standard-change-proposals/{id}.
func (h *StandardChangeHandler) GetProposal(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProposal(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, p, err)
}

// UpdateProposal handles PATCH /standard-change-proposals/{id}.
func (h *StandardChangeHandler) UpdateProposal(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateStandardChangeProposalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	p, err := h.svc.UpdateProposal(r.Context(), r.PathValue("id"), req)
	h.answer(w, r, http.StatusOK, p, err)
}

// SubmitProposal handles POST /standard-change-proposals/{id}/submit.
func (h *StandardChangeHandler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.SubmitProposal(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, p, err)
}

// WithdrawProposal handles POST /standard-change-proposals/{id}/withdraw.
func (h *StandardChangeHandler) WithdrawProposal(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.WithdrawProposal(r.Context(), r.PathValue("id"))
	h.answer(w, r, http.StatusOK, p, err)
}

// DecideProposal handles POST /standard-change-proposals/{id}/decision.
func (h *StandardChangeHandler) DecideProposal(w http.ResponseWriter, r *http.Request) {
	var req domain.DecideStandardChangeProposalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	p, err := h.svc.DecideProposal(r.Context(), r.PathValue("id"), req)
	h.answer(w, r, http.StatusOK, p, err)
}
