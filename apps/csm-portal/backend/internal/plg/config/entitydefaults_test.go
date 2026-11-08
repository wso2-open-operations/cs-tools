package config

import (
	"os"
	"strings"
	"testing"
)

// PLG reaches the same entity-service as the rest of this backend, as the same
// OAuth2 application. These tests pin that it inherits those settings rather
// than keeping a second copy, because the copy is what goes stale: a deployment
// that rotates the client secret would otherwise fix csm-portal and silently
// leave PLG authenticating with the old one.
func TestEntitySettingsAreInherited(t *testing.T) {
	clearPLGEnv(t)

	cfg, err := LoadWith("does-not-exist.json", EntityDefaults{
		BaseURL:      "https://entity.example",
		TokenURL:     "https://sts.example/token",
		ClientID:     "csm-app",
		ClientSecret: "s3cret",
		Scope:        "read",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Entity.BaseURL != "https://entity.example" {
		t.Errorf("baseURL = %q, want the inherited one", cfg.Entity.BaseURL)
	}
	if !cfg.Entity.OAuth.Enabled() {
		t.Fatal("the inherited grant should be enabled")
	}
	if cfg.Entity.OAuth.ClientID != "csm-app" || cfg.Entity.OAuth.ClientSecret != "s3cret" {
		t.Errorf("credentials not inherited: %+v", cfg.Entity.OAuth)
	}
}

// An explicit PLG_* value wins, for the case where PLG must reach entity-service
// as a different application.
func TestPLGOverridesBeatInheritance(t *testing.T) {
	clearPLGEnv(t)
	t.Setenv("PLG_ENTITY_BASE_URL", "https://plg-only.example")
	t.Setenv("PLG_ENTITY_OAUTH_CLIENT_ID", "plg-app")

	cfg, err := LoadWith("does-not-exist.json", EntityDefaults{
		BaseURL:      "https://entity.example",
		TokenURL:     "https://sts.example/token",
		ClientID:     "csm-app",
		ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Entity.BaseURL != "https://plg-only.example" {
		t.Errorf("baseURL = %q, want the override", cfg.Entity.BaseURL)
	}
	if cfg.Entity.OAuth.ClientID != "plg-app" {
		t.Errorf("clientID = %q, want the override", cfg.Entity.OAuth.ClientID)
	}
	// Anything the override did not name is still inherited.
	if cfg.Entity.OAuth.ClientSecret != "s3cret" {
		t.Errorf("clientSecret = %q, want the inherited one", cfg.Entity.OAuth.ClientSecret)
	}
}

// With nothing to inherit and nothing set, startup fails. It used to default to
// http://localhost:8200, which meant a deployment that forgot the variable came
// up healthy and failed every PLG request against a port that was not there.
func TestMissingEntityBaseURLIsFatal(t *testing.T) {
	clearPLGEnv(t)

	_, err := LoadWith("does-not-exist.json", EntityDefaults{})
	if err == nil {
		t.Fatal("want an error when no base URL is available from anywhere")
	}
	if !strings.Contains(err.Error(), "entity.baseUrl") {
		t.Errorf("error should name the setting, got: %v", err)
	}
}

// A half-configured grant is refused rather than carried to the first request,
// where it would look like an authorisation problem instead of a config one.
func TestPartialEntityGrantIsRefused(t *testing.T) {
	clearPLGEnv(t)

	_, err := LoadWith("does-not-exist.json", EntityDefaults{
		BaseURL:  "https://entity.example",
		ClientID: "csm-app", // no token URL, no secret
	})
	if err == nil {
		t.Fatal("want an error for a grant missing tokenUrl and clientSecret")
	}
	if !strings.Contains(err.Error(), "entity.oauth2") {
		t.Errorf("error should name the block, got: %v", err)
	}
}

// The client secret must not reach a log.
func TestEntityClientSecretIsRedacted(t *testing.T) {
	clearPLGEnv(t)

	cfg, err := LoadWith("does-not-exist.json", EntityDefaults{
		BaseURL:      "https://entity.example",
		TokenURL:     "https://sts.example/token",
		ClientID:     "csm-app",
		ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Redacted().Entity.OAuth.ClientSecret; got != "********" {
		t.Errorf("redacted secret = %q", got)
	}
	if cfg.Entity.OAuth.ClientSecret != "s3cret" {
		t.Error("Redacted must not mutate the original")
	}
}

// clearPLGEnv removes any PLG_* variable the ambient environment happens to
// carry, so these tests describe the code rather than the developer's shell.
func clearPLGEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, "PLG_") {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
}

// The Cloud Directory API is reached as its own machine client on the WSO2
// Cloud identity platform, not as the shared OAuth2 application, so none of
// the shared credentials may leak into it.
func TestCloudGrantIsNotInherited(t *testing.T) {
	clearPLGEnv(t)
	for _, k := range []string{"CLOUD_API_TOKEN_URL", "CLOUD_API_CLIENT_ID", "CLOUD_API_CLIENT_SECRET", "CLOUD_API_SCOPES"} {
		t.Setenv(k, "")
	}
	t.Setenv("CLOUD_API_BASE_URL", "https://cloud.example/directory")

	cfg, err := LoadWith("does-not-exist.json", EntityDefaults{
		BaseURL:      "https://entity.example",
		TokenURL:     "https://sts.example/token",
		ClientID:     "csm-app",
		ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Cloud.OAuth.Enabled() {
		t.Errorf("the cloud grant inherited the shared credentials: %+v", cfg.Cloud.OAuth)
	}
	if cfg.Cloud.OrgPath != "/orgs/{orgId}" || cfg.Cloud.UserEmailPath != "/users/{userId}/email" {
		t.Errorf("paths = %q, %q", cfg.Cloud.OrgPath, cfg.Cloud.UserEmailPath)
	}

	t.Setenv("CLOUD_API_TOKEN_URL", "https://accounts.cloud.example/oauth2/token")
	t.Setenv("CLOUD_API_CLIENT_ID", "CSM_PORTAL")
	t.Setenv("CLOUD_API_CLIENT_SECRET", "cloud-secret")
	cfg, err = LoadWith("does-not-exist.json", EntityDefaults{BaseURL: "https://entity.example"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Cloud.OAuth.ClientID != "CSM_PORTAL" || cfg.Redacted().Cloud.OAuth.ClientSecret != "********" {
		t.Errorf("cloud grant = %+v", cfg.Redacted().Cloud.OAuth)
	}
}
