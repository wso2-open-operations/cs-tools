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

// entityServiceOfferingClient abstracts the entity service service-offering operations.
type entityServiceOfferingClient interface {
	SearchServiceOfferings(ctx context.Context, body []byte) ([]byte, error)
}

// ServiceOfferingHandler handles HTTP requests for service offering operations.
type ServiceOfferingHandler struct {
	entity entityServiceOfferingClient
}

// NewServiceOfferingHandler creates a ServiceOfferingHandler backed by the given entity client.
func NewServiceOfferingHandler(entity entityServiceOfferingClient) *ServiceOfferingHandler {
	return &ServiceOfferingHandler{entity: entity}
}

// SearchServiceOfferings handles POST /service-offerings/search.
func (h *ServiceOfferingHandler) SearchServiceOfferings(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchServiceOfferings(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchServiceOfferings failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search service offerings.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
