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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
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
	GetRole(ctx context.Context, roleID string) ([]scim.RoleMember, error)
	// GetUserByID is used by withPortalRoles to resolve a target user's held
	// roles AND group memberships by their own Asgardeo ID (see that method's
	// own doc comment for why -- two distinct roles, or two distinct groups,
	// can share a display name).
	GetUserByID(ctx context.Context, userID string) (scim.UserAssignments, error)
	// AddRoleMembers grants a role to one or more users by email -- used by
	// CreateUser to grant each of the caller's requested grantRoles once the
	// new platform user exists.
	AddRoleMembers(ctx context.Context, roleID string, emails []string) error
}

// entityUserClient abstracts the entity service user operations used by UsersHandler.
type entityUserClient interface {
	GetUserMe(ctx context.Context) ([]byte, error)
	PatchUserMe(ctx context.Context, body []byte) ([]byte, error)
	SearchUsers(ctx context.Context, body []byte) ([]byte, error)
	GetUser(ctx context.Context, id string) ([]byte, error)
	GetUsersByIDs(ctx context.Context, body []byte) ([]byte, error)
	CreateUser(ctx context.Context, body []byte) ([]byte, error)
	ListSavedFilterViews(ctx context.Context, listKey string) ([]byte, error)
	SaveSavedFilterView(ctx context.Context, body []byte) ([]byte, error)
	DeleteSavedFilterView(ctx context.Context, listKey, name string) ([]byte, error)
	ReorderSavedFilterView(ctx context.Context, body []byte) ([]byte, error)
	UpdateUser(ctx context.Context, id string, body []byte) ([]byte, error)
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
	// timecardApproverRoleIDs are every real role ID (see
	// handler.RoleIDsForKey) GET /users/time-card-approvers fetches via SCIM
	// and merges. Configured once, out of band -- see that handler's own doc
	// comment for why this is the real, authoritative list of approvers, not
	// entity-service's own Postgres role table. More than one ID is possible:
	// AUTH_TIMECARD_APPROVER_ROLES can name several real role names, each
	// resolved to its own ID, and an approver holding only one of them must
	// still be listed.
	timecardApproverRoleIDs []string
	// access resolves the caller's token roles into the portal roles GET
	// /users/me reports. nil (every existing call site and test) reports none;
	// cmd/server/main.go sets it with WithAccessGuard.
	access *AccessGuard
	// grantableRoles is which portal roles CreateUser may grant via SCIM (see
	// ResolveGrantableRoles), each already resolved to its real role ID.
	// nil/empty (every existing call site and test) means no grantRoles value
	// is ever valid -- cmd/server/main.go sets it with WithGrantableRoles.
	grantableRoles []GrantableRole
}

// WithGrantableRoles makes CreateUser able to grant the given portal roles
// via SCIM when a caller's grantRoles field names one, and makes
// GetGrantableRoles (a separate handler, same resolved list) report them to
// the webapp. Returns h for chaining at the construction site.
func (h *UsersHandler) WithGrantableRoles(roles []GrantableRole) *UsersHandler {
	h.grantableRoles = roles
	return h
}

// WithAccessGuard makes GET /users/me report the portal roles the caller's
// token roles grant, using the same guard that authorises every route, so
// what the frontend is told and what the backend enforces cannot disagree.
// Returns h for chaining at the construction site.
func (h *UsersHandler) WithAccessGuard(g *AccessGuard) *UsersHandler {
	h.access = g
	return h
}

