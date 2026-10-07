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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// "[WSO2 Cloud Ops] Post resolution tasks" end to end against a real database
// with migrations 0181 and 0188: a Choreo incident is resolved, the 0181
// trigger records it, and one drain writes the report, the problem and the
// link -- or an alert task. Run with
// INCIDENT_REPORT_TEST_DSN=postgres://... go test -run PostResolutionIntegration ./internal/service/
//
// The flow names the Choreo service and three groups by their real ids. A
// row the test has to add for them is deleted afterwards; one the database
// already has is left alone.

const prIncidentID = "47777777-0000-0000-0000-0000000000a1"

// prEnsure inserts a row with a fixed id if it is missing, and registers its
// deletion only in that case.
func prEnsure(t *testing.T, pool *pgxpool.Pool, table, id, insert string, args ...any) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1)`, id).Scan(&exists); err != nil {
		t.Fatalf("check %s %s: %v", table, id, err)
	}
	if exists {
		return
	}
	irExec(t, pool, insert, args...)
	t.Cleanup(func() { irExec(t, pool, `DELETE FROM `+table+` WHERE id = $1`, id) })
}

func prCleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	irExec(t, pool, `UPDATE incident SET problem_id = NULL WHERE id = $1`, prIncidentID)
	irExec(t, pool, `DELETE FROM work_item WHERE id IN (SELECT id FROM problem WHERE incident_id = $1)`, prIncidentID)
	irExec(t, pool, `DELETE FROM work_item WHERE id IN (SELECT id FROM incident_task WHERE incident_id = $1)`, prIncidentID)
	irExec(t, pool, `DELETE FROM work_item WHERE id = $1`, prIncidentID)
	irExec(t, pool, `DELETE FROM event_outbox WHERE entity_type = 'incident' AND entity_id = $1`, prIncidentID)
}

// prSeed creates a Choreo incident, In Progress, impact HIGH / urgency MEDIUM /
// priority HIGH, and drains everything already pending.
func prSeed(t *testing.T, pool *pgxpool.Pool, d *IncidentReportDrainer) {
	t.Helper()
	for id, name := range map[string]string{
		groupWSO2SRETeam:            "WSO2 SRE Team",
		groupChoreoSpecialOps:       "Choreo Special Ops",
		groupAsgardeoOperationsTeam: "Asgardeo Operations Team",
	} {
		prEnsure(t, pool, `"group"`, id,
			`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, NOW(), NOW(), 't', 't', $2)`, id, name)
	}
	prEnsure(t, pool, "service", postResolutionServiceChoreo,
		`INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, NOW(), NOW(), 't', 't', 'Choreo', 'SVC-PR-1')`,
		postResolutionServiceChoreo)
	prCleanup(t, pool)
	t.Cleanup(func() { prCleanup(t, pool) })
	irExec(t, pool, `
		INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1, NOW(), NOW(), 't', 't', 'INC-PR-0001', 'Choreo gateway 502s', 'INCIDENT')`, prIncidentID)
	irExec(t, pool, `
		INSERT INTO incident (id, state, priority, impact, urgency, service_id)
		VALUES ($1, 'IN_PROGRESS', 'HIGH', 'HIGH', 'MEDIUM', $2)`, prIncidentID, postResolutionServiceChoreo)
	if _, err := d.drainOnce(context.Background()); err != nil {
		t.Fatalf("drain seed: %v", err)
	}
}

func TestPostResolutionIntegration_WorkaroundCreatesTheProblem(t *testing.T) {
	pool := incidentReportTestPool(t)
	d := irDrainer(pool)
	prSeed(t, pool, d)
	ctx := context.Background()

	irExec(t, pool, `UPDATE incident SET state = 'RESOLVED', resolution_code = 'SOLVED_WORK_AROUND' WHERE id = $1`, prIncidentID)
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}

	var (
		number, subject, createdBy, group, service, priority, impact, urgency, state, linked string
		active                                                                               bool
	)
	err := pool.QueryRow(ctx, `
		SELECT wi.number, wi.subject, wi.created_by, wi.assignment_group_id::text, p.service_id::text,
		       p.priority::text, p.impact::text, p.urgency::text, p.state::text, p.is_active,
		       (SELECT i.problem_id::text FROM incident i WHERE i.id = $1)
		FROM problem p JOIN work_item wi ON wi.id = p.id
		WHERE p.incident_id = $1`, prIncidentID).
		Scan(&number, &subject, &createdBy, &group, &service, &priority, &impact, &urgency, &state, &active, &linked)
	if err != nil {
		t.Fatalf("read the problem: %v", err)
	}
	if !strings.HasPrefix(number, "CS-PORTAL-") || subject != "Fix the root cause of INC-PR-0001" || createdBy != "system" {
		t.Errorf("number/subject/createdBy = %s / %q / %s", number, subject, createdBy)
	}
	if group != groupChoreoSpecialOps || service != postResolutionServiceChoreo {
		t.Errorf("group/service = %s / %s", group, service)
	}
	if priority != "HIGH" || impact != "HIGH" || urgency != "MEDIUM" || state != "NEW" || !active {
		t.Errorf("priority/impact/urgency/state/active = %s/%s/%s/%s/%v", priority, impact, urgency, state, active)
	}
	var problemID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM problem WHERE incident_id = $1`, prIncidentID).Scan(&problemID); err != nil {
		t.Fatalf("problem id: %v", err)
	}
	if linked != problemID {
		t.Errorf("incident.problem_id = %q, want the new problem %s", linked, problemID)
	}

	// The report and the link are incident UPDATEs the trigger records; they
	// carry no state change and must drain as no-ops, not a second problem.
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	var problems int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM problem WHERE incident_id = $1`, prIncidentID).Scan(&problems); err != nil {
		t.Fatalf("count problems: %v", err)
	}
	if problems != 1 {
		t.Errorf("problems = %d, want exactly 1", problems)
	}

	// Resolved again (reopened and re-resolved): it has a problem now, so no
	// second one -- block 8's "problem record does not exist".
	irExec(t, pool, `UPDATE incident SET state = 'IN_PROGRESS' WHERE id = $1`, prIncidentID)
	irExec(t, pool, `UPDATE incident SET state = 'RESOLVED' WHERE id = $1`, prIncidentID)
	for i := 0; i < 2; i++ {
		if _, err := d.drainOnce(ctx); err != nil {
			t.Fatalf("drainOnce: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM problem WHERE incident_id = $1`, prIncidentID).Scan(&problems); err != nil {
		t.Fatalf("count problems: %v", err)
	}
	if problems != 1 {
		t.Errorf("after re-resolving: problems = %d, want still 1", problems)
	}
}

