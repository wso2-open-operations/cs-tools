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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityAlertClient abstracts the entity service alert operations used by AlertHandler.
type entityAlertClient interface {
	GetAlert(ctx context.Context, id string) ([]byte, error)
	GetSmartAlert(ctx context.Context, id string) ([]byte, error)
}

// AlertHandler handles HTTP requests for alert and smart alert operations, delegating to
// the entity service for data access.
type AlertHandler struct {
	entity entityAlertClient
}

// NewAlertHandler creates an AlertHandler backed by the given entity client.
func NewAlertHandler(entity entityAlertClient) *AlertHandler {
	return &AlertHandler{entity: entity}
}

// GetAlert handles GET /alerts/{id}.
func (h *AlertHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetAlert(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAlert failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve alert.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetSmartAlert handles GET /smart-alerts/{id}.
func (h *AlertHandler) GetSmartAlert(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetSmartAlert(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSmartAlert failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve smart alert.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
