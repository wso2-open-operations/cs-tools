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

// AnnouncementRegistryHandler serves the CSM announcement registry's one-shot
// case read.
type AnnouncementRegistryHandler struct {
	svc service.AnnouncementRegistryService
}

// NewAnnouncementRegistryHandler constructs an AnnouncementRegistryHandler.
func NewAnnouncementRegistryHandler(svc service.AnnouncementRegistryService) *AnnouncementRegistryHandler {
	return &AnnouncementRegistryHandler{svc: svc}
}

// SearchRegistryCases handles POST /announcements/registry/cases. The body is
// the same shape as POST /cases/search; the response is the same too, with
// every matching announcement case in one list.
func (h *AnnouncementRegistryHandler) SearchRegistryCases(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchRegistryCases(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchRegistryRows handles POST /announcements/registry/rows. The body is
// the same shape as POST /cases/search; pagination applies to the grouped
// rows in the response, not to the cases behind them.
func (h *AnnouncementRegistryHandler) SearchRegistryRows(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchRegistryRows(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
