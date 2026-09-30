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
	"errors"
	"io"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// KBAdminHandler handles Knowledge-Admin operations: creating, renaming, and
// deactivating knowledge bases, and assigning/removing their approvers.
//
// NOTE (per the Sep 10 call): these endpoints are not yet role-gated to an
// actual "Knowledge Admin" role. Sajith needs to confirm whether the
// roles table has been migrated from ServiceNow before that check can be
// added -- any authenticated user can currently call these. This is a known,
// deliberate, and temporary gap, not an oversight: do not treat this as
// production-ready access control until that role check is added.
type KBAdminHandler struct {
	entity entityKBArticleClient
}

// NewKBAdminHandler constructs a KBAdminHandler with the given entity client.
func NewKBAdminHandler(entity entityKBArticleClient) *KBAdminHandler {
	return &KBAdminHandler{entity: entity}
}

func readAndValidateJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// CreateKnowledgeBase handles POST /knowledge-bases.
func (h *KBAdminHandler) CreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.CreateKnowledgeBase(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to create knowledge base.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// UpdateKnowledgeBaseName handles PATCH /knowledge-bases/{id}.
func (h *KBAdminHandler) UpdateKnowledgeBaseName(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.UpdateKnowledgeBaseName(r.Context(), id, body)
	if err != nil {
		mapUpstreamError(w, err, "Failed to update knowledge base.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SetKnowledgeBaseActive handles PATCH /knowledge-bases/{id}/active.
func (h *KBAdminHandler) SetKnowledgeBaseActive(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SetKnowledgeBaseActive(r.Context(), id, body)
	if err != nil {
		mapUpstreamError(w, err, "Failed to update knowledge base.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchKBManagerUsers handles POST /kb-manager-users/search -- lists
// current individual approvers, typically filtered by knowledgeBaseId for
// the Admin screen.
func (h *KBAdminHandler) SearchKBManagerUsers(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchKBManagerUsers(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to load approvers.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CreateKBManagerUser handles POST /kb-manager-users -- adds a user to a
// knowledge base's approver pool.
func (h *KBAdminHandler) CreateKBManagerUser(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.CreateKBManagerUser(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to add approver.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// DeleteKBManagerUser handles DELETE /kb-manager-users -- removes a user
// from a knowledge base's approver pool.
func (h *KBAdminHandler) DeleteKBManagerUser(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	if err := h.entity.DeleteKBManagerUser(r.Context(), body); err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to remove approver.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SearchKBManagerGroups handles POST /kb-manager-groups/search -- lists
// current group approvers, typically filtered by knowledgeBaseId for the
// Admin screen.
func (h *KBAdminHandler) SearchKBManagerGroups(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchKBManagerGroups(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to load group approvers.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CreateKBManagerGroup handles POST /kb-manager-groups -- adds a group to a
// knowledge base's approver pool.
func (h *KBAdminHandler) CreateKBManagerGroup(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.CreateKBManagerGroup(r.Context(), body)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to add group approver.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// DeleteKBManagerGroup handles DELETE /kb-manager-groups -- removes a
// group from a knowledge base's approver pool.
func (h *KBAdminHandler) DeleteKBManagerGroup(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readAndValidateJSONBody(w, r)
	if !ok {
		return
	}
	if err := h.entity.DeleteKBManagerGroup(r.Context(), body); err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to remove group approver.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
