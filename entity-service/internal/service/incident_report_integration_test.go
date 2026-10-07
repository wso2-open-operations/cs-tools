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

package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// End-to-end against a real database with every migration applied: the 0181
// trigger records the state change, the drainer applies the flow, and the
// outbox row is marked done only when the write commits.
//
// Run with INCIDENT_REPORT_TEST_DSN=postgres://... go test -run IncidentReportIntegration ./internal/service/

const (
	irIncidentID = "47777777-0000-0000-0000-000000000001"
	irUserID     = "47777777-0000-0000-0000-000000000002"
	irGroupID    = "47777777-0000-0000-0000-000000000003"
	irServiceID  = "47777777-0000-0000-0000-000000000004"
)

func incidentReportTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_REPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_REPORT_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func irExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", strings.SplitN(strings.TrimSpace(sql), "\n", 2)[0], err)
	}
}

func irCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	irExec(t, pool, `DELETE FROM work_item WHERE id IN (SELECT id FROM incident_task WHERE incident_id = $1)`, irIncidentID)
	irExec(t, pool, `DELETE FROM work_item WHERE id = $1`, irIncidentID)
	irExec(t, pool, `DELETE FROM event_outbox WHERE entity_type = 'incident' AND entity_id = $1`, irIncidentID)
	irExec(t, pool, `DELETE FROM service WHERE id = $1`, irServiceID)
	irExec(t, pool, `DELETE FROM "group" WHERE id = $1`, irGroupID)
	irExec(t, pool, `DELETE FROM "user" WHERE id = $1`, irUserID)
}

func irSeed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	irCleanup(t, pool)
	t.Cleanup(func() { irCleanup(t, pool) })
	irExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, NOW(), NOW(), 'ir.test')`, irUserID)
	irExec(t, pool, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by) VALUES ($1, NOW(), NOW(), 't', 't')`, irGroupID)
	irExec(t, pool, `INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, NOW(), NOW(), 't', 't', 'IR Test Service', 'SVC-IR-1')`, irServiceID)
	irExec(t, pool, `
		INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, assignment_group_id, assigned_to_id)
		VALUES ($1, '2026-10-02 08:47:41+00', NOW(), 't', 't', 'INC-IR-0001', 'Checkout is down', 'INCIDENT', $2, $3)`,
		irIncidentID, irGroupID, irUserID)
	irExec(t, pool, `INSERT INTO incident (id, state, priority, service_id) VALUES ($1, 'NEW', 'HIGH', $2)`, irIncidentID, irServiceID)
}

func irDrainer(pool *pgxpool.Pool) *IncidentReportDrainer {
	return NewIncidentReportDrainer(
		repository.NewIncidentReportRepository(repository.NewScoped(pool)),
		NewIncidentReportService(),
		time.Second, IncidentReportMaxAttempts)
}

func irPendingCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM event_outbox WHERE entity_type = 'incident' AND entity_id = $1 AND published_on IS NULL`, irIncidentID).Scan(&n); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	return n
}

func TestIncidentReportIntegration_BothFlows(t *testing.T) {
	pool := incidentReportTestPool(t)
	irSeed(t, pool)
	ctx := context.Background()
	d := irDrainer(pool)

	// --- Create Incident Report Task: NEW -> IN_PROGRESS -------------------
	irExec(t, pool, `UPDATE incident SET state = 'IN_PROGRESS' WHERE id = $1`, irIncidentID)
	if n := irPendingCount(t, pool); n != 1 {
		t.Fatalf("trigger recorded %d pending rows, want 1", n)
	}
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	var (
		number, subject, createdBy, priority, state, typ string
		group, assignee, service, incident               string
		active                                           bool
	)
	err := pool.QueryRow(ctx, `
		SELECT wi.number, wi.subject, wi.created_by, wi.assignment_group_id::text, wi.assigned_to_id::text,
		       it.priority::text, it.state::text, it.type::text, it.service_id::text, it.incident_id::text, it.is_active
		FROM incident_task it JOIN work_item wi ON wi.id = it.id
		WHERE it.incident_id = $1`, irIncidentID).
		Scan(&number, &subject, &createdBy, &group, &assignee, &priority, &state, &typ, &service, &incident, &active)
	if err != nil {
		t.Fatalf("read created task: %v", err)
	}
	if !strings.HasPrefix(number, "TASK1") {
		t.Errorf("number = %q, want the TASK series above ServiceNow's range (migration 0201)", number)
	}
	if subject != "[Incident Report] Create the incident report for INC-IR-0001" {
		t.Errorf("subject = %q", subject)
	}
	if group != irGroupID || assignee != irUserID || service != irServiceID || incident != irIncidentID {
		t.Errorf("group/assignee/service/incident = %s/%s/%s/%s", group, assignee, service, incident)
	}
	if priority != "CRITICAL" || typ != "INCIDENT_REPORT" || state != "OPEN" || !active || createdBy != "system" {
		t.Errorf("priority/type/state/active/createdBy = %s/%s/%s/%v/%s", priority, typ, state, active, createdBy)
	}
	if n := irPendingCount(t, pool); n != 0 {
		t.Errorf("%d rows still pending after a successful drain", n)
	}

	// --- Incident Report Generator: IN_PROGRESS -> RESOLVED ----------------
	irExec(t, pool, `UPDATE incident SET state = 'RESOLVED' WHERE id = $1`, irIncidentID)
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	var report string
	if err := pool.QueryRow(ctx, `SELECT incident_report FROM incident WHERE id = $1`, irIncidentID).Scan(&report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	for _, want := range []string{
		"<strong>Incident Number</strong><br />INC-IR-0001",
		"<strong>Incident Severity Level</strong><br />2 - High",
		"<strong>Incident Identification Time</strong><br />2026-10-02 08:47:41<br />",
		"<strong>Next Steps</strong><br />-<br /><br /></p>",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}

	// The report write is itself an incident UPDATE, so the trigger records
	// it. It must be drained as a no-op -- not loop, not create anything.
	if n := irPendingCount(t, pool); n != 1 {
		t.Fatalf("report write recorded %d pending rows, want 1", n)
	}
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if n := irPendingCount(t, pool); n != 0 {
		t.Errorf("report-write row not drained: %d pending", n)
	}
	var tasks int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incident_task WHERE incident_id = $1`, irIncidentID).Scan(&tasks); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if tasks != 1 {
		t.Errorf("tasks = %d, want exactly 1", tasks)
	}
}

