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

package db

import (
	"context"
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
)

// twoRepoConfig returns a minimal AppConfig with two distinct repos, for
// SyncConfigToDB fixtures.
func twoRepoConfig() *config.AppConfig {
	return &config.AppConfig{
		Repos: []config.RepoEntry{
			{Owner: "wso2-enterprise", Name: "repo-a", GithubProjectID: "PVT_a", ProjectTitle: "Project A", IssueQuery: `label:"Origin/CS"`},
			{Owner: "wso2-enterprise", Name: "repo-b", GithubProjectID: "PVT_b", ProjectTitle: "Project B", IssueQuery: `label:"Origin/CS"`},
		},
	}
}

// TestSyncConfigToDBCreatesProjectsAndRepos verifies a first sync creates one
// enabled project and repository row per configured repo.
func TestSyncConfigToDBCreatesProjectsAndRepos(t *testing.T) {
	pool := testPool(t)
	truncateAll(t, pool)
	ctx := context.Background()

	summary, err := SyncConfigToDB(ctx, pool, twoRepoConfig())
	if err != nil {
		t.Fatalf("SyncConfigToDB: %v", err)
	}
	if summary.ActiveRepos != 2 {
		t.Errorf("expected 2 active repos, got %d", summary.ActiveRepos)
	}
	if summary.DisabledRepos != 0 {
		t.Errorf("expected 0 disabled repos on first sync, got %d", summary.DisabledRepos)
	}

	var repoCount, projectCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM repositories WHERE enabled = true`).Scan(&repoCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE enabled = true`).Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if repoCount != 2 {
		t.Errorf("expected 2 enabled repositories, got %d", repoCount)
	}
	if projectCount != 2 {
		t.Errorf("expected 2 enabled projects, got %d", projectCount)
	}
}

// TestSyncConfigToDBIsIdempotent verifies running the same sync twice
// leaves exactly one repository row per repo, not a duplicate.
func TestSyncConfigToDBIsIdempotent(t *testing.T) {
	pool := testPool(t)
	truncateAll(t, pool)
	ctx := context.Background()
	cfg := twoRepoConfig()

	if _, err := SyncConfigToDB(ctx, pool, cfg); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if _, err := SyncConfigToDB(ctx, pool, cfg); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	var repoCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM repositories`).Scan(&repoCount); err != nil {
		t.Fatal(err)
	}
	if repoCount != 2 {
		t.Errorf("expected exactly 2 repository rows after two syncs, got %d", repoCount)
	}
}

// TestSyncConfigToDBDisablesRemovedRepos verifies a repo dropped from config
// gets disabled (not deleted) on the next sync, preserving its history.
func TestSyncConfigToDBDisablesRemovedRepos(t *testing.T) {
	pool := testPool(t)
	truncateAll(t, pool)
	ctx := context.Background()
	cfg := twoRepoConfig()

	if _, err := SyncConfigToDB(ctx, pool, cfg); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Drop repo-b from the config; it should be disabled, not deleted.
	shrunk := &config.AppConfig{Repos: cfg.Repos[:1]}
	summary, err := SyncConfigToDB(ctx, pool, shrunk)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if summary.DisabledRepos != 1 {
		t.Errorf("expected 1 disabled repo, got %d", summary.DisabledRepos)
	}

	var enabled bool
	if err := pool.QueryRow(ctx, `SELECT enabled FROM repositories WHERE owner = 'wso2-enterprise' AND name = 'repo-b'`).Scan(&enabled); err != nil {
		t.Fatalf("repo-b row should still exist: %v", err)
	}
	if enabled {
		t.Error("expected repo-b to be disabled, not enabled")
	}

	var stillExists int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM repositories`).Scan(&stillExists); err != nil {
		t.Fatal(err)
	}
	if stillExists != 2 {
		t.Errorf("expected repo-b's row to still exist (disabled, not deleted), got %d total rows", stillExists)
	}
}

// TestSyncConfigToDBNeverTouchesLastSyncedAt verifies a config re-sync
// leaves an existing repo's last_synced_at watermark untouched.
func TestSyncConfigToDBNeverTouchesLastSyncedAt(t *testing.T) {
	pool := testPool(t)
	truncateAll(t, pool)
	ctx := context.Background()
	cfg := twoRepoConfig()

	if _, err := SyncConfigToDB(ctx, pool, cfg); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE repositories SET last_synced_at = now() WHERE owner = 'wso2-enterprise' AND name = 'repo-a'`); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncConfigToDB(ctx, pool, cfg); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	var lastSyncedAt *string
	if err := pool.QueryRow(ctx, `SELECT last_synced_at::text FROM repositories WHERE owner = 'wso2-enterprise' AND name = 'repo-a'`).Scan(&lastSyncedAt); err != nil {
		t.Fatal(err)
	}
	if lastSyncedAt == nil {
		t.Error("expected last_synced_at to remain set after a config-sync run")
	}
}
