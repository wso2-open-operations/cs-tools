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

// TeamHandler handles HTTP requests for a team's member roster (an
// account's CRE/SRE team, not the internal WSO2-staff team registry
// csm-admin's own /teams/search already covers), backed by entity-service
// (team membership, role). Originally SPL-only (GET
// /spl/abt-team-members?teamId=...), merged into a plain, unprefixed route
// once SPL's own data source for it became this exact entity-service
// endpoint: there was no backing-system-shape translation left to justify a
// second, parallel /spl/* contract for it.
type TeamHandler struct {
	entity entityTeamsClient
}

// NewTeamHandler creates a TeamHandler backed by the given entity client.
func NewTeamHandler(entity entityTeamsClient) *TeamHandler {
	return &TeamHandler{entity: entity}
}

var hex32Pattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// normalizeToUUID converts a bare 32-hex-character the backing system sys_id (no
// dashes -- what SPL's old teamId query param carried, back when it went
// straight into a backing-system Table API query) into standard 8-4-4-4-12
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
		slog.ErrorContext(ctx, "entity GetTeamMembers failed", "userID", user.UserID, "teamID", teamID, "err", summarizeErr(err))
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
