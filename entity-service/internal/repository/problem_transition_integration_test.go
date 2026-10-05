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

// Integration tests for ProblemRepository.TransitionProblem and the
// assignment-group write in UpdateProblemFields, against a real Postgres
// with migrations through 0187 applied. Skipped unless
// PROBLEM_TRANSITION_TEST_DSN is set.
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

const (
	ptActorID    = "e1000000-0000-0000-0000-000000000001"
	ptEngineerID = "e1000000-0000-0000-0000-000000000002"
	ptGroupID    = "e2000000-0000-0000-0000-000000000001"
	ptActorEmail = "pt-actor@test.local"
	ptSubject    = "problem-transition integration test"
)

func problemTransitionPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PROBLEM_TRANSITION_TEST_DSN")
	if dsn == "" {
		t.Skip("PROBLEM_TRANSITION_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, ptSubject)
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, ptGroupID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, []string{ptActorID, ptEngineerID})
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	for _, u := range []struct{ id, name, email string }{
		{ptActorID, "pt-actor", "PT-Actor@test.local"},
		{ptEngineerID, "pt-engineer", "pt-engineer@test.local"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES ($1, $2, $2, $3, $4)`,
			u.id, now, u.name, u.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		VALUES ($1, $2, $2, 'test', 'test', 'Problem Managers')`, ptGroupID, now); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	return pool
}

// newTestProblem creates a NEW problem, then moves it straight to state
// (and sets assignee/resolutionCode) with raw SQL, so each case starts
// exactly where it needs to.
func newTestProblem(t *testing.T, pool *pgxpool.Pool, state domain.ProblemState, assignee, resolutionCode *string) string {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	p, err := repository.NewProblemRepository(scoped).CreateProblem(ctx, domain.CreateProblemRequest{Subject: ptSubject}, "test")
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}
	id := *p.ID
	if _, err := scoped.Exec(ctx, `UPDATE problem SET state = $2::problem_state_enum, resolution_code = $3::problem_resolution_code_enum WHERE id = $1`,
		id, string(state), resolutionCode); err != nil {
		t.Fatalf("set state: %v", err)
	}
	if _, err := scoped.Exec(ctx, `UPDATE work_item SET assigned_to_id = $2::uuid WHERE id = $1`, id, assignee); err != nil {
		t.Fatalf("set assignee: %v", err)
	}
	return id
}

type problemRow struct {
	state, problemState, resolutionCode, causeNotes, assignedTo, groupID *string
	active                                                               *bool
	resolvedOn, closedOn                                                 *time.Time
	resolvedBy                                                           *string
}

func readProblemRow(t *testing.T, pool *pgxpool.Pool, id string) problemRow {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	var r problemRow
	if err := repository.NewScoped(pool).QueryRow(ctx, `
		SELECT pr.state::TEXT, pr.problem_state::TEXT, pr.resolution_code::TEXT, pr.cause_notes,
		       wi.assigned_to_id::TEXT, wi.assignment_group_id::TEXT, pr.is_active,
		       pr.resolved_on, pr.closed_on, pr.resolved_by_id::TEXT
		FROM problem pr JOIN work_item wi ON wi.id = pr.id WHERE pr.id = $1`, id).Scan(
		&r.state, &r.problemState, &r.resolutionCode, &r.causeNotes,
		&r.assignedTo, &r.groupID, &r.active, &r.resolvedOn, &r.closedOn, &r.resolvedBy); err != nil {
		t.Fatalf("read back: %v", err)
	}
	return r
}

