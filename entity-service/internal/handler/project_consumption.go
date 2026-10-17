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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProjectConsumptionHandler handles HTTP requests for product consumption status and updates.
type ProjectConsumptionHandler struct {
	svc service.ProjectService
}

// NewProjectConsumptionHandler constructs a ProjectConsumptionHandler with the given service.
func NewProjectConsumptionHandler(svc service.ProjectService) *ProjectConsumptionHandler {
	return &ProjectConsumptionHandler{svc: svc}
}

// GetProjectConsumptionStatus handles GET and POST /projects/{id}/consumption/status.
func (h *ProjectConsumptionHandler) GetProjectConsumptionStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, err := h.svc.GetProjectConsumptionStatus(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	resp := struct {
		ApplicationID *string                         `json:"applicationId,omitempty"`
		Result        domain.ProjectConsumptionStatus `json:"result"`
	}{
		ApplicationID: status.ApplicationID,
		Result:        status,
	}

	// The result carries the project's client secret and HMAC secret keys.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateProjectConsumption handles PATCH and POST /projects/{id}/consumption.
func (h *ProjectConsumptionHandler) UpdateProjectConsumption(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.ProjectConsumptionUpdateRequest
	if !decodeRequest(w, r, &req) {
		return
	}

	ident := auth.IdentityFromContext(r.Context())
	updatedBy := "choreo-subscription-service"
	if ident.UserEmail != "" {
		updatedBy = ident.UserEmail
	} else if ident.ClientID != "" {
		updatedBy = ident.ClientID
	}

	if err := h.svc.UpdateProjectConsumption(r.Context(), id, req, updatedBy); err != nil {
		writeServiceError(w, r, err)
		return
	}

	resp := struct {
		Message string `json:"message"`
		Result  struct {
			Message string `json:"message"`
		} `json:"result"`
	}{
		Message: "Project consumption updated successfully",
		Result: struct {
			Message string `json:"message"`
		}{
			Message: "Project consumption updated successfully",
		},
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
