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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Retention of the audit log (migration 0295), against a real Postgres 18: deleting
// expired partitions and catch-all rows, the partition health check, and lock
// timeouts. Skipped without ENTITY_TEST_DATABASE_URL.

package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// retentionPartitions makes two monthly partitions around the 18-month cutoff,
// each holding one row: "expired" ended exactly 18 months ago, "recent" ended
// 17 months ago. Both are dropped again after the test.
func retentionPartitions(t *testing.T, pool *pgxpool.Pool) (expired, recent string) {
	t.Helper()
	expired, recent = "zz_audit_retention_expired", "zz_audit_retention_recent"
	drop := func() {
		for _, name := range []string{expired, recent} {
			_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+name)
		}
	}
	drop()
	t.Cleanup(drop)

	cutoff := `(date_trunc('month', now() AT TIME ZONE 'UTC') - make_interval(months => 18)) AT TIME ZONE 'UTC'`
	for _, p := range []struct{ name, from, to string }{
		{expired, cutoff + " - interval '1 month'", cutoff},
		{recent, cutoff, cutoff + " + interval '1 month'"},
	} {
		var from, to string
		if err := pool.QueryRow(context.Background(), "SELECT ("+p.from+")::text, ("+p.to+")::text").Scan(&from, &to); err != nil {
			t.Fatalf("bounds for %s: %v", p.name, err)
		}
		auditExec(t, pool, "CREATE TABLE "+p.name+" PARTITION OF csm_audit_log FOR VALUES FROM ('"+from+"') TO ('"+to+"')")
		auditExec(t, pool, `INSERT INTO csm_audit_log (created_on, schema_name, table_name, record_id, field_name, action, created_by, db_role, transaction_id)
		    VALUES ($1::timestamptz, 'public', 'zz_retention', '1', 'f', 'UPDATE', 'tester', 'tester', 1)`, from)
	}
	return expired, recent
}

func TestAuditMigration_DeleteExpiredPartitionsDeletesTheExpiredAndKeepsTheRest(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, recent := retentionPartitions(t, pool)

	got, err := repo.DeleteExpiredPartitions(context.Background(), 18)
	if err != nil {
		t.Fatalf("DeleteExpiredPartitions: %v", err)
	}

	names := strings.Join(got, " ")
	if !strings.Contains(names, expired) {
		t.Errorf("retired %q, want it to include %s (ended exactly 18 months ago)", names, expired)
	}
	if strings.Contains(names, recent) {
		t.Errorf("retired %q, which includes %s (ended 17 months ago)", names, recent)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", expired); n != 0 {
		t.Errorf("%s still exists: it must be dropped, not only detached", expired)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_retention'"); n != 1 {
		t.Errorf("zz_retention rows = %d, want 1: only the row in the recent partition survives", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = 'csm_audit_log_default'"); n != 1 {
		t.Error("the catch-all partition must never be deleted")
	}
}

func TestAuditMigration_DeleteExpiredPartitionsIsRepeatable(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, _ := retentionPartitions(t, pool)

	if _, err := repo.DeleteExpiredPartitions(context.Background(), 18); err != nil {
		t.Fatalf("first run: %v", err)
	}
	again, err := repo.DeleteExpiredPartitions(context.Background(), 18)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if strings.Contains(strings.Join(again, " "), expired) {
		t.Errorf("second run retired %s again", expired)
	}
}

// A longer retention keeps what a shorter one deletes.
func TestAuditMigration_DeleteExpiredPartitionsFollowsTheRetentionPeriod(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, _ := retentionPartitions(t, pool)

	if got, err := repo.DeleteExpiredPartitions(context.Background(), 24); err != nil {
		t.Fatalf("DeleteExpiredPartitions(24): %v", err)
	} else if strings.Contains(strings.Join(got, " "), expired) {
		t.Errorf("a 24-month retention deleted %s, which ended 18 months ago", expired)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", expired); n != 1 {
		t.Errorf("%s was deleted by a 24-month retention", expired)
	}
}

// Zero or negative puts the cutoff in the future and would delete every
// partition, so the function refuses it.
func TestAuditMigration_DeleteExpiredPartitionsRefusesAnUnsafeRetention(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	expired, recent := retentionPartitions(t, pool)

	for _, months := range []int{0, -1} {
		if _, err := repo.DeleteExpiredPartitions(context.Background(), months); err == nil {
			t.Errorf("DeleteExpiredPartitions(%d) succeeded, want an error", months)
		}
	}
	for _, name := range []string{expired, recent} {
		if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = $1", name); n != 1 {
			t.Errorf("%s was deleted by a refused call", name)
		}
	}
}

