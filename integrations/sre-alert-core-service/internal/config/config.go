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
	Cassandra CassandraConfig `toml:"cassandra"`
	Notify    NotifyConfig    `toml:"notify"`
	Server    ServerConfig    `toml:"server"`
	Lease     LeaseConfig     `toml:"lease"`
	Engine    EngineConfig    `toml:"engine"`
}

// EngineConfig tunes the dedup engine's fixed duplicate-folding window.
type EngineConfig struct {
	// DedupWindow bounds how long an incident absorbs duplicates before starting fresh, even if CSM still reports it open.
	DedupWindow Duration `toml:"dedup_window"`
}

// PollConfig tunes the alert poller's cadence, concurrency, and per-cycle alert id limits.
type PollConfig struct {
	// Interval is the backstop cadence; POST /alertz drives real-time pickup, so this only bounds how long a dropped ping goes unnoticed.
	Interval Duration `toml:"interval"`
	// Concurrency is fingerprint-sharded worker count; same-fingerprint alerts stay serialized on one worker.
	Concurrency int `toml:"concurrency"`
	// ReadConcurrency bounds parallel alert-row reads at cycle start, independent of the handling worker count.
	ReadConcurrency int `toml:"read_concurrency"`
	// MaxWindow caps how many alert ids a single poll cycle processes at once, bounding memory usage under large alert bursts.
	MaxWindow int `toml:"max_window"`
	// GapTimeout is how long an alert id may stay missing, measured from when the current leader
	// first saw it missing, before it's skipped and logged loudly. Every missing id in the window
	// ages at once, so a whole gap is skipped together after one GapTimeout.
	GapTimeout Duration `toml:"gap_timeout"`
}

// LeaseConfig tunes the lease electing one active poller across replicas, so standbys never double-process alerts.
type LeaseConfig struct {
	// TTL bounds how long a dead leader's work can go unresumed by a standby.
	TTL Duration `toml:"ttl"`
	// RenewInterval must stay well under TTL so one missed renewal never drops leadership.
	RenewInterval Duration `toml:"renew_interval"`
}

// CassandraConfig tunes startup connection retry attempts, backoff delay, connect timeout, and the per-query timeout used for every Cassandra call.
type CassandraConfig struct {
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
	// RetrySweepInterval retries outstanding CSM/Chat notifications; outage recovery, independent of poll.interval.
	RetrySweepInterval Duration `toml:"retry_sweep_interval"`
	// MaxCSMAttempts caps failed attempts before marking permanently failed, so bad payloads don't grow RetrySweep's scan cost forever.
	MaxCSMAttempts int `toml:"max_csm_attempts"`
	// ServiceCacheTTL bounds how long a label->CMDB-service-id resolution is reused before a fresh live /services/search call.
	ServiceCacheTTL Duration `toml:"service_cache_ttl"`
	// StateCheckInterval throttles how often a confirmed incident's status is re-checked against CSM;
	// without it, a flapping alert costs one CSM search per duplicate during a storm.
	StateCheckInterval Duration `toml:"state_check_interval"`
	// CSMRetryBaseDelay is the wait before the first RetrySweep-driven CSM retry after a failed attempt.
	CSMRetryBaseDelay Duration `toml:"csm_retry_base_delay"`
	// CSMRetryMultiplier grows the wait between successive CSM retries during a prolonged outage.
	CSMRetryMultiplier float64 `toml:"csm_retry_multiplier"`
	// CSMRetryMaxDelay caps how long the exponential CSM retry wait can grow to.
	CSMRetryMaxDelay Duration `toml:"csm_retry_max_delay"`
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

// defaults holds every tunable's production value, matching config.toml. Used as-is when
// config.toml is absent, and as the base a present config.toml overrides field by field.
func defaults() Config {
	return Config{
		Poll: PollConfig{
			Interval:        Duration(60 * time.Second),
			Concurrency:     128,
			ReadConcurrency: 64,
			MaxWindow:       2000,
			GapTimeout:      Duration(10 * time.Minute),
		},
		Lease: LeaseConfig{
			TTL:           Duration(15 * time.Second),
			RenewInterval: Duration(5 * time.Second),
		},
		Cassandra: CassandraConfig{
			ConnectMaxAttempts: 5,
			ConnectBaseDelay:   Duration(2 * time.Second),
			ConnectTimeout:     Duration(10 * time.Second),
			QueryTimeout:       Duration(10 * time.Second),
		},
		Notify: NotifyConfig{
			MaxAttempts:        3,
			RetryBaseDelay:     Duration(200 * time.Millisecond),
			HTTPTimeout:        Duration(10 * time.Second),
			RetrySweepInterval: Duration(30 * time.Second),
			MaxCSMAttempts:     13,
			ServiceCacheTTL:    Duration(15 * time.Minute),
			StateCheckInterval: Duration(1 * time.Minute),
			CSMRetryBaseDelay:  Duration(30 * time.Second),
			CSMRetryMultiplier: 3,
			CSMRetryMaxDelay:   Duration(time.Hour),
		},
		Server: ServerConfig{
			ShutdownGrace: Duration(15 * time.Second),
		},
		Engine: EngineConfig{
			DedupWindow: Duration(5 * time.Minute),
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
	case c.Poll.ReadConcurrency <= 0:
		return fmt.Errorf("poll.read_concurrency must be positive")
	case c.Poll.MaxWindow <= 0:
		return fmt.Errorf("poll.max_window must be positive")
	case c.Poll.GapTimeout <= 0:
		return fmt.Errorf("poll.gap_timeout must be positive")
	case c.Lease.TTL <= 0:
		return fmt.Errorf("lease.ttl must be positive")
	case c.Lease.RenewInterval <= 0:
		return fmt.Errorf("lease.renew_interval must be positive")
	case c.Lease.RenewInterval >= c.Lease.TTL:
		return fmt.Errorf("lease.renew_interval must be less than lease.ttl")
	case c.Cassandra.ConnectMaxAttempts <= 0:
		return fmt.Errorf("cassandra.connect_max_attempts must be positive")
	case c.Cassandra.ConnectBaseDelay <= 0:
		return fmt.Errorf("cassandra.connect_base_delay must be positive")
	case c.Cassandra.ConnectTimeout <= 0:
		return fmt.Errorf("cassandra.connect_timeout must be positive")
	case c.Cassandra.QueryTimeout <= 0:
		return fmt.Errorf("cassandra.query_timeout must be positive")
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
	case c.Server.ShutdownGrace <= 0:
		return fmt.Errorf("server.shutdown_grace must be positive")
	case c.Engine.DedupWindow <= 0:
		return fmt.Errorf("engine.dedup_window must be positive")
	}
	return nil
}
