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

// KBArticleHandler handles HTTP requests for the kb_article resource.
type KBArticleHandler struct {
	svc service.KBArticleService
}

// NewKBArticleHandler constructs a KBArticleHandler with the given service.
func NewKBArticleHandler(svc service.KBArticleService) *KBArticleHandler {
	return &KBArticleHandler{svc: svc}
}

// CreateKBArticle handles POST /kb-articles.
func (h *KBArticleHandler) CreateKBArticle(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateKBArticleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateKBArticle(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetKBArticle handles GET /kb-articles/{id}.
func (h *KBArticleHandler) GetKBArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	article, err := h.svc.GetKBArticle(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(article)
}

// SearchKBArticles handles POST /kb-articles/search.
func (h *KBArticleHandler) SearchKBArticles(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchKBArticlesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchKBArticles(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// PatchKBArticleState handles PATCH /kb-articles/{id}/state.
func (h *KBArticleHandler) PatchKBArticleState(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateKBArticleStateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	resp, err := h.svc.UpdateKBArticleState(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
// PatchKBArticleContent handles PATCH /kb-articles/{id}.
func (h *KBArticleHandler) PatchKBArticleContent(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateKBArticleContentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	article, err := h.svc.UpdateKBArticleContent(r.Context(), id, req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(article)
}

// DeleteKBArticle handles DELETE /kb-articles/{id}.
func (h *KBArticleHandler) DeleteKBArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.DeleteKBArticle(r.Context(), id); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListKBArticleHistory handles GET /kb-articles/{id}/history.
func (h *KBArticleHandler) ListKBArticleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.svc.ListKBArticleHistory(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
