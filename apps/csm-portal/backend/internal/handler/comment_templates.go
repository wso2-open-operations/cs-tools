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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// Messages for a comment template request that fails upstream. On a create or
// an update, entity-service's validation or name-clash message is shown as it
// wrote it (mapUpstreamError): the user can act on it ("a template named X
// already exists"). A list or delete failure has nothing to act on, so it gets
// the fixed message (mapUpstreamErrorGeneric).
const (
	errMsgCommentTemplatesListFailed  = "Your comment templates couldn't be loaded. Try again in a moment."
	errMsgCommentTemplateSaveFailed   = "The template couldn't be saved. Try again in a moment."
	errMsgCommentTemplateDeleteFailed = "The template couldn't be deleted. Try again in a moment."
)

type entityCommentTemplateClient interface {
	ListCaseCommentTemplates(ctx context.Context) ([]byte, error)
	CreatePersonalCaseCommentTemplate(ctx context.Context, body []byte) ([]byte, error)
	UpdatePersonalCaseCommentTemplate(ctx context.Context, id string, body []byte) ([]byte, error)
	DeletePersonalCaseCommentTemplate(ctx context.Context, id string) error
	CreateSharedCaseCommentTemplate(ctx context.Context, body []byte) ([]byte, error)
	UpdateSharedCaseCommentTemplate(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteSharedCaseCommentTemplate(ctx context.Context, id string) error
}

// CommentTemplateHandler serves the saved case comment templates the comment
// box offers. Who may do what is decided by the routes alone (cmd/server):
// anyone who can post a comment or work note lists the templates and manages
// their own; only an admin writes the shared ones. entity-service keys the
// personal routes to the forwarded x-user-id-token's user, so a personal
// call can never reach another user's template or a shared one.
type CommentTemplateHandler struct {
	entity entityCommentTemplateClient
}

// NewCommentTemplateHandler constructs a CommentTemplateHandler.
func NewCommentTemplateHandler(entity entityCommentTemplateClient) *CommentTemplateHandler {
	return &CommentTemplateHandler{entity: entity}
}

// ListCommentTemplates handles GET /users/me/case-comment-templates.
func (h *CommentTemplateHandler) ListCommentTemplates(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.ListCaseCommentTemplates(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCaseCommentTemplates failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, errMsgCommentTemplatesListFailed)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CreatePersonalCommentTemplate handles POST /users/me/case-comment-templates.
func (h *CommentTemplateHandler) CreatePersonalCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, h.entity.CreatePersonalCaseCommentTemplate, "CreatePersonalCaseCommentTemplate")
}

// UpdatePersonalCommentTemplate handles PATCH /users/me/case-comment-templates/{id}.
func (h *CommentTemplateHandler) UpdatePersonalCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, h.entity.UpdatePersonalCaseCommentTemplate, "UpdatePersonalCaseCommentTemplate")
}

// DeletePersonalCommentTemplate handles DELETE /users/me/case-comment-templates/{id}.
func (h *CommentTemplateHandler) DeletePersonalCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, h.entity.DeletePersonalCaseCommentTemplate, "DeletePersonalCaseCommentTemplate")
}

// CreateSharedCommentTemplate handles POST /case-comment-templates.
func (h *CommentTemplateHandler) CreateSharedCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, h.entity.CreateSharedCaseCommentTemplate, "CreateSharedCaseCommentTemplate")
}

// UpdateSharedCommentTemplate handles PATCH /case-comment-templates/{id}.
func (h *CommentTemplateHandler) UpdateSharedCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, h.entity.UpdateSharedCaseCommentTemplate, "UpdateSharedCaseCommentTemplate")
}

// DeleteSharedCommentTemplate handles DELETE /case-comment-templates/{id}.
func (h *CommentTemplateHandler) DeleteSharedCommentTemplate(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, h.entity.DeleteSharedCaseCommentTemplate, "DeleteSharedCaseCommentTemplate")
}

func (h *CommentTemplateHandler) create(w http.ResponseWriter, r *http.Request, create func(context.Context, []byte) ([]byte, error), op string) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	result, err := create(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, errMsgCommentTemplateSaveFailed)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *CommentTemplateHandler) update(w http.ResponseWriter, r *http.Request, update func(context.Context, string, []byte) ([]byte, error), op string) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := commentTemplateID(w, r)
	if !ok {
		return
	}
	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	result, err := update(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "templateId", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, errMsgCommentTemplateSaveFailed)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CommentTemplateHandler) delete(w http.ResponseWriter, r *http.Request, del func(context.Context, string) error, op string) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := commentTemplateID(w, r)
	if !ok {
		return
	}
	if err := del(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "templateId", id, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, errMsgCommentTemplateDeleteFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// commentTemplateID reads the {id} path value; on a bad one it has written
// the 400 and returns false.
func commentTemplateID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", false
	}
	return strings.ToLower(id), true
}
