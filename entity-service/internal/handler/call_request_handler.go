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

// CallRequestHandler handles HTTP requests for the call-request resource.
type CallRequestHandler struct {
	svc service.CallRequestService
}

// NewCallRequestHandler constructs a CallRequestHandler with the given service.
func NewCallRequestHandler(svc service.CallRequestService) *CallRequestHandler {
	return &CallRequestHandler{svc: svc}
}

// CreateCallRequest handles POST /call-requests.
func (h *CallRequestHandler) CreateCallRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCallRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateCallRequest(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCallRequests handles POST /call-requests/search.
func (h *CallRequestHandler) SearchCallRequests(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCallRequestsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCallRequests(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// PatchCallRequest handles PATCH /call-requests/{id}.
func (h *CallRequestHandler) PatchCallRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		apierror.WriteJSON(w, http.StatusBadRequest, "call request ID is required")
		return
	}

	var req domain.UpdateCallRequestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = id

	resp, err := h.svc.UpdateCallRequest(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
