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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxSystemDashboardBody bounds a PUT body: a definition may be up to
// domain.MaxSystemDashboardDefinitionBytes, and JSON string escaping can grow
// it, so the generic 1 MiB body cap does not fit.
const maxSystemDashboardBody = 4 << 20

// SystemDashboardHandler handles HTTP requests for the system dashboards the
// CSM portal backend stores here. Service to service only: the service refuses
// any other caller.
type SystemDashboardHandler struct {
	svc service.SystemDashboardService
}

// NewSystemDashboardHandler constructs a SystemDashboardHandler.
func NewSystemDashboardHandler(svc service.SystemDashboardService) *SystemDashboardHandler {
	return &SystemDashboardHandler{svc: svc}
}

// List handles GET /system-dashboards.
func (h *SystemDashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Get handles GET /system-dashboards/{key}.
func (h *SystemDashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("key"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Save handles PUT /system-dashboards/{key}: creates the system dashboard, or
// overwrites it when the key exists. 200 either way.
func (h *SystemDashboardHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req domain.SaveSystemDashboardRequest
	if !decodeRequestWithLimit(w, r, &req, maxSystemDashboardBody, "request body too large") {
		return
	}
	resp, err := h.svc.Save(r.Context(), r.PathValue("key"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /system-dashboards/{key}: 204, or 404 when no system
// dashboard has that key.
func (h *SystemDashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("key")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
