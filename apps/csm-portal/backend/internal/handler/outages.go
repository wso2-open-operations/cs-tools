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

// entityOutageClient abstracts the entity service outage operations used by OutageHandler.
type entityOutageClient interface {
	CreateOutage(ctx context.Context, body []byte) ([]byte, error)
	SearchOutages(ctx context.Context, body []byte) ([]byte, error)
	GetOutage(ctx context.Context, id string) ([]byte, error)
	PatchOutage(ctx context.Context, id string, body []byte) ([]byte, error)
	AddOutageCommunication(ctx context.Context, id string, body []byte) ([]byte, error)
	SearchOutageCommunications(ctx context.Context, id string, body []byte) ([]byte, error)
	GetOutageMetadata(ctx context.Context) ([]byte, error)
}

var validOutageTypes = map[string]bool{"outage": true, "degradation": true, "planned": true}

var validOutageCommunicationChannels = map[string]bool{"external": true, "internal": true, "additional": true}

// createOutageRequest mirrors the enum/format-constrained fields of the documented
// CreateOutageRequest schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type createOutageRequest struct {
	Type                string  `json:"type"`
	Begin               string  `json:"begin"`
	ShortDescription    string  `json:"shortDescription"`
	ConfigurationItemID *string `json:"configurationItemId"`
	IncidentID          *string `json:"incidentId"`
}

// validateCreateOutageBody checks the required fields (type, begin, shortDescription),
// the type enum, and any UUID-formatted linking fields, so obviously invalid requests
// are rejected before reaching the entity service.
func validateCreateOutageBody(body []byte) bool {
	var req createOutageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if !validOutageTypes[req.Type] {
		return false
	}
	if req.Begin == "" {
		return false
	}
	if req.ShortDescription == "" {
		return false
	}
	if req.ConfigurationItemID != nil && *req.ConfigurationItemID != "" && !uuidRe.MatchString(*req.ConfigurationItemID) {
		return false
	}
	if req.IncidentID != nil && *req.IncidentID != "" && !uuidRe.MatchString(*req.IncidentID) {
		return false
	}
	return true
}

// patchOutageRequest mirrors the enum/format-constrained fields of the documented
// PatchOutageRequest schema. It is decoded only to validate those fields at the
// boundary; the original raw body is still forwarded to the entity service unchanged.
type patchOutageRequest struct {
	Type                string  `json:"type"`
	ConfigurationItemID *string `json:"configurationItemId"`
	IncidentID          *string `json:"incidentId"`
}

// validatePatchOutageBody rejects an empty JSON object (a PATCH must change at least
// one field), and checks the type enum and any UUID-formatted linking fields when
// present. end is intentionally left unvalidated beyond JSON decoding: it is a
// tri-state field (absent/null/value) that the entity service interprets itself
// (see domain.PatchOutageRequest), so no closed check belongs here.
func validatePatchOutageBody(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	if len(fields) == 0 {
		return false
	}

	var req patchOutageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if req.Type != "" && !validOutageTypes[req.Type] {
		return false
	}
	if req.ConfigurationItemID != nil && *req.ConfigurationItemID != "" && !uuidRe.MatchString(*req.ConfigurationItemID) {
		return false
	}
	if req.IncidentID != nil && *req.IncidentID != "" && !uuidRe.MatchString(*req.IncidentID) {
		return false
	}
	return true
}

// addOutageCommunicationRequest mirrors the enum-constrained fields of the documented
// AddOutageCommunicationRequest schema. It is decoded only to validate those fields at
// the boundary; the original raw body is still forwarded to the entity service unchanged.
type addOutageCommunicationRequest struct {
	Channel string `json:"channel"`
	Body    string `json:"body"`
}

// validateAddOutageCommunicationBody checks the channel enum and that body is non-blank.
func validateAddOutageCommunicationBody(body []byte) bool {
	var req addOutageCommunicationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if !validOutageCommunicationChannels[req.Channel] {
		return false
	}
	if req.Body == "" {
		return false
	}
	return true
}

// OutageHandler handles HTTP requests for outage operations, delegating to the
// entity service for data access.
type OutageHandler struct {
	entity entityOutageClient
}

// NewOutageHandler creates an OutageHandler backed by the given entity client.
func NewOutageHandler(entity entityOutageClient) *OutageHandler {
	return &OutageHandler{entity: entity}
}

// CreateOutage handles POST /outages.
func (h *OutageHandler) CreateOutage(w http.ResponseWriter, r *http.Request) {
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

	if !validateCreateOutageBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateOutage(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateOutage failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create outage.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchOutages handles POST /outages/search.
func (h *OutageHandler) SearchOutages(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchOutages(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchOutages failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search outages.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetOutage handles GET /outages/{id}.
func (h *OutageHandler) GetOutage(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetOutage(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetOutage failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve outage.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchOutage handles PATCH /outages/{id}.
func (h *OutageHandler) PatchOutage(w http.ResponseWriter, r *http.Request) {
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

	if !validatePatchOutageBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.PatchOutage(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchOutage failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update outage.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AddOutageCommunication handles POST /outages/{id}/communications.
func (h *OutageHandler) AddOutageCommunication(w http.ResponseWriter, r *http.Request) {
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

	if !validateAddOutageCommunicationBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.AddOutageCommunication(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddOutageCommunication failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add outage communication.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchOutageCommunications handles POST /outages/{id}/communications/search.
func (h *OutageHandler) SearchOutageCommunications(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchOutageCommunications(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchOutageCommunications failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search outage communications.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetOutageMetadata handles GET /outages/metadata.
func (h *OutageHandler) GetOutageMetadata(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetOutageMetadata(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetOutageMetadata failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve outage metadata.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