// NewUsersHandler creates a UsersHandler backed by the given SCIM and entity
// clients and the startup-resolved directory. sftpgoAttachmentStorageEnabled
// mirrors the same runtime flag value main.go uses to decide whether to
// register AttachmentStorageHandler's routes (SFTPGO_ATTACHMENT_STORAGE_ENABLED),
// so GET /users/me can tell the frontend whether those routes are reachable.
// timecardApproverRoleIDs is GetTimeCardApprovers' own config -- see that
// handler's doc comment; pass nil/empty when GET /users/time-card-approvers
// is not registered (main.go only registers it once this is non-empty).
func NewUsersHandler(scim scimClient, entity entityUserClient, dir *directory.Directory, sftpgoAttachmentStorageEnabled bool, timecardApproverRoleIDs []string) *UsersHandler {
	return &UsersHandler{
		scim:                           scim,
		entity:                         entity,
		dir:                            dir,
		sftpgoAttachmentStorageEnabled: sftpgoAttachmentStorageEnabled,
		timecardApproverRoleIDs:        timecardApproverRoleIDs,
	}
}

// userMeResponse is the GET /users/me response shape.
type userMeResponse struct {
	// ID, TimeZone and Team come from the entity service; they are null (not
	// omitted) when the caller holds portal roles but has no user row there.
	ID        *string `json:"id"`
	Email     string  `json:"email"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	TimeZone  *string `json:"timeZone"`
	// Roles is which portal roles (viewer, cs_engineer, admin, ...) the
	// caller's token roles grant: several are possible. It is not the entity
	// service's role data, which this response no longer carries. Always
	// present, [] when they hold none.
	Roles []string `json:"roles"`
	// PhoneNumber is the phone on the caller's platform user record. It is never
	// read from the identity provider. Omitted when the record has none.
	PhoneNumber *string           `json:"phoneNumber,omitempty"`
	Team        *userTeamResponse `json:"team"`
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
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName *string `json:"firstName"`
	LastName  string  `json:"lastName"`
	TimeZone  *string `json:"timeZone"`
	// Phone is "user".phone; the entity service omits it when NULL. It is the
	// source of GET /users/me phoneNumber.
	Phone *string `json:"phone"`
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
// id, firstName, lastName, timeZone, and phoneNumber (the platform user record's
// phone) are sourced from the entity service; roles from the caller's token. The
// phone saved with the identity provider is never read.
func (h *UsersHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	resp := userMeResponse{
		Email:                          user.Email,
		SftpgoAttachmentStorageEnabled: h.sftpgoAttachmentStorageEnabled,
		Roles:                          []string{},
	}
	if h.access != nil {
		resp.Roles = h.access.RolesFor(user.Roles)
	}

	entityRaw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		// entity-service 404s GetUserMe when the caller's email has no "user"
		// row at all -- a real, reported case (an authenticated JWT whose
		// identity was never provisioned downstream). A bare 404 reaching the
		// webapp here isn't "page not found" the way it is for a resource id
		// in a URL; the frontend's data-fetching hook had nothing to render
		// and nothing resembling the 403 state it already knows how to show,
		// so it spun forever instead. Map it to 403 (the already-handled
		// "you don't have permission" case) rather than passing a 404
		// through that the caller can't act on and the UI doesn't expect
		// for this endpoint. The real reason is still logged, at ERROR
		// specifically so it's not lost alongside routine 403s.
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			slog.ErrorContext(r.Context(), "entity GetUserMe: user not found", "userID", user.UserID)
			if len(resp.Roles) == 0 {
				writeError(w, http.StatusForbidden, ErrMsgForbidden)
				return
			}
			// A cs_engineer caller with no user row yet gets one created here,
			// on the spot, from SCIM -- see provisionCsEngineerUser's own doc
			// comment. Best-effort: a failure falls through to the existing
			// token-only answer below, same as every other caller.
			if slices.Contains(resp.Roles, "cs_engineer") && h.provisionCsEngineerUser(r.Context(), user, &resp) {
				writeJSONValue(w, http.StatusOK, resp)
				return
			}
			// The caller holds portal roles via the token but has no user row:
			// answer from the token. Entity-sourced fields (id, timeZone,
			// team) stay null.
			if user.FirstName != "" {
				resp.FirstName = &user.FirstName
			}
			if user.LastName != "" {
				resp.LastName = &user.LastName
			}
			writeJSONValue(w, http.StatusOK, resp)
			return
		}
		slog.ErrorContext(r.Context(), "entity GetUserMe failed", "userID", user.UserID, "err", err)
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
		resp.PhoneNumber = entityResp.Phone
		resp.Team = h.teamForGroups(entityResp.Groups)
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// csEngineerLastNameMarker is appended to the token-sourced last name when
// auto-provisioning a cs_engineer caller's "user" row via
// provisionCsEngineerUser, flagging a row created this way rather than
// through the admin Add User flow or a human-entered profile.
const csEngineerLastNameMarker = "Ⓦ"

// withCsEngineerLastNameMarker appends csEngineerLastNameMarker to lastName,
// space-separated -- or returns the bare marker if lastName is empty (the
// token carried no family_name claim).
func withCsEngineerLastNameMarker(lastName string) string {
	lastName = strings.TrimSpace(lastName)
	if lastName == "" {
		return csEngineerLastNameMarker
	}
	return lastName + " " + csEngineerLastNameMarker
}

// provisionCsEngineerUser creates a "user" row, on the spot, for a caller who
// holds the cs_engineer role via their token (GetMe's 404 branch already
// confirmed this) but has no entity-service user row yet. Sources
// firstName/lastName/phone straight off the caller's own validated token
// (middleware.UserInfo -- given_name/family_name/phone_number claims), the
// same token-only posture ensureUserProvisioned (cases.go) already takes, and
// marks the created name by appending csEngineerLastNameMarker to the last
// name, so a row created this way is identifiable later. No SCIM call is
// made here at all -- unlike the admin Add User flow (CreateUser below),
// which has no token for the person it's creating and so has to look them up
// in SCIM by email instead, this caller IS the person being provisioned, so
// their own token already has everything needed.
//
// Best-effort end to end: any failure is logged and reported as not
// provisioned (false), so GetMe falls back to its existing token-only
// answer rather than failing the request outright.
func (h *UsersHandler) provisionCsEngineerUser(ctx context.Context, user *middleware.UserInfo, resp *userMeResponse) bool {
	firstName := strings.TrimSpace(user.FirstName)
	lastName := withCsEngineerLastNameMarker(user.LastName)

	body, err := json.Marshal(struct {
		FirstName string   `json:"firstName"`
		LastName  string   `json:"lastName"`
		Email     string   `json:"email"`
		Roles     []string `json:"roles"`
	}{
		FirstName: firstName,
		LastName:  lastName,
		Email:     user.Email,
		Roles:     []string{"internal"},
	})
	if err != nil {
		slog.ErrorContext(ctx, "provisionCsEngineerUser: marshal create-user request failed", "userID", user.UserID, "err", err)
		return false
	}

	raw, err := h.entity.CreateUser(ctx, body)
	if err != nil {
		slog.ErrorContext(ctx, "provisionCsEngineerUser: entity CreateUser failed", "userID", user.UserID, "err", err)
		return false
	}

	var created entityUserMeResponse
	if err := json.Unmarshal(raw, &created); err != nil {
		slog.ErrorContext(ctx, "provisionCsEngineerUser: parse create-user response failed", "userID", user.UserID, "err", err)
		return false
	}

	// entity-service's own POST /users has no phone field (see "Creating a
	// user" in this backend's CLAUDE.md) -- phone can only be set afterward,
	// via PATCH /users/{id}. Best-effort: the user already exists by this
	// point, so a failed phone patch must not undo the create.
	phone := strings.TrimSpace(user.PhoneNumber)
	if phone != "" && created.ID != "" {
		patchBody, err := json.Marshal(map[string]string{"phone": phone})
		if err != nil {
			slog.ErrorContext(ctx, "provisionCsEngineerUser: marshal phone patch failed", "userID", user.UserID, "err", err)
		} else if _, err := h.entity.UpdateUser(ctx, created.ID, patchBody); err != nil {
			slog.ErrorContext(ctx, "provisionCsEngineerUser: entity UpdateUser (phone) failed", "userID", user.UserID, "err", err)
		}
	}

	resp.ID = &created.ID
	resp.FirstName = &firstName
	resp.LastName = &lastName
	if phone != "" {
		resp.PhoneNumber = &phone
	}
	return true
}

// PatchMe handles PATCH /users/me.
// phoneNumber and timeZone are both written to the platform user record through the
// entity service, in one call when a request carries both. The identity provider is
// never written.
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

	// Entity treats an empty timeZone as absent, so reject it up front rather than
	// report a value that was never stored.
	if payload.TimeZone != nil && strings.TrimSpace(*payload.TimeZone) == "" {
		writeError(w, http.StatusBadRequest, "timeZone must not be empty.")
		return
	}

	resp := userUpdateResponse{}

	// One entity PATCH carrying only the fields this request changed.
	entityFields := map[string]string{}
	var phone *string
	if payload.PhoneNumber != nil {
		// Trimmed here, so the stored and echoed value is what was meant. A
		// blank phone becomes "" and clears it, as an empty one always has.
		trimmed := strings.TrimSpace(*payload.PhoneNumber)
		phone = &trimmed
		entityFields["phone"] = trimmed
	}
	if payload.TimeZone != nil {
		entityFields["timeZone"] = *payload.TimeZone
	}
	if len(entityFields) > 0 {
		errMsg := "Failed to update time zone."
		if payload.PhoneNumber != nil {
			errMsg = "Failed to update phone number."
			if payload.TimeZone != nil {
				errMsg = "Failed to update profile."
			}
		}
		patchBody, marshalErr := json.Marshal(entityFields)
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, errMsg)
			return
		}
		if _, entityErr := h.entity.PatchUserMe(r.Context(), patchBody); entityErr != nil {
			// Never log the body: it may carry the phone number.
			slog.ErrorContext(r.Context(), "entity PatchUserMe failed", "userID", user.UserID, "err", entityErr)
			mapUpstreamError(w, entityErr, errMsg)
			return
		}
		resp.PhoneNumber = phone
		resp.TimeZone = payload.TimeZone
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// updateUserRequest is the body of PATCH /users/{id}. Both fields are
// optional, but at least one must be present.
type updateUserRequest struct {
	TimeZone *string `json:"timeZone"`
	Phone    *string `json:"phone"`
}

// UpdateUser handles PATCH /users/{id} -- restricted to cs_engineer or admin
// via the route's PermWrite permission (cmd/server/main.go, held by exactly
// those two roles), same posture as CreateUser: this handler does not
// re-check the caller's role itself. Unlike PatchMe, which only ever
// resolves the caller's OWN id from their token, this lets one of those
// roles update someone else's profile -- needed once an admin can reassign a
// team's lead (POST /teams/{id}/lead): the incident call-escalation ladder
// dials the lead's own "user".phone, so a newly assigned lead with no number
// on file is a silent paging gap until this is set here.
func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var payload updateUserRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.TimeZone == nil && payload.Phone == nil {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}

	entityFields := map[string]string{}
	if payload.TimeZone != nil {
		entityFields["timeZone"] = *payload.TimeZone
	}
	if payload.Phone != nil {
		entityFields["phone"] = *payload.Phone
	}
	entityBody, err := json.Marshal(entityFields)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	raw, err := h.entity.UpdateUser(r.Context(), id, entityBody)
	if err != nil {
		// Never log the body: it may carry the phone number.
		slog.ErrorContext(r.Context(), "entity UpdateUser failed", "userID", user.UserID, "targetID", id, "err", err)
		mapUpstreamError(w, err, "Failed to update user.")
		return
	}
	writeJSON(w, http.StatusOK, raw)
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

	// Independent of both enrichments above: replaces entity-service's own
	// role vocabulary with the portal-role one, for an internal target only.
	enriched = h.withPortalRoles(r.Context(), enriched, user.UserID)

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

// createUserRequest is the POST /users request shape. Roles is validated
// against the directory's assignable-role allow-list -- entity-service
// deliberately does not validate role names itself (see domain.UserRole's
// own doc comment there), so this is the one place that does -- and is
// otherwise forwarded to the entity service unchanged. GrantRoles is
// portal-only and never reaches entity-service at all (see
// buildEntityCreateUserBody): it names zero or more GrantableRole.Key
// values, each granted via SCIM once the entity service user exists.
type createUserRequest struct {
	FirstName  string   `json:"firstName"`
	LastName   string   `json:"lastName"`
	Email      string   `json:"email"`
	Roles      []string `json:"roles"`
	GrantRoles []string `json:"grantRoles"`
}

// buildEntityCreateUserBody re-encodes req into exactly the fields
// entity-service's own CreateUserRequest expects. entity-service's decoder
// rejects unknown fields, so GrantRoles (meaningless there) cannot be
// forwarded as part of the raw request body the way most of this handler's
// other POST/PATCH bodies are -- same "rebuild from what was actually
// validated" precedent CreateCaseComment's own work_note rebuild follows in
// cases.go.
func buildEntityCreateUserBody(req createUserRequest) ([]byte, error) {
	return json.Marshal(struct {
		FirstName string   `json:"firstName"`
		LastName  string   `json:"lastName"`
		Email     string   `json:"email"`
		Roles     []string `json:"roles"`
	}{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Roles:     req.Roles,
	})
}

// CreateUser handles POST /users. Restricted to admin via the route's
// PermAdmin permission (cmd/server/main.go) — this handler itself only
// validates the request shape, it does not re-check the caller's role.
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req createUserRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	for _, role := range req.Roles {
		if !h.dir.IsValidRole(role) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("roles contains invalid value: %s", role))
			return
		}
	}
	if requestsInternalUserType(req.Roles) && !isWso2Email(req.Email) {
		writeError(w, http.StatusBadRequest, "an internal-type user must have a "+wso2EmailDomain+" email address")
		return
	}
	if requestsExternalUserType(req.Roles) {
		writeError(w, http.StatusBadRequest, "creating an external-type user is not available at this time")
		return
	}
	// A portal (Asgardeo) role only ever resolves for a wso2.com account (see
	// withPortalRoles' own isWso2Email gate on the read side) -- granting one
	// to a non-wso2.com target via grantRoles would add them to a SCIM role in
	// the wrong org with no way for this portal to ever display or act on it
	// again. Checked up front, before anything is created, so the mistake is
	// caught with a clear 400 instead of a silent no-op grant.
	if len(req.GrantRoles) > 0 && !isWso2Email(req.Email) {
		writeError(w, http.StatusBadRequest, "grantRoles can only be granted to a "+wso2EmailDomain+" account")
		return
	}

	// Resolved up front, before anything is created, so an unknown grantRoles
	// key fails fast with a 400 rather than after the entity service user
	// already exists.
	roleIDsToGrant := make([]string, 0, len(req.GrantRoles))
	for _, key := range req.GrantRoles {
		id, ok := RoleIDForKey(h.grantableRoles, key)
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("grantRoles contains invalid value: %s", key))
			return
		}
		roleIDsToGrant = append(roleIDsToGrant, id)
	}

	// The Add User form only collects an email -- when the caller sent no
	// name at all, fill firstName/lastName/phone from a SCIM search by that
	// email instead of asking for them, the same mechanism
	// provisionCsEngineerUser already uses for the cs_engineer auto-create
	// path. A caller that DOES send a name (any other API consumer) is left
	// exactly as before -- this only fires for the email-only case.
	var scimPhone *string
	if strings.TrimSpace(req.FirstName) == "" && strings.TrimSpace(req.LastName) == "" {
		scimInfo, err := h.scim.SearchUser(r.Context(), req.Email)
		if err != nil {
			slog.ErrorContext(r.Context(), "CreateUser: scim SearchUser failed", "userID", user.UserID, "err", err)
			writeError(w, http.StatusBadRequest, "Could not look up this email in the identity provider. Please try again or provide a name.")
			return
		}
		if scimInfo == nil {
			writeError(w, http.StatusBadRequest, "No matching user was found in the identity provider for this email. Please verify the email address.")
			return
		}
		req.FirstName = strings.TrimSpace(scimInfo.FirstName)
		req.LastName = strings.TrimSpace(scimInfo.LastName)
		if req.FirstName == "" && req.LastName == "" {
			writeError(w, http.StatusBadRequest, "The identity provider has no name on file for this email. Please provide a first or last name.")
			return
		}
		scimPhone = scimInfo.PhoneNumber
	}

	entityBody, err := buildEntityCreateUserBody(req)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to build entity CreateUser body", "userID", user.UserID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateUser(r.Context(), entityBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateUser failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create the user.")
		return
	}

	// entity-service's own POST /users has no phone field (see "Creating a
	// user" in this backend's CLAUDE.md), so a SCIM-sourced phone number is
	// set with a second, best-effort call -- the user already exists by this
	// point, so a failed phone patch must not undo the create.
	if scimPhone != nil {
		var created entityUserMeResponse
		if err := json.Unmarshal(result, &created); err != nil {
			slog.ErrorContext(r.Context(), "CreateUser: parse create-user response failed", "userID", user.UserID, "err", err)
		} else if created.ID != "" {
			patchBody, err := json.Marshal(map[string]string{"phone": *scimPhone})
			if err != nil {
				slog.ErrorContext(r.Context(), "CreateUser: marshal phone patch failed", "userID", user.UserID, "err", err)
			} else if _, err := h.entity.UpdateUser(r.Context(), created.ID, patchBody); err != nil {
				slog.ErrorContext(r.Context(), "CreateUser: entity UpdateUser (phone) failed", "userID", user.UserID, "err", err)
			}
		}
	}

	// Best-effort: the platform user already exists by this point, so a SCIM
	// failure must not be reported as a failed create -- it's logged instead,
	// the same posture ensureUserProvisioned (cases.go) takes for the
	// opposite direction of this same mechanism.
	for _, roleID := range roleIDsToGrant {
		if err := h.scim.AddRoleMembers(r.Context(), roleID, []string{req.Email}); err != nil {
			slog.ErrorContext(r.Context(), "scim AddRoleMembers failed", "userID", user.UserID, "roleID", roleID, "err", err)
		}
	}

	writeJSON(w, http.StatusCreated, result)
}

// grantUserRolesRequest is the POST /users/{id}/roles request shape -- the
// post-creation counterpart to createUserRequest's own GrantRoles field, for
// granting one or more portal permission roles (cs_engineer, escalator, ...)
// to a user who already exists, rather than only at the moment a new one is
// created. Each entry is a GrantableRole.Key, the same vocabulary GET
// /roles/grantable reports and POST /users' own grantRoles field accepts.
type grantUserRolesRequest struct {
	Roles []string `json:"roles"`
}

// grantUserRolesResponse reports which of the requested roles were actually
// granted via SCIM, and which were not -- a role can fail independently of
// the others (e.g. one SCIM role write times out while the rest succeed), so
// a single boolean success/failure would hide a partial result. Failed is
// omitted entirely on full success.
type grantUserRolesResponse struct {
	Granted []string `json:"granted"`
	Failed  []string `json:"failed,omitempty"`
}

// GrantUserRoles handles POST /users/{id}/roles: grants one or more portal
// permission roles (the same vocabulary GET /roles/grantable reports) to an
// existing platform user via SCIM. Restricted to admin via the route's
// PermAdmin permission (cmd/server/main.go) -- the same gate POST /users'
// own grantRoles field sits behind, since granting a role is exactly as
// sensitive whether it happens at creation time or afterward.
//
// Refused with a 400 when the target's email isn't a wso2.com address (see
// the isWso2Email check below) -- same guard CreateUser's own grantRoles
// field has, for the same reason: a portal role is meaningless for an
// account outside the wso2.com SCIM org, so granting one there would be a
// silent no-op the admin would have no way to notice.
//
// Unlike CreateUser's own grant step, a SCIM failure here is never swallowed
// as best-effort: granting the role is this endpoint's entire purpose, so a
// caller that asked for it must be told whether it actually happened rather
// than seeing a 200 that silently granted nothing. Every requested role is
// still attempted even if an earlier one fails, so one bad role never blocks
// the rest -- the response reports exactly which succeeded and which didn't
// (200 if all granted, 502 if none did, 207 Multi-Status for a mixed result).
func (h *UsersHandler) GrantUserRoles(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req grantUserRolesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if len(req.Roles) == 0 {
		writeError(w, http.StatusBadRequest, "roles must name at least one role")
		return
	}

	// Resolved up front, before the target user is even looked up, so an
	// unknown key fails fast with a 400 -- same precedent as CreateUser's own
	// grantRoles resolution. RoleIDForKey (not the plural RoleIDsForKey),
	// also matching CreateUser: granting "the escalator role" means one
	// specific real role, not every real role configured for that key.
	roleIDs := make(map[string]string, len(req.Roles))
	for _, key := range req.Roles {
		roleID, ok := RoleIDForKey(h.grantableRoles, key)
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("roles contains invalid value: %s", key))
			return
		}
		roleIDs[key] = roleID
	}

	// SCIM grants by email, not platform user id, so the target's email has
	// to be resolved first -- entity-service's own GetUser response already
	// carries it (UserDetail.Email), the same field entityUserMeResponse
	// already decodes for GetMe/CreateUser.
	target, err := h.entity.GetUser(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "GrantUserRoles: entity GetUser failed", "userID", user.UserID, "targetID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to look up the user.")
		return
	}
	var targetUser entityUserMeResponse
	if err := json.Unmarshal(target, &targetUser); err != nil || targetUser.Email == "" {
		slog.ErrorContext(r.Context(), "GrantUserRoles: parse target user failed", "userID", user.UserID, "targetID", id, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	// Same guard as CreateUser's own grantRoles field: a portal role only
	// ever resolves for a wso2.com account (withPortalRoles' isWso2Email
	// gate), so granting one to a non-wso2.com target would add them to a
	// SCIM role in the wrong org with no way for this portal to ever display
	// or act on it again.
	if !isWso2Email(targetUser.Email) {
		writeError(w, http.StatusBadRequest, "portal roles can only be granted to a "+wso2EmailDomain+" account")
		return
	}

	// granted starts as a non-nil empty slice (not var granted []string) so
	// json.Marshal writes "granted":[] rather than "granted":null on a total
	// failure -- openapi.yaml declares it a required array with no "nullable",
	// and a client that iterates it on a 502 would otherwise get null. failed
	// stays nil on purpose: its own omitempty tag then drops it entirely on a
	// full success, matching the schema's "omitted on full success" contract.
	granted := []string{}
	var failed []string
	for _, key := range req.Roles {
		if err := h.scim.AddRoleMembers(r.Context(), roleIDs[key], []string{targetUser.Email}); err != nil {
			slog.ErrorContext(r.Context(), "GrantUserRoles: scim AddRoleMembers failed", "userID", user.UserID, "targetID", id, "role", key, "err", err)
			failed = append(failed, key)
			continue
		}
		granted = append(granted, key)
	}

	resp := grantUserRolesResponse{Granted: granted, Failed: failed}
	switch {
	case len(failed) == 0:
		writeJSONValue(w, http.StatusOK, resp)
	case len(granted) == 0:
		writeJSONValue(w, http.StatusBadGateway, resp)
	default:
		writeJSONValue(w, http.StatusMultiStatus, resp)
	}
}

// timeCardApproversResponse is the GET /users/time-card-approvers response shape.
type timeCardApproversResponse struct {
	Approvers []timeCardApproverRef `json:"approvers"`
}

type timeCardApproverRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// GetTimeCardApprovers handles GET /users/time-card-approvers. Lists the real
// membership of every configured time-card-approver role via the SCIM
// operations service, rather than entity-service's own Postgres `role`/
// `user_role` tables (what POST /users/search's roleIds filter reads) --
// approval is actually granted by real role membership (see
// AUTH_TIMECARD_APPROVER_ROLES in "Access control"), and the Postgres table
// is a separate, syncable mirror that can drift from it. AUTH_TIMECARD_APPROVER_ROLES
// can name several real role names, each resolved to its own ID (see
// handler.RoleIDsForKey) -- an approver is anyone holding ANY of them, so
// every configured ID is queried and the results merged, deduplicated by
// member ID in case the same person holds more than one. Only registered
// (see cmd/server/main.go) once at least one ID is configured.
func (h *UsersHandler) GetTimeCardApprovers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	// Registered unconditionally (see cmd/server/main.go) so a disabled
	// deployment 404s cleanly here rather than falling through to the
	// wildcard GET /users/{id} route, which would reject the literal segment
	// "time-card-approvers" as an invalid UUID with 400 instead.
	if len(h.timecardApproverRoleIDs) == 0 {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	seen := make(map[string]struct{})
	approvers := make([]timeCardApproverRef, 0, len(h.timecardApproverRoleIDs))
	for _, roleID := range h.timecardApproverRoleIDs {
		members, err := h.scim.GetRole(r.Context(), roleID)
		if err != nil {
			slog.ErrorContext(r.Context(), "scim GetRole (time card approvers) failed", "userID", user.UserID, "roleID", roleID, "err", err)
			// A 401/403 here means this backend's own SCIM client credentials
			// lack the scope to read roles (see ASGARDEO_ROLE_IDS's own doc
			// comment) -- a deployment/configuration problem, not anything
			// about the calling portal user's own permissions.
			// mapUpstreamErrorGeneric's usual 401/403 pass-through would tell
			// an ordinary viewer "you don't have permission" for what is
			// actually a backend misconfiguration an admin needs to fix, so
			// those two codes are reported as a sanitized 502 instead; every
			// other status still goes through the usual mapping.
			var apiErr *apierror.Error
			if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
				writeError(w, http.StatusBadGateway, "Failed to list time card approvers.")
				return
			}
			mapUpstreamErrorGeneric(w, err, "Failed to list time card approvers.")
			return
		}

		for _, m := range members {
			if _, dup := seen[m.ID]; dup {
				continue
			}
			seen[m.ID] = struct{}{}
			approvers = append(approvers, timeCardApproverRef{ID: m.ID, Email: m.Email})
		}
	}
	writeJSONValue(w, http.StatusOK, timeCardApproversResponse{Approvers: approvers})
}

// ListSavedFilterViews handles GET /users/me/saved-filter-views.
func (h *UsersHandler) ListSavedFilterViews(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	listKey := r.URL.Query().Get("listKey")
	if listKey == "" {
		writeError(w, http.StatusBadRequest, "listKey is required.")
		return
	}

	result, err := h.entity.ListSavedFilterViews(r.Context(), listKey)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListSavedFilterViews failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to list saved filter views.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SaveSavedFilterView handles PATCH /users/me/saved-filter-views.
func (h *UsersHandler) SaveSavedFilterView(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SaveSavedFilterView(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SaveSavedFilterView failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to save the filter view.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteSavedFilterView handles DELETE /users/me/saved-filter-views.
func (h *UsersHandler) DeleteSavedFilterView(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	listKey := q.Get("listKey")
	name := q.Get("name")
	if listKey == "" || name == "" {
		writeError(w, http.StatusBadRequest, "listKey and name are required.")
		return
	}

	result, err := h.entity.DeleteSavedFilterView(r.Context(), listKey, name)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteSavedFilterView failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to delete the filter view.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ReorderSavedFilterView handles POST /users/me/saved-filter-views/reorder.
func (h *UsersHandler) ReorderSavedFilterView(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.ReorderSavedFilterView(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ReorderSavedFilterView failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to reorder saved filter views.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
