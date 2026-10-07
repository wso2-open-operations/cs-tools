// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package config loads tunables from config.toml (over built-in defaults) and secrets from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/caarlos0/env/v11"
)

// DefaultPath is used when CONFIG_PATH is unset; expected at the working directory root.
const DefaultPath = "config.toml"

// WriteMargin keeps request_wait under write_timeout, so a slow store answers 503, not a cut connection.
const WriteMargin = Duration(time.Second)

// Config groups every deployment tunable by the subsystem it configures.
type Config struct {
	Server    ServerConfig    `toml:"server"`
	Allocator AllocatorConfig `toml:"allocator"`
	Store     StoreConfig     `toml:"store"`
	Postgres  PostgresConfig  `toml:"postgres"`
	Wake      WakeConfig      `toml:"wake"`
	Reject    RejectConfig    `toml:"reject"`
	Log       LogConfig       `toml:"log"`
	Payloads  PayloadsConfig  `toml:"payloads"`
	// LegacyAuthSection flags a leftover [auth] table, no longer read now auth is AUTH_ENABLED.
	LegacyAuthSection bool `toml:"-"`
}

// ServerConfig tunes the HTTP server: shutdown drain, read/write timeouts and the 413 body limit.
type ServerConfig struct {
	ShutdownGrace  Duration `toml:"shutdown_grace"`
	DrainDelay     Duration `toml:"drain_delay"`
	RequestWait    Duration `toml:"request_wait"`
	AllocatorDrain Duration `toml:"allocator_drain"`
	// PayloadDrain is reserved after allocator_drain for the final raw_alerts insert.
	PayloadDrain Duration `toml:"payload_drain"`
	ReadTimeout  Duration `toml:"read_timeout"`
	WriteTimeout Duration `toml:"write_timeout"`
	IdleTimeout  Duration `toml:"idle_timeout"`
	MaxBodyBytes int64    `toml:"max_body_bytes"`
}

// AllocatorConfig tunes the id allocator: queue depth, batch size, and writer concurrency.
type AllocatorConfig struct {
	QueueSize        int   `toml:"queue_size"`
	QueueMaxBytes    int64 `toml:"queue_max_bytes"`
	MaxBatch         int   `toml:"max_batch"`
	WriteConcurrency int   `toml:"write_concurrency"`
}

// StoreConfig tunes row writes: attempts per id, the doubling backoff base and the query timeout.
type StoreConfig struct {
	InsertAttempts  int      `toml:"insert_attempts"`
	InsertBaseDelay Duration `toml:"insert_base_delay"`
	QueryTimeout    Duration `toml:"query_timeout"`
	ClaimTimeout    Duration `toml:"claim_timeout"`
	WriteDeadline   Duration `toml:"write_deadline"`
}

// PostgresConfig tunes startup connection retry, matching sre-alert-core-service, plus the warm pool floor and the credential check budget.
type PostgresConfig struct {
	ConnectMaxAttempts int      `toml:"connect_max_attempts"`
	ConnectBaseDelay   Duration `toml:"connect_base_delay"`
	ConnectTimeout     Duration `toml:"connect_timeout"`
	// MinConns keeps this many connections open so a webhook after a quiet spell doesn't pay for a new TLS connection.
	MinConns int `toml:"min_conns"`
	// AuthTimeout bounds one integration_users refresh query.
	AuthTimeout Duration `toml:"auth_timeout"`
	// AuthRefreshInterval is how often the in-memory copy of integration_users is reloaded.
	AuthRefreshInterval Duration `toml:"auth_refresh_interval"`
	// AuthMaxStale is how long the last good copy serves while refreshes fail, before auth answers 503.
	AuthMaxStale Duration `toml:"auth_max_stale"`
}

// WakeConfig bounds the fire-and-forget POST /alertz to alerts-core.
type WakeConfig struct {
	Timeout Duration `toml:"timeout"`
}

// RejectConfig tunes how much of a rejected webhook's body is kept for logging.
type RejectConfig struct {
	BodyPreviewChars int `toml:"body_preview_chars"`
}

// LogConfig bounds the raw webhook body logged before each transform.
type LogConfig struct {
	// PayloadMaxBytes caps the logged body; a longer one is logged truncated, and 0 turns the line off.
	PayloadMaxBytes int64 `toml:"payload_max_bytes"`
}

// PayloadsConfig tunes the in-memory buffer of raw webhook bodies written to raw_alerts.
type PayloadsConfig struct {
	FlushInterval  Duration `toml:"flush_interval"`
	MaxBufferBytes int64    `toml:"max_buffer_bytes"`
	FlushTimeout   Duration `toml:"flush_timeout"`
}

// Duration wraps time.Duration so TOML values like "30s" decode via time.ParseDuration.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for quoted duration strings.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	*d = Duration(parsed)
	return nil
}

