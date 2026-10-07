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
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityScheduleClient abstracts the entity service Team Schedule operations.
type entityScheduleClient interface {
	GetScheduleCatalogue(ctx context.Context) ([]byte, error)
	SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error)
	SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error)
	GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error)

	// Lead edit. The portal proxies these unchanged -- entity-service is where
	// "may this person edit this team's rota" is decided.
	CreateScheduleAssignment(ctx context.Context, body []byte) ([]byte, error)
	UpdateScheduleAssignment(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteScheduleAssignment(ctx context.Context, id, note string) ([]byte, error)
	GetScheduleActivity(ctx context.Context, teamKey, from, to string) ([]byte, error)
	GetMyLeadTeams(ctx context.Context) ([]byte, error)
	ApplyScheduleRange(ctx context.Context, body []byte) ([]byte, error)
	ApplyScheduleAbsence(ctx context.Context, body []byte) ([]byte, error)
	DeleteScheduleAbsence(ctx context.Context, id, note string) ([]byte, error)
	CreateScheduleAbsenceKind(ctx context.Context, body []byte) ([]byte, error)
	DeleteScheduleAbsenceKind(ctx context.Context, code string) ([]byte, error)
	GetScheduleEditMarkers(ctx context.Context, from, to string) ([]byte, error)
}

// ScheduleHandler handles the Team Schedule reads: who is working, when, and
// who is out of the rota.
type ScheduleHandler struct {
	entity entityScheduleClient
}

// NewScheduleHandler creates a ScheduleHandler backed by the given entity client.
func NewScheduleHandler(entity entityScheduleClient) *ScheduleHandler {
	return &ScheduleHandler{entity: entity}
}

// readScheduleBody authenticates the caller and returns the request body,
// having checked it is valid JSON and within the size limit. Returns ok=false
// when it has already written the response.
func readScheduleBody(w http.ResponseWriter, r *http.Request) (body []byte, userID string, ok bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, "", false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, "", false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, "", false
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, "", false
	}
	return body, user.UserID, true
}

