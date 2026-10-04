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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/sftpgo"
)

// maxInlineImageSizeBytes caps a single extracted inline image's decoded
// size. Mirrors the webapp's MAX_IMAGE_SIZE_BYTES
// (apps/csm-portal/webapp/src/components/rich-text-editor/richTextConstants.ts)
// and the backing system's own RichTextUtils.MAX_SIZE_BYTES for the equivalent
// SN-backed path — all three are deliberately the same 10MB limit. (SN's own
// _validateAttachmentSize error message says "15MB", which is inconsistent
// with its own MAX_SIZE_BYTES=10485760 constant it actually enforces — not
// mirrored here; this uses the real, enforced limit in both the check and
// the message.)
const maxInlineImageSizeBytes = 10 * 1024 * 1024 // 10MB

// inlineImageUploadShareTTL bounds the write-scoped SFTPGo share minted for
// each inline-image upload. Deliberately much shorter than uploadShareTTL
// (used by the browser-driven two-phase upload flow in
// AttachmentStorageHandler.MintUploadToken): this upload happens
// synchronously, entirely server-side, within a single request — there is no
// browser round-trip to wait out.
const inlineImageUploadShareTTL = 5 * time.Minute

// allowedInlineImageTypes is the set of data: URI image subtypes this
// backend will extract and store as real attachments — mirrors the backing system's
// own RichTextUtils.processRichTextContent allow-list exactly, including
// "jpg" alongside "jpeg" (not a registered MIME subtype, but what some
// browsers/editors emit for a data: URI, and SN explicitly allows it too).
var allowedInlineImageTypes = map[string]bool{
	"png":  true,
	"jpeg": true,
	"jpg":  true,
	"webp": true,
}

// base64InlineImageTagRe extracts every <img> tag whose src is a base64
// data: URI of an image MIME type. Capture groups: 1 = the declared subtype
// (png/jpeg/jpg/webp/...), 2 = the base64 payload, which may itself contain
// whitespace/newlines from a rich-text editor's line wrapping — stripped via
// base64WhitespaceRe before decoding. Mirrors the backing system's
// RichTextUtils.base64ImgRegex, translated to Go's RE2 syntax; the original
// pattern uses no backreferences or lookaround, so the translation is exact.
var base64InlineImageTagRe = regexp.MustCompile(`(?i)<img[^>]+src=["']data:(?:@file/|image/)([a-zA-Z0-9.+-]+);base64,([A-Za-z0-9+/=\r\n\s]+)["'][^>]*>`)

// unsupportedInlineImageTagRe extracts the declared subtype of every base64
// image data: URI <img> tag, regardless of whether that subtype is allowed —
// used to reject-fast on any unsupported type before any image in the same
// comment is uploaded, mirroring the backing system's own reject-fast behavior (see
// RichTextUtils.processRichTextContent's unsupportedRegex pass).
var unsupportedInlineImageTagRe = regexp.MustCompile(`(?i)<img[^>]+src=["']data:(?:@file/|image/)([^;]+);base64,[^"']+["'][^>]*>`)

// base64WhitespaceRe strips whitespace/newlines a rich-text editor may have
// wrapped a long base64 payload with before it reaches base64.StdEncoding,
// which rejects them.
var base64WhitespaceRe = regexp.MustCompile(`\s+`)

// inlineImageError carries both the HTTP status and caller-facing message
// for a rejected or failed inline-image extraction, so
// CaseHandler.CreateCaseComment can surface it directly instead of always
// falling back to a generic 500.
type inlineImageError struct {
	status  int
	message string
}

func (e *inlineImageError) Error() string { return e.message }

// write sends e as this backend's standard {"message": "..."} error body.
func (e *inlineImageError) write(w http.ResponseWriter) {
	writeError(w, e.status, e.message)
}

// inlineImageSftpgoClient is the subset of sftpgo.Client operations
// InlineImageProcessor needs: mint a per-caller token, create a write-scoped
// share for a single upload, and push the decoded image bytes to SFTPGo
// synchronously. Narrower than the existing sftpgoClient interface (used by
// AttachmentStorageHandler) since this processor never creates a read-scoped
// download share or reports a base/public URL.
type inlineImageSftpgoClient interface {
	MintToken(ctx context.Context, email, jwtAssertion string) (*sftpgo.Token, error)
	CreateShare(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error)
	UploadBytes(ctx context.Context, shareID, storageKey string, data []byte, contentType string) error
	// RemoveFile deletes one already-uploaded object during rollback — see
	// Process's rollback closure.
	RemoveFile(ctx context.Context, accessToken, storageKey string) error
}

