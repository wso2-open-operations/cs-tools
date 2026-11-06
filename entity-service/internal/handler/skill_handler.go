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
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SkillHandler serves the skills module: the catalogue, a person's own skills
// profile, the manager's view of everybody's, and the admin's catalogue edits.
// The service decides who may do what. No route takes a person's id for the
// caller's own profile: "my" is always the token's user.
type SkillHandler struct {
	svc service.SkillService
}

// NewSkillHandler constructs a SkillHandler.
func NewSkillHandler(svc service.SkillService) *SkillHandler {
	return &SkillHandler{svc: svc}
}

// GetSkillLevels handles GET /skills/levels.
func (h *SkillHandler) GetSkillLevels(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListLevelTypes(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetSkillCategories handles GET /skills/categories.
func (h *SkillHandler) GetSkillCategories(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListCategories(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// SearchSkills handles POST /skills/search.
func (h *SkillHandler) SearchSkills(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchSkillsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchSkills(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetSkill handles GET /skills/{id}.
func (h *SkillHandler) GetSkill(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetSkill(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// CreateSkill handles POST /skills.
func (h *SkillHandler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateSkillRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateSkill(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusCreated, resp)
}

// UpdateSkill handles PATCH /skills/{id}.
func (h *SkillHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateSkillRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.UpdateSkill(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// GetMySkills handles GET /skills/me.
func (h *SkillHandler) GetMySkills(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetMySkills(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// AddMySkill handles POST /skills/me.
func (h *SkillHandler) AddMySkill(w http.ResponseWriter, r *http.Request) {
	var req domain.AddMySkillRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AddMySkill(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusCreated, resp)
}

// SetMySkillLevel handles PUT /skills/me/{skillId}.
func (h *SkillHandler) SetMySkillLevel(w http.ResponseWriter, r *http.Request) {
	var req domain.SetMySkillLevelRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SetMySkillLevel(r.Context(), r.PathValue("skillId"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// RemoveMySkill handles DELETE /skills/me/{skillId}.
func (h *SkillHandler) RemoveMySkill(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveMySkill(r.Context(), r.PathValue("skillId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetMySkillHistory handles GET /skills/me/history?skillId=&limit=&offset=.
func (h *SkillHandler) GetMySkillHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := domain.SkillHistoryRequest{SkillID: q.Get("skillId")}
	for name, dst := range map[string]*int{"limit": &req.Pagination.Limit, "offset": &req.Pagination.Offset} {
		if raw := q.Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				writeServiceError(w, r, &apierror.ValidationError{Msg: name + " must be an integer"})
				return
			}
			*dst = n
		}
	}
	resp, err := h.svc.GetMySkillHistory(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// SearchUserSkills handles POST /skills/users/search.
func (h *SkillHandler) SearchUserSkills(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchUserSkillsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchUserSkills(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}
