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

// ChangeRequestHandler handles HTTP requests for the change-request resource.
type ChangeRequestHandler struct {
	svc service.ChangeRequestService
}

// NewChangeRequestHandler constructs a ChangeRequestHandler with the given service.
func NewChangeRequestHandler(svc service.ChangeRequestService) *ChangeRequestHandler {
	return &ChangeRequestHandler{svc: svc}
}

// SearchChangeRequests handles POST /change-requests/search.
func (h *ChangeRequestHandler) SearchChangeRequests(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchChangeRequestsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchChangeRequests(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetChangeRequest handles GET /change-requests/{id}.
func (h *ChangeRequestHandler) GetChangeRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.svc.GetChangeRequest(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

