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
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityChangeRequestClient abstracts the entity-service change-request
// operations used by ChangeRequestHandler.
type entityChangeRequestClient interface {
	CreateChangeRequest(ctx context.Context, req entity.CreateChangeRequestRequest) (entity.CreateChangeRequestResponse, error)
	SearchChangeRequests(ctx context.Context, req entity.SearchChangeRequestsRequest) (entity.SearchChangeRequestsResponse, error)
	GetChangeRequest(ctx context.Context, id string) (entity.ChangeRequest, error)
	UpdateChangeRequest(ctx context.Context, id string, req entity.PatchChangeRequestRequest) (entity.PatchChangeRequestResponse, error)
	GetChangeRequestApprovals(ctx context.Context, id string) (entity.ChangeRequestApprovals, error)
	DecideChangeRequestApproval(ctx context.Context, id string, req entity.ChangeRequestApprovalDecisionRequest) (entity.ChangeRequestApprovalDecisionResponse, error)
}

// ChangeRequestHandler handles HTTP requests for change-request operations.
//
// entity-service serves these routes on its PostgreSQL data source as well as on
// its ServiceNow one (a Postgres-mode deployment does not 404 them). What is
// computed for the customer -- customerCanAnswer, the expected-window check, the
// proposal rules -- is entity-service's PostgreSQL data source's, and absent on
// ServiceNow's.
type ChangeRequestHandler struct {
	entity entityChangeRequestClient
	// now is the clock a proposed implementation time is held against ("must be
	// still to come"); a field so a test can fix it.
	now func() time.Time
}

// Messages of PATCH /change-requests/{id} when it is served at the customer
// level (see patchChangeRequestAsCustomer).
const (
	errMsgCustomerPatchFields   = "Customers can only approve or reject a change request, confirm or reject its review, or propose a new implementation time. Other fields cannot be changed."
	errMsgCustomerPatchMixed    = "Send the approval or review and a proposed implementation time as separate requests."
	errMsgCustomerPatchBoth     = "Send either isCustomerApproved or isCustomerReviewed, not both."
	errMsgCustomerPatchEmpty    = "At least one of isCustomerApproved, isCustomerReviewed, plannedStartOn or plannedEndOn must be provided."
	errMsgCustomerPatchExpected = "expectedPlannedStartOn and expectedPlannedEndOn go with isCustomerApproved or isCustomerReviewed only."
)

// errMsgStaffPatchExpected is the 400 of PATCH /change-requests/{id} served at the
// staff level (see PatchChangeRequest) for a body that names the planned window a
// customer's answer was given for. That window is a precondition of the
// customer's own answer, which no staff action gives, so on a staff body it could
// only be a check that would never run: it is refused rather than dropped.
const errMsgStaffPatchExpected = "expectedPlannedStartOn and expectedPlannedEndOn go with a customer's own answer (isCustomerApproved or isCustomerReviewed), which staff cannot give; remove them from this request."

// errCodeChangeRequestForbidden is the machine-readable name (the error body's
// errorCode) of the refusals that mean "a customer may not do this here": the same
// code entity-service gives its own 403s of that kind (apierror.CodeChangeRequestForbidden
// there), so the webapp has one name for them whichever layer refused. This layer
// raises it for a field a customer may not set.
const errCodeChangeRequestForbidden = "change_request_forbidden"

// NewChangeRequestHandler creates a ChangeRequestHandler backed by the given entity client.
func NewChangeRequestHandler(entity entityChangeRequestClient) *ChangeRequestHandler {
	return &ChangeRequestHandler{entity: entity, now: time.Now}
}

// refusePlannedWindow answers a 400 with the readable reason when the window a
// request carries is not acceptable, and reports whether it did. entity-service
// checks the same again (this is the first layer, not the only one); the point
// here is that a typing mistake is told what is wrong before a round trip, in the
// words the webapp already maps.
func refusePlannedWindow(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	writeError(w, http.StatusBadRequest, err.Error())
	return true
}

// CreateChangeRequest handles POST /change-requests.
func (h *ChangeRequestHandler) CreateChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChangeRequestCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Subject == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if refusePlannedWindow(w, dto.ValidatePlannedWindow("plannedStartDate", req.PlannedStartDate, "plannedEndDate", req.PlannedEndDate, dto.PlannedWindowRules{})) {
		return
	}

	result, err := h.entity.CreateChangeRequest(r.Context(), dto.BuildEntityCreateChangeRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateChangeRequest failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create change request.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapChangeRequestCreate(result))
}

// SearchChangeRequests handles POST /projects/{id}/change-requests/search.
func (h *ChangeRequestHandler) SearchChangeRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChangeRequestSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchChangeRequests(r.Context(), dto.BuildEntitySearchChangeRequestsRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchChangeRequests failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search change requests.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchChangeRequests(result))
}

// GetChangeRequest handles GET /change-requests/{id}.
func (h *ChangeRequestHandler) GetChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetChangeRequest(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve change request.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestDetails(result))
}

