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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// caseIDRe is the id shape GET and PATCH /cases/{id} accept: a UUID, the
// 32-hex record id the legacy data source uses for a case, or a case number
// (2-5 letters then 4-12 digits). It is checked whatever headers the request
// carries.
var caseIDRe = regexp.MustCompile(`^(?:(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|(?i)[0-9a-f]{32}|[A-Za-z]{2,5}[0-9]{4,12})$`)

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
	// GetProductRepoMapping calls GET /products/github-repo?name= on the
	// entity service. The response is the GitHub repository for that product.
	GetProductRepoMapping(ctx context.Context, name string) ([]byte, error)
	CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error)
	SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error)
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error)
	DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	// GetCaseAttachment resolves a single attachment's metadata — used by the
	// SFTPGo-backed share-creation path; see AttachmentStorageHandler.
	GetCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	// ConfirmCaseAttachment transitions a 'pending' attachment row (created by
	// CreateCaseAttachment with status "pending") to 'complete' — used by the
	// SFTPGo-backed upload-confirm path; see AttachmentStorageHandler.
	ConfirmCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error)
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
	// inlineImages enables server-side inline-image extraction on
	// CreateCaseComment when non-nil — see WithInlineImageProcessor. nil on
	// every existing call site (including every test), which keeps
	// CreateCaseComment's behavior completely unchanged from before this
	// feature existed.
	inlineImages *InlineImageProcessor
	// engineering, when non-nil, files GitHub issues from a case instead of
	// the entity service — see WithEngineeringClient.
	engineering engineeringGitIssueClient
	// access backs the security-report type check in SearchCases -- see
	// WithAccessGuard. nil fails that check closed (denied), never open:
	// unlike UsersHandler's own optional use of this field (a display-only
	// enrichment, harmless if skipped), this one gates real access to data.
	access *AccessGuard
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// WithAccessGuard wires the same guard that authorises every route into this
// handler, so the case routes can additionally enforce PermViewSecurityCenter
// — a restriction PermView alone (the route-level permission they carry,
// shared with every other case-type view) cannot express. Returns h for
// chaining at the construction site.
//
// For a caller without that permission: SearchCases and AggregateCases scope
// the request to the non-security case types (scopeCaseSearchBody), GetCase
// refuses a security-report case with 403 once loaded
// (caseViewIsSecurityReport), and the per-case sub-resource reads load the
// case first to apply the same check (requireCaseVisibleToCaller). A handler
// constructed without a guard fails closed.
func (h *CaseHandler) WithAccessGuard(g *AccessGuard) *CaseHandler {
	h.access = g
	return h
}

// WithInlineImageProcessor enables server-side inline-image extraction on
// CreateCaseComment: a base64 data: URI embedded in a comment's rich-text
// HTML is extracted, uploaded as a real SFTPGo-backed attachment, and the
// HTML is rewritten to a ".iix" reference — mirroring the backing system's own
// RichTextUtils.processInlineImages for SN-backed comments. Only wired up in
// cmd/server/main.go when SFTPGO_ATTACHMENT_STORAGE_ENABLED is on; SN-backed
// comment creation is untouched either way, since SN's own scripted API
// already performs the equivalent extraction itself. Returns h for chaining
// at the construction site.
func (h *CaseHandler) WithInlineImageProcessor(p *InlineImageProcessor) *CaseHandler {
	h.inlineImages = p
	return h
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
	return resolvePlatformUserID(r.Context(), h.entity.GetUserMe, user)
}

// resolvePlatformUserID is resolveCurrentUserID's shared body: the caller's
// platform user id from getMe (the entity service's GET /users/me), or "" when
// the lookup fails or yields no id.
func resolvePlatformUserID(ctx context.Context, getMe func(context.Context) ([]byte, error), user *middleware.UserInfo) string {
	raw, err := getMe(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "entity GetUserMe failed while resolving the caller's platform user id", "userID", user.UserID, "err", summarizeErr(err))
		return ""
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		slog.ErrorContext(ctx, "entity GetUserMe: parse response failed while resolving the caller's platform user id", "userID", user.UserID, "err", summarizeErr(err))
		return ""
	}
	if me.ID == "" {
		slog.ErrorContext(ctx, "entity GetUserMe returned an empty id while resolving the caller's platform user id", "userID", user.UserID)
	}
	return me.ID
}

