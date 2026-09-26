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

// ProjectCaseStatsHandler handles GET /projects/{id}/cases/stats -- one of
// the two ProjectStatsService methods with a Postgres-backed implementation
// as well as the ServiceNow one, so it is wired and registered independently
// of ProjectStatsHandler (whose remaining stats routes stay ServiceNow-only),
// exactly as ProjectMetadataHandler is.
type ProjectCaseStatsHandler struct {
	svc service.ProjectCaseStatsService
}

// NewProjectCaseStatsHandler constructs a ProjectCaseStatsHandler with the given service.
func NewProjectCaseStatsHandler(svc service.ProjectCaseStatsService) *ProjectCaseStatsHandler {
	return &ProjectCaseStatsHandler{svc: svc}
}

// GetProjectCaseStats handles GET /projects/{id}/cases/stats.
//
// caseTypes is read with Query()["caseTypes"], so it is a repeated parameter
// (?caseTypes=a&caseTypes=b), matching the OpenAPI schema's
// style: form, explode: true -- not a comma-joined list.
func (h *ProjectCaseStatsHandler) GetProjectCaseStats(w http.ResponseWriter, r *http.Request) {
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
