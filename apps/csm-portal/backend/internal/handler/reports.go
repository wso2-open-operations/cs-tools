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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// reportsClient abstracts the backing-system report operations used by
// ReportsHandler.
type reportsClient interface {
	GetSLAReport(ctx context.Context, projectSysID, from, to string) (servicenow.SLAReportDetails, error)
	GetProjectReportDetails(ctx context.Context, projectSysID, from, to string) (servicenow.CSReportDetails, error)
	GetTimeLogBreakdown(ctx context.Context, projectID string) (servicenow.TimeLogBreakdownDetails, error)
}

// ReportsHandler handles HTTP requests for SupportPortalLite's
// project-level reports, delegating to the backing system service.
type ReportsHandler struct {
	servicenow  reportsClient
	accessGuard *AccessGuard
}

// NewReportsHandler creates a ReportsHandler backed by the given
// The backing system client. accessGuard enforces PermViewerAccess, SupportPortalLite's
// blanket audience gate.
func NewReportsHandler(sn reportsClient, accessGuard *AccessGuard) *ReportsHandler {
	return &ReportsHandler{servicenow: sn, accessGuard: accessGuard}
}

// GenerateSLAReport handles GET /generate-sla-report.
func (h *ReportsHandler) GenerateSLAReport(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	q := r.URL.Query()
	projectSysID, from, to := q.Get("projectSysId"), q.Get("from"), q.Get("to")
	if projectSysID == "" || from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "projectSysId, from, and to are required.")
		return
	}
	if err := servicenow.SanitizeQueryValue(projectSysID); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	report, err := h.servicenow.GetSLAReport(r.Context(), projectSysID, from, to)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetSLAReport failed", "userID", user.UserID, "projectSysId", projectSysID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to generate SLA report.")
		return
	}

	writeJSONValue(w, http.StatusOK, report)
}

// GetReportDetails handles GET /report-details.
func (h *ReportsHandler) GetReportDetails(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	q := r.URL.Query()
	projectSysID, from, to := q.Get("projectSysId"), q.Get("from"), q.Get("to")
	if projectSysID == "" || from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "projectSysId, from, and to are required.")
		return
	}
	if err := servicenow.SanitizeQueryValue(projectSysID); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	report, err := h.servicenow.GetProjectReportDetails(r.Context(), projectSysID, from, to)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetProjectReportDetails failed", "userID", user.UserID, "projectSysId", projectSysID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve report details.")
		return
	}

	writeJSONValue(w, http.StatusOK, report)
}

// GenerateTimelogsBreakdownReport handles GET /generate-timelogs-breakdown-report.
func (h *ReportsHandler) GenerateTimelogsBreakdownReport(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "projectId is required.")
		return
	}
	if err := servicenow.SanitizeQueryValue(projectID); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	report, err := h.servicenow.GetTimeLogBreakdown(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, servicenow.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetTimeLogBreakdown failed", "userID", user.UserID, "projectId", projectID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to generate timelogs breakdown report.")
		return
	}

	writeJSONValue(w, http.StatusOK, report)
}
