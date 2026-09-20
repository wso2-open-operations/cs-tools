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

// Package appconfig loads and validates config/app-config.yaml — operational
// tuning for server networking, the DB pool, in-process caches, GitHub client
// pacing, background jobs, and API limits. Domain configuration (tracked
// repos, status taxonomy, SLA budgets) lives in internal/config instead and
// is not duplicated here. The result is loaded once at process start and
// treated as immutable for the process lifetime.
package appconfig

// SecurityHeaders maps an HTTP response header name to the value the
// SecurityHeaders middleware (internal/middleware/headers.go) sets on every
// response. Keys are merged onto Default()'s built-in set in load.go's
// resolve(): a key present in app-config.yaml overrides that default's value
// or adds a new header; an absent key leaves the default untouched. This is
// the whole point of making the set config-driven — adding a header later is
// a YAML edit, not a code change.
type SecurityHeaders map[string]string

// Config is the fully resolved contents of app-config.yaml: every key either
// read from the file or filled in from Default.
type Config struct {
	Server          Server
	Database        Database
	Cache           Cache
	GitHub          GitHub
	Jobs            Jobs
	Seed            Seed
	API             API
	SecurityHeaders SecurityHeaders
	Readiness       Readiness
}

// Server holds the HTTP server's networking knobs.
type Server struct {
	Port                     int
	ReadHeaderTimeoutSeconds int
	ReadTimeoutSeconds       int
	WriteTimeoutSeconds      int
	IdleTimeoutSeconds       int
	ShutdownTimeoutSeconds   int
	MaxHeaderBytes           int
}

// Database holds optional pgxpool tuning. Every field is a pointer: nil
// leaves pgx's own default (or whatever DATABASE_URL itself encodes)
// untouched, since pgx's MaxConns default (max(4, runtime.NumCPU())) cannot
// be reproduced by any fixed literal.
type Database struct {
	MaxConns                 *int32 `yaml:"maxConns"`
	MinConns                 *int32 `yaml:"minConns"`
	MaxConnLifetimeMinutes   *int   `yaml:"maxConnLifetimeMinutes"`
	MaxConnIdleTimeMinutes   *int   `yaml:"maxConnIdleTimeMinutes"`
	HealthCheckPeriodSeconds *int   `yaml:"healthCheckPeriodSeconds"`
	ConnectTimeoutSeconds    *int   `yaml:"connectTimeoutSeconds"`
}

// CacheEntry is one in-process TTL cache's size and lifetime.
type CacheEntry struct {
	TTLSeconds int
	MaxEntries int
}

// Cache holds the three in-process TTL caches the handlers keep.
type Cache struct {
	Overview   CacheEntry
	Timeseries CacheEntry
	Titles     CacheEntry
}

// GitHub holds the GraphQL client's timeouts, retry policy, and pacing.
type GitHub struct {
	RequestTimeoutSeconds       int
	TitlesRequestTimeoutSeconds int
	MaxRetries                  int
	RetryBackoffUnitSeconds     int
	RetryAfterCapSeconds        int
	SearchPageDelayMs           int
	DetailPageDelayMs           int
	TitlesBatchSize             int
}

// Jobs holds the background recompute/sync job knobs.
type Jobs struct {
	SyncRunDeadlineMinutes    int
	LockReleaseTimeoutSeconds int
	RecomputePageSize         int
}

// Seed holds cmd/seed's own pacing knob.
type Seed struct {
	InterIssueDelayMs int
}

// API holds request-validation limits mirrored in openapi.yaml.
type API struct {
	IssuesDefaultLimit     int
	IssuesMaxLimit         int
	TitlesMaxIDs           int
	TitlesMaxBodyBytes     int
	TimeseriesDefaultDays  int
	TimeseriesMinDays      int
	TimeseriesMaxDays      int
	PriorityParamMaxLength int
	StatusParamMaxLength   int
}

// Readiness holds GET /readyz's tuning: the DB ping deadline, how long a
// computed result is reused, and whether pool saturation alone fails the
// probe. See internal/handler/health.go for how these are consumed.
type Readiness struct {
	TimeoutSeconds                 int
	CacheTTLSeconds                int
	FailOnPoolSaturation           bool
	PoolSaturationThresholdPercent int
	DrainGracePeriodSeconds        int
}

// Default returns the built-in default Config: one value per field, each
// equal to what the code hardcoded before app-config.yaml existed. Database
// is left entirely nil, since its pointer fields have no fixed-literal
// default to fall back to (nil means "let the pgxpool driver default apply").
func Default() Config {
	return Config{
		Server: Server{
			Port:                     8080,
			ReadHeaderTimeoutSeconds: 10,
			ReadTimeoutSeconds:       30,
			WriteTimeoutSeconds:      30,
			IdleTimeoutSeconds:       60,
			ShutdownTimeoutSeconds:   15,
			MaxHeaderBytes:           1048576,
		},
		Database: Database{},
		Cache: Cache{
			Overview:   CacheEntry{TTLSeconds: 30, MaxEntries: 100},
			Timeseries: CacheEntry{TTLSeconds: 60, MaxEntries: 100},
			Titles:     CacheEntry{TTLSeconds: 900, MaxEntries: 5000},
		},
		GitHub: GitHub{
			RequestTimeoutSeconds:       60,
			TitlesRequestTimeoutSeconds: 15,
			MaxRetries:                  3,
			RetryBackoffUnitSeconds:     2,
			RetryAfterCapSeconds:        60,
			SearchPageDelayMs:           250,
			DetailPageDelayMs:           200,
			TitlesBatchSize:             100,
		},
		Jobs: Jobs{
			SyncRunDeadlineMinutes:    15,
			LockReleaseTimeoutSeconds: 5,
			RecomputePageSize:         200,
		},
		Seed: Seed{
			InterIssueDelayMs: 150,
		},
		API: API{
			IssuesDefaultLimit:     200,
			IssuesMaxLimit:         500,
			TitlesMaxIDs:           200,
			TitlesMaxBodyBytes:     65536,
			TimeseriesDefaultDays:  30,
			TimeseriesMinDays:      7,
			TimeseriesMaxDays:      365,
			PriorityParamMaxLength: 50,
			StatusParamMaxLength:   50,
		},
		SecurityHeaders: SecurityHeaders{
			"Content-Security-Policy":           "default-src 'none'; frame-ancestors 'none'",
			"X-Content-Type-Options":            "nosniff",
			"X-Frame-Options":                   "DENY",
			"Referrer-Policy":                   "no-referrer",
			"X-Permitted-Cross-Domain-Policies": "none",
			"Cache-Control":                     "no-store",
			"Strict-Transport-Security":         "max-age=31536000; includeSubDomains",
		},
		Readiness: Readiness{
			TimeoutSeconds:                 2,
			CacheTTLSeconds:                1,
			FailOnPoolSaturation:           false,
			PoolSaturationThresholdPercent: 100,
			DrainGracePeriodSeconds:        0,
		},
	}
}
