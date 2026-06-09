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

// CaseHandler handles HTTP requests for the case resource.
type CaseHandler struct {
	svc service.CaseService
}

// NewCaseHandler constructs a CaseHandler with the given service.
func NewCaseHandler(svc service.CaseService) *CaseHandler {
	return &CaseHandler{svc: svc}
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.svc.GetCaseByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	c, err := h.svc.CreateCase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

// PatchCase handles PATCH /cases/{id} and allows updating the case state.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateCaseStateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	c, err := h.svc.UpdateCaseState(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

// CreateCaseComment handles POST /cases/{id}/comments.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCaseCommentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	comment, err := h.svc.CreateCaseComment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(comment)
}

// SearchCaseComments handles POST /cases/{id}/comments/search.
func (h *CaseHandler) SearchCaseComments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCaseCommentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.SearchCaseComments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCases handles POST /cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCases(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
