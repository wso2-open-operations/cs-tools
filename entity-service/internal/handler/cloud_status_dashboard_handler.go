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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CloudStatusDashboardHandler serves the public cloud status dashboard's reads.
//
// Consumed by the public cloud status dashboard through csm-integration-service,
// not by the portal. The response shapes are that dashboard's, not this
// service's -- see the domain types for why they look the way they do.
type CloudStatusDashboardHandler struct {
	svc service.CloudStatusDashboardService
}

// NewCloudStatusDashboardHandler constructs the handler.
func NewCloudStatusDashboardHandler(svc service.CloudStatusDashboardService) *CloudStatusDashboardHandler {
	return &CloudStatusDashboardHandler{svc: svc}
}

// Monitors handles GET /cloud-status/monitors?cloud=…
func (h *CloudStatusDashboardHandler) Monitors(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Monitors(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Incidents handles GET /cloud-status/incidents?cloud=…
func (h *CloudStatusDashboardHandler) Incidents(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Incidents(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Availabilities handles GET /cloud-status/availabilities?cloud=…
func (h *CloudStatusDashboardHandler) Availabilities(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Availabilities(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// AvailabilityHistory handles GET /cloud-status/availability-history?cloud=…
//
// The ServiceNow resource is at /history; the name here says what it returns
// rather than inheriting a path that reads as a generic history endpoint.
// The dashboard's own route is /api/v1/history/availabilities, so neither
// side is renaming the other.
func (h *CloudStatusDashboardHandler) AvailabilityHistory(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.AvailabilityHistory(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// IncidentDetail handles GET /cloud-status/incidents/{id}?cloud=…
//
// 404 when no outage with that id belongs to that cloud, matching the
// ServiceNow resource. The 200 body has one of two shapes; see the service.
func (h *CloudStatusDashboardHandler) IncidentDetail(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.IncidentDetail(r.Context(), r.PathValue("id"), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if resp == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "outage not found for this cloud"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
