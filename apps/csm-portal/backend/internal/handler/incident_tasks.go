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

// entityIncidentTaskClient abstracts the entity service incident-task operations used
// by IncidentTaskHandler. Search and get only -- there is no create/update path, same
// as the entity service's own IncidentTaskHandler.
type entityIncidentTaskClient interface {
	SearchIncidentTasks(ctx context.Context, body []byte) ([]byte, error)
	AggregateIncidentTasks(ctx context.Context, body []byte) ([]byte, error)
	GetIncidentTask(ctx context.Context, id string) ([]byte, error)
}

// IncidentTaskHandler handles HTTP requests for incident-task operations, delegating
// to the entity service for data access.
type IncidentTaskHandler struct {
	entity entityIncidentTaskClient
}

// NewIncidentTaskHandler creates an IncidentTaskHandler backed by the given entity client.
func NewIncidentTaskHandler(entity entityIncidentTaskClient) *IncidentTaskHandler {
	return &IncidentTaskHandler{entity: entity}
}

// SearchIncidentTasks handles POST /incident-tasks/search.
func (h *IncidentTaskHandler) SearchIncidentTasks(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchIncidentTasks(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchIncidentTasks failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search incident tasks.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateIncidentTasks handles POST /incident-tasks/aggregate.
// Server-side aggregation of incident tasks by a single field (e.g. state,
// assignmentGroup), capped to the top maxGroups buckets with the remainder
// folded into othersCount. The groupBy allowlist is validated upstream by
// the entity service; this layer only forwards the request and passes the
// response through as-is.
func (h *IncidentTaskHandler) AggregateIncidentTasks(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.AggregateIncidentTasks(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateIncidentTasks failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate incident tasks.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetIncidentTask handles GET /incident-tasks/{id}.
func (h *IncidentTaskHandler) GetIncidentTask(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetIncidentTask(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetIncidentTask failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve incident task.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
