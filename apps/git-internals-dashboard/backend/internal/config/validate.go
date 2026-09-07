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
	"strings"
)

// ValidationError aggregates every rule violation found in one config, so a
// misconfigured deploy sees every problem at once instead of fixing them one
// failed boot at a time (port of schema.ts's superRefine, which collects all
// zod issues rather than failing on the first).
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return "invalid SLA config:\n  " + strings.Join(e.Issues, "\n  ")
}

// Validate checks cfg against every rule schema.ts enforces: required
// fields, valid enum values, numeric ranges, uniqueness, and alias targets.
// Returns nil when cfg is valid.
func Validate(cfg *AppConfig) error {
	var issues []string
	add := func(format string, args ...any) {
		issues = append(issues, fmt.Sprintf(format, args...))
	}

	if len(cfg.Repos) == 0 {
		add("repos: array must contain at least 1 element(s)")
	}
	seenRepo := make(map[string]bool, len(cfg.Repos))
	seenProjectID := make(map[string]bool, len(cfg.Repos))
	for i, r := range cfg.Repos {
		if r.Owner == "" {
			add("repos.%d.owner: must not be empty", i)
		}
		if r.Name == "" {
			add("repos.%d.name: must not be empty", i)
		}
		if r.GithubProjectID == "" {
			add("repos.%d.githubProjectId: must not be empty", i)
		}
		if r.ProjectTitle == "" {
			add("repos.%d.projectTitle: must not be empty", i)
		}
		if r.IssueQuery == "" {
			add("repos.%d.issueQuery: must not be empty", i)
		}
		key := r.Owner + "/" + r.Name
		if seenRepo[key] {
			add("duplicate repo: %s", key)
		}
		seenRepo[key] = true
		if r.GithubProjectID != "" {
			if seenProjectID[r.GithubProjectID] {
				add("duplicate githubProjectId: %s", r.GithubProjectID)
			}
			seenProjectID[r.GithubProjectID] = true
		}
	}

	if len(cfg.Taxonomy.Statuses) == 0 {
		add("taxonomy.statuses: array must contain at least 1 element(s)")
	}
	statusNames := make(map[string]bool, len(cfg.Taxonomy.Statuses))
	seenStatusName := make(map[string]bool, len(cfg.Taxonomy.Statuses))
	for i, s := range cfg.Taxonomy.Statuses {
		if !s.Category.valid() {
			add("taxonomy.statuses.%d.category: invalid enum value %q", i, s.Category)
		}
		if seenStatusName[s.Name] {
			add("duplicate status name: %q", s.Name)
		}
		seenStatusName[s.Name] = true
		statusNames[s.Name] = true
	}

	seenAlias := make(map[string]bool, len(cfg.Taxonomy.Aliases))
	for i, a := range cfg.Taxonomy.Aliases {
		if a.Alias == "" {
			add("taxonomy.aliases.%d.alias: must not be empty", i)
		}
		if a.Canonical == "" {
			add("taxonomy.aliases.%d.canonical: must not be empty", i)
		}
		if seenAlias[a.Alias] {
			add("duplicate alias: %s", a.Alias)
		}
		seenAlias[a.Alias] = true
		if a.Canonical != "" && !statusNames[a.Canonical] {
			add("alias %q maps to unknown canonical %q", a.Alias, a.Canonical)
		}
	}

	seenPriority := make(map[string]bool, len(cfg.Budgets))
	for i, b := range cfg.Budgets {
		if b.Priority == "" {
			add("budgets.%d.priority: must not be empty", i)
		}
		if b.BudgetHours <= 0 {
			add("budgets.%d.budgetHours: must be positive", i)
		}
		if !b.Coverage.valid() {
			add("budgets.%d.coverage: invalid enum value %q", i, b.Coverage)
		}
		if seenPriority[b.Priority] {
			add("duplicate budget priority: %s", b.Priority)
		}
		seenPriority[b.Priority] = true
	}

	if cfg.Settings.PossibleThreshold <= 0 || cfg.Settings.PossibleThreshold >= 1 {
		add("settings.possibleThreshold: must be strictly between 0 and 1")
	}
	if cfg.Settings.RecomputeIntervalMinutes <= 0 {
		add("settings.recomputeIntervalMinutes: must be positive")
	}
	if cfg.Settings.SyncOverlapMinutes < 0 {
		add("settings.syncOverlapMinutes: must not be negative")
	}
	if cfg.Settings.SnapshotHourUtc < 0 || cfg.Settings.SnapshotHourUtc > 23 {
		add("settings.snapshotHourUtc: must be between 0 and 23")
	}
	if cfg.Settings.SeedSnapshotDays <= 0 {
		add("settings.seedSnapshotDays: must be positive")
	}
	if cfg.Settings.SeedClosedLookbackDays <= 0 {
		add("settings.seedClosedLookbackDays: must be positive")
	}

	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}
