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

// ITServiceHandler handles HTTP requests for the CMDB IT services resource.
type ITServiceHandler struct {
	svc service.ITServiceService
}

// NewITServiceHandler constructs an ITServiceHandler with the given service.
func NewITServiceHandler(svc service.ITServiceService) *ITServiceHandler {
	return &ITServiceHandler{svc: svc}
}

// SearchITServices handles POST /services/search.
func (h *ITServiceHandler) SearchITServices(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchITServicesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchITServices(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
