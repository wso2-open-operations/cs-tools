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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SNWritebackFailureHandler handles the operator routes over
// sn_writeback_failures (DATA_SOURCE=postgres-servicenow-dual-write only):
// what the previous system is missing, and the replay of one row. Both are
// registered behind internalOnly in routes.go.
type SNWritebackFailureHandler struct {
	svc service.SNWritebackFailureService
}

// NewSNWritebackFailureHandler constructs an SNWritebackFailureHandler.
func NewSNWritebackFailureHandler(svc service.SNWritebackFailureService) *SNWritebackFailureHandler {
	return &SNWritebackFailureHandler{svc: svc}
}

// ListSNWritebackFailures handles GET /sn-writeback-failures
// [?entityType=&entityId=&limit=].
func (h *SNWritebackFailureHandler) ListSNWritebackFailures(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	resp, err := h.svc.ListSNWritebackFailures(r.Context(), q.Get("entityType"), q.Get("entityId"), limit)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ReplaySNWritebackFailure handles POST /sn-writeback-failures/{id}/replay.
func (h *SNWritebackFailureHandler) ReplaySNWritebackFailure(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ReplaySNWritebackFailure(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
