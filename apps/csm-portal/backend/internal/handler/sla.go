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

// This file proxies the SLA module's admin-config surface (entity-service
// migrations 0245-0250): field registry, calendars, definitions,
// notification rules. Every route is registered with PermAdmin -- the same
// "no non-admin staff role should ever reach this" gate the rota admin
// grants above use, since SLA configuration is equally global, admin-only
// config with no per-project scoping to narrow it by. entity-service
// re-checks the caller is an admin itself (RequireInternalCaller plus
// AUTH_ADMIN_ROLES) regardless -- this permission is the coarse, route-level
// line, not the only one.
//
// Raw passthrough: this handler forwards whatever body the caller sent and
// whatever entity-service answers, the same convention
// ProjectHandler.UpdateProject already uses for most of this backend --
// entity-service itself does the real request-shape and business-rule
// validation (predicate JSON, threshold rules, status transitions), so
// duplicating it here would only let the two drift.

// entitySLAClient is the entity-service calls this handler needs.
type entitySLAClient interface {
	GetSLAWorkItemTypes(ctx context.Context) ([]byte, error)
	GetSLAFieldCatalog(ctx context.Context, workItemType string) ([]byte, error)
	SetSLAFieldEnabled(ctx context.Context, fieldCatalogEntryID string, body []byte) ([]byte, error)
	ListSLACalendars(ctx context.Context) ([]byte, error)
	GetSLACalendar(ctx context.Context, id string) ([]byte, error)
	UpdateSLACalendarWorkingWeek(ctx context.Context, id string, body []byte) ([]byte, error)
	AddSLAHoliday(ctx context.Context, calendarID string, body []byte) ([]byte, error)
	DeleteSLAHoliday(ctx context.Context, calendarID, holidayID string) ([]byte, error)
	SearchSLADefinitions(ctx context.Context, body []byte) ([]byte, error)
	GetSLADefinition(ctx context.Context, id string) ([]byte, error)
	CreateSLADefinition(ctx context.Context, body []byte) ([]byte, error)
	UpdateSLADefinition(ctx context.Context, id string, body []byte) ([]byte, error)
	SetSLADefinitionStatus(ctx context.Context, id string, body []byte) ([]byte, error)
	ListSLANotificationRules(ctx context.Context) ([]byte, error)
	GetSLANotificationRule(ctx context.Context, id string) ([]byte, error)
	CreateSLANotificationRule(ctx context.Context, body []byte) ([]byte, error)
	UpdateSLANotificationRule(ctx context.Context, id string, body []byte) ([]byte, error)
}

// SLAHandler proxies the SLA module's admin-config routes.
type SLAHandler struct {
	entity entitySLAClient
}

// NewSLAHandler creates an SLAHandler.
func NewSLAHandler(entity entitySLAClient) *SLAHandler {
	return &SLAHandler{entity: entity}
}

// slaForward authenticates the caller and writes the result of call, mapping
// a refusal from entity-service the same way rota access / case alert
// settings do: keep its own reason for a 400/403/404/409, fall back to a
// generic message for anything else.
func (h *SLAHandler) slaForward(w http.ResponseWriter, r *http.Request, op, fallbackMsg string, okStatus int,
	call func(ctx context.Context) ([]byte, error)) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := call(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", user.UserID, "err", err)
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
				writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
				return
			}
		}
		mapUpstreamErrorGeneric(w, err, fallbackMsg)
		return
	}
	if okStatus == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, okStatus, result)
}

// slaForwardWithBody is slaForward plus reading and forwarding the request
// body.
func (h *SLAHandler) slaForwardWithBody(w http.ResponseWriter, r *http.Request, op, fallbackMsg string, okStatus int,
	call func(ctx context.Context, body []byte) ([]byte, error)) {
	body, _, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	h.slaForward(w, r, op, fallbackMsg, okStatus, func(ctx context.Context) ([]byte, error) { return call(ctx, body) })
}

// GetSLAWorkItemTypes handles GET /sla/work-item-types.
func (h *SLAHandler) GetSLAWorkItemTypes(w http.ResponseWriter, r *http.Request) {
	h.slaForward(w, r, "GetSLAWorkItemTypes", "Failed to load SLA work item types.", http.StatusOK, h.entity.GetSLAWorkItemTypes)
}

// GetSLAFieldCatalog handles GET /sla/field-catalog.
func (h *SLAHandler) GetSLAFieldCatalog(w http.ResponseWriter, r *http.Request) {
	h.slaForward(w, r, "GetSLAFieldCatalog", "Failed to load the SLA field catalog.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) {
			return h.entity.GetSLAFieldCatalog(ctx, r.URL.Query().Get("workItemType"))
		})
}

// SetSLAFieldEnabled handles PATCH /sla/field-catalog/{fieldCatalogEntryId}/enablement.
func (h *SLAHandler) SetSLAFieldEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("fieldCatalogEntryId")
	h.slaForwardWithBody(w, r, "SetSLAFieldEnabled", "Failed to update the field's availability.", http.StatusOK,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.SetSLAFieldEnabled(ctx, id, body)
		})
}