// InlineImageProcessor extracts base64 data: URI images embedded in a
// comment's rich-text HTML, uploads each as a real SFTPGo-backed attachment,
// and rewrites the HTML to reference it via a ".iix"-suffixed <img src> —
// mirroring the backing system's own
// RichTextUtils.processRichTextContent/_deleteAttachments for SN-backed
// comments
// (notes/sn-customer-portal-api/script-includes/RichTextUtils.js). Only used
// by CaseHandler.CreateCaseComment, and only when
// SFTPGO_ATTACHMENT_STORAGE_ENABLED is on — see
// CaseHandler.WithInlineImageProcessor.
type InlineImageProcessor struct {
	entity entityCaseClient
	sftpgo inlineImageSftpgoClient
}

// NewInlineImageProcessor creates an InlineImageProcessor backed by the given
// entity and SFTPGo clients.
func NewInlineImageProcessor(entity entityCaseClient, sftpgo inlineImageSftpgoClient) *InlineImageProcessor {
	return &InlineImageProcessor{entity: entity, sftpgo: sftpgo}
}

// Process extracts every base64 inline image from htmlContent, uploads each
// as a real attachment on caseID (storage-keyed under projectID's namespace;
// see buildStorageKey), and returns HTML with every extracted <img> src
// rewritten to "/<attachmentId>.iix". email/jwtAssertion authenticate this
// processor's own SFTPGo calls (see sftpgo.Client.MintToken). Returns
// htmlContent unchanged, with a nil error, when it carries no base64 inline
// image at all.
//
// Any unsupported inline image MIME subtype anywhere in htmlContent is
// rejected before any image is processed, mirroring the backing system's reject-fast
// behavior.
//
// Each image follows the same pending-first ordering as the browser-driven
// upload flow (see AttachmentStorageHandler.MintUploadToken/ConfirmUpload):
// the metadata row is created in "pending" status, the bytes are uploaded to
// SFTPGo, and only then is the row confirmed "complete" — so a crash or
// upstream failure can never leave a durable "complete" row whose bytes do
// not exist. Any failure partway through a multi-image comment — a size
// violation, an entity-service error, or an SFTPGo error — rolls back every
// attachment already created earlier in this same call, deleting both the
// metadata rows (DELETE /attachments/{id}) and any bytes already uploaded to
// SFTPGo, and rejects the whole comment: a partially-processed comment is
// never posted.
func (p *InlineImageProcessor) Process(ctx context.Context, email, jwtAssertion, caseID, projectID, htmlContent string) (string, *inlineImageError) {
	for _, m := range unsupportedInlineImageTagRe.FindAllStringSubmatch(htmlContent, -1) {
		detected := strings.ToLower(m[1])
		if !allowedInlineImageTypes[detected] {
			return "", &inlineImageError{
				status:  http.StatusBadRequest,
				message: fmt.Sprintf("Unsupported inline image type: '%s'. Allowed types: png, jpeg, jpg, webp.", detected),
			}
		}
	}

	matches := base64InlineImageTagRe.FindAllStringSubmatch(htmlContent, -1)
	if len(matches) == 0 {
		return htmlContent, nil
	}

	result := htmlContent
	var accessToken string
	var createdAttachmentIDs []string
	var uploadedStorageKeys []string

	// Rollback runs on a context detached from the request's cancellation:
	// the most likely moment to need cleanup is exactly when the request is
	// being torn down (caller gone, deadline hit), and a rollback that dies
	// with the request would leave the orphans it exists to remove. It
	// deletes BOTH the metadata rows created earlier in this call AND any
	// bytes already uploaded to SFTPGo (including the possibly-partial
	// object of the upload that failed), logging — never propagating — each
	// individual cleanup failure: the whole comment is already being
	// rejected, and a half-finished cleanup must still attempt the rest.
	rollback := func() {
		rbCtx := context.WithoutCancel(ctx)
		for _, id := range createdAttachmentIDs {
			if _, err := p.entity.DeleteCaseAttachment(rbCtx, id); err != nil {
				slog.ErrorContext(rbCtx, "inline-image rollback: DeleteCaseAttachment failed", "attachmentID", id, "err", summarizeErr(err))
			}
		}
		for _, key := range uploadedStorageKeys {
			if err := p.sftpgo.RemoveFile(rbCtx, accessToken, key); err != nil {
				slog.ErrorContext(rbCtx, "inline-image rollback: sftpgo RemoveFile failed", "storageKey", key, "err", summarizeErr(err))
			}
		}
	}

	for i, m := range matches {
		fullTag := m[0]
		imageType := strings.ToLower(m[1])
		cleanedBase64 := base64WhitespaceRe.ReplaceAllString(m[2], "")

		decoded, err := base64.StdEncoding.DecodeString(cleanedBase64)
		if err != nil {
			rollback()
			return "", &inlineImageError{status: http.StatusBadRequest, message: "Failed to decode an embedded image."}
		}
		// Validated BEFORE any upstream write, unlike the backing system's own
		// implementation (which validates size only after writing the
		// attachment and rolls back if too big) — this avoids an unnecessary
		// entity-service + SFTPGo round trip for an oversized image.
		if len(decoded) > maxInlineImageSizeBytes {
			rollback()
			return "", &inlineImageError{status: http.StatusBadRequest, message: "Image exceeds 10MB limit."}
		}

		contentType := "image/" + imageType
		fileName := fmt.Sprintf("inline-image-%d-%d.%s", time.Now().UnixMilli(), i+1, imageType)
		// A freshly generated id used only to give this image's storage path
		// its own directory (see buildStorageKey) — distinct from the
		// entity-assigned attachment id used below for the .iix reference and
		// for rollback, exactly as MintUploadToken already does for the
		// browser-driven upload path.
		storageDirID := newAttachmentID()
		storageKey := buildStorageKey(projectID, caseID, storageDirID, fileName)

		attachmentBody, err := json.Marshal(struct {
			ReferenceID   string `json:"referenceId"`
			ReferenceType string `json:"referenceType"`
			Name          string `json:"name"`
			Type          string `json:"type"`
			StorageKey    string `json:"storageKey"`
			SizeBytes     int    `json:"sizeBytes"`
			Status        string `json:"status"`
		}{
			ReferenceID:   caseID,
			ReferenceType: "case",
			Name:          fileName,
			Type:          contentType,
			StorageKey:    storageKey,
			SizeBytes:     len(decoded),
			// "pending" until the bytes are actually in SFTPGo — the row is
			// only confirmed "complete" after UploadBytes succeeds below,
			// mirroring the browser-driven two-phase flow
			// (MintUploadToken/ConfirmUpload). A crash or upstream failure
			// between here and the confirm leaves a pending row (invisible
			// to attachment lists, reconcilable later), never a durable
			// "complete" row whose bytes do not exist.
			Status: "pending",
		})
		if err != nil {
			rollback()
			return "", &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
		}

		createResult, err := p.entity.CreateCaseAttachment(ctx, attachmentBody)
		if err != nil {
			slog.ErrorContext(ctx, "inline-image entity CreateCaseAttachment failed", "caseID", caseID, "err", summarizeErr(err))
			rollback()
			return "", &inlineImageError{status: http.StatusBadGateway, message: "Failed to store an embedded image."}
		}
		var created struct {
			Attachment struct {
				ID string `json:"id"`
			} `json:"attachment"`
		}
		if err := json.Unmarshal(createResult, &created); err != nil || created.Attachment.ID == "" {
			slog.ErrorContext(ctx, "inline-image: failed to parse entity CreateCaseAttachment response", "caseID", caseID, "err", err)
			rollback()
			return "", &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
		}
		attachmentID := created.Attachment.ID
		// Tracked for rollback BEFORE the SFTPGo write below: if that write
		// fails, this row still needs to be deleted — only its bytes are
		// missing, not its metadata.
		createdAttachmentIDs = append(createdAttachmentIDs, attachmentID)

		if accessToken == "" {
			token, err := p.sftpgo.MintToken(ctx, email, jwtAssertion)
			if err != nil {
				slog.ErrorContext(ctx, "inline-image sftpgo MintToken failed", "caseID", caseID, "err", summarizeErr(err))
				rollback()
				return "", &inlineImageError{status: http.StatusBadGateway, message: "Failed to store an embedded image."}
			}
			accessToken = token.AccessToken
		}

		shareID, err := p.sftpgo.CreateShare(ctx, accessToken, path.Dir(storageKey), sftpgo.ShareScopeWrite, inlineImageUploadShareTTL)
		if err != nil {
			slog.ErrorContext(ctx, "inline-image sftpgo CreateShare failed", "caseID", caseID, "err", summarizeErr(err))
			rollback()
			return "", &inlineImageError{status: http.StatusBadGateway, message: "Failed to store an embedded image."}
		}

		if err := p.sftpgo.UploadBytes(ctx, shareID, storageKey, decoded, contentType); err != nil {
			slog.ErrorContext(ctx, "inline-image sftpgo UploadBytes failed", "caseID", caseID, "err", summarizeErr(err))
			// A failed TUS upload can still have written a partial object;
			// include this key in the byte cleanup rather than assuming the
			// failure left nothing behind.
			uploadedStorageKeys = append(uploadedStorageKeys, storageKey)
			rollback()
			return "", &inlineImageError{status: http.StatusBadGateway, message: "Failed to store an embedded image."}
		}
		uploadedStorageKeys = append(uploadedStorageKeys, storageKey)

		// Bytes are in SFTPGo; only now does the row become durable
		// "complete" — see the Status comment on the create call above.
		if _, err := p.entity.ConfirmCaseAttachment(ctx, attachmentID); err != nil {
			slog.ErrorContext(ctx, "inline-image entity ConfirmCaseAttachment failed", "caseID", caseID, "attachmentID", attachmentID, "err", summarizeErr(err))
			rollback()
			return "", &inlineImageError{status: http.StatusBadGateway, message: "Failed to store an embedded image."}
		}

		result = strings.Replace(result, fullTag, `<img src="/`+attachmentID+`.iix">`, 1)
	}

	return result, nil
}

