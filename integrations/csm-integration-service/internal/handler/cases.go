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

// entityCaseClient abstracts the entity service case operations used by CaseHandler.
type entityCaseClient interface {
	PatchCase(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access. See AccountHandler's doc comment: there is no
// end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type CaseHandler struct {
	entity entityCaseClient
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// PatchCase handles PATCH /cases/{id}. The request body is forwarded verbatim;
// the entity service enforces its own field-combination rules and 400s
// otherwise, so this handler does not re-validate that. A state/severity/
// workState-only update succeeds for this M2M-only service on a Postgres data
// source; every other field this shape accepts is ServiceNow-data-source-only
// and requires a forwarded end-user identity token this service cannot
// supply, so those calls receive a mapped 401 from upstream.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.PatchCase(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateCaseComment handles POST /cases/{id}/comments. Targets an entity-service
// operation that requires a forwarded end-user identity token on both data
// sources (the comment's author is resolved from that token) — this service is
// strictly M2M with no mechanism to supply one, so calls here always receive a
// mapped 401 from upstream. Kept for API-shape completeness, not because it
// currently succeeds. The request body is forwarded verbatim.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.CreateCaseComment(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create case comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
