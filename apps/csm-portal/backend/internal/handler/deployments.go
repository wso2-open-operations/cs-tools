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

// entityDeploymentClient abstracts the entity service deployment operations used by DeploymentHandler.
type entityDeploymentClient interface {
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
}

// DeploymentHandler handles HTTP requests for deployment operations, delegating to the
// entity service for data access.
type DeploymentHandler struct {
	entity entityDeploymentClient
}

// NewDeploymentHandler creates a DeploymentHandler backed by the given entity client.
func NewDeploymentHandler(entity entityDeploymentClient) *DeploymentHandler {
	return &DeploymentHandler{entity: entity}
}

// SearchDeployments handles POST /projects/{id}/deployments/search.
func (h *DeploymentHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" {
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

	// TODO: Decode into a typed SearchDeploymentsRequest and validate fields before forwarding.

	result, err := h.entity.SearchDeployments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}