// resetDefaultPartition swaps the catch-all partition for an empty one, the only
// way to clear it (the log is append-only). The purge tests leave rows in it,
// and a row there is a health problem for every other test.
func resetDefaultPartition(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	reset := func() {
		ctx := context.Background()
		for _, sql := range []string{
			"ALTER TABLE csm_audit_log DETACH PARTITION csm_audit_log_default",
			"DROP TABLE csm_audit_log_default",
			"CREATE TABLE csm_audit_log_default PARTITION OF csm_audit_log DEFAULT",
			"CREATE TRIGGER csm_audit_log_no_truncate BEFORE TRUNCATE ON csm_audit_log_default FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change()",
		} {
			if _, err := pool.Exec(ctx, sql); err != nil {
				t.Fatalf("reset default partition: %s: %v", sql, err)
			}
		}
	}
	reset()
	t.Cleanup(reset)
}

// defaultRow puts one row into the catch-all partition, created monthsAgo months
// ago. 40 and 3 months back have no partition in a database built by the
// migration, so both land in the catch-all.
func defaultRow(t *testing.T, pool *pgxpool.Pool, recordID string, monthsAgo int) {
	t.Helper()
	auditExec(t, pool, `INSERT INTO csm_audit_log (created_on, schema_name, table_name, record_id, field_name, action, created_by, db_role, transaction_id)
	    VALUES (now() - make_interval(months => $2), 'public', 'zz_default_purge', $1, 'f', 'UPDATE', 'tester', 'tester', 1)`, recordID, monthsAgo)
}

