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

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A new problem gets ServiceNow's defaults -- impact Low, urgency Low,
// priority Planning (discovery script 63) -- on both create paths, and
// migration 0194 gives them to problems created before. Run with
// ENTITY_TEST_DATABASE_URL; every row it makes is deleted afterwards.
const (
	ppSNProblemID = "47777777-0000-0000-0000-0000000000d1"
	ppOldID       = "47777777-0000-0000-0000-0000000000d2"
	ppSyncedID    = "47777777-0000-0000-0000-0000000000d3"
	ppHighID      = "47777777-0000-0000-0000-0000000000d4"
)

func ppRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) [3]string {
	t.Helper()
	var r [3]string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(priority::text,''), COALESCE(impact::text,''), COALESCE(urgency::text,'')
		FROM problem WHERE id = $1`, id).Scan(&r[0], &r[1], &r[2]); err != nil {
		t.Fatalf("read problem %s: %v", id, err)
	}
	return r
}

func TestProblemPriorityIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	var portalID string
	cleanup := func() {
		for _, id := range []string{ppSNProblemID, ppOldID, ppSyncedID, ppHighID, portalID} {
			if id == "" {
				continue
			}
			if _, err := pool.Exec(context.Background(), `DELETE FROM work_item WHERE id = $1`, id); err != nil {
				t.Errorf("CLEANUP FAILED, delete work_item %s by hand: %v", id, err)
			}
		}
	}
	cleanup()
	t.Cleanup(func() { cleanup(); pool.Close() })
	repo := NewProblemRepository(NewScoped(pool))
	want := [3]string{"PLANNING", "LOW", "LOW"}

	// Postgres-only create.
	created, err := repo.CreateProblem(ctx, domain.CreateProblemRequest{Subject: "priority test (portal)"}, "t@example.com",
		ProblemPriorityFields{Priority: "PLANNING", Impact: "LOW", Urgency: "LOW"})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}
	portalID = *created.ID
	if got := ppRow(t, ctx, pool, portalID); got != want {
		t.Errorf("Postgres-only create: priority/impact/urgency = %v, want %v", got, want)
	}

	// Dual-write create: the priority ServiceNow gave, defaults for the rest.
	if _, err := repo.CreateProblemFromServiceNow(ctx, domain.CreateProblemRequest{Subject: "priority test (sn)"},
		ppSNProblemID, "PRB-PP-0001", "t@example.com", nil, "HIGH"); err != nil {
		t.Fatalf("CreateProblemFromServiceNow: %v", err)
	}
	if got := ppRow(t, ctx, pool, ppSNProblemID); got != [3]string{"HIGH", "LOW", "LOW"} {
		t.Errorf("dual-write create: priority/impact/urgency = %v, want [HIGH LOW LOW]", got)
	}

	// Migration 0194: a problem created before, with no priority, gets the
	// defaults; a synced one with its own priority is left alone.
	for id, prio := range map[string]any{ppOldID: nil, ppSyncedID: "CRITICAL"} {
		if _, err := pool.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES ($1, NOW(), NOW(), 't', 't', $2, 'priority backfill', 'PROBLEM')`, id, "PRB-PP-"+id[len(id)-2:]); err != nil {
			t.Fatalf("seed work_item: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO problem (id, state, priority) VALUES ($1, 'NEW', $2::problem_priority_enum)`, id, prio); err != nil {
			t.Fatalf("seed problem: %v", err)
		}
	}
	// One with impact High / urgency Medium and no priority: derived, High.
	if _, err := pool.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1, NOW(), NOW(), 't', 't', 'PRB-PP-d4', 'priority backfill', 'PROBLEM')`, ppHighID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO problem (id, state, impact, urgency) VALUES ($1, 'NEW', 'HIGH', 'MEDIUM')`, ppHighID); err != nil {
		t.Fatalf("seed problem: %v", err)
	}
	migration, err := os.ReadFile("../../migrations/0194_problem_default_priority.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for i := 0; i < 2; i++ { // twice: re-running it must change nothing
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("run migration 0194 (pass %d): %v", i+1, err)
		}
	}
	if got := ppRow(t, ctx, pool, ppOldID); got != want {
		t.Errorf("backfilled problem: %v, want %v", got, want)
	}
	if got := ppRow(t, ctx, pool, ppHighID); got != [3]string{"HIGH", "HIGH", "MEDIUM"} {
		t.Errorf("backfilled High x Medium problem: %v, want [HIGH HIGH MEDIUM]", got)
	}
	if got := ppRow(t, ctx, pool, ppSyncedID); got != [3]string{"CRITICAL", "", ""} {
		t.Errorf("synced problem with its own priority was changed: %v", got)
	}
}
