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
	UpdateIncident(ctx context.Context, id string, body []byte) ([]byte, error)
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
// operation. This service is strictly M2M and has no mechanism to forward an
// end-user identity token, but that operation's ServiceNow layer falls back to a
// separately-configured M2M ServiceNow credential when no end-user token is
// present, and only 401s if that fallback credential is itself unconfigured in
// the target environment — so a mapped 401 here is possible, not guaranteed. A
// live end-to-end call through this exact path against wso2sndev on 2026-09-20
// succeeded with no 401, creating a real incident (INC0096966). See the
// entity-client method's doc comment.
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

// PatchIncident handles PATCH /incidents/{id}. Unlike PATCH /cases/{id}
// (CaseHandler.PatchCase), this operation has no Postgres-data-source path:
// on a Postgres data source entity-service rejects it outright (503, not
// supported on this data source yet — no field combination succeeds there
// today); on a ServiceNow data source it goes through the same
// M2M-credential-fallback mechanism as CreateIncident/SearchIncidents above,
// so a mapped 401 here is possible (if that credential isn't configured in
// the target environment) but not guaranteed. The request body is forwarded
// verbatim; the entity service enforces its own field validation and 400s
// otherwise, so this handler does not re-validate that.
func (h *IncidentHandler) PatchIncident(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.UpdateIncident(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateIncident failed", "incidentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update incident.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchIncidents handles POST /incidents/search. Targets the same
// ServiceNow-backed entity-service operation family as CreateIncident above —
// it goes through the same M2M-credential fallback described there, so a
// mapped 401 here is possible (if the target environment's M2M ServiceNow
// credential isn't configured) but not guaranteed. The request body is
// forwarded verbatim; the entity service enforces its own field validation
// and 400s otherwise, so this handler does not re-validate that.
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
