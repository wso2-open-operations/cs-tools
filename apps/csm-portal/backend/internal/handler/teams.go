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
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityTeamsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityTeamsClient interface {
	GetTeamMembers(ctx context.Context, teamID string) ([]byte, error)
	GetTeam(ctx context.Context, teamID string) ([]byte, error)
	UpdateTeam(ctx context.Context, teamID string, body []byte) ([]byte, error)
	AssignTeamLead(ctx context.Context, teamID string, body []byte) ([]byte, error)
}

type entityTeamMember struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
	Role  *string `json:"role"`
}

type entityGetTeamMembersResponse struct {
	Members []entityTeamMember `json:"members"`
}

// TeamMemberView is one entry of the GET /teams/{id}/members response.
type TeamMemberView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// entityTeamDetail mirrors entity-service's domain.TeamDetail.
type entityTeamDetail struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	ChatWebhookURL *string `json:"chatWebhookUrl"`
}

// TeamDetailView is the GET/PATCH /teams/{id} response.
type TeamDetailView struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	ChatWebhookURL *string `json:"chatWebhookUrl"`
}

// updateTeamRequest is the body of PATCH /teams/{id}. chatWebhookUrl is the
// only settable field today.
type updateTeamRequest struct {
	ChatWebhookURL *string `json:"chatWebhookUrl"`
}

// assignTeamLeadRequest is the body of POST /teams/{id}/lead.
type assignTeamLeadRequest struct {
	UserID *string `json:"userId"`
}

// TeamHandler handles HTTP requests for a team's member roster (an
// account's CRE/SRE team, not the internal WSO2-staff team registry
// csm-admin's own /teams/search already covers), backed by entity-service
// (team membership, role). Originally SPL-only (GET
// /spl/abt-team-members?teamId=...), merged into a plain, unprefixed route
// once SPL's own data source for it became this exact entity-service
// endpoint: there was no ServiceNow-shape translation left to justify a
// second, parallel /spl/* contract for it.
type TeamHandler struct {
	entity entityTeamsClient
	// access gates GetChatWebhook/UpdateChatWebhook to an Asgardeo admin or
	// this specific team's own lead (see authorizeChatWebhookActor below). A
	// nil access denies -- never grants -- same posture as CommentHandler's
	// own access field. Set with WithAccessGuard.
	access *AccessGuard
}

// NewTeamHandler creates a TeamHandler backed by the given entity client.
func NewTeamHandler(entity entityTeamsClient) *TeamHandler {
	return &TeamHandler{entity: entity}
}

// WithAccessGuard sets the AccessGuard GetChatWebhook/UpdateChatWebhook use to
// recognize an Asgardeo admin. Returns the receiver so it chains onto
// NewTeamHandler at the construction site in cmd/server/main.go.
func (h *TeamHandler) WithAccessGuard(g *AccessGuard) *TeamHandler {
	h.access = g
	return h
}

var hex32Pattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// normalizeToUUID converts a bare 32-hex-character ServiceNow sys_id (no
// dashes -- what SPL's old teamId query param carried, back when it went
// straight into a ServiceNow Table API query) into standard 8-4-4-4-12
// dashed UUID form, matching how entity-service's Postgres ids are
// formatted. A value that doesn't match that bare-hex shape (already
// dashed, which is what the frontend now sends via an account's own
// creTeam.id) is passed through unchanged -- this never rejects input
// itself, entity-service's own UUID validation does that.
func normalizeToUUID(id string) string {
	if !hex32Pattern.MatchString(id) {
		return id
	}
	lower := strings.ToLower(id)
	return fmt.Sprintf("%s-%s-%s-%s-%s", lower[0:8], lower[8:12], lower[12:16], lower[16:20], lower[20:32])
}

// GetTeamMembers handles GET /teams/{id}/members.
func (h *TeamHandler) GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" || !uuidRe.MatchString(normalizeToUUID(teamID)) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	ctx := r.Context()

	raw, err := h.entity.GetTeamMembers(ctx, normalizeToUUID(teamID))
	if err != nil {
		slog.ErrorContext(ctx, "entity GetTeamMembers failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve team members.")
		return
	}
	var resp entityGetTeamMembersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		slog.ErrorContext(ctx, "unmarshal entity-service team members response failed", "userID", user.UserID, "teamID", teamID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	if len(resp.Members) == 0 {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	views := make([]TeamMemberView, 0, len(resp.Members))
	for _, m := range resp.Members {
		view := TeamMemberView{Name: m.Name}
		if m.Email != nil {
			view.Email = *m.Email
		}
		if m.Role != nil {
			view.Role = *m.Role
		}

		views = append(views, view)
	}

	writeJSONValue(w, http.StatusOK, views)
}

// callerIsTeamLead reports whether user currently holds role "lead" on the
// given team, by fetching the team's own roster and matching on email
// (case-insensitively) -- the roster already carries each member's email, so
// unlike CaseHandler.callerMayDeescalate this needs no separate GetUserMe
// call to resolve the caller's own platform id first. Fails closed (false) on
// any upstream or parse error, or when the team has no such member: an
// authorization check must never silently grant on a failure it can't
// explain.
func (h *TeamHandler) callerIsTeamLead(r *http.Request, teamID string, user *middleware.UserInfo) bool {
	if user.Email == "" {
		return false
	}
	raw, err := h.entity.GetTeamMembers(r.Context(), teamID)
	if err != nil {
		return false
	}
	var resp entityGetTeamMembersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false
	}
	for _, m := range resp.Members {
		if m.Role == nil || !strings.EqualFold(*m.Role, "lead") {
			continue
		}
		if m.Email != nil && strings.EqualFold(*m.Email, user.Email) {
			return true
		}
	}
	return false
}

