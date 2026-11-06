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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// RotaAccessHandler serves the Team Schedule's Rota access tab: who holds a
// rota admin role or is a team's rota marker, and granting or removing either.
// The service decides who may; grants are PUT and removals DELETE, both
// idempotent, so a retried click lands where the first one meant to.
type RotaAccessHandler struct {
	svc service.RotaAccessService
}

// NewRotaAccessHandler constructs a RotaAccessHandler.
func NewRotaAccessHandler(svc service.RotaAccessService) *RotaAccessHandler {
	return &RotaAccessHandler{svc: svc}
}

// GetRotaAccess handles GET /team-schedule/rota-access.
func (h *RotaAccessHandler) GetRotaAccess(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GrantRotaAdmin handles PUT /team-schedule/rota-access/admins/{family}/{userId}.
func (h *RotaAccessHandler) GrantRotaAdmin(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.GrantRotaAdmin(r.Context(), r.PathValue("family"), r.PathValue("userId")))
}

// RevokeRotaAdmin handles DELETE /team-schedule/rota-access/admins/{family}/{userId}.
func (h *RotaAccessHandler) RevokeRotaAdmin(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.RevokeRotaAdmin(r.Context(), r.PathValue("family"), r.PathValue("userId")))
}

// GrantRotaMarker handles PUT /team-schedule/rota-access/markers/{teamKey}/{userId}.
func (h *RotaAccessHandler) GrantRotaMarker(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.GrantRotaMarker(r.Context(), r.PathValue("teamKey"), r.PathValue("userId")))
}

// RevokeRotaMarker handles DELETE /team-schedule/rota-access/markers/{teamKey}/{userId}.
func (h *RotaAccessHandler) RevokeRotaMarker(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.RevokeRotaMarker(r.Context(), r.PathValue("teamKey"), r.PathValue("userId")))
}

// AddTeamMember handles PUT /team-schedule/teams/{teamKey}/members/{userId}.
func (h *RotaAccessHandler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.AddTeamMember(r.Context(), r.PathValue("teamKey"), r.PathValue("userId")))
}

// RemoveTeamMember handles DELETE /team-schedule/teams/{teamKey}/members/{userId}.
func (h *RotaAccessHandler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	h.done(w, r, h.svc.RemoveTeamMember(r.Context(), r.PathValue("teamKey"), r.PathValue("userId")))
}

// updateTeamMemberRequest is the body of PATCH .../members/{userId}.
// alertTier is a raw message so that an absent field (leave the tier) and an
// explicit null (clear it) can be told apart.
type updateTeamMemberRequest struct {
	Role      *string         `json:"role"`
	AlertTier json.RawMessage `json:"alertTier"`
}

// UpdateTeamMember handles PATCH /team-schedule/teams/{teamKey}/members/{userId}.
func (h *RotaAccessHandler) UpdateTeamMember(w http.ResponseWriter, r *http.Request) {
	var req updateTeamMemberRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	patch := domain.TeamMemberPatch{Role: req.Role}
	if req.AlertTier != nil {
		patch.AlertTierSet = true
		if !bytes.Equal(bytes.TrimSpace(req.AlertTier), []byte("null")) {
			var tier string
			if err := json.Unmarshal(req.AlertTier, &tier); err != nil {
				writeServiceError(w, r, &apierror.ValidationError{Msg: "alertTier must be T1, T2, T3 or null"})
				return
			}
			patch.AlertTier = &tier
		}
	}
	m, err := h.svc.UpdateTeamMember(r.Context(), r.PathValue("teamKey"), r.PathValue("userId"), patch)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, m)
}

// moveTeamMemberRequest is the body of POST .../members/{userId}/move.
type moveTeamMemberRequest struct {
	ToTeamKey string `json:"toTeamKey"`
}

// MoveTeamMember handles POST /team-schedule/teams/{teamKey}/members/{userId}/move.
func (h *RotaAccessHandler) MoveTeamMember(w http.ResponseWriter, r *http.Request) {
	var req moveTeamMemberRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	m, err := h.svc.MoveTeamMember(r.Context(), r.PathValue("teamKey"), r.PathValue("userId"), req.ToTeamKey)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, m)
}

// done answers a grant or removal: 204, or the service's refusal.
func (h *RotaAccessHandler) done(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
