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

// Port of v3's src/server/config/schema.test.ts — every case there has a
// direct counterpart here.
package config

import (
	"strings"
	"testing"
)

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
	if cfg.Settings.PossibleThreshold != 0.75 {
		t.Errorf("expected default possibleThreshold=0.75, got %v", cfg.Settings.PossibleThreshold)
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

func TestValidateRejectsBadCoverageValue(t *testing.T) {
	cfg := validFixture()
	cfg.Budgets[0].Coverage = "9x5_utc"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for bad coverage value")
	}
}

func TestValidateRejectsEmptyRepos(t *testing.T) {
	cfg := validFixture()
	cfg.Repos = nil
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for empty repos")
	}
}
