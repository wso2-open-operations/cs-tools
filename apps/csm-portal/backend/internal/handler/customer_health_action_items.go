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
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/risk"
)

// CreateActionItem handles POST
// /customer-health/risks/{riskId}/action-items.
func (h *CustomerHealthHandler) CreateActionItem(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	riskID, ok := parseIntPathValue(w, r, "riskId")
	if !ok {
		return
	}

	var payload risk.CreateActionItemRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.CreateActionItem(r.Context(), riskID, payload, user.Email)
	if err != nil {
		var valErr *risk.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Message)
			return
		}
		slog.ErrorContext(r.Context(), "risk CreateActionItem failed", "userID", user.UserID, "riskId", riskID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create action item.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// UpdateActionItemStatus handles PUT
// /customer-health/action-items/{actionItemId}/status.
func (h *CustomerHealthHandler) UpdateActionItemStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	actionItemID, ok := parseIntPathValue(w, r, "actionItemId")
	if !ok {
		return
	}

	var payload risk.UpdateActionItemStatusRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.UpdateActionItemStatus(r.Context(), actionItemID, payload.Status, payload.ResolutionComment, user.Email)
	if err != nil {
		var valErr *risk.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Message)
			return
		}
		slog.ErrorContext(r.Context(), "risk UpdateActionItemStatus failed", "userID", user.UserID, "actionItemId", actionItemID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to update action item status.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// UpdateActionItem handles PUT
// /customer-health/action-items/{actionItemId}.
func (h *CustomerHealthHandler) UpdateActionItem(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	actionItemID, ok := parseIntPathValue(w, r, "actionItemId")
	if !ok {
		return
	}

	var payload risk.UpdateActionItemRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.UpdateActionItem(r.Context(), actionItemID, payload)
	if err != nil {
		var valErr *risk.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Message)
			return
		}
		slog.ErrorContext(r.Context(), "risk UpdateActionItem failed", "userID", user.UserID, "actionItemId", actionItemID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to update action item.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetActionItemsByRisk handles GET
// /customer-health/risks/{riskId}/action-items.
func (h *CustomerHealthHandler) GetActionItemsByRisk(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	riskID, ok := parseIntPathValue(w, r, "riskId")
	if !ok {
		return
	}
	statusFilter := optionalQueryParam(r, "status")

	result, err := h.risk.GetActionItemsByRisk(r.Context(), riskID, statusFilter)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetActionItemsByRisk failed", "userID", user.UserID, "riskId", riskID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve action items.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetActionItemsByAccount handles GET
// /customer-health/accounts/{accountSysId}/action-items.
func (h *CustomerHealthHandler) GetActionItemsByAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	accountSysID := r.PathValue("accountSysId")
	if accountSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	projectSysID := optionalQueryParam(r, "projectSysId")
	statusFilter := optionalQueryParam(r, "status")

	result, err := h.risk.GetActionItemsByAccount(r.Context(), accountSysID, projectSysID, statusFilter)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetActionItemsByAccount failed", "userID", user.UserID, "accountSysId", accountSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve action items.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// CreateActionItemComment handles POST
// /customer-health/action-items/{actionItemId}/comments.
func (h *CustomerHealthHandler) CreateActionItemComment(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	actionItemID, ok := parseIntPathValue(w, r, "actionItemId")
	if !ok {
		return
	}

	var payload risk.CreateCommentRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.CreateActionItemComment(r.Context(), actionItemID, payload.Comment, user.Email)
	if err != nil {
		var valErr *risk.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Message)
			return
		}
		slog.ErrorContext(r.Context(), "risk CreateActionItemComment failed", "userID", user.UserID, "actionItemId", actionItemID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to post comment.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetActionItemComments handles GET
// /customer-health/action-items/{actionItemId}/comments.
func (h *CustomerHealthHandler) GetActionItemComments(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	actionItemID, ok := parseIntPathValue(w, r, "actionItemId")
	if !ok {
		return
	}

	result, err := h.risk.GetActionItemComments(r.Context(), actionItemID)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetActionItemComments failed", "userID", user.UserID, "actionItemId", actionItemID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve comments.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}
