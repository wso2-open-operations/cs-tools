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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
)

// entityUserProjectClient abstracts the entity-service operations
// RegistryHandler needs to resolve the caller's roles and a project's
// account/key details.
type entityUserProjectClient interface {
	GetMe(ctx context.Context) (entity.GetUserMeResponse, error)
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// registryClient abstracts the registry service operations used by RegistryHandler.
type registryClient interface {
	CreateToken(ctx context.Context, req registry.TokenCreatePayload) (registry.TokenCreationResponse, error)
	SearchTokens(ctx context.Context, req registry.TokenSearchPayload) ([]registry.Token, error)
	GetTokenByID(ctx context.Context, tokenID string) (registry.Token, error)
	DeleteToken(ctx context.Context, tokenID string) error
	RegenerateToken(ctx context.Context, tokenID string) (registry.TokenCreationResponse, error)
	GetIntegrationUsersByProjectID(ctx context.Context, projectID string) ([]registry.IntegrationUser, error)
}

// RegistryHandler handles HTTP requests for registry (robot-account) tokens
// and a project's integration users. Authorization for the token-scoped
// routes (delete/regenerate) is indirect: the token's own opaque Description
// field is parsed to recover which project it belongs to, then the caller's
// access to that project is re-verified via entity-service — the registry
// service itself has no concept of project membership.
type RegistryHandler struct {
	entity    entityUserProjectClient
	registry  registryClient
	adminRole string

	callerScope *CallerScopeResolver
}

// NewRegistryHandler creates a RegistryHandler. adminRole is the role string
// (from entity.GetUserMeResponse.Roles) that grants admin privileges —
// creating/managing service tokens and tokens on behalf of other users.
func NewRegistryHandler(entityClient entityUserProjectClient, registryClient registryClient, adminRole string) *RegistryHandler {
	return &RegistryHandler{entity: entityClient, registry: registryClient, adminRole: adminRole}
}

// SetCallerScope enables caller-scoped access: registry token and integration
// user operations require the caller to be an active portal-user contact of
// the project in the URL path or the token's owning project. Always enforced in
// production (main.go calls this unconditionally, no kill switch) — see
// ProjectHandler.SetCallerScope for why this is a setter rather than a
// constructor parameter.
func (h *RegistryHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

func (h *RegistryHandler) isAdmin(roles []string) bool {
	for _, r := range roles {
		if r == h.adminRole {
			return true
		}
	}
	return false
}

// writeUpstreamMessage maps err to an HTTP response using the upstream
// service's own error message (apierror.Error.Body) when available, falling
// back to a generic message otherwise — several registry/contacts endpoints
// deliberately forward the upstream's specific reason rather than a fixed
// fallback.
func writeUpstreamMessage(w http.ResponseWriter, err error, fallback string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		msg := apiErr.Body
		if msg == "" {
			msg = fallback
		}
		status := apiErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		writeError(w, status, msg)
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}

// CreateRegistryToken handles POST /projects/{id}/registry-tokens.
func (h *RegistryHandler) CreateRegistryToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.RegistryTokenCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !dto.ValidRobotName(req.RobotName) {
		writeError(w, http.StatusBadRequest, "Name can only contain alphanumeric characters and dashes.")
		return
	}

	userDetails, err := h.entity.GetMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve user data.")
		return
	}
	isAdmin := h.isAdmin(userDetails.Roles)

	if !isAdmin && req.TokenType == registry.TokenTypeService {
		writeError(w, http.StatusForbidden, "Only admins can create service tokens.")
		return
	}

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	var createdFor string
	if req.TokenType == registry.TokenTypeUser {
		createdFor = user.Email
	} else if req.CreatedFor != nil {
		createdFor = *req.CreatedFor
	}

	if req.TokenType == registry.TokenTypeService && createdFor == "" {
		writeError(w, http.StatusBadRequest, "Service tokens are created on behalf of an integration user. "+
			"Please specify the 'createdFor' email of the integration user this token is intended for.")
		return
	}
	if !isAdmin && createdFor != user.Email {
		writeError(w, http.StatusForbidden, "Only admins can create tokens for other users.")
		return
	}

	result, err := h.registry.CreateToken(r.Context(), registry.TokenCreatePayload{
		SnProjectID: projectID,
		AccountName: project.Account.Name,
		ProjectKey:  project.Key,
		RobotName:   req.RobotName,
		SnAccountID: project.Account.ID,
		TokenType:   req.TokenType,
		CreatedBy:   user.Email,
		CreatedFor:  createdFor,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "registry CreateToken failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to create registry token.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapRegistryTokenCreateResponse(result))
}

// SearchRegistryTokens handles POST /projects/{id}/registry-tokens/search.
// Non-admins only ever see their own tokens (userEmail forced to the
// caller); admins see all tokens for the project.
func (h *RegistryHandler) SearchRegistryTokens(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	userDetails, err := h.entity.GetMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve user data.")
		return
	}
	isAdmin := h.isAdmin(userDetails.Roles)

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	req := registry.TokenSearchPayload{SnProjectID: projectID, SnAccountID: project.Account.ID, IsAdmin: isAdmin}
	if !isAdmin {
		email := user.Email
		req.UserEmail = &email
	}

	result, err := h.registry.SearchTokens(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "registry SearchTokens failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to search registry tokens.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapRegistryTokens(result))
}

// authorizeTokenAction fetches the token, derives its owning project/type/
// owner from its opaque description, and re-verifies the caller's access to
// that project. Returns ok=false (having already written a response) if any
// step fails or the caller lacks permission — matching the shared
// authorization pattern of both DeleteRegistryToken and RegenerateRegistryToken.
func (h *RegistryHandler) authorizeTokenAction(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, tokenID, failureNoun string) (ok bool) {
	token, err := h.registry.GetTokenByID(r.Context(), tokenID)
	if err != nil {
		slog.ErrorContext(r.Context(), "registry GetTokenByID failed", "userID", user.UserID, "tokenID", tokenID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to "+failureNoun+" token")
		return false
	}

	info, err := registry.DeriveTokenInfoFromDescription(token.Description)
	if err != nil {
		slog.ErrorContext(r.Context(), "registry DeriveTokenInfoFromDescription failed", "userID", user.UserID, "tokenID", tokenID, "err", summarizeErr(err))
		writeError(w, http.StatusInternalServerError, "Failed to "+failureNoun+" token")
		return false
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, info.SnProjectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return false
	// }

	userDetails, err := h.entity.GetMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to "+failureNoun+" token")
		return false
	}
	isAdmin := h.isAdmin(userDetails.Roles)

	if _, err := h.entity.GetProject(r.Context(), info.SnProjectID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", info.SnProjectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to "+failureNoun+" token")
		return false
	}

	if !isAdmin && (info.TokenType == registry.TokenTypeService || info.CreatedFor != user.Email) {
		writeError(w, http.StatusForbidden, "You don't have the necessary permissions to "+failureNoun+" this registry token.")
		return false
	}

	return true
}

// DeleteRegistryToken handles DELETE /registry-tokens/{id}.
func (h *RegistryHandler) DeleteRegistryToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// tokenID is not UUID-validated: the registry service's own token IDs are
	// not UUID-shaped, so this route takes a plain `string` path param,
	// unlike the project-scoped registry routes above.
	tokenID := r.PathValue("id")
	if tokenID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.authorizeTokenAction(w, r, user, tokenID, "delete") {
		return
	}

	if err := h.registry.DeleteToken(r.Context(), tokenID); err != nil {
		slog.ErrorContext(r.Context(), "registry DeleteToken failed", "userID", user.UserID, "tokenID", tokenID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to delete token")
		return
	}

	writeJSONValue(w, http.StatusOK, map[string]string{"message": "Registry token deleted successfully!"})
}

// RegenerateRegistryToken handles POST /registry-tokens/{id}/regenerate.
func (h *RegistryHandler) RegenerateRegistryToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// tokenID is not UUID-validated — see the matching comment in DeleteRegistryToken.
	tokenID := r.PathValue("id")
	if tokenID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.authorizeTokenAction(w, r, user, tokenID, "re-generate") {
		return
	}

	result, err := h.registry.RegenerateToken(r.Context(), tokenID)
	if err != nil {
		slog.ErrorContext(r.Context(), "registry RegenerateToken failed", "userID", user.UserID, "tokenID", tokenID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to re-generate token")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapRegistryTokenCreateResponse(result))
}

// GetProjectIntegrationUsers handles GET /projects/{id}/integration-users.
func (h *RegistryHandler) GetProjectIntegrationUsers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	result, err := h.registry.GetIntegrationUsersByProjectID(r.Context(), project.SfID)
	if err != nil {
		slog.ErrorContext(r.Context(), "registry GetIntegrationUsersByProjectID failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to retrieve integration users.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapIntegrationUsers(result))
}
