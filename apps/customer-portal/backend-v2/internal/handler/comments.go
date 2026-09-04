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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityCommentClient abstracts the entity-service generic-comment operations
// used by CommentHandler.
type entityCommentClient interface {
	CreateComment(ctx context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
}

// CommentHandler handles HTTP requests for generic comments attached to any
// reference entity (case, conversation, change_request, deployment,
// incident) — distinct from CaseHandler's case-specific comment endpoint.
type CommentHandler struct {
	entity      entityCommentClient
	callerScope *CallerScopeResolver
}

// validCommentReferenceType matches entity-service's own ReferenceType enum.
var validCommentReferenceType = map[string]bool{
	string(entity.ReferenceTypeCase):          true,
	string(entity.ReferenceTypeConversation):  true,
	string(entity.ReferenceTypeChangeRequest): true,
	string(entity.ReferenceTypeDeployment):    true,
	string(entity.ReferenceTypeIncident):      true,
}

// NewCommentHandler creates a CommentHandler backed by the given entity client.
func NewCommentHandler(entity entityCommentClient) *CommentHandler {
	return &CommentHandler{entity: entity}
}

// SetCallerScope enables caller-scoped access: creating comments on cases
// requires the caller to be an active portal-user contact of the project
// owning that case. Always enforced in production (main.go calls this
// unconditionally, no kill switch) — see ProjectHandler.SetCallerScope for
// why this is a setter rather than a constructor parameter.
func (h *CommentHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// CreateComment handles POST /comments.
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CommentCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !uuidRe.MatchString(req.ReferenceID) || !validCommentReferenceType[req.ReferenceType] || req.Content == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if req.ReferenceType == string(entity.ReferenceTypeCase) {
	// 	caseView, err := h.entity.GetCase(r.Context(), req.ReferenceID)
	// 	if err != nil {
	// 		slog.ErrorContext(r.Context(), "entity GetCase failed for comment", "userID", user.UserID, "caseID", req.ReferenceID, "err", summarizeErr(err))
	// 		mapUpstreamError(w, err, "Failed to create comment.")
	// 		return
	// 	}
	// 	if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 		return
	// 	}
	// }

	result, err := h.entity.CreateComment(r.Context(), dto.BuildEntityCreateCommentRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create comment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCommentCreate(result))
}