// authorizeChatWebhookActor writes a 403 and returns false unless the caller
// is an Asgardeo admin or this specific team's own lead -- the only two
// audiences allowed to view or change a team's Chat space configuration (a
// bearer credential once set). Fails closed when no AccessGuard is wired.
func (h *TeamHandler) authorizeChatWebhookActor(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, teamID string) bool {
	isAdmin := h.access != nil && h.access.Permits(PermAdmin, user.Roles)
	if isAdmin || h.callerIsTeamLead(r, teamID, user) {
		return true
	}
	writeError(w, http.StatusForbidden, ErrMsgForbidden)
	return false
}

// GetTeam handles GET /teams/{id} -- an Asgardeo admin or this specific
// team's own lead only (authorizeChatWebhookActor); every other caller
// holding the route's own PermViewSharedEntity floor gets 403. A single
// generic team resource (mirroring GET /users/{id}) rather than a
// dedicated /teams/{id}/chat-webhook route.
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" || !uuidRe.MatchString(normalizeToUUID(teamID)) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if !h.authorizeChatWebhookActor(w, r, user, normalizeToUUID(teamID)) {
		return
	}

	ctx := r.Context()
	raw, err := h.entity.GetTeam(ctx, normalizeToUUID(teamID))
	if err != nil {
		slog.ErrorContext(ctx, "entity GetTeam failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve the team.")
		return
	}
	var detail entityTeamDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		slog.ErrorContext(ctx, "unmarshal entity-service team response failed", "userID", user.UserID, "teamID", teamID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSONValue(w, http.StatusOK, TeamDetailView(detail))
}

// UpdateTeam handles PATCH /teams/{id} -- an Asgardeo admin or this specific
// team's own lead only (authorizeChatWebhookActor). chatWebhookUrl is the
// only settable field today; entity-service itself validates it (well-formed
// https://chat.googleapis.com/... or empty to clear), and its 400 reason is
// echoed verbatim (mapUpstreamError) so the form can show it.
func (h *TeamHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" || !uuidRe.MatchString(normalizeToUUID(teamID)) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	teamID = normalizeToUUID(teamID)

	if !h.authorizeChatWebhookActor(w, r, user, teamID) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var payload updateTeamRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.ChatWebhookURL == nil {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}
	url := strings.TrimSpace(*payload.ChatWebhookURL)
	entityBody, err := json.Marshal(map[string]string{"chatWebhookUrl": url})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	ctx := r.Context()
	raw, err := h.entity.UpdateTeam(ctx, teamID, entityBody)
	if err != nil {
		slog.ErrorContext(ctx, "entity UpdateTeam failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamError(w, err, "Failed to update the team's Chat space.")
		return
	}
	var detail entityTeamDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		slog.ErrorContext(ctx, "unmarshal entity-service team response failed", "userID", user.UserID, "teamID", teamID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSONValue(w, http.StatusOK, TeamDetailView(detail))
}

// AssignLead handles POST /teams/{id}/lead -- restricted to admin via the
// route's PermAdmin permission (cmd/server/main.go), same posture as
// CreateUser: this handler does not re-check the caller's role itself.
// Reassigns the team's real, paging-connected 'lead' role -- see
// entity-service's own CLAUDE.md for what that touches beyond this feature.
func (h *TeamHandler) AssignLead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	teamID := r.PathValue("id")
	if teamID == "" || !uuidRe.MatchString(normalizeToUUID(teamID)) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	teamID = normalizeToUUID(teamID)

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var payload assignTeamLeadRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.UserID == nil || !uuidRe.MatchString(*payload.UserID) {
		writeError(w, http.StatusBadRequest, "userId must be a valid UUID.")
		return
	}

	entityBody, err := json.Marshal(map[string]string{"userId": *payload.UserID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	ctx := r.Context()
	if _, err := h.entity.AssignTeamLead(ctx, teamID, entityBody); err != nil {
		slog.ErrorContext(ctx, "entity AssignTeamLead failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamError(w, err, "Failed to assign the team lead.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