func TestAuditMigration_DeleteExpiredDefaultRowsDeletesOnlyTheExpiredRows(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old-1", 40)
	defaultRow(t, pool, "old-2", 19)
	defaultRow(t, pool, "young", 3)

	deleted, err := repo.DeleteExpiredDefaultRows(context.Background(), 18)
	if err != nil {
		t.Fatalf("DeleteExpiredDefaultRows: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (the rows 40 and 19 months old)", deleted)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge'"); n != 1 {
		t.Errorf("zz_default_purge rows = %d, want 1", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge' AND record_id = 'young'"); n != 1 {
		t.Error("the row from 3 months ago must survive")
	}
}

// The catch-all is replaced by a new table, so it must still be the attached
// default partition, keep its TRUNCATE guard, and stay append-only.
func TestAuditMigration_DeleteExpiredDefaultRowsLeavesTheCatchAllIntact(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old", 40)
	defaultRow(t, pool, "young", 3)

	if _, err := repo.DeleteExpiredDefaultRows(context.Background(), 18); err != nil {
		t.Fatalf("DeleteExpiredDefaultRows: %v", err)
	}

	if n := auditCount(t, pool, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
	    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname = 'csm_audit_log_default'
	      AND pg_get_expr(c.relpartbound, c.oid) = 'DEFAULT'`); n != 1 {
		t.Errorf("csm_audit_log_default is not the attached default partition (%d)", n)
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM pg_class WHERE relname = 'csm_audit_log_default_purge'"); n != 0 {
		t.Error("the table swapped out was not dropped")
	}
	if n := auditCount(t, pool, `SELECT count(*) FROM pg_trigger
	    WHERE tgrelid = 'csm_audit_log_default'::regclass AND tgname = 'csm_audit_log_no_truncate'`); n != 1 {
		t.Error("the new catch-all has lost its TRUNCATE guard")
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM csm_audit_log_default"); err == nil {
		t.Error("a DELETE on the new catch-all succeeded: the log must stay append-only")
	}
}

// With nothing past retention it changes nothing and swaps nothing.
func TestAuditMigration_DeleteExpiredDefaultRowsWithNothingExpiredChangesNothing(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "young", 3)
	var before uint32
	if err := pool.QueryRow(context.Background(), "SELECT 'csm_audit_log_default'::regclass::oid").Scan(&before); err != nil {
		t.Fatalf("oid: %v", err)
	}

	deleted, err := repo.DeleteExpiredDefaultRows(context.Background(), 18)
	if err != nil {
		t.Fatalf("DeleteExpiredDefaultRows: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
	var after uint32
	if err := pool.QueryRow(context.Background(), "SELECT 'csm_audit_log_default'::regclass::oid").Scan(&after); err != nil {
		t.Fatalf("oid: %v", err)
	}
	if before != after {
		t.Error("the catch-all table was replaced although nothing was past retention")
	}
}

func TestAuditMigration_DeleteExpiredDefaultRowsIsRepeatable(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "old", 40)

	if deleted, err := repo.DeleteExpiredDefaultRows(context.Background(), 18); err != nil || deleted != 1 {
		t.Fatalf("first run = (%d, %v), want (1, nil)", deleted, err)
	}
	if deleted, err := repo.DeleteExpiredDefaultRows(context.Background(), 18); err != nil || deleted != 0 {
		t.Errorf("second run = (%d, %v), want (0, nil)", deleted, err)
	}
}

func TestAuditMigration_DeleteExpiredDefaultRowsRefusesAnUnsafeRetention(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	resetDefaultPartition(t, pool)
	defaultRow(t, pool, "young", 3)

	for _, months := range []int{0, -1} {
		if _, err := repo.DeleteExpiredDefaultRows(context.Background(), months); err == nil {
			t.Errorf("DeleteExpiredDefaultRows(%d) succeeded, want an error", months)
		}
	}
	if n := auditCount(t, pool, "SELECT count(*) FROM csm_audit_log WHERE table_name = 'zz_default_purge'"); n != 1 {
		t.Errorf("rows = %d after refused calls, want 1", n)
	}
}

func auditHealth(t *testing.T, repo repository.AuditLogRepository, monthsAhead int) string {
	t.Helper()
	problems, err := repo.PartitionHealth(context.Background(), monthsAhead)
	if err != nil {
		t.Fatalf("PartitionHealth(%d): %v", monthsAhead, err)
	}
	return strings.Join(problems, " | ")
}

// The daily run creates monthsAhead months, so a missed one is reported the day
// it is missed, not once the buffer is down to two.
func TestAuditMigration_HealthReportsAMissedFuturePartition(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	if _, err := repo.CreatePartitions(context.Background(), 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	// However many future months the database holds, asking for that many is
	// healthy and asking for one more is one short of the configured buffer.
	have := auditCount(t, pool, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
	    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname ~ '^csm_audit_log_\d{4}_\d{2}$'
	      AND to_date(substr(c.relname, 15), 'YYYY_MM') > date_trunc('month', now() AT TIME ZONE 'UTC')::date`)
	if got := auditHealth(t, repo, have); got != "" {
		t.Errorf("health with %d months ahead = %q, want healthy", have, got)
	}
	if got := auditHealth(t, repo, have+1); !strings.Contains(got, "missing monthly partition") {
		t.Errorf("health with %d months ahead = %q, want a missing-partition problem", have+1, got)
	}
	// The monitoring call with no arguments keeps its old minimum of 2.
	if n := auditCount(t, pool, "SELECT count(*) FROM audit_partition_health()"); n != 0 {
		t.Errorf("audit_partition_health() = %d problems, want 0", n)
	}
}

// The current month counts too: without its partition every new audit row lands in
// the catch-all.
func TestAuditMigration_HealthReportsAMissingCurrentMonth(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	ctx := context.Background()
	if _, err := repo.CreatePartitions(ctx, 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	var current, bound string
	if err := pool.QueryRow(ctx, `SELECT 'csm_audit_log_' || to_char(now() AT TIME ZONE 'UTC', 'YYYY_MM')`).Scan(&current); err != nil {
		t.Fatalf("name: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pg_get_expr(relpartbound, oid) FROM pg_class WHERE relname = $1`, current).Scan(&bound); err != nil {
		t.Fatalf("bound of %s: %v", current, err)
	}
	t.Cleanup(func() {
		var attached int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname = $1`, current).Scan(&attached)
		if attached == 0 {
			_, _ = pool.Exec(ctx, "ALTER TABLE csm_audit_log ATTACH PARTITION "+current+" "+bound)
		}
	})

	if got := auditHealth(t, repo, 3); got != "" {
		t.Fatalf("health before the test = %q, want healthy", got)
	}
	auditExec(t, pool, "ALTER TABLE csm_audit_log DETACH PARTITION "+current)
	got := auditHealth(t, repo, 3)
	if !strings.Contains(got, strings.TrimPrefix(current, "csm_audit_log_")) {
		t.Errorf("health = %q, want the missing current month %s reported", got, current)
	}
}

// A missing month inside the horizon is reported even when a partition further
// out exists: counting partitions would let the distant one cover for it.
func TestAuditMigration_HealthReportsAMissingMonthDespiteADistantPartition(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	repo := repository.NewAuditLogRepository(repository.NewScoped(pool))
	ctx := context.Background()
	if _, err := repo.CreatePartitions(ctx, 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	var next, bound, from, to string
	if err := pool.QueryRow(ctx, `SELECT
	    'csm_audit_log_' || to_char(date_trunc('month', now() AT TIME ZONE 'UTC') + interval '1 month', 'YYYY_MM'),
	    ((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '40 months') AT TIME ZONE 'UTC')::text,
	    ((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '41 months') AT TIME ZONE 'UTC')::text`).Scan(&next, &from, &to); err != nil {
		t.Fatalf("names: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pg_get_expr(relpartbound, oid) FROM pg_class WHERE relname = $1`, next).Scan(&bound); err != nil {
		t.Fatalf("bound of %s: %v", next, err)
	}

	const distant = "zz_audit_health_distant"
	restore := func() {
		_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS "+distant)
		var attached int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		    WHERE i.inhparent = 'csm_audit_log'::regclass AND c.relname = $1`, next).Scan(&attached)
		if attached == 0 {
			_, _ = pool.Exec(ctx, "ALTER TABLE csm_audit_log ATTACH PARTITION "+next+" "+bound)
		}
	}
	t.Cleanup(restore)
	restore()

	if got := auditHealth(t, repo, 3); got != "" {
		t.Fatalf("health before the test = %q, want healthy", got)
	}
	auditExec(t, pool, "ALTER TABLE csm_audit_log DETACH PARTITION "+next)
	auditExec(t, pool, "CREATE TABLE "+distant+" PARTITION OF csm_audit_log FOR VALUES FROM ('"+from+"') TO ('"+to+"')")

	got := auditHealth(t, repo, 3)
	if !strings.Contains(got, "missing monthly partition") || !strings.Contains(got, strings.TrimPrefix(next, "csm_audit_log_")) {
		t.Errorf("health = %q, want the missing month %s reported although a distant partition exists", got, next)
	}
}

// Deleting waits on the same advisory lock as creating partitions. The lock
// timeout has to cover that wait too: with the timeout set after the lock, a run
// queued behind another one waits as long as the other holds it.
func TestAuditMigration_DeletionGivesUpWhileAnotherRunHoldsTheLock(t *testing.T) {
	pool := newAuditMigrationTestPool(t)
	ctx := context.Background()

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('csm_audit_log_partitions', 0))"); err != nil {
		t.Fatalf("take the lock: %v", err)
	}

	for _, call := range []string{
		"SELECT audit_delete_expired_partitions(18, '300ms')",
		"SELECT audit_delete_expired_default_rows(18, '300ms')",
	} {
		t.Run(call, func(t *testing.T) {
			// Far longer than the lock timeout: a call that ignores it only ends
			// when this deadline cancels it, with a different error.
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			started := time.Now()
			_, err := pool.Exec(callCtx, call)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("%s = %v, want a lock timeout (55P03)", call, err)
			}
			if took := time.Since(started); took > 3*time.Second {
				t.Errorf("%s gave up after %s, want about the 300ms timeout", call, took)
			}
		})
	}
}
