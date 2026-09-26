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
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// rawServer mirrors Server but with every field optional (nil = absent from
// the YAML), so a default is applied only when the key is missing entirely —
// an explicit value, even an invalid one like 0, must reach Validate rather
// than being silently replaced.
type rawServer struct {
	Port                     *int `yaml:"port"`
	ReadHeaderTimeoutSeconds *int `yaml:"readHeaderTimeoutSeconds"`
	ReadTimeoutSeconds       *int `yaml:"readTimeoutSeconds"`
	WriteTimeoutSeconds      *int `yaml:"writeTimeoutSeconds"`
	IdleTimeoutSeconds       *int `yaml:"idleTimeoutSeconds"`
	ShutdownTimeoutSeconds   *int `yaml:"shutdownTimeoutSeconds"`
	MaxHeaderBytes           *int `yaml:"maxHeaderBytes"`
}

func (r rawServer) resolve(d Server) Server {
	s := d
	if r.Port != nil {
		s.Port = *r.Port
	}
	if r.ReadHeaderTimeoutSeconds != nil {
		s.ReadHeaderTimeoutSeconds = *r.ReadHeaderTimeoutSeconds
	}
	if r.ReadTimeoutSeconds != nil {
		s.ReadTimeoutSeconds = *r.ReadTimeoutSeconds
	}
	if r.WriteTimeoutSeconds != nil {
		s.WriteTimeoutSeconds = *r.WriteTimeoutSeconds
	}
	if r.IdleTimeoutSeconds != nil {
		s.IdleTimeoutSeconds = *r.IdleTimeoutSeconds
	}
	if r.ShutdownTimeoutSeconds != nil {
		s.ShutdownTimeoutSeconds = *r.ShutdownTimeoutSeconds
	}
	if r.MaxHeaderBytes != nil {
		s.MaxHeaderBytes = *r.MaxHeaderBytes
	}
	return s
}

// rawCacheEntry mirrors CacheEntry with optional fields.
type rawCacheEntry struct {
	TTLSeconds *int `yaml:"ttlSeconds"`
	MaxEntries *int `yaml:"maxEntries"`
}

func (r rawCacheEntry) resolve(d CacheEntry) CacheEntry {
	c := d
	if r.TTLSeconds != nil {
		c.TTLSeconds = *r.TTLSeconds
	}
	if r.MaxEntries != nil {
		c.MaxEntries = *r.MaxEntries
	}
	return c
}

// rawCache mirrors Cache with optional per-entry fields.
type rawCache struct {
	Overview   rawCacheEntry `yaml:"overview"`
	Timeseries rawCacheEntry `yaml:"timeseries"`
}

func (r rawCache) resolve(d Cache) Cache {
	return Cache{
		Overview:   r.Overview.resolve(d.Overview),
		Timeseries: r.Timeseries.resolve(d.Timeseries),
	}
}

// rawGitHub mirrors GitHub with optional fields.
type rawGitHub struct {
	RequestTimeoutSeconds   *int `yaml:"requestTimeoutSeconds"`
	MaxRetries              *int `yaml:"maxRetries"`
	RetryBackoffUnitSeconds *int `yaml:"retryBackoffUnitSeconds"`
	RetryAfterCapSeconds    *int `yaml:"retryAfterCapSeconds"`
	SearchPageDelayMs       *int `yaml:"searchPageDelayMs"`
	DetailPageDelayMs       *int `yaml:"detailPageDelayMs"`
}

func (r rawGitHub) resolve(d GitHub) GitHub {
	g := d
	if r.RequestTimeoutSeconds != nil {
		g.RequestTimeoutSeconds = *r.RequestTimeoutSeconds
	}
	if r.MaxRetries != nil {
		g.MaxRetries = *r.MaxRetries
	}
	if r.RetryBackoffUnitSeconds != nil {
		g.RetryBackoffUnitSeconds = *r.RetryBackoffUnitSeconds
	}
	if r.RetryAfterCapSeconds != nil {
		g.RetryAfterCapSeconds = *r.RetryAfterCapSeconds
	}
	if r.SearchPageDelayMs != nil {
		g.SearchPageDelayMs = *r.SearchPageDelayMs
	}
	if r.DetailPageDelayMs != nil {
		g.DetailPageDelayMs = *r.DetailPageDelayMs
	}
	return g
}

// rawJobs mirrors Jobs with optional fields.
type rawJobs struct {
	SyncRunDeadlineMinutes    *int `yaml:"syncRunDeadlineMinutes"`
	LockReleaseTimeoutSeconds *int `yaml:"lockReleaseTimeoutSeconds"`
	RecomputePageSize         *int `yaml:"recomputePageSize"`
}

