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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityCodeDashboardShareClient is the part of the entity client the code
// dashboard sharing routes use.
type entityCodeDashboardShareClient interface {
	ListCodeDashboardShares(ctx context.Context, id string) ([]entity.CustomDashboardShare, error)
	AddCodeDashboardShare(ctx context.Context, id, userID string) ([]entity.CustomDashboardShare, error)
	RemoveCodeDashboardShare(ctx context.Context, id, userID string) error
	ListCodeDashboardTeamShares(ctx context.Context, id string) ([]entity.CustomDashboardTeamShare, error)
	AddCodeDashboardTeamShare(ctx context.Context, id, teamID string) ([]entity.CustomDashboardTeamShare, error)
	RemoveCodeDashboardTeamShare(ctx context.Context, id, teamID string) error
	GetCodeDashboardLink(ctx context.Context, id string) (entity.CustomDashboardLink, error)
	EnableCodeDashboardLink(ctx context.Context, id string) (entity.CustomDashboardLink, error)
	DisableCodeDashboardLink(ctx context.Context, id string) error
	OpenCodeDashboardLink(ctx context.Context, linkID string) (entity.OpenedCustomDashboardLink, error)
	ListSharedCodeDashboards(ctx context.Context) ([]entity.SharedCustomDashboardSummary, error)
	GetSharedCodeDashboard(ctx context.Context, id string) (entity.SharedCodeDashboard, error)
}

// CodeDashboardShareHandler shares the caller's saved advanced dashboards with
// people, registry teams and by link, and serves the advanced dashboards
// shared with the caller: the custom dashboard sharing routes, with the same
// requests, answers and messages, on entity-service's code dashboard routes
// (migration 0296). Like the other advanced dashboard routes it exists only
// when DASHBOARD_GENERATE_CODE_ENABLED is on (main.go). A shared dashboard is
// answered with its source as stored: this backend never runs it, and the
// webapp checks it and runs it in its sandbox, where its data comes through
// the fixed portal APIs with the viewer's own permissions.
type CodeDashboardShareHandler struct {
	entity entityCodeDashboardShareClient
	// teams maps registry team keys to entity-service team ids and back, as
	// for custom dashboards; only its team methods are used.
	teams *CustomDashboardHandler
}

// NewCodeDashboardShareHandler returns the handler using client, with teams as
// the registry teams a dashboard can be shared with (as
// CustomDashboardHandler.WithTeams).
func NewCodeDashboardShareHandler(client entityCodeDashboardShareClient, teams []directory.TeamResult) *CodeDashboardShareHandler {
	return &CodeDashboardShareHandler{entity: client, teams: (&CustomDashboardHandler{}).WithTeams(teams)}
}

// signedIn writes the 401 and returns false when the request has no user.
func signedIn(w http.ResponseWriter, r *http.Request) bool {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return false
	}
	return true
}

// ListCodeDashboardShares handles GET /dashboards/code/{id}/shares (owner only).
func (h *CodeDashboardShareHandler) ListCodeDashboardShares(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	shares, err := h.entity.ListCodeDashboardShares(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCodeDashboardShares failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardSharesFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, customDashboardShareListResponse{Shares: shares})
}

