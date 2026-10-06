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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

const (
	envReadTimeout   = "REST_READ_TIMEOUT"
	envWriteTimeout  = "REST_WRITE_TIMEOUT"
	envEntityTimeout = "ENTITY_SERVICE_TIMEOUT"

	defaultReadTimeout  = 60 * time.Second
	defaultWriteTimeout = 60 * time.Second
)

// timeouts holds the configurable request timeouts.
type timeouts struct {
	read   time.Duration // REST server ReadTimeout
	write  time.Duration // REST server WriteTimeout
	entity time.Duration // entity-service HTTP client Timeout
}

// loadTimeouts reads the timeout variables (Go duration strings such as "60s"
// or "1m30s"; unset or empty means the default) and validates them: every
// value must parse and be positive. No ordering between them is enforced;
// operators are advised to keep the entity client timeout shorter than the
// server write timeout so the handler can still return a clean error when the
// upstream call times out.
func loadTimeouts(getenv func(string) string) (timeouts, error) {
	var t timeouts
	var err error
	if t.read, err = durationEnv(getenv, envReadTimeout, defaultReadTimeout); err != nil {
		return timeouts{}, err
	}
	if t.write, err = durationEnv(getenv, envWriteTimeout, defaultWriteTimeout); err != nil {
		return timeouts{}, err
	}
	if t.entity, err = durationEnv(getenv, envEntityTimeout, entity.DefaultTimeout); err != nil {
		return timeouts{}, err
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
		return 0, fmt.Errorf("%s=%q is not a valid duration (e.g. \"60s\"): %w", key, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero", key, raw)
	}
	return d, nil
}
