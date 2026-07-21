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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// TaskHandler handles HTTP requests for the tasks resource.
type TaskHandler struct {
	svc service.TaskService
}

// NewTaskHandler constructs a TaskHandler with the given service.
func NewTaskHandler(svc service.TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

// parsePaginationQuery parses the limit/offset query params of r, defaulting
// either to 0 (letting the service layer apply its own default) when absent.
// Returns false and writes a 400 response if a supplied value is not an integer.
func parsePaginationQuery(w http.ResponseWriter, r *http.Request) (limit int, offset int, ok bool) {
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			apierror.WriteJSON(w, http.StatusBadRequest, "limit must be an integer")
			return 0, 0, false
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			apierror.WriteJSON(w, http.StatusBadRequest, "offset must be an integer")
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}

// SearchCaseTasks handles GET /cases/{id}/tasks.
func (h *TaskHandler) SearchCaseTasks(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	limit, offset, ok := parsePaginationQuery(w, r)
	if !ok {
		return
	}

	resp, err := h.svc.SearchCaseTasks(r.Context(), caseID, limit, offset)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetTask handles GET /tasks/{id}.
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.svc.GetTask(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
