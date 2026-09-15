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
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// scimClient abstracts the SCIM service operations used by UsersHandler,
// allowing the handler to be tested independently of the real HTTP client.
type scimClient interface {
	SearchUser(ctx context.Context, email string) (*scim.UserInfo, error)
	SearchExternalUser(ctx context.Context, email string) (*scim.ExternalUserInfo, error)
	UpdateUserPhone(ctx context.Context, userID, mobile string) (*string, error)
}

// entityUserClient abstracts the entity service user operations used by UsersHandler.
type entityUserClient interface {
	GetUserMe(ctx context.Context) ([]byte, error)
	PatchUserMe(ctx context.Context, body []byte) ([]byte, error)
	SearchUsers(ctx context.Context, body []byte) ([]byte, error)
	GetUser(ctx context.Context, id string) ([]byte, error)
	GetUsersByIDs(ctx context.Context, body []byte) ([]byte, error)
}

// UsersHandler handles HTTP requests for user-related operations.
type UsersHandler struct {
	scim   scimClient
	entity entityUserClient
	// dir is the startup-resolved team registry and role allow-list. Every
	// team key <-> group name translation on this handler's paths is a lookup
	// in it, never an upstream call: the entity service does not hold the
	// registry, so it can only answer membership questions when this layer
	// hands it the group names to ask about.
	dir *directory.Directory
	// sftpgoAttachmentStorageEnabled mirrors SFTPGO_ATTACHMENT_STORAGE_ENABLED
	// (see cmd/server/main.go), surfaced on GET /users/me so the frontend can
	// tell whether AttachmentStorageHandler's routes are reachable without
	// probing them.
	sftpgoAttachmentStorageEnabled bool
}

// NewUsersHandler creates a UsersHandler backed by the given SCIM and entity
// clients and the startup-resolved directory. sftpgoAttachmentStorageEnabled
// mirrors the same runtime flag value main.go uses to decide whether to
// register AttachmentStorageHandler's routes (SFTPGO_ATTACHMENT_STORAGE_ENABLED),
// so GET /users/me can tell the frontend whether those routes are reachable.
func NewUsersHandler(scim scimClient, entity entityUserClient, dir *directory.Directory, sftpgoAttachmentStorageEnabled bool) *UsersHandler {
	return &UsersHandler{scim: scim, entity: entity, dir: dir, sftpgoAttachmentStorageEnabled: sftpgoAttachmentStorageEnabled}
}

// userMeResponse is the GET /users/me response shape.
type userMeResponse struct {
	ID          *string           `json:"id,omitempty"`
	Email       string            `json:"email"`
	FirstName   *string           `json:"firstName,omitempty"`
	LastName    *string           `json:"lastName,omitempty"`
	TimeZone    *string           `json:"timeZone,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	PhoneNumber *string           `json:"phoneNumber,omitempty"`
	Team        *userTeamResponse `json:"team,omitempty"`
	// SftpgoAttachmentStorageEnabled mirrors the backend's
	// SFTPGO_ATTACHMENT_STORAGE_ENABLED runtime flag. Always present (never
	// omitted) so the frontend can distinguish "flag is off" from "field not
	// yet known to this backend version" only by absence on an old backend —
	// a new field an existing caller ignores, so this is backward compatible.
	SftpgoAttachmentStorageEnabled bool `json:"sftpgoAttachmentStorageEnabled"`
}

// userTeamResponse is the caller's resolved ABT (Account-Based Team). Nil when
// the caller belongs to no group in the team registry, or when the upstream
// membership lookup failed (best-effort, never fails the identity response).
type userTeamResponse struct {
	TeamKey  string `json:"teamKey"`
	TeamName string `json:"teamName"`
	// Family is omitted, not empty, when the team is unclassified: not every
	// ABT team is classified into a family, and "" is not one of the four
	// values the contract's enum permits.
	Family string `json:"family,omitempty"`
}

// entityGroupRef is one group the entity service reports the caller as a member
// of. Membership is live state, so it is the one part of team resolution that
// still costs an upstream call -- the registry that turns a group name into a
// team is resolved here at startup.
type entityGroupRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// entityUserMeResponse is the subset of the entity GET /users/me response we care about.
type entityUserMeResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName"`
	LastName  string   `json:"lastName"`
	TimeZone  *string  `json:"timeZone"`
	Roles     []string `json:"roles"`
	// Groups is every group the caller belongs to, or absent when the upstream
	// membership lookup failed. The team is derived from it here rather than
	// upstream, since the registry lives in this service.
	Groups []entityGroupRef `json:"groups"`
}

// userUpdateRequest is the PATCH /users/me request shape.
type userUpdateRequest struct {
	PhoneNumber *string `json:"phoneNumber,omitempty"`
	TimeZone    *string `json:"timeZone,omitempty"`
}

