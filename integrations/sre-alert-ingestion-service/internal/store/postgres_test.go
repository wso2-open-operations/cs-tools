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

// This file exercises PostgresStore against a real Postgres database when
// one is available, rather than mocking database/sql — the SQL itself
// (upsert semantics, NULL handling, the partial index) is exactly what
// would go untested by an interface mock. It is guarded by
// SRE_ALERT_TEST_DATABASE_URL and skips cleanly when that isn't set, so
// `go test ./...` works with no external dependency in an environment
// without Postgres available. internal/worker's tests cover the
// retry/escalation *logic* independently via a hand-rolled mock of the
// store.Store interface, and do not depend on this test running.
package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/store"
)

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SRE_ALERT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SRE_ALERT_TEST_DATABASE_URL not set; skipping PostgresStore integration test (see this file's doc comment)")
	}
	return dsn
}

// applyMigration runs every migrations/*.up.sql file in order against dsn
// directly (not through PostgresStore, which deliberately doesn't run
// migrations itself) so the test starts from a known, current schema.
func applyMigration(t *testing.T, dsn string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db for migration: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`DROP TABLE IF EXISTS alert_buffer`); err != nil {
		t.Fatalf("drop existing table: %v", err)
	}
	if _, err := db.Exec(`DROP SEQUENCE IF EXISTS alert_number_seq`); err != nil {
		t.Fatalf("drop existing sequence: %v", err)
	}

	ups := []string{
		"0001_create_alert_buffer.up.sql",
		"0002_add_alert_number.up.sql",
	}
	for _, name := range ups {
		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, name)) // #nosec G304 -- fixed, repo-relative test fixture path
		if err != nil {
			t.Fatalf("read migration file %s: %v", name, err)
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func TestPostgresStore_EnqueueAndLifecycle(t *testing.T) {
	dsn := testDSN(t)
	applyMigration(t, dsn)

	s, err := store.NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	id := "11111111-1111-4111-8111-111111111111"
	alertNumber, err := s.Enqueue(ctx, id, func(alertNumber string) ([]byte, error) {
		return []byte(`{"source":"test"}`), nil
	})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if alertNumber == "" {
		t.Fatal("Enqueue() returned an empty alertNumber")
	}
	if !strings.HasPrefix(alertNumber, "ALT") {
		t.Errorf("alertNumber = %q, want it to start with ALT", alertNumber)
	}

	batch, err := s.PendingBatch(ctx, 10)
	if err != nil {
		t.Fatalf("PendingBatch() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("PendingBatch() returned %d rows, want 1", len(batch))
	}
	got := batch[0]
	if got.ID != id {
		t.Errorf("row ID = %q, want %q", got.ID, id)
	}
	if got.AlertNumber != alertNumber {
		t.Errorf("row AlertNumber = %q, want %q (the value Enqueue returned)", got.AlertNumber, alertNumber)
	}
	if got.Status != store.StatusPending {
		t.Errorf("row Status = %q, want %q", got.Status, store.StatusPending)
	}
	if got.RetryCount != 0 {
		t.Errorf("row RetryCount = %d, want 0", got.RetryCount)
	}
	if got.LastAttemptAt != nil {
		t.Errorf("row LastAttemptAt = %v, want nil (never attempted)", got.LastAttemptAt)
	}
	// Compared as parsed JSON, not raw bytes: Postgres's JSONB column
	// reformats on round-trip (e.g. adds a space after ":"), which is
	// correct, expected behavior, not something this test should fail on.
	var gotPayload, wantPayload map[string]any
	if err := json.Unmarshal(got.Payload, &gotPayload); err != nil {
		t.Fatalf("row Payload is not valid JSON: %v; raw: %s", err, got.Payload)
	}
	if err := json.Unmarshal([]byte(`{"source":"test"}`), &wantPayload); err != nil {
		t.Fatalf("test fixture is not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(gotPayload, wantPayload) {
		t.Errorf("row Payload = %s, want (parsed) %v", got.Payload, wantPayload)
	}

	// One failed-but-retryable attempt: retry_count increments, row stays pending.
	if err := s.MarkAttemptFailed(ctx, id, "csm-integration-service returned 401"); err != nil {
		t.Fatalf("MarkAttemptFailed() error = %v", err)
	}
	batch, err = s.PendingBatch(ctx, 10)
	if err != nil {
		t.Fatalf("PendingBatch() after MarkAttemptFailed error = %v", err)
	}
	if len(batch) != 1 || batch[0].RetryCount != 1 {
		t.Fatalf("after MarkAttemptFailed, batch = %+v, want 1 row with RetryCount=1", batch)
	}
	if batch[0].LastAttemptAt == nil {
		t.Error("LastAttemptAt is nil after MarkAttemptFailed, want set")
	}
	if batch[0].LastError != "csm-integration-service returned 401" {
		t.Errorf("LastError = %q, want the recorded error", batch[0].LastError)
	}

	// Delivered: row leaves the pending batch.
	if err := s.MarkDelivered(ctx, id, "inc-123"); err != nil {
		t.Fatalf("MarkDelivered() error = %v", err)
	}
	batch, err = s.PendingBatch(ctx, 10)
	if err != nil {
		t.Fatalf("PendingBatch() after MarkDelivered error = %v", err)
	}
	if len(batch) != 0 {
		t.Fatalf("PendingBatch() after MarkDelivered = %d rows, want 0 (delivered rows aren't pending)", len(batch))
	}
}

func TestPostgresStore_MarkEscalatedAndMarkFailedLeavePendingSet(t *testing.T) {
	dsn := testDSN(t)
	applyMigration(t, dsn)

	s, err := store.NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	buildPayload := func(alertNumber string) ([]byte, error) { return []byte(`{}`), nil }

	escalatedID := "22222222-2222-4222-8222-222222222222"
	if _, err := s.Enqueue(ctx, escalatedID, buildPayload); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := s.MarkEscalated(ctx, escalatedID, "retry budget exhausted"); err != nil {
		t.Fatalf("MarkEscalated() error = %v", err)
	}

	failedID := "33333333-3333-4333-8333-333333333333"
	if _, err := s.Enqueue(ctx, failedID, buildPayload); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := s.MarkFailed(ctx, failedID, "upstream returned 400: invalid payload"); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	batch, err := s.PendingBatch(ctx, 10)
	if err != nil {
		t.Fatalf("PendingBatch() error = %v", err)
	}
	if len(batch) != 0 {
		t.Fatalf("PendingBatch() = %d rows, want 0 (escalated/failed rows aren't pending)", len(batch))
	}
}

// TestPostgresStore_AlertNumbersAreSequentialAndUnique pins the exact format
// (see internal/store.PostgresStore.Enqueue's "ALT" + 7-digit format) and
// that alert_number_seq actually enforces uniqueness across concurrent
// Enqueue calls -- the UNIQUE constraint added by
// migrations/0002_add_alert_number.up.sql is what CreateAlertIncidentMapping
// and the dedup tag both depend on never colliding.
func TestPostgresStore_AlertNumbersAreSequentialAndUnique(t *testing.T) {
	dsn := testDSN(t)
	applyMigration(t, dsn)

	s, err := store.NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	buildPayload := func(alertNumber string) ([]byte, error) { return []byte(`{}`), nil }

	const n = 20
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("aaaaaaaa-aaaa-4aaa-8aaa-%012d", i)
		alertNumber, err := s.Enqueue(ctx, id, buildPayload)
		if err != nil {
			t.Fatalf("Enqueue() [%d] error = %v", i, err)
		}
		if matched, merr := regexp.MatchString(`^ALT\d{7}$`, alertNumber); merr != nil || !matched {
			t.Errorf("alertNumber = %q, want it to match ^ALT\\d{7}$", alertNumber)
		}
		if seen[alertNumber] {
			t.Fatalf("alertNumber %q was generated more than once across %d Enqueue calls", alertNumber, n)
		}
		seen[alertNumber] = true
	}
}
