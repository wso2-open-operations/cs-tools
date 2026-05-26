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
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

// updatesClient abstracts the updates service operations used by UpdatesHandler,
// allowing the handler to be tested independently of the real HTTP client.
type updatesClient interface {
	GetRecommendedUpdateLevels(ctx context.Context, userEmail string) ([]updates.RecommendedUpdateLevel, error)
	GetProductUpdateLevels(ctx context.Context) ([]updates.ProductUpdateLevel, error)
	SearchUpdatesBetweenUpdateLevels(ctx context.Context, payload updates.SearchPayload, userEmail string) (map[string]updates.UpdateLevelGroup, error)
}

// UpdatesHandler handles HTTP requests for update-related operations,
// delegating to the updates service for data access.
type UpdatesHandler struct {
	updates updatesClient
}

// NewUpdatesHandler creates an UpdatesHandler backed by the given updates client.
func NewUpdatesHandler(updates updatesClient) *UpdatesHandler {
	return &UpdatesHandler{updates: updates}
}

// GetRecommendedUpdateLevels handles GET /updates/recommended-update-levels.
// The user email is sourced from the JWT claims, not a query parameter.
func (h *UpdatesHandler) GetRecommendedUpdateLevels(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.updates.GetRecommendedUpdateLevels(r.Context(), user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "updates GetRecommendedUpdateLevels failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to get recommended update levels.")
		return
	}

	writeJSONValue(w, http.StatusOK, result)
}

// GetProductUpdateLevels handles GET /updates/product-update-levels.
func (h *UpdatesHandler) GetProductUpdateLevels(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.updates.GetProductUpdateLevels(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "updates GetProductUpdateLevels failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to get product update levels.")
		return
	}

	writeJSONValue(w, http.StatusOK, result)
}

// SearchUpdatesBetweenUpdateLevels handles POST /updates/levels/search.
func (h *UpdatesHandler) SearchUpdatesBetweenUpdateLevels(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload updates.SearchPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.updates.SearchUpdatesBetweenUpdateLevels(r.Context(), payload, user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "updates SearchUpdatesBetweenUpdateLevels failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search updates.")
		return
	}

	writeJSONValue(w, http.StatusOK, result)
}