// userUpdateResponse is the PATCH /users/me response shape.
type userUpdateResponse struct {
	PhoneNumber *string `json:"phoneNumber,omitempty"`
	TimeZone    *string `json:"timeZone,omitempty"`
}

// GetMe handles GET /users/me.
// id, firstName, lastName, timeZone, and roles are sourced from the entity service.
// phoneNumber is sourced from SCIM.
func (h *UsersHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	resp := userMeResponse{
		Email:                          user.Email,
		SftpgoAttachmentStorageEnabled: h.sftpgoAttachmentStorageEnabled,
	}

	entityRaw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe failed", "userID", user.UserID, "err", err)
		// A caller cannot distinguish "no roles/team" from "upstream identity
		// resolution failed" if this falls through to a 200 with zeroed
		// fields, so the failure must surface as an error response.
		mapUpstreamErrorGeneric(w, err, "Failed to fetch the current user.")
		return
	}

	var entityResp entityUserMeResponse
	if jsonErr := json.Unmarshal(entityRaw, &entityResp); jsonErr != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe: parse response failed", "userID", user.UserID, "err", jsonErr)
	} else {
		resp.ID = &entityResp.ID
		resp.FirstName = entityResp.FirstName
		resp.LastName = &entityResp.LastName
		resp.TimeZone = entityResp.TimeZone
		if entityResp.Roles != nil {
			resp.Roles = entityResp.Roles
		}
		resp.Team = h.teamForGroups(entityResp.Groups)
	}

	scimInfo, err := h.scim.SearchUser(r.Context(), user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "scim SearchUser failed", "userID", user.UserID, "err", err)
	} else if scimInfo == nil {
		slog.WarnContext(r.Context(), "no SCIM user found", "userID", user.UserID)
	} else {
		resp.PhoneNumber = scimInfo.PhoneNumber
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// PatchMe handles PATCH /users/me.
// phoneNumber update is handled via SCIM; timeZone update is handled via the entity service.
func (h *UsersHandler) PatchMe(w http.ResponseWriter, r *http.Request) {
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

	if len(bytes.TrimSpace(body)) == 0 {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
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

	resp := userUpdateResponse{}

	if payload.PhoneNumber != nil {
		updatedPhone, err := h.scim.UpdateUserPhone(r.Context(), user.UserID, *payload.PhoneNumber)
		if err != nil {
			slog.ErrorContext(r.Context(), "scim UpdateUserPhone failed", "userID", user.UserID, "err", err)
			mapUpstreamError(w, err, "Failed to update phone number.")
			return
		}
		resp.PhoneNumber = updatedPhone
	}

	if payload.TimeZone != nil {
		patchBody, marshalErr := json.Marshal(map[string]string{"timeZone": *payload.TimeZone})
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update time zone.")
			return
		}
		if _, entityErr := h.entity.PatchUserMe(r.Context(), patchBody); entityErr != nil {
			slog.ErrorContext(r.Context(), "entity PatchUserMe failed", "userID", user.UserID, "err", entityErr)
			mapUpstreamError(w, entityErr, "Failed to update time zone.")
			return
		}
		resp.TimeZone = payload.TimeZone
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// SearchUsers handles POST /users/search.
func (h *UsersHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
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

	// The registry lives here, so the teamIds filter is resolved here: the
	// entity service is handed group names it can run a membership query with.
	body, err = h.resolveUserSearchFilters(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.entity.SearchUsers(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchUsers failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search users.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetUser handles GET /users/{id}.
//
// Returns one user's profile including their group and team membership, and for external
// contacts their per-project access. Registered after /users/me, which is the more specific
// pattern and therefore still wins for that exact path.
func (h *UsersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetUser(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUser failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to fetch the user.")
		return
	}

	// The upstream response carries the user's groups; which of those are
	// registry teams is this service's knowledge, so the teams block is added
	// here. Best-effort, matching every other enrichment on this profile: a
	// response we cannot re-shape is still worth returning as-is.
	enriched, err := h.withUserTeams(result)
	if err != nil {
		slog.WarnContext(r.Context(), "entity GetUser: could not derive team membership",
			"userID", user.UserID, "err", err)
		enriched = result
	}

	// SCIM's "external" org existence/lock check is independent of the teams
	// enrichment above, so a failure in either never blocks the other.
	enriched = h.withExternalAccountStatus(r.Context(), enriched, user.UserID)

	writeJSON(w, http.StatusOK, enriched)
}

// getUsersByIDsRequest is the request body for POST /users/by-ids.
type getUsersByIDsRequest struct {
	IDs []string `json:"ids"`
}

// GetUsersByIDs handles POST /users/by-ids -- resolves a batch of user ids
// to their profiles in one call (e.g. for showing names on a list of
// records that each reference a user by id, rather than looking each one
// up individually).
func (h *UsersHandler) GetUsersByIDs(w http.ResponseWriter, r *http.Request) {
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

	var req getUsersByIDsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	forwardBody, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.GetUsersByIDs(r.Context(), forwardBody)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to look up users.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