// ListSLACalendars handles GET /sla/calendars.
func (h *SLAHandler) ListSLACalendars(w http.ResponseWriter, r *http.Request) {
	h.slaForward(w, r, "ListSLACalendars", "Failed to load SLA calendars.", http.StatusOK, h.entity.ListSLACalendars)
}

// GetSLACalendar handles GET /sla/calendars/{id}.
func (h *SLAHandler) GetSLACalendar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForward(w, r, "GetSLACalendar", "Failed to load the SLA calendar.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.GetSLACalendar(ctx, id) })
}

// UpdateSLACalendarWorkingWeek handles PATCH /sla/calendars/{id}.
func (h *SLAHandler) UpdateSLACalendarWorkingWeek(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForwardWithBody(w, r, "UpdateSLACalendarWorkingWeek", "Failed to save the calendar's working week.", http.StatusOK,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.UpdateSLACalendarWorkingWeek(ctx, id, body)
		})
}

// AddSLAHoliday handles POST /sla/calendars/{id}/holidays.
func (h *SLAHandler) AddSLAHoliday(w http.ResponseWriter, r *http.Request) {
	calendarID := r.PathValue("id")
	h.slaForwardWithBody(w, r, "AddSLAHoliday", "Failed to add the holiday.", http.StatusCreated,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.AddSLAHoliday(ctx, calendarID, body)
		})
}

// DeleteSLAHoliday handles DELETE /sla/calendars/{id}/holidays/{holidayId}.
func (h *SLAHandler) DeleteSLAHoliday(w http.ResponseWriter, r *http.Request) {
	calendarID, holidayID := r.PathValue("id"), r.PathValue("holidayId")
	h.slaForward(w, r, "DeleteSLAHoliday", "Failed to remove the holiday.", http.StatusNoContent,
		func(ctx context.Context) ([]byte, error) {
			return h.entity.DeleteSLAHoliday(ctx, calendarID, holidayID)
		})
}

// SearchSLADefinitions handles POST /sla/definitions/search.
func (h *SLAHandler) SearchSLADefinitions(w http.ResponseWriter, r *http.Request) {
	h.slaForwardWithBody(w, r, "SearchSLADefinitions", "Failed to search SLA definitions.", http.StatusOK, h.entity.SearchSLADefinitions)
}

// GetSLADefinition handles GET /sla/definitions/{id}.
func (h *SLAHandler) GetSLADefinition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForward(w, r, "GetSLADefinition", "Failed to load the SLA definition.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.GetSLADefinition(ctx, id) })
}

// CreateSLADefinition handles POST /sla/definitions.
func (h *SLAHandler) CreateSLADefinition(w http.ResponseWriter, r *http.Request) {
	h.slaForwardWithBody(w, r, "CreateSLADefinition", "Failed to create the SLA definition.", http.StatusCreated, h.entity.CreateSLADefinition)
}

// UpdateSLADefinition handles PATCH /sla/definitions/{id}.
func (h *SLAHandler) UpdateSLADefinition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForwardWithBody(w, r, "UpdateSLADefinition", "Failed to save the SLA definition.", http.StatusOK,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.UpdateSLADefinition(ctx, id, body)
		})
}

// SetSLADefinitionStatus handles PATCH /sla/definitions/{id}/status.
func (h *SLAHandler) SetSLADefinitionStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForwardWithBody(w, r, "SetSLADefinitionStatus", "Failed to change the SLA definition's status.", http.StatusOK,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.SetSLADefinitionStatus(ctx, id, body)
		})
}

// ListSLANotificationRules handles GET /sla/notification-rules.
func (h *SLAHandler) ListSLANotificationRules(w http.ResponseWriter, r *http.Request) {
	h.slaForward(w, r, "ListSLANotificationRules", "Failed to load SLA notification rules.", http.StatusOK, h.entity.ListSLANotificationRules)
}

// GetSLANotificationRule handles GET /sla/notification-rules/{id}.
func (h *SLAHandler) GetSLANotificationRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForward(w, r, "GetSLANotificationRule", "Failed to load the SLA notification rule.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.GetSLANotificationRule(ctx, id) })
}

// CreateSLANotificationRule handles POST /sla/notification-rules.
func (h *SLAHandler) CreateSLANotificationRule(w http.ResponseWriter, r *http.Request) {
	h.slaForwardWithBody(w, r, "CreateSLANotificationRule", "Failed to create the SLA notification rule.", http.StatusCreated, h.entity.CreateSLANotificationRule)
}

// UpdateSLANotificationRule handles PATCH /sla/notification-rules/{id}.
func (h *SLAHandler) UpdateSLANotificationRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.slaForwardWithBody(w, r, "UpdateSLANotificationRule", "Failed to save the SLA notification rule.", http.StatusOK,
		func(ctx context.Context, body []byte) ([]byte, error) {
			return h.entity.UpdateSLANotificationRule(ctx, id, body)
		})
}
