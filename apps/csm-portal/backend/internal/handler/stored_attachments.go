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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// storedAttachmentEntityClient is the entity-service surface the stored
// attachment routes need. The entity service owns every access decision (the
// caller's own row-level security), so this handler adds no case lookups.
type storedAttachmentEntityClient interface {
	InitiateStoredAttachmentUpload(ctx context.Context, body []byte) ([]byte, error)
}

// StoredAttachmentHandler forwards the start of an upload to the entity service,
// which records the attachment and decides who may add it. The bytes themselves
// move through BlobAttachmentHandler, into Azure Blob Storage.
type StoredAttachmentHandler struct {
	entity storedAttachmentEntityClient
}

// NewStoredAttachmentHandler constructs a StoredAttachmentHandler.
func NewStoredAttachmentHandler(e storedAttachmentEntityClient) *StoredAttachmentHandler {
	return &StoredAttachmentHandler{entity: e}
}

// storedInlineImageTypes is the only set of types a streamed inline image is
// sent as; anything else is neutralised to application/octet-stream.
var storedInlineImageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// mapStoredAttachmentError maps an entity-service failure for these routes. The
// entity service's refusals here are written for the person using the portal
// ("This case is closed, so attachments can't be added", "Only the person who
// uploaded an attachment can delete it", "too many attachment links requested"),
// so 400, 403, 409 and 429 carry its message; anything internal is generic.
// errCodeAttachmentRemoved is the errorCode entity-service gives a 410 for an
// attachment that was deleted or removed, passed on unchanged.
const errCodeAttachmentRemoved = "attachment_removed"

// ErrMsgAttachmentRemoved is the message sent with that 410.
const ErrMsgAttachmentRemoved = "This image was removed."

func mapStoredAttachmentError(w http.ResponseWriter, err error, fallback string) {
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		writeError(w, http.StatusInternalServerError, fallback)
		return
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
	case http.StatusNotFound:
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
	case http.StatusGone:
		// An attachment (an inline comment image) that was removed on purpose.
		// The webapp tells it apart from a 404 by this code, never by the words.
		writeErrorCode(w, http.StatusGone, ErrMsgAttachmentRemoved, errCodeAttachmentRemoved)
	case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests:
		writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallback))
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		writeError(w, http.StatusServiceUnavailable, fallback)
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// readStoredAttachmentBody authenticates the caller and reads the capped body.
func readStoredAttachmentBody(w http.ResponseWriter, r *http.Request) (*middleware.UserInfo, []byte, bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
		} else {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		}
		return nil, nil, false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, nil, false
	}
	return user, body, true
}

// InitiateUpload handles POST /attachments/upload.
func (h *StoredAttachmentHandler) InitiateUpload(w http.ResponseWriter, r *http.Request) {
	user, body, ok := readStoredAttachmentBody(w, r)
	if !ok {
		return
	}
	if isCommentUpload(body) {
		if msg := commentUploadTarget(body); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	out, err := h.entity.InitiateStoredAttachmentUpload(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity InitiateStoredAttachmentUpload failed", "userID", user.UserID, "err", summarizeErr(err))
		mapStoredAttachmentError(w, err, "Failed to start the attachment upload.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, out)
}

// commentUploadTarget checks where a comment (inline image) upload is attached:
// exactly one of parentId (an existing work item) or projectId (a draft
// description image, uploaded before the case exists; a valid UUID). It returns
// a message for the caller when the body is wrong, "" when it is fine. The body
// itself is forwarded to entity-service untouched.
func commentUploadTarget(body []byte) string {
	var t struct {
		ParentID  *string `json:"parentId"`
		ProjectID *string `json:"projectId"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return "invalid request body"
	}
	hasParent := t.ParentID != nil && *t.ParentID != ""
	hasProject := t.ProjectID != nil && *t.ProjectID != ""
	switch {
	case hasParent && hasProject:
		return "provide either parentId or projectId for a comment image, not both"
	case !hasParent && !hasProject:
		return "parentId or projectId is required for a comment image"
	case hasProject && !uuidRe.MatchString(*t.ProjectID):
		return "projectId must be a valid UUID"
	}
	return ""
}

// isCommentUpload reports whether the body is for a type "comment" attachment.
func isCommentUpload(body []byte) bool {
	var p struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(body, &p) == nil && p.Type == "comment"
}
