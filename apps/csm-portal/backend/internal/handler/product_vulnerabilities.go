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
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityProductVulnerabilityClient abstracts the entity service product vulnerability operations.
type entityProductVulnerabilityClient interface {
	SearchProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error)
	GetProductVulnerability(ctx context.Context, id string) ([]byte, error)
}

// ProductVulnerabilityHandler handles HTTP requests for product vulnerability operations.
type ProductVulnerabilityHandler struct {
	entity entityProductVulnerabilityClient
}

// NewProductVulnerabilityHandler creates a ProductVulnerabilityHandler backed by the given entity client.
func NewProductVulnerabilityHandler(entity entityProductVulnerabilityClient) *ProductVulnerabilityHandler {
	return &ProductVulnerabilityHandler{entity: entity}
}

// SearchProductVulnerabilities handles POST /products/vulnerabilities/search.
func (h *ProductVulnerabilityHandler) SearchProductVulnerabilities(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchProductVulnerabilities(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVulnerabilities failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search product vulnerabilities.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetProductVulnerability handles GET /products/vulnerabilities/{id}.
func (h *ProductVulnerabilityHandler) GetProductVulnerability(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetProductVulnerability(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProductVulnerability failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve product vulnerability.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

