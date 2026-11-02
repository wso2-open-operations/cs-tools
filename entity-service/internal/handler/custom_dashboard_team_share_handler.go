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

// CustomDashboardTeamShareHandler handles HTTP requests for sharing the
// caller's saved dashboards with teams (owner side). A team's members read the
// dashboard through CustomDashboardShareHandler's shared-dashboards routes.
type CustomDashboardTeamShareHandler struct {
	svc service.CustomDashboardTeamShareService
}

// NewCustomDashboardTeamShareHandler constructs a CustomDashboardTeamShareHandler.
func NewCustomDashboardTeamShareHandler(svc service.CustomDashboardTeamShareService) *CustomDashboardTeamShareHandler {
	return &CustomDashboardTeamShareHandler{svc: svc}
}

// ListTeamShares handles GET /users/me/dashboards/{id}/team-shares.
func (h *CustomDashboardTeamShareHandler) ListTeamShares(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListTeamShares(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// AddTeamShare handles PUT /users/me/dashboards/{id}/team-shares/{teamId}: no
// body; answers the dashboard's team shares after the change. Sharing again is
// a 200 too.
func (h *CustomDashboardTeamShareHandler) AddTeamShare(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.AddTeamShare(r.Context(), r.PathValue("id"), r.PathValue("teamId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// RemoveTeamShare handles DELETE /users/me/dashboards/{id}/team-shares/{teamId}.
func (h *CustomDashboardTeamShareHandler) RemoveTeamShare(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveTeamShare(r.Context(), r.PathValue("id"), r.PathValue("teamId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
