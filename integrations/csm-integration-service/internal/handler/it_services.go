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

// entityITServiceClient abstracts the entity service IT-service search
// operation used by ITServiceHandler.
type entityITServiceClient interface {
	SearchITServices(ctx context.Context, body []byte) ([]byte, error)
}

// ITServiceHandler handles HTTP requests for CMDB IT-service search,
// delegating to the entity service for data access. See AccountHandler's doc
// comment: there is no end-user identity checked here — Choreo's API Manager
// gateway is the trust boundary for this service's M2M/third-party
// consumers.
type ITServiceHandler struct {
	entity entityITServiceClient
}

// NewITServiceHandler creates an ITServiceHandler backed by the given entity
// client.
func NewITServiceHandler(entity entityITServiceClient) *ITServiceHandler {
	return &ITServiceHandler{entity: entity}
}

// SearchITServices handles POST /services/search. Targets the same
// ServiceNow-backed entity-service operation family as IncidentHandler's
// CreateIncident/SearchIncidents — it goes through the same M2M-credential
// fallback described there, so a mapped 401 here is possible (if the target
// environment's M2M ServiceNow credential isn't configured) but not
// guaranteed. The request body is forwarded verbatim; the entity service
// enforces its own field validation and 400s otherwise, so this handler does
// not re-validate that.
func (h *ITServiceHandler) SearchITServices(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchITServices(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchITServices failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search services.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
