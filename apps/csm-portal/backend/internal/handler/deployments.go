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

// injectDeploymentID merges a deployment ID into a JSON request body as deploymentIds: [id].
func injectDeploymentID(body []byte, deploymentID string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	ids, err := json.Marshal([]string{deploymentID})
	if err != nil {
		return nil, err
	}
	m["deploymentIds"] = ids
	return json.Marshal(m)
}

// entityDeploymentClient abstracts the entity service deployment operations used by DeploymentHandler.
type entityDeploymentClient interface {
	PostDeployment(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
	PatchDeployment(ctx context.Context, deploymentID string, body []byte) ([]byte, error)
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

// PostDeployment handles POST /deployments.
// Forwards the request body directly to the entity service and returns 201 on success.
func (h *DeploymentHandler) PostDeployment(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.PostDeployment(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PostDeployment failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to create deployment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// PatchDeployment handles PATCH /deployments/{id}.
// Accepts name, type, description (detail fields) or active=false (deactivation) and forwards to the entity service.
func (h *DeploymentHandler) PatchDeployment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("id")
	if deploymentID == "" || !uuidRe.MatchString(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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

	result, err := h.entity.PatchDeployment(r.Context(), deploymentID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchDeployment failed", "userID", user.UserID, "deploymentID", deploymentID, "err", err)
		mapUpstreamError(w, err, "Failed to update deployment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchDeployments handles POST /deployments/search.
// Project IDs and other filters are accepted directly in the request body.
func (h *DeploymentHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchDeployments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchDeployedProducts handles POST /deployments/{id}/products/search.
func (h *DeploymentHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("id")
	if deploymentID == "" {
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

	entityBody, err := injectDeploymentID(body, deploymentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchDeployedProducts(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "userID", user.UserID, "deploymentID", deploymentID, "err", err)
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
