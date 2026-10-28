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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityStandardChangeClient is the entity-service calls the Standard Changes pages
// need. Each takes the raw JSON body (nil for none) and returns the raw answer.
type entityStandardChangeClient interface {
	StandardChangeCall(ctx context.Context, method, path string, body []byte) ([]byte, error)
}

// StandardChangeHandler proxies the standard change catalogue and its proposals.
// entity-service decides everything about them (who may suspend a template, edit
// or submit a proposal, decide as the CAB) and its refusals are shown as they are.
type StandardChangeHandler struct {
	entity entityStandardChangeClient
}

// NewStandardChangeHandler creates a StandardChangeHandler.
func NewStandardChangeHandler(entity entityStandardChangeClient) *StandardChangeHandler {
	return &StandardChangeHandler{entity: entity}
}

// SearchTemplates handles POST /standard-change-templates/search.
func (h *StandardChangeHandler) SearchTemplates(w http.ResponseWriter, r *http.Request) {
	h.withBody(w, r, http.MethodPost, "/standard-change-templates/search", http.StatusOK, "Failed to load standard change templates.")
}

// GetTemplate handles GET /standard-change-templates/{id}.
func (h *StandardChangeHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodGet, "/standard-change-templates/{id}", "", "Failed to load the standard change template.")
}

// GetTemplateVersion handles GET /standard-change-templates/{id}/versions/{versionId}.
func (h *StandardChangeHandler) GetTemplateVersion(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("versionId")
	if !uuidRe.MatchString(versionID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	h.noBody(w, r, http.MethodGet, "/standard-change-templates/{id}", "/versions/"+versionID, "Failed to load the template version.")
}

// SuspendTemplate handles POST /standard-change-templates/{id}/suspend.
func (h *StandardChangeHandler) SuspendTemplate(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodPost, "/standard-change-templates/{id}", "/suspend", "Failed to suspend the template.")
}

// ResumeTemplate handles POST /standard-change-templates/{id}/resume.
func (h *StandardChangeHandler) ResumeTemplate(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodPost, "/standard-change-templates/{id}", "/resume", "Failed to resume the template.")
}

// ClearTemplateReview handles POST /standard-change-templates/{id}/clear-review.
func (h *StandardChangeHandler) ClearTemplateReview(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodPost, "/standard-change-templates/{id}", "/clear-review", "Failed to clear the review flag.")
}

// CreateProposal handles POST /standard-change-proposals.
func (h *StandardChangeHandler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	h.withBody(w, r, http.MethodPost, "/standard-change-proposals", http.StatusCreated, "Failed to create the proposal.")
}

// SearchProposals handles POST /standard-change-proposals/search.
func (h *StandardChangeHandler) SearchProposals(w http.ResponseWriter, r *http.Request) {
	h.withBody(w, r, http.MethodPost, "/standard-change-proposals/search", http.StatusOK, "Failed to load proposals.")
}

// GetProposal handles GET /standard-change-proposals/{id}.
func (h *StandardChangeHandler) GetProposal(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodGet, "/standard-change-proposals/{id}", "", "Failed to load the proposal.")
}

// UpdateProposal handles PATCH /standard-change-proposals/{id}.
func (h *StandardChangeHandler) UpdateProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	h.withBody(w, r, http.MethodPatch, "/standard-change-proposals/"+id, http.StatusOK, "Failed to save the proposal.")
}

// SubmitProposal handles POST /standard-change-proposals/{id}/submit.
func (h *StandardChangeHandler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodPost, "/standard-change-proposals/{id}", "/submit", "Failed to submit the proposal.")
}

// WithdrawProposal handles POST /standard-change-proposals/{id}/withdraw.
func (h *StandardChangeHandler) WithdrawProposal(w http.ResponseWriter, r *http.Request) {
	h.noBody(w, r, http.MethodPost, "/standard-change-proposals/{id}", "/withdraw", "Failed to withdraw the proposal.")
}

// DecideProposal handles POST /standard-change-proposals/{id}/decision.
func (h *StandardChangeHandler) DecideProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	h.withBody(w, r, http.MethodPost, "/standard-change-proposals/"+id+"/decision", http.StatusOK, "Failed to record the decision.")
}

// noBody forwards a call that carries no body. base holds "{id}", replaced by the
// validated path id; suffix is appended.
func (h *StandardChangeHandler) noBody(w http.ResponseWriter, r *http.Request, method, base, suffix, fallback string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	path := strings.Replace(base, "{id}", id, 1) + suffix
	h.forward(w, r, user.UserID, method, path, nil, http.StatusOK, fallback)
}

// withBody forwards a call with the request's JSON body, checked for size and shape.
func (h *StandardChangeHandler) withBody(w http.ResponseWriter, r *http.Request, method, path string, okStatus int, fallback string) {
	body, callerID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	h.forward(w, r, callerID, method, path, body, okStatus, fallback)
}

// forward makes the call. A refusal (400, 403, 404, 409) keeps entity-service's own
// reason and errorCode so the page can show why: who may act on a proposal or a
// template is entity-service's decision and its words say what to do.
func (h *StandardChangeHandler) forward(w http.ResponseWriter, r *http.Request, callerID, method, path string, body []byte, okStatus int, fallback string) {
	result, err := h.entity.StandardChangeCall(r.Context(), method, path, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity standard change call failed", "userID", callerID, "method", method, "path", path, "err", err)
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
				writeErrorCode(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallback), upstreamErrorCode(apiErr.Body))
				return
			}
		}
		mapUpstreamError(w, err, fallback)
		return
	}
	writeJSON(w, okStatus, result)
}
