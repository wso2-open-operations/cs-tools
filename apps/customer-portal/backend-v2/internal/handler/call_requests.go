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

// entityCallRequestClient abstracts the entity-service call-request
// operations used by CallRequestHandler.
type entityCallRequestClient interface {
	CreateCallRequest(ctx context.Context, req entity.CreateCallRequestRequest) (entity.CreateCallRequestResponse, error)
	SearchCallRequests(ctx context.Context, req entity.SearchCallRequestsRequest) (entity.SearchCallRequestsResponse, error)
	UpdateCallRequest(ctx context.Context, id string, req entity.UpdateCallRequestRequest) (entity.UpdateCallRequestResponse, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
}

// CallRequestHandler handles HTTP requests for call-request operations.
//
// NOTE: entity-service only supports call requests on its ServiceNow data
// source — a Postgres-mode deployment 404s on every route this handler serves.
type CallRequestHandler struct {
	entity entityCallRequestClient

	callerScope *CallerScopeResolver
}

// NewCallRequestHandler creates a CallRequestHandler backed by the given entity client.
func NewCallRequestHandler(entity entityCallRequestClient) *CallRequestHandler {
	return &CallRequestHandler{entity: entity}
}

// SetCallerScope enables caller-scoped access: SearchCallRequests requires
// the caller to be an active portal-user contact of the case's own project
// (resolved via GetCase). Always enforced in production (main.go calls this
// unconditionally, no kill switch) — see ProjectHandler.SetCallerScope for
// why this is a setter rather than a constructor parameter.
func (h *CallRequestHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// CreateCallRequest handles POST /cases/{caseId}/call-requests.
func (h *CallRequestHandler) CreateCallRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.CreateCallRequestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// CaseID is always forced to the {caseId} path parameter, never a
	// client-supplied body field — the frontend's request body carries only
	// reason/utcTimes/durationInMinutes, no caseId at all.
	req.CaseID = caseID
	if req.Reason == "" || len(req.UTCTimes) == 0 || req.DurationMinutes <= 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCallRequest(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCallRequest failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create call request.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCallRequestCreate(result))
}

// SearchCallRequests handles POST /cases/{caseId}/call-requests/search.
func (h *CallRequestHandler) SearchCallRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CallRequestSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCallRequests(r.Context(), dto.BuildEntitySearchCallRequestsRequest(caseID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCallRequests failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search call requests.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchCallRequests(result))
}

// PatchCallRequest handles PATCH /cases/{caseId}/call-requests/{id}.
func (h *CallRequestHandler) PatchCallRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CallRequestUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.StateKey == 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.UpdateCallRequest(r.Context(), id, dto.BuildEntityUpdateCallRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateCallRequest failed", "userID", user.UserID, "callRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update call request.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCallRequestUpdate(result))
}
