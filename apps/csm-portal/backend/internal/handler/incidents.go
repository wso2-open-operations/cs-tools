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
)

// entityIncidentClient abstracts the entity service incident operations used by IncidentHandler.
type entityIncidentClient interface {
	SearchIncidents(ctx context.Context, body []byte) ([]byte, error)
	AggregateIncidents(ctx context.Context, body []byte) ([]byte, error)
	CreateIncident(ctx context.Context, body []byte) ([]byte, error)
	GetIncident(ctx context.Context, id string) ([]byte, error)
	PatchIncident(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateComment(ctx context.Context, body []byte) ([]byte, error)
	SearchComments(ctx context.Context, body []byte) ([]byte, error)
	SearchIncidentActivities(ctx context.Context, id string, body []byte) ([]byte, error)
	HandOffIncidentToSpecialist(ctx context.Context, id string, body []byte) ([]byte, error)
	ListSpecialistHandoffTeams(ctx context.Context, serviceID string) ([]byte, error)
	GetIncidentCreateDefaults(ctx context.Context) ([]byte, error)
	// GetUserMe resolves the caller's own platform user record — needed by
	// the close-ownership guard in PatchIncident; see resolveCurrentUserID
	// (cases.go), shared with CaseHandler's own identical use.
	GetUserMe(ctx context.Context) ([]byte, error)
}

// searchIncidentsRequest mirrors the enum/format-constrained fields of the documented
// IncidentSearchPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type searchIncidentsRequest struct {
	Filters struct {
		Priorities []string `json:"priorities"`
		ParentIDs  []string `json:"parentIds"`
	} `json:"filters"`
	SortBy struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sortBy"`
}

var (
	validIncidentPriorities = map[string]bool{
		"CRITICAL": true,
		"HIGH":     true,
		"MODERATE": true,
		"LOW":      true,
		"PLANNING": true,
	}
	validIncidentSortFields = map[string]bool{"createdOn": true, "updatedOn": true, "openedOn": true}
	validIncidentSortOrders = map[string]bool{"asc": true, "desc": true}

	validIncidentCategories    = map[string]bool{"INQUIRY": true, "SERVICE_INTERRUPTION": true, "SECURITY": true}
	validIncidentSubcategories = map[string]bool{
		"DHCP": true, "ORACLE": true, "CPU": true, "KEYBOARD": true, "DOS_DDOS": true,
		"PRIVILEGE_ESCALATIONS": true, "THREAT_INTELLIGENCE": true, "SCANS_AND_PROBES": true,
		"APPLICATION_SECURITY": true, "CONFIG_CHANGE_REQUEST": true, "IP_ADDRESS": true,
		"FULL_OUTAGE": true, "SQL_SERVER": true, "SLOWNESS": true, "MEMORY": true, "MOUSE": true,
		"PRIVACY": true, "DATA_BREACH": true, "SYSTEM_COMPROMISES": true, "DNS": true, "OS": true,
		"DISK": true, "VPN": true, "MALWARE": true, "VULNERABILITY": true, "UNAUTHORIZED_ACCESS": true,
		"IDENTITY_PROTECTION": true, "PHISHING": true, "IMPROPER_CONFIGURATION": true,
		"INFORMATION_REQUEST": true, "DB2": true, "PARTIAL_OUTAGE": true, "EMAIL": true,
		"MONITOR": true, "WIRELESS": true,
	}
	validIncidentContactTypes = map[string]bool{
		"SELF_SERVICE": true, "EMAIL": true, "WALK_IN": true, "AZURE": true, "EMAIL_INTERNAL": true,
		"SITE_247": true, "DIRECT": true, "PHONE": true, "SENTINEL": true, "VIRTUAL_AGENT": true,
		"CHAT": true, "EMAIL_EXTERNAL": true,
	}
	validIncidentImpacts   = map[string]bool{"HIGH": true, "MEDIUM": true, "LOW": true}
	validIncidentUrgencies = map[string]bool{"HIGH": true, "MEDIUM": true, "LOW": true}

	validIncidentStates = map[string]bool{
		"NEW": true, "IN_PROGRESS": true, "ON_HOLD": true, "RESOLVED": true, "CLOSED": true, "CANCELLED": true,
	}

	validHandoffReasonCodes = map[string]bool{"no-runbook": true, "runbook-not-working": true}
)

// maxHandoffEscalationTeamLen bounds escalationTeam's shape. Which keys are
// valid is the entity service's SPECIALIST_HANDOFF_CONFIG, per product, so
// it -- not a list here -- decides, and answers 400 for an unknown team.
const maxHandoffEscalationTeamLen = 64

