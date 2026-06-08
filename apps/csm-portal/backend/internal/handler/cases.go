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
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var errUserNotFound = errors.New("authenticated user not found in entity service")

// resolveUserID looks up the entity-service UUID for the given email via users/search.
// TODO: remove once DB-level auth propagates the entity UUID directly through the token.
func resolveUserID(ctx context.Context, entity entityCaseClient, email string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"searchQuery": email,
		"pagination":  map[string]any{"limit": 1, "offset": 0},
	})
	if err != nil {
		return "", err
	}
	resp, err := entity.SearchUsers(ctx, body)
	if err != nil {
		return "", err
	}
	var result struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	// Verify exact email match — SearchUsers may perform fuzzy/prefix search.
	if len(result.Users) == 0 || result.Users[0].Email != email {
		return "", errUserNotFound
	}
	return result.Users[0].ID, nil
}

// injectCreatedBy merges createdBy into a JSON request body.
func injectCreatedBy(body []byte, userID string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	id, err := json.Marshal(userID)
	if err != nil {
		return nil, err
	}
	m["createdBy"] = id
	return json.Marshal(m)
}

// injectProjectID merges a project ID into a JSON request body as projectIds: [id].
func injectProjectID(body []byte, projectID string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	ids, err := json.Marshal([]string{projectID})
	if err != nil {
		return nil, err
	}
	m["projectIds"] = ids
	return json.Marshal(m)
}

// entityCaseClient abstracts the entity service operations used by CaseHandler,
// allowing the handler to be tested independently of the real HTTP client.
type entityCaseClient interface {
	CreateCase(ctx context.Context, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	GetCase(ctx context.Context, caseID string) ([]byte, error)
	SearchUsers(ctx context.Context, body []byte) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access.
type CaseHandler struct {
	entity entityCaseClient
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// CreateCase handles POST /cases.
// createdBy is resolved server-side by looking up the authenticated user's email
// in the entity service and injecting the UUID into the request body.
// TODO: remove the users/search lookup once DB-level auth propagates the entity
// UUID directly through the token claims.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
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

	userID, err := resolveUserID(r.Context(), h.entity, user.Email)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			// Token is valid but the entity service has no record for this user — 403, not 401.
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
		slog.ErrorContext(r.Context(), "resolveUserID failed", "email", user.Email, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	entityBody, err := injectCreatedBy(body, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCase(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", userID, "err", err)
		mapUpstreamError(w, err, "Failed to create case.")
		return
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &created); err == nil && created.ID != "" {
		w.Header().Set("Location", "/cases/"+created.ID)
	}
	writeJSON(w, http.StatusCreated, result)
}

// CreateCaseComment handles POST /cases/{id}/comments.
// createdBy is resolved server-side from the authenticated user's email.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
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

	userID, err := resolveUserID(r.Context(), h.entity, user.Email)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
		slog.ErrorContext(r.Context(), "resolveUserID failed", "email", user.Email, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	entityBody, err := injectCreatedBy(body, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCaseComment(r.Context(), caseID, entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "userID", userID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to create case comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchCaseComments handles POST /cases/{id}/comments/search.
func (h *CaseHandler) SearchCaseComments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
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

	result, err := h.entity.SearchCaseComments(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseComments failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to search case comments.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchProjectCases handles POST /projects/{id}/cases/search.
// It enforces the project scope by injecting projectIds: [id] into the entity request.
func (h *CaseHandler) SearchProjectCases(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
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

	entityBody, err := injectProjectID(body, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "projectID", projectID, "err", err)
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, "Case ID cannot be empty!")
		return
	}
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve case details.")
		return
	}

	result, err = injectNextStates(result)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to inject nextStates", "userID", user.UserID, "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, "Failed to process case details.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
