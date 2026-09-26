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

// KBManagerUserHandler handles HTTP requests for the
// knowledge_base_manager_user resource. createdBy is currently hardcoded
// to "system" for both handlers below -- NOT YET wired to the
// authenticated caller's real identity (see the service layer's own doc
// comment; same open question as KB article authorship).
type KBManagerUserHandler struct {
	svc service.KBManagerUserService
}

// NewKBManagerUserHandler constructs a KBManagerUserHandler with the given service.
func NewKBManagerUserHandler(svc service.KBManagerUserService) *KBManagerUserHandler {
	return &KBManagerUserHandler{svc: svc}
}

// SearchKBManagerUsers handles POST /kb-manager-users/search.
func (h *KBManagerUserHandler) SearchKBManagerUsers(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchKBManagerUsersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchKBManagerUsers(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateKBManagerUser handles POST /kb-manager-users.
func (h *KBManagerUserHandler) CreateKBManagerUser(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateKBManagerUserRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	m, err := h.svc.CreateKBManagerUser(r.Context(), req, "system")
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(m)
}

// deleteKBManagerUserRequest is the body for DELETE /kb-manager-users -- a
// knowledge base + user pair, since this table has no single-column
// primary key convenient to address by path alone from the caller's
// perspective.
type deleteKBManagerUserRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	UserID          string `json:"userId"`
}

// DeleteKBManagerUser handles DELETE /kb-manager-users.
func (h *KBManagerUserHandler) DeleteKBManagerUser(w http.ResponseWriter, r *http.Request) {
	var req deleteKBManagerUserRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.DeleteKBManagerUser(r.Context(), req.KnowledgeBaseID, req.UserID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// KBManagerGroupHandler handles HTTP requests for the
// knowledge_base_manager_group resource.
type KBManagerGroupHandler struct {
	svc service.KBManagerGroupService
}

// NewKBManagerGroupHandler constructs a KBManagerGroupHandler with the given service.
func NewKBManagerGroupHandler(svc service.KBManagerGroupService) *KBManagerGroupHandler {
	return &KBManagerGroupHandler{svc: svc}
}

// SearchKBManagerGroups handles POST /kb-manager-groups/search.
func (h *KBManagerGroupHandler) SearchKBManagerGroups(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchKBManagerGroupsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchKBManagerGroups(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateKBManagerGroup handles POST /kb-manager-groups.
func (h *KBManagerGroupHandler) CreateKBManagerGroup(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateKBManagerGroupRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	m, err := h.svc.CreateKBManagerGroup(r.Context(), req, "system")
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(m)
}

// deleteKBManagerGroupRequest is the body for DELETE /kb-manager-groups.
type deleteKBManagerGroupRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	GroupID         string `json:"groupId"`
}

// DeleteKBManagerGroup handles DELETE /kb-manager-groups.
func (h *KBManagerGroupHandler) DeleteKBManagerGroup(w http.ResponseWriter, r *http.Request) {
	var req deleteKBManagerGroupRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.DeleteKBManagerGroup(r.Context(), req.KnowledgeBaseID, req.GroupID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
