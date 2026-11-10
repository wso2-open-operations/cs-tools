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

// Package config loads deployment tunables from a TOML file, validating every value before returning it.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultPath is used when CONFIG_PATH is unset; expected at the repo or deployment root.
const DefaultPath = "config.toml"

// Config groups every deployment tunable, previously hardcoded constants, by the subsystem it configures. Security-sensitive constants (e.g. auth.Iterations, the PBKDF2 round count) intentionally stay as Go constants rather than config.toml fields, since they're not meant to vary per deployment.
type Config struct {
	Poll      PollConfig      `toml:"poll"`
	Postgres  PostgresConfig  `toml:"postgres"`
	Notify    NotifyConfig    `toml:"notify"`
	Server    ServerConfig    `toml:"server"`
	Engine    EngineConfig    `toml:"engine"`
	Retention RetentionConfig `toml:"retention"`
}

// RetentionConfig bounds how long processed alerts and settled incidents are kept.
type RetentionConfig struct {
	// Interval is how often one replica purges expired rows.
	Interval Duration `toml:"interval"`
	// Alerts is how long a processed alert row, and a raw_alerts webhook body, is kept.
	Alerts Duration `toml:"alerts"`
	// Incidents is how long an incident with nothing left to deliver is kept after its last alert.
	Incidents Duration `toml:"incidents"`
}

// EngineConfig tunes the dedup engine's fixed duplicate-folding window.
type EngineConfig struct {
	// DedupWindow bounds how long an incident absorbs duplicates before starting fresh, even if CSM still reports it open.
	DedupWindow Duration `toml:"dedup_window"`
}

// PollConfig tunes the poller's cadence, concurrency, and batch size; applies per replica, no leader election.
type PollConfig struct {
	// Interval is the backstop cadence; POST /alertz drives real-time pickup, so this only bounds how long a dropped ping goes unnoticed.
	Interval Duration `toml:"interval"`
	// Concurrency is fingerprint-sharded worker count per replica; same-fingerprint alerts stay serialized on one worker.
	Concurrency int `toml:"concurrency"`
	// MaxBatch caps the seed rows of one claim; siblings sharing a seed's fingerprint can add as many again, and 2 x MaxBatch alerts can be in flight.
	MaxBatch int `toml:"max_batch"`
	// ClaimTTL bounds how long a claimed-but-unfinished alert is held before any replica may reclaim it, recovering work stranded by a crashed replica.
	ClaimTTL Duration `toml:"claim_ttl"`
}

// PostgresConfig tunes startup connection retry attempts, backoff delay, connect timeout, and the per-query timeout used for every Postgres call.
type PostgresConfig struct {
	ConnectMaxAttempts int      `toml:"connect_max_attempts"`
	ConnectBaseDelay   Duration `toml:"connect_base_delay"`
	ConnectTimeout     Duration `toml:"connect_timeout"`
	QueryTimeout       Duration `toml:"query_timeout"`
}

