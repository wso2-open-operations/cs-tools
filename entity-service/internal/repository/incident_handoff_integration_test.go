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

// Integration tests for IncidentRepository.ApplySpecialistHandoff and the
// specialist-handoff summary GetIncidentByID derives, against a real Postgres
// with the migrations applied. Skipped unless
// INCIDENT_HANDOFF_TEST_DSN is set.
package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	hoSpecialGroup = "e7000000-0000-0000-0000-000000000001"
	hoSubGroup     = "e7000000-0000-0000-0000-000000000002"
	hoTaskSubject  = "[Runbook Task] handoff integration test"
	hoActor        = "ic-engineer@test.local"
)

func handoffPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_HANDOFF_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_HANDOFF_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	seedIncidentCreateFixture(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, hoTaskSubject)
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = ANY($1::uuid[])`, []string{hoSpecialGroup, hoSubGroup})
	}
	cleanup()
	t.Cleanup(cleanup)
	now := time.Now().UTC()
	for id, name := range map[string]string{hoSpecialGroup: "Test Special Ops", hoSubGroup: "Test Sub Special Ops"} {
		if _, err := pool.Exec(ctx, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', $3)`, id, now, name); err != nil {
			t.Fatalf("seed group: %v", err)
		}
	}
	return pool
}

// inProgressIncident creates an incident assigned to the engineer, In Progress.
func inProgressIncident(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	resp, err := repo.CreateIncident(ctx, icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	state := "IN_PROGRESS"
	if err := repo.UpdateIncidentLifecycle(ctx, resp.Incident.ID, repository.IncidentLifecycleUpdate{State: &state}, "jane.doe@test.local"); err != nil {
		t.Fatalf("move to In Progress: %v", err)
	}
	return resp.Incident.ID
}

const hoBlob = `{"reasonCode":"runbook-not-working","reasonDescription":"Runbook doesn't solve the incident","escalationTeam":"ho-test-sub-team"}`

func hoPlan(snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
	group := hoSubGroup
	return repository.SpecialistHandoffPlan{GroupID: group, TaskSubject: hoTaskSubject, TaskGroupID: &group, WorkNotes: []string{hoBlob}}, nil
}

// TestSpecialistHandoff_WritesAndReadsBack: the handoff moves the group,
// clears the assignee, opens the runbook task and writes the reason note;
// GetIncidentByID then derives the summary from them, GitHub link included.
func TestSpecialistHandoff_WritesAndReadsBack(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))

	if v, err := repo.GetIncidentByID(ctx, incID); err != nil || v.SpecialistHandoff != nil {
		t.Fatalf("before the handoff: summary %+v err %v, want none", v.SpecialistHandoff, err)
	}

	var seen repository.SpecialistHandoffSnapshot
	written, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(s repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		seen = s
		return hoPlan(s)
	})
	if err != nil {
		t.Fatalf("ApplySpecialistHandoff: %v", err)
	}
	if seen.State != "IN_PROGRESS" || seen.ServiceID == nil || *seen.ServiceID != icServiceID || seen.Number == "" {
		t.Errorf("snapshot %+v, want the In Progress incident on %s", seen, icServiceID)
	}
	if written.GroupName != "Test Sub Special Ops" || !strings.HasPrefix(written.TaskNumber, "TASK1") {
		t.Errorf("written %+v", written)
	}

	// The follow-up note the service writes once GitHub has answered.
	if _, err := repo.CreateIncidentComment(ctx, incID, domain.CommentTypeWorkNote,
		"Escalated to Special Ops team. Escalated by IC Engineer(ic-engineer@test.local) Opened an internal issue. Please access the ticket using the link https://github.com/wso2-enterprise/choreo/issues/42 to add more details to the ticket if needed", hoActor); err != nil {
		t.Fatalf("escalated note: %v", err)
	}

	var task struct {
		state, priority, service, group string
		active                          bool
	}
	if err := repository.NewScoped(pool).QueryRow(ctx, `
		SELECT it.state::text, it.priority::text, it.service_id::text, wi.assignment_group_id::text, it.is_active
		FROM incident_task it JOIN work_item wi ON wi.id = it.id WHERE it.id = $1 AND it.incident_id = $2`, written.TaskID, incID).
		Scan(&task.state, &task.priority, &task.service, &task.group, &task.active); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if task.state != "OPEN" || task.priority != "CRITICAL" || task.service != icServiceID || task.group != hoSubGroup || !task.active {
		t.Errorf("task %+v, want OPEN CRITICAL on the incident's service, in the Special Ops group, active", task)
	}

	v, err := repo.GetIncidentByID(ctx, incID)
	if err != nil {
		t.Fatalf("GetIncidentByID: %v", err)
	}
	if v.AssignmentGroup == nil || v.AssignmentGroup.ID != hoSubGroup || v.AssignedTo != nil {
		t.Errorf("group %+v assignee %+v, want the specialist group and no assignee", v.AssignmentGroup, v.AssignedTo)
	}
	s := v.SpecialistHandoff
	if s == nil {
		t.Fatal("SpecialistHandoff is nil after a handoff")
	}
	if s.ReasonCode != "runbook-not-working" || s.ReasonDescription != "Runbook doesn't solve the incident" ||
		s.EscalationTeam == nil || *s.EscalationTeam != "ho-test-sub-team" || s.HandedOffBy == nil || *s.HandedOffBy != hoActor || s.HandedOffAt == "" {
		t.Errorf("summary reason/team/by: %+v", s)
	}
	if s.GithubIssueURL == nil || *s.GithubIssueURL != "https://github.com/wso2-enterprise/choreo/issues/42" {
		t.Errorf("github url %v", s.GithubIssueURL)
	}
	if s.AssignmentGroup.ID != hoSubGroup || s.Task.Number != written.TaskNumber || s.Task.Subject != hoTaskSubject ||
		s.Task.StateLabel == nil || *s.Task.StateLabel != "Open" {
		t.Errorf("summary group/task: %+v / %+v", s.AssignmentGroup, s.Task)
	}
}