// Duration unwraps to a plain time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Defaults returns every tunable's production value, matching config.toml.example.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			ShutdownGrace:  Duration(30 * time.Second),
			DrainDelay:     Duration(5 * time.Second),
			RequestWait:    Duration(10 * time.Second),
			AllocatorDrain: Duration(10 * time.Second),
			PayloadDrain:   Duration(5 * time.Second),
			ReadTimeout:    Duration(10 * time.Second),
			WriteTimeout:   Duration(30 * time.Second),
			IdleTimeout:    Duration(60 * time.Second),
			MaxBodyBytes:   1 << 20,
		},
		Allocator: AllocatorConfig{
			QueueSize:        10000,
			QueueMaxBytes:    256 << 20,
			MaxBatch:         500,
			WriteConcurrency: 16,
		},
		Store: StoreConfig{
			InsertAttempts:  5,
			InsertBaseDelay: Duration(250 * time.Millisecond),
			QueryTimeout:    Duration(2 * time.Second),
			ClaimTimeout:    Duration(5 * time.Second),
			WriteDeadline:   Duration(8 * time.Second),
		},
		Postgres: PostgresConfig{
			ConnectMaxAttempts:  5,
			ConnectBaseDelay:    Duration(2 * time.Second),
			ConnectTimeout:      Duration(10 * time.Second),
			MinConns:            2,
			AuthTimeout:         Duration(5 * time.Second),
			AuthRefreshInterval: Duration(30 * time.Second),
			AuthMaxStale:        Duration(15 * time.Minute),
		},
		Wake:   WakeConfig{Timeout: Duration(2 * time.Second)},
		Reject: RejectConfig{BodyPreviewChars: 500},
		Log:    LogConfig{PayloadMaxBytes: 64 << 10},
		Payloads: PayloadsConfig{
			FlushInterval:  Duration(5 * time.Minute),
			MaxBufferBytes: 32 << 20,
			FlushTimeout:   Duration(30 * time.Second),
		},
	}
}

// Load reads CONFIG_PATH or DefaultPath over Defaults, field by field; a missing file is fine.
func Load(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = DefaultPath
	}

	cfg := Defaults()
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	// [auth] is no longer read; flag it so a deployment expecting auth isn't left without it.
	cfg.LegacyAuthSection = md.IsDefined("auth")
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

// Validate rejects zero/negative tunables that would silently disable limits, retries or timeouts.
func (c Config) Validate() error {
	switch {
	case c.Server.ShutdownGrace <= 0:
		return fmt.Errorf("server.shutdown_grace must be positive")
	case c.Server.DrainDelay <= 0:
		return fmt.Errorf("server.drain_delay must be positive")
	case c.Server.RequestWait <= 0:
		return fmt.Errorf("server.request_wait must be positive")
	case c.Server.AllocatorDrain <= 0:
		return fmt.Errorf("server.allocator_drain must be positive")
	case c.Server.PayloadDrain <= 0:
		return fmt.Errorf("server.payload_drain must be positive")
	case c.Server.DrainDelay+c.Server.RequestWait+c.Server.AllocatorDrain+c.Server.PayloadDrain > c.Server.ShutdownGrace:
		return fmt.Errorf("server.drain_delay + request_wait + allocator_drain + payload_drain must not exceed shutdown_grace")
	case c.Server.ReadTimeout <= 0:
		return fmt.Errorf("server.read_timeout must be positive")
	case c.Server.WriteTimeout <= 0:
		return fmt.Errorf("server.write_timeout must be positive")
	case c.Server.RequestWait+WriteMargin > c.Server.WriteTimeout:
		return fmt.Errorf("server.request_wait must be at least %v below write_timeout", WriteMargin.Duration())
	case c.Server.IdleTimeout <= 0:
		return fmt.Errorf("server.idle_timeout must be positive")
	case c.Server.MaxBodyBytes <= 0:
		return fmt.Errorf("server.max_body_bytes must be positive")
	case c.Allocator.QueueSize <= 0:
		return fmt.Errorf("allocator.queue_size must be positive")
	case c.Allocator.QueueMaxBytes <= 0:
		return fmt.Errorf("allocator.queue_max_bytes must be positive")
	case c.Allocator.QueueMaxBytes < 2*c.Server.MaxBodyBytes:
		return fmt.Errorf("allocator.queue_max_bytes must be at least 2 x server.max_body_bytes")
	case c.Allocator.MaxBatch <= 0:
		return fmt.Errorf("allocator.max_batch must be positive")
	case c.Allocator.WriteConcurrency <= 0:
		return fmt.Errorf("allocator.write_concurrency must be positive")
	case c.Store.InsertAttempts <= 0:
		return fmt.Errorf("store.insert_attempts must be positive")
	case c.Store.InsertBaseDelay <= 0:
		return fmt.Errorf("store.insert_base_delay must be positive")
	case c.Store.QueryTimeout <= 0:
		return fmt.Errorf("store.query_timeout must be positive")
	case c.Store.ClaimTimeout <= 0:
		return fmt.Errorf("store.claim_timeout must be positive")
	case c.Store.WriteDeadline <= 0:
		return fmt.Errorf("store.write_deadline must be positive")
	case c.Store.WriteDeadline >= c.Server.RequestWait:
		return fmt.Errorf("store.write_deadline must be under server.request_wait, or a sender can get 503 for an alert that was stored and resend it")
	case c.Postgres.ConnectMaxAttempts <= 0:
		return fmt.Errorf("postgres.connect_max_attempts must be positive")
	case c.Postgres.ConnectBaseDelay <= 0:
		return fmt.Errorf("postgres.connect_base_delay must be positive")
	case c.Postgres.ConnectTimeout <= 0:
		return fmt.Errorf("postgres.connect_timeout must be positive")
	case c.Postgres.MinConns < 0:
		return fmt.Errorf("postgres.min_conns must not be negative")
	case c.Postgres.AuthTimeout <= 0:
		return fmt.Errorf("postgres.auth_timeout must be positive")
	case c.Postgres.AuthRefreshInterval <= 0:
		return fmt.Errorf("postgres.auth_refresh_interval must be positive")
	case c.Postgres.AuthMaxStale <= c.Postgres.AuthRefreshInterval:
		return fmt.Errorf("postgres.auth_max_stale must exceed postgres.auth_refresh_interval, or one slow refresh would fail every request")
	case c.Wake.Timeout <= 0:
		return fmt.Errorf("wake.timeout must be positive")
	case c.Reject.BodyPreviewChars <= 0:
		return fmt.Errorf("reject.body_preview_chars must be positive")
	case c.Log.PayloadMaxBytes < 0:
		return fmt.Errorf("log.payload_max_bytes must not be negative")
	case c.Payloads.FlushInterval <= 0:
		return fmt.Errorf("payloads.flush_interval must be positive")
	case c.Payloads.FlushTimeout <= 0:
		return fmt.Errorf("payloads.flush_timeout must be positive")
	case c.Payloads.MaxBufferBytes < 2*c.Server.MaxBodyBytes:
		return fmt.Errorf("payloads.max_buffer_bytes must be at least 2 x server.max_body_bytes, so the largest body fits under the early-flush mark")
	}
	return nil
}