// TestProblemTransition_FullWalk drives an unassigned problem through all
// five moves and checks each one's side effects.
func TestProblemTransition_FullWalk(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	id := newTestProblem(t, pool, domain.ProblemStateNew, nil, nil)

	causeNotes := "stale cache key"
	for _, name := range domain.ProblemTransitionNames {
		req := domain.UpdateProblemRequest{ID: id}
		if name == "fix" {
			req.CauseNotes = &causeNotes
		}
		res, err := repo.TransitionProblem(ctx, req, domain.ProblemTransitions[name], ptActorEmail)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.Reverted {
			t.Fatalf("%s: reverted on an unassigned problem", name)
		}
		r := readProblemRow(t, pool, id)
		want := string(domain.ProblemTransitions[name].To)
		if deref(r.state) != want || deref(r.problemState) != want {
			t.Fatalf("%s: state/problem_state = %s/%s, want %s", name, deref(r.state), deref(r.problemState), want)
		}
	}

	r := readProblemRow(t, pool, id)
	if deref(r.causeNotes) != causeNotes {
		t.Errorf("cause_notes = %s, want the value sent with 'fix'", deref(r.causeNotes))
	}
	if deref(r.resolutionCode) != "FIX_APPLIED" {
		t.Errorf("resolution_code = %s, want FIX_APPLIED after 'resolve'", deref(r.resolutionCode))
	}
	if r.resolvedOn == nil || deref(r.resolvedBy) != ptActorID {
		t.Errorf("resolved_on/resolved_by_id = %v/%s, want set to now/the actor (email matched case-insensitively)", r.resolvedOn, deref(r.resolvedBy))
	}
	if r.active == nil || *r.active || r.closedOn == nil {
		t.Errorf("is_active/closed_on = %v/%v, want false/set after 'close'", r.active, r.closedOn)
	}
}

// TestProblemTransition_WrongStateRejected: a move from any state but its
// own From is a ValidationError, and nothing is written.
func TestProblemTransition_WrongStateRejected(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	id := newTestProblem(t, pool, domain.ProblemStateNew, nil, nil)

	notes := "should not be saved"
	_, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id, CauseNotes: &notes}, domain.ProblemTransitions["confirm"], ptActorEmail)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	r := readProblemRow(t, pool, id)
	if deref(r.state) != "NEW" || r.causeNotes != nil {
		t.Errorf("state/cause_notes = %s/%s, want NEW/<nil> (nothing written)", deref(r.state), deref(r.causeNotes))
	}
}

// TestProblemTransition_CloseRefusedWhenRiskAccepted ports the native
// "Complete" action's canComplete() guard.
func TestProblemTransition_CloseRefusedWhenRiskAccepted(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	riskAccepted := "RISK_ACCEPTED"
	id := newTestProblem(t, pool, domain.ProblemStateResolved, nil, &riskAccepted)

	_, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id}, domain.ProblemTransitions["close"], ptActorEmail)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if r := readProblemRow(t, pool, id); deref(r.state) != "RESOLVED" {
		t.Errorf("state = %s, want RESOLVED", deref(r.state))
	}
}

