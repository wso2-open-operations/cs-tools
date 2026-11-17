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

// Integration tests for the port of ServiceNow's "Cascade closure of
// Incident Tasks" (closing or canceling an incident closes its open tasks)
// and IncidentTaskRepository.UpdateIncidentTask, against a real Postgres
// with migrations through 0189 applied. Skipped unless
// INCIDENT_TASK_GATE_TEST_DSN is set.
package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const gateTaskSubject = "incident-task-gate integration test"

func incidentTaskGatePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_TASK_GATE_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_TASK_GATE_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	seedIncidentCreateFixture(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	cleanup := func() {
		_, _ = repository.NewScoped(pool).Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, gateTaskSubject)
	}
	cleanup()
	t.Cleanup(cleanup)
	return pool
}

// resolvedIncident creates an incident and moves it to RESOLVED with a
// resolution code and notes, so CLOSED only has the task gate left to pass.
func resolvedIncident(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	resp, err := repo.CreateIncident(ctx, icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	state, code, notes := "RESOLVED", "SOLVED_PERMANENTLY", "fixed the gateway config"
	if _, err := repo.UpdateIncidentLifecycle(ctx, resp.Incident.ID, repository.IncidentLifecycleUpdate{
		State: &state, ResolutionCode: &code, ResolutionNotes: &notes,
	}, "jane.doe@test.local"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resp.Incident.ID
}

// addTask inserts an incident task in the given state; nil leaves state NULL.
func addTask(t *testing.T, pool *pgxpool.Pool, incidentID string, state *string) (id, number string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if err := repository.NewScoped(pool).QueryRow(ctx, `
		WITH wi AS (
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES (gen_random_uuid(), NOW(), NOW(), 'test', 'test', next_portal_work_item_number(), $1, 'INCIDENT_TASK')
			RETURNING id, number
		), it AS (
			INSERT INTO incident_task (id, opened_on, state, is_active, incident_id)
			SELECT id, NOW(), $2::TEXT::incident_task_state_enum, TRUE, $3::uuid FROM wi
			RETURNING id
		)
		SELECT wi.id::TEXT, wi.number FROM wi JOIN it ON it.id = wi.id`,
		gateTaskSubject, state, incidentID).Scan(&id, &number); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return id, number
}

// moveIncident sets the incident's state as the ic-engineer fixture user.
func moveIncident(t *testing.T, pool *pgxpool.Pool, id, state string) {
	t.Helper()
	if _, err := repository.NewIncidentRepository(repository.NewScoped(pool)).UpdateIncidentLifecycle(
		repository.WithSystemIdentity(context.Background()), id,
		repository.IncidentLifecycleUpdate{State: &state}, "ic-engineer@test.local"); err != nil {
		t.Fatalf("move incident to %s: %v", state, err)
	}
}

type taskRow struct {
	state     *string
	active    bool
	closedOn  *time.Time
	closedBy  *string
	workNotes []string
}

func readTask(t *testing.T, pool *pgxpool.Pool, id string) taskRow {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	var r taskRow
	if err := scoped.QueryRow(ctx, `SELECT state::TEXT, is_active, closed_on, closed_by_id::TEXT FROM incident_task WHERE id = $1`, id).
		Scan(&r.state, &r.active, &r.closedOn, &r.closedBy); err != nil {
		t.Fatalf("read task: %v", err)
	}
	rows, err := scoped.Query(ctx, `SELECT content FROM comment WHERE work_item_id = $1 AND type = 'WORK_NOTE' ORDER BY created_on`, id)
	if err != nil {
		t.Fatalf("read task work notes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("scan work note: %v", err)
		}
		r.workNotes = append(r.workNotes, c)
	}
	return r
}

func incidentNumber(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var n string
	if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT number FROM work_item WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("read incident number: %v", err)
	}
	return n
}

func incidentState(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT state::TEXT FROM incident WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read incident state: %v", err)
	}
	return s
}

func strp(s string) *string { return &s }

// TestIncidentTaskCascade_CloseClosesOpenTasks: closing the incident moves
// every open task (OPEN, WORK_IN_PROGRESS, NULL) to CLOSED_INCOMPLETE with
// ServiceNow's work note and closure fields; an already-closed task keeps
// its state, closed_on and notes.
func TestIncidentTaskCascade_CloseClosesOpenTasks(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	open, _ := addTask(t, pool, incID, strp("OPEN"))
	wip, _ := addTask(t, pool, incID, strp("WORK_IN_PROGRESS"))
	null, _ := addTask(t, pool, incID, nil)
	done, _ := addTask(t, pool, incID, strp("OPEN"))
	tasks := repository.NewIncidentTaskRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: done, State: strp("CLOSED_COMPLETE")}, "jane.doe@test.local"); err != nil {
		t.Fatalf("close task beforehand: %v", err)
	}
	doneBefore := readTask(t, pool, done)

	moveIncident(t, pool, incID, "CLOSED")

	if got := incidentState(t, pool, incID); got != "CLOSED" {
		t.Fatalf("incident state = %s, want CLOSED", got)
	}
	wantNote := "Incident Task is Closed Incomplete based on closure of " + incidentNumber(t, pool, incID) + "."
	for name, id := range map[string]string{"OPEN": open, "WORK_IN_PROGRESS": wip, "NULL": null} {
		r := readTask(t, pool, id)
		if deref(r.state) != "CLOSED_INCOMPLETE" || r.active || r.closedOn == nil || deref(r.closedBy) != icEngineerID {
			t.Errorf("%s task: state=%s active=%v closed_on=%v closed_by=%s, want CLOSED_INCOMPLETE/false/set/%s",
				name, deref(r.state), r.active, r.closedOn, deref(r.closedBy), icEngineerID)
		}
		if len(r.workNotes) != 1 || r.workNotes[0] != wantNote {
			t.Errorf("%s task work notes = %q, want [%q]", name, r.workNotes, wantNote)
		}
	}
	after := readTask(t, pool, done)
	if deref(after.state) != "CLOSED_COMPLETE" || len(after.workNotes) != 0 || !after.closedOn.Equal(*doneBefore.closedOn) {
		t.Errorf("already-closed task changed: state=%s notes=%q closed_on %v -> %v", deref(after.state), after.workNotes, doneBefore.closedOn, after.closedOn)
	}
}

