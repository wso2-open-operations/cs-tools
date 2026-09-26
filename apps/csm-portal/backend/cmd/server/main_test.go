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
	"os"
	"os/exec"
	"testing"
)

// TestValidateHTTPSURL covers the pure validation logic behind mustHTTPSURL,
// which gates SFTPGO_BASE_URL and SFTPGO_PUBLIC_BASE_URL: both are used to
// build requests carrying the caller's email and raw gateway JWT (see
// internal/sftpgo.Client.MintToken) or a public download link handed to end
// users (see internal/sftpgo.Client.PublicShareURL), so neither may be
// non-HTTPS or carry embedded userinfo.
func TestValidateHTTPSURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "https ok", value: "https://sftpgo.internal.example.com", wantErr: false},
		{name: "https with trailing slash ok", value: "https://sftpgo.internal.example.com/", wantErr: false},
		{name: "https with path rejected", value: "https://sftpgo.internal.example.com/api", wantErr: true},
		{name: "https with query rejected", value: "https://sftpgo.internal.example.com?x=1", wantErr: true},
		{name: "https with fragment rejected", value: "https://sftpgo.internal.example.com#frag", wantErr: true},
		{name: "http rejected", value: "http://sftpgo.internal.example.com", wantErr: true},
		{name: "scheme-less rejected", value: "sftpgo.internal.example.com", wantErr: true},
		{name: "unparseable rejected", value: "https://%zz", wantErr: true},
		{name: "embedded userinfo rejected", value: "https://user:pass@sftpgo.internal.example.com", wantErr: true},
		{name: "embedded username only rejected", value: "https://user@sftpgo.internal.example.com", wantErr: true},
		{name: "no host rejected", value: "https:///api", wantErr: true},
		{name: "opaque no host rejected", value: "https:opaque", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPSURL(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHTTPSURL(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestValidateHTTPSBaseURL covers the check behind ENGINEERING_ENTITY_BASE_URL:
// unlike validateHTTPSURL it allows a path, since a gateway-hosted service's
// base URL normally has one, but it still refuses anything that would send the
// OAuth2 token or request in cleartext or carry credentials in the URL.
func TestValidateHTTPSBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "https host ok", value: "https://engineering.example.com", wantErr: false},
		{name: "https with trailing slash ok", value: "https://engineering.example.com/", wantErr: false},
		{name: "https with gateway path ok", value: "https://engineering.example.com/org/service/v1.0", wantErr: false},
		{name: "https with port ok", value: "https://engineering.example.com:8443/v1", wantErr: false},
		{name: "http rejected", value: "http://engineering.example.com", wantErr: true},
		{name: "http on localhost rejected", value: "http://localhost:9090", wantErr: true},
		{name: "no scheme rejected", value: "engineering.example.com/v1", wantErr: true},
		{name: "empty host rejected", value: "https:///v1", wantErr: true},
		{name: "userinfo rejected", value: "https://user:pass@engineering.example.com/v1", wantErr: true},
		{name: "query rejected", value: "https://engineering.example.com/v1?x=1", wantErr: true},
		{name: "fragment rejected", value: "https://engineering.example.com/v1#frag", wantErr: true},
		{name: "unparseable rejected", value: "https://exa mple.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPSBaseURL(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHTTPSBaseURL(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestLoadSftpgoConfigRejectsBadURLs exercises the real exit path exercised
// at server startup: loadSftpgoConfig calls os.Exit(1) via mustHTTPSURL when
// SFTPGO_BASE_URL or SFTPGO_PUBLIC_BASE_URL is invalid while the feature flag
// is on, so this re-execs the test binary as a subprocess (the standard Go
// pattern for testing os.Exit call sites) and asserts on its exit status.
func TestLoadSftpgoConfigRejectsBadURLs(t *testing.T) {
	if os.Getenv("BE_LOAD_SFTPGO_CONFIG_SUBPROCESS") == "1" {
		loadSftpgoConfig()
		return
	}

	tests := []struct {
		name            string
		baseURL         string
		publicBaseURL   string
		wantExitNonZero bool
	}{
		{name: "valid https base URL, no public URL", baseURL: "https://sftpgo.internal.example.com", wantExitNonZero: false},
		{name: "valid https for both", baseURL: "https://sftpgo.internal.example.com", publicBaseURL: "https://share.example.com", wantExitNonZero: false},
		{name: "http base URL rejected", baseURL: "http://sftpgo.internal.example.com", wantExitNonZero: true},
		{name: "http public base URL rejected", baseURL: "https://sftpgo.internal.example.com", publicBaseURL: "http://share.example.com", wantExitNonZero: true},
		{name: "userinfo in base URL rejected", baseURL: "https://user:pass@sftpgo.internal.example.com", wantExitNonZero: true},
		{name: "userinfo in public base URL rejected", baseURL: "https://sftpgo.internal.example.com", publicBaseURL: "https://user:pass@share.example.com", wantExitNonZero: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLoadSftpgoConfigRejectsBadURLs$")
			cmd.Env = append(os.Environ(),
				"BE_LOAD_SFTPGO_CONFIG_SUBPROCESS=1",
				"SFTPGO_ATTACHMENT_STORAGE_ENABLED=true",
				"SFTPGO_BASE_URL="+tt.baseURL,
				"SFTPGO_PUBLIC_BASE_URL="+tt.publicBaseURL,
			)
			out, err := cmd.CombinedOutput()

			if tt.wantExitNonZero {
				if err == nil {
					t.Fatalf("expected loadSftpgoConfig to exit non-zero, but it exited cleanly; output:\n%s", out)
				}
				var exitErr *exec.ExitError
				if !isExitError(err, &exitErr) {
					t.Fatalf("expected an *exec.ExitError, got %T: %v; output:\n%s", err, err, out)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected loadSftpgoConfig to exit cleanly, got error %v; output:\n%s", err, out)
			}
		})
	}
}

// isExitError reports whether err is an *exec.ExitError and, if so, assigns
// it to *target.
func isExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// TestValidateCustomerEntityDataSource covers the pure validation logic
// behind loadCustomerEntityDataSource, which resolves which data source the
// paired entity-service instance is configured with.
func TestValidateCustomerEntityDataSource(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "servicenow ok", value: "servicenow", wantErr: false},
		{name: "postgres ok", value: "postgres", wantErr: false},
		{name: "empty rejected", value: "", wantErr: true},
		{name: "unrecognized value rejected", value: "mysql", wantErr: true},
		{name: "wrong case rejected (caller must normalize first)", value: "Postgres", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomerEntityDataSource(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCustomerEntityDataSource(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestCheckAnnouncementDataSourceCompatibility covers the pure check behind
// validateAnnouncementDataSourceCompatibility: entity-service's Postgres-
// backed project search rejects a non-empty excludeProjectKeys outright, so
// a mandatory denylist configured against a Postgres-backed entity-service
// would make every announcement audience search fail — this must be caught,
// but only when the denylist is actually non-empty; an unconfigured (empty)
// denylist against Postgres is fine, since nothing is ever sent to exclude.
func TestCheckAnnouncementDataSourceCompatibility(t *testing.T) {
	tests := []struct {
		name                string
		dataSource          string
		excludedProjectKeys []string
		wantErr             bool
	}{
		{name: "servicenow with keys ok", dataSource: "servicenow", excludedProjectKeys: []string{"Apexia"}, wantErr: false},
		{name: "servicenow with no keys ok", dataSource: "servicenow", excludedProjectKeys: nil, wantErr: false},
		{name: "postgres with no keys ok", dataSource: "postgres", excludedProjectKeys: nil, wantErr: false},
		{name: "postgres with empty (non-nil) keys ok", dataSource: "postgres", excludedProjectKeys: []string{}, wantErr: false},
		{name: "postgres with keys rejected", dataSource: "postgres", excludedProjectKeys: []string{"Apexia"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAnnouncementDataSourceCompatibility(tt.dataSource, tt.excludedProjectKeys)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkAnnouncementDataSourceCompatibility(%q, %v) error = %v, wantErr %v",
					tt.dataSource, tt.excludedProjectKeys, err, tt.wantErr)
			}
		})
	}
}

// TestStartupRejectsPostgresWithExcludedProjectKeys exercises the real exit
// path exercised at server startup: main() calls
// validateAnnouncementDataSourceCompatibility(loadCustomerEntityDataSource(),
// loadAnnouncementExcludedProjectKeys()), which calls os.Exit(1) if a
// mandatory excluded-project-key denylist is configured against a
// Postgres-backed entity-service (see the two functions' own doc comments
// for why). This re-execs the test binary as a subprocess (the standard Go
// pattern for testing os.Exit call sites) and asserts on its exit status.
func TestStartupRejectsPostgresWithExcludedProjectKeys(t *testing.T) {
	if os.Getenv("BE_STARTUP_DATA_SOURCE_GUARD_SUBPROCESS") == "1" {
		validateAnnouncementDataSourceCompatibility(loadCustomerEntityDataSource(), loadAnnouncementExcludedProjectKeys())
		return
	}

	tests := []struct {
		name                string
		dataSource          string
		excludedProjectKeys string
		wantExitNonZero     bool
	}{
		{name: "servicenow with keys starts fine", dataSource: "servicenow", excludedProjectKeys: "Apexia,Veridian", wantExitNonZero: false},
		{name: "postgres with no keys starts fine", dataSource: "postgres", excludedProjectKeys: "", wantExitNonZero: false},
		{name: "default data source (unset) with keys starts fine", dataSource: "", excludedProjectKeys: "Apexia", wantExitNonZero: false},
		{name: "postgres with keys rejected", dataSource: "postgres", excludedProjectKeys: "Apexia,Veridian", wantExitNonZero: true},
		{name: "unrecognized data source rejected", dataSource: "mysql", excludedProjectKeys: "", wantExitNonZero: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestStartupRejectsPostgresWithExcludedProjectKeys$")
			cmd.Env = append(os.Environ(),
				"BE_STARTUP_DATA_SOURCE_GUARD_SUBPROCESS=1",
				"CUSTOMER_ENTITY_DATA_SOURCE="+tt.dataSource,
				"CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS="+tt.excludedProjectKeys,
			)
			out, err := cmd.CombinedOutput()

			if tt.wantExitNonZero {
				if err == nil {
					t.Fatalf("expected startup to exit non-zero, but it exited cleanly; output:\n%s", out)
				}
				var exitErr *exec.ExitError
				if !isExitError(err, &exitErr) {
					t.Fatalf("expected an *exec.ExitError, got %T: %v; output:\n%s", err, err, out)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected startup to exit cleanly, got error %v; output:\n%s", err, out)
			}
		})
	}
}
