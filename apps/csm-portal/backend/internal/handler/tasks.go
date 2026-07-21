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
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	defaultTaskLimit = 20
	maxTaskLimit     = 100
)

// entityTaskClient abstracts the entity service task operations.
type entityTaskClient interface {
	SearchCaseTasks(ctx context.Context, caseID string, query url.Values) ([]byte, error)
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

// GetCaseTasks handles GET /cases/{caseId}/tasks.
func (h *TaskHandler) GetCaseTasks(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	limit := defaultTaskLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxTaskLimit {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 100")
			return
		}
		limit = n
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	forward := url.Values{}
	forward.Set("limit", strconv.Itoa(limit))
	forward.Set("offset", strconv.Itoa(offset))

	result, err := h.entity.SearchCaseTasks(r.Context(), caseID, forward)
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
