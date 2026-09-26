// Package config loads runtime configuration for the PLG CS portal backend.
//
// Configuration comes from a JSON file (default ./config.json, override with
// PLG_CONFIG_FILE or the -config flag). Every value can also be overridden by
// an environment variable, which is what a container deployment will use — the
// file is for local development.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ServerConfig describes how the HTTP server is exposed.
type ServerConfig struct {
	Port int `json:"port"`
	// AllowedOrigins is the CORS allow-list for the local webapp dev server.
	// Empty disables CORS handling entirely.
	AllowedOrigins []string `json:"allowedOrigins"`
	// RequestTimeoutSeconds bounds every request. Zero means 15.
	RequestTimeoutSeconds int `json:"requestTimeoutSeconds"`
}

// OAuth2Config describes the client-credentials grant the portal uses to reach a
// protected queue.
//
// Client credentials rather than any interactive flow, because the portal acts
// as itself: there is no user behind a background poll and nothing to consent
// to.
type OAuth2Config struct {
	// TokenURL is the authorisation server's token endpoint.
	TokenURL string `json:"tokenUrl"`
	// ClientID and ClientSecret identify the portal. Sent as HTTP Basic
	// credentials, which is what RFC 6749 prefers and what Choreo's STS accepts.
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	// Scope is optional and passed through verbatim when set.
	Scope string `json:"scope"`
}

// Enabled reports whether the client-credentials grant is configured at all.
// Any one field being present counts, so a half-filled block is caught by
// validate rather than silently ignored.
func (o OAuth2Config) Enabled() bool {
	return o.TokenURL != "" || o.ClientID != "" || o.ClientSecret != ""
}

// validate refuses a half-configured grant, and refuses to put a client secret
// on the wire in plaintext.

// validateNamed is validate with the configuration block's name in the message,
// so a failure says which grant is wrong when there is more than one.
func (o OAuth2Config) validateNamed(what string) error {
	if !o.Enabled() {
		return nil
	}

	var missing []string
	if o.TokenURL == "" {
		missing = append(missing, "tokenUrl")
	}
	if o.ClientID == "" {
		missing = append(missing, "clientId")
	}
	if o.ClientSecret == "" {
		missing = append(missing, "clientSecret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s is missing %s — a client-credentials grant needs all three",
			what, strings.Join(missing, " and "))
	}

	u, err := url.Parse(o.TokenURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%s.tokenUrl %q must be an absolute http(s) URL", what, o.TokenURL)
	}
	// A client secret over plaintext HTTP is readable by anything on the path.
	// Permitted against loopback, because that is how this gets tested.
	if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return fmt.Errorf("%s.tokenUrl %q is plaintext http to a remote host — "+
			"the client secret would travel in the clear", what, o.TokenURL)
	}
	return nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Config is the fully resolved runtime configuration.
type Config struct {
	// Entity is where the data lives. The BFF has no database of its own — it
	// reaches entity-service over HTTP, so this is a base URL.
	Entity  EntityConfig  `json:"entity"`
	Server  ServerConfig  `json:"server"`
	Logging LoggingConfig `json:"logging"`
}

// EntityConfig points the BFF at entity-service.
//
// No credentials. entity-service has no authentication — its security is
// network placement — so there is nothing to configure. When the slice merges
// and sits behind the Choreo gateway this grows a client-credentials block, and
// nothing else here changes.
type EntityConfig struct {
	// BaseURL is entity-service's root, e.g. http://localhost:8200.
	//
	// Left empty, it is filled from what csm-portal already knows — see
	// EntityDefaults. PLG and csm-portal talk to the same entity-service, so
	// this only needs setting when they must differ.
	BaseURL string `json:"baseUrl"`

	// OAuth is the client-credentials grant used to reach entity-service.
	//
	// entity-service sits behind a gateway that requires a token, and PLG
	// authenticates as the same OAuth2 application as every other upstream
	// client in this backend. Normally inherited from csm-portal rather than
	// configured here — see EntityDefaults.
	OAuth OAuth2Config `json:"oauth2"`
	// TimeoutSeconds bounds one call. Defaults to 20.
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// LoggingConfig is the one place logging is configured.
//
// Both fields exist because a deployment wants something different from a
// laptop. Locally, text at DEBUG is readable and greppable; in a platform that
// collects stdout into a searchable store, JSON at INFO means every line's
// correlation id, user and status are queryable fields rather than substrings
// somebody has to write a regex for.
type LoggingConfig struct {
	// Level is DEBUG, INFO, WARN or ERROR. Anything below it is dropped before
	// it is formatted, so a silenced DEBUG line costs almost nothing.
	Level string `json:"level"`

	// Format is "text" or "json".
	Format string `json:"format"`
}

// Load reads the JSON config file at path (a missing file is not an error —
// environment variables alone can supply everything), applies environment
// overrides, fills in defaults, and validates the result.
// EntityDefaults is what csm-portal already knows about entity-service, handed
// to PLG so it does not keep a second copy of settings describing one service.
//
// PLG's own PLG_* settings win where they are set; anything left empty is
// filled from here. In practice nothing needs setting: both halves of this
// process talk to the same entity-service as the same OAuth2 application.
type EntityDefaults struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
}

// Load reads the configuration with no inherited defaults. Used by tests; the
// server calls LoadWith so PLG inherits csm-portal's entity-service settings.
func Load(path string) (*Config, error) { return LoadWith(path, EntityDefaults{}) }

