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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// usageMetricsClient abstracts the entity-service operations used by
// UsageMetricsHandler.
type usageMetricsClient interface {
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

// UsageMetricsHandler handles HTTP requests for the usage-metrics domain
// (/usage-metrics/*), delegating to entity-service.
// Every endpoint in this domain requires PermUsageMetricsViewer directly —
// unlike this domain's former sibling, the time-log project
// report, it is NOT layered on top of the narrower PermViewerAccess
// (Viewer-only) gate (and Customer Health has its own, PermViewCustomerHealth);
// see PermUsageMetricsViewer's own doc comment for why.
type UsageMetricsHandler struct {
	client      usageMetricsClient
	accessGuard *AccessGuard
}

// NewUsageMetricsHandler creates a UsageMetricsHandler.
func NewUsageMetricsHandler(client usageMetricsClient, accessGuard *AccessGuard) *UsageMetricsHandler {
	return &UsageMetricsHandler{client: client, accessGuard: accessGuard}
}

// authorize is the permission gate common to every handler in this file.
// Returns false (response already written) if it fails.
func (h *UsageMetricsHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return false
	}
	if !h.accessGuard.Permits(PermUsageMetricsViewer, user.Roles) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return false
	}
	return true
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
		slog.ErrorContext(r.Context(), "GetAllProjects failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchInstanceMetrics failed", "err", err)
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
		slog.ErrorContext(r.Context(), "GetInstanceMetricsStats failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchInstanceUsages failed", "err", err)
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
		slog.ErrorContext(r.Context(), "GetInstanceUsagesStats failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchUsageMetricsDeployments failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchUsageMetricsProjects failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchUsageMetricsDeployedProducts failed", "err", err)
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
		slog.ErrorContext(r.Context(), "SearchUsageMetricsInstances failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search instances.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// deployedProductMetricsDateRange is the minimal shape read out of the
// request body to run validateMetricsDateRange before forwarding — the
// original raw body (not a re-marshaled struct) is what's actually
// forwarded upstream.
type deployedProductMetricsDateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// maxMetricsDateRangeDays mirrors Ballerina's
// MAX_METRICS_DATE_RANGE_DAYS constant.
const maxMetricsDateRangeDays = 366

// validateMetricsDateRange mirrors Ballerina's validateMetricsDateRange:
// startDate/endDate must be valid YYYY-MM-DD dates, endDate must not be
// before startDate, and the span between them must not exceed
// maxMetricsDateRangeDays. Returns a caller-facing error message (safe to
// return directly in a 400 response, matching how the Ballerina original's
// message is shown to the caller) or nil if the range is valid.
func validateMetricsDateRange(startDate, endDate string) error {
	const layout = "2006-01-02"
	start, startErr := time.Parse(layout, startDate)
	end, endErr := time.Parse(layout, endDate)
	if startErr != nil || endErr != nil {
		return errors.New("startDate and endDate must be valid dates in YYYY-MM-DD format")
	}
	diff := end.Sub(start)
	if diff < 0 {
		return errors.New("endDate must not be before startDate")
	}
	if diff > maxMetricsDateRangeDays*24*time.Hour {
		return fmt.Errorf("date range must not exceed %d days", maxMetricsDateRangeDays)
	}
	return nil
}

// validateDeployedProductDateRange decodes startDate/endDate out of body and
// validates them via validateMetricsDateRange. Writes a 400 and returns
// false on an invalid range or unparseable body.
func validateDeployedProductDateRange(w http.ResponseWriter, body []byte) bool {
	var dateRange deployedProductMetricsDateRange
	if err := json.Unmarshal(body, &dateRange); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return false
	}
	if err := validateMetricsDateRange(dateRange.StartDate, dateRange.EndDate); err != nil {
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
		slog.ErrorContext(r.Context(), "GetDeployedProductMetrics failed", "deployedProductID", id, "err", err)
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
		slog.ErrorContext(r.Context(), "GetDeployedProductUsageCounts failed", "deployedProductID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve deployed product usage counts.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