// NotifyConfig tunes retry attempts, backoff delay, and per-call timeout for outbound CSM and Chat webhook requests.
type NotifyConfig struct {
	MaxAttempts    int      `toml:"max_attempts"`
	RetryBaseDelay Duration `toml:"retry_base_delay"`
	HTTPTimeout    Duration `toml:"http_timeout"`
	// RetrySweepInterval is the longest delivery waits when no fold triggers it; it bounds retry latency.
	RetrySweepInterval Duration `toml:"retry_sweep_interval"`
	// MaxCSMAttempts caps failed attempts before marking permanently failed, so bad payloads don't grow RetrySweep's scan cost forever.
	MaxCSMAttempts int `toml:"max_csm_attempts"`
	// ServiceCacheTTL bounds how long a label->CMDB-service-id resolution is reused before a fresh live /services/search call.
	ServiceCacheTTL Duration `toml:"service_cache_ttl"`
	// StateCheckInterval throttles how often a confirmed incident's status is re-checked against CSM, else a flapping alert costs one search per duplicate.
	StateCheckInterval Duration `toml:"state_check_interval"`
	// CSMRetryBaseDelay is the wait before the first RetrySweep-driven CSM retry after a failed attempt.
	CSMRetryBaseDelay Duration `toml:"csm_retry_base_delay"`
	// CSMRetryMultiplier grows the wait between successive CSM retries during a prolonged outage.
	CSMRetryMultiplier float64 `toml:"csm_retry_multiplier"`
	// CSMRetryMaxDelay caps how long the exponential CSM retry wait can grow to.
	CSMRetryMaxDelay Duration `toml:"csm_retry_max_delay"`
	// ChatFallbackDelay is how long CSM gets to confirm a new incident before it is posted to the Chat fallback; zero posts on the first CSM failure.
	ChatFallbackDelay Duration `toml:"chat_fallback_delay"`
	// ChatThreadingEnabled threads each incident's Chat fallback card and its Duplicate/OK replies into one Google Chat thread; a new incident after the dedup window gets its own thread.
	ChatThreadingEnabled bool `toml:"chat_threading_enabled"`
	// DeliveryConcurrency is how many incidents each replica delivers to CSM/Chat in parallel, independent of alert processing.
	DeliveryConcurrency int `toml:"delivery_concurrency"`
	// MuteRuleRefreshInterval is how often the active alert mute rules are fetched from CSM, and how often a muted incident is re-checked against them.
	MuteRuleRefreshInterval Duration `toml:"mute_rule_refresh_interval"`
	// MuteRuleMaxStaleness is how long the last fetched alert mute rules are still used while refreshes fail; after that nothing is muted (fail open).
	MuteRuleMaxStaleness Duration `toml:"mute_rule_max_staleness"`
}

// ServerConfig tunes how long the HTTP server waits for in-flight requests to drain during a graceful shutdown before forcing the process to exit.
type ServerConfig struct {
	ShutdownGrace Duration `toml:"shutdown_grace"`
}

// Duration wraps time.Duration so human-readable TOML values like "30s" decode correctly via the standard library's time.ParseDuration function.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler, the interface the TOML decoder invokes to parse quoted duration strings like "30s" into a Duration value.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	*d = Duration(parsed)
	return nil
}

// Duration unwraps to a plain time.Duration for standard library timer and context functions.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// defaults holds every tunable's production value, used as-is when config.toml is absent, or as the base a present config.toml overrides field by field.
func defaults() Config {
	return Config{
		Poll: PollConfig{
			Interval:    Duration(10 * time.Second),
			Concurrency: 128,
			MaxBatch:    500,
			ClaimTTL:    Duration(2 * time.Minute),
		},
		Postgres: PostgresConfig{
			ConnectMaxAttempts: 5,
			ConnectBaseDelay:   Duration(2 * time.Second),
			ConnectTimeout:     Duration(10 * time.Second),
			QueryTimeout:       Duration(5 * time.Second),
		},
		Notify: NotifyConfig{
			MaxAttempts:          3,
			RetryBaseDelay:       Duration(200 * time.Millisecond),
			HTTPTimeout:          Duration(10 * time.Second),
			RetrySweepInterval:   Duration(15 * time.Second),
			MaxCSMAttempts:       20,
			ServiceCacheTTL:      Duration(time.Hour),
			StateCheckInterval:   Duration(2 * time.Minute),
			CSMRetryBaseDelay:    Duration(30 * time.Second),
			CSMRetryMultiplier:   2,
			CSMRetryMaxDelay:     Duration(15 * time.Minute),
			ChatThreadingEnabled: true,
			ChatFallbackDelay:    Duration(25 * time.Second),
			DeliveryConcurrency:  64,

			MuteRuleRefreshInterval: Duration(30 * time.Second),
			MuteRuleMaxStaleness:    Duration(5 * time.Minute),
		},
		Server: ServerConfig{
			ShutdownGrace: Duration(15 * time.Second),
		},
		Engine: EngineConfig{
			DedupWindow: Duration(5 * time.Minute),
		},
		Retention: RetentionConfig{
			Interval:  Duration(time.Hour),
			Alerts:    Duration(7 * 24 * time.Hour),
			Incidents: Duration(30 * 24 * time.Hour),
		},
	}
}

