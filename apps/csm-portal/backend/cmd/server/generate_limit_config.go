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

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
)

// generateRateLimits are the per-user limits on POST /dashboards/generate
// (handler.GenerateRateLimiter).
type generateRateLimits struct {
	PerMinute int
	PerDay    int
}

// loadGenerateRateLimits reads DASHBOARD_GENERATE_LIMIT_PER_MINUTE and
// DASHBOARD_GENERATE_LIMIT_PER_DAY: whole numbers > 0, defaulting to
// handler.DefaultGenerateLimitPerMinute and DefaultGenerateLimitPerDay. There
// is no "off": each generate call is a paid model call. Pure, so tests pass
// their own getenv.
func loadGenerateRateLimits(getenv func(string) string) (generateRateLimits, error) {
	perMinute, err := positiveIntEnv(getenv, "DASHBOARD_GENERATE_LIMIT_PER_MINUTE", handler.DefaultGenerateLimitPerMinute)
	if err != nil {
		return generateRateLimits{}, err
	}
	perDay, err := positiveIntEnv(getenv, "DASHBOARD_GENERATE_LIMIT_PER_DAY", handler.DefaultGenerateLimitPerDay)
	if err != nil {
		return generateRateLimits{}, err
	}
	if perDay < perMinute {
		return generateRateLimits{}, fmt.Errorf("DASHBOARD_GENERATE_LIMIT_PER_DAY (%d) must not be below DASHBOARD_GENERATE_LIMIT_PER_MINUTE (%d)", perDay, perMinute)
	}
	return generateRateLimits{PerMinute: perMinute, PerDay: perDay}, nil
}

// positiveIntEnv reads key as a whole number > 0 (whitespace trimmed), or def
// when it is unset or blank.
func positiveIntEnv(getenv func(string) string, key string, def int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: must be a whole number > 0, got %q", key, raw)
	}
	return n, nil
}
