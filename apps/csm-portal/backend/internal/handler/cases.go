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
	"regexp"
	"strconv"
	"strings"
	"time"

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

// injectReferenceFields merges referenceId and referenceType into a JSON object body,
// matching the shape the entity service's reference-generic comment/attachment
// endpoints expect. Used by non-case reference types (e.g. change request, incident)
// whose BFF routes are scoped by URL path rather than by request body.
func injectReferenceFields(body []byte, referenceID, referenceType string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	idJSON, err := json.Marshal(referenceID)
	if err != nil {
		return nil, err
	}
	typeJSON, err := json.Marshal(referenceType)
	if err != nil {
		return nil, err
	}
	m["referenceId"] = idJSON
	m["referenceType"] = typeJSON
	return json.Marshal(m)
}

// entityCaseClient abstracts the entity service operations used by CaseHandler,
// allowing the handler to be tested independently of the real HTTP client.
type entityCaseClient interface {
	CreateCase(ctx context.Context, body []byte) ([]byte, error)
	PatchCase(ctx context.Context, caseID string, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchComments(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseEscalations(ctx context.Context, caseID string) ([]byte, error)
	CreateCaseEscalation(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCaseActivities(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	AggregateCases(ctx context.Context, body []byte) ([]byte, error)
	SearchFeedback(ctx context.Context, body []byte) ([]byte, error)
	AggregateFeedback(ctx context.Context, body []byte) ([]byte, error)
	GetCase(ctx context.Context, caseID string) ([]byte, error)
	CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error)
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error)
	DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	GetAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	UpdateAttachment(ctx context.Context, attachmentID string, body []byte) ([]byte, error)
	CreateCallRequest(ctx context.Context, body []byte) ([]byte, error)
	SearchCallRequests(ctx context.Context, body []byte) ([]byte, error)
	SearchAllCallRequests(ctx context.Context, body []byte) ([]byte, error)
	PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error)
	CreateCaseGithubIssue(ctx context.Context, caseID string, body []byte) ([]byte, error)
	AddCaseTag(ctx context.Context, caseID string, body []byte) ([]byte, error)
	RemoveCaseTag(ctx context.Context, caseID, tagID string) ([]byte, error)
	SearchTags(ctx context.Context, body []byte) ([]byte, error)
	// GetUserMe resolves the caller's own platform user record — the same
	// call that backs GET /users/me. Needed by the public-comment ownership
	// guard; see CaseHandler.resolveCurrentUserID.
	GetUserMe(ctx context.Context) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access.
type CaseHandler struct {
	entity entityCaseClient
	// deescalationAllowedRoles is the set of platform roles (lowercased)
	// permitted to de-escalate a case, set via SetDeescalationAllowedRoles.
	// nil/empty means none are configured, which CreateCaseEscalation treats
	// as "de-escalation disabled for everyone" (fail closed) rather than
	// "unrestricted" -- see SetDeescalationAllowedRoles's doc comment.
	deescalationAllowedRoles map[string]bool
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// SetDeescalationAllowedRoles configures the platform roles permitted to
// de-escalate a case (escalating stays open to any authenticated user; only
// de-escalation is role-gated). Role names are matched case-insensitively
// against the caller's own GET /users/me roles.
//
// Deliberately fails closed: an empty or never-called configuration means no
// role passes the check, so de-escalation is refused for everyone rather than
// silently left unrestricted -- the same "fail closed when unconfigured"
// convention this file already uses (see resolveCurrentUserID). A deployment
// that wants de-escalation enabled must configure it explicitly.
func (h *CaseHandler) SetDeescalationAllowedRoles(roles []string) {
	set := make(map[string]bool, len(roles))
	for _, r := range roles {
		r = strings.ToLower(strings.TrimSpace(r))
		if r != "" {
			set[r] = true
		}
	}
	h.deescalationAllowedRoles = set
}

// isDeescalationAction reports whether a case-escalation request body's
// "action" field is DEESCALATE (case-insensitive). A missing/empty action
// defaults to ESCALATE per the entity service's own contract, so only an
// explicit "DEESCALATE"/"deescalate"/etc. value counts.
func isDeescalationAction(body []byte) bool {
	var payload struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return strings.EqualFold(payload.Action, "DEESCALATE")
}

// callerHasDeescalationRole resolves the caller's platform roles via
// GET /users/me and checks them against the configured allow-list. Fails
// closed (returns false) on any lookup/parse error, same convention as
// resolveCurrentUserID.
func (h *CaseHandler) callerHasDeescalationRole(r *http.Request, user *middleware.UserInfo) bool {
	if len(h.deescalationAllowedRoles) == 0 {
		return false
	}
	raw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe failed while checking de-escalation authorization", "userID", user.UserID, "err", err)
		return false
	}
	var me struct {
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe: parse response failed while checking de-escalation authorization", "userID", user.UserID, "err", err)
		return false
	}
	for _, role := range me.Roles {
		if h.deescalationAllowedRoles[strings.ToLower(strings.TrimSpace(role))] {
			return true
		}
	}
	return false
}

// resolveCurrentUserID returns the caller's platform user id — the id
// GET /users/me resolves via the entity service — for comparing against a
// platform record's own user references (e.g. a case's assigned engineer).
//
// This is deliberately NOT user.UserID from the JWT: that claim is whatever
// identity value the gateway/identity provider embeds, an identifier from a
// completely different space than the platform's own user record id. The two
// are never equal, so comparing them always fails. The dashboard
// "__current_user__" placeholder had exactly this bug and was fixed the same
// way, by resolving the caller through GET /users/me instead of trusting the
// raw claim.
//
// Returns an empty id when the lookup fails or yields nothing, so callers
// gating on it fail closed rather than falling back to an id that can never
// match.
func (h *CaseHandler) resolveCurrentUserID(r *http.Request, user *middleware.UserInfo) string {
	raw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe failed while resolving the caller's platform user id", "userID", user.UserID, "err", err)
		return ""
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe: parse response failed while resolving the caller's platform user id", "userID", user.UserID, "err", err)
		return ""
	}
	if me.ID == "" {
		slog.ErrorContext(r.Context(), "entity GetUserMe returned an empty id while resolving the caller's platform user id", "userID", user.UserID)
	}
	return me.ID
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
	"image/png":                    true,
	"image/jpeg":                   true,
	"image/gif":                    true,
	"image/webp":                   true,
	"application/pdf":              true,
	"text/plain":                   true,
	"application/zip":              true,
	"application/x-zip-compressed": true,
	"application/msword":           true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
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
		mapUpstreamErrorGeneric(w, err, "Failed to create case.")
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
			mapUpstreamErrorGeneric(w, err, "Failed to create case comment.")
			return
		}
		var currentCase struct {
			Type             string  `json:"type"`
			State            string  `json:"state"`
			WorkState        *string `json:"workState"`
			AssignedEngineer *struct {
				ID *string `json:"id"`
			} `json:"assignedEngineer"`
		}
		if err := json.Unmarshal(current, &currentCase); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse case state for comment guard", "userID", user.UserID, "caseID", caseID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if currentCase.Type == caseTypeAnnouncement {
			// Announcement cases publish immediately and have no
			// work_in_progress/ongoing workflow, and may carry no assigned
			// engineer at all — the state and ownership gates below don't
			// apply. They still block new comments once closed, like every
			// other case type.
			if currentCase.State == caseStateClosed {
				writeError(w, http.StatusConflict, ErrMsgCommentOnClosedCase)
				return
			}
		} else {
			if currentCase.State != caseStateWorkInProgress || currentCase.WorkState == nil || *currentCase.WorkState != "ongoing" {
				writeError(w, http.StatusConflict, ErrMsgCommentNotAllowed)
				return
			}
			// Ownership check. assignedEngineer.id is a platform user record id, so
			// it can only be compared against the caller's own platform id — never
			// against the identity provider's user id on the JWT. Resolved here,
			// after the state gate, so the extra lookup is only paid on a request
			// that would otherwise be accepted.
			currentUserID := h.resolveCurrentUserID(r, user)
			if currentUserID == "" {
				// The caller's identity could not be established, so ownership
				// cannot be decided either way: fail closed, but as a server-side
				// failure rather than a misleading "you are not the assignee".
				writeError(w, http.StatusInternalServerError, ErrMsgInternal)
				return
			}
			if currentCase.AssignedEngineer == nil || currentCase.AssignedEngineer.ID == nil || *currentCase.AssignedEngineer.ID != currentUserID {
				writeError(w, http.StatusForbidden, ErrMsgCommentNotOwnCase)
				return
			}
		}
	}

	// Work notes are blocked on closed cases (separate from the in-progress guard above).
	if reqMeta.Type == "work_note" {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during work-note closed guard", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to create case comment.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to create case comment.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to search case comments.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to search case activities.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to search cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateCases handles POST /cases/aggregate.
// Server-side aggregation of cases by a single field (e.g. account, state),
// capped to the top maxGroups buckets with the remainder folded into
// othersCount. The groupBy allowlist is validated upstream by the entity
// service; this layer only forwards the request and passes the response
// through as-is.
func (h *CaseHandler) AggregateCases(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.AggregateCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateCases failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchFeedback handles POST /cases/feedback/search.
// Search of case-feedback (satisfaction rating) records across cases,
// filterable by case, accounts, and submission date range. Backs the
// case-feedback dashboard's list view. This is a plain forward-and-return
// proxy: filters/pagination validation is the entity service's job, this
// layer only enforces auth, a body size cap, and valid JSON.
func (h *CaseHandler) SearchFeedback(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchFeedback(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchFeedback failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search case feedback.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateFeedback handles POST /cases/feedback/aggregate.
// Date-bucketed rating aggregation of case-feedback records across cases.
// Backs the case-feedback dashboard's rating-trend chart. Same
// forward-and-return proxy contract as SearchFeedback: the bucket enum and
// filters are validated upstream by the entity service.
func (h *CaseHandler) AggregateFeedback(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.AggregateFeedback(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateFeedback failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate case feedback.")
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
			mapUpstreamErrorGeneric(w, err, "Failed to create case attachment.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to create case attachment.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to search case attachments.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment content.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to delete case attachment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetAttachment handles GET /attachments/{id}.
// Returns the attachment's metadata as JSON; for the binary file contents, see
// GetCaseAttachmentContent (GET /attachments/{id}/content).
func (h *CaseHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// validAttachmentReferenceTypes mirrors the entity service's own
// validReferenceTypes allowlist for attachment operations (sn_case_service.go),
// so an obviously invalid referenceType is rejected at this boundary instead
// of reaching the entity service.
var validAttachmentReferenceTypes = map[string]bool{
	"case":           true,
	"conversation":   true,
	"change_request": true,
	"deployment":     true,
	"incident":       true,
}

// updateAttachmentRequest mirrors the entity service's domain.UpdateAttachmentRequest
// shape. Description uses json.RawMessage (rather than *string) so an explicit
// JSON null (clear the description) can be distinguished from an absent field,
// matching the entity service's own tri-state semantics for this field.
type updateAttachmentRequest struct {
	ReferenceID   string          `json:"referenceId"`
	ReferenceType string          `json:"referenceType"`
	Name          *string         `json:"name,omitempty"`
	Description   json.RawMessage `json:"description,omitempty"`
}

// validateUpdateAttachmentBody rejects a non-object body (including null and
// arrays), a missing/invalid referenceId, an invalid referenceType, and a body
// where neither name nor description is present, so obviously invalid requests
// are rejected before reaching the entity service.
func validateUpdateAttachmentBody(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	if len(fields) == 0 {
		return false
	}

	var req updateAttachmentRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return false
	}
	if req.ReferenceID == "" || !uuidRe.MatchString(req.ReferenceID) {
		return false
	}
	if !validAttachmentReferenceTypes[req.ReferenceType] {
		return false
	}
	if req.Name == nil && len(req.Description) == 0 {
		return false
	}
	return true
}

// UpdateAttachment handles PATCH /attachments/{id}.
// Accepts referenceId, referenceType, and optionally name/description; the body
// is forwarded verbatim once validated.
func (h *CaseHandler) UpdateAttachment(w http.ResponseWriter, r *http.Request) {
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

	if !validateUpdateAttachmentBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.UpdateAttachment(r.Context(), attachmentID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to update attachment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AddCaseTag handles POST /cases/{id}/tags.
// Raw pass-through — free-text tag creation is validated at the entity layer.
func (h *CaseHandler) AddCaseTag(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.AddCaseTag(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCaseTag failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add case tag.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// RemoveCaseTag handles DELETE /cases/{id}/tags/{tagId}.
// The entity service returns 204 No Content on success; forwarded as-is with no body.
func (h *CaseHandler) RemoveCaseTag(w http.ResponseWriter, r *http.Request) {
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

	tagID := r.PathValue("tagId")
	if tagID == "" || !uuidRe.MatchString(tagID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if _, err := h.entity.RemoveCaseTag(r.Context(), caseID, tagID); err != nil {
		slog.ErrorContext(r.Context(), "entity RemoveCaseTag failed", "userID", user.UserID, "caseID", caseID, "tagID", tagID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to remove case tag.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SearchTags handles POST /tags/search.
// The JSON body ({filters:{searchQuery}, limit}) is forwarded to the entity
// service verbatim, and the raw response returned as-is. Limit bounds are
// enforced upstream, so there is nothing to re-validate here.
func (h *CaseHandler) SearchTags(w http.ResponseWriter, r *http.Request) {
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

	h.forwardTagSearch(w, r, user.UserID, body)
}

// tagSearchRequest is the request body of POST /tags/search. It exists only so
// the deprecated GET alias can build the exact same bytes the POST forwards;
// the POST itself never decodes the body, it passes it through untouched.
type tagSearchRequest struct {
	Filters struct {
		SearchQuery string `json:"searchQuery"`
	} `json:"filters"`
	Limit int `json:"limit"`
}

// SearchTagsQuery handles GET /tags/search?q={query}&limit={limit}, the
// query-parameter form tag search used before it moved to a JSON body. The
// parameters are translated into the POST body and forwarded through the same
// client call, so only one request shape ever leaves this service.
//
// Deprecated: use POST /tags/search instead. This alias exists only to bridge
// one release, so callers still on the GET do not get a 405 while this service
// and its callers are deployed separately. Delete it once that release has
// shipped.
func (h *CaseHandler) SearchTagsQuery(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	var req tagSearchRequest
	req.Filters.SearchQuery = r.URL.Query().Get("q")
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		// Bounds are enforced upstream, as they are for the POST; a value that
		// is merely out of range is forwarded and rejected there.
		req.Limit = parsed
	}

	body, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	h.forwardTagSearch(w, r, user.UserID, body)
}

// forwardTagSearch is the single outbound path shared by POST /tags/search and
// its deprecated GET alias.
func (h *CaseHandler) forwardTagSearch(w http.ResponseWriter, r *http.Request, userID string, body []byte) {
	result, err := h.entity.SearchTags(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTags failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search tags.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchCase handles PATCH /cases/{id}.
// Accepts state, severity, workState, watchList, assigneeEmail, or acknowledge and forwards
// to the entity service. The body is forwarded verbatim, so fields with no local guard (like
// acknowledge, whose first-write-wins semantics and role gate both live upstream) need no
// handling here — only state and workState are pre-validated, because their guards depend on
// the case's current state.
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
		State              *string `json:"state"`
		WorkState          *string `json:"workState"`
		AutocloseHoldUntil *string `json:"autocloseHoldUntil"`
	}
	patchErr := json.Unmarshal(body, &patch)
	if patchErr == nil && (patch.State != nil || patch.WorkState != nil) {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during state validation", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to retrieve current case state.")
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

	// The auto-closure hold work note (below) must only fire for an actual change in
	// the hold date, not for a retry or a no-op PATCH that resends the existing value —
	// otherwise every duplicate request pollutes the case's activity feed with an
	// identical note. Read the prior value before the PATCH; after it, the case
	// already reflects the new value and there'd be nothing to diff against.
	var priorHoldDate string
	if patchErr == nil && patch.AutocloseHoldUntil != nil {
		if current, err := h.entity.GetCase(r.Context(), caseID); err != nil {
			slog.WarnContext(r.Context(), "entity GetCase failed reading prior autoclose hold date; proceeding without dedup", "userID", user.UserID, "caseID", caseID, "err", err)
		} else {
			// autoclosureStateTime is only "the hold date" while the case is actually
			// ON_HOLD — for every other autoclosureStep it's when that other stage next
			// advances, a value unrelated to any hold. Without gating on the step, the
			// very first hold on a case (whose autoclosureStateTime already holds some
			// unrelated staged-advance date matching the FE's pre-filled picker default)
			// gets misread as "unchanged" and its note silently skipped.
			var currentCase struct {
				AutoclosureStep      *string `json:"autoclosureStep"`
				AutoclosureStateTime *string `json:"autoclosureStateTime"`
			}
			if err := json.Unmarshal(current, &currentCase); err == nil &&
				currentCase.AutoclosureStep != nil && *currentCase.AutoclosureStep == "ON_HOLD" &&
				currentCase.AutoclosureStateTime != nil {
				priorHoldDate = formatHoldDate(*currentCase.AutoclosureStateTime)
			}
		}
	}

	result, err := h.entity.PatchCase(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	// Setting/extending the auto-closure hold has no visible trail of its own on the
	// case (unlike the legacy ticketing UI's equivalent action, which records a work
	// note). Record one here so CS engineers can see when a hold was set/extended and
	// until when. Best-effort and fire-and-forget: the hold PATCH above already
	// succeeded, so this secondary write must not delay the response or fail/roll back
	// the request if it errors. context.WithoutCancel keeps the request-scoped values
	// the entity client needs (x-user-id-token, correlation id) while detaching from
	// the request's own cancellation, which fires as soon as the handler returns —
	// a bare context.Background() would drop those values and the note would reach
	// the entity service unattributed.
	if patchErr == nil && patch.AutocloseHoldUntil != nil && formatHoldDate(*patch.AutocloseHoldUntil) != priorHoldDate {
		holdUntil := *patch.AutocloseHoldUntil
		detached := context.WithoutCancel(r.Context())
		go func() {
			ctx, cancel := context.WithTimeout(detached, 15*time.Second)
			defer cancel()
			h.recordAutocloseHoldWorkNote(ctx, user, caseID, holdUntil)
		}()
	}

	writeJSON(w, http.StatusOK, result)
}

// formatHoldDate renders an auto-closure hold timestamp (RFC3339, as sent by the
// FE or read back from the entity service) as the date-only form CS engineers see
// in the UI and in the work note, since the hold is date-granularity. Falls back
// to the raw input when it doesn't parse, so an already-invalid value is not
// silently dropped from the comparison/note.
func formatHoldDate(raw string) string {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02")
	}
	return raw
}

// recordAutocloseHoldWorkNote adds an internal work note documenting an
// auto-closure hold set/extension, mirroring the work note the legacy
// ticketing UI's equivalent action used to write. Best-effort: failures are
// logged, never surfaced to the caller, since the primary hold PATCH already
// succeeded by the time this runs.
func (h *CaseHandler) recordAutocloseHoldWorkNote(ctx context.Context, user *middleware.UserInfo, caseID, holdUntil string) {
	note := "Please note that this case is on-hold until " + formatHoldDate(holdUntil) +
		", hence it will not go through the auto closure process. It will be eligible " +
		"for auto-closure again after this date passes, or if the case state is changed " +
		"to 'Waiting on WSO2'."

	body, err := json.Marshal(map[string]string{
		"type":    "work_note",
		"content": note,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build autoclose hold work note body", "userID", user.UserID, "caseID", caseID, "err", err)
		return
	}

	if _, err := h.entity.CreateCaseComment(ctx, caseID, body); err != nil {
		slog.WarnContext(ctx, "failed to record autoclose hold work note", "userID", user.UserID, "caseID", caseID, "err", err)
	}
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
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case details.")
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

// GetCaseEscalations handles GET /cases/{id}/escalations.
func (h *CaseHandler) GetCaseEscalations(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchCaseEscalations(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseEscalations failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case escalation history.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateCaseEscalation handles POST /cases/{id}/escalations.
func (h *CaseHandler) CreateCaseEscalation(w http.ResponseWriter, r *http.Request) {
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

	if isDeescalationAction(body) && !h.callerHasDeescalationRole(r, user) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	result, err := h.entity.CreateCaseEscalation(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseEscalation failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create case escalation.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
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
		mapUpstreamErrorGeneric(w, err, "Failed to create call request.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to search call requests.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchAllCallRequests handles POST /call-requests/search — standalone call
// request search across all cases (not scoped to one case; see SearchCallRequests
// for that path, which is nested under /cases/{id}/). Raw pass-through
// body/response. Despite the shared "search" name with the case-scoped path,
// this is a distinct route (flat, no case-id path param) with no collision --
// forwards to the entity service's own /call-requests/search-all, which keeps
// its "-all" suffix to stay distinct from ITS sibling case-scoped path.
func (h *CaseHandler) SearchAllCallRequests(w http.ResponseWriter, r *http.Request) {
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

	if !isJSONObjectOrEmpty(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchAllCallRequests(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAllCallRequests failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search call requests.")
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
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
