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

// entityAlertIncidentMappingClient abstracts the entity service alert-incident-mapping
// operations used by AlertIncidentMappingHandler.
type entityAlertIncidentMappingClient interface {
	CreateAlertIncidentMapping(ctx context.Context, body []byte) ([]byte, error)
	LookupAlertIncidentMappings(ctx context.Context, body []byte) ([]byte, error)
}

// AlertIncidentMappingHandler handles HTTP requests for alert-incident-mapping
// operations, delegating to the entity service for data access. Unlike
// IncidentHandler above, the entity-service operations behind these two
// endpoints are Postgres-only with no ServiceNow dependency, so no forwarded
// end-user identity is required — this service's M2M identity is sufficient
// and these endpoints are expected to actually succeed today. See
// AccountHandler's doc comment: there is no end-user identity checked here —
// Choreo's API Manager gateway is the trust boundary for this service's
// M2M/third-party consumers.
type AlertIncidentMappingHandler struct {
	entity entityAlertIncidentMappingClient
}

// NewAlertIncidentMappingHandler creates an AlertIncidentMappingHandler backed
// by the given entity client.
func NewAlertIncidentMappingHandler(entity entityAlertIncidentMappingClient) *AlertIncidentMappingHandler {
	return &AlertIncidentMappingHandler{entity: entity}
}

// CreateAlertIncidentMapping handles POST /alert-incident-mappings. Targets a
// Postgres-only entity-service operation with no ServiceNow dependency — this
// service's M2M identity is sufficient, so this call is expected to actually
// succeed today, unlike CreateIncident above. The request body is forwarded
// verbatim; the entity service enforces its own field validation and 400s
// otherwise, so this handler does not re-validate that.
func (h *AlertIncidentMappingHandler) CreateAlertIncidentMapping(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.CreateAlertIncidentMapping(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAlertIncidentMapping failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create alert-incident mapping.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// LookupAlertIncidentMappings handles POST /alert-incident-mappings/lookup.
// Same Postgres-only, M2M-friendly situation as CreateAlertIncidentMapping
// above — expected to actually succeed today. The request body is forwarded
// verbatim; the entity service enforces its own field validation and 400s
// otherwise, so this handler does not re-validate that. An empty match
// returns 200 with an empty `mappings` array, not 404.
func (h *AlertIncidentMappingHandler) LookupAlertIncidentMappings(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.LookupAlertIncidentMappings(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity LookupAlertIncidentMappings failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to look up alert-incident mappings.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
