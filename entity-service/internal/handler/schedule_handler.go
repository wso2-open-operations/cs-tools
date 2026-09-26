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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ScheduleHandler serves the Team Schedule reads.
type ScheduleHandler struct {
	svc service.ScheduleService
}

// NewScheduleHandler constructs a ScheduleHandler with the given service.
func NewScheduleHandler(svc service.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{svc: svc}
}

func writeScheduleJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GetScheduleCatalogue handles GET /team-schedule/catalogue -- the zones, windows
// and absence kinds a client needs before it can draw anything.
func (h *ScheduleHandler) GetScheduleCatalogue(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.Catalogue(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, cat)
}

// SearchScheduleAssignments handles POST /team-schedule/assignments/search.
func (h *ScheduleHandler) SearchScheduleAssignments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchScheduleAssignmentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchAssignments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// SearchScheduleAbsences handles POST /team-schedule/absences/search.
func (h *ScheduleHandler) SearchScheduleAbsences(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchScheduleAbsencesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchAbsences(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetScheduleOnDuty handles GET /team-schedule/on-duty[?at=RFC3339] -- who is
// responsible right now, or at the instant asked for.
func (h *ScheduleHandler) GetScheduleOnDuty(w http.ResponseWriter, r *http.Request) {
	var at *time.Time
	if raw := r.URL.Query().Get("at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "at must be an RFC3339 timestamp"})
			return
		}
		at = &parsed
	}
	resp, err := h.svc.OnDuty(r.Context(), at)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}
