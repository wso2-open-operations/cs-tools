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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// This file is the admin-config surface of the SLA module -- the field
// registry, calendars, definitions and notification rules migrations
// 0245-0250 back. Every service here already enforces its own
// internal-caller-plus-admin-role gate (service.requireSLAAdmin); these
// handlers only move JSON. The matching engine and the live sla_instance
// read API are a separate, not-yet-built piece (see this change's own plan
// file).

func writeSLAJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// SLAFieldRegistryHandler serves GET /sla/work-item-types, GET
// /sla/field-catalog and PATCH /sla/field-catalog/{id}/enablement.
type SLAFieldRegistryHandler struct {
	svc service.SLAFieldRegistryService
}

// NewSLAFieldRegistryHandler constructs an SLAFieldRegistryHandler.
func NewSLAFieldRegistryHandler(svc service.SLAFieldRegistryService) *SLAFieldRegistryHandler {
	return &SLAFieldRegistryHandler{svc: svc}
}

func (h *SLAFieldRegistryHandler) ListWorkItemTypes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListWorkItemTypes(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, map[string]any{"workItemTypes": out})
}

func (h *SLAFieldRegistryHandler) ListFieldCatalog(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListFieldCatalog(r.Context(), r.URL.Query().Get("workItemType"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, map[string]any{"fields": out})
}

func (h *SLAFieldRegistryHandler) SetFieldEnabled(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateSLAFieldEnablementRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.SetFieldEnabled(r.Context(), r.PathValue("fieldCatalogEntryId"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

// SLACalendarHandler serves GET /sla/calendars[/{id}], PATCH
// /sla/calendars/{id}, and the holiday sub-resource under it.
type SLACalendarHandler struct {
	svc service.SLACalendarService
}

// NewSLACalendarHandler constructs an SLACalendarHandler.
func NewSLACalendarHandler(svc service.SLACalendarService) *SLACalendarHandler {
	return &SLACalendarHandler{svc: svc}
}

func (h *SLACalendarHandler) List(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, map[string]any{"calendars": out})
}

func (h *SLACalendarHandler) Get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLACalendarHandler) UpdateWorkingWeek(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateSLACalendarWorkingWeekRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.UpdateWorkingWeek(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLACalendarHandler) AddHoliday(w http.ResponseWriter, r *http.Request) {
	var req domain.AddSLAHolidayRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.AddHoliday(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusCreated, out)
}

func (h *SLACalendarHandler) DeleteHoliday(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteHoliday(r.Context(), r.PathValue("id"), r.PathValue("holidayId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SLADefinitionHandler serves the SLA definition CRUD + status routes.
type SLADefinitionHandler struct {
	svc service.SLADefinitionService
}

// NewSLADefinitionHandler constructs an SLADefinitionHandler.
func NewSLADefinitionHandler(svc service.SLADefinitionService) *SLADefinitionHandler {
	return &SLADefinitionHandler{svc: svc}
}

func (h *SLADefinitionHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchSLADefinitionsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLADefinitionHandler) Get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLADefinitionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.UpsertSLADefinitionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusCreated, out)
}

func (h *SLADefinitionHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req domain.UpsertSLADefinitionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLADefinitionHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	var req domain.SetSLADefinitionStatusRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.SetStatus(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

// SLANotificationRuleHandler serves the SLA notification rule CRUD routes.
type SLANotificationRuleHandler struct {
	svc service.SLANotificationRuleService
}

// NewSLANotificationRuleHandler constructs an SLANotificationRuleHandler.
func NewSLANotificationRuleHandler(svc service.SLANotificationRuleService) *SLANotificationRuleHandler {
	return &SLANotificationRuleHandler{svc: svc}
}

func (h *SLANotificationRuleHandler) List(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, map[string]any{"rules": out})
}

func (h *SLANotificationRuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}

func (h *SLANotificationRuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.UpsertSLANotificationRuleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusCreated, out)
}

func (h *SLANotificationRuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req domain.UpsertSLANotificationRuleRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	out, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSLAJSON(w, http.StatusOK, out)
}
