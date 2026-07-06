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

// entityGroupClient abstracts the entity service group operations.
type entityGroupClient interface {
	SearchGroups(ctx context.Context, body []byte) ([]byte, error)
}

// GroupHandler handles HTTP requests for group operations.
type GroupHandler struct {
	entity entityGroupClient
}

// NewGroupHandler creates a GroupHandler backed by the given entity client.
func NewGroupHandler(entity entityGroupClient) *GroupHandler {
	return &GroupHandler{entity: entity}
}

// SearchGroups handles POST /groups/search.
func (h *GroupHandler) SearchGroups(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchGroups(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchGroups failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search groups.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
