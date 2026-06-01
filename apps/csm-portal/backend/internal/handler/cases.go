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
)

// injectProjectID merges a project ID into a JSON request body as projectIds: [id].
func injectProjectID(body []byte, projectID string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	ids, err := json.Marshal([]string{projectID})
	if err != nil {
		return nil, err
	}
	m["projectIds"] = ids
	return json.Marshal(m)
}

// entityCaseClient abstracts the entity service operations used by CaseHandler,
// allowing the handler to be tested independently of the real HTTP client.
type entityCaseClient interface {
	CreateCase(ctx context.Context, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	GetCase(ctx context.Context, caseID string) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access.
type CaseHandler struct {
	entity entityCaseClient
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// CreateCase handles POST /cases.
// TODO: createdBy is currently supplied by the client. Once DB-level auth is
// in place it will be injected server-side from the authenticated user and
// removed from the request payload.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
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

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCase(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to create case.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchProjectCases handles POST /projects/{id}/cases/search.
// It enforces the project scope by injecting projectIds: [id] into the entity request.
func (h *CaseHandler) SearchProjectCases(w http.ResponseWriter, r *http.Request) {
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
		if _, ok := err.(*http.MaxBytesError); ok {
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

	entityBody, err := injectProjectID(body, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "projectID", projectID, "err", err)
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, "Case ID cannot be empty!")
		return
	}

	result, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve case details.")
		return
	}

	// TODO: Unmarshal result into a typed CaseResponse and transform it
	// to the CSM portal response shape before writing.
	writeJSON(w, http.StatusOK, result)
}