// validateCaseEscalationBody decodes and validates a case-escalation request
// body against the entity service's CaseEscalationCreateRequest contract:
// unknown fields are rejected, "action" (if present) must be exactly one of
// the four literal forms the schema enumerates -- "ESCALATE", "escalate",
// "DEESCALATE", "deescalate" -- (missing key defaults to ESCALATE; an
// explicit "action": null is rejected, since the contract only permits an
// omitted key or a string value, never a null), and "reason" is required and
// non-blank unless the effective action is DEESCALATE. Returns the
// normalized, upper-case effective action and whether the body is valid.
//
// "action" is decoded as json.RawMessage rather than *string because a *string
// cannot distinguish an omitted key from an explicit "action": null -- both
// decode to a nil pointer. json.RawMessage stays nil only when the key is
// absent; when the key is present its raw bytes are captured verbatim
// (including the literal `null`), so the two cases can be told apart.
// "reason" does not need the same treatment: {"reason": null, ...} is already
// correctly rejected by the existing nil-check below (a null reason yields no
// usable reason, same as an absent one), so *string is sufficient there.
func validateCaseEscalationBody(body []byte) (action string, ok bool) {
	var req struct {
		Reason *string         `json:"reason"`
		Action json.RawMessage `json:"action"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return "", false
	}
	// Reject trailing data after the first JSON value (e.g. two concatenated
	// JSON objects) -- json.Decoder.Decode only consumes the first value and
	// silently leaves the rest unread.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return "", false
	}

	action = "ESCALATE"
	if req.Action != nil {
		var actionStr string
		if err := json.Unmarshal(req.Action, &actionStr); err != nil {
			// Covers both an explicit "action": null and any non-string value.
			return "", false
		}
		switch actionStr {
		case "ESCALATE", "escalate":
			action = "ESCALATE"
		case "DEESCALATE", "deescalate":
			action = "DEESCALATE"
		default:
			return "", false
		}
	}

	if action != "DEESCALATE" && (req.Reason == nil || strings.TrimSpace(*req.Reason) == "") {
		return "", false
	}

	return action, true
}

// callerIsNotifiedOnCurrentEscalation reports whether the caller is one of the
// people notified about the case's current (most recent) escalation level --
// the only people authorized to de-escalate it. Escalating stays open to any
// authenticated user; only de-escalation is gated this way.
//
// Fails closed (returns false) on any lookup/parse error or when the case has
// no escalation history at all (nothing to de-escalate, nobody was notified).
// Matches by the caller's platform user id first (GET /users/me's own id
// against a notified user's id, both platform UUIDs), falling back to a
// case-insensitive email match when either id is empty -- the notified-user
// id can be empty when the backing data source could not resolve a platform
// record for that recipient.
func (h *CaseHandler) callerIsNotifiedOnCurrentEscalation(r *http.Request, caseID string, user *middleware.UserInfo) bool {
	raw, err := h.entity.SearchCaseEscalations(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseEscalations failed while checking de-escalation authorization", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		return false
	}
	var history struct {
		CurrentNotifiedUsers []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"currentNotifiedUsers"`
	}
	if err := json.Unmarshal(raw, &history); err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseEscalations: parse response failed while checking de-escalation authorization", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		return false
	}
	if len(history.CurrentNotifiedUsers) == 0 {
		return false
	}

	callerRaw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe failed while checking de-escalation authorization", "userID", user.UserID, "err", summarizeErr(err))
		return false
	}
	var caller struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(callerRaw, &caller); err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe: parse response failed while checking de-escalation authorization", "userID", user.UserID, "err", summarizeErr(err))
		return false
	}

	for _, notified := range history.CurrentNotifiedUsers {
		if caller.ID != "" && notified.ID != "" && caller.ID == notified.ID {
			return true
		}
		// Only fall back to email when an id is unavailable on either side --
		// two different platform users must never be treated as the same
		// person just because both ids happen to be missing and their emails
		// happen to match by coincidence or staleness on one side.
		if (caller.ID == "" || notified.ID == "") &&
			caller.Email != "" && notified.Email != "" &&
			strings.EqualFold(caller.Email, notified.Email) {
			return true
		}
	}
	return false
}

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// maxCaseBodyBytes caps case-create bodies at 10 MiB to accommodate rich descriptions.
const maxCaseBodyBytes = 10 << 20