// TestSpecialistHandoff_RefusedPlanWritesNothing: an eligibility refusal
// rolls the whole handoff back.
func TestSpecialistHandoff_RefusedPlanWritesNothing(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	before, _ := repo.GetIncidentByID(ctx, incID)

	refusal := &apierror.ConflictError{Msg: "not eligible"}
	_, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		return repository.SpecialistHandoffPlan{}, refusal
	})
	if !errors.Is(err, refusal) {
		t.Fatalf("got %v, want the plan's refusal", err)
	}
	after, _ := repo.GetIncidentByID(ctx, incID)
	if (before.AssignmentGroup == nil) != (after.AssignmentGroup == nil) || after.SpecialistHandoff != nil {
		t.Errorf("incident changed: group %+v -> %+v, summary %+v", before.AssignmentGroup, after.AssignmentGroup, after.SpecialistHandoff)
	}
	var tasks int
	_ = repository.NewScoped(pool).QueryRow(ctx, `SELECT count(*) FROM incident_task WHERE incident_id = $1`, incID).Scan(&tasks)
	if tasks != 0 {
		t.Errorf("%d tasks written by a refused handoff", tasks)
	}
}

// TestSpecialistHandoff_UnknownGroupAndIncident: a specialist group missing
// from this database is a ValidationError; an unknown incident NotFound.
func TestSpecialistHandoff_UnknownGroupAndIncident(t *testing.T) {
	pool := handoffPool(t)
	incID := inProgressIncident(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))

	_, err := repo.ApplySpecialistHandoff(ctx, incID, hoActor, func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
		return repository.SpecialistHandoffPlan{GroupID: "e7000000-0000-0000-0000-0000000000ff", TaskSubject: hoTaskSubject}, nil
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("unknown group: %T %v, want ValidationError", err, err)
	}
	_, err = repo.ApplySpecialistHandoff(ctx, "e7000000-0000-0000-0000-0000000000aa", hoActor, hoPlan)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("unknown incident: %T %v, want NotFoundError", err, err)
	}
}
