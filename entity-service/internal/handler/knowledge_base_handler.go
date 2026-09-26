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

// KnowledgeBaseHandler handles HTTP requests for the knowledge_base resource.
type KnowledgeBaseHandler struct {
	svc service.KnowledgeBaseService
}

// NewKnowledgeBaseHandler constructs a KnowledgeBaseHandler with the given service.
func NewKnowledgeBaseHandler(svc service.KnowledgeBaseService) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{svc: svc}
}

// ListKnowledgeBases handles GET /knowledge-bases.
func (h *KnowledgeBaseHandler) ListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListKnowledgeBases(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateKnowledgeBase handles POST /knowledge-bases.
func (h *KnowledgeBaseHandler) CreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateKnowledgeBaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	kb, err := h.svc.CreateKnowledgeBase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(kb)
}

// UpdateKnowledgeBaseName handles PATCH /knowledge-bases/{id}.
func (h *KnowledgeBaseHandler) UpdateKnowledgeBaseName(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateKnowledgeBaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	kb, err := h.svc.UpdateKnowledgeBaseName(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(kb)
}

// SetKnowledgeBaseActive handles PATCH /knowledge-bases/{id}/active.
func (h *KnowledgeBaseHandler) SetKnowledgeBaseActive(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateKnowledgeBaseActiveRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	kb, err := h.svc.SetKnowledgeBaseActive(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(kb)
}
