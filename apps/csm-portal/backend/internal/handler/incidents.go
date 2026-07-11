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

// entityIncidentClient abstracts the entity service incident operations used by IncidentHandler.
type entityIncidentClient interface {
	SearchIncidents(ctx context.Context, body []byte) ([]byte, error)
}

// searchIncidentsRequest mirrors the enum/format-constrained fields of the documented
// IncidentSearchPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type searchIncidentsRequest struct {
	Filters struct {
		Priorities []string `json:"priorities"`
		ParentIDs  []string `json:"parentIds"`
	} `json:"filters"`
	SortBy struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sortBy"`
}

var (
	validIncidentPriorities = map[string]bool{
		"CRITICAL": true,
		"HIGH":     true,
		"MODERATE": true,
		"LOW":      true,
		"PLANNING": true,
	}
	validIncidentSortFields = map[string]bool{"createdOn": true, "updatedOn": true, "openedOn": true}
	validIncidentSortOrders = map[string]bool{"asc": true, "desc": true}
)

// validateSearchIncidentsBody checks the filter/sort fields with a known, fixed set of
// valid values (priority enums, parentIds as UUIDs, sort field/order enums) so obviously
// invalid requests are rejected before reaching the entity service.
func validateSearchIncidentsBody(body []byte) bool {
	var req searchIncidentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	for _, p := range req.Filters.Priorities {
		if !validIncidentPriorities[p] {
			return false
		}
	}
	for _, id := range req.Filters.ParentIDs {
		if !uuidRe.MatchString(id) {
			return false
		}
	}
	if req.SortBy.Field != "" && !validIncidentSortFields[req.SortBy.Field] {
		return false
	}
	if req.SortBy.Order != "" && !validIncidentSortOrders[req.SortBy.Order] {
		return false
	}
	return true
}

// IncidentHandler handles HTTP requests for incident operations, delegating to the
// entity service for data access.
type IncidentHandler struct {
	entity entityIncidentClient
}

// NewIncidentHandler creates an IncidentHandler backed by the given entity client.
func NewIncidentHandler(entity entityIncidentClient) *IncidentHandler {
	return &IncidentHandler{entity: entity}
}

// SearchIncidents handles POST /incidents/search.
func (h *IncidentHandler) SearchIncidents(w http.ResponseWriter, r *http.Request) {
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

	if !validateSearchIncidentsBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchIncidents(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchIncidents failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search incidents.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}
