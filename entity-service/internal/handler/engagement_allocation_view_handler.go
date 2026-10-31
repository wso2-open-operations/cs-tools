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
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// EngagementAllocationViewHandler serves the allocation list, detail and updates.
type EngagementAllocationViewHandler struct {
	svc service.EngagementAllocationViewService
}

// NewEngagementAllocationViewHandler constructs the handler with the given service.
func NewEngagementAllocationViewHandler(svc service.EngagementAllocationViewService) *EngagementAllocationViewHandler {
	return &EngagementAllocationViewHandler{svc: svc}
}

// SearchAllocations handles POST /customer-engagements/allocations/search.
func (h *EngagementAllocationViewHandler) SearchAllocations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchEngagementAllocationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetAllocation handles GET /customer-engagements/allocations/{id}.
func (h *EngagementAllocationViewHandler) GetAllocation(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// ListAllocationUpdates handles GET /customer-engagements/allocations/{id}/updates[?limit=&offset=].
func (h *EngagementAllocationViewHandler) ListAllocationUpdates(w http.ResponseWriter, r *http.Request) {
	var p domain.Pagination
	for name, dst := range map[string]*int{"limit": &p.Limit, "offset": &p.Offset} {
		if raw := r.URL.Query().Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				writeServiceError(w, r, &apierror.ValidationError{Msg: name + " must be an integer"})
				return
			}
			*dst = n
		}
	}
	resp, err := h.svc.ListUpdates(r.Context(), r.PathValue("id"), p)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// CreateAllocationUpdate handles POST /customer-engagements/allocations/{id}/updates.
func (h *EngagementAllocationViewHandler) CreateAllocationUpdate(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateEngagementAllocationUpdateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	created, err := h.svc.CreateUpdate(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusCreated, created)
}
