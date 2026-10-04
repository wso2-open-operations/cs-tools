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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// attachmentsClient abstracts the backing system attachment-download
// operation used by AttachmentsHandler.
type attachmentsClient interface {
	// RequireCaseAttachment confirms attachmentSysID is attached to a case
	// before DownloadAttachment is called with it -- see its own doc
	// comment on servicenow.Client for why: DownloadAttachment's only
	// input is the attachment's own sys_id, with no scoping of its own.
	RequireCaseAttachment(ctx context.Context, attachmentSysID string) error
	DownloadAttachment(ctx context.Context, attachmentSysID string) (body []byte, contentType string, contentDisposition string, err error)
}

// AttachmentsHandler handles HTTP requests for downloading a case
// attachment, delegating to the backing system service.
type AttachmentsHandler struct {
	servicenow  attachmentsClient
	accessGuard *AccessGuard
}

// NewAttachmentsHandler creates a AttachmentsHandler backed by the
// given the backing system client. accessGuard enforces PermViewerAccess,
// SupportPortalLite's blanket audience gate, plus PermDownloadAttachment for
// DownloadAttachment specifically.
func NewAttachmentsHandler(sn attachmentsClient, accessGuard *AccessGuard) *AttachmentsHandler {
	return &AttachmentsHandler{servicenow: sn, accessGuard: accessGuard}
}

// DownloadAttachment handles GET /attachments/{attachmentId}/download. Only
// an allowlisted set of Content-Type values (safeAttachmentTypes, defined
// in cases.go) are honored, and the response always forces
// Content-Disposition: attachment regardless of what the backing system sent —
// mirroring this backend's existing GetCaseAttachmentContent convention,
// which never trusts an upstream Content-Type/Content-Disposition for
// inline rendering.
func (h *AttachmentsHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	if !requireViewerPermission(w, user, h.accessGuard, PermDownloadAttachment) {
		return
	}

	attachmentID := r.PathValue("attachmentId")
	if attachmentID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if err := h.servicenow.RequireCaseAttachment(r.Context(), attachmentID); err != nil {
		if errors.Is(err, servicenow.ErrAttachmentNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow RequireCaseAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment content.")
		return
	}

	content, contentType, _, err := h.servicenow.DownloadAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow DownloadAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment content.")
		return
	}

	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !safeAttachmentTypes[ct] {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment")
	_, _ = w.Write(content) // #nosec G705 -- Content-Type is allowlisted above; Content-Disposition: attachment prevents inline rendering
}
