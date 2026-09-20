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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app-config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadHonorsAppConfigPathOverride verifies Load reads from
// APP_CONFIG_PATH instead of its default path when the env var is set.
func TestLoadHonorsAppConfigPathOverride(t *testing.T) {
	path := writeConfig(t, "server:\n  port: 9090\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config to load, got: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port=9090, got %d", cfg.Server.Port)
	}
}

// TestLoadUnsetAndMissingReturnsDefaults verifies that with APP_CONFIG_PATH
// unset and the default path absent, Load tolerates the miss and returns
// Default() rather than erroring.
func TestLoadUnsetAndMissingReturnsDefaults(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error when the default path is absent, got: %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("expected Default(), got: %+v", cfg)
	}
}

// TestLoadExplicitPathMissingReturnsError verifies that an explicitly set
// APP_CONFIG_PATH pointing nowhere is fatal, unlike the default-path-missing
// case.
func TestLoadExplicitPathMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APP_CONFIG_PATH", filepath.Join(dir, "does-not-exist.yaml"))

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an explicit missing config file")
	}
	if !strings.Contains(err.Error(), "app config not found at") {
		t.Errorf("expected 'app config not found at' in error, got: %v", err)
	}
}

// TestLoadMalformedYAMLReturnsError verifies unparseable YAML is fatal.
func TestLoadMalformedYAMLReturnsError(t *testing.T) {
	path := writeConfig(t, "server: [this is not a mapping\n")
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "invalid app config at") {
		t.Errorf("expected 'invalid app config at' in error, got: %v", err)
	}
}

// TestLoadUnknownFieldRejected verifies a typo'd or unrecognized key
// (e.g. an operator misspelling readiness.cacheTTLSeconds) fails loading
// instead of being silently ignored and leaving the intended field at its
// default.
func TestLoadUnknownFieldRejected(t *testing.T) {
	path := writeConfig(t, "readiness:\n  cacheTTSeconds: 5\n")
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an unknown config key")
	}
	if !strings.Contains(err.Error(), "invalid app config at") {
		t.Errorf("expected 'invalid app config at' in error, got: %v", err)
	}
}

// TestLoadExplicitZeroRejected proves the raw-pointer idiom works: an
// explicit 0 for a field that must be positive is not silently replaced by
// its default — it reaches Validate and fails.
func TestLoadExplicitZeroRejected(t *testing.T) {
	path := writeConfig(t, "server:\n  readTimeoutSeconds: 0\n")
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an explicit 0 readTimeoutSeconds")
	}
	if !strings.Contains(err.Error(), "server.readTimeoutSeconds") {
		t.Errorf("expected error naming server.readTimeoutSeconds, got: %v", err)
	}
}

// TestLoadHonorsExplicitZeroForLegalFields proves an explicit 0 is honored
// (not defaulted away) for the fields where 0 is a legitimate value.
func TestLoadHonorsExplicitZeroForLegalFields(t *testing.T) {
	path := writeConfig(t, "github:\n  maxRetries: 0\nseed:\n  interIssueDelayMs: 0\ndatabase:\n  minConns: 0\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if cfg.GitHub.MaxRetries != 0 {
		t.Errorf("expected github.maxRetries=0, got %d", cfg.GitHub.MaxRetries)
	}
	if cfg.Seed.InterIssueDelayMs != 0 {
		t.Errorf("expected seed.interIssueDelayMs=0, got %d", cfg.Seed.InterIssueDelayMs)
	}
	if cfg.Database.MinConns == nil || *cfg.Database.MinConns != 0 {
		t.Errorf("expected database.minConns=0, got %v", cfg.Database.MinConns)
	}
}

// TestLoadPartialFileLeavesRestAtDefault verifies a file setting only one key
// leaves every other key at its Default() value.
func TestLoadPartialFileLeavesRestAtDefault(t *testing.T) {
	path := writeConfig(t, "server:\n  port: 9090\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	want := Default()
	want.Server.Port = 9090
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("expected only server.port to differ from Default(), got: %+v", cfg)
	}
}

// TestLoadValidatesTheCommittedConfig guards against the repo's actual
// backend/config/app-config.yaml ever drifting from the code's own built-in
// defaults: the committed file is the default file, so loading it must
// produce exactly Default().
func TestLoadValidatesTheCommittedConfig(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", filepath.Join("..", "..", "config", "app-config.yaml"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected the committed app-config.yaml to be valid, got: %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("committed app-config.yaml has drifted from Default():\n got:  %+v\n want: %+v", cfg, Default())
	}
}

// TestLoadSecurityHeadersAbsentUsesDefaults verifies that with no
// securityHeaders key in the YAML at all, every default header survives
// unchanged.
func TestLoadSecurityHeadersAbsentUsesDefaults(t *testing.T) {
	path := writeConfig(t, "server:\n  port: 9090\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if !reflect.DeepEqual(cfg.SecurityHeaders, Default().SecurityHeaders) {
		t.Errorf("expected default security headers, got: %+v", cfg.SecurityHeaders)
	}
}

