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

// DB-backed against the docker-composed Postgres (see testPool in
// ingest_test.go).
package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
)

// TestIngestIssueBreachedEverStaysStickyAcrossPriorityDowngrade is an
// ingest-level regression: pct_consumed is not monotonic (a priority
// downgrade can widen the budget and drop pct back under 1.0), so
// breached_ever must be sticky — OR'd against its prior value, never
// recomputed from scratch — or a real historical breach silently
// disappears from the projection the instant the priority changes.
func TestIngestIssueBreachedEverStaysStickyAcrossPriorityDowngrade(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	appCfg := &config.AppConfig{
		Taxonomy: config.Taxonomy{
			Statuses: []config.StatusEntry{
				{Name: "In Progress", Category: config.CategoryProductSide, AccruesSla: true},
			},
			Aliases: []config.AliasEntry{},
		},
		Budgets: []config.BudgetEntry{
			{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1},
			{Priority: "High(P2)", BudgetHours: 48, Coverage: config.Coverage24x7, Rank: 2},
		},
		Settings: config.Settings{AtRiskThreshold: 0.75, UnknownStatusPolicy: config.UnknownStatusPause},
	}
	runtime := BuildRuntimeConfig(appCfg)

	var projectID, repositoryID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_breach_sticky_test", "Breach Sticky Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-repo-breach-sticky", `label:"Origin/CS"`, projectID).Scan(&repositoryID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM issue_sla WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issue_status_events WHERE issue_id IN (SELECT id FROM issues WHERE repository_id = $1)`, repositoryID)
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repositoryID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})

	const itemCreated = "2026-01-01T00:00:00.000Z"
	const statusUpdated = "2026-01-01T00:00:00.000Z"

	pairFor := func(priorityLabel string) Pair {
		return Pair{
			Node: github.IssueNode{
				Number: 1, State: "OPEN", URL: "https://github.com/test-owner/test-repo-breach-sticky/issues/1",
				CreatedAt: itemCreated, UpdatedAt: itemCreated,
				Labels: []string{"Priority/" + priorityLabel},
			},
			Detail: github.IssueDetail{
				Number: 1,
				ProjectStatuses: []github.ProjectStatus{
					{ProjectID: "PVT_breach_sticky_test", Status: strp("In Progress"), StatusUpdatedAt: strp(statusUpdated), ItemCreatedAt: strp(itemCreated)},
				},
			},
		}
	}
	ictx := Context{
		RepositoryID: repositoryID, SlaProjectID: projectID,
		Repo:    RepoRef{Owner: "test-owner", Name: "test-repo-breach-sticky", GithubProjectID: "PVT_breach_sticky_test"},
		Runtime: runtime, Source: "github",
	}

	// First ingest: Critical(P1) (24h budget), 30h consumed -> pct=1.25,
	// VIOLATED -> breached_ever must become true.
	now1, _ := time.Parse(time.RFC3339, "2026-01-02T06:00:00.000Z") // itemCreated + 30h
	ictx.Now = now1
	result, err := IngestIssue(ctx, pool, pairFor("Critical(P1)"), ictx)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if result.SlaState != sla.Violated {
		t.Fatalf("expected first ingest to report VIOLATED, got %s", result.SlaState)
	}

	var breachedEver bool
	if err := pool.QueryRow(ctx, `SELECT breached_ever FROM issue_sla WHERE issue_id = $1`, result.IssueID).Scan(&breachedEver); err != nil {
		t.Fatalf("query breached_ever after first ingest: %v", err)
	}
	if !breachedEver {
		t.Fatalf("expected breached_ever=true after first ingest")
	}

	// Second ingest: same issue downgraded to High(P2) (48h budget) -> pct
	// drops to 31/48 ≈ 0.65 (no longer VIOLATED) -> breached_ever must stay
	// true.
	ictx.Now = now1.Add(time.Hour)
	result2, err := IngestIssue(ctx, pool, pairFor("High(P2)"), ictx)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if result2.SlaState == sla.Violated {
		t.Fatalf("expected second ingest to no longer report VIOLATED after downgrade, got %s", result2.SlaState)
	}

	if err := pool.QueryRow(ctx, `SELECT breached_ever FROM issue_sla WHERE issue_id = $1`, result.IssueID).Scan(&breachedEver); err != nil {
		t.Fatalf("query breached_ever after downgrade: %v", err)
	}
	if !breachedEver {
		t.Errorf("expected breached_ever to stay true (sticky) after a priority downgrade dropped pct below 1.0")
	}
}
