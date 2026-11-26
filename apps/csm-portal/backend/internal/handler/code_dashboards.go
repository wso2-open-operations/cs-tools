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
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// Messages for an advanced dashboard that cannot be saved, loaded or
// deleted, written to be shown to the user as they are.
const (
	errMsgCodeDashboardNameEmpty    = "The dashboard needs a name before it can be saved."
	errMsgCodeDashboardNameTooLong  = "The dashboard name is too long: keep it under 255 characters."
	errMsgCodeDashboardSourceEmpty  = "There is no dashboard code to save."
	errMsgCodeDashboardTooLarge     = "This dashboard's code is too large to save. Ask for a smaller dashboard."
	errMsgCodeDashboardSaveFailed   = "The dashboard couldn't be saved. Try again in a moment."
	errMsgCodeDashboardListFailed   = "Your saved advanced dashboards couldn't be loaded. Try again in a moment."
	errMsgCodeDashboardLoadFailed   = "The dashboard couldn't be loaded. Try again in a moment."
	errMsgCodeDashboardDeleteFailed = "The dashboard couldn't be deleted. Try again in a moment."
)

// entityCodeDashboardClient is the part of the entity client the code
// dashboard routes use.
type entityCodeDashboardClient interface {
	ListCodeDashboards(ctx context.Context) ([]entity.CodeDashboardSummary, error)
	GetCodeDashboard(ctx context.Context, id string) (entity.CodeDashboard, error)
	SaveCodeDashboard(ctx context.Context, id, name, source string) (entity.CodeDashboard, error)
	DeleteCodeDashboard(ctx context.Context, id string) error
}

// codeDashboardSummary is one entry of GET /dashboards/code: no source.
type codeDashboardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

// codeDashboardListResponse is the body of GET /dashboards/code.
type codeDashboardListResponse struct {
	Dashboards []codeDashboardSummary `json:"dashboards"`
}

// codeDashboardResponse is one saved advanced dashboard with its source: the
// body of GET and PUT /dashboards/code/{id}.
type codeDashboardResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

// saveCodeDashboardRequest is the body of PUT /dashboards/code/{id}: the name
// the user gave it and the source the advanced page generated.
type saveCodeDashboardRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// CodeDashboardHandler serves the caller's saved advanced (code) dashboards,
// stored in entity-service (/users/me/code-dashboards, migration 0295). Like
// the generate-code route it exists only when DASHBOARD_GENERATE_CODE_ENABLED
// is on (main.go). Only the owner can reach a dashboard: another user's is a
// 404, the same as a missing one. The source is checked for size only: it is
// never run here, and the webapp checks it again before every run, in its
// sandbox, where its data comes through the fixed portal APIs with the
// viewer's own permissions.
type CodeDashboardHandler struct {
	entity entityCodeDashboardClient
}

// NewCodeDashboardHandler returns the handler using client.
func NewCodeDashboardHandler(client entityCodeDashboardClient) *CodeDashboardHandler {
	return &CodeDashboardHandler{entity: client}
}

// codeDashboardID reads and checks {id}; on a bad one it has written the 400
// and returns false. The id is lower-cased, as the custom dashboards' are.
func codeDashboardID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", false
	}
	return strings.ToLower(id), true
}

func newCodeDashboardResponse(d entity.CodeDashboard) codeDashboardResponse {
	return codeDashboardResponse{ID: d.ID, Name: d.Name, Source: d.Source, CreatedOn: d.CreatedOn, UpdatedOn: d.UpdatedOn}
}

// ListCodeDashboards handles GET /dashboards/code: the caller's saved
// advanced dashboards, ordered by name (entity-service's order), never null.
func (h *CodeDashboardHandler) ListCodeDashboards(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	list, err := h.entity.ListCodeDashboards(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCodeDashboards failed", "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCodeDashboardListFailed)
		return
	}
	resp := codeDashboardListResponse{Dashboards: make([]codeDashboardSummary, 0, len(list))}
	for _, d := range list {
		resp.Dashboards = append(resp.Dashboards, codeDashboardSummary{
			ID: d.ID, Name: d.Name, CreatedOn: d.CreatedOn, UpdatedOn: d.UpdatedOn,
		})
	}
	writeJSONValue(w, http.StatusOK, resp)
}

// GetCodeDashboard handles GET /dashboards/code/{id}: one of the caller's
// saved advanced dashboards with its source.
func (h *CodeDashboardHandler) GetCodeDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	saved, err := h.entity.GetCodeDashboard(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCodeDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCodeDashboardLoadFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, newCodeDashboardResponse(saved))
}

// SaveCodeDashboard handles PUT /dashboards/code/{id}: saves the advanced
// page's dashboard under the name the user gave it, creating it or replacing
// the caller's own dashboard with this id (200 either way). The name is
// trimmed; the source is stored as sent, at most generatedCodeMaxLength
// characters (the code the generate-code route accepts back), else 413.
// Upstream refusals are passed on with mapUpstreamError, as the custom
// dashboards' save does: a name the user already uses (409), the
// 50-dashboard limit (400). Another user's id is a 404.
func (h *CodeDashboardHandler) SaveCodeDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	var req saveCodeDashboardRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, errMsgCodeDashboardNameEmpty)
		return
	}
	if utf8.RuneCountInString(name) > customDashboardNameMaxLength {
		writeError(w, http.StatusBadRequest, errMsgCodeDashboardNameTooLong)
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		writeError(w, http.StatusBadRequest, errMsgCodeDashboardSourceEmpty)
		return
	}
	if utf8.RuneCountInString(req.Source) > generatedCodeMaxLength {
		writeError(w, http.StatusRequestEntityTooLarge, errMsgCodeDashboardTooLarge)
		return
	}

	saved, err := h.entity.SaveCodeDashboard(r.Context(), id, name, req.Source)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SaveCodeDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamError(w, err, errMsgCodeDashboardSaveFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, newCodeDashboardResponse(saved))
}

// DeleteCodeDashboard handles DELETE /dashboards/code/{id}: removes one of
// the caller's saved advanced dashboards (204).
func (h *CodeDashboardHandler) DeleteCodeDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := codeDashboardID(w, r)
	if !ok {
		return
	}
	if err := h.entity.DeleteCodeDashboard(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteCodeDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCodeDashboardDeleteFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
