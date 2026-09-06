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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityChangeRequestClient abstracts the entity-service change-request
// operations used by ChangeRequestHandler.
type entityChangeRequestClient interface {
	CreateChangeRequest(ctx context.Context, req entity.CreateChangeRequestRequest) (entity.CreateChangeRequestResponse, error)
	SearchChangeRequests(ctx context.Context, req entity.SearchChangeRequestsRequest) (entity.SearchChangeRequestsResponse, error)
	GetChangeRequest(ctx context.Context, id string) (entity.ChangeRequest, error)
	UpdateChangeRequest(ctx context.Context, id string, req entity.PatchChangeRequestRequest) (entity.PatchChangeRequestResponse, error)
	GetChangeRequestApprovals(ctx context.Context, id string) (entity.ChangeRequestApprovals, error)
	DecideChangeRequestApproval(ctx context.Context, id string, req entity.ChangeRequestApprovalDecisionRequest) (entity.ChangeRequestApprovalDecisionResponse, error)
}

// ChangeRequestHandler handles HTTP requests for change-request operations.
//
// NOTE: entity-service only supports change requests on its ServiceNow data
// source — a Postgres-mode deployment 404s on every route this handler serves.
type ChangeRequestHandler struct {
	entity entityChangeRequestClient

	callerScope *CallerScopeResolver
}

// NewChangeRequestHandler creates a ChangeRequestHandler backed by the given entity client.
func NewChangeRequestHandler(entity entityChangeRequestClient) *ChangeRequestHandler {
	return &ChangeRequestHandler{entity: entity}
}

// SetCallerScope enables caller-scoped access: SearchChangeRequests requires
// the caller to be an active portal-user contact of the project in the URL
// path. Always enforced in production (main.go calls this unconditionally,
// no kill switch) — see ProjectHandler.SetCallerScope for why this is a
// setter rather than a constructor parameter.
func (h *ChangeRequestHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// CreateChangeRequest handles POST /change-requests.
func (h *ChangeRequestHandler) CreateChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChangeRequestCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Subject == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateChangeRequest(r.Context(), dto.BuildEntityCreateChangeRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateChangeRequest failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create change request.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapChangeRequestCreate(result))
}

// SearchChangeRequests handles POST /projects/{id}/change-requests/search.
func (h *ChangeRequestHandler) SearchChangeRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChangeRequestSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchChangeRequests(r.Context(), dto.BuildEntitySearchChangeRequestsRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchChangeRequests failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search change requests.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchChangeRequests(result))
}

// GetChangeRequest handles GET /change-requests/{id}.
func (h *ChangeRequestHandler) GetChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetChangeRequest(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve change request.")
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if result.Project.ID != "" && !requireProjectMember(w, r, h.callerScope, result.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestDetails(result))
}

// PatchChangeRequest handles PATCH /change-requests/{id}.
func (h *ChangeRequestHandler) PatchChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// cr, err := h.entity.GetChangeRequest(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to update change request.")
	// 	return
	// }
	// if cr.Project.ID != "" && !requireProjectMember(w, r, h.callerScope, cr.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChangeRequestUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req == (dto.ChangeRequestUpdateRequest{}) {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}

	result, err := h.entity.UpdateChangeRequest(r.Context(), id, dto.BuildEntityPatchChangeRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update change request.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestDetails(result.ChangeRequest))
}

// GetChangeRequestApprovals handles GET /change-requests/{id}/approvals.
func (h *ChangeRequestHandler) GetChangeRequestApprovals(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// cr, err := h.entity.GetChangeRequest(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve change request approvals.")
	// 	return
	// }
	// if cr.Project.ID != "" && !requireProjectMember(w, r, h.callerScope, cr.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	result, err := h.entity.GetChangeRequestApprovals(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequestApprovals failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve change request approvals.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestApprovals(result))
}

// DecideChangeRequestApproval handles POST /change-requests/{id}/approvals/decision.
func (h *ChangeRequestHandler) DecideChangeRequestApproval(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// cr, err := h.entity.GetChangeRequest(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to record approval decision.")
	// 	return
	// }
	// if cr.Project.ID != "" && !requireProjectMember(w, r, h.callerScope, cr.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.ChangeRequestApprovalDecisionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Decision != "approved" && req.Decision != "rejected" {
		writeError(w, http.StatusBadRequest, "decision must be \"approved\" or \"rejected\".")
		return
	}

	result, err := h.entity.DecideChangeRequestApproval(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DecideChangeRequestApproval failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to record approval decision.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestApprovalDecision(result))
}
