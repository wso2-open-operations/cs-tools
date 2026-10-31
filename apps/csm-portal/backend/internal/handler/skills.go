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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entitySkillClient is the entity-service calls the skills module needs.
type entitySkillClient interface {
	GetSkillLevels(ctx context.Context) ([]byte, error)
	GetSkillCategories(ctx context.Context) ([]byte, error)
	SearchSkills(ctx context.Context, body []byte) ([]byte, error)
	GetSkill(ctx context.Context, id string) ([]byte, error)
	CreateSkill(ctx context.Context, body []byte) ([]byte, error)
	UpdateSkill(ctx context.Context, id string, body []byte) ([]byte, error)
	GetMySkills(ctx context.Context) ([]byte, error)
	AddMySkill(ctx context.Context, body []byte) ([]byte, error)
	SetMySkillLevel(ctx context.Context, skillID string, body []byte) ([]byte, error)
	RemoveMySkill(ctx context.Context, skillID string) ([]byte, error)
	GetMySkillHistory(ctx context.Context, skillID, limit, offset string) ([]byte, error)
	SearchUserSkills(ctx context.Context, body []byte) ([]byte, error)
}

// SkillHandler proxies the skills module: the catalogue, a person's own
// skills, the skill manager's view of everybody's, and the admin's catalogue
// edits. It decides nothing about who may do what: entity-service does, from
// the forwarded user token ("my" is always that token's user, never an id from
// the request). This checks only the shape of the request (valid JSON, UUID
// path ids) and passes bodies and responses through unchanged.
type SkillHandler struct {
	entity entitySkillClient
}

// NewSkillHandler creates a SkillHandler backed by the given entity client.
func NewSkillHandler(entity entitySkillClient) *SkillHandler {
	return &SkillHandler{entity: entity}
}

// writeSkillError answers a failed skills call. A 400, 403, 404 or 409 keeps
// entity-service's own message (and its machine-readable errorCode when it
// names one): those are the caller's to act on ("only a CSM admin can ...",
// "this skill is already on your profile"). Anything else stays generic.
func writeSkillError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
			writeErrorCode(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg), upstreamErrorCode(apiErr.Body))
			return
		}
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}

// skillCaller returns the authenticated caller's id, or writes a 401.
func skillCaller(w http.ResponseWriter, r *http.Request) (string, bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return "", false
	}
	return user.UserID, true
}

// skillCall runs one entity call and writes its result with status, or the
// error. A nil-body 204 writes no content.
func (h *SkillHandler) skillCall(w http.ResponseWriter, r *http.Request, op, fallbackMsg string, status int,
	call func() ([]byte, error)) {
	userID, ok := skillCaller(w, r)
	if !ok {
		return
	}
	result, err := call()
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", userID, "err", err)
		writeSkillError(w, err, fallbackMsg)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, result)
}

// skillBodyCall is skillCall for a route that forwards the request body.
func (h *SkillHandler) skillBodyCall(w http.ResponseWriter, r *http.Request, op, fallbackMsg string, status int,
	call func(body []byte) ([]byte, error)) {
	body, _, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	h.skillCall(w, r, op, fallbackMsg, status, func() ([]byte, error) { return call(body) })
}

// skillPathID returns the named path value when it is a UUID, else writes a 400.
func skillPathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", false
	}
	return id, true
}

// GetSkillLevels handles GET /skills/levels.
func (h *SkillHandler) GetSkillLevels(w http.ResponseWriter, r *http.Request) {
	h.skillCall(w, r, "GetSkillLevels", "Failed to load the skill levels.", http.StatusOK,
		func() ([]byte, error) { return h.entity.GetSkillLevels(r.Context()) })
}

// GetSkillCategories handles GET /skills/categories.
func (h *SkillHandler) GetSkillCategories(w http.ResponseWriter, r *http.Request) {
	h.skillCall(w, r, "GetSkillCategories", "Failed to load the skill categories.", http.StatusOK,
		func() ([]byte, error) { return h.entity.GetSkillCategories(r.Context()) })
}

