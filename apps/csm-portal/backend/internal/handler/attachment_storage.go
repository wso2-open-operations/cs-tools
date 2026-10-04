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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
)

// shareTTL bounds how long a created SFTPGo share — and by extension the
// public download/inline-image URL handed to the caller — stays valid. Kept
// short because a share is minted fresh on every request that needs one; see
// AttachmentStorageHandler.CreateAttachmentShare's doc comment on why shares
// are never created eagerly.
const shareTTL = 5 * time.Minute

// uploadShareTTL bounds how long a write-scoped upload share (minted by
// MintUploadToken) stays valid. Deliberately much longer than shareTTL: an
// upload share has to stay open for the entire duration of the browser's
// direct TUS upload to SFTPGo — including retries/resumes of a large or
// slow file — not just long enough to redeem a single GET. 45 minutes is a
// documented, reasonable choice rather than a value verified against any
// real-world upload-duration data; revisit if large attachments start
// failing with an expired-share error near the end of a long upload.
const uploadShareTTL = 45 * time.Minute

// jwtAssertionHeader mirrors the unexported constant of the same name in
// internal/middleware/auth.go — that package does not export it, so it is
// duplicated here rather than introducing a cross-package export for a
// single literal.
const jwtAssertionHeader = "x-jwt-assertion"

// sftpgoClient abstracts the SFTPGo operations used by AttachmentStorageHandler,
// allowing the handler to be tested without a live SFTPGo instance.
type sftpgoClient interface {
	MintToken(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error)
	CreateShare(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error)
	PublicShareURL(shareID string) string
	BaseURL() string
}

// attachmentStorageEntityClient is the entity-service surface
// AttachmentStorageHandler needs: everything the case attachment flows use,
// plus the change-request and incident detail lookups that back this
// handler's per-reference-type access checks (an attachment can reference a
// case, a change request, or an incident — see referenceType on the entity
// service's attachment schema).
type attachmentStorageEntityClient interface {
	entityCaseClient
	GetChangeRequest(ctx context.Context, id string) ([]byte, error)
	GetIncident(ctx context.Context, id string) ([]byte, error)
}

// AttachmentStorageHandler implements the SFTPGo-backed attachment-storage
// endpoints: minting a write-scoped SFTPGo share before an upload, and
// creating a short-lived read-scoped public download share for an
// already-stored attachment. It never touches attachment bytes itself —
// uploads and downloads go directly between the browser and SFTPGo, and the
// browser never sees a bearer credential of any kind: a Share is scoped to
// exactly one storage path and one direction (read or write), so the worst
// a leaked share id can do is read or write that single path — see package
// sftpgo's doc comment. Its routes are only registered (and therefore only
// reachable) when SFTPGO_ATTACHMENT_STORAGE_ENABLED is on — see
// cmd/server/main.go. The existing streaming attachment endpoints on
// CaseHandler are completely unaffected by this handler and by the flag.
type AttachmentStorageHandler struct {
	entity attachmentStorageEntityClient
	sftpgo sftpgoClient
}

// NewAttachmentStorageHandler creates an AttachmentStorageHandler backed by
// the given entity and SFTPGo clients.
func NewAttachmentStorageHandler(entity attachmentStorageEntityClient, sftpgo sftpgoClient) *AttachmentStorageHandler {
	return &AttachmentStorageHandler{entity: entity, sftpgo: sftpgo}
}

