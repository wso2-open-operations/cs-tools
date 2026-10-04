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
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// usageMetricsServiceNowClient abstracts the backing-system operations used by
// UsageMetricsHandler.
type usageMetricsServiceNowClient interface {
	GetAllProjects(ctx context.Context, search string) ([]byte, error)
	SearchInstanceMetrics(ctx context.Context, payload []byte) ([]byte, error)
	GetInstanceMetricsStats(ctx context.Context, payload []byte) ([]byte, error)
	SearchInstanceUsages(ctx context.Context, payload []byte) ([]byte, error)
	GetInstanceUsagesStats(ctx context.Context, payload []byte) ([]byte, error)
	SearchUsageMetricsDeployments(ctx context.Context, payload []byte) ([]byte, error)
	SearchUsageMetricsProjects(ctx context.Context, payload []byte) ([]byte, error)
	SearchUsageMetricsDeployedProducts(ctx context.Context, payload []byte) ([]byte, error)
	SearchUsageMetricsInstances(ctx context.Context, payload []byte) ([]byte, error)
	GetDeployedProductMetrics(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error)
	GetDeployedProductUsageCounts(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error)
}

// UsageMetricsHandler handles HTTP requests for the SupportPortalLite
// usage-metrics domain (/usage-metrics/*), delegating to the backing system's
// custom scoped-app API. Every endpoint in this domain requires both the
// blanket PermViewerAccess gate and the narrower PermUsageMetricsViewer gate —
// mirrors Ballerina operations:checkUsageMetricsAccess, which every
// usage-metrics resource function in service.bal calls before anything
// else.
type UsageMetricsHandler struct {
	client      usageMetricsServiceNowClient
	accessGuard *AccessGuard
}

// NewUsageMetricsHandler creates a UsageMetricsHandler.
func NewUsageMetricsHandler(client usageMetricsServiceNowClient, accessGuard *AccessGuard) *UsageMetricsHandler {
	return &UsageMetricsHandler{client: client, accessGuard: accessGuard}
}

// authorize runs both SPL permission gates common to every handler in this
// file. Returns false (response already written) if either check fails.
func (h *UsageMetricsHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return false
	}
	return requireViewerPermission(w, user, h.accessGuard, PermUsageMetricsViewer)
}

// readUsageMetricsBody caps, reads, and JSON-validates a request body, matching the
// existing pattern in accounts.go's SearchAccounts. Writes the appropriate
// error response and returns ok=false on any failure.
func readUsageMetricsBody(w http.ResponseWriter, r *http.Request) (body []byte, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, isTooLarge := err.(*http.MaxBytesError); isTooLarge {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// GetProjects handles GET /usage-metrics/projects.
func (h *UsageMetricsHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}

	search := r.URL.Query().Get("search")

	result, err := h.client.GetAllProjects(r.Context(), search)
	if err != nil {
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetAllProjects failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to list projects.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchInstanceMetrics handles POST /usage-metrics/instances/metrics/search.
func (h *UsageMetricsHandler) SearchInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchInstanceMetrics(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchInstanceMetrics failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search instance metrics.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetInstanceMetricsStats handles POST /usage-metrics/instances/metrics/stats.
func (h *UsageMetricsHandler) GetInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.GetInstanceMetricsStats(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetInstanceMetricsStats failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve instance metrics stats.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchInstanceUsages handles POST /usage-metrics/instances/usages/search.
func (h *UsageMetricsHandler) SearchInstanceUsages(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchInstanceUsages(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchInstanceUsages failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search instance usages.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetInstanceUsagesStats handles POST /usage-metrics/instances/usages/stats.
func (h *UsageMetricsHandler) GetInstanceUsagesStats(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.GetInstanceUsagesStats(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetInstanceUsagesStats failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve instance usages stats.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchDeployments handles POST /usage-metrics/deployments/search.
func (h *UsageMetricsHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchUsageMetricsDeployments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchUsageMetricsDeployments failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search deployments.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchProjects handles POST /usage-metrics/projects/search.
func (h *UsageMetricsHandler) SearchProjects(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchUsageMetricsProjects(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchUsageMetricsProjects failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search projects.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchDeployedProducts handles POST /usage-metrics/deployed-products/search.
func (h *UsageMetricsHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchUsageMetricsDeployedProducts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchUsageMetricsDeployedProducts failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search deployed products.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchInstances handles POST /usage-metrics/instances/search.
func (h *UsageMetricsHandler) SearchInstances(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	result, err := h.client.SearchUsageMetricsInstances(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow SearchUsageMetricsInstances failed", "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search instances.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// deployedProductMetricsDateRange is the minimal shape read out of the
// request body to run ValidateMetricsDateRange before forwarding — the
// original raw body (not a re-marshaled struct) is what's actually
// forwarded upstream.
type deployedProductMetricsDateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// validateDeployedProductDateRange decodes startDate/endDate out of body and
// validates them via servicenow.ValidateMetricsDateRange. Writes a 400 and
// returns false on an invalid range or unparseable body.
func validateDeployedProductDateRange(w http.ResponseWriter, body []byte) bool {
	var dateRange deployedProductMetricsDateRange
	if err := json.Unmarshal(body, &dateRange); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return false
	}
	if err := servicenow.ValidateMetricsDateRange(dateRange.StartDate, dateRange.EndDate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// GetDeployedProductMetrics handles POST /usage-metrics/deployed-products/{id}/metrics/search.
func (h *UsageMetricsHandler) GetDeployedProductMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	if !validateDeployedProductDateRange(w, body) {
		return
	}
	result, err := h.client.GetDeployedProductMetrics(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetDeployedProductMetrics failed", "deployedProductID", id, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve deployed product metrics.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetDeployedProductUsageCounts handles POST /usage-metrics/deployed-products/{id}/metrics/usage-counts/search.
func (h *UsageMetricsHandler) GetDeployedProductUsageCounts(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, ok := readUsageMetricsBody(w, r)
	if !ok {
		return
	}
	if !validateDeployedProductDateRange(w, body) {
		return
	}
	result, err := h.client.GetDeployedProductUsageCounts(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetDeployedProductUsageCounts failed", "deployedProductID", id, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve deployed product usage counts.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
