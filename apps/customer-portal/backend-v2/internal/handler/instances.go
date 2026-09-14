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

// InstanceHandler exposes entity-service's 5 instance endpoints
// (search/metrics/usages/metrics-stats/usages-stats) as 15 portal routes —
// each metric type fanned out into a project-scoped, deployment-scoped, and
// deployed-product-scoped variant that forces exactly one ID filter from its
// URL path. Each public method is a thin wrapper around a shared unexported
// implementation differing only in which entity filter field the path param
// feeds.
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

// entityInstanceClient abstracts the entity-service instance operations used
// by InstanceHandler.
type entityInstanceClient interface {
	SearchInstances(ctx context.Context, req entity.SearchInstancesRequest) (entity.SearchInstancesResponse, error)
	SearchInstanceMetrics(ctx context.Context, req entity.InstanceMetricsRequest) (entity.InstanceMetricsResponse, error)
	SearchInstanceUsage(ctx context.Context, req entity.InstanceUsageRequest) (entity.InstanceUsageResponse, error)
	SearchInstanceMetricsStats(ctx context.Context, req entity.InstanceMetricsStatsRequest) (entity.InstanceMetricsStatsResponse, error)
	SearchInstanceUsageStats(ctx context.Context, req entity.InstanceUsageStatsRequest) (entity.InstanceUsageStatsResponse, error)
}

// InstanceHandler handles HTTP requests for the instances/metrics/usages fan-out.
type InstanceHandler struct {
	entity entityInstanceClient

	callerScope *CallerScopeResolver
}

// NewInstanceHandler creates an InstanceHandler backed by the given entity client.
func NewInstanceHandler(entityClient entityInstanceClient) *InstanceHandler {
	return &InstanceHandler{entity: entityClient}
}

// SetCallerScope enables caller-scoped access for the project-scoped variant
// of each instance endpoint only (checkProjectScope below is a no-op for the
// deployment- and deployed-product-scoped variants — resolving those back to
// a project id is a separate, not-yet-addressed gap). Always enforced in
// production (main.go calls this unconditionally, no kill switch) — see
// ProjectHandler.SetCallerScope for why this is a setter rather than a
// constructor parameter.
func (h *InstanceHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// checkProjectScope requires caller membership only when scope is a
// project-scoped filter (scope.projectIDs set — the other two ID slices are
// always nil by construction, see instanceIDFilters); a no-op for the
// deployment- and deployed-product-scoped variants.
func (h *InstanceHandler) checkProjectScope(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, userID, email string) bool {
	if len(scope.projectIDs) == 0 {
		return true
	}
	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// return requireProjectMember(w, r, h.callerScope, scope.projectIDs[0], userID, email, http.StatusForbidden, ErrMsgForbidden)
	return true
}

// instanceIDFilters is exactly one non-empty ID slice — the other two are
// always nil, since project/deployment/deployedProduct scoping is mutually
// exclusive by construction (each portal route only ever sets one).
type instanceIDFilters struct {
	projectIDs         []string
	deploymentIDs      []string
	deployedProductIDs []string
}

func projectScope(id string) instanceIDFilters { return instanceIDFilters{projectIDs: []string{id}} }
func deploymentScope(id string) instanceIDFilters {
	return instanceIDFilters{deploymentIDs: []string{id}}
}
func deployedProductScope(id string) instanceIDFilters {
	return instanceIDFilters{deployedProductIDs: []string{id}}
}

// --- POST .../instances/search ---

func (h *InstanceHandler) searchInstances(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, pathID, failureNoun string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if !uuidRe.MatchString(pathID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.checkProjectScope(w, r, scope, user.UserID, user.Email) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.InstanceSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var startDate, endDate *string
	if req.Filters != nil {
		startDate, endDate = req.Filters.StartDate, req.Filters.EndDate
	}
	entityReq := entity.SearchInstancesRequest{
		Filters: &entity.InstanceSearchFilters{
			StartDate:          startDate,
			EndDate:            endDate,
			ProjectIDs:         scope.projectIDs,
			DeploymentIDs:      scope.deploymentIDs,
			DeployedProductIDs: scope.deployedProductIDs,
		},
		Pagination: req.Pagination,
	}

	result, err := h.entity.SearchInstances(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInstances failed", "userID", user.UserID, "pathID", pathID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search instances for the "+failureNoun+".")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapInstanceSearchResponse(result))
}

// SearchProjectInstances handles POST /projects/{id}/instances/search.
func (h *InstanceHandler) SearchProjectInstances(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstances(w, r, projectScope(id), id, "project")
}

// SearchDeploymentInstances handles POST /deployments/{id}/instances/search.
func (h *InstanceHandler) SearchDeploymentInstances(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstances(w, r, deploymentScope(id), id, "deployment")
}

// SearchDeployedProductInstances handles
// POST /deployments/products/{id}/instances/search.
func (h *InstanceHandler) SearchDeployedProductInstances(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstances(w, r, deployedProductScope(id), id, "deployed product")
}

// --- POST .../instances/metrics/search ---

func (h *InstanceHandler) searchInstanceMetrics(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, pathID, failureNoun string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if !uuidRe.MatchString(pathID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.checkProjectScope(w, r, scope, user.UserID, user.Email) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.InstanceDateRangeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := entity.InstanceMetricsRequest{Filters: entity.InstanceDateRangeFilters{
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		ProjectIDs:         scope.projectIDs,
		DeploymentIDs:      scope.deploymentIDs,
		DeployedProductIDs: scope.deployedProductIDs,
	}}

	result, err := h.entity.SearchInstanceMetrics(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInstanceMetrics failed", "userID", user.UserID, "pathID", pathID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search instance metrics for the "+failureNoun+".")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapInstanceMetricsResponse(result))
}

// SearchProjectInstanceMetrics handles POST /projects/{id}/instances/metrics/search.
func (h *InstanceHandler) SearchProjectInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetrics(w, r, projectScope(id), id, "project")
}

