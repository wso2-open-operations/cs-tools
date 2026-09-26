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

// entityScheduleClient abstracts the entity service Team Schedule operations.
type entityScheduleClient interface {
	GetScheduleCatalogue(ctx context.Context) ([]byte, error)
	SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error)
	SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error)
	GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error)
}

// ScheduleHandler handles the Team Schedule reads: who is working, when, and
// who is out of the rota.
type ScheduleHandler struct {
	entity entityScheduleClient
}

// NewScheduleHandler creates a ScheduleHandler backed by the given entity client.
func NewScheduleHandler(entity entityScheduleClient) *ScheduleHandler {
	return &ScheduleHandler{entity: entity}
}

// readScheduleBody authenticates the caller and returns the request body,
// having checked it is valid JSON and within the size limit. Returns ok=false
// when it has already written the response.
func readScheduleBody(w http.ResponseWriter, r *http.Request) (body []byte, userID string, ok bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, "", false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, "", false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, "", false
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, "", false
	}
	return body, user.UserID, true
}

// GetScheduleCatalogue handles GET /team-schedule/catalogue.
func (h *ScheduleHandler) GetScheduleCatalogue(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleCatalogue(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleCatalogue failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the schedule catalogue.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAssignments handles POST /team-schedule/assignments/search.
func (h *ScheduleHandler) SearchScheduleAssignments(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAssignments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAssignments failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search the schedule.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAbsences handles POST /team-schedule/absences/search.
func (h *ScheduleHandler) SearchScheduleAbsences(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAbsences(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAbsences failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search schedule absences.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetScheduleOnDuty handles GET /team-schedule/on-duty[?at=RFC3339] -- who is
// responsible at this instant. The `at` parameter is passed through unchanged;
// the entity service validates it.
func (h *ScheduleHandler) GetScheduleOnDuty(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleOnDuty(r.Context(), r.URL.Query().Get("at"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleOnDuty failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who is on duty.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
