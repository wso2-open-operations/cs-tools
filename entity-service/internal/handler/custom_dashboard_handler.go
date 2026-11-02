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

// CustomDashboardHandler handles HTTP requests for the caller's saved custom
// dashboards.
type CustomDashboardHandler struct {
	svc service.CustomDashboardService
}

// NewCustomDashboardHandler constructs a CustomDashboardHandler.
func NewCustomDashboardHandler(svc service.CustomDashboardService) *CustomDashboardHandler {
	return &CustomDashboardHandler{svc: svc}
}

func writeCustomDashboardJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// List handles GET /users/me/dashboards.
func (h *CustomDashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Get handles GET /users/me/dashboards/{id}.
func (h *CustomDashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Save handles PUT /users/me/dashboards/{id}: creates the dashboard, or
// replaces it when the caller already has one with that id. 200 either way.
func (h *CustomDashboardHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req domain.SaveCustomDashboardRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Save(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /users/me/dashboards/{id}.
func (h *CustomDashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