func TestPostResolutionIntegration_FalseAlarmCreatesTheAlertTask(t *testing.T) {
	pool := incidentReportTestPool(t)
	d := irDrainer(pool)
	prSeed(t, pool, d)
	ctx := context.Background()

	irExec(t, pool, `UPDATE incident SET state = 'RESOLVED', resolution_code = 'FALSE_ALARM' WHERE id = $1`, prIncidentID)
	if _, err := d.drainOnce(ctx); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	var subject, group, service, priority, typ, state string
	err := pool.QueryRow(ctx, `
		SELECT wi.subject, wi.assignment_group_id::text, it.service_id::text, it.priority::text, it.type::text, it.state::text
		FROM incident_task it JOIN work_item wi ON wi.id = it.id
		WHERE it.incident_id = $1 AND it.type = 'DEFAULT'`, prIncidentID).
		Scan(&subject, &group, &service, &priority, &typ, &state)
	if err != nil {
		t.Fatalf("read the alert task: %v", err)
	}
	if subject != "[Alert Task][Falser Alarm] INC-PR-0001 alert is a false alarm" || group != groupWSO2SRETeam ||
		service != postResolutionServiceChoreo || priority != "CRITICAL" || state != "OPEN" {
		t.Errorf("task = %q group %s service %s priority %s state %s", subject, group, service, priority, state)
	}
	var problems int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM problem WHERE incident_id = $1`, prIncidentID).Scan(&problems); err != nil {
		t.Fatalf("count problems: %v", err)
	}
	if problems != 0 {
		t.Errorf("a false alarm created %d problems", problems)
	}
}

// Any service gets the workaround problem, not only the flow's two; it takes
// the incident's own service and group.
func TestPostResolutionIntegration_WorkaroundOnAnotherService(t *testing.T) {
	pool := incidentReportTestPool(t)
	d := irDrainer(pool)
	prSeed(t, pool, d)
	ctx := context.Background()

	const otherService = "47777777-0000-0000-0000-0000000000b1"
	prEnsure(t, pool, "service", otherService,
		`INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, NOW(), NOW(), 't', 't', 'monitoring', 'SVC-PR-2')`,
		otherService)
	irExec(t, pool, `UPDATE incident SET service_id = $2 WHERE id = $1`, prIncidentID, otherService)
	irExec(t, pool, `UPDATE work_item SET assignment_group_id = $2 WHERE id = $1`, prIncidentID, groupWSO2SRETeam)
	irExec(t, pool, `UPDATE incident SET state = 'RESOLVED', resolution_code = 'SOLVED_WORK_AROUND' WHERE id = $1`, prIncidentID)
	for i := 0; i < 3; i++ {
		if _, err := d.drainOnce(ctx); err != nil {
			t.Fatalf("drainOnce: %v", err)
		}
	}

	var group, service, linked string
	err := pool.QueryRow(ctx, `
		SELECT wi.assignment_group_id::text, p.service_id::text, (SELECT i.problem_id::text FROM incident i WHERE i.id = $1)
		FROM problem p JOIN work_item wi ON wi.id = p.id
		WHERE p.incident_id = $1`, prIncidentID).Scan(&group, &service, &linked)
	if err != nil {
		t.Fatalf("read the problem: %v", err)
	}
	if group != groupWSO2SRETeam || service != otherService || linked == "" {
		t.Errorf("group/service/linked = %s / %s / %q, want the incident's group and service, linked", group, service, linked)
	}
}