func (r rawJobs) resolve(d Jobs) Jobs {
	j := d
	if r.SyncRunDeadlineMinutes != nil {
		j.SyncRunDeadlineMinutes = *r.SyncRunDeadlineMinutes
	}
	if r.LockReleaseTimeoutSeconds != nil {
		j.LockReleaseTimeoutSeconds = *r.LockReleaseTimeoutSeconds
	}
	if r.RecomputePageSize != nil {
		j.RecomputePageSize = *r.RecomputePageSize
	}
	return j
}

// rawSeed mirrors Seed with optional fields.
type rawSeed struct {
	InterIssueDelayMs *int `yaml:"interIssueDelayMs"`
}

func (r rawSeed) resolve(d Seed) Seed {
	s := d
	if r.InterIssueDelayMs != nil {
		s.InterIssueDelayMs = *r.InterIssueDelayMs
	}
	return s
}

// rawAPI mirrors API with optional fields.
type rawAPI struct {
	IssuesDefaultLimit     *int `yaml:"issuesDefaultLimit"`
	IssuesMaxLimit         *int `yaml:"issuesMaxLimit"`
	TimeseriesDefaultDays  *int `yaml:"timeseriesDefaultDays"`
	TimeseriesMinDays      *int `yaml:"timeseriesMinDays"`
	TimeseriesMaxDays      *int `yaml:"timeseriesMaxDays"`
	PriorityParamMaxLength *int `yaml:"priorityParamMaxLength"`
	StatusParamMaxLength   *int `yaml:"statusParamMaxLength"`
	AbtTeamParamMaxLength  *int `yaml:"abtTeamParamMaxLength"`
	FilterParamMaxValues   *int `yaml:"filterParamMaxValues"`
}

func (r rawAPI) resolve(d API) API {
	a := d
	if r.IssuesDefaultLimit != nil {
		a.IssuesDefaultLimit = *r.IssuesDefaultLimit
	}
	if r.IssuesMaxLimit != nil {
		a.IssuesMaxLimit = *r.IssuesMaxLimit
	}
	if r.TimeseriesDefaultDays != nil {
		a.TimeseriesDefaultDays = *r.TimeseriesDefaultDays
	}
	if r.TimeseriesMinDays != nil {
		a.TimeseriesMinDays = *r.TimeseriesMinDays
	}
	if r.TimeseriesMaxDays != nil {
		a.TimeseriesMaxDays = *r.TimeseriesMaxDays
	}
	if r.PriorityParamMaxLength != nil {
		a.PriorityParamMaxLength = *r.PriorityParamMaxLength
	}
	if r.StatusParamMaxLength != nil {
		a.StatusParamMaxLength = *r.StatusParamMaxLength
	}
	if r.AbtTeamParamMaxLength != nil {
		a.AbtTeamParamMaxLength = *r.AbtTeamParamMaxLength
	}
	if r.FilterParamMaxValues != nil {
		a.FilterParamMaxValues = *r.FilterParamMaxValues
	}
	return a
}

// rawReadiness mirrors Readiness with optional fields.
type rawReadiness struct {
	TimeoutSeconds                 *int  `yaml:"timeoutSeconds"`
	CacheTTLSeconds                *int  `yaml:"cacheTTLSeconds"`
	FailOnPoolSaturation           *bool `yaml:"failOnPoolSaturation"`
	PoolSaturationThresholdPercent *int  `yaml:"poolSaturationThresholdPercent"`
	DrainGracePeriodSeconds        *int  `yaml:"drainGracePeriodSeconds"`
}

func (r rawReadiness) resolve(d Readiness) Readiness {
	rd := d
	if r.TimeoutSeconds != nil {
		rd.TimeoutSeconds = *r.TimeoutSeconds
	}
	if r.CacheTTLSeconds != nil {
		rd.CacheTTLSeconds = *r.CacheTTLSeconds
	}
	if r.FailOnPoolSaturation != nil {
		rd.FailOnPoolSaturation = *r.FailOnPoolSaturation
	}
	if r.PoolSaturationThresholdPercent != nil {
		rd.PoolSaturationThresholdPercent = *r.PoolSaturationThresholdPercent
	}
	if r.DrainGracePeriodSeconds != nil {
		rd.DrainGracePeriodSeconds = *r.DrainGracePeriodSeconds
	}
	return rd
}