// TestIncidentTaskCascade_CancelSkipsOpenTasks: canceling the incident moves
// open tasks to CLOSED_SKIPPED with the cancelation work note.
func TestIncidentTaskCascade_CancelSkipsOpenTasks(t *testing.T) {
	pool := incidentTaskGatePool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	resp, err := repository.NewIncidentRepository(repository.NewScoped(pool)).CreateIncident(ctx, icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	incID := resp.Incident.ID
	task, _ := addTask(t, pool, incID, strp("OPEN"))

	moveIncident(t, pool, incID, "CANCELED")

	r := readTask(t, pool, task)
	wantNote := "Incident Task is Closed Skipped based on cancelation of " + incidentNumber(t, pool, incID) + "."
	if deref(r.state) != "CLOSED_SKIPPED" || r.active || len(r.workNotes) != 1 || r.workNotes[0] != wantNote {
		t.Errorf("task: state=%s active=%v notes=%q, want CLOSED_SKIPPED/false/[%q]", deref(r.state), r.active, r.workNotes, wantNote)
	}
}

// TestIncidentTaskCascade_OnlyOnClose: resolving (or any non-closing move)
// leaves tasks open; an incident with no tasks closes as before.
func TestIncidentTaskCascade_OnlyOnClose(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	task, _ := addTask(t, pool, incID, strp("OPEN"))
	if r := readTask(t, pool, task); deref(r.state) != "OPEN" || !r.active {
		t.Errorf("task after resolve: state=%s active=%v, want OPEN/true", deref(r.state), r.active)
	}

	bare := resolvedIncident(t, pool)
	moveIncident(t, pool, bare, "CLOSED")
	if got := incidentState(t, pool, bare); got != "CLOSED" {
		t.Errorf("incident with no tasks: state = %s, want CLOSED", got)
	}
}

// TestUpdateIncidentTask_CloseAndReopen checks ServiceNow's task-table side
// effects: closing deactivates and stamps closed_on / closed_by_id; reopening
// reactivates and KEEPS them; closing again keeps the original stamps.
func TestUpdateIncidentTask_CloseAndReopen(t *testing.T) {
	pool := incidentTaskGatePool(t)
	incID := resolvedIncident(t, pool)
	taskID, _ := addTask(t, pool, incID, strp("OPEN"))
	tasks := repository.NewIncidentTaskRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	// Email matched case-insensitively: the fixture stores ic-engineer@test.local.
	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskID, State: strp("CLOSED_INCOMPLETE"), CloseNotes: strp("ran out of time")}, "IC-Engineer@test.local"); err != nil {
		t.Fatalf("close: %v", err)
	}
	first := readTask(t, pool, taskID)
	if deref(first.state) != "CLOSED_INCOMPLETE" || first.active || first.closedOn == nil || deref(first.closedBy) != icEngineerID {
		t.Fatalf("after close: state=%s active=%v closed_on=%v closed_by=%s", deref(first.state), first.active, first.closedOn, deref(first.closedBy))
	}
	detail, err := tasks.GetIncidentTask(ctx, taskID)
	if err != nil || detail.CloseNotes == nil || *detail.CloseNotes != "ran out of time" {
		t.Errorf("GetIncidentTask closeNotes = %v (err %v), want the saved notes", detail.CloseNotes, err)
	}

	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskID, State: strp("OPEN")}, "jane.doe@test.local"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reopened := readTask(t, pool, taskID)
	if deref(reopened.state) != "OPEN" || !reopened.active || reopened.closedOn == nil || !reopened.closedOn.Equal(*first.closedOn) || deref(reopened.closedBy) != icEngineerID {
		t.Errorf("after reopen: state=%s active=%v closed_on=%v closed_by=%s, want OPEN/true/kept/kept",
			deref(reopened.state), reopened.active, reopened.closedOn, deref(reopened.closedBy))
	}

	if err := tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: taskID, State: strp("CLOSED_COMPLETE")}, "ic-caller@test.local"); err != nil {
		t.Fatalf("close again: %v", err)
	}
	again := readTask(t, pool, taskID)
	if again.active || !again.closedOn.Equal(*first.closedOn) || deref(again.closedBy) != icEngineerID {
		t.Errorf("after closing again: active=%v closed_on=%v closed_by=%s, want false and the original stamps", again.active, again.closedOn, deref(again.closedBy))
	}

	err = tasks.UpdateIncidentTask(ctx, domain.UpdateIncidentTaskRequest{ID: "f0000000-0000-0000-0000-000000000001", State: strp("OPEN")}, "x@test.local")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("unknown task: got %T: %v, want NotFoundError", err, err)
	}
}
