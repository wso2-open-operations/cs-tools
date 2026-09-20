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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MetricsHandler serves GET /metrics/overview and GET /metrics/timeseries.
// Responses are cached in-memory per this replica only —
// under Choreo's multi-replica autoscaling a request can land on any
// replica, so a cache hit/miss here is not cross-replica consistent, which
// is fine: these are read-mostly aggregates a few seconds stale is harmless
// for, not anything requiring the job lock's cross-replica guarantee.
type MetricsHandler struct {
	pool            *pgxpool.Pool
	cfg             *config.AppConfig
	api             appconfig.API
	overviewCache   *metrics.TTLCache[string, metrics.Overview]
	timeseriesCache *metrics.TTLCache[string, metrics.Timeseries]
}

// NewMetricsHandler creates a MetricsHandler.
func NewMetricsHandler(pool *pgxpool.Pool, cfg *config.AppConfig, cacheCfg appconfig.Cache, api appconfig.API) *MetricsHandler {
	return &MetricsHandler{
		pool: pool,
		cfg:  cfg,
		api:  api,
		overviewCache: metrics.NewTTLCache[string, metrics.Overview](
			time.Duration(cacheCfg.Overview.TTLSeconds)*time.Second, cacheCfg.Overview.MaxEntries),
		timeseriesCache: metrics.NewTTLCache[string, metrics.Timeseries](
			time.Duration(cacheCfg.Timeseries.TTLSeconds)*time.Second, cacheCfg.Timeseries.MaxEntries),
	}
}

// GetOverview handles GET /metrics/overview.
func (h *MetricsHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	var repo, priority *string
	if raw := v.Get("repo"); raw != "" {
		if !repoParamRe.MatchString(raw) { // shared with issues_query.go
			apierror.ValidationFailed(w, "repo must be owner/name")
			return
		}
		repo = &raw
	}
	if raw := v.Get("priority"); raw != "" {
		if len(raw) > h.api.PriorityParamMaxLength {
			apierror.ValidationFailed(w, fmt.Sprintf("priority must be at most %d characters", h.api.PriorityParamMaxLength))
			return
		}
		priority = &raw
	}

	key := fmt.Sprintf("%v|%v", derefOr(repo, ""), derefOr(priority, ""))
	result, err := h.overviewCache.GetOrSet(key, func() (metrics.Overview, error) {
		return metrics.BuildOverview(r.Context(), h.pool, h.cfg, repo, priority)
	})
	if err != nil {
		apierror.Internal(w, r, "build overview failed", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetTimeseries handles GET /metrics/timeseries.
func (h *MetricsHandler) GetTimeseries(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	var repo *string
	if raw := v.Get("repo"); raw != "" {
		if !repoParamRe.MatchString(raw) {
			apierror.ValidationFailed(w, "repo must be owner/name")
			return
		}
		repo = &raw
	}

	days := h.api.TimeseriesDefaultDays
	if raw := v.Get("days"); raw != "" {
		n, err := parseIntInRange(raw, h.api.TimeseriesMinDays, h.api.TimeseriesMaxDays)
		if err != nil {
			apierror.ValidationFailed(w, fmt.Sprintf("days must be an integer between %d and %d", h.api.TimeseriesMinDays, h.api.TimeseriesMaxDays))
			return
		}
		days = n
	}

	groupBy := "priority"
	if raw := v.Get("groupBy"); raw != "" {
		if raw != "priority" && raw != "none" {
			apierror.ValidationFailed(w, "groupBy must be priority or none")
			return
		}
		groupBy = raw
	}

	metric := "violated"
	if raw := v.Get("metric"); raw != "" {
		switch raw {
		case "violated", "at_risk", "total":
			metric = raw
		default:
			apierror.ValidationFailed(w, "metric must be violated, at_risk, or total")
			return
		}
	}

	key := fmt.Sprintf("%v|%d|%s|%s", derefOr(repo, ""), days, groupBy, metric)
	result, err := h.timeseriesCache.GetOrSet(key, func() (metrics.Timeseries, error) {
		return metrics.BuildTimeseries(r.Context(), h.pool, h.cfg, repo, days, groupBy, metric)
	})
	if err != nil {
		apierror.Internal(w, r, "build timeseries failed", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// derefOr returns *s, or fallback if s is nil.
func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

// parseIntInRange parses raw as an int and rejects it if outside [lo, hi].
func parseIntInRange(raw string, lo, hi int) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("out of range")
	}
	return n, nil
}
