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
)

// entityProjectClient abstracts the entity service project operations used by ProjectHandler.
type entityProjectClient interface {
	GetProject(ctx context.Context, id string) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error)
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

// ProjectHandler handles HTTP requests for project operations, delegating to the
// entity service for data access. See AccountHandler's doc comment: there is no
// end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type ProjectHandler struct {
	entity entityProjectClient
}

// NewProjectHandler creates a ProjectHandler backed by the given entity client.
func NewProjectHandler(entity entityProjectClient) *ProjectHandler {
	return &ProjectHandler{entity: entity}
}

// GetProject handles GET /projects/{id}.
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetProject(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchProjects handles POST /projects/search.
func (h *ProjectHandler) SearchProjects(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchProjects(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjects failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search projects.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchProjectContacts handles POST /projects/{id}/contacts/search.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
func (h *ProjectHandler) SearchProjectContacts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchProjectContacts(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjectContacts failed", "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search project contacts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateProject handles PATCH /projects/{id}. This is a ServiceNow-data-source-only
// entity-service operation (used by the Account Closure Process automation to write
// closure-state fields on a project) — a caller with no forwarded x-user-id-token
// gets a mapped 401. The request body is forwarded verbatim; the entity service
// enforces its own "at least one field" business rule and 400s otherwise, so this
// handler does not re-validate that.
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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

	result, err := h.entity.UpdateProject(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateProject failed", "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update project.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