// maxCommentBodyBytes caps comment-create bodies at 10 MiB. Comments can carry
// inline images as base64 data URIs, which inflate raw image size by ~33%, so
// a 1 MiB global cap rejects images well under the backing system's own limit.
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
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", user.UserID, "err", summarizeErr(err))
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
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(body, &reqMeta) // body is already validated JSON

	// The route's own permission (PermCreateWorkNote) is deliberately
	// broader than this: it also admits a worknote_creator-only caller, who
	// must NOT be able to post anything but a work_note. Narrow back down
	// to full PermWrite for every other type -- see PermCreateWorkNote's
	// own doc comment.
	hasFullWrite := h.access != nil && h.access.Permits(PermWrite, user.Roles)
	if reqMeta.Type != "work_note" && !hasFullWrite {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	// A worknote-creator-only caller is only ever allowed to reach here with
	// type=work_note (just checked above) -- but body is still the
	// caller-supplied raw bytes, forwarded to the entity service unchanged
	// below. encoding/json's handling of a duplicate "type" key (last one
	// wins) is an implementation detail, not a wire-format guarantee the
	// entity service is bound by; if it parses the same bytes differently,
	// a body like {"type":"comment","type":"work_note"} could pass this
	// check yet be stored as a customer-visible comment. Rebuild the body
	// from what THIS check actually approved rather than forwarding the
	// ambiguous original, so there is no decoder for the two services to
	// disagree on. Full-PermWrite callers are unaffected: they may post any
	// type, so there is nothing narrower here to enforce for them.
	if !hasFullWrite {
		rebuilt, err := json.Marshal(struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}{Type: "work_note", Content: reqMeta.Content})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to rebuild work-note comment body", "userID", user.UserID, "caseID", caseID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		body = rebuilt
	}

	if reqMeta.Type != "work_note" {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during comment guard", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
			slog.ErrorContext(r.Context(), "entity GetCase failed during work-note closed guard", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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

	// Extract any base64 inline image embedded in the comment's rich-text
	// HTML into a real SFTPGo-backed attachment before forwarding to the
	// entity service — mirrors the backing system's own RichTextUtils processing for
	// SN-backed comments (that path is untouched: it already runs inside the
	// SN scripted API, not here). Only active when
	// SFTPGO_ATTACHMENT_STORAGE_ENABLED is on; see WithInlineImageProcessor.
	if h.inlineImages != nil {
		newBody, ierr := h.processCommentInlineImages(r, user, caseID, body)
		if ierr != nil {
			ierr.write(w)
			return
		}
		body = newBody
	}

	result, err := h.entity.CreateCaseComment(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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

	// injectReferenceFields is nil-safe: a JSON `null` body decodes to a nil
	// map, which a direct key assignment would panic on.
	newBody, err := injectReferenceFields(body, caseID, "case")
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.requireCaseVisibleToCaller(w, r, user, caseID, "Failed to search case comments.") {
		return
	}

	result, err := h.entity.SearchComments(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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

	if !h.requireCaseVisibleToCaller(w, r, user, caseID, "Failed to search case activities.") {
		return
	}

	result, err := h.entity.SearchCaseActivities(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseActivities failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search case activities.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// securityReportCaseType is the one case type value Security Center is
// restricted to — see scopeCaseSearchBody's own doc comment.
const securityReportCaseType = "security_report_analysis"

// caseFieldFilterFragment is the subset of CaseFieldFilter (entity-service's
// openapi.yaml) this handler needs to read out of an otherwise-opaque,
// forwarded-verbatim request body: which field a predicate names and what
// values it matches. Untyped fields (op, and every other CaseFieldFilter
// property) are simply not decoded.
type caseFieldFilterFragment struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// nonSecurityCaseTypes is every case type the entity service recognises
// (its validCaseType set) except securityReportCaseType. It is the allow-list
// scopeCaseSearchBody injects for a caller without PermViewSecurityCenter:
// the entity service accepts only `op: in` on the type field, so exclusion
// has to be expressed as "every other type". Keep it in step with the entity
// service's own set — a type missing here is invisible to those callers.
var nonSecurityCaseTypes = []string{"case", "service_request", "announcement", "engagement"}

// errSecurityReportsRestricted is returned by scopeCaseSearchBody when the
// request explicitly names securityReportCaseType.
var errSecurityReportsRestricted = errors.New("search names the restricted case type")

// canViewSecurityCenter reports whether user holds PermViewSecurityCenter. A
// handler constructed without WithAccessGuard fails closed, the same way the
// inline-image redaction does.
func (h *CaseHandler) canViewSecurityCenter(user *middleware.UserInfo) bool {
	return h.access != nil && h.access.Permits(PermViewSecurityCenter, user.Roles)
}

// scopeCaseSearchBody rewrites a POST /cases/search or /cases/aggregate body
// for a caller who may not see security-report cases. The generic filter
// grammar (entity-service's CaseFieldFilter) is the only way a request can
// name a case type: the top-level filters.filters array and each
// filters.anyOf branch. The rules are:
//
//   - any type predicate (top level or in a branch) whose values include
//     securityReportCaseType → errSecurityReportsRestricted (the caller gets
//     403, matching the behaviour before this scoping existed);
//   - a top-level type predicate naming other types only → body unchanged,
//     the caller's own narrower filter already excludes the restricted type;
//   - no top-level type predicate → a `type in nonSecurityCaseTypes` predicate
//     is appended to filters.filters. It is ANDed with every other predicate,
//     including anyOf branches, so a branch-level type filter is still
//     narrowed by it.
//
// Everything else in the body is carried through as json.RawMessage, so no
// other field is reshaped or re-encoded. A body whose filters/filters/anyOf
// members are not the documented shapes is reported as an error (→ 400); the
// entity service would reject it too, this just does so without forwarding.
func scopeCaseSearchBody(body []byte) ([]byte, error) {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	if req == nil {
		req = map[string]json.RawMessage{}
	}
	var filters map[string]json.RawMessage
	if raw, ok := req["filters"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &filters); err != nil {
			return nil, err
		}
	}
	if filters == nil {
		filters = map[string]json.RawMessage{}
	}
	var top []json.RawMessage
	if raw, ok := filters["filters"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &top); err != nil {
			return nil, err
		}
	}
	hasTopLevelTypeFilter := false
	for _, raw := range top {
		var f caseFieldFilterFragment
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		if f.Field != "type" {
			continue
		}
		hasTopLevelTypeFilter = true
		if slices.Contains(f.Values, securityReportCaseType) {
			return nil, errSecurityReportsRestricted
		}
	}
	var branches []struct {
		Filters []caseFieldFilterFragment `json:"filters"`
	}
	if raw, ok := filters["anyOf"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &branches); err != nil {
			return nil, err
		}
	}
	for _, b := range branches {
		for _, f := range b.Filters {
			if f.Field == "type" && slices.Contains(f.Values, securityReportCaseType) {
				return nil, errSecurityReportsRestricted
			}
		}
	}
	if hasTopLevelTypeFilter {
		return body, nil
	}
	injected, err := json.Marshal(map[string]any{"field": "type", "op": "in", "values": nonSecurityCaseTypes})
	if err != nil {
		return nil, err
	}
	topJSON, err := json.Marshal(append(top, injected))
	if err != nil {
		return nil, err
	}
	filters["filters"] = topJSON
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, err
	}
	req["filters"] = filtersJSON
	return json.Marshal(req)
}

// writeScopedCaseSearchError maps a scopeCaseSearchBody failure to a response.
func writeScopedCaseSearchError(w http.ResponseWriter, err error) {
	if errors.Is(err, errSecurityReportsRestricted) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}
	writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
}

