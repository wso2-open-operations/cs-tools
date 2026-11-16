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

// CustomDashboardLinkHandler handles HTTP requests for sharing the caller's
// saved dashboards by link: the owner turns the link on or off, anyone else
// opens it. An opened dashboard is read through CustomDashboardShareHandler's
// shared-dashboards routes.
type CustomDashboardLinkHandler struct {
	svc service.CustomDashboardLinkService
}

// NewCustomDashboardLinkHandler constructs a CustomDashboardLinkHandler.
func NewCustomDashboardLinkHandler(svc service.CustomDashboardLinkService) *CustomDashboardLinkHandler {
	return &CustomDashboardLinkHandler{svc: svc}
}

// GetLink handles GET /users/me/dashboards/{id}/link.
func (h *CustomDashboardLinkHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetLink(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// EnableLink handles PUT /users/me/dashboards/{id}/link: no body; answers the
// link. Turning it on again is a 200 with the same link.
func (h *CustomDashboardLinkHandler) EnableLink(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.EnableLink(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}

// DisableLink handles DELETE /users/me/dashboards/{id}/link.
func (h *CustomDashboardLinkHandler) DisableLink(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DisableLink(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OpenLink handles POST /users/me/shared-dashboards/link: body {linkId};
// answers the dashboard it opened.
func (h *CustomDashboardLinkHandler) OpenLink(w http.ResponseWriter, r *http.Request) {
	var req domain.OpenCustomDashboardLinkRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.OpenLink(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCustomDashboardJSON(w, http.StatusOK, resp)
}
