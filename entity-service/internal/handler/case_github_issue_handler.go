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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CaseGithubIssueHandler handles HTTP requests for filing a GitHub issue from a case.
type CaseGithubIssueHandler struct {
	svc service.CaseGithubIssueService
}

// NewCaseGithubIssueHandler constructs a CaseGithubIssueHandler with the given service.
func NewCaseGithubIssueHandler(svc service.CaseGithubIssueService) *CaseGithubIssueHandler {
	return &CaseGithubIssueHandler{svc: svc}
}

// CreateCaseGithubIssue handles POST /cases/{id}/github-issues.
func (h *CaseGithubIssueHandler) CreateCaseGithubIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		apierror.WriteJSON(w, http.StatusBadRequest, "case ID is required")
		return
	}

	var req domain.CreateCaseGithubIssueRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = id

	resp, err := h.svc.CreateCaseGithubIssue(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
