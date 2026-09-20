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

// entityOpportunityClient abstracts the entity service opportunity operations used
// by OpportunityHandler. ServiceNow data source only; read-only, so a pure M2M
// caller succeeds here (unlike ServiceNow-backed write operations elsewhere in this
// service).
type entityOpportunityClient interface {
	SearchOpportunities(ctx context.Context, body []byte) ([]byte, error)
	GetOpportunity(ctx context.Context, id string) ([]byte, error)
}

// OpportunityHandler handles HTTP requests for opportunity operations, delegating
// to the entity service for data access. See AccountHandler's doc comment: there is
// no end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type OpportunityHandler struct {
	entity entityOpportunityClient
}

// NewOpportunityHandler creates an OpportunityHandler backed by the given entity client.
func NewOpportunityHandler(entity entityOpportunityClient) *OpportunityHandler {
	return &OpportunityHandler{entity: entity}
}

// SearchOpportunities handles POST /opportunities/search.
func (h *OpportunityHandler) SearchOpportunities(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchOpportunities(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchOpportunities failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search opportunities.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetOpportunity handles GET /opportunities/{id}.
func (h *OpportunityHandler) GetOpportunity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetOpportunity(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetOpportunity failed", "opportunityID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve opportunity.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
