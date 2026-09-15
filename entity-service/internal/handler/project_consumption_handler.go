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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProjectConsumptionHandler handles HTTP requests for a project's
// product-consumption provisioning state.
type ProjectConsumptionHandler struct {
	svc service.ProjectConsumptionService
}

// NewProjectConsumptionHandler constructs a ProjectConsumptionHandler with the
// given service.
func NewProjectConsumptionHandler(svc service.ProjectConsumptionService) *ProjectConsumptionHandler {
	return &ProjectConsumptionHandler{svc: svc}
}

// GetProjectConsumption handles GET /projects/{id}/consumption.
func (h *ProjectConsumptionHandler) GetProjectConsumption(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectConsumption(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateProjectConsumption handles PATCH /projects/{id}/consumption.
func (h *ProjectConsumptionHandler) UpdateProjectConsumption(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateProjectConsumptionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.UpdateProjectConsumption(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
