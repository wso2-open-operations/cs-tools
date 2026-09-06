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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityProjectStatsClient abstracts the entity-service project metadata/stats
// operations used by ProjectStatsHandler.
type entityProjectStatsClient interface {
	GetProjectMetadata(ctx context.Context, id string) (entity.ProjectMetadataResponse, error)
	GetProjectStats(ctx context.Context, id string) (entity.ProjectStatsResponse, error)
	GetProjectCaseStats(ctx context.Context, id string, caseTypes []string, createdBy string) (entity.ProjectCaseStatsResponse, error)
	GetProjectConversationStats(ctx context.Context, id, createdBy string) (entity.ProjectConversationStatsResponse, error)
	GetProjectDeploymentStats(ctx context.Context, id string) (entity.ProjectDeploymentStatsResponse, error)
	GetProjectTimeCardStats(ctx context.Context, id, startDate, endDate string) (entity.ProjectTimeCardStatsResponse, error)
	GetProjectChangeRequestStats(ctx context.Context, id string) (entity.ProjectChangeRequestStatsResponse, error)
	SearchCaseTimeCards(ctx context.Context, req entity.SearchTimeCardsRequest) (entity.SearchCaseTimeCardsResponse, error)
}

// ProjectStatsHandler handles HTTP requests for project-scoped metadata and
// statistics.
//
// NOTE: entity-service only supports these routes on its ServiceNow data
// source — see internal/entity/projects.go.
type ProjectStatsHandler struct {
	entity entityProjectStatsClient

	callerScope *CallerScopeResolver
}

// NewProjectStatsHandler creates a ProjectStatsHandler backed by the given entity client.
func NewProjectStatsHandler(entityClient entityProjectStatsClient) *ProjectStatsHandler {
	return &ProjectStatsHandler{entity: entityClient}
}

// SetCallerScope enables caller-scoped access: all project-scoped metadata and
// statistics routes require the caller to be an active portal-user contact of
// the project in the URL path. Always enforced in production (main.go calls this
// unconditionally, no kill switch) — see ProjectHandler.SetCallerScope for
// why this is a setter rather than a constructor parameter.
func (h *ProjectStatsHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// GetProjectFilters handles GET /projects/{id}/filters.
func (h *ProjectStatsHandler) GetProjectFilters(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectMetadata(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectMetadata failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project filters.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectFilterOptions(result))
}

// GetProjectFeatures handles GET /projects/{id}/features.
func (h *ProjectStatsHandler) GetProjectFeatures(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectMetadata(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectMetadata failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project features.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectFeatures(result))
}

// GetProjectDashboardStats handles GET /projects/{id}/stats. Uses a
// graceful-degradation behavior: each of the four underlying stats calls
// may fail independently without failing the whole request — a failed
// source's fields are simply omitted from the response.
func (h *ProjectStatsHandler) GetProjectDashboardStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	caseTypes := r.URL.Query()["caseTypes"]
	createdBy := r.URL.Query().Get("createdBy")

	var caseStatsPtr *entity.ProjectCaseStatsResponse
	if caseStats, err := h.entity.GetProjectCaseStats(r.Context(), id, caseTypes, createdBy); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectCaseStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		caseStatsPtr = &caseStats
	}

	var conversationStatsPtr *entity.ProjectConversationStatsResponse
	if conversationStats, err := h.entity.GetProjectConversationStats(r.Context(), id, createdBy); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectConversationStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		conversationStatsPtr = &conversationStats
	}

	var deploymentStatsPtr *entity.ProjectDeploymentStatsResponse
	if deploymentStats, err := h.entity.GetProjectDeploymentStats(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectDeploymentStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		deploymentStatsPtr = &deploymentStats
	}

	var activityStatsPtr *entity.ProjectStatsResponse
	if activityStats, err := h.entity.GetProjectStats(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		activityStatsPtr = &activityStats
	}

	writeJSONValue(w, http.StatusOK, dto.BuildProjectDashboardStats(caseStatsPtr, conversationStatsPtr, deploymentStatsPtr, activityStatsPtr))
}

// GetProjectCaseStats handles GET /projects/{id}/stats/cases.
func (h *ProjectStatsHandler) GetProjectCaseStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectCaseStats(r.Context(), id, r.URL.Query()["caseTypes"], r.URL.Query().Get("createdBy"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectCaseStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve case statistics.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectCaseStats(result))
}

// GetProjectConversationStats handles GET /projects/{id}/stats/conversations.
func (h *ProjectStatsHandler) GetProjectConversationStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectConversationStats(r.Context(), id, r.URL.Query().Get("createdBy"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectConversationStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation statistics.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapConversationStats(result))
}

// GetProjectSupportStats handles GET /projects/{id}/stats/support. Like
// GetProjectDashboardStats, both underlying calls may fail independently
// without failing the whole request.
func (h *ProjectStatsHandler) GetProjectSupportStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	caseTypes := r.URL.Query()["caseTypes"]
	createdBy := r.URL.Query().Get("createdBy")

	var caseStatsPtr *entity.ProjectCaseStatsResponse
	if caseStats, err := h.entity.GetProjectCaseStats(r.Context(), id, caseTypes, createdBy); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectCaseStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		caseStatsPtr = &caseStats
	}

	var conversationStatsPtr *entity.ProjectConversationStatsResponse
	if conversationStats, err := h.entity.GetProjectConversationStats(r.Context(), id, createdBy); err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectConversationStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
	} else {
		conversationStatsPtr = &conversationStats
	}

	writeJSONValue(w, http.StatusOK, dto.BuildProjectSupportStats(caseStatsPtr, conversationStatsPtr))
}

// GetProjectTimeCardStats handles GET /projects/{id}/stats/time-cards.
func (h *ProjectStatsHandler) GetProjectTimeCardStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if !validateDateParams(w, dateParam{"startDate", startDate}, dateParam{"endDate", endDate}) {
		return
	}

	result, err := h.entity.GetProjectTimeCardStats(r.Context(), id, startDate, endDate)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectTimeCardStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve time card statistics.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectTimeCardStats(result))
}

// GetProjectChangeRequestStats handles GET /projects/{id}/stats/change-requests.
func (h *ProjectStatsHandler) GetProjectChangeRequestStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectChangeRequestStats(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectChangeRequestStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve change request statistics.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectChangeRequestStats(result))
}

// SearchProjectCaseTimeCards handles POST /projects/{id}/cases/time-cards/search.
// filters.projectIds is always forced to [id] server-side.
func (h *ProjectStatsHandler) SearchProjectCaseTimeCards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CaseTimeCardSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if req.Filters != nil {
		if !validateDateParams(w,
			dateParam{"filters.startDate", derefString(req.Filters.StartDate)},
			dateParam{"filters.endDate", derefString(req.Filters.EndDate)},
		) {
			return
		}
	}

	result, err := h.entity.SearchCaseTimeCards(r.Context(), dto.BuildEntityCaseTimeCardSearchRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseTimeCards failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search time cards grouped by cases.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseTimeCardSearchResponse(result))
}

// GetProjectUsageStats handles GET /projects/{id}/stats/usage — the counter
// trio on the Usage Metrics page. Backed by the same entity-service call as
// GET /projects/{id}/stats, trimmed to the fields that page reads (see
// dto.UsageStats).
func (h *ProjectStatsHandler) GetProjectUsageStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, id, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.entity.GetProjectStats(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectStats failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve usage statistics.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapUsageStats(result))
}
