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

// AutoclosureHandler serves the two routes operations/csm-scheduled-tasks uses
// to run the case auto-closure workflow.
type AutoclosureHandler struct {
	svc service.AutoclosureService
}

// NewAutoclosureHandler constructs an AutoclosureHandler.
func NewAutoclosureHandler(svc service.AutoclosureService) *AutoclosureHandler {
	return &AutoclosureHandler{svc: svc}
}

// FindCandidates handles POST /autoclosure/candidates.
func (h *AutoclosureHandler) FindCandidates(w http.ResponseWriter, r *http.Request) {
	var req domain.AutoclosureCandidatesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.FindCandidates(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Advance handles POST /autoclosure/cases/{id}/advance.
func (h *AutoclosureHandler) Advance(w http.ResponseWriter, r *http.Request) {
	var req domain.AutoclosureAdvanceRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Advance(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