// Load falls back to CONFIG_PATH env var, then DefaultPath, when path is empty; every tunable starts at its built-in default, and config.toml values override field by field, so partial files work fine and missing files are not an error.
func Load(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = DefaultPath
	}

	cfg := defaults()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("load config %s: %w", path, err)
		}
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

// validate rejects zero/negative tunables that would silently disable retries or timeouts.
func (c Config) validate() error {
	switch {
	case c.Poll.Interval <= 0:
		return fmt.Errorf("poll.interval must be positive")
	case c.Poll.Concurrency <= 0:
		return fmt.Errorf("poll.concurrency must be positive")
	case c.Poll.MaxBatch <= 0:
		return fmt.Errorf("poll.max_batch must be positive")
	case c.Poll.ClaimTTL <= 0:
		return fmt.Errorf("poll.claim_ttl must be positive")
	case c.Postgres.ConnectMaxAttempts <= 0:
		return fmt.Errorf("postgres.connect_max_attempts must be positive")
	case c.Postgres.ConnectBaseDelay <= 0:
		return fmt.Errorf("postgres.connect_base_delay must be positive")
	case c.Postgres.ConnectTimeout <= 0:
		return fmt.Errorf("postgres.connect_timeout must be positive")
	case c.Postgres.QueryTimeout <= 0:
		return fmt.Errorf("postgres.query_timeout must be positive")
	case c.Notify.MaxAttempts <= 0:
		return fmt.Errorf("notify.max_attempts must be positive")
	case c.Notify.RetryBaseDelay <= 0:
		return fmt.Errorf("notify.retry_base_delay must be positive")
	case c.Notify.HTTPTimeout <= 0:
		return fmt.Errorf("notify.http_timeout must be positive")
	case c.Notify.RetrySweepInterval <= 0:
		return fmt.Errorf("notify.retry_sweep_interval must be positive")
	case c.Notify.MaxCSMAttempts <= 0:
		return fmt.Errorf("notify.max_csm_attempts must be positive")
	case c.Notify.ServiceCacheTTL <= 0:
		return fmt.Errorf("notify.service_cache_ttl must be positive")
	case c.Notify.StateCheckInterval <= 0:
		return fmt.Errorf("notify.state_check_interval must be positive")
	case c.Notify.CSMRetryBaseDelay <= 0:
		return fmt.Errorf("notify.csm_retry_base_delay must be positive")
	case c.Notify.CSMRetryMultiplier <= 1:
		return fmt.Errorf("notify.csm_retry_multiplier must be greater than 1")
	case c.Notify.CSMRetryMaxDelay.Duration() < c.Notify.CSMRetryBaseDelay.Duration():
		return fmt.Errorf("notify.csm_retry_max_delay must be at least csm_retry_base_delay")
	case c.Notify.ChatFallbackDelay < 0:
		return fmt.Errorf("notify.chat_fallback_delay must not be negative")
	case c.Notify.DeliveryConcurrency <= 0:
		return fmt.Errorf("notify.delivery_concurrency must be positive")
	case c.Notify.MuteRuleRefreshInterval <= 0:
		return fmt.Errorf("notify.mute_rule_refresh_interval must be positive")
	case c.Notify.MuteRuleMaxStaleness < c.Notify.MuteRuleRefreshInterval:
		return fmt.Errorf("notify.mute_rule_max_staleness must be at least mute_rule_refresh_interval")
	case c.Server.ShutdownGrace <= 0:
		return fmt.Errorf("server.shutdown_grace must be positive")
	case c.Engine.DedupWindow <= 0:
		return fmt.Errorf("engine.dedup_window must be positive")
	case c.Retention.Interval <= 0:
		return fmt.Errorf("retention.interval must be positive")
	case c.Retention.Alerts < c.Engine.DedupWindow:
		return fmt.Errorf("retention.alerts must be at least engine.dedup_window")
	case c.Retention.Incidents < c.Engine.DedupWindow:
		return fmt.Errorf("retention.incidents must be at least engine.dedup_window")
	}
	return nil
}