// A failed write rolls back with nothing left behind, the row stays pending
// with its error recorded, waits out its backoff, and succeeds once the
// cause is gone.
func TestIncidentReportIntegration_RetryAfterFailure(t *testing.T) {
	pool := incidentReportTestPool(t)
	irSeed(t, pool)
	ctx := context.Background()
	d := irDrainer(pool)

	// Make every incident_task insert fail, as a broken constraint or an
	// outage mid-transaction would.
	irExec(t, pool, `
		CREATE OR REPLACE FUNCTION ir_test_reject() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'ir test: insert rejected'; END $$ LANGUAGE plpgsql`)
	irExec(t, pool, `DROP TRIGGER IF EXISTS ir_test_reject ON incident_task`)
	irExec(t, pool, `CREATE TRIGGER ir_test_reject BEFORE INSERT ON incident_task FOR EACH ROW EXECUTE FUNCTION ir_test_reject()`)
	t.Cleanup(func() {
		irExec(t, pool, `DROP TRIGGER IF EXISTS ir_test_reject ON incident_task`)
		irExec(t, pool, `DROP FUNCTION IF EXISTS ir_test_reject()`)
	})

	irExec(t, pool, `UPDATE incident SET state = 'IN_PROGRESS' WHERE id = $1`, irIncidentID)
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}

	var (
		attempts int
		lastErr  string
	)
	if err := pool.QueryRow(ctx, `
		SELECT attempts, last_error FROM event_outbox
		WHERE entity_type = 'incident' AND entity_id = $1 AND published_on IS NULL`, irIncidentID).Scan(&attempts, &lastErr); err != nil {
		t.Fatalf("failed row should still be pending: %v", err)
	}
	if attempts != 1 || !strings.Contains(lastErr, "insert rejected") {
		t.Errorf("attempts=%d lastErr=%q", attempts, lastErr)
	}
	// The work_item half of the insert must have rolled back with the rest.
	var orphans int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM work_item WHERE subject LIKE '%INC-IR-0001%' AND type = 'INCIDENT_TASK'`).Scan(&orphans); err != nil {
		t.Fatalf("count orphans: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d orphan work_item rows left by a failed insert", orphans)
	}

	// Within the backoff window the row is not retried.
	ids, err := repository.NewIncidentReportRepository(repository.NewScoped(pool)).PendingChanges(ctx, 100)
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("row retried inside its backoff window: %v", ids)
	}

	// Cause fixed and backoff elapsed: the retry applies it.
	irExec(t, pool, `DROP TRIGGER ir_test_reject ON incident_task`)
	irExec(t, pool, `UPDATE event_outbox SET last_attempt_on = NOW() - INTERVAL '1 minute' WHERE entity_type = 'incident' AND entity_id = $1`, irIncidentID)
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if n := irPendingCount(t, pool); n != 0 {
		t.Errorf("row still pending after a successful retry")
	}
	var tasks int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incident_task WHERE incident_id = $1`, irIncidentID).Scan(&tasks); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if tasks != 1 {
		t.Errorf("tasks = %d after retry, want 1", tasks)
	}
}

// MissingSchema is empty on a database with every migration, and names the
// trigger when it is gone.
func TestIncidentReportIntegration_MissingSchema(t *testing.T) {
	pool := incidentReportTestPool(t)
	ctx := context.Background()
	repo := repository.NewIncidentReportRepository(repository.NewScoped(pool))
	if missing, err := repo.MissingSchema(ctx); err != nil || len(missing) != 0 {
		t.Fatalf("with 0181 applied: missing=%v err=%v, want none", missing, err)
	}
	irExec(t, pool, `DROP TRIGGER incident_outbox ON incident`)
	t.Cleanup(func() {
		irExec(t, pool, `CREATE TRIGGER incident_outbox AFTER UPDATE ON incident FOR EACH ROW EXECUTE FUNCTION trg_event_outbox()`)
	})
	missing, err := repo.MissingSchema(ctx)
	if err != nil || len(missing) != 1 || missing[0] != "incident trigger incident_outbox" {
		t.Errorf("without the trigger: missing=%v err=%v, want [incident trigger incident_outbox]", missing, err)
	}
}
