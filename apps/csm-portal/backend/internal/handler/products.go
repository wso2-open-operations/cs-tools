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

// entityProductClient abstracts the entity service product operations used by ProductHandler.
type entityProductClient interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
}

// ProductHandler handles HTTP requests for product operations, delegating to the
// entity service for data access.
type ProductHandler struct {
	entity entityProductClient
}

// NewProductHandler creates a ProductHandler backed by the given entity client.
func NewProductHandler(entity entityProductClient) *ProductHandler {
	return &ProductHandler{entity: entity}
}

// SearchProducts handles POST /products/search.
func (h *ProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
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

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// TODO: Decode into a typed SearchProductsRequest and validate fields before forwarding.

	result, err := h.entity.SearchProducts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProducts failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search products.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}

// SearchProductVersions handles POST /product/{id}/versions/search.
func (h *ProductHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	productID := r.PathValue("id")
	if productID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
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

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// TODO: Decode into a typed SearchProductVersionsRequest and validate fields before forwarding.

	result, err := h.entity.SearchProductVersions(r.Context(), productID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVersions failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search product versions.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}
