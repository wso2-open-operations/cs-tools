package config

import (
	"os"
	"testing"
)

// Every config.json key must be reachable from an environment variable,
// because Choreo deploys from environment alone — a value that exists only in
// the JSON file is a value that cannot be changed where it matters.
//
// This is a table of the mapping rather than a test of one field: adding a
// config key without an override is the mistake it exists to catch, and that
// mistake already happened once with server.requestTimeoutSeconds.
func TestEveryConfigKeyHasAnEnvOverride(t *testing.T) {
	tests := []struct {
		env   string
		value string
		read  func(*Config) string
	}{
		{"PLG_ENTITY_BASE_URL", "http://entity.test:9000", func(c *Config) string { return c.Entity.BaseURL }},
		{"PLG_ENTITY_OAUTH_TOKEN_URL", "https://sts.test/token", func(c *Config) string { return c.Entity.OAuth.TokenURL }},
		{"PLG_ENTITY_OAUTH_CLIENT_ID", "cid", func(c *Config) string { return c.Entity.OAuth.ClientID }},
		{"PLG_ENTITY_OAUTH_CLIENT_SECRET", "csec", func(c *Config) string { return c.Entity.OAuth.ClientSecret }},
		{"PLG_ENTITY_SCOPES", "read", func(c *Config) string { return c.Entity.OAuth.Scope }},
		{"PLG_ENTITY_TIMEOUT_SECONDS", "31", func(c *Config) string { return itoa(c.Entity.TimeoutSeconds) }},
		{"PLG_SERVER_PORT", "9111", func(c *Config) string { return itoa(c.Server.Port) }},
		{"PLG_SERVER_REQUEST_TIMEOUT_SECONDS", "43", func(c *Config) string { return itoa(c.Server.RequestTimeoutSeconds) }},
		{"PLG_ALLOWED_ORIGINS", "https://a.test", func(c *Config) string { return c.Server.AllowedOrigins[0] }},
		{"PLG_LOG_LEVEL", "WARN", func(c *Config) string { return c.Logging.Level }},
		{"PLG_LOG_FORMAT", "json", func(c *Config) string { return c.Logging.Format }},
	}

	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv(tc.env, tc.value)
			c := &Config{}
			applyEnvOverrides(c)
			if got := tc.read(c); got != tc.value {
				t.Errorf("%s=%q did not reach the config: got %q", tc.env, tc.value, got)
			}
		})
	}
}

// Lists take their own path through applyEnvOverrides, so the one that remains
// is asserted separately rather than squeezed into the string table above.
//
// This used to cover the queue's boolean and list variables too. They went when
// registrations stopped being polled by this backend — see package plg.
func TestListOverrides(t *testing.T) {
	t.Run("PLG_ALLOWED_ORIGINS", func(t *testing.T) {
		t.Setenv("PLG_ALLOWED_ORIGINS", "https://a.test, https://b.test ,https://c.test")
		c := &Config{}
		applyEnvOverrides(c)
		if len(c.Server.AllowedOrigins) != 3 || c.Server.AllowedOrigins[1] != "https://b.test" {
			t.Errorf("list not split and trimmed: %#v", c.Server.AllowedOrigins)
		}
	})
}

// An unset variable must leave the file's value alone — otherwise every
// unspecified variable would silently zero a configured field.
func TestUnsetEnvLeavesTheFileValue(t *testing.T) {
	// t.Setenv first, then Unsetenv: t.Setenv is what registers the cleanup that
	// puts the caller's own value back. Unsetting directly would delete it for
	// the rest of the binary, so a developer who runs `go test` with this
	// variable exported would have every later test see an environment they did
	// not configure — and only when this test happened to run first.
	t.Setenv("PLG_SERVER_REQUEST_TIMEOUT_SECONDS", "")
	os.Unsetenv("PLG_SERVER_REQUEST_TIMEOUT_SECONDS")
	c := &Config{}
	c.Server.RequestTimeoutSeconds = 15
	applyEnvOverrides(c)
	if c.Server.RequestTimeoutSeconds != 15 {
		t.Errorf("an unset variable overwrote the configured value: %d", c.Server.RequestTimeoutSeconds)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
