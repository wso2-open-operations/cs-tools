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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityProblemClient abstracts the entity service problem operations used by ProblemHandler.
type entityProblemClient interface {
	SearchProblems(ctx context.Context, body []byte) ([]byte, error)
	AggregateProblems(ctx context.Context, body []byte) ([]byte, error)
	GetProblem(ctx context.Context, id string) ([]byte, error)
	CreateProblem(ctx context.Context, body []byte) ([]byte, error)
	UpdateProblem(ctx context.Context, id string, body []byte) ([]byte, error)
}

// createProblemRequest mirrors the enum/format-constrained fields of the documented
// CreateProblemPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type createProblemRequest struct {
	Subject           string `json:"subject"`
	Category          string `json:"category"`
	Subcategory       string `json:"subcategory"`
	OriginCaseID      string `json:"originCaseId"`
	PrimaryIncidentID string `json:"primaryIncidentId"`
}

// validateCreateProblemBody checks that the body decodes as a JSON object with no
// unknown fields, that the required subject is present and non-blank, and that any
// optional UUID-formatted linking fields are well-formed, so obviously invalid
// requests are rejected before reaching the entity service.
func validateCreateProblemBody(body []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req createProblemRequest
	if err := dec.Decode(&req); err != nil {
		return false
	}
	if dec.More() {
		return false
	}
	if strings.TrimSpace(req.Subject) == "" {
		return false
	}
	if req.OriginCaseID != "" && !uuidRe.MatchString(req.OriginCaseID) {
		return false
	}
	if req.PrimaryIncidentID != "" && !uuidRe.MatchString(req.PrimaryIncidentID) {
		return false
	}
	return true
}

// updateProblemRequest mirrors the format-constrained fields of the documented
// UpdateProblemPayload schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
//
// Transition is deliberately not represented here as a typed/validated field: it is one
// of "assess"/"confirm"/"fix"/"resolve"/"close" per ServiceNow's own server-side
// validation, but every layer below this one (ServiceNow, the Ballerina entity-service,
// the Go entity-service) ships it as a plain unvalidated string on purpose. A closed-enum
// check here would swallow ServiceNow's own actionable error message ("Invalid
// transition: ...") behind a generic validation error instead. Do not add one.
type updateProblemRequest struct {
	AssignedToID      string `json:"assignedToId"`
	AssignmentGroupID string `json:"assignmentGroupId"`
}

// validateUpdateProblemBody rejects an empty JSON object (matching the "at least one
// field required" contract of UpdateProblemPayload), and checks the UUID-formatted
// fields (assignedToId, assignmentGroupId) when present and non-empty. transition is
// intentionally left unconstrained beyond decoding as a string (see updateProblemRequest
// doc comment) — an absent, null, or non-string value for it is handled by the type
// decode itself (json.Unmarshal fails for a non-string transition), not by a semantic
// check here.
func validateUpdateProblemBody(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	if len(fields) == 0 {
		return false
	}

	// Decoding transition here (as *string) only confirms it is a JSON string (or
	// null/absent) without constraining its value, matching the "no closed-enum
	// validation" requirement while still rejecting an invalid JSON type such as
	// "transition": 5.
	var transition struct {
		Transition *string `json:"transition"`
	}
	if err := json.Unmarshal(body, &transition); err != nil {
		return false
	}

	var req updateProblemRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if req.AssignedToID != "" && !uuidRe.MatchString(req.AssignedToID) {
		return false
	}
	if req.AssignmentGroupID != "" && !uuidRe.MatchString(req.AssignmentGroupID) {
		return false
	}
	return true
}

// ProblemHandler handles HTTP requests for problem operations, delegating to the
// entity service for data access.
type ProblemHandler struct {
	entity entityProblemClient
}

// NewProblemHandler creates a ProblemHandler backed by the given entity client.
func NewProblemHandler(entity entityProblemClient) *ProblemHandler {
	return &ProblemHandler{entity: entity}
}

// SearchProblems handles POST /problems/search.
func (h *ProblemHandler) SearchProblems(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchProblems(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProblems failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search problems.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateProblems handles POST /problems/aggregate.
// Server-side aggregation of problems by a single field (e.g. state,
// assignmentGroup), capped to the top maxGroups buckets with the remainder
// folded into othersCount. The groupBy allowlist is validated upstream by
// the entity service; this layer only forwards the request and passes the
// response through as-is.
func (h *ProblemHandler) AggregateProblems(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.AggregateProblems(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateProblems failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate problems.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateProblem handles POST /problems.
func (h *ProblemHandler) CreateProblem(w http.ResponseWriter, r *http.Request) {
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

	if !validateCreateProblemBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateProblem(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateProblem failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create problem.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetProblem handles GET /problems/{id}.
func (h *ProblemHandler) GetProblem(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetProblem(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProblem failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve problem.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchProblem handles PATCH /problems/{id}.
func (h *ProblemHandler) PatchProblem(w http.ResponseWriter, r *http.Request) {
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

	if !validateUpdateProblemBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.UpdateProblem(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateProblem failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update problem.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
