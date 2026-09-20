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

// entityIncidentClient abstracts the entity service incident operations used by IncidentHandler.
type entityIncidentClient interface {
	CreateIncident(ctx context.Context, body []byte) ([]byte, error)
	SearchIncidents(ctx context.Context, body []byte) ([]byte, error)
}

// IncidentHandler handles HTTP requests for incident operations, delegating to the
// entity service for data access. See AccountHandler's doc comment: there is no
// end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type IncidentHandler struct {
	entity entityIncidentClient
}

// NewIncidentHandler creates an IncidentHandler backed by the given entity client.
func NewIncidentHandler(entity entityIncidentClient) *IncidentHandler {
	return &IncidentHandler{entity: entity}
}

// CreateIncident handles POST /incidents. Targets a ServiceNow-backed entity-service
// operation that requires a forwarded end-user identity token — this service is
// strictly M2M with no mechanism to supply one, so calls here always receive a
// mapped 401 from upstream. Kept for API-shape completeness (see the entity-client
// method's doc comment), not because it currently succeeds. The request body is
// forwarded verbatim; the entity service enforces its own field validation and
// 400s otherwise, so this handler does not re-validate that.
func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.CreateIncident(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateIncident failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create incident.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchIncidents handles POST /incidents/search. Targets the same
// ServiceNow-backed entity-service operation family as CreateIncident above —
// it also requires a forwarded end-user identity token that this service
// cannot supply, so calls here always receive a mapped 401 from upstream.
// Kept for API-shape completeness, not because it currently succeeds. The
// request body is forwarded verbatim; the entity service enforces its own
// field validation and 400s otherwise, so this handler does not re-validate
// that.
func (h *IncidentHandler) SearchIncidents(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchIncidents(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchIncidents failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search incidents.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