// createIncidentRequest mirrors the enum/format-constrained fields of the documented
// CreateIncidentPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type createIncidentRequest struct {
	CallerID            string `json:"callerId"`
	Category            string `json:"category"`
	Subcategory         string `json:"subcategory"`
	ServiceID           string `json:"serviceId"`
	ServiceOfferingID   string `json:"serviceOfferingId"`
	ConfigurationItemID string `json:"configurationItemId"`
	ContactType         string `json:"contactType"`
	Impact              string `json:"impact"`
	Urgency             string `json:"urgency"`
	// AssignmentGroupID is optional. Only its shape is checked here; whether
	// the group may be chosen (an active support group of some service) is
	// the entity service's rule, answered there with 400.
	AssignmentGroupID  string   `json:"assignmentGroupId"`
	AssignedEngineerID string   `json:"assignedEngineerId"`
	Subject            string   `json:"subject"`
	WatchList          []string `json:"watchList"`
	ParentID           string   `json:"parentId"`
	ParentIncidentID   string   `json:"parentIncidentId"`
	ChangeRequestID    string   `json:"changeRequestId"`
	ProblemID          string   `json:"problemId"`
	CausedByID         string   `json:"causedById"`
}

// validateCreateIncidentBody checks the required fields, enum fields (category, subcategory,
// contactType, impact, urgency), and every UUID-formatted field with a known, fixed set of
// valid values so obviously invalid requests are rejected before reaching the entity service.
func validateCreateIncidentBody(body []byte) bool {
	var req createIncidentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if req.CallerID == "" || !uuidRe.MatchString(req.CallerID) {
		return false
	}
	if !validIncidentCategories[req.Category] {
		return false
	}
	if req.Subcategory != "" && !validIncidentSubcategories[req.Subcategory] {
		return false
	}
	if req.ServiceID == "" || !uuidRe.MatchString(req.ServiceID) {
		return false
	}
	if req.ServiceOfferingID != "" && !uuidRe.MatchString(req.ServiceOfferingID) {
		return false
	}
	if req.ConfigurationItemID != "" && !uuidRe.MatchString(req.ConfigurationItemID) {
		return false
	}
	if req.ContactType != "" && !validIncidentContactTypes[req.ContactType] {
		return false
	}
	if !validIncidentImpacts[req.Impact] {
		return false
	}
	if !validIncidentUrgencies[req.Urgency] {
		return false
	}
	if req.AssignmentGroupID != "" && !uuidRe.MatchString(req.AssignmentGroupID) {
		return false
	}
	if req.AssignedEngineerID != "" && !uuidRe.MatchString(req.AssignedEngineerID) {
		return false
	}
	if req.Subject == "" {
		return false
	}
	for _, w := range req.WatchList {
		if !uuidRe.MatchString(w) {
			return false
		}
	}
	if req.ParentID != "" && !uuidRe.MatchString(req.ParentID) {
		return false
	}
	if req.ParentIncidentID != "" && !uuidRe.MatchString(req.ParentIncidentID) {
		return false
	}
	if req.ChangeRequestID != "" && !uuidRe.MatchString(req.ChangeRequestID) {
		return false
	}
	if req.ProblemID != "" && !uuidRe.MatchString(req.ProblemID) {
		return false
	}
	if req.CausedByID != "" && !uuidRe.MatchString(req.CausedByID) {
		return false
	}
	return true
}

// updateIncidentRequest mirrors the enum/format-constrained fields of the documented
// UpdateIncidentPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type updateIncidentRequest struct {
	Priority            string   `json:"priority"`
	State               string   `json:"state"`
	Category            string   `json:"category"`
	Subcategory         string   `json:"subcategory"`
	ContactType         string   `json:"contactType"`
	Impact              string   `json:"impact"`
	Urgency             string   `json:"urgency"`
	ParentID            string   `json:"parentId"`
	ParentIncidentID    string   `json:"parentIncidentId"`
	AssignmentGroupID   string   `json:"assignmentGroupId"`
	AssignedEngineerID  string   `json:"assignedEngineerId"`
	ServiceID           string   `json:"serviceId"`
	ServiceOfferingID   string   `json:"serviceOfferingId"`
	ConfigurationItemID string   `json:"configurationItemId"`
	ChangeRequestID     string   `json:"changeRequestId"`
	ProblemID           string   `json:"problemId"`
	CausedByID          string   `json:"causedById"`
	ResolvedByID        string   `json:"resolvedById"`
	WatchList           []string `json:"watchList"`
}

