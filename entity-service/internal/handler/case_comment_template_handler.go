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
	"context"
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CaseCommentTemplateHandler handles the saved case comment template routes:
// the caller's own under /users/me/case-comment-templates and the shared ones
// under /case-comment-templates.
type CaseCommentTemplateHandler struct {
	svc service.CaseCommentTemplateService
}

// NewCaseCommentTemplateHandler constructs a CaseCommentTemplateHandler.
func NewCaseCommentTemplateHandler(svc service.CaseCommentTemplateService) *CaseCommentTemplateHandler {
	return &CaseCommentTemplateHandler{svc: svc}
}

// List handles GET /users/me/case-comment-templates.
func (h *CaseCommentTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCaseCommentTemplateJSON(w, http.StatusOK, resp)
}

// CreatePersonal handles POST /users/me/case-comment-templates.
func (h *CaseCommentTemplateHandler) CreatePersonal(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, h.svc.CreatePersonal)
}

// UpdatePersonal handles PATCH /users/me/case-comment-templates/{id}.
func (h *CaseCommentTemplateHandler) UpdatePersonal(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, h.svc.UpdatePersonal)
}

// DeletePersonal handles DELETE /users/me/case-comment-templates/{id}.
func (h *CaseCommentTemplateHandler) DeletePersonal(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, h.svc.DeletePersonal)
}

// CreateShared handles POST /case-comment-templates.
func (h *CaseCommentTemplateHandler) CreateShared(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, h.svc.CreateShared)
}

// UpdateShared handles PATCH /case-comment-templates/{id}.
func (h *CaseCommentTemplateHandler) UpdateShared(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, h.svc.UpdateShared)
}

// DeleteShared handles DELETE /case-comment-templates/{id}.
func (h *CaseCommentTemplateHandler) DeleteShared(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, h.svc.DeleteShared)
}

type createCaseCommentTemplateFunc func(ctx context.Context, req domain.CreateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error)

func (h *CaseCommentTemplateHandler) create(w http.ResponseWriter, r *http.Request, create createCaseCommentTemplateFunc) {
	var req domain.CreateCaseCommentTemplateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	t, err := create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCaseCommentTemplateJSON(w, http.StatusCreated, t)
}

type updateCaseCommentTemplateFunc func(ctx context.Context, id string, req domain.UpdateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error)

func (h *CaseCommentTemplateHandler) update(w http.ResponseWriter, r *http.Request, update updateCaseCommentTemplateFunc) {
	var req domain.UpdateCaseCommentTemplateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	t, err := update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCaseCommentTemplateJSON(w, http.StatusOK, t)
}

type deleteCaseCommentTemplateFunc func(ctx context.Context, id string) error

func (h *CaseCommentTemplateHandler) delete(w http.ResponseWriter, r *http.Request, del deleteCaseCommentTemplateFunc) {
	if err := del(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeCaseCommentTemplateJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
