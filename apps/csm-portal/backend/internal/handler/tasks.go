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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityTaskClient abstracts the entity service task operations.
type entityTaskClient interface {
	SearchCaseTasks(ctx context.Context, caseID string, body []byte) ([]byte, error)
	GetTask(ctx context.Context, id string) ([]byte, error)
}

// TaskHandler handles HTTP requests for task operations.
type TaskHandler struct {
	entity entityTaskClient
}

// NewTaskHandler creates a TaskHandler backed by the given entity client.
func NewTaskHandler(entity entityTaskClient) *TaskHandler {
	return &TaskHandler{entity: entity}
}

// SearchCaseTasks handles POST /cases/{caseId}/tasks/search.
// The endpoint is path-scoped, so the request body (e.g. {"pagination":{"limit":...,"offset":...}})
// is capped and forwarded to the entity service as-is, and the response is returned verbatim.
func (h *TaskHandler) SearchCaseTasks(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" || !uuidRe.MatchString(caseID) {
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

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCaseTasks(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseTasks failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve case tasks.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetTask handles GET /tasks/{id}.
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetTask(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetTask failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve task.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