// TestLoadSecurityHeadersMergesCustomHeaderOntoDefaults verifies a custom
// header in the YAML is added alongside every built-in default, so the set
// is purely additive/overridable via config with no code change.
func TestLoadSecurityHeadersMergesCustomHeaderOntoDefaults(t *testing.T) {
	path := writeConfig(t, "securityHeaders:\n  X-Custom-Header: custom-value\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if got := cfg.SecurityHeaders["X-Custom-Header"]; got != "custom-value" {
		t.Errorf("expected custom header to be present, got %q", got)
	}
	for name, value := range Default().SecurityHeaders {
		if cfg.SecurityHeaders[name] != value {
			t.Errorf("expected default %s=%q to survive alongside the custom header, got %q", name, value, cfg.SecurityHeaders[name])
		}
	}
}

// TestLoadSecurityHeadersOverridesDefaultValue verifies the YAML can change
// one default header's value without needing to repeat every other default.
func TestLoadSecurityHeadersOverridesDefaultValue(t *testing.T) {
	path := writeConfig(t, "securityHeaders:\n  X-Frame-Options: SAMEORIGIN\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if got := cfg.SecurityHeaders["X-Frame-Options"]; got != "SAMEORIGIN" {
		t.Errorf("expected overridden X-Frame-Options=SAMEORIGIN, got %q", got)
	}
	if got := cfg.SecurityHeaders["X-Content-Type-Options"]; got != "nosniff" {
		t.Errorf("expected untouched default X-Content-Type-Options=nosniff, got %q", got)
	}
}

// TestLoadSecurityHeadersCaseInsensitiveOverride verifies a YAML header name
// differing only in case from a default still overrides that default (not
// coexists with it), since http.Header.Set canonicalizes names anyway — two
// case-variant map entries would otherwise both resolve to the same wire
// header with a nondeterministic winner (Go map iteration order).
func TestLoadSecurityHeadersCaseInsensitiveOverride(t *testing.T) {
	path := writeConfig(t, "securityHeaders:\n  x-frame-options: SAMEORIGIN\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if got, want := len(cfg.SecurityHeaders), len(Default().SecurityHeaders); got != want {
		t.Errorf("expected exactly %d headers (case-variant key must override, not add), got %d: %+v", want, got, cfg.SecurityHeaders)
	}
	if got := cfg.SecurityHeaders["X-Frame-Options"]; got != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options=SAMEORIGIN (overridden via lowercase YAML key), got %q", got)
	}
}

// TestLoadSecurityHeadersCaseVariantCollisionRejected verifies that two RAW
// keys in the same file which canonicalize to the same header name (as
// opposed to one raw key overriding a default, covered above) are rejected
// rather than merged — Go map iteration order over raw would otherwise pick
// one of them nondeterministically.
func TestLoadSecurityHeadersCaseVariantCollisionRejected(t *testing.T) {
	path := writeConfig(t, "securityHeaders:\n  X-Frame-Options: DENY\n  x-frame-options: SAMEORIGIN\n")
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for case-variant duplicate securityHeaders keys")
	}
	if !strings.Contains(err.Error(), "invalid app config at") {
		t.Errorf("expected 'invalid app config at' in error, got: %v", err)
	}
}

// TestLoadReadinessAbsentAppliesDefaults verifies that with no readiness:
// key in the YAML at all, every readiness default applies unchanged.
func TestLoadReadinessAbsentAppliesDefaults(t *testing.T) {
	path := writeConfig(t, "server:\n  port: 9090\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	if cfg.Readiness != Default().Readiness {
		t.Errorf("expected default readiness config, got: %+v", cfg.Readiness)
	}
}

// TestLoadReadinessExplicitZeroTimeoutRejected proves the raw-pointer idiom
// applies to readiness too: an explicit 0 for timeoutSeconds must reach
// Validate and fail, not be silently defaulted away.
func TestLoadReadinessExplicitZeroTimeoutRejected(t *testing.T) {
	path := writeConfig(t, "readiness:\n  timeoutSeconds: 0\n")
	t.Setenv("APP_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an explicit 0 readiness.timeoutSeconds")
	}
	if !strings.Contains(err.Error(), "readiness.timeoutSeconds") {
		t.Errorf("expected error naming readiness.timeoutSeconds, got: %v", err)
	}
}

// TestLoadReadinessPartialOverrideLeavesRestAtDefault verifies setting one
// readiness key leaves its siblings at their Default() values.
func TestLoadReadinessPartialOverrideLeavesRestAtDefault(t *testing.T) {
	path := writeConfig(t, "readiness:\n  cacheTTLSeconds: 5\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	want := Default().Readiness
	want.CacheTTLSeconds = 5
	if cfg.Readiness != want {
		t.Errorf("expected only cacheTTLSeconds to differ from Default(), got: %+v", cfg.Readiness)
	}
}

// TestLoadReadinessDrainGracePeriodOverrideLeavesRestAtDefault verifies
// setting readiness.drainGracePeriodSeconds leaves its siblings at their
// Default() values. Both Default() and the committed app-config.yaml have
// DrainGracePeriodSeconds at 0, so no other test exercises this field's YAML
// tag or its resolve() wiring — a broken tag or a missing nil-check here
// would silently produce the same zero value and go undetected.
func TestLoadReadinessDrainGracePeriodOverrideLeavesRestAtDefault(t *testing.T) {
	path := writeConfig(t, "readiness:\n  drainGracePeriodSeconds: 5\n")
	t.Setenv("APP_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
	want := Default().Readiness
	want.DrainGracePeriodSeconds = 5
	if cfg.Readiness != want {
		t.Errorf("expected only drainGracePeriodSeconds to differ from Default(), got: %+v", cfg.Readiness)
	}
}