// TestProblemTransition_AssessRule reproduces ServiceNow's "Update Problem
// State to Assess" rule: with an assignee after the save, the problem lands
// on ASSESS, the other writes stand, and Reverted is set.
func TestProblemTransition_AssessRule(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	engineer := ptEngineerID

	t.Run("existing assignee", func(t *testing.T) {
		id := newTestProblem(t, pool, domain.ProblemStateAssess, &engineer, nil)
		notes := "saved even though the move is reverted"
		res, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id, CauseNotes: &notes}, domain.ProblemTransitions["confirm"], ptActorEmail)
		if err != nil {
			t.Fatalf("TransitionProblem: %v", err)
		}
		if !res.Reverted {
			t.Error("Reverted = false, want true")
		}
		r := readProblemRow(t, pool, id)
		if deref(r.state) != "ASSESS" || deref(r.problemState) != "ASSESS" || deref(r.causeNotes) != notes {
			t.Errorf("state/problem_state/cause_notes = %s/%s/%s, want ASSESS/ASSESS/%s", deref(r.state), deref(r.problemState), deref(r.causeNotes), notes)
		}
	})

	t.Run("assignee set in the same request", func(t *testing.T) {
		id := newTestProblem(t, pool, domain.ProblemStateAssess, nil, nil)
		res, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id, AssignedToID: &engineer}, domain.ProblemTransitions["confirm"], ptActorEmail)
		if err != nil {
			t.Fatalf("TransitionProblem: %v", err)
		}
		r := readProblemRow(t, pool, id)
		if !res.Reverted || deref(r.state) != "ASSESS" || deref(r.assignedTo) != ptEngineerID {
			t.Errorf("reverted/state/assignee = %v/%s/%s, want true/ASSESS/%s", res.Reverted, deref(r.state), deref(r.assignedTo), ptEngineerID)
		}
	})

	t.Run("assess with an assignee is not a revert", func(t *testing.T) {
		id := newTestProblem(t, pool, domain.ProblemStateNew, &engineer, nil)
		res, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id}, domain.ProblemTransitions["assess"], ptActorEmail)
		if err != nil {
			t.Fatalf("TransitionProblem: %v", err)
		}
		if r := readProblemRow(t, pool, id); res.Reverted || deref(r.state) != "ASSESS" {
			t.Errorf("reverted/state = %v/%s, want false/ASSESS", res.Reverted, deref(r.state))
		}
	})

	t.Run("resolve side effects stand on a revert", func(t *testing.T) {
		id := newTestProblem(t, pool, domain.ProblemStateFixInProgress, &engineer, nil)
		res, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: id}, domain.ProblemTransitions["resolve"], ptActorEmail)
		if err != nil {
			t.Fatalf("TransitionProblem: %v", err)
		}
		r := readProblemRow(t, pool, id)
		if !res.Reverted || deref(r.state) != "ASSESS" || deref(r.resolutionCode) != "FIX_APPLIED" || r.resolvedOn != nil {
			t.Errorf("reverted/state/resolution_code/resolved_on = %v/%s/%s/%v, want true/ASSESS/FIX_APPLIED/<nil>",
				res.Reverted, deref(r.state), deref(r.resolutionCode), r.resolvedOn)
		}
	})
}

// TestProblemUpdateFields_AssignmentGroup covers the work_item
// assignment_group_id write: the group comes back with its name, and an
// unknown group is a ValidationError naming assignmentGroupId.
func TestProblemUpdateFields_AssignmentGroup(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	id := newTestProblem(t, pool, domain.ProblemStateAssess, nil, nil)

	group := ptGroupID
	res, err := repo.UpdateProblemFields(ctx, domain.UpdateProblemRequest{ID: id, AssignmentGroupID: &group}, ptActorEmail)
	if err != nil {
		t.Fatalf("UpdateProblemFields: %v", err)
	}
	if res.AssignmentGroup == nil || res.AssignmentGroup.ID != ptGroupID || res.AssignmentGroup.Name != "Problem Managers" {
		t.Errorf("AssignmentGroup = %+v, want %s/Problem Managers", res.AssignmentGroup, ptGroupID)
	}
	if r := readProblemRow(t, pool, id); deref(r.groupID) != ptGroupID {
		t.Errorf("assignment_group_id = %s, want %s", deref(r.groupID), ptGroupID)
	}

	unknown := "e2000000-0000-0000-0000-0000000000ff"
	_, err = repo.UpdateProblemFields(ctx, domain.UpdateProblemRequest{ID: id, AssignmentGroupID: &unknown}, ptActorEmail)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || len(ve.Msg) < len("assignmentGroupId") || ve.Msg[:len("assignmentGroupId")] != "assignmentGroupId" {
		t.Fatalf("expected ValidationError naming assignmentGroupId, got %T: %v", err, err)
	}
}

// TestProblemTransition_NotFound: an unknown id is a NotFoundError.
func TestProblemTransition_NotFound(t *testing.T) {
	pool := problemTransitionPool(t)
	repo := repository.NewProblemRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	_, err := repo.TransitionProblem(ctx, domain.UpdateProblemRequest{ID: "e3000000-0000-0000-0000-000000000001"}, domain.ProblemTransitions["assess"], ptActorEmail)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}