// Env is read from the environment; PG* and <SOURCE>_ALERT_CONFIG are read by their packages.
type Env struct {
	Port string `env:"PORT" envDefault:"8080"`
	// WakeURL is alerts-core's POST /alertz; empty skips the wake-up and the 10s poll still runs.
	WakeURL string `env:"ALERT_CORE_WAKE_URL"`
	// AuthEnabledRaw is AUTH_ENABLED, unparsed; read AuthEnabled.
	AuthEnabledRaw string `env:"AUTH_ENABLED"`
	// AuthAuditOnlyRaw is AUTH_AUDIT_ONLY, unparsed; read AuthAuditOnly.
	AuthAuditOnlyRaw string `env:"AUTH_AUDIT_ONLY"`
	AuthEnabled      bool   `env:"-"`
	AuthAuditOnly    bool   `env:"-"`
	// WakeToken is the shared ALERT_CORE_WAKE_TOKEN alerts-core checks on /alertz, sent only over https.
	WakeToken string `env:"ALERT_CORE_WAKE_TOKEN"`
	// DBFallbackChatURL is the Google Chat webhook that gets alerts the database could not store; empty only logs them.
	DBFallbackChatURL string `env:"DB_FALLBACK_CHAT_WEBHOOK_URL"`
}

// LoadEnv parses Env.
func LoadEnv() (Env, error) {
	var e Env
	if err := env.Parse(&e); err != nil {
		return Env{}, fmt.Errorf("env config: %w", err)
	}
	e.WakeURL = strings.TrimSpace(e.WakeURL)
	e.WakeToken = strings.TrimSpace(e.WakeToken)
	e.DBFallbackChatURL = strings.TrimSpace(e.DBFallbackChatURL)
	var err error
	if e.AuthEnabled, err = parseBool("AUTH_ENABLED", e.AuthEnabledRaw); err != nil {
		return Env{}, err
	}
	if e.AuthAuditOnly, err = parseBool("AUTH_AUDIT_ONLY", e.AuthAuditOnlyRaw); err != nil {
		return Env{}, err
	}
	return e, nil
}

// parseBool reads a true/false switch (case and spaces ignored); anything else fails startup.
func parseBool(name, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false", "0", "no", "off":
		return false, nil
	case "true", "1", "yes", "on":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false, got %q", name, strings.TrimSpace(raw))
	}
}