// LoadWith reads the configuration, filling anything PLG did not set from d.
func LoadWith(path string, d EntityDefaults) (*Config, error) {
	cfg := defaults()

	if path == "" {
		path = envOr("PLG_CONFIG_FILE", "config.json")
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied config path
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", path, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		// Fall through: environment variables must supply the required values.
	default:
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	applyEnvOverrides(cfg)
	applyEntityDefaults(cfg, d)
	applyDefaults(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Entity: EntityConfig{TimeoutSeconds: 20},
		Server: ServerConfig{Port: 8100},
	}
}

// applyEntityDefaults inherits csm-portal's entity-service settings for every
// field PLG left empty. Called after the environment, so an explicit PLG_* value
// always wins.
func applyEntityDefaults(c *Config, d EntityDefaults) {
	if c.Entity.BaseURL == "" {
		c.Entity.BaseURL = d.BaseURL
	}
	if c.Entity.OAuth.TokenURL == "" {
		c.Entity.OAuth.TokenURL = d.TokenURL
	}
	if c.Entity.OAuth.ClientID == "" {
		c.Entity.OAuth.ClientID = d.ClientID
	}
	if c.Entity.OAuth.ClientSecret == "" {
		c.Entity.OAuth.ClientSecret = d.ClientSecret
	}
	if c.Entity.OAuth.Scope == "" {
		c.Entity.OAuth.Scope = d.Scope
	}
}

func applyDefaults(c *Config) {
	if c.Entity.TimeoutSeconds == 0 {
		c.Entity.TimeoutSeconds = 20
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8090
	}
	if c.Server.RequestTimeoutSeconds == 0 {
		c.Server.RequestTimeoutSeconds = 15
	}
	if c.Logging.Level == "" {
		// INFO, not ERROR: the access log and the poller's per-batch summary are
		// INFO, and they are the two things anyone actually wants when asking
		// what the portal has been doing.
		c.Logging.Level = "INFO"
	}
	if c.Logging.Format == "" {
		// Text by default so a laptop's terminal stays readable. Deployments set
		// json, which is what makes the fields queryable.
		c.Logging.Format = "text"
	}

}

func applyEnvOverrides(c *Config) {
	setString(&c.Entity.BaseURL, "PLG_ENTITY_BASE_URL")
	// Overrides, for the unusual case where PLG must reach entity-service as a
	// different application than the rest of this backend. Normally unset.
	setString(&c.Entity.OAuth.TokenURL, "PLG_ENTITY_OAUTH_TOKEN_URL")
	setString(&c.Entity.OAuth.ClientID, "PLG_ENTITY_OAUTH_CLIENT_ID")
	setString(&c.Entity.OAuth.ClientSecret, "PLG_ENTITY_OAUTH_CLIENT_SECRET")
	setString(&c.Entity.OAuth.Scope, "PLG_ENTITY_SCOPES")
	setInt(&c.Entity.TimeoutSeconds, "PLG_ENTITY_TIMEOUT_SECONDS")
	setInt(&c.Server.Port, "PLG_SERVER_PORT")
	// The only config.json key that had no environment override. It mattered
	// because Choreo deploys from environment variables alone: the value was
	// reachable in a local config file and unreachable in the place it would
	// actually need tuning.
	setInt(&c.Server.RequestTimeoutSeconds, "PLG_SERVER_REQUEST_TIMEOUT_SECONDS")
	if v := os.Getenv("PLG_ALLOWED_ORIGINS"); v != "" {
		c.Server.AllowedOrigins = splitList(v)
	}
	setString(&c.Logging.Level, "PLG_LOG_LEVEL")
	setString(&c.Logging.Format, "PLG_LOG_FORMAT")

}

// splitList parses a comma-separated environment variable.
func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Validate reports configuration that would fail at runtime in a confusing way.
func (c *Config) Validate() error {
	// The one setting a deployment cannot be without. A BFF that cannot reach
	// entity-service has no data at all, so this fails at startup rather than on
	// the first request.
	if strings.TrimSpace(c.Entity.BaseURL) == "" {
		return errors.New("entity.baseUrl is required — normally inherited from " +
			"CUSTOMER_ENTITY_BASE_URL, or set PLG_ENTITY_BASE_URL to override it")
	}
	if err := c.Entity.OAuth.validateNamed("entity.oauth2"); err != nil {
		return err
	}
	if !strings.HasPrefix(c.Entity.BaseURL, "http://") && !strings.HasPrefix(c.Entity.BaseURL, "https://") {
		return fmt.Errorf("entity.baseUrl %q must be an absolute http(s) URL", c.Entity.BaseURL)
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range", c.Server.Port)
	}

	// Refused at startup rather than silently defaulted: a typo like "WARNING"
	// or "pretty" would otherwise leave the portal logging at a level nobody
	// chose, and the mistake only shows up when a log someone needed is absent.
	switch strings.ToUpper(c.Logging.Level) {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("logging.level %q must be DEBUG, INFO, WARN or ERROR", c.Logging.Level)
	}
	switch strings.ToLower(c.Logging.Format) {
	case "text", "json":
	default:
		return fmt.Errorf("logging.format %q must be \"text\" or \"json\"", c.Logging.Format)
	}

	return nil
}

// Redacted returns a copy safe to log.
//
// The BFF has no database and no ingest credentials of its own, so the only
// secret left is the entity-service client secret.
func (c *Config) Redacted() Config {
	cp := *c
	if cp.Entity.OAuth.ClientSecret != "" {
		cp.Entity.OAuth.ClientSecret = "********"
	}
	return cp
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func setString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func setInt(dst *int, key string) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}
