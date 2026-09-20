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

// Tests for Validate: required fields, enum values, duplicate/uniqueness
// checks, alias targets, and default application.
package config

import (
	"strings"
	"testing"
)

// validFixture returns a minimal AppConfig that passes Validate as-is, for
// tests to mutate into an invalid shape.
func validFixture() *AppConfig {
	return &AppConfig{
		Repos: []RepoEntry{
			{
				Owner:           "wso2-enterprise",
				Name:            "wso2-iam-internal",
				GithubProjectID: "PVT_kwDOATtEas4ADJ4n",
				ProjectTitle:    "IAM Internals",
				IssueQuery:      `label:"Origin/CS"`,
			},
		},
		Taxonomy: Taxonomy{
			Statuses: []StatusEntry{
				{Name: "Open", Category: CategoryProductSide, AccruesSla: true},
				{Name: "Resolved", Category: CategoryOther, AccruesSla: false, IsTerminal: true},
			},
			Aliases: []AliasEntry{{Alias: "Re-Opened", Canonical: "Open"}},
		},
		Budgets:  []BudgetEntry{{Priority: "Critical(P1)", BudgetHours: 24, Coverage: Coverage24x7, Rank: 1}},
		Settings: defaultSettings(),
	}
}

// hasIssueContaining reports whether err is a *ValidationError with at least
// one issue string containing substr.
func hasIssueContaining(err error, substr string) bool {
	ve, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	for _, issue := range ve.Issues {
		if strings.Contains(issue, substr) {
			return true
		}
	}
	return false
}

// TestValidateAcceptsValidFixtureAndAppliesDefaults verifies a minimal valid
// config passes and every omitted field receives its documented default.
func TestValidateAcceptsValidFixtureAndAppliesDefaults(t *testing.T) {
	cfg := validFixture()
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected valid fixture to pass, got: %v", err)
	}
	if cfg.Taxonomy.Statuses[0].IsTerminal != false {
		t.Errorf("expected default isTerminal=false")
	}
	if cfg.Taxonomy.Statuses[0].SortOrder != 0 {
		t.Errorf("expected default sortOrder=0")
	}
	if cfg.Settings.AtRiskThreshold != 0.75 {
		t.Errorf("expected default atRiskThreshold=0.75, got %v", cfg.Settings.AtRiskThreshold)
	}
	if cfg.Settings.RecomputeIntervalMinutes != 10 {
		t.Errorf("expected default recomputeIntervalMinutes=10, got %v", cfg.Settings.RecomputeIntervalMinutes)
	}
	if cfg.Settings.SyncOverlapMinutes != 15 {
		t.Errorf("expected default syncOverlapMinutes=15, got %v", cfg.Settings.SyncOverlapMinutes)
	}
	if cfg.Settings.SnapshotHourUtc != 0 {
		t.Errorf("expected default snapshotHourUtc=0, got %v", cfg.Settings.SnapshotHourUtc)
	}
	if cfg.Settings.SeedSnapshotDays != 90 {
		t.Errorf("expected default seedSnapshotDays=90, got %v", cfg.Settings.SeedSnapshotDays)
	}
	if cfg.Settings.SeedClosedLookbackDays != 90 {
		t.Errorf("expected default seedClosedLookbackDays=90, got %v", cfg.Settings.SeedClosedLookbackDays)
	}
}

// TestValidateRejectsDuplicateRepoOwnerName verifies two repos with the same
// owner/name fail validation with a "duplicate repo" issue.
func TestValidateRejectsDuplicateRepoOwnerName(t *testing.T) {
	cfg := validFixture()
	cfg.Repos = append(cfg.Repos, cfg.Repos[0])
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !hasIssueContaining(err, "duplicate repo") {
		t.Errorf("expected a 'duplicate repo' issue, got: %v", err)
	}
}

// TestValidateRejectsDuplicateGithubProjectID verifies two repos (even with
// distinct owner/name) sharing one githubProjectId fail validation.
func TestValidateRejectsDuplicateGithubProjectID(t *testing.T) {
	cfg := validFixture()
	dup := cfg.Repos[0]
	dup.Owner = "other-owner"
	dup.Name = "other-name"
	cfg.Repos = append(cfg.Repos, dup)
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !hasIssueContaining(err, "duplicate githubProjectId") {
		t.Errorf("expected a 'duplicate githubProjectId' issue, got: %v", err)
	}
}

// TestValidateRejectsAliasToUnknownCanonical verifies an alias whose
// canonical name isn't a declared status fails validation.
func TestValidateRejectsAliasToUnknownCanonical(t *testing.T) {
	cfg := validFixture()
	cfg.Taxonomy.Aliases = append(cfg.Taxonomy.Aliases, AliasEntry{Alias: "Weird", Canonical: "Nonexistent Status"})
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !hasIssueContaining(err, `unknown canonical "Nonexistent Status"`) {
		t.Errorf("expected an 'unknown canonical' issue, got: %v", err)
	}
}

// TestValidateRejectsBadCoverageValue verifies a budget entry naming a
// coverage window outside SlaCoverage.valid's set fails validation.
func TestValidateRejectsBadCoverageValue(t *testing.T) {
	cfg := validFixture()
	cfg.Budgets[0].Coverage = "9x5_utc"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for bad coverage value")
	}
}

// TestValidateRejectsEmptyRepos verifies a config with no repos at all fails
// validation.
func TestValidateRejectsEmptyRepos(t *testing.T) {
	cfg := validFixture()
	cfg.Repos = nil
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for empty repos")
	}
}

// TestValidateAcceptsWellFormedHolidays verifies a list of valid ISO dates
// passes.
func TestValidateAcceptsWellFormedHolidays(t *testing.T) {
	cfg := validFixture()
	cfg.Holidays = []string{"2026-01-26", "2026-08-15"}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected well-formed holidays to pass, got: %v", err)
	}
}

// TestValidateRejectsMalformedHoliday verifies a holiday string that isn't a
// YYYY-MM-DD date fails validation.
func TestValidateRejectsMalformedHoliday(t *testing.T) {
	cfg := validFixture()
	cfg.Holidays = []string{"26/01/2026"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for malformed holiday date")
	}
	if !hasIssueContaining(err, "invalid date") {
		t.Errorf("expected an 'invalid date' issue, got: %v", err)
	}
}

// TestValidateRejectsDuplicateHoliday verifies the same holiday date listed
// twice fails validation.
func TestValidateRejectsDuplicateHoliday(t *testing.T) {
	cfg := validFixture()
	cfg.Holidays = []string{"2026-01-26", "2026-01-26"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for duplicate holiday")
	}
	if !hasIssueContaining(err, "duplicate holiday") {
		t.Errorf("expected a 'duplicate holiday' issue, got: %v", err)
	}
}

// TestValidateRejectsBadUnknownStatusPolicy verifies an unrecognized
// unknownStatusPolicy value fails validation rather than silently falling
// through to one branch.
func TestValidateRejectsBadUnknownStatusPolicy(t *testing.T) {
	cfg := validFixture()
	cfg.Settings.UnknownStatusPolicy = "ignore"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for bad unknownStatusPolicy")
	}
	if !hasIssueContaining(err, "unknownStatusPolicy") {
		t.Errorf("expected an 'unknownStatusPolicy' issue, got: %v", err)
	}
}
