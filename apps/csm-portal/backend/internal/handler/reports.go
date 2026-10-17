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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// reportsClient abstracts the report operations used by ReportsHandler.
type reportsClient interface {
	GetProjectReportDetails(ctx context.Context, projectID, from, to string) (viewer.CSReportDetails, error)
	GetTimeLogBreakdown(ctx context.Context, projectID string) (viewer.TimeLogBreakdownDetails, error)
}

// ReportsHandler handles HTTP requests for the project reports, delegating
// to entity-service.
type ReportsHandler struct {
	reports     reportsClient
	accessGuard *AccessGuard
}

// NewReportsHandler creates a ReportsHandler. accessGuard enforces PermView,
// the same read audience as the rest of the portal, so a cs_engineer or admin
// can open these reports from a project page without also holding Viewer.
func NewReportsHandler(reports reportsClient, accessGuard *AccessGuard) *ReportsHandler {
	return &ReportsHandler{reports: reports, accessGuard: accessGuard}
}

// requireReportAccess allows anyone who can already read the portal (PermView).
func requireReportAccess(w http.ResponseWriter, r *http.Request, guard *AccessGuard) (*middleware.UserInfo, bool) {
	return requirePermission(w, r, guard, PermView)
}

// GetReportDetails handles GET /report-details.
func (h *ReportsHandler) GetReportDetails(w http.ResponseWriter, r *http.Request) {
	user, ok := requireReportAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	q := r.URL.Query()
	projectID, from, to := q.Get("projectSysId"), q.Get("from"), q.Get("to")
	if projectID == "" || from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "projectSysId, from, and to are required.")
		return
	}
	if _, ok := parseReportTime(from); !ok {
		writeError(w, http.StatusBadRequest, "from must be a date (YYYY-MM-DD) or RFC 3339 timestamp.")
		return
	}
	if _, ok := parseReportTime(to); !ok {
		writeError(w, http.StatusBadRequest, "to must be a date (YYYY-MM-DD) or RFC 3339 timestamp.")
		return
	}

	report, err := h.reports.GetProjectReportDetails(r.Context(), projectID, from, to)
	if err != nil {
		if errors.Is(err, viewer.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetProjectReportDetails failed", "userID", user.UserID, "projectId", projectID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve report details.")
		return
	}

	writeJSONValue(w, http.StatusOK, report)
}

// GenerateTimelogsBreakdownReport handles GET /generate-timelogs-breakdown-report.
func (h *ReportsHandler) GenerateTimelogsBreakdownReport(w http.ResponseWriter, r *http.Request) {
	user, ok := requireReportAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "projectId is required.")
		return
	}

	report, err := h.reports.GetTimeLogBreakdown(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, viewer.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetTimeLogBreakdown failed", "userID", user.UserID, "projectId", projectID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to generate timelogs breakdown report.")
		return
	}

	writeJSONValue(w, http.StatusOK, report)
}