// mintUploadTokenRequest is the request body of
// POST /cases/{id}/attachments/upload-token. The frontend must send the
// file's metadata up front, before it uploads any bytes: MintUploadToken now
// creates the attachment's metadata row (in "pending" status) as part of
// minting the upload credential, and the entity service has no other source
// for this data — it never sees the file bytes on this data source (see
// domain.CreateAttachmentRequest in the entity service).
type mintUploadTokenRequest struct {
	// Filename is the file's name including extension. Required; forwarded
	// verbatim as the entity service's CreateAttachmentRequest.Name.
	Filename string `json:"filename"`
	// MimeType is the file's MIME type (e.g. "image/png", "application/pdf").
	// Required; forwarded verbatim as CreateAttachmentRequest.Type.
	MimeType string `json:"mimeType"`
	// SizeBytes is the file's size in bytes, as reported by the browser
	// before upload. Required: this backend never sees the file's bytes, so
	// this is the only source of truth for size on this data source (mirrors
	// CreateAttachmentRequest.SizeBytes).
	SizeBytes int `json:"sizeBytes"`
	// Description is an optional free-text description, forwarded verbatim.
	// Mirrors AttachmentCreatePayload.description on the existing
	// (non-SFTPGo) attachment-creation path.
	Description *string `json:"description,omitempty"`
	// ReferenceType identifies what kind of entity the path's {id} refers to:
	// "case" (the default when omitted, preserving the pre-existing contract
	// for every caller that never sends this field), "change_request", or
	// "incident" — the same reference types the existing (non-SFTPGo)
	// POST /attachments path accepts. Additive: the field is optional, and an
	// unrecognised value is rejected with 400 rather than guessed at.
	ReferenceType string `json:"referenceType,omitempty"`
}

// Attachment reference types accepted by MintUploadToken and understood by
// CreateAttachmentShare. Mirrors the entity service's ReferenceType enum for
// the subset of entities this handler can authorize against.
const (
	refTypeCase          = "case"
	refTypeChangeRequest = "change_request"
	refTypeIncident      = "incident"
)

// uploadTokenResponse is the response body of
// POST /cases/{id}/attachments/upload-token.
type uploadTokenResponse struct {
	// ID is the id of the attachment metadata row MintUploadToken creates (in
	// "pending" status) before minting the share below. The frontend must
	// hold onto this and send it back as the path parameter to
	// POST /cases/{id}/attachments/{attachmentId}/confirm once the browser's
	// direct-to-SFTPGo upload succeeds — see
	// AttachmentStorageHandler.ConfirmUpload.
	ID string `json:"id"`
	// ShareID is a write-scoped, passwordless SFTPGo share id, restricted to
	// StorageKey's parent directory (see the CreateShare call in
	// MintUploadToken for why it is the directory and not the file itself —
	// SFTPGo's shares-chunked-uploads endpoint always resolves the TUS
	// "path" metadata relative to the share's own scoped path, confirmed
	// empirically against a real instance; scoping the share to the exact
	// file, rather than its directory, made every upload fail). The frontend
	// embeds this id as the "share_id" key in the TUS Upload-Metadata header
	// it sends to SFTPGo's POST /shares-chunked-uploads, and MUST send only
	// StorageKey's final path segment (everything after the last "/") as
	// the "path" key — NOT the full StorageKey — since the share's own root
	// already covers the directory portion; sending the full StorageKey as
	// "path" would double it up (e.g. ".../case-1/case-1/<id>") and fail.
	// That share id is the entire credential; no bearer token or
	// Authorization header is ever involved in the upload. The frontend must
	// also set "mkdir_parents" to "true" in the same Upload-Metadata header:
	// this directory is not guaranteed to already exist (e.g. a case's first
	// attachment ever), and SFTPGo does not create it implicitly otherwise.
	// The share's own server-side expiry governs how long the upload window
	// stays open; the frontend does not need that value, it either works or
	// the share is gone.
	ShareID       string `json:"shareId"`
	SftpgoBaseURL string `json:"sftpgoBaseUrl"`
	// StorageKey is the exact SFTPGo path the uploaded file must end up at,
	// and must later be sent back unchanged as
	// CreateAttachmentRequest.storageKey when the frontend creates the
	// attachment metadata row. Minted here, server-side, rather than left
	// for the frontend to invent, so the id embedded in it is guaranteed to
	// match no other attachment. See buildStorageKey for the path
	// convention, and ShareID's doc comment above for how the frontend must
	// derive the TUS upload's "path" metadata from this value.
	StorageKey string `json:"storageKey"`
}

