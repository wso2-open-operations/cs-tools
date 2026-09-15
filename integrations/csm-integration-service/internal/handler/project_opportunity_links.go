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

// entityProjectOpportunityLinkClient abstracts the entity service
// project-opportunity-link operation used by ProjectOpportunityLinkHandler.
// ServiceNow data source only; read-only, so a pure M2M caller succeeds here.
// There is no by-id fetch for this resource — the underlying ServiceNow data has
// no single-record endpoint (search only).
type entityProjectOpportunityLinkClient interface {
	SearchProjectOpportunityLinks(ctx context.Context, body []byte) ([]byte, error)
}

// ProjectOpportunityLinkHandler handles HTTP requests for the project-opportunity
// link resource, delegating to the entity service for data access. See
// AccountHandler's doc comment: there is no end-user identity checked here —
// Choreo's API Manager gateway is the trust boundary for this service's
// M2M/third-party consumers.
type ProjectOpportunityLinkHandler struct {
	entity entityProjectOpportunityLinkClient
}

// NewProjectOpportunityLinkHandler creates a ProjectOpportunityLinkHandler backed
// by the given entity client.
func NewProjectOpportunityLinkHandler(entity entityProjectOpportunityLinkClient) *ProjectOpportunityLinkHandler {
	return &ProjectOpportunityLinkHandler{entity: entity}
}

// SearchProjectOpportunityLinks handles POST /project-opportunity-links/search.
func (h *ProjectOpportunityLinkHandler) SearchProjectOpportunityLinks(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchProjectOpportunityLinks(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjectOpportunityLinks failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search project-opportunity links.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
