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

package appconfig

import (
	"fmt"
	"strings"
)

// headerTokenChars lists the non-alphanumeric bytes RFC 7230 allows in an
// HTTP header field name (the "tchar" set minus letters/digits, which are
// checked separately below).
const headerTokenChars = "!#$%&'*+-.^_`|~" // #nosec G101 -- charset constant for header-name validation, not a credential

// isValidHeaderName reports whether name is a syntactically legal HTTP
// header field name: non-empty, and built only from RFC 7230 token
// characters.
func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune(headerTokenChars, rune(c)):
		default:
			return false
		}
	}
	return true
}

// ValidationError aggregates every rule violation found in one config, so a
// misconfigured deploy sees every problem at once instead of fixing them one
// failed boot at a time.
type ValidationError struct {
	Issues []string
}

// Error renders every collected issue as one multi-line message.
func (e *ValidationError) Error() string {
	return "invalid app config:\n  " + strings.Join(e.Issues, "\n  ")
}

// Validate checks cfg against every configuration rule. Returns nil when cfg
// is valid.
func Validate(cfg *Config) error {
	var issues []string
	add := func(format string, args ...any) {
		issues = append(issues, fmt.Sprintf(format, args...))
	}
	positive := func(field string, v int) {
		if v <= 0 {
			add("%s: must be positive", field)
		}
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		add("server.port: must be between 1 and 65535")
	}
	positive("server.readHeaderTimeoutSeconds", cfg.Server.ReadHeaderTimeoutSeconds)
	positive("server.readTimeoutSeconds", cfg.Server.ReadTimeoutSeconds)
	positive("server.writeTimeoutSeconds", cfg.Server.WriteTimeoutSeconds)
	positive("server.idleTimeoutSeconds", cfg.Server.IdleTimeoutSeconds)
	positive("server.shutdownTimeoutSeconds", cfg.Server.ShutdownTimeoutSeconds)
	positive("server.maxHeaderBytes", cfg.Server.MaxHeaderBytes)

	if cfg.Database.MaxConns != nil && *cfg.Database.MaxConns < 1 {
		add("database.maxConns: must be at least 1")
	}
	if cfg.Database.MinConns != nil && *cfg.Database.MinConns < 0 {
		add("database.minConns: must not be negative")
	}
	if cfg.Database.MaxConns != nil && cfg.Database.MinConns != nil && *cfg.Database.MinConns > *cfg.Database.MaxConns {
		add("database.minConns: must not exceed database.maxConns")
	}
	if cfg.Database.MaxConnLifetimeMinutes != nil && *cfg.Database.MaxConnLifetimeMinutes <= 0 {
		add("database.maxConnLifetimeMinutes: must be positive")
	}
	if cfg.Database.MaxConnIdleTimeMinutes != nil && *cfg.Database.MaxConnIdleTimeMinutes <= 0 {
		add("database.maxConnIdleTimeMinutes: must be positive")
	}
	if cfg.Database.HealthCheckPeriodSeconds != nil && *cfg.Database.HealthCheckPeriodSeconds <= 0 {
		add("database.healthCheckPeriodSeconds: must be positive")
	}
	if cfg.Database.ConnectTimeoutSeconds != nil && *cfg.Database.ConnectTimeoutSeconds <= 0 {
		add("database.connectTimeoutSeconds: must be positive")
	}

	for name, entry := range map[string]CacheEntry{
		"overview":   cfg.Cache.Overview,
		"timeseries": cfg.Cache.Timeseries,
	} {
		positive(fmt.Sprintf("cache.%s.ttlSeconds", name), entry.TTLSeconds)
		if entry.MaxEntries < 1 {
			add("cache.%s.maxEntries: must be at least 1", name)
		}
	}

	if cfg.GitHub.MaxRetries < 0 {
		add("github.maxRetries: must not be negative")
	}
	positive("github.requestTimeoutSeconds", cfg.GitHub.RequestTimeoutSeconds)
	positive("github.retryBackoffUnitSeconds", cfg.GitHub.RetryBackoffUnitSeconds)
	positive("github.retryAfterCapSeconds", cfg.GitHub.RetryAfterCapSeconds)
	positive("github.searchPageDelayMs", cfg.GitHub.SearchPageDelayMs)
	positive("github.detailPageDelayMs", cfg.GitHub.DetailPageDelayMs)

	if cfg.Jobs.RecomputePageSize < 1 {
		add("jobs.recomputePageSize: must be at least 1")
	}
	positive("jobs.syncRunDeadlineMinutes", cfg.Jobs.SyncRunDeadlineMinutes)
	positive("jobs.lockReleaseTimeoutSeconds", cfg.Jobs.LockReleaseTimeoutSeconds)

	if cfg.Seed.InterIssueDelayMs < 0 {
		add("seed.interIssueDelayMs: must not be negative")
	}

	positive("api.issuesDefaultLimit", cfg.API.IssuesDefaultLimit)
	positive("api.issuesMaxLimit", cfg.API.IssuesMaxLimit)
	positive("api.timeseriesDefaultDays", cfg.API.TimeseriesDefaultDays)
	positive("api.timeseriesMinDays", cfg.API.TimeseriesMinDays)
	positive("api.timeseriesMaxDays", cfg.API.TimeseriesMaxDays)
	positive("api.priorityParamMaxLength", cfg.API.PriorityParamMaxLength)
	positive("api.statusParamMaxLength", cfg.API.StatusParamMaxLength)
	positive("api.abtTeamParamMaxLength", cfg.API.AbtTeamParamMaxLength)
	positive("api.filterParamMaxValues", cfg.API.FilterParamMaxValues)
	if cfg.API.IssuesDefaultLimit > cfg.API.IssuesMaxLimit {
		add("api.issuesDefaultLimit: must not exceed api.issuesMaxLimit")
	}
	if cfg.API.TimeseriesMinDays > cfg.API.TimeseriesDefaultDays {
		add("api.timeseriesMinDays: must not exceed api.timeseriesDefaultDays")
	}
	if cfg.API.TimeseriesDefaultDays > cfg.API.TimeseriesMaxDays {
		add("api.timeseriesDefaultDays: must not exceed api.timeseriesMaxDays")
	}

	for name := range cfg.SecurityHeaders {
		if !isValidHeaderName(name) {
			add("securityHeaders: %q is not a valid HTTP header name", name)
		}
	}

	positive("readiness.timeoutSeconds", cfg.Readiness.TimeoutSeconds)
	if cfg.Readiness.TimeoutSeconds >= cfg.Server.WriteTimeoutSeconds {
		add("readiness.timeoutSeconds: must be strictly less than server.writeTimeoutSeconds (got %d, server.writeTimeoutSeconds=%d)",
			cfg.Readiness.TimeoutSeconds, cfg.Server.WriteTimeoutSeconds)
	}
	if cfg.Readiness.CacheTTLSeconds < 0 {
		add("readiness.cacheTTLSeconds: must not be negative")
	}
	if cfg.Readiness.PoolSaturationThresholdPercent < 1 || cfg.Readiness.PoolSaturationThresholdPercent > 100 {
		add("readiness.poolSaturationThresholdPercent: must be between 1 and 100")
	}
	if cfg.Readiness.DrainGracePeriodSeconds < 0 {
		add("readiness.drainGracePeriodSeconds: must not be negative")
	}

	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}