// MintUploadToken handles POST /cases/{id}/attachments/upload-token. It
// registers the attachment's metadata row up front — in "pending" status,
// via the entity service — and only then mints a write-scoped, passwordless
// SFTPGo share restricted to a single, freshly generated storage path, for
// the browser to use directly against SFTPGo's share-authenticated
// chunked/TUS upload endpoint (POST /shares-chunked-uploads) — this backend
// never sees the uploaded bytes, and no bearer credential of any kind
// reaches the browser: a write-scoped share can do nothing but write to the
// one path it names.
//
// Creating the pending row before minting the share closes a real
// reliability gap in the previous design: a browser upload could succeed in
// SFTPGo while CSM never learned the file existed at all, because nothing
// durable was recorded until the frontend made a second, separate call after
// the upload finished — a call that could simply never arrive (tab closed,
// network drop, crash). Recording "an upload was started" first means a
// future reconciliation job can always find and clean up an upload that
// never got confirmed; there is no longer a window where a real file in
// SFTPGo has zero trace in CSM. See AttachmentStorageHandler.ConfirmUpload
// for the second half of this flow.
//
// Requires write access to the target case, checked via the same guard
// CaseHandler.CreateCaseAttachment already applies (case exists and is not
// closed): a row is never created, and a share never minted, for a case the
// caller could not otherwise attach a file to.
//
// Reference types other than "case": the request body's optional
// referenceType field declares what the path's {id} refers to (the route is
// kept unchanged for compatibility; {id} is really the reference entity's
// id). "change_request" and "incident" are recognised and authorized (the
// caller must be able to read the referenced entity, mirroring how the CR and
// incident detail endpoints gate reads today), but then rejected with 422:
// the entity service's direct-upload persistence is case-scoped today — it
// has no storage-key-backed attachment records for change requests or
// incidents — so those uploads must go through the standard attachment path
// instead. This gives a clear, honest error rather than the confusing
// not-found a change-request id used to produce when it was assumed to be a
// case id. Anything else is rejected with 400.
func (h *AttachmentStorageHandler) MintUploadToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	referenceID := r.PathValue("id")
	if referenceID == "" || !uuidRe.MatchString(referenceID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	// Decode strictly: reject unknown fields and a trailing second JSON value
	// rather than silently ignoring either, per this repo's boundary-input
	// validation convention (validate and reject unexpected input before it
	// is ever forwarded to an upstream service).
	var req mintUploadTokenRequest
	dec := json.NewDecoder(bytes.NewReader(rawBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// Checked here, cheaply, before any entity/SFTPGo call: these three
	// fields are the only source of truth for this data (this backend never
	// sees the file's bytes), and the entity service rejects their absence
	// anyway (see domain.CreateAttachmentRequest validation) — failing fast
	// on an obviously-incomplete request avoids a wasted GetCase + MintToken
	// round trip. The entity service's own validation remains authoritative
	// for anything more specific than "present."
	if req.Filename == "" || req.MimeType == "" || req.SizeBytes <= 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Empty defaults to "case": every existing caller (which never sends the
	// field) keeps exactly the pre-existing behavior.
	referenceType := req.ReferenceType
	if referenceType == "" {
		referenceType = refTypeCase
	}
	switch referenceType {
	case refTypeCase:
		// Falls through to the full flow below.
	case refTypeChangeRequest, refTypeIncident:
		// Authorize FIRST — the caller must be able to read the referenced
		// entity (the same gate its detail endpoint applies: the entity
		// service's own 403/404 decides) — and only then report the
		// capability gap. A caller with no access to the entity learns
		// nothing about which storage modes it supports.
		if !h.canReadReference(w, r, referenceType, referenceID) {
			return
		}
		// The entity service's direct-upload persistence is case-scoped
		// today: it has no storage-key-backed attachment records for change
		// requests or incidents, so there is nothing this flow could durably
		// record. Fail with a clear 422 instead of pretending — the standard
		// attachment upload path still works for these types.
		writeError(w, http.StatusUnprocessableEntity, ErrMsgAttachmentStorageUnsupportedRef)
		return
	default:
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	caseID := referenceID
	projectID, ok := h.canWriteCase(w, r, caseID)
	if !ok {
		return
	}

	jwtAssertion := r.Header.Get(jwtAssertionHeader)
	if jwtAssertion == "" {
		// The Auth middleware already rejects any request without this
		// header before it reaches here, so this is not a real caller path —
		// fail closed rather than mint a token with an empty credential.
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// MintToken is still needed here even though the resulting access token
	// is never returned to the frontend: this backend still has to
	// authenticate its own server-side call to POST /api/v2/user/shares
	// below, on the caller's behalf.
	token, err := h.sftpgo.MintToken(r.Context(), user.Email, jwtAssertion)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo MintToken failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to obtain an upload token.")
		return
	}

	// Generated here, server-side, rather than left for the frontend to
	// invent: the frontend has no way to guarantee id uniqueness or apply the
	// storage-key convention, and both are this backend's responsibility.
	attachmentID := newAttachmentID()
	storageKey := buildStorageKey(projectID, caseID, attachmentID, req.Filename)

	// Create the attachment's metadata row BEFORE minting the upload share —
	// see the doc comment above on why this ordering is load-bearing. If this
	// fails, the upload must not proceed: minting a share for a file with no
	// corresponding CSM record would recreate the exact gap this change
	// closes.
	attachmentBody, err := json.Marshal(struct {
		ReferenceID   string  `json:"referenceId"`
		ReferenceType string  `json:"referenceType"`
		Name          string  `json:"name"`
		Type          string  `json:"type"`
		StorageKey    string  `json:"storageKey"`
		SizeBytes     int     `json:"sizeBytes"`
		Status        string  `json:"status"`
		Description   *string `json:"description,omitempty"`
	}{
		ReferenceID:   referenceID,
		ReferenceType: referenceType,
		Name:          req.Filename,
		Type:          req.MimeType,
		StorageKey:    storageKey,
		SizeBytes:     req.SizeBytes,
		Status:        "pending",
		Description:   req.Description,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to marshal pending attachment request", "userID", user.UserID, "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	createResult, err := h.entity.CreateCaseAttachment(r.Context(), attachmentBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseAttachment (pending) failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to register the attachment.")
		return
	}
	var created struct {
		Attachment struct {
			ID string `json:"id"`
		} `json:"attachment"`
	}
	if err := json.Unmarshal(createResult, &created); err != nil || created.Attachment.ID == "" {
		slog.ErrorContext(r.Context(), "failed to parse entity CreateCaseAttachment response", "userID", user.UserID, "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	// The share is scoped to storageKey's parent DIRECTORY, not storageKey
	// itself — confirmed empirically against a real SFTPGo instance that a
	// write-scope share used with POST /shares-chunked-uploads always
	// resolves the TUS "path" metadata relative to the share's own path, so
	// a share scoped to the exact file (rather than its directory) makes
	// every upload against it fail with "unable to write to file". Since
	// buildStorageKey now makes the attachment id its own directory
	// (".../cases/<caseId>/<attachmentId>/<filename>"), this directory is
	// the attachment's OWN directory, not the whole case's — tighter than
	// before: the worst a leaked share id can do is write within this one
	// attachment's own folder, not the entire case's attachment tree.
	// Confirmed empirically that SFTPGo's mkdir_parents still creates this
	// now-one-level-deeper directory (case dir + attachment-id dir) and
	// writes the file inside it in a single TUS upload. See
	// uploadTokenResponse.ShareID's doc comment for the corresponding
	// frontend-side contract.
	shareDir := path.Dir(storageKey)
	shareID, err := h.sftpgo.CreateShare(r.Context(), token.AccessToken, shareDir, sftpgo.ShareScopeWrite, uploadShareTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo CreateShare (write) failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to obtain an upload token.")
		return
	}

	writeJSONValue(w, http.StatusOK, uploadTokenResponse{
		ID:            created.Attachment.ID,
		ShareID:       shareID,
		SftpgoBaseURL: h.sftpgo.BaseURL(),
		StorageKey:    storageKey,
	})
}

// ConfirmUpload handles POST /cases/{caseId}/attachments/{attachmentId}/confirm.
// It is the second half of the two-step upload flow MintUploadToken starts:
// once the browser's direct-to-SFTPGo upload has actually succeeded, the
// frontend calls this to transition the attachment's metadata row from
// "pending" to "complete" — see the entity service's ConfirmCaseAttachment,
// which enforces that only the same actor who created the pending row may
// confirm it (stricter than this data source's other attachment operations,
// which impose no per-resource ACL beyond authentication today).
//
// caseId is accepted on the path for consistency with this backend's other
// nested attachment/case routes, but is not otherwise used: the entity
// service's confirm endpoint is addressed by attachment id alone, and the
// actor check it performs is what actually authorizes this call — there is
// no independent case-level guard to apply here.
func (h *AttachmentStorageHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
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
	attachmentID := r.PathValue("attachmentId")
	if attachmentID == "" || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.ConfirmCaseAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ConfirmCaseAttachment failed", "userID", user.UserID, "caseID", caseID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to confirm the attachment upload.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// attachmentMeta is the subset of the entity service's Attachment fields this
// handler needs (see openapi.yaml's Attachment schema) plus storageKey.
//
// Assumption flagged: storageKey does not exist on the entity service's
// Attachment schema today — every existing attachment is a base64 payload
// the entity service stores itself, with no notion of an external storage
// path. This field is assumed to be added by a corresponding entity-service
// change (out of scope for this layer/PR) that populates it with the SFTPGo
// path an attachment was uploaded to, only when it was created via the
// SFTPGo-backed upload path. A missing/empty storageKey here is treated as
// "not shareable" rather than an error — see StorageKey's nil check below —
// so an attachment stored the old way fails cleanly instead of panicking.
type attachmentMeta struct {
	ReferenceID   string  `json:"referenceId"`
	ReferenceType string  `json:"referenceType"`
	StorageKey    *string `json:"storageKey"`
}

// shareResponse is the response body of POST /attachments/{id}/share.
type shareResponse struct {
	ShareURL string `json:"shareUrl"`
}

// CreateAttachmentShare handles POST /attachments/{id}/share. It creates a
// fresh, short-lived (shareTTL) SFTPGo public share for one attachment's
// stored file and returns its public URL.
//
// This must be called lazily — once per attachment id, only when that
// specific attachment is actually opened or an inline image is actually
// rendered — never eagerly for a whole attachment list or comment thread: a
// share is a real SFTPGo object with its own lifecycle, and creating one for
// every attachment on every list/search response would mint SFTPGo shares
// nobody asked to open.
func (h *AttachmentStorageHandler) CreateAttachmentShare(w http.ResponseWriter, r *http.Request) {
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

	raw, err := h.entity.GetCaseAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCaseAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment.")
		return
	}
	var meta attachmentMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse attachment metadata", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	// Read-access check, fail closed: the caller must be shown to have read
	// access to the entity this attachment references before a public
	// (passwordless) share URL is ever minted for it. The check is keyed on
	// the attachment's own referenceType/referenceId as reported by the
	// entity service; a missing, empty, or unrecognised reference means
	// access CANNOT be established — deny with 404, never assume a type. (In
	// particular, the backing data source that predates referenceType on
	// attachment details reports it empty; its attachments carry no
	// storageKey and were never shareable through this endpoint anyway.)
	// Per-type, the check mirrors the gate each entity's own detail/content
	// endpoints apply today: the entity service's own 403/404 on the detail
	// lookup decides who can see what.
	if !h.canReadReference(w, r, meta.ReferenceType, meta.ReferenceID) {
		return
	}

	if meta.StorageKey == nil || *meta.StorageKey == "" {
		writeError(w, http.StatusConflict, ErrMsgAttachmentNotShareable)
		return
	}

	jwtAssertion := r.Header.Get(jwtAssertionHeader)
	if jwtAssertion == "" {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	token, err := h.sftpgo.MintToken(r.Context(), user.Email, jwtAssertion)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo MintToken failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to create a share for this attachment.")
		return
	}

	shareID, err := h.sftpgo.CreateShare(r.Context(), token.AccessToken, *meta.StorageKey, sftpgo.ShareScopeRead, shareTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "sftpgo CreateShare (read) failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		writeError(w, http.StatusBadGateway, "Failed to create a share for this attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, shareResponse{ShareURL: h.sftpgo.PublicShareURL(shareID)})
}

// canWriteCase mirrors CaseHandler.CreateCaseAttachment's closed-case guard —
// the same check that gates whether a case may receive a new attachment
// today — reused here so an upload token is never minted for a case an
// upload could not proceed against anyway. projectID is the case's
// projectId as reported by the entity service (see domain.Case.ProjectID),
// used by MintUploadToken to build the storage key; it is "" when the
// upstream response omits it, which buildStorageKey treats as "no project
// concept for this case" rather than an error.
func (h *AttachmentStorageHandler) canWriteCase(w http.ResponseWriter, r *http.Request, caseID string) (projectID string, ok bool) {
	current, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during attachment-storage write guard", "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to validate case access.")
		return "", false
	}
	var currentCase struct {
		State     string `json:"state"`
		ProjectID string `json:"projectId"`
	}
	if err := json.Unmarshal(current, &currentCase); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse case state for attachment-storage write guard", "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return "", false
	}
	if currentCase.State == "closed" {
		writeError(w, http.StatusConflict, ErrMsgAttachmentOnClosedCase)
		return "", false
	}
	return currentCase.ProjectID, true
}

// newAttachmentID generates a random UUID v4 for a not-yet-created
// attachment. Mirrors middleware.newCorrelationID's approach (that helper is
// unexported in a different package, so it is duplicated here rather than
// exporting a single-purpose helper across a package boundary for it).
func newAttachmentID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("attachment_storage: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// buildStorageKey computes the SFTPGo path an attachment's bytes live under:
// "/attachments/project-<projectId>/cases/<caseId>/<attachmentId>/<filename>".
// SFTPGo permissions are granted per project, so the project segment is
// load-bearing whenever a project is known.
//
// The attachment id is a directory, not a filename prefix: every attachment
// gets its own freshly generated UUID directory, so the leaf underneath it
// can be the sanitized original filename itself with zero risk of collision
// with any other attachment — no disambiguation or retry logic needed, ever.
// This also means SFTPGo's own share-download handler, which sets
// Content-Disposition's filename from path.Base() of the stored path (real
// SFTPGo OSS source, internal/httpd/api_utils.go), serves the file back with
// exactly its original name and extension, rather than a UUID-prefixed one.
// A human browsing this tree directly (e.g. over SFTP) also sees real
// filenames, with the UUID directories providing per-attachment isolation
// instead of noise in every filename. See sanitizeFilenameForStorageKey for
// the sanitization rules — filename is untrusted, attacker-controlled input
// that ends up in a filesystem path, so it is never used as-is. If
// sanitization strips the filename to nothing (empty, all separators/dots,
// or otherwise entirely invalid), the attachment id is reused as the leaf
// name too, so the path is always well-formed even in that edge case.
//
// projectID is "" when the case's own record carries no project reference.
// Cases in this Postgres/CSM-native data source are NOT guaranteed to have a
// project: domain.Case.ProjectID exists on the schema, but nothing in
// entity-service enforces it is always populated for a CSM-native case
// (unlike backing-system-sourced cases, which are always project-scoped). Rather
// than block minting a token over a missing project reference, this falls
// back to a project-less path shape,
// "/attachments/cases/<caseId>/<attachmentId>/<filename>", which still
// uniquely identifies the file. This fallback path cannot be granted SFTPGo
// permissions per-project the way the documented convention can; it is
// accepted here as a deliberate, narrower scope (case-only) rather than a
// blocker, and should be revisited if/when CSM-native cases gain a
// guaranteed project reference.
func buildStorageKey(projectID, caseID, attachmentID, filename string) string {
	sanitized := sanitizeFilenameForStorageKey(filename)
	if sanitized == "" {
		sanitized = attachmentID
	}
	if projectID == "" {
		return fmt.Sprintf("/attachments/cases/%s/%s/%s", caseID, attachmentID, sanitized)
	}
	return fmt.Sprintf("/attachments/project-%s/cases/%s/%s/%s", projectID, caseID, attachmentID, sanitized)
}

// maxSanitizedFilenameLen caps the sanitized filename portion of a storage
// key's leaf segment. Combined with the attachmentID (36 chars) and a
// separator, this keeps the leaf well under common filesystem/SFTPGo
// path-component length limits (typically 255 bytes) even for a filename
// with multi-byte UTF-8 characters.
const maxSanitizedFilenameLen = 200

// sanitizeFilenameForStorageKey makes an untrusted, user-supplied filename
// safe to use as one path segment of an SFTPGo storage key. filename becomes
// part of a filesystem path on a real backing store, so it is treated as
// hostile input rather than display text:
//
//   - Path separators ("/", "\") are stripped so the result cannot introduce
//     extra path segments (which would, among other things, change what
//     directory path.Dir(storageKey) resolves to — see buildStorageKey's
//     callers, which rely on that directory for share scoping).
//   - ".." sequences are stripped so the result cannot be used for path
//     traversal.
//   - Control characters (including NUL) are stripped.
//   - Leading dots are stripped, so the result can never collide with "." or
//     ".." on its own, or produce a hidden-file-like leaf.
//   - The result is capped to maxSanitizedFilenameLen bytes (after the above
//     stripping), measured in a way that never splits a multi-byte UTF-8
//     rune.
//
// If every character is stripped (filename was empty, all separators/dots,
// or otherwise entirely invalid), this returns "" and the caller falls back
// to the bare attachmentID leaf — today's behavior — rather than producing a
// malformed or empty path segment.
func sanitizeFilenameForStorageKey(filename string) string {
	var b []rune
	for _, r := range filename {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		b = append(b, r)
	}
	cleaned := string(b)

	// Strip ".." sequences (after separator/control-char removal, so an
	// input like "..\/.." can't reassemble one post-sanitization).
	for {
		replaced := strings.ReplaceAll(cleaned, "..", "")
		if replaced == cleaned {
			break
		}
		cleaned = replaced
	}

	// Strip leading dots.
	cleaned = strings.TrimLeft(cleaned, ".")

	// Cap length without splitting a multi-byte rune.
	if len(cleaned) > maxSanitizedFilenameLen {
		truncated := cleaned[:maxSanitizedFilenameLen]
		for len(truncated) > 0 && !utf8.ValidString(truncated) {
			truncated = truncated[:len(truncated)-1]
		}
		cleaned = truncated
	}

	return cleaned
}

// canReadReference confirms the caller can view the entity an attachment
// references, dispatching on the attachment's own referenceType. FAIL
// CLOSED: an empty or unrecognised reference type, or a missing reference
// id, denies with 404 — it is never assumed to mean "case". An empty
// referenceType is a real occurrence, not just a defensive case: the backing
// data source that predates referenceType on attachment details reports it
// empty. On failure the response has already been written; the caller must
// only return.
func (h *AttachmentStorageHandler) canReadReference(w http.ResponseWriter, r *http.Request, referenceType, referenceID string) bool {
	if referenceID == "" {
		slog.WarnContext(r.Context(), "attachment access denied: attachment carries no reference id", "referenceType", referenceType)
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return false
	}
	switch referenceType {
	case refTypeCase:
		return h.canReadCase(w, r, referenceID)
	case refTypeChangeRequest:
		if _, err := h.entity.GetChangeRequest(r.Context(), referenceID); err != nil {
			slog.ErrorContext(r.Context(), "entity GetChangeRequest failed during attachment-storage read guard", "changeRequestID", referenceID, "err", summarizeErr(err))
			mapUpstreamErrorGeneric(w, err, "Failed to validate change request access.")
			return false
		}
		return true
	case refTypeIncident:
		if _, err := h.entity.GetIncident(r.Context(), referenceID); err != nil {
			slog.ErrorContext(r.Context(), "entity GetIncident failed during attachment-storage read guard", "incidentID", referenceID, "err", summarizeErr(err))
			mapUpstreamErrorGeneric(w, err, "Failed to validate incident access.")
			return false
		}
		return true
	default:
		slog.WarnContext(r.Context(), "attachment access denied: unrecognised reference type", "referenceType", referenceType)
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return false
	}
}

// canReadCase confirms the caller can view the target case at all — mirrors
// the (lack of) additional gating on GetCaseAttachmentContent and
// SearchCaseAttachments today: those endpoints impose no case-state
// restriction and rely entirely on the entity service's own GetCase access
// control (a 403/404 from upstream) to decide who can see what.
func (h *AttachmentStorageHandler) canReadCase(w http.ResponseWriter, r *http.Request, caseID string) bool {
	if _, err := h.entity.GetCase(r.Context(), caseID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during attachment-storage read guard", "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to validate case access.")
		return false
	}
	return true
}
