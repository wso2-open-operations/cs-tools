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

// SearchScheduleEngagementAllocations handles
// POST /team-schedule/engagement-allocations/search.
func (h *ScheduleHandler) SearchScheduleEngagementAllocations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchScheduleEngagementAllocationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchEngagementAllocations(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// DismissScheduleEngagementAllocation handles
// POST /team-schedule/engagement-allocations/dismiss: take a synced
// allocation off a person's roster over some days, or put them back.
func (h *ScheduleHandler) DismissScheduleEngagementAllocation(w http.ResponseWriter, r *http.Request) {
	var req domain.DismissScheduleEngagementAllocationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.DismissEngagementAllocation(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetScheduleOnDuty handles GET /team-schedule/on-duty[?at=RFC3339][&includeOnLeave=true]
// -- who is responsible right now, or at the instant asked for; with
// includeOnLeave, people away that day too, marked onLeave.
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
	includeOnLeave := false
	if raw := r.URL.Query().Get("includeOnLeave"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "includeOnLeave must be true or false"})
			return
		}
		includeOnLeave = v
	}
	resp, err := h.svc.OnDuty(r.Context(), at, includeOnLeave)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// CreateScheduleAssignment handles POST /team-schedule/assignments.
func (h *ScheduleHandler) CreateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateScheduleAssignmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	created, err := h.svc.CreateAssignment(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusCreated, created)
}

// UpdateScheduleAssignment handles PATCH /team-schedule/assignments/{id}.
func (h *ScheduleHandler) UpdateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateScheduleAssignmentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	updated, err := h.svc.UpdateAssignment(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, updated)
}

// DeleteScheduleAssignment handles DELETE /team-schedule/assignments/{id}.
//
// A note is accepted but not required: taking somebody off a slot is worth
// explaining, and the activity row has somewhere to put it.
func (h *ScheduleHandler) DeleteScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	var note *string
	if v := r.URL.Query().Get("note"); v != "" {
		note = &v
	}
	if err := h.svc.DeleteAssignment(r.Context(), r.PathValue("id"), note); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteScheduleAbsence handles DELETE /team-schedule/absences/{id}.
func (h *ScheduleHandler) DeleteScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	var note *string
	if v := r.URL.Query().Get("note"); v != "" {
		note = &v
	}
	if err := h.svc.DeleteAbsence(r.Context(), r.PathValue("id"), note); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateScheduleAbsenceKind handles POST /team-schedule/absence-kinds.
func (h *ScheduleHandler) CreateScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateScheduleAbsenceKindRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	kind, err := h.svc.CreateAbsenceKind(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusCreated, kind)
}

// DeleteScheduleAbsenceKind handles DELETE /team-schedule/absence-kinds/{code}.
func (h *ScheduleHandler) DeleteScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAbsenceKind(r.Context(), r.PathValue("code")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetScheduleActivity handles GET /team-schedule/activity.
func (h *ScheduleHandler) GetScheduleActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := h.svc.TeamActivity(r.Context(), q.Get("teamKey"), q.Get("from"), q.Get("to"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, map[string]any{"activity": rows, "count": len(rows)})
}

// GetMyLeadTeams handles GET /team-schedule/my-lead-teams.
func (h *ScheduleHandler) GetMyLeadTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.svc.MyLeadTeams(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, teams)
}

// ApplyScheduleRange handles POST /team-schedule/assignments/apply.
func (h *ScheduleHandler) ApplyScheduleRange(w http.ResponseWriter, r *http.Request) {
	var req domain.ApplyScheduleRangeRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.ApplyRange(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, res)
}

// ApplyScheduleAbsence handles POST /team-schedule/absences/apply.
func (h *ScheduleHandler) ApplyScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	var req domain.ApplyScheduleAbsenceRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.ApplyAbsence(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, res)
}

// ImportRota handles POST /team-schedule/rotas/{code}/import?dryRun=true|false.
func (h *ScheduleHandler) ImportRota(w http.ResponseWriter, r *http.Request) {
	dryRun := false
	if raw := r.URL.Query().Get("dryRun"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			apierror.WriteJSON(w, http.StatusBadRequest, "dryRun must be true or false")
			return
		}
		dryRun = v
	}
	var req domain.ImportRotaRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.ImportRota(r.Context(), r.PathValue("code"), req, dryRun)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	status := http.StatusCreated
	if dryRun {
		status = http.StatusOK
	}
	writeScheduleJSON(w, status, res)
}

// GetScheduleEditMarkers handles GET /team-schedule/edit-markers.
func (h *ScheduleHandler) GetScheduleEditMarkers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.svc.EditMarkers(r.Context(), q.Get("from"), q.Get("to"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, res)
}
