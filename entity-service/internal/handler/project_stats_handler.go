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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProjectStatsHandler handles HTTP requests for project-scoped statistics,
// backed by the ServiceNow data source only. GET /projects/{id}/metadata
// (also a ProjectStatsService method) is handled separately by
// ProjectMetadataHandler, which does have a Postgres-backed implementation.
type ProjectStatsHandler struct {
	svc service.ProjectStatsService
}

// NewProjectStatsHandler constructs a ProjectStatsHandler with the given service.
func NewProjectStatsHandler(svc service.ProjectStatsService) *ProjectStatsHandler {
	return &ProjectStatsHandler{svc: svc}
}

// GetProjectStats handles GET /projects/{id}/stats.
func (h *ProjectStatsHandler) GetProjectStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectStats(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectCaseStats handles GET /projects/{id}/cases/stats.
func (h *ProjectStatsHandler) GetProjectCaseStats(w http.ResponseWriter, r *http.Request) {
	req := domain.ProjectCaseStatsRequest{
		CaseTypes: r.URL.Query()["caseTypes"],
		CreatedBy: r.URL.Query().Get("createdBy"),
	}
	resp, err := h.svc.GetProjectCaseStats(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectConversationStats handles GET /projects/{id}/conversations/stats.
func (h *ProjectStatsHandler) GetProjectConversationStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectConversationStats(r.Context(), r.PathValue("id"), r.URL.Query().Get("createdBy"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectDeploymentStats handles GET /projects/{id}/deployments/stats.
func (h *ProjectStatsHandler) GetProjectDeploymentStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectDeploymentStats(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectTimeCardStats handles GET /projects/{id}/time-cards/stats.
func (h *ProjectStatsHandler) GetProjectTimeCardStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectTimeCardStats(
		r.Context(), r.PathValue("id"), r.URL.Query().Get("startDate"), r.URL.Query().Get("endDate"),
	)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectChangeRequestStats handles GET /projects/{id}/change-requests/stats.
func (h *ProjectStatsHandler) GetProjectChangeRequestStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectChangeRequestStats(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
