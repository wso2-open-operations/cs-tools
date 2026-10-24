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

package config

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func minimal() map[string]string {
	return map[string]string{
		"DB_HOST":            "db.internal",
		"DB_USER":            "til",
		"DB_PASSWORD":        "secret",
		"DB_NAME":            "til",
		"AUTH_JWKS_ENDPOINT": "https://idp.example.com/oauth2/jwks",
		"AUTH_ISSUER":        "https://idp.example.com/oauth2/token",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, warnings, err := Load(env(minimal()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9090" || cfg.DB.Port != "3306" || cfg.DB.TLS != store.TLSOff || cfg.ReadTimeout != 15*time.Second || cfg.WriteTimeout != 45*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.Novera != nil || len(cfg.CORSOrigins) != 0 {
		t.Fatalf("optional features should be off: %+v", cfg)
	}
	if cfg.WhatMaxLength != 500 {
		t.Fatalf("WhatMaxLength = %d, want the default 500", cfg.WhatMaxLength)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestLoadReadsEverything(t *testing.T) {
	v := minimal()
	for k, val := range map[string]string{
		"PORT":                      "9000",
		"DB_TLS":                    "true",
		"REST_READ_TIMEOUT":         "5s",
		"REST_WRITE_TIMEOUT":        "1m",
		"CORS_ALLOWED_ORIGINS":      "https://one.example, https://two.example ,",
		"AUTH_AUDIENCE":             "webapp",
		"AUTH_MODERATOR_GROUPS":     "til-moderators,admins",
		"ONE_WSO2_WEBAPP_CLIENT_ID": "webapp",
		"ONE_WSO2_BASE_URL":         "https://one.example/",
		"ENTRY_TEXT_MAX_LENGTH":     "800",
		"NOVERA_NOTIFY_URL":         "https://gateway.example/novera/internal/notifications",
		"NOVERA_TOKEN_URL":          "https://gateway.example/token",
		"NOVERA_CLIENT_ID":          "til",
		"NOVERA_CLIENT_SECRET":      "s",
	} {
		v[k] = val
	}
	cfg, warnings, err := Load(env(v))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %q", warnings)
	}
	if cfg.Port != "9000" || cfg.DB.TLS != store.TLSVerify || cfg.ReadTimeout != 5*time.Second || cfg.WriteTimeout != time.Minute {
		t.Fatalf("server = %+v", cfg)
	}
	if !slices.Equal(cfg.CORSOrigins, []string{"https://one.example", "https://two.example"}) ||
		!slices.Equal(cfg.Auth.ModeratorGroups, []string{"til-moderators", "admins"}) ||
		cfg.WebappBaseURL != "https://one.example" || cfg.WhatMaxLength != 800 {
		t.Fatalf("lists = %+v", cfg)
	}
	if cfg.Novera == nil || cfg.Novera.NotifyURL != v["NOVERA_NOTIFY_URL"] || cfg.Novera.IconURL != "https://one.example/til-logo.png" {
		t.Fatalf("novera = %+v", cfg.Novera)
	}
}

func TestLoadDBTLS(t *testing.T) {
	for value, want := range map[string]store.TLSMode{"false": store.TLSOff, "true": store.TLSVerify, "skip-verify": store.TLSSkipVerify} {
		v := minimal()
		v["DB_TLS"] = value
		cfg, warnings, err := Load(env(v))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DB.TLS != want {
			t.Errorf("DB_TLS=%s: TLS = %v, want %v", value, cfg.DB.TLS, want)
		}
		warned := slices.ContainsFunc(warnings, func(w string) bool { return strings.HasPrefix(w, "DB_TLS is skip-verify") })
		if warned != (want == store.TLSSkipVerify) {
			t.Errorf("DB_TLS=%s: warnings = %q", value, warnings)
		}
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, _, err := Load(env(map[string]string{
		"PORT":                  "http",
		"REST_READ_TIMEOUT":     "soon",
		"DB_TLS":                "1",
		"AUTH_JWKS_ENDPOINT":    "idp.example.com/jwks",
		"ONE_WSO2_BASE_URL":     "one.example",
		"ENTRY_TEXT_MAX_LENGTH": "5001",
		"NOVERA_NOTIFY_URL":     "https://gateway.example/notify",
	}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{
		"PORT must be a port number",
		"REST_READ_TIMEOUT must be a positive duration",
		"DB_HOST is required", "DB_USER is required", "DB_PASSWORD is required", "DB_NAME is required",
		"DB_TLS must be true, false or skip-verify",
		"AUTH_JWKS_ENDPOINT must be an absolute http(s) URL",
		"AUTH_ISSUER is required",
		"ONE_WSO2_BASE_URL must be an absolute http(s) URL",
		"ENTRY_TEXT_MAX_LENGTH must be a whole number from 1 to 5000",
		"NOVERA_TOKEN_URL is required", "NOVERA_CLIENT_ID is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}
