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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
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

// SearchCaseTasks handles POST /cases/{id}/tasks/search.
func (h *TaskHandler) SearchCaseTasks(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req domain.SearchCaseTasksRequest
	if !decodeRequest(w, r, &req) {
		return
	}

	resp, err := h.svc.SearchCaseTasks(r.Context(), caseID, req)
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

// CreateCaseTask handles POST /cases/{id}/tasks.
func (h *TaskHandler) CreateCaseTask(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req domain.CreateCaseTaskRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = caseID

	result, err := h.svc.CreateCaseTask(r.Context(), caseID, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

// UpdateTask handles PATCH /tasks/{id}.
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req domain.UpdateTaskRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = id

	result, err := h.svc.UpdateTask(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