// AddCodeDashboardShare handles PUT /dashboards/code/{id}/shares/{userId} (no
// body) and answers the shares after the change. As for custom dashboards the
// refusals the user can act on (the caller, or not internal staff: 400) are
// passed on.
func (h *CodeDashboardShareHandler) AddCodeDashboardShare(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, userID, ok := customDashboardAndUserIDs(w, r)
	if !ok {
		return
	}
	shares, err := h.entity.AddCodeDashboardShare(r.Context(), id, userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCodeDashboardShare failed", "dashboardId", id, "userId", userID, "err", err)
		mapUpstreamError(w, err, errMsgCustomDashboardShareFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, customDashboardShareListResponse{Shares: shares})
}

// RemoveCodeDashboardShare handles DELETE /dashboards/code/{id}/shares/{userId} (204).
func (h *CodeDashboardShareHandler) RemoveCodeDashboardShare(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, userID, ok := customDashboardAndUserIDs(w, r)
	if !ok {
		return
	}
	if err := h.entity.RemoveCodeDashboardShare(r.Context(), id, userID); err != nil {
		slog.ErrorContext(r.Context(), "entity RemoveCodeDashboardShare failed", "dashboardId", id, "userId", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardUnshareFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListCodeDashboardTeamShares handles GET /dashboards/code/{id}/team-shares
// (owner only): each team by registry key and family.
func (h *CodeDashboardShareHandler) ListCodeDashboardTeamShares(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	shares, err := h.entity.ListCodeDashboardTeamShares(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCodeDashboardTeamShares failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgTeamSharesFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, h.teams.teamShareListResponse(shares))
}

// AddCodeDashboardTeamShare handles PUT /dashboards/code/{id}/team-shares/{teamKey}
// (no body). A key that is not a registry team with a group id is a 404
// without an upstream call.
func (h *CodeDashboardShareHandler) AddCodeDashboardTeamShare(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, team, ok := h.teams.customDashboardIDAndTeam(w, r)
	if !ok {
		return
	}
	shares, err := h.entity.AddCodeDashboardTeamShare(r.Context(), id, team.id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCodeDashboardTeamShare failed", "dashboardId", id, "teamKey", team.key, "err", err)
		mapUpstreamError(w, err, errMsgTeamShareFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, h.teams.teamShareListResponse(shares))
}

// RemoveCodeDashboardTeamShare handles DELETE /dashboards/code/{id}/team-shares/{teamKey} (204).
func (h *CodeDashboardShareHandler) RemoveCodeDashboardTeamShare(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, team, ok := h.teams.customDashboardIDAndTeam(w, r)
	if !ok {
		return
	}
	if err := h.entity.RemoveCodeDashboardTeamShare(r.Context(), id, team.id); err != nil {
		slog.ErrorContext(r.Context(), "entity RemoveCodeDashboardTeamShare failed", "dashboardId", id, "teamKey", team.key, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgTeamUnshareFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetCodeDashboardLink handles GET /dashboards/code/{id}/link (owner only).
func (h *CodeDashboardShareHandler) GetCodeDashboardLink(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	link, err := h.entity.GetCodeDashboardLink(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCodeDashboardLink failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgDashboardLinkFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, link)
}

// EnableCodeDashboardLink handles PUT /dashboards/code/{id}/link (no body); a
// link that is already on keeps its id.
func (h *CodeDashboardShareHandler) EnableCodeDashboardLink(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	link, err := h.entity.EnableCodeDashboardLink(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity EnableCodeDashboardLink failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgDashboardLinkEnableFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, link)
}

// DisableCodeDashboardLink handles DELETE /dashboards/code/{id}/link (204).
func (h *CodeDashboardShareHandler) DisableCodeDashboardLink(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	if err := h.entity.DisableCodeDashboardLink(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity DisableCodeDashboardLink failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgDashboardLinkDisableFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OpenCodeDashboardLink handles POST /dashboards/shared/code/link, body
// {linkId}, as OpenDashboardLink: the link id is in the body and never
// logged, and an unknown or turned-off link is a 404 with its own message.
func (h *CodeDashboardShareHandler) OpenCodeDashboardLink(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	var req openDashboardLinkRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if !uuidRe.MatchString(req.LinkID) {
		writeError(w, http.StatusBadRequest, errMsgDashboardLinkInvalid)
		return
	}
	opened, err := h.entity.OpenCodeDashboardLink(r.Context(), strings.ToLower(req.LinkID))
	if err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			writeError(w, http.StatusNotFound, errMsgDashboardLinkNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "entity OpenCodeDashboardLink failed", "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgDashboardLinkOpenFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, opened)
}

// ListSharedCodeDashboards handles GET /dashboards/shared/code: the advanced
// dashboards other users shared with the caller, ordered by name, never null.
func (h *CodeDashboardShareHandler) ListSharedCodeDashboards(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	list, err := h.entity.ListSharedCodeDashboards(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListSharedCodeDashboards failed", "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgSharedDashboardListFailed)
		return
	}
	if list == nil {
		list = []entity.SharedCustomDashboardSummary{}
	}
	writeJSONValue(w, http.StatusOK, sharedDashboardListResponse{Dashboards: list})
}

// GetSharedCodeDashboard handles GET /dashboards/shared/code/{id}: one advanced
// dashboard shared with the caller, the owner's current source, view only. Not
// shared with the caller (including the caller's own) is a 404.
func (h *CodeDashboardShareHandler) GetSharedCodeDashboard(w http.ResponseWriter, r *http.Request) {
	if !signedIn(w, r) {
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	shared, err := h.entity.GetSharedCodeDashboard(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSharedCodeDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCodeDashboardLoadFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, shared)
}
