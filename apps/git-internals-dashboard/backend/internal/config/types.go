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

// Package config loads and validates config/sla-config.yaml — the single
// source of truth for repos, status taxonomy, and SLA budgets. The result is
// loaded once at process start and treated as immutable for the process
// lifetime.
package config

// StatusCategory classifies a taxonomy status by who owns it while an issue
// sits there.
type StatusCategory string

const (
	CategoryProductSide StatusCategory = "PRODUCT_SIDE"
	CategoryCSSide      StatusCategory = "CS_SIDE"
	CategoryOther       StatusCategory = "OTHER"
	CategoryTransient   StatusCategory = "TRANSIENT"
)

// valid reports whether c is one of the four recognized categories.
func (c StatusCategory) valid() bool {
	switch c {
	case CategoryProductSide, CategoryCSSide, CategoryOther, CategoryTransient:
		return true
	default:
		return false
	}
}

// SlaCoverage names an SLA budget's coverage window.
type SlaCoverage string

const (
	Coverage24x7    SlaCoverage = "24x7"
	Coverage12x5Ist SlaCoverage = "12x5_ist"
)

// valid reports whether c is one of the two recognized coverage windows.
func (c SlaCoverage) valid() bool {
	switch c {
	case Coverage24x7, Coverage12x5Ist:
		return true
	default:
		return false
	}
}

// RepoEntry is one tracked GitHub repository and its linked Projects (v2) board.
type RepoEntry struct {
	Owner           string `yaml:"owner"`
	Name            string `yaml:"name"`
	GithubProjectID string `yaml:"githubProjectId"`
	ProjectTitle    string `yaml:"projectTitle"`
	IssueQuery      string `yaml:"issueQuery"`
}

// StatusEntry is one row of the board-status taxonomy. Name may be "" — the
// transient empty status a board item briefly holds when removed from the
// board.
type StatusEntry struct {
	Name       string         `yaml:"name"`
	Category   StatusCategory `yaml:"category"`
	AccruesSla bool           `yaml:"accruesSla"`
	IsTerminal bool           `yaml:"isTerminal"`
	SortOrder  int            `yaml:"sortOrder"`
}

// AliasEntry maps a raw board status string to its canonical taxonomy name.
type AliasEntry struct {
	Alias     string `yaml:"alias"`
	Canonical string `yaml:"canonical"`
}

// BudgetEntry is the SLA budget for one priority tier.
type BudgetEntry struct {
	Priority    string      `yaml:"priority"`
	BudgetHours float64     `yaml:"budgetHours"`
	Coverage    SlaCoverage `yaml:"coverage"`
	Rank        int         `yaml:"rank"`
}

// Taxonomy is the full board-status vocabulary: every known status plus the
// aliases that normalize raw board strings onto it.
type Taxonomy struct {
	Statuses []StatusEntry `yaml:"statuses"`
	Aliases  []AliasEntry  `yaml:"aliases"`
}

// UnknownStatusPolicy controls how the SLA clock treats a board status
// absent from taxonomy.statuses — a renamed or newly added column the
// config hasn't caught up to yet.
type UnknownStatusPolicy string

const (
	// UnknownStatusPause pauses accrual for an unknown status, same as
	// today's (implicit, undocumented) behavior — safe against
	// over-counting, but silently undercounts if the status is actually
	// product-side.
	UnknownStatusPause UnknownStatusPolicy = "pause"
	// UnknownStatusAccrue treats an unknown status as product-side —
	// safer against silently hiding a stalled issue from breach alerts,
	// at the cost of possibly over-counting until the status is
	// classified.
	UnknownStatusAccrue UnknownStatusPolicy = "accrue"
)

// valid reports whether p is one of the two recognized policies.
func (p UnknownStatusPolicy) valid() bool {
	switch p {
	case UnknownStatusPause, UnknownStatusAccrue:
		return true
	default:
		return false
	}
}

// Settings holds the tunable knobs governing SLA math and job cadence.
// Defaults apply only when the corresponding YAML key is entirely absent
// (see rawSettings in load.go).
type Settings struct {
	AtRiskThreshold          float64
	RecomputeIntervalMinutes int
	SyncOverlapMinutes       int
	SnapshotHourUtc          int
	SeedSnapshotDays         int
	SeedClosedLookbackDays   int
	UnknownStatusPolicy      UnknownStatusPolicy
}

// defaultSettings returns the default value for every Settings field.
func defaultSettings() Settings {
	return Settings{
		AtRiskThreshold:          0.75,
		RecomputeIntervalMinutes: 10,
		SyncOverlapMinutes:       15,
		SnapshotHourUtc:          0,
		SeedSnapshotDays:         90,
		SeedClosedLookbackDays:   90,
		UnknownStatusPolicy:      UnknownStatusPause,
	}
}

// AppConfig is the fully parsed and validated contents of sla-config.yaml.
type AppConfig struct {
	Repos    []RepoEntry
	Taxonomy Taxonomy
	Budgets  []BudgetEntry
	Settings Settings
	// Holidays is a list of ISO 8601 dates ("2026-01-26") excluded from the
	// 12x5_ist coverage window (24x7 budgets are unaffected — a holiday
	// only removes hours from a window that already excludes weekends).
	Holidays []string
}