// validateUpdateIncidentBody rejects an empty JSON object (matching the documented
// minProperties: 1 on UpdateIncidentPayload — a PATCH must change at least one field),
// and checks the enum fields (priority, state, category, subcategory, contactType,
// impact, urgency) and every UUID-formatted field with a known, fixed set of valid
// values so obviously invalid requests are rejected before reaching the entity service.
// An absent, null, or empty-string value for any individual optional field is treated
// as "leave unchanged" or "clear", matching the entity service's own PATCH semantics,
// and is not rejected here.
func validateUpdateIncidentBody(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	if len(fields) == 0 {
		return false
	}

	var req updateIncidentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if req.Priority != "" && !validIncidentPriorities[req.Priority] {
		return false
	}
	if req.State != "" && !validIncidentStates[req.State] {
		return false
	}
	if req.Category != "" && !validIncidentCategories[req.Category] {
		return false
	}
	if req.Subcategory != "" && !validIncidentSubcategories[req.Subcategory] {
		return false
	}
	if req.ContactType != "" && !validIncidentContactTypes[req.ContactType] {
		return false
	}
	if req.Impact != "" && !validIncidentImpacts[req.Impact] {
		return false
	}
	if req.Urgency != "" && !validIncidentUrgencies[req.Urgency] {
		return false
	}
	if req.ParentID != "" && !uuidRe.MatchString(req.ParentID) {
		return false
	}
	if req.ParentIncidentID != "" && !uuidRe.MatchString(req.ParentIncidentID) {
		return false
	}
	if req.AssignmentGroupID != "" && !uuidRe.MatchString(req.AssignmentGroupID) {
		return false
	}
	if req.AssignedEngineerID != "" && !uuidRe.MatchString(req.AssignedEngineerID) {
		return false
	}
	if req.ServiceID != "" && !uuidRe.MatchString(req.ServiceID) {
		return false
	}
	if req.ServiceOfferingID != "" && !uuidRe.MatchString(req.ServiceOfferingID) {
		return false
	}
	if req.ConfigurationItemID != "" && !uuidRe.MatchString(req.ConfigurationItemID) {
		return false
	}
	if req.ChangeRequestID != "" && !uuidRe.MatchString(req.ChangeRequestID) {
		return false
	}
	if req.ProblemID != "" && !uuidRe.MatchString(req.ProblemID) {
		return false
	}
	if req.CausedByID != "" && !uuidRe.MatchString(req.CausedByID) {
		return false
	}
	if req.ResolvedByID != "" && !uuidRe.MatchString(req.ResolvedByID) {
		return false
	}
	for _, w := range req.WatchList {
		if !uuidRe.MatchString(w) {
			return false
		}
	}
	return true
}

// validateSearchIncidentsBody checks the filter/sort fields with a known, fixed set of
// valid values (priority enums, parentIds as UUIDs, sort field/order enums) so obviously
// invalid requests are rejected before reaching the entity service.
func validateSearchIncidentsBody(body []byte) bool {
	var req searchIncidentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	for _, p := range req.Filters.Priorities {
		if !validIncidentPriorities[p] {
			return false
		}
	}
	for _, id := range req.Filters.ParentIDs {
		if !uuidRe.MatchString(id) {
			return false
		}
	}
	if req.SortBy.Field != "" && !validIncidentSortFields[req.SortBy.Field] {
		return false
	}
	if req.SortBy.Order != "" && !validIncidentSortOrders[req.SortBy.Order] {
		return false
	}
	return true
}

// handOffIncidentRequest mirrors the enum-constrained fields of the documented
// HandOffIncidentToSpecialistRequest schema. It is decoded only to validate those
// fields at the boundary; the original raw body is still forwarded to the entity
// service unchanged.
type handOffIncidentRequest struct {
	ReasonCode     string  `json:"reasonCode"`
	EscalationTeam *string `json:"escalationTeam"`
}

// validateHandOffIncidentBody checks the required reasonCode enum and, when present,
// the escalationTeam enum, so obviously invalid requests are rejected before reaching
// the entity service. createGithubIssue is a plain boolean with no enum to check; an
// invalid type there is caught by json.Unmarshal.
func validateHandOffIncidentBody(body []byte) bool {
	var req handOffIncidentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if !validHandoffReasonCodes[req.ReasonCode] {
		return false
	}
	if req.EscalationTeam != nil && len(*req.EscalationTeam) > maxHandoffEscalationTeamLen {
		return false
	}
	return true
}

