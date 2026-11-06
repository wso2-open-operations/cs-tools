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
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityRotaAccessClient is the entity-service calls the Team Schedule's Rota
// access tab needs. Its own interface rather than more methods on
// entityScheduleClient, so nothing that fakes the schedule client changes.
type entityRotaAccessClient interface {
	GetRotaAccess(ctx context.Context) ([]byte, error)
	PutRotaAdmin(ctx context.Context, family, userID string) ([]byte, error)
	DeleteRotaAdmin(ctx context.Context, family, userID string) ([]byte, error)
	PutRotaMarker(ctx context.Context, teamKey, userID string) ([]byte, error)
	DeleteRotaMarker(ctx context.Context, teamKey, userID string) ([]byte, error)
	PutTeamMember(ctx context.Context, teamKey, userID string) ([]byte, error)
	DeleteTeamMember(ctx context.Context, teamKey, userID string) ([]byte, error)
	PatchTeamMember(ctx context.Context, teamKey, userID string, body []byte) ([]byte, error)
	MoveTeamMember(ctx context.Context, teamKey, userID string, body []byte) ([]byte, error)
}

// RotaAccessHandler proxies the Rota access tab: who holds a rota admin role
// (one family each) or is a team's rota marker, and granting or removing
// either. Who may is entity-service's decision -- a CSM admin for rota admin,
// a CSM admin or the family's rota admin for a team's marker -- so this checks
// only the shape of the request and forwards the caller's identity.
type RotaAccessHandler struct {
	entity entityRotaAccessClient
}

// NewRotaAccessHandler creates a RotaAccessHandler.
func NewRotaAccessHandler(entity entityRotaAccessClient) *RotaAccessHandler {
	return &RotaAccessHandler{entity: entity}
}

var (
	rotaFamilyRe  = regexp.MustCompile(`(?i)^(cre|sre|sme)$`)
	rotaTeamKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
)

// GetRotaAccess handles GET /team-schedule/rota-access. Somebody who may
// change nothing gets an empty answer, which is how the page hides the tab.
func (h *RotaAccessHandler) GetRotaAccess(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.GetRotaAccess(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetRotaAccess failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load rota access.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PutRotaAdmin handles PUT /team-schedule/rota-access/admins/{family}/{userId}.
func (h *RotaAccessHandler) PutRotaAdmin(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "family", rotaFamilyRe, "PutRotaAdmin", "Failed to grant rota admin.", h.entity.PutRotaAdmin)
}

// DeleteRotaAdmin handles DELETE /team-schedule/rota-access/admins/{family}/{userId}.
func (h *RotaAccessHandler) DeleteRotaAdmin(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "family", rotaFamilyRe, "DeleteRotaAdmin", "Failed to remove rota admin.", h.entity.DeleteRotaAdmin)
}

// PutRotaMarker handles PUT /team-schedule/rota-access/markers/{teamKey}/{userId}.
func (h *RotaAccessHandler) PutRotaMarker(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "teamKey", rotaTeamKeyRe, "PutRotaMarker", "Failed to add the rota manager.", h.entity.PutRotaMarker)
}

// DeleteRotaMarker handles DELETE /team-schedule/rota-access/markers/{teamKey}/{userId}.
func (h *RotaAccessHandler) DeleteRotaMarker(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "teamKey", rotaTeamKeyRe, "DeleteRotaMarker", "Failed to remove the rota manager.", h.entity.DeleteRotaMarker)
}

// PutTeamMember handles PUT /team-schedule/teams/{teamKey}/members/{userId}:
// adding somebody to a team. Who may, and to which team, is entity-service's
// decision -- a CSM admin, or the rota admin of the team's family.
func (h *RotaAccessHandler) PutTeamMember(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "teamKey", rotaTeamKeyRe, "PutTeamMember", "Failed to add the team member.", h.entity.PutTeamMember)
}

// DeleteTeamMember handles DELETE /team-schedule/teams/{teamKey}/members/{userId}.
// entity-service refuses it while the person has upcoming turns, a Case Alerts
// tier or (for a rota admin) a lead role, and says why; the reason is passed on.
func (h *RotaAccessHandler) DeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "teamKey", rotaTeamKeyRe, "DeleteTeamMember", "Failed to remove the team member.", h.entity.DeleteTeamMember)
}

// change checks one grant or removal's path, forwards it, and answers 204.
//
// A refusal keeps entity-service's own reason -- "only a CSM admin can grant
// or remove rota admin", "that person is not on lyra" -- so the tab can say
// why rather than only that nothing changed. Anything else stays generic.
func (h *RotaAccessHandler) change(w http.ResponseWriter, r *http.Request, scopeName string, scopeRe *regexp.Regexp,
	op, fallbackMsg string, call func(ctx context.Context, scope, userID string) ([]byte, error)) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	scope, target := r.PathValue(scopeName), r.PathValue("userId")
	if !scopeRe.MatchString(scope) || !uuidRe.MatchString(target) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if _, err := call(r.Context(), scope, target); err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", user.UserID, "err", err)
		writeMemberChangeError(w, err, fallbackMsg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeMemberChangeError answers a refused team member change with
// entity-service's own reason for a 400, 403, 404 or 409; anything else
// stays generic.
func writeMemberChangeError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
			writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
			return
		}
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}

// PatchTeamMember handles PATCH /team-schedule/teams/{teamKey}/members/{userId}:
// a member's role (engineer or sub lead) and Case Alerts tier. The body is
// forwarded as is; entity-service validates it, decides who may (a CSM admin)
// and says why when it refuses, and the reason is passed on.
func (h *RotaAccessHandler) PatchTeamMember(w http.ResponseWriter, r *http.Request) {
	h.changeWithBody(w, r, "PatchTeamMember", "Failed to change the team member.", h.entity.PatchTeamMember)
}

// MoveTeamMember handles POST /team-schedule/teams/{teamKey}/members/{userId}/move:
// {teamKey} is the team they leave, the body names the team they join.
// entity-service refuses it, writing nothing, while the person has upcoming
// turns or an alert tier on the source team; the reason is passed on.
func (h *RotaAccessHandler) MoveTeamMember(w http.ResponseWriter, r *http.Request) {
	h.changeWithBody(w, r, "MoveTeamMember", "Failed to move the team member.", h.entity.MoveTeamMember)
}

// changeWithBody checks one team member change's path and that its body is
// JSON, forwards both, and answers entity-service's 200 body.
func (h *RotaAccessHandler) changeWithBody(w http.ResponseWriter, r *http.Request, op, fallbackMsg string,
	call func(ctx context.Context, teamKey, userID string, body []byte) ([]byte, error)) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	teamKey, target := r.PathValue("teamKey"), r.PathValue("userId")
	if !rotaTeamKeyRe.MatchString(teamKey) || !uuidRe.MatchString(target) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	result, err := call(r.Context(), teamKey, target, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", userID, "err", err)
		writeMemberChangeError(w, err, fallbackMsg)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