// GetScheduleCatalogue handles GET /team-schedule/catalogue.
func (h *ScheduleHandler) GetScheduleCatalogue(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleCatalogue(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleCatalogue failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the schedule catalogue.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAssignments handles POST /team-schedule/assignments/search.
func (h *ScheduleHandler) SearchScheduleAssignments(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAssignments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAssignments failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search the schedule.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAbsences handles POST /team-schedule/absences/search.
func (h *ScheduleHandler) SearchScheduleAbsences(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAbsences(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAbsences failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search schedule absences.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetScheduleOnDuty handles GET /team-schedule/on-duty[?at=RFC3339] -- who is
// responsible at this instant. The `at` parameter is passed through unchanged;
// the entity service validates it.
func (h *ScheduleHandler) GetScheduleOnDuty(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleOnDuty(r.Context(), r.URL.Query().Get("at"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleOnDuty failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who is on duty.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateScheduleAssignment handles POST /team-schedule/assignments.
//
// The portal does not decide who may edit: entity-service checks that the
// caller leads the team the slot belongs to, and the body is passed through
// unchanged. PermWrite here only keeps the control out of the hands of someone
// who could not use it at all.
func (h *ScheduleHandler) CreateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateScheduleAssignment(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateScheduleAssignment failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add the assignment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// UpdateScheduleAssignment handles PATCH /team-schedule/assignments/{id}.
func (h *ScheduleHandler) UpdateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.UpdateScheduleAssignment(r.Context(), r.PathValue("id"), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateScheduleAssignment failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to change the assignment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteScheduleAssignment handles DELETE /team-schedule/assignments/{id}.
func (h *ScheduleHandler) DeleteScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAssignment(r.Context(), r.PathValue("id"), r.URL.Query().Get("note")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAssignment failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to remove the assignment.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetScheduleActivity handles GET /team-schedule/activity.
func (h *ScheduleHandler) GetScheduleActivity(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	result, err := h.entity.GetScheduleActivity(r.Context(), q.Get("teamKey"), q.Get("from"), q.Get("to"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleActivity failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the schedule history.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetMyLeadTeams handles GET /team-schedule/my-lead-teams.
func (h *ScheduleHandler) GetMyLeadTeams(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetMyLeadTeams(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMyLeadTeams failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to check your team permissions.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// mapScheduleWriteError is mapUpstreamErrorGeneric for a lead's edit to the
// rota, except that a refusal keeps entity-service's own reason. Its 403 and
// 409 messages are written for the lead -- "that engineer is not on
// americas...", "this person already has a window that overlaps this one..."
// -- and without them the picker could only say that nothing had saved,
// leaving a lead to guess why. Every other status stays generic, so nothing
// internal reaches the page.
func mapScheduleWriteError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusConflict) {
		writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
		return
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}

// ApplyScheduleRange handles POST /team-schedule/assignments/apply.
func (h *ScheduleHandler) ApplyScheduleRange(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.ApplyScheduleRange(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ApplyScheduleRange failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change the rota.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ApplyScheduleAbsence handles POST /team-schedule/absences/apply.
func (h *ScheduleHandler) ApplyScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.ApplyScheduleAbsence(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ApplyScheduleAbsence failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change who is away.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteScheduleAbsence handles DELETE /team-schedule/absences/{id} -- the
// picker removing a whole span of leave or allocation in one click.
func (h *ScheduleHandler) DeleteScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAbsence(r.Context(), r.PathValue("id"), r.URL.Query().Get("note")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAbsence failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to remove that leave or allocation.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateScheduleAbsenceKind handles POST /team-schedule/absence-kinds -- a
// lead adding a leave or allocation tag the catalogue does not have yet.
func (h *ScheduleHandler) CreateScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateScheduleAbsenceKind(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateScheduleAbsenceKind failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add the tag.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// DeleteScheduleAbsenceKind handles DELETE /team-schedule/absence-kinds/{code}
// -- a lead deleting a tag a lead added, once nothing uses it.
func (h *ScheduleHandler) DeleteScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAbsenceKind(r.Context(), r.PathValue("code")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAbsenceKind failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to delete the tag.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetScheduleEditMarkers handles GET /team-schedule/edit-markers.
func (h *ScheduleHandler) GetScheduleEditMarkers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	result, err := h.entity.GetScheduleEditMarkers(r.Context(), q.Get("from"), q.Get("to"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleEditMarkers failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who changed the rota.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// viewerScheduleClient abstracts the ServiceNow ABT team schedule operation
// used by ViewerScheduleHandler.
type viewerScheduleClient interface {
	GetABTTeamSchedule(ctx context.Context, from, duration, teamID, eventType, teamScheduleURL string) (servicenow.ABTTeamScheduleData, error)
}

// ViewerScheduleHandler handles HTTP requests for the ABT team schedule,
// delegating to the ServiceNow service.
type ViewerScheduleHandler struct {
	servicenow      viewerScheduleClient
	accessGuard     *AccessGuard
	teamScheduleURL string
}

// NewViewerScheduleHandler creates a ViewerScheduleHandler backed by the given
// ServiceNow client. accessGuard enforces PermViewerAccess, SupportPortalLite's
// blanket audience gate; teamScheduleURL is the static URL echoed back in
// every response (TEAM_SCHEDULE_URL).
func NewViewerScheduleHandler(sn viewerScheduleClient, accessGuard *AccessGuard, teamScheduleURL string) *ViewerScheduleHandler {
	return &ViewerScheduleHandler{servicenow: sn, accessGuard: accessGuard, teamScheduleURL: teamScheduleURL}
}

// GetABTTeamSchedule handles GET /abt-team-schedule. All query
// parameters are optional, mirroring the Ballerina resource function's
// `string?` parameters.
func (h *ViewerScheduleHandler) GetABTTeamSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	q := r.URL.Query()
	from, duration, teamID, eventType := q.Get("from"), q.Get("duration"), q.Get("teamId"), q.Get("eventType")
	if teamID != "" {
		if err := servicenow.SanitizeQueryValue(teamID); err != nil {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	}

	schedule, err := h.servicenow.GetABTTeamSchedule(r.Context(), from, duration, teamID, eventType, h.teamScheduleURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetABTTeamSchedule failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve team schedule.")
		return
	}

	writeJSONValue(w, http.StatusOK, schedule)
}
