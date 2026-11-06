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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CustomDashboardShareHandler handles HTTP requests for sharing the caller's
// saved dashboards (owner side) and for the dashboards shared with the caller
// (recipient side, view only).
type CustomDashboardShareHandler struct {
	svc service.CustomDashboardShareService
}

// NewCustomDashboardShareHandler constructs a CustomDashboardShareHandler.
func NewCustomDashboardShareHandler(svc service.CustomDashboardShareService) *CustomDashboardShareHandler {
	return &CustomDashboardShareHandler{svc: svc}
}

// ListShares handles GET /users/me/dashboards/{id}/shares.
func (h *CustomDashboardShareHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListShares(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// AddShare handles PUT /users/me/dashboards/{id}/shares/{userId}: no body;
// answers the dashboard's shares after the change. Sharing again is a 200 too.
func (h *CustomDashboardShareHandler) AddShare(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.AddShare(r.Context(), r.PathValue("id"), r.PathValue("userId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// RemoveShare handles DELETE /users/me/dashboards/{id}/shares/{userId}.
func (h *CustomDashboardShareHandler) RemoveShare(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveShare(r.Context(), r.PathValue("id"), r.PathValue("userId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSharedWithMe handles GET /users/me/shared-dashboards.
func (h *CustomDashboardShareHandler) ListSharedWithMe(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListSharedWithMe(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// GetSharedWithMe handles GET /users/me/shared-dashboards/{id}.
func (h *CustomDashboardShareHandler) GetSharedWithMe(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetSharedWithMe(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}
