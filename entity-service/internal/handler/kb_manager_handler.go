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

// KBManagerHandler handles HTTP requests for the kb_manager resource.
type KBManagerHandler struct {
	svc service.KBManagerService
}

// NewKBManagerHandler constructs a KBManagerHandler with the given service.
func NewKBManagerHandler(svc service.KBManagerService) *KBManagerHandler {
	return &KBManagerHandler{svc: svc}
}

// SearchKBManagers handles POST /kb-managers/search.
func (h *KBManagerHandler) SearchKBManagers(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchKBManagersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchKBManagers(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateKBManager handles POST /kb-managers.
func (h *KBManagerHandler) CreateKBManager(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateKBManagerRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	m, err := h.svc.CreateKBManager(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(m)
}

// deleteKBManagerRequest is the body for DELETE /kb-managers -- a knowledge
// base + user pair, since kb_managers has no single-column primary key
// convenient to address by path alone from the caller's perspective.
type deleteKBManagerRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	UserID          string `json:"userId"`
}

// DeleteKBManager handles DELETE /kb-managers.
func (h *KBManagerHandler) DeleteKBManager(w http.ResponseWriter, r *http.Request) {
	var req deleteKBManagerRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.DeleteKBManager(r.Context(), req.KnowledgeBaseID, req.UserID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