// SearchSkills handles POST /skills/search.
func (h *SkillHandler) SearchSkills(w http.ResponseWriter, r *http.Request) {
	h.skillBodyCall(w, r, "SearchSkills", "Failed to search skills.", http.StatusOK,
		func(body []byte) ([]byte, error) { return h.entity.SearchSkills(r.Context(), body) })
}

// GetSkill handles GET /skills/{id}.
func (h *SkillHandler) GetSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := skillPathID(w, r, "id")
	if !ok {
		return
	}
	h.skillCall(w, r, "GetSkill", "Failed to load the skill.", http.StatusOK,
		func() ([]byte, error) { return h.entity.GetSkill(r.Context(), id) })
}

// CreateSkill handles POST /skills. Admin only; entity-service decides.
func (h *SkillHandler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	h.skillBodyCall(w, r, "CreateSkill", "Failed to create the skill.", http.StatusCreated,
		func(body []byte) ([]byte, error) { return h.entity.CreateSkill(r.Context(), body) })
}

// UpdateSkill handles PATCH /skills/{id}. Admin only; entity-service decides.
func (h *SkillHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := skillPathID(w, r, "id")
	if !ok {
		return
	}
	h.skillBodyCall(w, r, "UpdateSkill", "Failed to update the skill.", http.StatusOK,
		func(body []byte) ([]byte, error) { return h.entity.UpdateSkill(r.Context(), id, body) })
}

// GetMySkills handles GET /skills/me.
func (h *SkillHandler) GetMySkills(w http.ResponseWriter, r *http.Request) {
	h.skillCall(w, r, "GetMySkills", "Failed to load your skills.", http.StatusOK,
		func() ([]byte, error) { return h.entity.GetMySkills(r.Context()) })
}

// AddMySkill handles POST /skills/me.
func (h *SkillHandler) AddMySkill(w http.ResponseWriter, r *http.Request) {
	h.skillBodyCall(w, r, "AddMySkill", "Failed to add the skill.", http.StatusCreated,
		func(body []byte) ([]byte, error) { return h.entity.AddMySkill(r.Context(), body) })
}

// SetMySkillLevel handles PUT /skills/me/{skillId}.
func (h *SkillHandler) SetMySkillLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := skillPathID(w, r, "skillId")
	if !ok {
		return
	}
	h.skillBodyCall(w, r, "SetMySkillLevel", "Failed to change the skill level.", http.StatusOK,
		func(body []byte) ([]byte, error) { return h.entity.SetMySkillLevel(r.Context(), id, body) })
}

// RemoveMySkill handles DELETE /skills/me/{skillId}.
func (h *SkillHandler) RemoveMySkill(w http.ResponseWriter, r *http.Request) {
	id, ok := skillPathID(w, r, "skillId")
	if !ok {
		return
	}
	h.skillCall(w, r, "RemoveMySkill", "Failed to remove the skill.", http.StatusNoContent,
		func() ([]byte, error) { return h.entity.RemoveMySkill(r.Context(), id) })
}

// GetMySkillHistory handles GET /skills/me/history?skillId=&limit=&offset=.
// limit and offset are passed through for entity-service to validate.
func (h *SkillHandler) GetMySkillHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	skillID := q.Get("skillId")
	if skillID != "" && !uuidRe.MatchString(skillID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	h.skillCall(w, r, "GetMySkillHistory", "Failed to load your skill history.", http.StatusOK,
		func() ([]byte, error) {
			return h.entity.GetMySkillHistory(r.Context(), skillID, q.Get("limit"), q.Get("offset"))
		})
}

// SearchUserSkills handles POST /skills/users/search. Skill manager only;
// entity-service decides.
func (h *SkillHandler) SearchUserSkills(w http.ResponseWriter, r *http.Request) {
	h.skillBodyCall(w, r, "SearchUserSkills", "Failed to search people's skills.", http.StatusOK,
		func(body []byte) ([]byte, error) { return h.entity.SearchUserSkills(r.Context(), body) })
}
