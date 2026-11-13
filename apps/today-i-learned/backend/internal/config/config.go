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

// Package config reads the service's settings from environment variables
// (see .env.example) and checks them before anything starts.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/entry"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/novera"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

// Config is every setting the service runs with.
type Config struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	CORSOrigins  []string

	DB store.Config

	JWKSEndpoint string
	Auth         middleware.AuthConfig

	WebappClientID string
	WebappBaseURL  string

	// WhatMaxLength is ENTRY_TEXT_MAX_LENGTH: the most characters of plain
	// text a learning may have.
	WhatMaxLength int

	// Novera is nil when new entries aren't announced.
	Novera *novera.Config
}

// Load reads the settings through getenv. It returns every problem at once,
// and warnings for settings that are valid but probably not intended.
func Load(getenv func(string) string) (Config, []string, error) {
	r := reader{getenv: getenv}
	cfg := Config{
		Port:         r.port("PORT", "9090"),
		ReadTimeout:  r.duration("REST_READ_TIMEOUT", 15*time.Second),
		WriteTimeout: r.duration("REST_WRITE_TIMEOUT", 45*time.Second),
		CORSOrigins:  r.list("CORS_ALLOWED_ORIGINS"),
		DB: store.Config{
			Host:     r.required("DB_HOST"),
			Port:     r.port("DB_PORT", "3306"),
			User:     r.required("DB_USER"),
			Password: r.required("DB_PASSWORD"),
			Name:     r.required("DB_NAME"),
			TLS:      r.dbTLS("DB_TLS"),
		},
		JWKSEndpoint: r.url("AUTH_JWKS_ENDPOINT", true),
		Auth: middleware.AuthConfig{
			Issuer:          r.required("AUTH_ISSUER"),
			Audiences:       r.list("AUTH_AUDIENCE"),
			ModeratorGroups: r.list("AUTH_MODERATOR_GROUPS"),
			ClockSkew:       5 * time.Second,
		},
		WebappClientID: r.optional("ONE_WSO2_WEBAPP_CLIENT_ID"),
		WebappBaseURL:  strings.TrimRight(r.url("ONE_WSO2_BASE_URL", false), "/"),
		WhatMaxLength:  r.intInRange("ENTRY_TEXT_MAX_LENGTH", entry.DefaultWhatMaxLength, 1, entry.MaxWhatMaxLength),
	}

	if r.anySet("NOVERA_NOTIFY_URL", "NOVERA_TOKEN_URL", "NOVERA_CLIENT_ID", "NOVERA_CLIENT_SECRET") {
		cfg.Novera = &novera.Config{
			NotifyURL:    r.url("NOVERA_NOTIFY_URL", true),
			TokenURL:     r.url("NOVERA_TOKEN_URL", true),
			ClientID:     r.required("NOVERA_CLIENT_ID"),
			ClientSecret: r.required("NOVERA_CLIENT_SECRET"),
			IconURL:      novera.IconURL(cfg.WebappBaseURL),
		}
	}

	var warnings []string
	if cfg.DB.TLS == store.TLSSkipVerify {
		warnings = append(warnings, "DB_TLS is skip-verify: the database connection is encrypted, but its certificate is not checked")
	}
	if cfg.WebappClientID == "" {
		warnings = append(warnings, "ONE_WSO2_WEBAPP_CLIENT_ID is not set: no one can share or edit entries")
	}
	if len(cfg.Auth.ModeratorGroups) == 0 {
		warnings = append(warnings, "AUTH_MODERATOR_GROUPS is not set: only authors can edit or delete their entries")
	}
	if cfg.Novera == nil {
		warnings = append(warnings, "NOVERA_* is not set: new entries are not announced in Google Chat")
	}

	if len(r.problems) > 0 {
		return Config{}, warnings, errors.New("invalid configuration:\n  " + strings.Join(r.problems, "\n  "))
	}
	return cfg, warnings, nil
}

type reader struct {
	getenv   func(string) string
	problems []string
}

func (r *reader) optional(key string) string {
	return strings.TrimSpace(r.getenv(key))
}

func (r *reader) required(key string) string {
	v := r.optional(key)
	if v == "" {
		r.problems = append(r.problems, key+" is required")
	}
	return v
}

func (r *reader) anySet(keys ...string) bool {
	for _, k := range keys {
		if r.optional(k) != "" {
			return true
		}
	}
	return false
}

// list reads a comma-separated value, dropping empty items.
func (r *reader) list(key string) []string {
	var out []string
	for _, item := range strings.Split(r.optional(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// url reads an absolute http(s) URL.
func (r *reader) url(key string, required bool) string {
	v := r.optional(key)
	if v == "" {
		if required {
			r.problems = append(r.problems, key+" is required")
		}
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		r.problems = append(r.problems, key+" must be an absolute http(s) URL")
	}
	return v
}

// dbTLS reads false (or unset), true or skip-verify, and nothing else.
func (r *reader) dbTLS(key string) store.TLSMode {
	switch r.optional(key) {
	case "", "false":
		return store.TLSOff
	case "true":
		return store.TLSVerify
	case "skip-verify":
		return store.TLSSkipVerify
	}
	r.problems = append(r.problems, key+" must be true, false or skip-verify")
	return store.TLSOff
}

func (r *reader) port(key, fallback string) string {
	v := r.optional(key)
	if v == "" {
		return fallback
	}
	if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 65535 {
		r.problems = append(r.problems, key+" must be a port number")
	}
	return v
}

func (r *reader) intInRange(key string, fallback, lo, hi int) int {
	v := r.optional(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		r.problems = append(r.problems, fmt.Sprintf("%s must be a whole number from %d to %d", key, lo, hi))
		return fallback
	}
	return n
}

func (r *reader) duration(key string, fallback time.Duration) time.Duration {
	v := r.optional(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		r.problems = append(r.problems, fmt.Sprintf("%s must be a positive duration such as 30s", key))
		return fallback
	}
	return d
}