// processCommentInlineImages runs InlineImageProcessor on the request body's
// top-level "content" field, if present and if it looks like it might carry
// an embedded data: URI image — a cheap prefilter so a comment with no
// inline image pays no extra upstream-call cost. Returns body unchanged
// (same slice) when there is nothing to do. Only called from
// CreateCaseComment when h.inlineImages is non-nil — see
// CaseHandler.WithInlineImageProcessor.
func (h *CaseHandler) processCommentInlineImages(r *http.Request, user *middleware.UserInfo, caseID string, body []byte) ([]byte, *inlineImageError) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		// body was already validated as well-formed JSON by the caller; a
		// non-object top level just means there is no "content" field.
		return body, nil
	}
	raw, ok := payload["content"]
	if !ok {
		return body, nil
	}
	var content string
	if err := json.Unmarshal(raw, &content); err != nil {
		// "content" isn't a plain string on this request; leave it for the
		// entity service's own validation to reject.
		return body, nil
	}
	if !strings.Contains(content, "base64,") {
		return body, nil
	}

	jwtAssertion := r.Header.Get(jwtAssertionHeader)
	if jwtAssertion == "" {
		// The Auth middleware already rejects any request without this
		// header before it reaches here — fail closed rather than mint a
		// token with an empty credential.
		return nil, &inlineImageError{status: http.StatusUnauthorized, message: ErrMsgUnauthorized}
	}

	caseRaw, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed while resolving project for inline-image extraction", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		return nil, &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
	}
	var currentCase struct {
		ProjectID string `json:"projectId"`
	}
	if err := json.Unmarshal(caseRaw, &currentCase); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse case for inline-image extraction", "userID", user.UserID, "caseID", caseID, "err", err)
		return nil, &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
	}

	newContent, ierr := h.inlineImages.Process(r.Context(), user.Email, jwtAssertion, caseID, currentCase.ProjectID, content)
	if ierr != nil {
		return nil, ierr
	}
	if newContent == content {
		return body, nil
	}

	newContentJSON, err := json.Marshal(newContent)
	if err != nil {
		return nil, &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
	}
	payload["content"] = newContentJSON
	newBody, err := json.Marshal(payload)
	if err != nil {
		return nil, &inlineImageError{status: http.StatusInternalServerError, message: ErrMsgInternal}
	}
	return newBody, nil
}
