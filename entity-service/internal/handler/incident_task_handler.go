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

// IncidentTaskHandler handles HTTP requests for the incident-tasks resource.
// Search and get only -- there is no create/update path.
type IncidentTaskHandler struct {
	svc service.IncidentTaskService
}

// NewIncidentTaskHandler constructs an IncidentTaskHandler with the given service.
func NewIncidentTaskHandler(svc service.IncidentTaskService) *IncidentTaskHandler {
	return &IncidentTaskHandler{svc: svc}
}

// SearchIncidentTasks handles POST /incident-tasks/search.
func (h *IncidentTaskHandler) SearchIncidentTasks(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchIncidentTasksRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchIncidentTasks(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// AggregateIncidentTasks handles POST /incident-tasks/aggregate.
func (h *IncidentTaskHandler) AggregateIncidentTasks(w http.ResponseWriter, r *http.Request) {
	var req domain.AggregateIncidentTasksRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AggregateIncidentTasks(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetIncidentTask handles GET /incident-tasks/{id}.
func (h *IncidentTaskHandler) GetIncidentTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.svc.GetIncidentTask(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
