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
	ID        *string `json:"id,omitempty"`
	Email     string  `json:"email"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	TimeZone  *string `json:"timeZone,omitempty"`
	// Roles is which portal roles (viewer, cs_engineer, admin, ...) the
	// caller's token roles grant: several are possible. It is not the entity
	// service's role data, which this response no longer carries. Always
	// present, [] when they hold none.
	Roles       []string          `json:"roles"`
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
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName *string `json:"firstName"`
	LastName  string  `json:"lastName"`
	TimeZone  *string `json:"timeZone"`
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
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
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
// phoneNumber is updated via SCIM and then mirrored to the entity service; timeZone is updated
// via the entity service. A request carrying both makes one entity call.
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
	// report a value that was never stored. Must precede the identity provider call.
	if payload.TimeZone != nil && strings.TrimSpace(*payload.TimeZone) == "" {
		writeError(w, http.StatusBadRequest, "timeZone must not be empty.")
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

	// Mirror into the platform database with one entity PATCH carrying only the
	// fields this request changed. SCIM has already succeeded; it is idempotent,
	// so a client retry after an entity failure is safe.
	entityFields := map[string]string{}
	if payload.PhoneNumber != nil {
		// Store what the identity provider actually kept, falling back to the request.
		stored := *payload.PhoneNumber
		if resp.PhoneNumber != nil {
			stored = *resp.PhoneNumber
		}
		entityFields["phone"] = stored
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
