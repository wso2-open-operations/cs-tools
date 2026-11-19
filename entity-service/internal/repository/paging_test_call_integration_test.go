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

// This is an integration test: it runs the test-call statements of
// PagingContactRepository against a live PostgreSQL instance -- the cooldown
// shared by a profile number's tests (paging_test_call, migration 0242) and a
// paging-only number's (paging_contact), and which of the two a reported
// result lands on.
//
// It builds a private schema holding "user", paging_contact (the columns of
// migration 0216, without its audit trigger) and paging_test_call (migration
// 0242 itself, run twice) and drops it afterwards, so it needs no real data
// and touches none. Skipped without a DSN:
//
//	PAGING_CONTACT_TEST_DSN=postgres://... go test ./internal/repository/ -run PagingTestCall

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	pagingTestUserA = "7a000000-0000-4000-8000-000000000001"
	pagingTestUserB = "7a000000-0000-4000-8000-000000000002"
)

func pagingTestCallPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PAGING_CONTACT_TEST_DSN")
	if dsn == "" {
		t.Skip("PAGING_CONTACT_TEST_DSN not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("paging_test_call_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect with search_path: %v", err)
	}
	t.Cleanup(pool.Close)

	migration, err := os.ReadFile("../../migrations/0242_paging_test_call.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, ddl := range []string{
		`CREATE TABLE "user" (id UUID PRIMARY KEY, email VARCHAR(255), phone VARCHAR(32),
			name VARCHAR(255), first_name VARCHAR(255), last_name VARCHAR(255), user_name VARCHAR(255))`,
		`CREATE TABLE paging_contact (
			user_id          UUID PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
			phone            VARCHAR(16) NOT NULL,
			set_by           VARCHAR(255) NOT NULL,
			set_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_test_at     TIMESTAMPTZ NULL,
			last_test_status VARCHAR(20) NULL)`,
		string(migration),
		string(migration), // re-runnable
	} {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	for id, phone := range map[string]string{pagingTestUserA: " +94770000001 ", pagingTestUserB: "077 000 0002"} {
		if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, email, phone) VALUES ($1, $2, $3)`, id, id+"@wso2.com", phone); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	return pool
}

func TestPagingTestCallIntegration(t *testing.T) {
	pool := pagingTestCallPool(t)
	ctx := context.Background()
	repo := repository.NewPagingContactRepository(pool)
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	cooldown := 2 * time.Minute

	phones, err := repo.ProfilePhones(ctx, []string{pagingTestUserA, pagingTestUserB})
	if err != nil || len(phones) != 1 || phones[pagingTestUserA] != "+94770000001" {
		t.Fatalf("profile phones %v %v, want only A's, trimmed", phones, err)
	}

	// A profile test, then the cooldown holds both numbers.
	if ok, err := repo.MarkProfileTestPending(ctx, "rota@wso2.com", pagingTestUserA, "+94770000001", t0, t0.Add(-cooldown)); err != nil || !ok {
		t.Fatalf("first profile test: %v %v", ok, err)
	}
	if _, err := repo.Upsert(ctx, "lead@example.com", pagingTestUserA, "+94771234123"); err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(time.Minute)
	if ok, err := repo.MarkProfileTestPending(ctx, "rota@wso2.com", pagingTestUserA, "+94770000001", t1, t1.Add(-cooldown)); err != nil || ok {
		t.Fatalf("profile test within the cooldown: %v %v, want refused", ok, err)
	}
	if _, ok, err := repo.MarkTestPending(ctx, "rota@wso2.com", pagingTestUserA, t1, t1.Add(-cooldown)); err != nil || ok {
		t.Fatalf("paging test within a profile test's cooldown: %v %v, want refused", ok, err)
	}

	// After the cooldown a paging-only test may go ahead; with both pending,
	// the result is the newer one's.
	t2 := t0.Add(3 * time.Minute)
	if _, ok, err := repo.MarkTestPending(ctx, "rota@wso2.com", pagingTestUserA, t2, t2.Add(-cooldown)); err != nil || !ok {
		t.Fatalf("paging test after the cooldown: %v %v", ok, err)
	}
	if found, err := repo.RecordTestResult(ctx, "service:notify", pagingTestUserA, "busy", t2.Add(30*time.Second)); err != nil || !found {
		t.Fatalf("record: %v %v", found, err)
	}
	row, _ := repo.Get(ctx, pagingTestUserA)
	tests, _ := repo.ProfileTests(ctx, []string{pagingTestUserA})
	if row.LastTestStatus == nil || *row.LastTestStatus != "busy" || tests[pagingTestUserA].LastTestStatus != "pending" {
		t.Fatalf("newer paging test: paging %v, profile %q", row.LastTestStatus, tests[pagingTestUserA].LastTestStatus)
	}

	// A newer profile test takes the next result.
	t3 := t0.Add(6 * time.Minute)
	if ok, err := repo.MarkProfileTestPending(ctx, "admin@wso2.com", pagingTestUserA, "+94770000009", t3, t3.Add(-cooldown)); err != nil || !ok {
		t.Fatalf("second profile test: %v %v", ok, err)
	}
	if found, err := repo.RecordTestResult(ctx, "service:notify", pagingTestUserA, "completed", t3.Add(40*time.Second)); err != nil || !found {
		t.Fatalf("record: %v %v", found, err)
	}
	tests, _ = repo.ProfileTests(ctx, []string{pagingTestUserA})
	got := tests[pagingTestUserA]
	if got.LastTestStatus != "completed" || got.Phone != "+94770000009" || !got.LastTestAt.Equal(t3.Add(40*time.Second)) {
		t.Fatalf("profile test %+v", got)
	}
	if row, _ := repo.Get(ctx, pagingTestUserA); *row.LastTestStatus != "busy" {
		t.Fatalf("the paging-only number's result moved: %v", *row.LastTestStatus)
	}

	// A repeated result -- both tests settled -- is recorded nowhere: it must
	// not land on the paging-only number.
	if found, err := repo.RecordTestResult(ctx, "service:notify", pagingTestUserA, "failed", t3.Add(time.Minute)); err != nil || found {
		t.Fatalf("repeated result: %v %v, want not found", found, err)
	}
	if row, _ := repo.Get(ctx, pagingTestUserA); *row.LastTestStatus != "busy" {
		t.Fatalf("a repeated result overwrote the paging-only number: %v", *row.LastTestStatus)
	}
	if tests, _ := repo.ProfileTests(ctx, []string{pagingTestUserA}); tests[pagingTestUserA].LastTestStatus != "completed" {
		t.Fatalf("a repeated result overwrote the profile test: %v", tests[pagingTestUserA].LastTestStatus)
	}

	// Neither number tested: nothing to record.
	if found, err := repo.RecordTestResult(ctx, "service:notify", pagingTestUserB, "busy", t3); err != nil || found {
		t.Fatalf("no test: %v %v, want not found", found, err)
	}
	// A status the table does not know is refused by the database.
	if _, err := pool.Exec(ctx, `UPDATE paging_test_call SET last_test_status = 'answered'`); err == nil {
		t.Fatal("the status check let 'answered' in")
	}
	// The test goes with the user.
	if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, pagingTestUserA); err != nil {
		t.Fatal(err)
	}
	if tests, _ := repo.ProfileTests(ctx, []string{pagingTestUserA}); len(tests) != 0 {
		t.Fatalf("test outlived its user: %v", tests)
	}
}