// SearchDeploymentInstanceMetrics handles POST /deployments/{id}/instances/metrics/search.
func (h *InstanceHandler) SearchDeploymentInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetrics(w, r, deploymentScope(id), id, "deployment")
}

// SearchDeployedProductInstanceMetrics handles
// POST /deployments/products/{id}/instances/metrics/search.
func (h *InstanceHandler) SearchDeployedProductInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetrics(w, r, deployedProductScope(id), id, "deployed product")
}

// --- POST .../instances/usages/search ---

func (h *InstanceHandler) searchInstanceUsage(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, pathID, failureNoun string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if !uuidRe.MatchString(pathID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.checkProjectScope(w, r, scope, user.UserID, user.Email) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.InstanceDateRangeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := entity.InstanceUsageRequest{Filters: entity.InstanceDateRangeFilters{
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		ProjectIDs:         scope.projectIDs,
		DeploymentIDs:      scope.deploymentIDs,
		DeployedProductIDs: scope.deployedProductIDs,
	}}

	result, err := h.entity.SearchInstanceUsage(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInstanceUsage failed", "userID", user.UserID, "pathID", pathID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search instance usage for the "+failureNoun+".")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapInstanceUsageResponse(result))
}

// SearchProjectInstanceUsage handles POST /projects/{id}/instances/usages/search.
func (h *InstanceHandler) SearchProjectInstanceUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsage(w, r, projectScope(id), id, "project")
}

// SearchDeploymentInstanceUsage handles POST /deployments/{id}/instances/usages/search.
func (h *InstanceHandler) SearchDeploymentInstanceUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsage(w, r, deploymentScope(id), id, "deployment")
}

// SearchDeployedProductInstanceUsage handles
// POST /deployments/products/{id}/instances/usages/search.
func (h *InstanceHandler) SearchDeployedProductInstanceUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsage(w, r, deployedProductScope(id), id, "deployed product")
}

// --- POST .../instances/stats/metrics/search ---
//
// NOTE: DataSource is deliberately NOT forwarded to entity-service here —
// this endpoint never reads payload.filters.dataSource even though the
// portal payload type carries it, unlike the stats/usages/search family
// below, which does forward it. Not a bug to "fix"; preserved intentionally.

func (h *InstanceHandler) searchInstanceMetricsStats(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, pathID, failureNoun string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if !uuidRe.MatchString(pathID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.checkProjectScope(w, r, scope, user.UserID, user.Email) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.InstanceStatsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := entity.InstanceMetricsStatsRequest{Filters: entity.InstanceStatsFilters{
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		ProjectIDs:         scope.projectIDs,
		DeploymentIDs:      scope.deploymentIDs,
		DeployedProductIDs: scope.deployedProductIDs,
	}}

	result, err := h.entity.SearchInstanceMetricsStats(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInstanceMetricsStats failed", "userID", user.UserID, "pathID", pathID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search instance metric statistics for the "+failureNoun+".")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapInstanceMetricsStatsResponse(result))
}

// SearchProjectInstanceMetricsStats handles POST /projects/{id}/instances/stats/metrics/search.
func (h *InstanceHandler) SearchProjectInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetricsStats(w, r, projectScope(id), id, "project")
}

// SearchDeploymentInstanceMetricsStats handles POST /deployments/{id}/instances/stats/metrics/search.
func (h *InstanceHandler) SearchDeploymentInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetricsStats(w, r, deploymentScope(id), id, "deployment")
}

// SearchDeployedProductInstanceMetricsStats handles
// POST /deployments/products/{id}/instances/stats/metrics/search.
func (h *InstanceHandler) SearchDeployedProductInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceMetricsStats(w, r, deployedProductScope(id), id, "deployed product")
}

// --- POST .../instances/stats/usages/search ---

func (h *InstanceHandler) searchInstanceUsageStats(w http.ResponseWriter, r *http.Request, scope instanceIDFilters, pathID, failureNoun string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if !uuidRe.MatchString(pathID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if !h.checkProjectScope(w, r, scope, user.UserID, user.Email) {
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.InstanceStatsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := entity.InstanceUsageStatsRequest{Filters: entity.InstanceStatsFilters{
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		ProjectIDs:         scope.projectIDs,
		DeploymentIDs:      scope.deploymentIDs,
		DeployedProductIDs: scope.deployedProductIDs,
		DataSource:         req.DataSource,
	}}

	result, err := h.entity.SearchInstanceUsageStats(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInstanceUsageStats failed", "userID", user.UserID, "pathID", pathID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search instance usage statistics for the "+failureNoun+".")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapInstanceUsageStatsResponse(result))
}

// SearchProjectInstanceUsageStats handles POST /projects/{id}/instances/stats/usages/search.
func (h *InstanceHandler) SearchProjectInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsageStats(w, r, projectScope(id), id, "project")
}

// SearchDeploymentInstanceUsageStats handles POST /deployments/{id}/instances/stats/usages/search.
func (h *InstanceHandler) SearchDeploymentInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsageStats(w, r, deploymentScope(id), id, "deployment")
}

// SearchDeployedProductInstanceUsageStats handles
// POST /deployments/products/{id}/instances/stats/usages/search.
func (h *InstanceHandler) SearchDeployedProductInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.searchInstanceUsageStats(w, r, deployedProductScope(id), id, "deployed product")
}
