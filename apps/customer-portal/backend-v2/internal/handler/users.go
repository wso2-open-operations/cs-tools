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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/scim"
)

// entityUserClient abstracts the entity-service user operations used by UserHandler.
type entityUserClient interface {
	GetMe(ctx context.Context) (entity.GetUserMeResponse, error)
	PatchMe(ctx context.Context, req entity.PatchUserMeRequest) (entity.PatchUserMeResponse, error)
	// RegisterInvitedMemberships completes the caller's onboarding in
	// Salesforce and the CSM database. Only called when firstAccessEnabled
	// is set.
	RegisterInvitedMemberships(ctx context.Context) error
}

// scimUserClient abstracts the SCIM operations used by UserHandler.
type scimUserClient interface {
	SearchUser(ctx context.Context, email string) (*scim.UserInfo, error)
	UpdateUserPhone(ctx context.Context, userID, mobile string) (*string, error)
}

// UserHandler handles HTTP requests for the logged-in user's own profile.
// Profile fields (name, timezone, roles) are sourced from entity-service;
// phone number is sourced from SCIM.
type UserHandler struct {
	entity entityUserClient
	scim   scimUserClient
	// firstAccessEnabled is CSM_MIGRATION_FIRST_ACCESS_ENABLED. It belongs
	// to the ServiceNow-to-CSM cutover and is off in every deployment until
	// that day: off means GetMe behaves exactly as it always has, because
	// the block guarded by it is never entered.
	firstAccessEnabled bool
}

// NewUserHandler creates a UserHandler backed by the given entity and SCIM
// clients. firstAccessEnabled turns on the post-sign-in onboarding call; see
// UserHandler.firstAccessEnabled.
func NewUserHandler(entity entityUserClient, scim scimUserClient, firstAccessEnabled bool) *UserHandler {
	return &UserHandler{entity: entity, scim: scim, firstAccessEnabled: firstAccessEnabled}
}

// firstAccessTimeout bounds the background onboarding call. It runs after the
// response has been written, on a context detached from the request, so the
// request's own cancellation must not kill it and it must not run forever.
const firstAccessTimeout = 20 * time.Second

// userUpdateRequest is the PATCH /users/me request shape. At least one field
// must be set.
type userUpdateRequest struct {
	PhoneNumber *string `json:"phoneNumber,omitempty"`
	TimeZone    *string `json:"timeZone,omitempty"`
}

// GetMe handles GET /users/me.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// Register a first sign-in's invited memberships before reading the profile, so
	// this response and the requests after it already see the REGISTERED state.
	if h.firstAccessEnabled {
		h.completeFirstAccess(r.Context(), user.UserID)
	}

	result, err := h.entity.GetMe(r.Context())
	if err != nil {
		// entity-service 404s GetMe when the caller's email has no "user" row
		// at all -- a real, reported case (an authenticated JWT whose identity
		// was never provisioned downstream). A bare 404 reaching the webapp
		// here isn't "page not found" the way it is for a resource id in a
		// URL; the frontend's data-fetching hook had nothing to render and
		// nothing resembling the 403 state it already knows how to show, so
		// it spun forever instead. Map it to 403 (the already-handled "you
		// don't have permission" case) rather than passing a 404 through
		// that the caller can't act on and the UI doesn't expect for this
		// endpoint. The real reason is still logged, at ERROR specifically
		// so it's not lost alongside routine 403s.
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			slog.ErrorContext(r.Context(), "entity GetMe: user not found", "userID", user.UserID)
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve user profile.")
		return
	}

	resp := dto.MapUserMe(result)

	// Phone number is sourced from SCIM, not entity-service. A SCIM lookup
	// failure or miss is not fatal to the request — the profile is still
	// useful without a phone number.
	scimInfo, scimErr := h.scim.SearchUser(r.Context(), user.Email)
	if scimErr != nil {
		slog.ErrorContext(r.Context(), "scim SearchUser failed", "userID", user.UserID, "err", summarizeErr(scimErr))
	} else if scimInfo == nil {
		slog.WarnContext(r.Context(), "no SCIM user found", "userID", user.UserID)
	} else {
		resp.PhoneNumber = scimInfo.PhoneNumber
		resp.LastPasswordUpdateTime = scimInfo.LastPasswordUpdateTime
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// completeFirstAccess runs the onboarding call described in GetMe. It never
// returns anything: its only possible outcome for the caller is a log line.
func (h *UserHandler) completeFirstAccess(ctx context.Context, userID string) {
	ctx, cancel := context.WithTimeout(ctx, firstAccessTimeout)
	defer cancel()
	if err := h.entity.RegisterInvitedMemberships(ctx); err != nil {
		// Not an error the user can act on, and not one that should page
		// anyone: the Salesforce event that follows an invitation reaches
		// entity-service by its own path as well.
		slog.WarnContext(ctx, "entity RegisterInvitedMemberships failed", "userID", userID, "err", summarizeErr(err))
	}
}

// PatchMe handles PATCH /users/me. phoneNumber is updated via SCIM first;
// the phone number SCIM stored (or the requested one if SCIM returns none) and
// any timeZone are then mirrored to entity-service in a single PATCH. At least
// one field must be provided. If SCIM fails entity-service is not called; if
// the entity call fails the request fails, and a retry is safe because the
// SCIM update is idempotent.
func (h *UserHandler) PatchMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var payload userUpdateRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.PhoneNumber == nil && payload.TimeZone == nil {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}

	if payload.TimeZone != nil && strings.TrimSpace(*payload.TimeZone) == "" {
		writeError(w, http.StatusBadRequest, "timeZone must not be empty.")
		return
	}

	resp := dto.UserUpdateResponse{}

	entityReq := entity.PatchUserMeRequest{TimeZone: payload.TimeZone}

	if payload.PhoneNumber != nil {
		updatedPhone, err := h.scim.UpdateUserPhone(r.Context(), user.UserID, *payload.PhoneNumber)
		if err != nil {
			slog.ErrorContext(r.Context(), "scim UpdateUserPhone failed", "userID", user.UserID, "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to update phone number.")
			return
		}
		resp.PhoneNumber = updatedPhone
		stored := payload.PhoneNumber
		if updatedPhone != nil {
			stored = updatedPhone
		}
		entityReq.Phone = stored
	}

	if entityReq.Phone != nil || entityReq.TimeZone != nil {
		if _, err := h.entity.PatchMe(r.Context(), entityReq); err != nil {
			slog.ErrorContext(r.Context(), "entity PatchMe failed", "userID", user.UserID,
				"phoneUpdated", entityReq.Phone != nil, "timeZoneUpdated", entityReq.TimeZone != nil, "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to update profile.")
			return
		}
		resp.TimeZone = payload.TimeZone
	}

	writeJSONValue(w, http.StatusOK, resp)
}
