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
	"net/url"
	"testing"
)

// TestDatabaseDSN_PasswordRoundTrip covers the entire reason this exists:
// a password containing reserved URL characters ("?", "@", "/", a space)
// must come back out of url.Parse exactly as it went in, because
// url.UserPassword percent-encodes it when the DSN is built rather than
// requiring the operator to do so by hand.
func TestDatabaseDSN_PasswordRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "question mark", password: "p@ss?word"},
		{name: "at sign", password: "we?rd@pass/word"},
		{name: "forward slash", password: "pass/word"},
		{name: "space", password: "pass word"},
		{name: "all of the above", password: "p@ss w/ord?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := databaseDSN("db-example.internal", "5432", "alert_user", tt.password, "sre_alerts", "require")

			parsed, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("url.Parse(%q) failed: %v", dsn, err)
			}
			gotPassword, ok := parsed.User.Password()
			if !ok {
				t.Fatalf("parsed DSN %q has no password", dsn)
			}
			if gotPassword != tt.password {
				t.Errorf("password round-trip mismatch: got %q, want %q (dsn=%q)", gotPassword, tt.password, dsn)
			}
		})
	}
}

// TestDatabaseDSN_HostPortAndSSLMode covers the non-password parts of the
// DSN: host, port, and sslmode all land in the expected place.
func TestDatabaseDSN_HostPortAndSSLMode(t *testing.T) {
	dsn := databaseDSN("db-example.internal", "5433", "alert_user", "alert_pass", "sre_alerts", "require")

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", dsn, err)
	}
	if got, want := parsed.Scheme, "postgres"; got != want {
		t.Errorf("scheme = %q, want %q", got, want)
	}
	if got, want := parsed.Hostname(), "db-example.internal"; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
	if got, want := parsed.Port(), "5433"; got != want {
		t.Errorf("port = %q, want %q", got, want)
	}
	if got, want := parsed.User.Username(), "alert_user"; got != want {
		t.Errorf("user = %q, want %q", got, want)
	}
	if got, want := parsed.Path, "/sre_alerts"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got, want := parsed.Query().Get("sslmode"), "require"; got != want {
		t.Errorf("sslmode = %q, want %q", got, want)
	}
}

// TestDatabaseDSN_IPv6Host confirms a bare IPv6 literal in DB_HOST comes
// back out of url.Parse as the exact address, with the configured port
// still recognized separately — the regression net.JoinHostPort exists to
// prevent: "host:port" string concatenation on an IPv6 address (which
// already contains colons) produces something pgx/url.Parse reads as one
// opaque host, silently dropping the port.
func TestDatabaseDSN_IPv6Host(t *testing.T) {
	dsn := databaseDSN("2001:db8::1", "5433", "alert_user", "alert_pass", "sre_alerts", "require")

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", dsn, err)
	}
	if got, want := parsed.Hostname(), "2001:db8::1"; got != want {
		t.Errorf("host = %q, want %q (dsn=%q)", got, want, dsn)
	}
	if got, want := parsed.Port(), "5433"; got != want {
		t.Errorf("port = %q, want %q (dsn=%q)", got, want, dsn)
	}
}

// TestDatabaseDSN_EmptySSLMode confirms an empty sslmode is written through
// as an empty query value rather than being omitted or defaulted — pgx
// applies its own default behavior for an empty sslmode, and this function
// must not second-guess that.
func TestDatabaseDSN_EmptySSLMode(t *testing.T) {
	dsn := databaseDSN("localhost", "5432", "alert_user", "alert_pass", "sre_alerts", "")

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", dsn, err)
	}
	q := parsed.Query()
	if got, ok := q["sslmode"]; !ok || len(got) != 1 || got[0] != "" {
		t.Errorf("sslmode query param = %v, want a single empty value present (dsn=%q)", got, dsn)
	}
}

// TestBuildDatabaseDSN_HostPortDefaults covers envOrDefault's DB_HOST/DB_PORT
// fallback to "localhost"/"5432" when unset, exercised through
// buildDatabaseDSN end to end (with DB_USER/DB_PASSWORD/DB_NAME set so
// mustEnv doesn't exit the test process).
func TestBuildDatabaseDSN_HostPortDefaults(t *testing.T) {
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE"} {
		t.Setenv(key, "")
	}
	t.Setenv("DB_USER", "alert_user")
	t.Setenv("DB_PASSWORD", "alert_pass")
	t.Setenv("DB_NAME", "sre_alerts")

	dsn := buildDatabaseDSN()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", dsn, err)
	}
	if got, want := parsed.Hostname(), "localhost"; got != want {
		t.Errorf("default host = %q, want %q", got, want)
	}
	if got, want := parsed.Port(), "5432"; got != want {
		t.Errorf("default port = %q, want %q", got, want)
	}
}
