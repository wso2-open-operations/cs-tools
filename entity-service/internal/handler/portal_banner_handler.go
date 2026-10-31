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

// PortalBannerHandler serves the HTML banners both portals show.
type PortalBannerHandler struct {
	svc service.PortalBannerService
}

// NewPortalBannerHandler constructs a PortalBannerHandler.
func NewPortalBannerHandler(svc service.PortalBannerService) *PortalBannerHandler {
	return &PortalBannerHandler{svc: svc}
}

func writeBannerJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ListCustomerBanners handles GET /banners. The customer audience is fixed
// by this route: published rows with the customer portal flag, whoever calls.
func (h *PortalBannerHandler) ListCustomerBanners(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListCustomer(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeBannerJSON(w, http.StatusOK, resp)
}

// ListCSMBanners handles GET /portal-banners.
func (h *PortalBannerHandler) ListCSMBanners(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListCSM(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeBannerJSON(w, http.StatusOK, resp)
}

// ListAdminBanners handles GET /portal-banners/manage.
func (h *PortalBannerHandler) ListAdminBanners(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListAdmin(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeBannerJSON(w, http.StatusOK, resp)
}

// CreateBanner handles POST /portal-banners.
func (h *PortalBannerHandler) CreateBanner(w http.ResponseWriter, r *http.Request) {
	var req domain.SavePortalBannerRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	banner, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeBannerJSON(w, http.StatusCreated, banner)
}

// UpdateBanner handles PUT /portal-banners/{id}.
func (h *PortalBannerHandler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	var req domain.SavePortalBannerRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	banner, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeBannerJSON(w, http.StatusOK, banner)
}
