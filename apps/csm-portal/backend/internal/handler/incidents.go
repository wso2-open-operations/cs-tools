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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityIncidentClient abstracts the entity service incident operations used by IncidentHandler.
type entityIncidentClient interface {
	SearchIncidents(ctx context.Context, body []byte) ([]byte, error)
	CreateIncident(ctx context.Context, body []byte) ([]byte, error)
	GetIncident(ctx context.Context, id string) ([]byte, error)
	PatchIncident(ctx context.Context, id string, body []byte) ([]byte, error)
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
)

// createIncidentRequest mirrors the enum/format-constrained fields of the documented
// CreateIncidentPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type createIncidentRequest struct {
	CallerID            string   `json:"callerId"`
	Category            string   `json:"category"`
	Subcategory         string   `json:"subcategory"`
	ServiceID           string   `json:"serviceId"`
	ServiceOfferingID   string   `json:"serviceOfferingId"`
	ConfigurationItemID string   `json:"configurationItemId"`
	ContactType         string   `json:"contactType"`
	Impact              string   `json:"impact"`
	Urgency             string   `json:"urgency"`
	AssignmentGroupID   string   `json:"assignmentGroupId"`
	AssignedEngineerID  string   `json:"assignedEngineerId"`
	Subject             string   `json:"subject"`
	WatchList           []string `json:"watchList"`
	ParentID            string   `json:"parentId"`
	ParentIncidentID    string   `json:"parentIncidentId"`
	ChangeRequestID     string   `json:"changeRequestId"`
	ProblemID           string   `json:"problemId"`
	CausedByID          string   `json:"causedById"`
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

// IncidentHandler handles HTTP requests for incident operations, delegating to the
// entity service for data access.
type IncidentHandler struct {
	entity entityIncidentClient
}

// NewIncidentHandler creates an IncidentHandler backed by the given entity client.
func NewIncidentHandler(entity entityIncidentClient) *IncidentHandler {
	return &IncidentHandler{entity: entity}
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
		mapUpstreamError(w, err, "Failed to search incidents.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
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
		mapUpstreamError(w, err, "Failed to create incident.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
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
		mapUpstreamError(w, err, "Failed to retrieve incident.")
		return
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

	result, err := h.entity.PatchIncident(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchIncident failed", "userID", user.UserID, "incidentID", id, "err", err)
		mapUpstreamError(w, err, "Failed to update incident.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
