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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityEngagementAllocationClient abstracts the entity service's customer
// engagement allocation operations.
type entityEngagementAllocationClient interface {
	SearchEngagementAllocations(ctx context.Context, body []byte) ([]byte, error)
	GetEngagementAllocation(ctx context.Context, id string) ([]byte, error)
	ListEngagementAllocationUpdates(ctx context.Context, id, limit, offset string) ([]byte, error)
	CreateEngagementAllocationUpdate(ctx context.Context, id string, body []byte) ([]byte, error)
}

// EngagementAllocationHandler proxies the customer engagement allocation list,
// detail and status updates. It is a thin pass-through: the entity service
// validates the input and decides who may do what (internal staff only; only
// the allocated engineer may post an update), identifying the caller from the
// forwarded user token. Nothing is resolved or compared here.
type EngagementAllocationHandler struct {
	entity entityEngagementAllocationClient
}

// NewEngagementAllocationHandler creates an EngagementAllocationHandler.
func NewEngagementAllocationHandler(entity entityEngagementAllocationClient) *EngagementAllocationHandler {
	return &EngagementAllocationHandler{entity: entity}
}

// SearchEngagementAllocations handles POST /engagement-allocations/search. The
// body is forwarded unchanged and the page returned as sent.
func (h *EngagementAllocationHandler) SearchEngagementAllocations(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchEngagementAllocations(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchEngagementAllocations failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search engagement allocations.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// allocationIDParam returns the {id} path value, or writes a 400 and reports false.
func allocationIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return "", false
	}
	return id, true
}

// GetEngagementAllocation handles GET /engagement-allocations/{id}.
func (h *EngagementAllocationHandler) GetEngagementAllocation(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id, ok := allocationIDParam(w, r)
	if !ok {
		return
	}
	result, err := h.entity.GetEngagementAllocation(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetEngagementAllocation failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the engagement allocation.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListEngagementAllocationUpdates handles GET /engagement-allocations/{id}/updates.
// limit and offset are passed through; the entity service validates them.
func (h *EngagementAllocationHandler) ListEngagementAllocationUpdates(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id, ok := allocationIDParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	result, err := h.entity.ListEngagementAllocationUpdates(r.Context(), id, q.Get("limit"), q.Get("offset"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListEngagementAllocationUpdates failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the allocation updates.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateEngagementAllocationUpdate handles POST /engagement-allocations/{id}/updates.
// The body ({subject, content}) is forwarded unchanged. The entity service
// refuses anyone but the allocated engineer, and the portal does not second-guess
// it: that refusal and a validation failure keep their own reason, so the caller
// can say why nothing was posted.
func (h *EngagementAllocationHandler) CreateEngagementAllocationUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := allocationIDParam(w, r)
	if !ok {
		return
	}
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateEngagementAllocationUpdate(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateEngagementAllocationUpdate failed", "userID", userID, "err", err)
		mapAllocationUpdateError(w, err, "Failed to post the update.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// mapAllocationUpdateError is mapUpstreamErrorGeneric except that a 403 (not the
// allocated engineer) and a 400 (missing or too long subject or content) keep
// entity-service's own message, which is written for the person posting. Every
// other status stays generic, so nothing internal reaches the page.
func mapAllocationUpdateError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusBadRequest) {
		writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
		return
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}
