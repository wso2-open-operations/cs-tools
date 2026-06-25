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

// entityCatalogClient abstracts the entity service catalog operations.
type entityCatalogClient interface {
	SearchCatalogs(ctx context.Context, body []byte) ([]byte, error)
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) ([]byte, error)
}

// CatalogHandler handles HTTP requests for catalog operations.
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// SearchCatalogs handles POST /catalogs/search.
func (h *CatalogHandler) SearchCatalogs(w http.ResponseWriter, r *http.Request) {
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

	var parsed struct {
		DeployedProductID string `json:"deployedProductId"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.DeployedProductID != "" && !uuidRe.MatchString(parsed.DeployedProductID) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
			return
		}
	}

	result, err := h.entity.SearchCatalogs(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCatalogs failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search catalogs.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCatalogItemVariables handles GET /catalogs/{catalogId}/items/{catalogItemId}/variables.
func (h *CatalogHandler) GetCatalogItemVariables(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	catalogID := r.PathValue("catalogId")
	if catalogID == "" || !uuidRe.MatchString(catalogID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	catalogItemID := r.PathValue("catalogItemId")
	if catalogItemID == "" || !uuidRe.MatchString(catalogItemID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetCatalogItemVariables(r.Context(), catalogID, catalogItemID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCatalogItemVariables failed", "userID", user.UserID, "catalogID", catalogID, "catalogItemID", catalogItemID, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve catalog item variables.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
