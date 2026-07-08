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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// stripField removes the named key from a JSON object body, if present.
func stripField(body []byte, field string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	delete(m, field)
	return json.Marshal(m)
}

// entityCaseClient abstracts the entity service operations used by CaseHandler,
// allowing the handler to be tested independently of the real HTTP client.
type entityCaseClient interface {
	CreateCase(ctx context.Context, body []byte) ([]byte, error)
	PatchCase(ctx context.Context, caseID string, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchComments(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseActivities(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	GetCase(ctx context.Context, caseID string) ([]byte, error)
	CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error)
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error)
	DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	CreateCallRequest(ctx context.Context, body []byte) ([]byte, error)
	SearchCallRequests(ctx context.Context, body []byte) ([]byte, error)
	PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error)
	CreateCaseGithubIssue(ctx context.Context, caseID string, body []byte) ([]byte, error)
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

// maxCaseBodyBytes caps case-create bodies at 10 MiB to accommodate rich descriptions.
const maxCaseBodyBytes = 10 << 20

// maxCommentBodyBytes caps comment-create bodies at 10 MiB. Comments can carry
// inline images as base64 data URIs, which inflate raw image size by ~33%, so
// a 1 MiB global cap rejects images well under ServiceNow's own limit.
const maxCommentBodyBytes = 10 << 20

// maxAttachmentBodyBytes caps attachment-create bodies at 15 MiB. The entity
// service enforces a 10 MB decoded file limit; base64 encoding inflates that
// to ~13.3 MB of encoded data plus JSON overhead.
const maxAttachmentBodyBytes = 15 << 20

// safeAttachmentTypes is the allowlist of Content-Type values that may be
// served inline. Anything not in this set is coerced to application/octet-stream
// to prevent a stored-XSS attack via a crafted upstream Content-Type (e.g.
// text/html). All responses also carry Content-Disposition: attachment and
// X-Content-Type-Options: nosniff regardless of type.
var safeAttachmentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"application/pdf":          true,
	"text/plain":               true,
	"application/zip":          true,
	"application/x-zip-compressed": true,
	"application/msword":       true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCaseBodyBytes)
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

	// Strip any client-supplied createdBy to prevent identity spoofing.
	body, err = stripField(body, "createdBy")
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCase(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", user.UserID, "err", err)
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
// createdBy is resolved by the entity service from the forwarded x-user-id-token.
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

	r.Body = http.MaxBytesReader(w, r.Body, maxCommentBodyBytes)
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

	// Work notes are internal-only and exempt from the state gate.
	var reqMeta struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(body, &reqMeta) // body is already validated JSON

	if reqMeta.Type != "work_note" {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during comment guard", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamError(w, err, "Failed to create case comment.")
			return
		}
		var currentCase struct {
			State     string  `json:"state"`
			WorkState *string `json:"workState"`
		}
		if err := json.Unmarshal(current, &currentCase); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse case state for comment guard", "userID", user.UserID, "caseID", caseID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if currentCase.State != "work_in_progress" || currentCase.WorkState == nil || *currentCase.WorkState != "ongoing" {
			writeError(w, http.StatusConflict, ErrMsgCommentNotAllowed)
			return
		}
	}

	// Work notes are blocked on closed cases (separate from the in-progress guard above).
	if reqMeta.Type == "work_note" {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during work-note closed guard", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamError(w, err, "Failed to create case comment.")
			return
		}
		var currentCase struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(current, &currentCase); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse case state for work-note guard", "userID", user.UserID, "caseID", caseID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if currentCase.State == "closed" {
			writeError(w, http.StatusConflict, ErrMsgWorkNoteOnClosedCase)
			return
		}
	}

	result, err := h.entity.CreateCaseComment(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to create case comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchCaseComments handles POST /cases/{id}/comments/search.
// Injects referenceId and referenceType into the payload and forwards to POST /comments/search.
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

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	payload["referenceId"] = caseID
	payload["referenceType"] = "case"

	newBody, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SearchComments(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to search case comments.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchCaseActivities handles POST /cases/{id}/activities/search.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
func (h *CaseHandler) SearchCaseActivities(w http.ResponseWriter, r *http.Request) {
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

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCaseActivities(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseActivities failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to search case activities.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchCases handles POST /cases/search.
// Project IDs and other filters are accepted directly in the request body.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateCaseAttachment handles POST /attachments.
func (h *CaseHandler) CreateCaseAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentBodyBytes)
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

	// Block attachment uploads on closed cases.
	var attachMeta struct {
		ReferenceID   string `json:"referenceId"`
		ReferenceType string `json:"referenceType"`
	}
	_ = json.Unmarshal(body, &attachMeta) // body is already validated JSON
	if attachMeta.ReferenceType == "case" && attachMeta.ReferenceID != "" {
		current, err := h.entity.GetCase(r.Context(), attachMeta.ReferenceID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during attachment closed guard", "userID", user.UserID, "caseID", attachMeta.ReferenceID, "err", err)
			mapUpstreamError(w, err, "Failed to create case attachment.")
			return
		}
		var currentCase struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(current, &currentCase); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse case state for attachment guard", "userID", user.UserID, "caseID", attachMeta.ReferenceID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if currentCase.State == "closed" {
			writeError(w, http.StatusConflict, ErrMsgAttachmentOnClosedCase)
			return
		}
	}

	result, err := h.entity.CreateCaseAttachment(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseAttachment failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to create case attachment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchCaseAttachments handles POST /attachments/search.
func (h *CaseHandler) SearchCaseAttachments(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchCaseAttachments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseAttachments failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search case attachments.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCaseAttachmentContent handles GET /attachments/{id}/content.
func (h *CaseHandler) GetCaseAttachmentContent(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	attachmentID := r.PathValue("id")
	if attachmentID == "" || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	content, contentType, err := h.entity.GetCaseAttachmentContent(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCaseAttachmentContent failed", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve attachment content.")
		return
	}

	// Strip Content-Type parameters (e.g. charset) before the allowlist check.
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !safeAttachmentTypes[ct] {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment")
	_, _ = w.Write(content) // #nosec G705 -- Content-Type is allowlisted above; Content-Disposition: attachment prevents inline rendering
}

// DeleteCaseAttachment handles DELETE /attachments/{id}.
func (h *CaseHandler) DeleteCaseAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	attachmentID := r.PathValue("id")
	if attachmentID == "" || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.DeleteCaseAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteCaseAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		mapUpstreamError(w, err, "Failed to delete case attachment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchCase handles PATCH /cases/{id}.
// Accepts state, severity, workState, watchList, or assigneeEmail and forwards to the entity service.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(r.Header.Get("x-user-id-token")) == "" && !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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

	// Validate state transition and workState guard before forwarding to the entity service.
	var patch struct {
		State     *string `json:"state"`
		WorkState *string `json:"workState"`
	}
	if err := json.Unmarshal(body, &patch); err == nil && (patch.State != nil || patch.WorkState != nil) {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during state validation", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamError(w, err, "Failed to retrieve current case state.")
			return
		}
		var currentCase struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(current, &currentCase); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse current case state", "userID", user.UserID, "caseID", caseID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if patch.State != nil && !isValidStateTransition(currentCase.State, *patch.State) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidTransition)
			return
		}
		if patch.WorkState != nil && currentCase.State != caseStateWorkInProgress {
			writeError(w, http.StatusBadRequest, ErrMsgWorkStateNotAllowed)
			return
		}
	}

	result, err := h.entity.PatchCase(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to update case.")
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
	if strings.TrimSpace(r.Header.Get("x-user-id-token")) == "" && !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
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

// injectCaseIDField merges caseId into a JSON request body as {"caseId": "<id>"}.
func injectCaseIDField(body []byte, caseID string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("request body must be a JSON object")
	}
	idJSON, err := json.Marshal(caseID)
	if err != nil {
		return nil, err
	}
	m["caseId"] = idJSON
	return json.Marshal(m)
}

// CreateCallRequest handles POST /cases/{id}/call-requests.
// Injects the case ID from the URL path into the body as caseId before forwarding
// to the entity service.
func (h *CaseHandler) CreateCallRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
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

	entityBody, err := injectCaseIDField(body, caseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCallRequest(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCallRequest failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to create call request.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchCallRequests handles POST /cases/{id}/call-requests/search.
// Injects the case ID from the URL path into the body as caseId before forwarding
// to the entity service.
func (h *CaseHandler) SearchCallRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
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

	entityBody, err := injectCaseIDField(body, caseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCallRequests(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCallRequests failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to search call requests.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchCallRequest handles PATCH /cases/{id}/call-requests/{callRequestId}.
// Forwards the body unchanged to the entity service's PATCH /call-requests/{callRequestId}.
//
// This is the single mutation surface for call requests, including the agent-only
// (WSO2 engineer) state transitions (schedule/reschedule, reject, conclude+notes)
// selected by the target `state` in the body. The backend has no role-based access
// control layer yet, so any authenticated user may invoke them today; engineer-only
// gating is a follow-up and MUST NOT be invented here.
func (h *CaseHandler) PatchCallRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	callRequestID := r.PathValue("callRequestId")
	if callRequestID == "" || !uuidRe.MatchString(callRequestID) {
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

	entityBody, err := injectCaseIDField(body, caseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.PatchCallRequest(r.Context(), callRequestID, entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCallRequest failed", "userID", user.UserID, "caseID", caseID, "callRequestID", callRequestID, "err", err)
		mapUpstreamError(w, err, "Failed to update call request.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateCaseGithubIssue handles POST /cases/{id}/github-issues.
func (h *CaseHandler) CreateCaseGithubIssue(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
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

	result, err := h.entity.CreateCaseGithubIssue(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseGithubIssue failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to create GitHub issue.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
