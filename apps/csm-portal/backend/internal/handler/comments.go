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

// entityCommentClient abstracts the entity service's generic comment
// operations — edit and soft-delete apply to a comment by id regardless of
// which aggregate (case, change request, incident, ...) it was created
// under, so there is one client interface and one handler for both, rather
// than one per aggregate. GET /comments/{id}/history also exists on the
// entity service but deliberately has no BFF pass-through here — see
// CommentHandler's doc comment.
type entityCommentClient interface {
	UpdateComment(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteComment(ctx context.Context, id string) ([]byte, error)
}

// CommentHandler handles HTTP requests for the generic comment resource:
// PATCH /comments/{id} and DELETE /comments/{id}. Comments are created and
// searched through per-aggregate routes (e.g. POST /cases/{id}/comments),
// but once a comment exists its id is enough to edit or delete it, so those
// two operations are exposed here once instead of duplicated per aggregate.
//
// GET /comments/{id}/history exists on the entity service (author-or-admin
// gated) but is intentionally not exposed here: there is no UI consumer yet
// (a dedicated history-viewer UI was deferred), and adding an unscoped or
// under-scoped pass-through for it would repeat the mistake already made and
// caught at the entity-service layer for this same feature.
type CommentHandler struct {
	entity entityCommentClient
}

// NewCommentHandler creates a CommentHandler backed by the given entity client.
func NewCommentHandler(entity entityCommentClient) *CommentHandler {
	return &CommentHandler{entity: entity}
}

// UpdateComment handles PATCH /comments/{id} with body {"content": "..."}.
//
// Uses mapUpstreamError, not mapUpstreamErrorGeneric: the upstream 400 ("a
// deleted comment cannot be edited") is a rejection of the edit the caller
// just attempted, the same reasoning that puts the ten existing PATCH/update
// handlers on mapUpstreamError (see backend CLAUDE.md's Handler
// conventions) — this is simply an eleventh. The upstream 403 ("only the
// comment's author or an admin may modify it") is not affected by this
// choice either way: both mapUpstreamError and mapUpstreamErrorGeneric
// always return the fixed ErrMsgForbidden for 403, never the upstream text.
func (h *CommentHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.UpdateComment(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateComment failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update comment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteComment handles DELETE /comments/{id}. Soft delete only — content is
// never destroyed. Carries no body, but still uses mapUpstreamError rather
// than mapUpstreamErrorGeneric: the upstream 409 ("comment is already
// deleted") is a rejection of the delete action the caller just took on this
// specific comment, not an unvalidated payload — the same reasoning as
// UpdateComment above, just triggered by path state instead of body content.
// The entity service returns 204 No Content on success, forwarded as-is with
// no body (same pattern as CaseHandler.RemoveCaseTag).
func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
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

	if _, err := h.entity.DeleteComment(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteComment failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to delete comment.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
