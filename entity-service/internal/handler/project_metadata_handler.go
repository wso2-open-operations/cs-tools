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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProjectMetadataHandler handles GET /projects/{id}/metadata -- the one
// ProjectStatsService method with a Postgres-backed implementation as well as
// the ServiceNow one, so it is wired and registered independently of
// ProjectStatsHandler (whose remaining stats routes stay ServiceNow-only).
type ProjectMetadataHandler struct {
	svc service.ProjectMetadataService
}

// NewProjectMetadataHandler constructs a ProjectMetadataHandler with the given service.
func NewProjectMetadataHandler(svc service.ProjectMetadataService) *ProjectMetadataHandler {
	return &ProjectMetadataHandler{svc: svc}
}

// GetProjectMetadata handles GET /projects/{id}/metadata.
func (h *ProjectMetadataHandler) GetProjectMetadata(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectMetadata(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
