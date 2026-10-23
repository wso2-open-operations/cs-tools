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

// QueryHourHandler serves query-hour utilisation.
type QueryHourHandler struct {
	svc service.QueryHourService
}

// NewQueryHourHandler constructs a QueryHourHandler.
func NewQueryHourHandler(svc service.QueryHourService) *QueryHourHandler {
	return &QueryHourHandler{svc: svc}
}

// GetWeeklyReport handles GET /reports/query-hours: the accounts that
// have exceeded their query-support allowance or are close to it,
// together with the audience their owners imply.
//
// Read-only. The recompute that refreshes the stored figures is a
// separate, explicitly-invoked call.
func (h *QueryHourHandler) GetWeeklyReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.WeeklyReport(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// RecomputeQueryHours handles POST /query-hours/recompute: rebuild every
// opportunity's utilisation, write the figures back, and return the
// projects that crossed a threshold.
//
// The transitions are returned rather than acted on. Whether a
// threshold crossing becomes an email is the caller's decision, which
// keeps a recompute safe to run at any time -- including twice.
func (h *QueryHourHandler) RecomputeQueryHours(w http.ResponseWriter, r *http.Request) {
	transitions, err := h.svc.Recompute(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if transitions == nil {
		// An explicit empty list, never null: a caller iterating the
		// response should not have to special-case a quiet run.
		transitions = []domain.QueryHourTransition{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"transitions": transitions})
}
