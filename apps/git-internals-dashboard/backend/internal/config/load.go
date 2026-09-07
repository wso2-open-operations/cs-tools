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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// rawSettings mirrors Settings but with every field optional (nil = absent
// from the YAML), so a default is applied only when the key is missing
// entirely — an explicit value, even an invalid one like 0, must reach
// Validate rather than being silently replaced (port of zod's
// z.number().default(...), which only substitutes on undefined).
type rawSettings struct {
	PossibleThreshold        *float64 `yaml:"possibleThreshold"`
	RecomputeIntervalMinutes *int     `yaml:"recomputeIntervalMinutes"`
	SyncOverlapMinutes       *int     `yaml:"syncOverlapMinutes"`
	SnapshotHourUtc          *int     `yaml:"snapshotHourUtc"`
	SeedSnapshotDays         *int     `yaml:"seedSnapshotDays"`
	SeedClosedLookbackDays   *int     `yaml:"seedClosedLookbackDays"`
}

func (r rawSettings) resolve() Settings {
	s := defaultSettings()
	if r.PossibleThreshold != nil {
		s.PossibleThreshold = *r.PossibleThreshold
	}
	if r.RecomputeIntervalMinutes != nil {
		s.RecomputeIntervalMinutes = *r.RecomputeIntervalMinutes
	}
	if r.SyncOverlapMinutes != nil {
		s.SyncOverlapMinutes = *r.SyncOverlapMinutes
	}
	if r.SnapshotHourUtc != nil {
		s.SnapshotHourUtc = *r.SnapshotHourUtc
	}
	if r.SeedSnapshotDays != nil {
		s.SeedSnapshotDays = *r.SeedSnapshotDays
	}
	if r.SeedClosedLookbackDays != nil {
		s.SeedClosedLookbackDays = *r.SeedClosedLookbackDays
	}
	return s
}

// rawAppConfig is the direct YAML unmarshal target; Settings stays raw so
// missing-vs-zero can be told apart before defaults are resolved.
type rawAppConfig struct {
	Repos    []RepoEntry   `yaml:"repos"`
	Taxonomy Taxonomy      `yaml:"taxonomy"`
	Budgets  []BudgetEntry `yaml:"budgets"`
	Settings rawSettings   `yaml:"settings"`
}

// configPath resolves SLA_CONFIG_PATH (absolute path recommended) or falls
// back to <cwd>/config/sla-config.yaml — the repo-default, committed file.
func configPath() string {
	if fromEnv := strings.TrimSpace(os.Getenv("SLA_CONFIG_PATH")); fromEnv != "" {
		abs, err := filepath.Abs(fromEnv)
		if err != nil {
			return fromEnv
		}
		return abs
	}
	return filepath.Join("config", "sla-config.yaml")
}

// Load reads, parses, and validates the SLA config (SPEC §5.1). Every call
// re-reads the file from disk — callers that want a single immutable
// snapshot for the process lifetime (the normal case) call this once at
// boot and pass the result down, rather than relying on any hidden
// memoization.
func Load() (*AppConfig, error) {
	path := configPath()
	raw, err := os.ReadFile(path) // #nosec G304 -- path comes from SLA_CONFIG_PATH or the fixed repo-relative default
	if err != nil {
		return nil, fmt.Errorf("SLA config not found at %s — set SLA_CONFIG_PATH or restore the file: %w", path, err)
	}

	var parsed rawAppConfig
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid SLA config at %s: %w", path, err)
	}

	cfg := &AppConfig{
		Repos:    parsed.Repos,
		Taxonomy: parsed.Taxonomy,
		Budgets:  parsed.Budgets,
		Settings: parsed.Settings.resolve(),
	}
	if cfg.Taxonomy.Aliases == nil {
		cfg.Taxonomy.Aliases = []AliasEntry{}
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid SLA config at %s: %w", path, err)
	}
	return cfg, nil
}