// IncidentHandler handles HTTP requests for incident operations, delegating to the
// entity service for data access.
type IncidentHandler struct {
	entity entityIncidentClient
	// access backs the inline-image redaction in every read response — see
	// WithAccessGuard and CaseHandler's own field of the same name/reasoning.
	// nil fails that check closed (redacts), never open.
	access *AccessGuard
}

// NewIncidentHandler creates an IncidentHandler backed by the given entity client.
func NewIncidentHandler(entity entityIncidentClient) *IncidentHandler {
	return &IncidentHandler{entity: entity}
}

// WithAccessGuard wires the same guard that authorises every route into this
// handler, so every read response can redact an embedded raw base64 inline
// image (see redactRawBase64Images's own doc comment) for a caller who
// lacks PermDownloadAttachment. Returns h for chaining at the construction
// site.
func (h *IncidentHandler) WithAccessGuard(g *AccessGuard) *IncidentHandler {
	h.access = g
	return h
}

// SearchIncidents handles POST /incidents/search.
func (h *IncidentHandler) SearchIncidents(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !validateSearchIncidentsBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchIncidents(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchIncidents failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search incidents.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}

// AggregateIncidents handles POST /incidents/aggregate.
// Server-side aggregation of incidents by a single field (e.g. state,
// assignmentGroup, businessService), capped to the top maxGroups buckets
// with the remainder folded into othersCount. The groupBy allowlist is
// validated upstream by the entity service; this layer only forwards the
// request and passes the response through as-is.
func (h *IncidentHandler) AggregateIncidents(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.AggregateIncidents(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateIncidents failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate incidents.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateIncident handles POST /incidents.
func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !validateCreateIncidentBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateIncident(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateIncident failed", "userID", user.UserID, "err", err)
		mapCreateIncidentError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// errCodeIncidentAssignmentGroupNotAllowed is the entity service's errorCode
// for an assignmentGroupId that is not an active support group of any service.
const errCodeIncidentAssignmentGroupNotAllowed = "incident_assignment_group_not_allowed"

// mapCreateIncidentError maps a failed create. The one refusal the create form
// has to show as such -- the chosen group is no longer an active support group
// (errorCode incident_assignment_group_not_allowed) -- keeps its 400, its
// message and its errorCode. Every other upstream error goes through
// mapUpstreamErrorGeneric unchanged, so no other entity-service 400 message
// (some quote database details) reaches the caller.
func mapCreateIncidentError(w http.ResponseWriter, err error) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest &&
		upstreamErrorCode(apiErr.Body) == errCodeIncidentAssignmentGroupNotAllowed {
		if msg := upstreamErrorMessageStrict(apiErr.Body, ""); msg != "" {
			writeErrorCode(w, http.StatusBadRequest, msg, errCodeIncidentAssignmentGroupNotAllowed)
			return
		}
	}
	mapUpstreamErrorGeneric(w, err, "Failed to create incident.")
}

// GetIncident handles GET /incidents/{id}.
func (h *IncidentHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetIncident(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetIncident failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve incident.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchIncident handles PATCH /incidents/{id}.
func (h *IncidentHandler) PatchIncident(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !validateUpdateIncidentBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Validate the state transition and (for a close) ownership before
	// forwarding to the entity service — mirrors PatchCase's own shape in
	// cases.go. One fetch of the current incident serves both checks.
	var patch struct {
		State *string `json:"state"`
	}
	patchErr := json.Unmarshal(body, &patch)
	if patchErr == nil && patch.State != nil {
		current, err := h.entity.GetIncident(r.Context(), id)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetIncident failed during state validation", "userID", user.UserID, "incidentID", id, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to update incident.")
			return
		}
		var currentIncident struct {
			State      string `json:"state"`
			AssignedTo *struct {
				ID string `json:"id"`
			} `json:"assignedTo"`
		}
		if err := json.Unmarshal(current, &currentIncident); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse current incident state", "userID", user.UserID, "incidentID", id, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}

		// Scenario 3: reject an illegal from→to transition before forwarding —
		// previously nothing server-side checked this at all (only that the
		// target value was a legal enum member), so a direct PATCH could jump
		// straight from NEW to CLOSED.
		if !isValidIncidentStateTransition(currentIncident.State, *patch.State) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidTransition)
			return
		}

		// Scenarios 1-2: closing an incident is restricted to its own
		// assignee, unless the caller is admin (an operational override).
		if *patch.State == incidentStateClosed && !(h.access != nil && h.access.Permits(PermAdmin, user.Roles)) {
			currentUserID := resolveCurrentUserID(r.Context(), h.entity, user)
			if currentUserID == "" {
				writeError(w, http.StatusInternalServerError, ErrMsgInternal)
				return
			}
			if currentIncident.AssignedTo == nil || currentIncident.AssignedTo.ID != currentUserID {
				writeError(w, http.StatusForbidden, ErrMsgIncidentCloseNotOwnCase)
				return
			}
		}
	}

	result, err := h.entity.PatchIncident(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchIncident failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamError(w, err, "Failed to update incident.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateIncidentComment handles POST /incidents/{id}/comments.
// Injects referenceId and referenceType into the payload and forwards to the entity
// service's reference-generic POST /comments.
func (h *IncidentHandler) CreateIncidentComment(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxCommentBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if _, err := h.entity.GetIncident(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity GetIncident failed during comment guard", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create incident comment.")
		return
	}

	newBody, err := injectReferenceFields(body, id, "incident")
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateComment(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create incident comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchIncidentActivities handles POST /incidents/{id}/activities/search.
// Confirmed as a real, distinct endpoint from the case one. The request body is capped
// and forwarded to the entity service as-is (no fields are injected) and the response is
// returned verbatim.
func (h *IncidentHandler) SearchIncidentActivities(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchIncidentActivities(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchIncidentActivities failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search incident activities.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchIncidentComments handles POST /incidents/{id}/comments/search.
// Injects referenceId and referenceType into the payload and forwards to the entity
// service's reference-generic POST /comments/search.
func (h *IncidentHandler) SearchIncidentComments(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	newBody, err := injectReferenceFields(body, id, "incident")
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SearchComments(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search incident comments.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// handOffIncidentResponseEnvelope is the subset of HandOffIncidentToSpecialistResponse
// this handler decodes for observability only. It is never re-marshalled or used to
// build the caller's response -- the raw upstream body is forwarded verbatim so
// githubIssueError (and every other field) reaches the caller unchanged.
type handOffIncidentResponseEnvelope struct {
	Handoff struct {
		GithubIssueError *string `json:"githubIssueError"`
	} `json:"handoff"`
}

// HandOffIncidentToSpecialist handles POST /incidents/{id}/specialist-handoffs.
// The handoff itself can succeed (ServiceNow state committed) while the internal
// GitHub issue creation fails -- the entity service reports that as a non-nil
// handoff.githubIssueError on an otherwise-200 response rather than an error status.
// This is surfaced explicitly in the server log rather than left to a caller who
// might not inspect the nested field, so a silent partial failure is still visible
// operator-side even if a webapp caller ever missed it. The response body itself is
// forwarded unchanged, so the caller always has the field to check too.
func (h *IncidentHandler) HandOffIncidentToSpecialist(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !validateHandOffIncidentBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.HandOffIncidentToSpecialist(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity HandOffIncidentToSpecialist failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamError(w, err, "Failed to hand off incident to specialist group.")
		return
	}

	var envelope handOffIncidentResponseEnvelope
	if err := json.Unmarshal(result, &envelope); err != nil {
		slog.WarnContext(r.Context(), "entity HandOffIncidentToSpecialist: decode response for githubIssueError check failed", "userID", user.UserID, "incidentID", id, "err", err)
	} else if envelope.Handoff.GithubIssueError != nil {
		slog.WarnContext(r.Context(), "incident handed off to specialist group but internal issue creation failed",
			"userID", user.UserID, "incidentID", id, "githubIssueError", *envelope.Handoff.GithubIssueError)
	}

	writeJSON(w, http.StatusOK, result)
}

// ListSpecialistHandoffTeams handles GET /specialist-handoff-teams?serviceId=: the Special
// Ops teams the "Escalate to specialist team" dialog offers for the incident's service,
// passed through from the entity service. Several teams mean the user must pick one; one
// team is the handoff's target with nothing to pick.
func (h *IncidentHandler) ListSpecialistHandoffTeams(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	serviceID := r.URL.Query().Get("serviceId")
	if serviceID != "" && !uuidRe.MatchString(serviceID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	result, err := h.entity.ListSpecialistHandoffTeams(r.Context(), serviceID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListSpecialistHandoffTeams failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to load the specialist teams.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetIncidentCreateDefaults handles GET /incidents/create-defaults: the
// entity service's default service and its support group -- the team an
// incident is assigned to when no group is chosen and its service has none --
// so the create form can show it. Passed through untouched:
// {"defaultServiceId": uuid|null, "defaultGroup": {"id","name"}|null}.
func (h *IncidentHandler) GetIncidentCreateDefaults(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.GetIncidentCreateDefaults(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetIncidentCreateDefaults failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the incident defaults.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
