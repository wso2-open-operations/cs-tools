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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"fmt"
	"time"
)

// Request-timeout defaults. This service only serves small JSON reads, so the
// defaults are tight. Ordering rules:
//   - REST_WRITE_TIMEOUT must be >= handler.RelatedBudget (the related-articles
//     lookup chains several upstream calls under that budget); loadConfig
//     rejects anything lower.
//   - ENTITY_SERVICE_TIMEOUT should stay shorter than REST_WRITE_TIMEOUT so a
//     single slow upstream call still leaves room for a clean error. That one
//     is advice, not enforced.
const (
	defaultRESTReadTimeout      = 10 * time.Second
	defaultRESTWriteTimeout     = 30 * time.Second
	defaultEntityServiceTimeout = 10 * time.Second
)

// timeouts holds the operator-configurable request timeouts.
type timeouts struct {
	RESTRead      time.Duration
	RESTWrite     time.Duration
	EntityService time.Duration
}

// loadTimeouts resolves the timeouts from the environment via getenv. Each
// value is a Go duration string (e.g. "45s", "2m"); unset or empty selects the
// default. Every value must be > 0; ordering against the related-articles budget is checked by loadConfig.
func loadTimeouts(getenv func(string) string) (timeouts, error) {
	var t timeouts
	for _, f := range []struct {
		key string
		def time.Duration
		dst *time.Duration
	}{
		{"REST_READ_TIMEOUT", defaultRESTReadTimeout, &t.RESTRead},
		{"REST_WRITE_TIMEOUT", defaultRESTWriteTimeout, &t.RESTWrite},
		{"ENTITY_SERVICE_TIMEOUT", defaultEntityServiceTimeout, &t.EntityService},
	} {
		d, err := durationEnv(getenv, f.key, f.def)
		if err != nil {
			return timeouts{}, err
		}
		*f.dst = d
	}
	return t, nil
}

func durationEnv(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	raw := getenv(key)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: duration must be > 0, got %q", key, raw)
	}
	return d, nil
}
