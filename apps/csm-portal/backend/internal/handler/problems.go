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
	GetProblem(ctx context.Context, id string) ([]byte, error)
	CreateProblem(ctx context.Context, body []byte) ([]byte, error)
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
		mapUpstreamError(w, err, "Failed to search problems.")
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
		mapUpstreamError(w, err, "Failed to create problem.")
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
		mapUpstreamError(w, err, "Failed to retrieve problem.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