// caseViewIsSecurityReport reports whether a case response body's type is
// securityReportCaseType. Best-effort: a body without a readable string type
// is not a security report.
func caseViewIsSecurityReport(caseJSON []byte) bool {
	var cv struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(caseJSON, &cv); err != nil {
		return false
	}
	return cv.Type == securityReportCaseType
}

// requireCaseVisibleToCaller enforces the Security Center restriction on a
// per-case sub-resource route (comments, activities, escalations): for a
// caller without PermViewSecurityCenter it loads the case and refuses with
// 403 when it is a security report. A holder costs no upstream call. A load
// failure is mapped like any other upstream error (so an unknown id is still
// 404), using fallbackMsg. Returns false when a response has been written.
func (h *CaseHandler) requireCaseVisibleToCaller(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, caseID, fallbackMsg string) bool {
	if h.canViewSecurityCenter(user) {
		return true
	}
	caseJSON, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, fallbackMsg)
		return false
	}
	if caseViewIsSecurityReport(caseJSON) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return false
	}
	return true
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

	if !h.canViewSecurityCenter(user) {
		if body, err = scopeCaseSearchBody(body); err != nil {
			writeScopedCaseSearchError(w, err)
			return
		}
	}

	result, err := h.entity.SearchCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "err", summarizeErr(err))
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

	// Same scoping as SearchCases: the aggregate takes the same filters
	// object, and a groupBy of "type" would otherwise count the restricted
	// type for any PermView caller.
	if !h.canViewSecurityCenter(user) {
		if body, err = scopeCaseSearchBody(body); err != nil {
			writeScopedCaseSearchError(w, err)
			return
		}
	}

	result, err := h.entity.AggregateCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateCases failed", "userID", user.UserID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity SearchFeedback failed", "userID", user.UserID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity AggregateFeedback failed", "userID", user.UserID, "err", summarizeErr(err))
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
			slog.ErrorContext(r.Context(), "entity GetCase failed during attachment closed guard", "userID", user.UserID, "caseID", attachMeta.ReferenceID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity CreateCaseAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity SearchCaseAttachments failed", "userID", user.UserID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity GetCaseAttachmentContent failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity DeleteCaseAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity UpdateAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity AddCaseTag failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity RemoveCaseTag failed", "userID", user.UserID, "caseID", caseID, "tagID", tagID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity SearchTags failed", "userID", userID, "err", summarizeErr(err))
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
	if !caseIDRe.MatchString(caseID) {
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
		BestCaseFixEta     *string `json:"bestCaseFixEta"`
		MostLikelyFixEta   *string `json:"mostLikelyFixEta"`
		WorstCaseFixEta    *string `json:"worstCaseFixEta"`
	}
	patchErr := json.Unmarshal(body, &patch)
	if patchErr == nil && (patch.State != nil || patch.WorkState != nil) {
		current, err := h.entity.GetCase(r.Context(), caseID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity GetCase failed during state validation", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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

	result, err := h.entity.PatchCase(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	// Setting/extending the auto-closure hold has no visible trail of its own on the
	// case (unlike the legacy ticketing UI's equivalent action, which records a work
	// note). Record one here so CS engineers can see when a hold was set/extended and
	// until when — every PATCH that carries autocloseHoldUntil gets one, with no
	// no-op/dedup check: the field this would need to key off
	// (autoclosureStep/autoclosureStateTime) isn't reliably populated on a case read,
	// and the legacy ticketing UI's own equivalent action has the exact same
	// behavior (it re-posts an identical note on every resend too), so this matches
	// established behavior rather than deviating from it. Best-effort and
	// fire-and-forget: the hold PATCH above already succeeded, so this secondary
	// write must not delay the response or fail/roll back the request if it errors.
	// context.WithoutCancel keeps the request-scoped values the entity client needs
	// (x-user-id-token, correlation id) while detaching from the request's own
	// cancellation, which fires as soon as the handler returns — a bare
	// context.Background() would drop those values and the note would reach the
	// entity service unattributed.
	if patchErr == nil && patch.AutocloseHoldUntil != nil {
		holdUntil := *patch.AutocloseHoldUntil
		detached := context.WithoutCancel(r.Context())
		go func() {
			ctx, cancel := context.WithTimeout(detached, 15*time.Second)
			defer cancel()
			h.recordAutocloseHoldWorkNote(ctx, user, caseID, holdUntil)
		}()
	}

	// A fix-ETA update has no trail of its own on the case unless the caller also
	// sets addPublicComment, which posts a separate customer-visible comment
	// entirely inside the entity service. Record an internal work note here too,
	// independent of that flag, so CS engineers can see from the case's own
	// history that a fix-ETA change happened at all. Sourced from the PATCH
	// response (the values the entity service actually committed), not the
	// request, since that's the correct source of truth regardless of backing
	// data source. Best-effort and fire-and-forget, same reasoning as the
	// autoclose-hold note above: the PATCH already succeeded, so this secondary
	// write must not delay the response or fail the request if it errors, and
	// context.WithoutCancel keeps the request-scoped values the entity client
	// needs while detaching from the request's own cancellation.
	if patchErr == nil && (patch.BestCaseFixEta != nil || patch.MostLikelyFixEta != nil || patch.WorstCaseFixEta != nil) {
		fieldsPatched := fixEtaFieldsPatched{
			best:       patch.BestCaseFixEta != nil,
			mostLikely: patch.MostLikelyFixEta != nil,
			worst:      patch.WorstCaseFixEta != nil,
		}
		detached := context.WithoutCancel(r.Context())
		go func() {
			ctx, cancel := context.WithTimeout(detached, 15*time.Second)
			defer cancel()
			h.recordFixEtaWorkNote(ctx, user, caseID, result, fieldsPatched)
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
		slog.WarnContext(ctx, "failed to record autoclose hold work note", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	}
}

// fixEtaFieldsPatched records which of the three fix-ETA fields were present
// on the incoming PATCH request, so recordFixEtaWorkNote can mention only
// fields that were actually part of this PATCH — the PATCH response's "case"
// object generally carries the full case, including fix-ETA values untouched
// by this request, so gating on the response alone would misattribute them.
type fixEtaFieldsPatched struct {
	best       bool
	mostLikely bool
	worst      bool
}

// recordFixEtaWorkNote adds an internal work note documenting a fix-ETA
// update, giving CS engineers a trail of the change on the case itself even
// when the caller didn't also request a customer-visible comment. Only
// fields present on the incoming request (fieldsPatched) are mentioned, but
// their values are read from the PATCH response rather than the request body
// so the note reflects what was actually committed upstream. Best-effort:
// failures are logged, never surfaced to the caller, since the primary PATCH
// already succeeded by the time this runs.
func (h *CaseHandler) recordFixEtaWorkNote(ctx context.Context, user *middleware.UserInfo, caseID string, result []byte, fieldsPatched fixEtaFieldsPatched) {
	var response struct {
		Case struct {
			BestCaseFixEta   string `json:"bestCaseFixEta"`
			MostLikelyFixEta string `json:"mostLikelyFixEta"`
			WorstCaseFixEta  string `json:"worstCaseFixEta"`
		} `json:"case"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		slog.ErrorContext(ctx, "failed to parse PATCH response for fix ETA work note", "userID", user.UserID, "caseID", caseID, "err", err)
		return
	}

	var parts []string
	if fieldsPatched.best {
		parts = append(parts, "Best case: "+response.Case.BestCaseFixEta)
	}
	if fieldsPatched.mostLikely {
		parts = append(parts, "Most likely: "+response.Case.MostLikelyFixEta)
	}
	if fieldsPatched.worst {
		parts = append(parts, "Worst case: "+response.Case.WorstCaseFixEta)
	}
	if len(parts) == 0 {
		return
	}
	note := "Fix ETA updated — " + strings.Join(parts, ", ")

	body, err := json.Marshal(map[string]string{
		"type":    "work_note",
		"content": note,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build fix ETA work note body", "userID", user.UserID, "caseID", caseID, "err", err)
		return
	}

	if _, err := h.entity.CreateCaseComment(ctx, caseID, body); err != nil {
		slog.WarnContext(ctx, "failed to record fix ETA work note", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
	if !caseIDRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case details.")
		return
	}
	if !h.canViewSecurityCenter(user) && caseViewIsSecurityReport(result) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
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

	if !h.requireCaseVisibleToCaller(w, r, user, caseID, "Failed to retrieve case escalation history.") {
		return
	}

	result, err := h.entity.SearchCaseEscalations(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseEscalations failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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

	action, valid := validateCaseEscalationBody(body)
	if !valid {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if action == "DEESCALATE" && !h.callerIsNotifiedOnCurrentEscalation(r, caseID, user) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	result, err := h.entity.CreateCaseEscalation(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseEscalation failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity CreateCallRequest failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity SearchCallRequests failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity SearchAllCallRequests failed", "userID", user.UserID, "err", summarizeErr(err))
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
		slog.ErrorContext(r.Context(), "entity PatchCallRequest failed", "userID", user.UserID, "caseID", caseID, "callRequestID", callRequestID, "err", summarizeErr(err))
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

	if h.engineering != nil {
		h.createGitHubIssueViaEngineering(w, r, user, caseID, body)
		return
	}

	result, err := h.entity.CreateCaseGithubIssue(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseGithubIssue failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to create GitHub issue.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// viewerCaseClient abstracts the backing-system operations used by ViewerCaseHandler.
// GetCases/GetCaseByNumber/GetCommentsAndWorknotes used to live here too,
// backed first by the backing system and later by a Postgres translation layer --
// both removed in favor of calling CS Portal's own POST /cases/search,
// GET /cases/{id}, and POST /cases/{id}/comments/search directly (worknote
// creation similarly merged onto POST /cases/{id}/comments, using the same
// entity-service CommentType distinction CS Portal's own comment handler
// already exposes -- see splWorknotesHandler's removal). Attachments have no
// entity-service equivalent at all yet (no Postgres storage/backfill path),
// so that one stays here, legacy-data-source, unmerged.
type viewerCaseClient interface {
	GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

// ViewerCaseHandler handles HTTP requests for SupportPortalLite's case-
// attachments endpoint -- the one piece of the case domain with no
// Postgres/entity-service equivalent to merge onto (see viewerCaseClient's own
// doc comment). Reading, searching, and commenting on cases now goes
// through CS Portal's own /cases routes directly.
type ViewerCaseHandler struct {
	sn          viewerCaseClient
	accessGuard *AccessGuard
}

// NewViewerCaseHandler creates a ViewerCaseHandler.
func NewViewerCaseHandler(sn viewerCaseClient, accessGuard *AccessGuard) *ViewerCaseHandler {
	return &ViewerCaseHandler{sn: sn, accessGuard: accessGuard}
}

// GetAttachmentsInfo handles GET /cases/{caseId}/attachments-info.
func (h *ViewerCaseHandler) GetAttachmentsInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetAttachmentsInfo(r.Context(), caseID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrCaseNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetAttachmentsInfo failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case attachments.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}