// PatchChangeRequest handles PATCH /change-requests/{id}.
//
// The route lets two kinds of caller in (middleware.RequirePermissionOneOf) and
// this handler honours as much of the body as the level they came in at:
//
//   - ActionUpdate (admin / agent / internal): the full customer-safe field set
//     of dto.ChangeRequestUpdateRequest, as before. Keys outside that set are
//     dropped by the decode (state, assignedTeamId, ...); the one exception is the
//     expected window of a customer's answer, which a staff body can never carry
//     and is refused with a 400 (errMsgStaffPatchExpected) rather than dropped,
//     because dropping it would silently lose a check the caller asked for.
//   - ActionDecide only (customer / partner roles): the customer's own answer
//     and nothing else -- see patchChangeRequestAsCustomer.
//
// Anything other than a positive match on ActionUpdate is served at the customer
// level, including a request that never passed through the middleware: the
// restriction cannot be lost by forgetting to wire it.
func (h *ChangeRequestHandler) PatchChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	if granted, _ := middleware.GrantedActionFromContext(r.Context()); granted != middleware.ActionUpdate {
		h.patchChangeRequestAsCustomer(w, r, user.UserID, id, body)
		return
	}

	var req dto.ChangeRequestUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.HasExpectedWindow() {
		writeError(w, http.StatusBadRequest, errMsgStaffPatchExpected)
		return
	}
	if req == (dto.ChangeRequestUpdateRequest{}) {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}
	if refusePlannedWindow(w, dto.ValidatePlannedWindow("plannedStartOn", req.PlannedStartOn, "plannedEndOn", req.PlannedEndOn, dto.PlannedWindowRules{})) {
		return
	}

	result, err := h.entity.UpdateChangeRequest(r.Context(), id, dto.BuildEntityPatchChangeRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateChangeRequest failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update change request.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestUpdate(result))
}

// patchChangeRequestAsCustomer serves PATCH /change-requests/{id} for a caller
// who may give the customer's answer but not edit the change request.
//
// The body is decoded into dto.ChangeRequestCustomerUpdateRequest with unknown
// fields refused, so any key outside its six fields -- a real one such as
// "title" or "state", a misspelt one, or one that differs only in case -- is a
// 403 and nothing is forwarded; and entity-service's request is then built from
// those six fields alone. There is no code path by which an extra key rides
// along.
//
// The body must be exactly one of:
//
//   - the customer's answer: isCustomerApproved (Customer Approval) or
//     isCustomerReviewed (Customer Review), one of the two, true or false,
//     optionally with expectedPlannedStartOn / expectedPlannedEndOn, the window
//     the customer was shown (the answer is then only recorded while that is
//     still the window);
//   - a proposed implementation window: plannedStartOn and/or plannedEndOn.
//
// Combining the two is refused (400): a customer who proposes a different time
// has not approved the old one, and the two have different outcomes upstream.
// The expected window with anything but an answer is refused too.
//
// Whether this caller may answer THIS change request is not decided here.
// entity-service resolves the caller from the forwarded user token and accepts
// the answer only from a REGISTERED PORTAL_USER contact of the change request's
// own project (a customer of another project is refused with a 403), only
// while the change request is in the state the answer belongs to (otherwise a
// 409), and records it exactly as the approvals/decision route does.
func (h *ChangeRequestHandler) patchChangeRequestAsCustomer(w http.ResponseWriter, r *http.Request, userID, id string, body []byte) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req dto.ChangeRequestCustomerUpdateRequest
	if err := dec.Decode(&req); err != nil {
		// encoding/json has no typed error for an unknown field.
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			writeErrorCode(w, http.StatusForbidden, errMsgCustomerPatchFields, errCodeChangeRequestForbidden)
			return
		}
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	switch {
	case req.HasExpectedWindow() && !req.HasDecision():
		writeError(w, http.StatusBadRequest, errMsgCustomerPatchExpected)
		return
	case !req.HasDecision() && !req.HasWindow():
		writeError(w, http.StatusBadRequest, errMsgCustomerPatchEmpty)
		return
	case req.HasDecision() && req.HasWindow():
		writeError(w, http.StatusBadRequest, errMsgCustomerPatchMixed)
		return
	case req.IsCustomerApproved != nil && req.IsCustomerReviewed != nil:
		writeError(w, http.StatusBadRequest, errMsgCustomerPatchBoth)
		return
	}
	// A proposed time is checked before anything is sent: well-formed, in range,
	// still to come, and (with both bounds) a window with a duration. entity-service
	// repeats every one of these against what is stored, and is the authority.
	if req.HasWindow() && refusePlannedWindow(w, dto.ValidatePlannedWindow("plannedStartOn", req.PlannedStartOn, "plannedEndOn", req.PlannedEndOn,
		dto.PlannedWindowRules{Now: h.now(), RequireFuture: true, RequireOrder: true})) {
		return
	}

	result, err := h.entity.UpdateChangeRequest(r.Context(), id, dto.BuildEntityCustomerPatchChangeRequestRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateChangeRequest (customer) failed", "userID", userID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update change request.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestUpdate(result))
}

// GetChangeRequestApprovals handles GET /change-requests/{id}/approvals.
func (h *ChangeRequestHandler) GetChangeRequestApprovals(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetChangeRequestApprovals(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequestApprovals failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve change request approvals.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestApprovals(result))
}

// DecideChangeRequestApproval handles POST /change-requests/{id}/approvals/decision.
func (h *ChangeRequestHandler) DecideChangeRequestApproval(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.ChangeRequestApprovalDecisionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Decision != "approved" && req.Decision != "rejected" {
		writeError(w, http.StatusBadRequest, "decision must be \"approved\" or \"rejected\".")
		return
	}

	result, err := h.entity.DecideChangeRequestApproval(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DecideChangeRequestApproval failed", "userID", user.UserID, "changeRequestID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to record approval decision.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapChangeRequestApprovalDecision(result))
}
