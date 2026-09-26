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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// defaultSweepStaleFor is how old a stored position must be before the sweep
// will recompute it. One hour: short enough that a project's consumption is
// never badly out of date, long enough that the hourly sub-cron does not
// re-derive projects nothing has touched.
const defaultSweepStaleFor = time.Hour

// QueryHourHandler handles the query-hour endpoints — the port of
// ServiceNow's `[Query Hour] UpdateTime Card` flow. See
// service.QueryHourService for what it replaces and why.
type QueryHourHandler struct {
	svc service.QueryHourService
}

// NewQueryHourHandler constructs a QueryHourHandler with the given service.
func NewQueryHourHandler(svc service.QueryHourService) *QueryHourHandler {
	return &QueryHourHandler{svc: svc}
}

// GetProjectQueryHours handles GET /projects/{id}/query-hours — the stored
// position, without recomputing. 404 when the project has never been computed.
func (h *QueryHourHandler) GetProjectQueryHours(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// RecomputeProjectQueryHours handles
// POST /projects/{id}/query-hours/recompute — re-derives one project's
// position and pushes it to Choreo if the state moved.
//
// A POST, not a GET, because it writes and has an outbound side effect. It is
// idempotent in the way that matters: running it twice with unchanged time
// cards recomputes the same numbers and pushes nothing the second time.
func (h *QueryHourHandler) RecomputeProjectQueryHours(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Recompute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// RecomputeForTimeCard handles
// POST /time-cards/{id}/query-hours/recompute — the direct replacement for
// the ServiceNow flow's trigger. Recomputes only the time card's own project,
// not every project under its account.
func (h *QueryHourHandler) RecomputeForTimeCard(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.RecomputeForTimeCard(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SweepQueryHours handles POST /query-hours/sweep?staleForMinutes=&limit= —
// recomputes the stalest projects. Backs the sub-cron in
// operations/csm-scheduled-tasks.
//
// Returns 200 with per-project errors in the body rather than failing the
// whole call when individual projects fail: a sweep that abandons everything
// because one project is broken would stall on that project forever, since it
// stays the stalest.
func (h *QueryHourHandler) SweepQueryHours(w http.ResponseWriter, r *http.Request) {
	staleFor := defaultSweepStaleFor
	if raw := r.URL.Query().Get("staleForMinutes"); raw != "" {
		mins, err := strconv.Atoi(raw)
		if err != nil || mins < 0 {
			apierror.WriteJSON(w, http.StatusBadRequest, "staleForMinutes must be a non-negative integer")
			return
		}
		staleFor = time.Duration(mins) * time.Minute
	}

	limit := 0 // 0 lets the service apply its own default
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			apierror.WriteJSON(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	resp, err := h.svc.Sweep(r.Context(), staleFor, limit)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetWeeklyReport handles GET /query-hours/weekly-report.
//
// Backs the query_hours_weekly_report sub-cron in
// operations/csm-scheduled-tasks, which renders and sends the email. The
// division is the same one every report task here uses: this service owns
// what is true about the estate, the scheduled task owns who hears about it
// and what the mail looks like.
//
// A GET with no parameters: the report has no filters, no pagination and no
// caller-supplied input at all — it is one fixed question about the whole
// estate.
func (h *QueryHourHandler) GetWeeklyReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.WeeklyReport(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
