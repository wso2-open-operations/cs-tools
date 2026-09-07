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
	"fmt"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConfigSyncSummary reports what SyncConfigToDB did.
type ConfigSyncSummary struct {
	ActiveRepos   int
	DisabledRepos int
}

// SyncConfigToDB upserts projects/repositories from cfg (SPEC §8.4, port of
// config-sync.ts). Idempotent and safe to run on every boot: these rows are
// a derived cache for referential integrity only — nothing edits them
// directly. Rows for repos no longer present in the file are disabled, never
// deleted, so their FK children (issues, events, snapshots) keep their
// history; last_synced_at is never touched here — only the seed/sync
// pipeline (which actually fetches data) owns that column.
func SyncConfigToDB(ctx context.Context, pool *pgxpool.Pool, cfg *config.AppConfig) (ConfigSyncSummary, error) {
	projectIDByGh := make(map[string]int32, len(cfg.Repos))

	for _, r := range cfg.Repos {
		if _, ok := projectIDByGh[r.GithubProjectID]; ok {
			continue
		}
		var projectID int32
		err := pool.QueryRow(ctx, `
			INSERT INTO projects (github_project_id, title, enabled)
			VALUES ($1, $2, true)
			ON CONFLICT (github_project_id)
			DO UPDATE SET title = $2, enabled = true, updated_at = now()
			RETURNING id
		`, r.GithubProjectID, r.ProjectTitle).Scan(&projectID)
		if err != nil {
			return ConfigSyncSummary{}, fmt.Errorf("db: upsert project %s: %w", r.GithubProjectID, err)
		}
		projectIDByGh[r.GithubProjectID] = projectID
	}

	configuredRepoKeys := make(map[string]bool, len(cfg.Repos))
	configuredProjectIDs := make(map[string]bool, len(cfg.Repos))
	for _, r := range cfg.Repos {
		projectID := projectIDByGh[r.GithubProjectID]
		htmlURL := fmt.Sprintf("https://github.com/%s/%s", r.Owner, r.Name)
		_, err := pool.Exec(ctx, `
			INSERT INTO repositories (owner, name, issue_query, html_url, sla_project_id, enabled)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (owner, name)
			DO UPDATE SET issue_query = $3, html_url = $4, sla_project_id = $5, enabled = true, updated_at = now()
		`, r.Owner, r.Name, r.IssueQuery, htmlURL, projectID)
		if err != nil {
			return ConfigSyncSummary{}, fmt.Errorf("db: upsert repository %s/%s: %w", r.Owner, r.Name, err)
		}
		configuredRepoKeys[r.Owner+"/"+r.Name] = true
		configuredProjectIDs[r.GithubProjectID] = true
	}

	disabledRepos, err := disableMissing(ctx, pool, "repositories", "owner || '/' || name", configuredRepoKeys)
	if err != nil {
		return ConfigSyncSummary{}, err
	}
	if _, err := disableMissing(ctx, pool, "projects", "github_project_id", configuredProjectIDs); err != nil {
		return ConfigSyncSummary{}, err
	}

	return ConfigSyncSummary{ActiveRepos: len(cfg.Repos), DisabledRepos: disabledRepos}, nil
}

// disableMissing sets enabled=false on every currently-enabled row of table
// whose keyExpr value is not in keep, and reports how many rows changed.
func disableMissing(ctx context.Context, pool *pgxpool.Pool, table, keyExpr string, keep map[string]bool) (int, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id, %s FROM %s WHERE enabled = true`, keyExpr, table))
	if err != nil {
		return 0, fmt.Errorf("db: list enabled %s: %w", table, err)
	}
	var toDisable []int32
	for rows.Next() {
		var id int32
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			rows.Close()
			return 0, fmt.Errorf("db: scan %s: %w", table, err)
		}
		if !keep[key] {
			toDisable = append(toDisable, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("db: iterate %s: %w", table, err)
	}
	if len(toDisable) == 0 {
		return 0, nil
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s SET enabled = false, updated_at = now() WHERE id = ANY($1)`, table), toDisable)
	if err != nil {
		return 0, fmt.Errorf("db: disable %s: %w", table, err)
	}
	return len(toDisable), nil
}