// resolveSecurityHeaders overlays raw onto a copy of defaults: a key present
// in raw either overrides that default's value or adds a new header
// alongside the defaults; an absent (nil or empty) raw map leaves every
// default untouched. Unlike every other resolve() method in this file, there
// is no bare "absent vs explicit zero" ambiguity to preserve here — an
// arbitrary string-keyed map has no fixed set of legal keys, so "merge onto
// defaults" is the only sensible resolution rule.
//
// Two raw keys that canonicalize to the same header name (e.g.
// "X-Frame-Options" and "x-frame-options" both present) are rejected rather
// than merged: Go map iteration order over raw would otherwise pick one of
// them nondeterministically, so two replicas loading the same file could
// emit different headers.
func resolveSecurityHeaders(raw, defaults SecurityHeaders) (SecurityHeaders, error) {
	merged := make(SecurityHeaders, len(defaults)+len(raw))
	for k, v := range defaults {
		merged[http.CanonicalHeaderKey(k)] = v
	}
	rawKeys := make(map[string]string, len(raw))
	for k, v := range raw {
		canonical := http.CanonicalHeaderKey(k)
		if other, ok := rawKeys[canonical]; ok {
			return nil, fmt.Errorf("securityHeaders: %q and %q both canonicalize to %q — keep only one", other, k, canonical)
		}
		rawKeys[canonical] = k
		merged[canonical] = v
	}
	return merged, nil
}

// rawConfig is the direct YAML unmarshal target; every section stays raw so
// missing-vs-zero can be told apart before defaults are resolved. Database is
// unmarshaled straight into Database itself: its fields are already pointers
// (nil = "leave pgx's own default alone"), so no separate raw shape is
// needed.
type rawConfig struct {
	Server          rawServer       `yaml:"server"`
	Database        Database        `yaml:"database"`
	Cache           rawCache        `yaml:"cache"`
	GitHub          rawGitHub       `yaml:"github"`
	Jobs            rawJobs         `yaml:"jobs"`
	Seed            rawSeed         `yaml:"seed"`
	API             rawAPI          `yaml:"api"`
	SecurityHeaders SecurityHeaders `yaml:"securityHeaders"`
	Readiness       rawReadiness    `yaml:"readiness"`
}

func (r rawConfig) resolve() (Config, error) {
	d := Default()
	securityHeaders, err := resolveSecurityHeaders(r.SecurityHeaders, d.SecurityHeaders)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Server:          r.Server.resolve(d.Server),
		Database:        r.Database,
		Cache:           r.Cache.resolve(d.Cache),
		GitHub:          r.GitHub.resolve(d.GitHub),
		Jobs:            r.Jobs.resolve(d.Jobs),
		Seed:            r.Seed.resolve(d.Seed),
		API:             r.API.resolve(d.API),
		SecurityHeaders: securityHeaders,
		Readiness:       r.Readiness.resolve(d.Readiness),
	}, nil
}

// ResolvedPath returns the path Load will read from: APP_CONFIG_PATH when
// set, otherwise the repo-default config/app-config.yaml. Exported so
// cmd/server can log which file is live without duplicating the resolution
// logic.
func ResolvedPath() string {
	return configPath()
}

// configPath resolves APP_CONFIG_PATH (absolute path recommended) or falls
// back to <cwd>/config/app-config.yaml — the repo-default, committed file.
func configPath() string {
	if fromEnv := strings.TrimSpace(os.Getenv("APP_CONFIG_PATH")); fromEnv != "" {
		abs, err := filepath.Abs(fromEnv)
		if err != nil {
			return fromEnv
		}
		return abs
	}
	return filepath.Join("config", "app-config.yaml")
}

// Load reads, parses, and validates the app config. An explicitly set
// APP_CONFIG_PATH that cannot be read is an error — an operator's explicit
// intent must not silently degrade. The default path being absent, with
// APP_CONFIG_PATH unset, is tolerated: Load logs a warning and returns
// Default(). A present-but-unparseable or invalid file is always an error,
// regardless of which path produced it.
func Load() (Config, error) {
	path := configPath()
	explicit := strings.TrimSpace(os.Getenv("APP_CONFIG_PATH")) != ""

	raw, err := os.ReadFile(path) // #nosec G304 -- path comes from APP_CONFIG_PATH or the fixed repo-relative default
	if err != nil {
		if !explicit && errors.Is(err, os.ErrNotExist) {
			slog.Warn("app-config.yaml not found; using built-in defaults", "path", path)
			return Default(), nil
		}
		return Config{}, fmt.Errorf("app config not found at %s — set APP_CONFIG_PATH or restore the file: %w", path, err)
	}

	var parsed rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return Config{}, fmt.Errorf("invalid app config at %s: %w", path, err)
	}

	cfg, err := parsed.resolve()
	if err != nil {
		return Config{}, fmt.Errorf("invalid app config at %s: %w", path, err)
	}
	if err := Validate(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid app config at %s: %w", path, err)
	}
	return cfg, nil
}
