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

// CodeDashboardShareHandler handles HTTP requests for sharing the caller's
// saved code dashboards (owner side: people, teams, the link) and for the
// code dashboards shared with the caller (recipient side, view only), as the
// custom dashboard share, team share and link handlers do for JSON ones.
type CodeDashboardShareHandler struct {
	svc service.CodeDashboardShareService
}

// NewCodeDashboardShareHandler constructs a CodeDashboardShareHandler.
func NewCodeDashboardShareHandler(svc service.CodeDashboardShareService) *CodeDashboardShareHandler {
	return &CodeDashboardShareHandler{svc: svc}
}

// writeCodeDashboardShare writes resp as 200, or the service error.
func writeCodeDashboardShare(w http.ResponseWriter, r *http.Request, resp any, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// writeCodeDashboardShareGone writes 204, or the service error.
func writeCodeDashboardShareGone(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListShares handles GET /users/me/code-dashboards/{id}/shares.
func (h *CodeDashboardShareHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListShares(r.Context(), r.PathValue("id"))
	writeCodeDashboardShare(w, r, resp, err)
}

// AddShare handles PUT /users/me/code-dashboards/{id}/shares/{userId}: no
// body; answers the dashboard's shares after the change.
func (h *CodeDashboardShareHandler) AddShare(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.AddShare(r.Context(), r.PathValue("id"), r.PathValue("userId"))
	writeCodeDashboardShare(w, r, resp, err)
}

// RemoveShare handles DELETE /users/me/code-dashboards/{id}/shares/{userId}.
func (h *CodeDashboardShareHandler) RemoveShare(w http.ResponseWriter, r *http.Request) {
	writeCodeDashboardShareGone(w, r, h.svc.RemoveShare(r.Context(), r.PathValue("id"), r.PathValue("userId")))
}

// ListTeamShares handles GET /users/me/code-dashboards/{id}/team-shares.
func (h *CodeDashboardShareHandler) ListTeamShares(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListTeamShares(r.Context(), r.PathValue("id"))
	writeCodeDashboardShare(w, r, resp, err)
}

// AddTeamShare handles PUT /users/me/code-dashboards/{id}/team-shares/{teamId}:
// no body; answers the dashboard's team shares after the change.
func (h *CodeDashboardShareHandler) AddTeamShare(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.AddTeamShare(r.Context(), r.PathValue("id"), r.PathValue("teamId"))
	writeCodeDashboardShare(w, r, resp, err)
}

// RemoveTeamShare handles DELETE /users/me/code-dashboards/{id}/team-shares/{teamId}.
func (h *CodeDashboardShareHandler) RemoveTeamShare(w http.ResponseWriter, r *http.Request) {
	writeCodeDashboardShareGone(w, r, h.svc.RemoveTeamShare(r.Context(), r.PathValue("id"), r.PathValue("teamId")))
}

// GetLink handles GET /users/me/code-dashboards/{id}/link.
func (h *CodeDashboardShareHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetLink(r.Context(), r.PathValue("id"))
	writeCodeDashboardShare(w, r, resp, err)
}

// EnableLink handles PUT /users/me/code-dashboards/{id}/link: no body;
// answers the link. On again is a 200 with the same link.
func (h *CodeDashboardShareHandler) EnableLink(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.EnableLink(r.Context(), r.PathValue("id"))
	writeCodeDashboardShare(w, r, resp, err)
}

// DisableLink handles DELETE /users/me/code-dashboards/{id}/link.
func (h *CodeDashboardShareHandler) DisableLink(w http.ResponseWriter, r *http.Request) {
	writeCodeDashboardShareGone(w, r, h.svc.DisableLink(r.Context(), r.PathValue("id")))
}

// OpenLink handles POST /users/me/shared-code-dashboards/link: body {linkId},
// so the link id is never in a path or an access log; answers the dashboard
// it opened.
func (h *CodeDashboardShareHandler) OpenLink(w http.ResponseWriter, r *http.Request) {
	var req domain.OpenCustomDashboardLinkRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.OpenLink(r.Context(), req)
	writeCodeDashboardShare(w, r, resp, err)
}

// ListSharedWithMe handles GET /users/me/shared-code-dashboards.
func (h *CodeDashboardShareHandler) ListSharedWithMe(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListSharedWithMe(r.Context())
	writeCodeDashboardShare(w, r, resp, err)
}

// GetSharedWithMe handles GET /users/me/shared-code-dashboards/{id}.
func (h *CodeDashboardShareHandler) GetSharedWithMe(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetSharedWithMe(r.Context(), r.PathValue("id"))
	writeCodeDashboardShare(w, r, resp, err)
}
