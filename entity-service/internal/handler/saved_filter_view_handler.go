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

// SavedFilterViewHandler handles HTTP requests for the caller's saved list
// filter views.
type SavedFilterViewHandler struct {
	svc service.SavedFilterViewService
}

// NewSavedFilterViewHandler constructs a SavedFilterViewHandler.
func NewSavedFilterViewHandler(svc service.SavedFilterViewService) *SavedFilterViewHandler {
	return &SavedFilterViewHandler{svc: svc}
}

// List handles GET /users/me/saved-filter-views?listKey=.
func (h *SavedFilterViewHandler) List(w http.ResponseWriter, r *http.Request) {
	listKey := domain.SavedFilterListKey(r.URL.Query().Get("listKey"))
	resp, err := h.svc.List(r.Context(), listKey)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Save handles PATCH /users/me/saved-filter-views.
func (h *SavedFilterViewHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req domain.SaveSavedFilterViewRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Save(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Delete handles DELETE /users/me/saved-filter-views?listKey=&name=.
func (h *SavedFilterViewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	resp, err := h.svc.Delete(r.Context(), domain.SavedFilterListKey(q.Get("listKey")), q.Get("name"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Reorder handles POST /users/me/saved-filter-views/reorder.
func (h *SavedFilterViewHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req domain.ReorderSavedFilterViewRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Reorder(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
