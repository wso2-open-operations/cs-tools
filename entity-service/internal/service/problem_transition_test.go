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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func strp(s string) *string { return &s }

// problemDetailStub answers the pre-move requirement check and the
// post-write re-read: a problem that already has an assignee and fix notes,
// so every move's requirements are met.
func problemDetailStub(context.Context, string) (domain.ProblemDetail, error) {
	id, notes := testDeploymentUUID, "rotate logs"
	return domain.ProblemDetail{ID: &id, AssignedTo: &domain.EntityRef{ID: testUUID, Name: "Jane"}, FixNotes: &notes}, nil
}

func userCtxProblem(t *testing.T) context.Context {
	return contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

// The table is ServiceNow's ProblemUtils._PROBLEM_TRANSITIONS (101 New, 102
// Assess, 103 RCA, 104 Fix in Progress, 106 Resolved, 107 Closed), in
// problem_state_enum labels.
func TestProblemTransitions_MatchServiceNow(t *testing.T) {
	want := map[string][2]string{
		"assess":  {"NEW", "ASSESS"},
		"confirm": {"ASSESS", "ROOT_CAUSE_ANALYSIS"},
		"fix":     {"ROOT_CAUSE_ANALYSIS", "FIX_IN_PROGRESS"},
		"resolve": {"FIX_IN_PROGRESS", "RESOLVED"},
		"close":   {"RESOLVED", "CLOSED"},
	}
	if len(problemTransitions) != len(want) {
		t.Fatalf("transitions = %d, want %d", len(problemTransitions), len(want))
	}
	for name, ft := range want {
		got, ok := problemTransitions[name]
		if !ok || got.Name != name || got.From != ft[0] || got.To != ft[1] {
			t.Errorf("%s = %+v, want %s -> %s", name, got, ft[0], ft[1])
		}
	}
}

// DATA_SOURCE=postgres: the move and its fields go to Postgres in one call,
// with the from-state enforced; there is no ServiceNow to call.
func TestUpdateProblem_PlainPostgres_TransitionEnforcesFromState(t *testing.T) {
	var gotT repository.ProblemTransition
	var gotEnforce bool
	var gotReq domain.UpdateProblemRequest
	repo := &stubProblemRepo{
		applyProblemTransition: func(_ context.Context, req domain.UpdateProblemRequest, tr repository.ProblemTransition, enforce bool, actor string) (time.Time, error) {
			gotReq, gotT, gotEnforce = req, tr, enforce
			if actor != "jane.doe@example.com" {
				t.Errorf("actor = %q", actor)
			}
			return time.Now(), nil
		},
		getProblem: problemDetailStub,
	}
	svc := NewProblemService(repo)
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{
		ID: testDeploymentUUID, Transition: strp(" fix "), CauseNotes: strp("disk full"), FixNotes: strp("rotate logs"),
	}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	if gotT != problemTransitions["fix"] || !gotEnforce {
		t.Errorf("transition %+v enforce %v, want fix with the from-state enforced", gotT, gotEnforce)
	}
	if strOrEmpty(gotReq.CauseNotes) != "disk full" || strOrEmpty(gotReq.FixNotes) != "rotate logs" {
		t.Errorf("fields did not ride along with the move: %+v", gotReq)
	}
}

// DATA_SOURCE=postgres: plain fields (now including the assignment group)
// are written; there is no mirror to dispatch.
func TestUpdateProblem_PlainPostgres_FieldsIncludingAssignmentGroup(t *testing.T) {
	var got domain.UpdateProblemRequest
	repo := &stubProblemRepo{
		updateProblemFields: func(_ context.Context, req domain.UpdateProblemRequest, _ string) (time.Time, error) {
			got = req
			return time.Now(), nil
		},
		getProblem: problemDetailStub,
	}
	if _, err := NewProblemService(repo).UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{
		ID: testDeploymentUUID, AssignmentGroupID: strp(testUUID),
	}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	if strOrEmpty(got.AssignmentGroupID) != testUUID {
		t.Errorf("assignmentGroupId = %v, want %s", got.AssignmentGroupID, testUUID)
	}
}

// Dual-write: Postgres first with the from-state ENFORCED, then the move and
// the request's fields are mirrored to ServiceNow asynchronously (e).
func TestUpdateProblem_DualWrite_TransitionGoesToPostgresFirstThenMirrors(t *testing.T) {
	var pgReq domain.UpdateProblemRequest
	mirrored := make(chan domain.UpdateProblemRequest, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirrored <- req
			return domain.UpdateProblemResponse{}, nil
		},
	}
	repo := &stubProblemRepo{
		applyProblemTransition: func(_ context.Context, req domain.UpdateProblemRequest, tr repository.ProblemTransition, enforce bool, _ string) (time.Time, error) {
			pgReq = req
			if tr != problemTransitions["resolve"] || !enforce {
				t.Errorf("Postgres got %+v enforce=%v, want resolve with the from-state enforced", tr, enforce)
			}
			return time.Now(), nil
		},
		getProblem: problemDetailStub,
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{
		ID: testDeploymentUUID, Transition: strp("resolve"), FixNotes: strp("patched"),
	}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	if strOrEmpty(pgReq.FixNotes) != "patched" {
		t.Errorf("Postgres got %+v, want the fields of the same request", pgReq)
	}
	select {
	case req := <-mirrored:
		if strOrEmpty(req.Transition) != "resolve" || strOrEmpty(req.FixNotes) != "patched" {
			t.Errorf("mirror got %+v, want the transition and its fields", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the transition was never mirrored to ServiceNow")
	}
}

// Dual-write: a ServiceNow refusal (its 409 when a business rule reverts the
// move, or its 400 "invalid state") never reaches the caller, and Postgres has
// already moved (a).
func TestUpdateProblem_DualWrite_ServiceNowRefusalIsNotReturned(t *testing.T) {
	for name, refusal := range map[string]error{
		"409 refusal":   &apierror.ConflictError{Msg: "State transition rejected"},
		"400 bad state": &apierror.ValidationError{Msg: "Invalid state transition. Current state: 102"},
	} {
		t.Run(name, func(t *testing.T) {
			attempted := make(chan struct{}, 1)
			mirror := &stubMirrorProblemService{
				updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
					attempted <- struct{}{}
					return domain.UpdateProblemResponse{}, refusal
				},
			}
			var pgMoves int
			repo := &stubProblemRepo{
				applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
					pgMoves++
					return time.Now(), nil
				},
				getProblem: problemDetailStub,
			}
			failures := &recordingSNWritebackFailures{}
			svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(failures))
			if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("confirm")}); err != nil {
				t.Fatalf("err = %v, want success despite the ServiceNow refusal", err)
			}
			if pgMoves != 1 {
				t.Errorf("Postgres moves = %d, want 1", pgMoves)
			}
			select {
			case <-attempted:
			case <-time.After(2 * time.Second):
				t.Fatal("the mirror was never attempted")
			}
		})
	}
}

// Dual-write: the from-state check still rejects a wrong-state move with the
// 400, and ServiceNow is not called (b). The mirror stub panics if called.
func TestUpdateProblem_DualWrite_WrongFromStateIsRefusedWithoutCallingServiceNow(t *testing.T) {
	wrong := &apierror.ValidationError{Msg: "Invalid state transition. Current state: ASSESS"}
	repo := &stubProblemRepo{
		applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
			return time.Time{}, wrong
		},
		getProblem: problemDetailStub,
	}
	mirror := &stubMirrorProblemService{
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			t.Error("ServiceNow must not be called when Postgres refuses the move")
			return domain.UpdateProblemResponse{}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	_, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("assess")})
	if !errors.Is(err, wrong) {
		t.Errorf("err = %v, want the 400 naming the state", err)
	}
	time.Sleep(100 * time.Millisecond) // let a wrongly dispatched mirror show itself
}

// Dual-write: Postgres failing is returned and nothing is mirrored.
func TestUpdateProblem_DualWrite_PostgresFailureIsReturned(t *testing.T) {
	boom := errors.New("connection reset")
	repo := &stubProblemRepo{
		applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
			return time.Time{}, boom
		},
		getProblem: problemDetailStub,
	}
	mirror := &stubMirrorProblemService{
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			t.Error("ServiceNow must not be called when Postgres fails")
			return domain.UpdateProblemResponse{}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("assess")}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the Postgres failure", err)
	}
	time.Sleep(100 * time.Millisecond)
}

// Stuck row (c): Postgres is NEW while ServiceNow is already at Assess and
// answers "Current state: 102". The assess succeeds in Postgres; the
// ServiceNow error is only logged.
func TestUpdateProblem_DualWrite_StuckRowMovesInPostgres(t *testing.T) {
	attempted := make(chan struct{}, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			attempted <- struct{}{}
			return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "Invalid state transition. Current state: 102"}
		},
	}
	var gotTr repository.ProblemTransition
	repo := &stubProblemRepo{
		applyProblemTransition: func(_ context.Context, _ domain.UpdateProblemRequest, tr repository.ProblemTransition, enforce bool, _ string) (time.Time, error) {
			gotTr = tr
			if !enforce {
				t.Error("from-state must be enforced")
			}
			return time.Now(), nil
		},
		getProblem: problemInState("NEW", true),
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("assess")}); err != nil {
		t.Fatalf("err = %v, want the stuck row to move in Postgres", err)
	}
	if gotTr != problemTransitions["assess"] {
		t.Errorf("Postgres transition = %+v, want assess", gotTr)
	}
	select {
	case <-attempted:
	case <-time.After(2 * time.Second):
		t.Fatal("the mirror was never attempted")
	}
}

// Dual-write: a fields-only request with an assignment group still goes
// Postgres first, and the async mirror carries the group to ServiceNow.
func TestUpdateProblem_DualWrite_FieldsMirrorTheAssignmentGroup(t *testing.T) {
	mirrored := make(chan domain.UpdateProblemRequest, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirrored <- req
			return domain.UpdateProblemResponse{}, nil
		},
	}
	repo := &stubProblemRepo{
		updateProblemFields: func(context.Context, domain.UpdateProblemRequest, string) (time.Time, error) { return time.Now(), nil },
		getProblem:          problemDetailStub,
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignmentGroupID: strp(testUUID)}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	select {
	case req := <-mirrored:
		if strOrEmpty(req.AssignmentGroupID) != testUUID || req.Transition != nil {
			t.Errorf("mirror got %+v, want the group and no transition", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the assignment group was never mirrored to ServiceNow")
	}
}

// An unknown transition is refused before anything is called, in both modes.
func TestUpdateProblem_UnknownTransitionRejected(t *testing.T) {
	for name, svc := range map[string]ProblemService{
		"postgres":   NewProblemService(&stubProblemRepo{}),
		"dual-write": NewProblemServiceWithSNMirror(&stubProblemRepo{}, &stubMirrorProblemService{}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{})),
	} {
		_, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("reopen")})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %T %v, want a ValidationError", name, err, err)
		}
	}
}

// ServiceNow's state model wants an assignee for Assess and fix notes for
// Resolved (discovery script 61). A move missing one is a 400 before
// ServiceNow or Postgres is touched, in both modes -- the stub repo and
// mirror panic if either is reached.
func TestUpdateProblem_TransitionRequirements(t *testing.T) {
	bare := func(context.Context, string) (domain.ProblemDetail, error) {
		id := testDeploymentUUID
		return domain.ProblemDetail{ID: &id}, nil
	}
	for name, svc := range map[string]ProblemService{
		"postgres":   NewProblemService(&stubProblemRepo{getProblem: bare}),
		"dual-write": NewProblemServiceWithSNMirror(&stubProblemRepo{getProblem: bare}, &stubMirrorProblemService{}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{})),
	} {
		for _, c := range []struct {
			req  domain.UpdateProblemRequest
			want string
		}{
			{domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("assess")}, "assess needs an assignee"},
			{domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("resolve")}, "resolve needs fix notes"},
			{domain.UpdateProblemRequest{ID: testDeploymentUUID, Transition: strp("resolve"), FixNotes: strp(" ")}, "resolve needs fix notes"},
		} {
			_, err := svc.UpdateProblem(userCtxProblem(t), c.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, c.want) {
				t.Errorf("%s %s: err = %v, want a ValidationError %q", name, *c.req.Transition, err, c.want)
			}
		}
	}
}

// A requirement met by the request itself passes: the assignee or fix notes
// ride along with the move, as in ServiceNow's own Assess / Resolve dialogs.
func TestUpdateProblem_RequirementsMetByTheRequest(t *testing.T) {
	bare := func(context.Context, string) (domain.ProblemDetail, error) {
		id := testDeploymentUUID
		return domain.ProblemDetail{ID: &id}, nil
	}
	var moved []string
	repo := &stubProblemRepo{
		getProblem: bare,
		applyProblemTransition: func(_ context.Context, _ domain.UpdateProblemRequest, tr repository.ProblemTransition, _ bool, _ string) (time.Time, error) {
			moved = append(moved, tr.Name)
			return time.Now(), nil
		},
	}
	svc := NewProblemService(repo)
	for _, req := range []domain.UpdateProblemRequest{
		{ID: testDeploymentUUID, Transition: strp("assess"), AssignedToID: strp(testUUID)},
		{ID: testDeploymentUUID, Transition: strp("resolve"), FixNotes: strp("patched the gateway")},
		{ID: testDeploymentUUID, Transition: strp("confirm")},
	} {
		if _, err := svc.UpdateProblem(userCtxProblem(t), req); err != nil {
			t.Errorf("%s: %v", *req.Transition, err)
		}
	}
	if strings.Join(moved, ",") != "assess,resolve,confirm" {
		t.Errorf("moved = %v", moved)
	}
}

// The portal sends targetResolutionDate as ServiceNow's "YYYY-MM-DD
// HH:mm:ss" (UTC) -- the format this API documents. Postgres must get the
// same instant as RFC3339, and the ServiceNow mirror its own format back.
func TestUpdateProblem_TargetDateAcceptsThePortalFormat(t *testing.T) {
	for in, want := range map[string][2]string{
		"2026-10-06 00:00:00":       {"2026-10-06T00:00:00Z", "2026-10-06 00:00:00"},
		"2026-10-06T05:30:00+05:30": {"2026-10-06T00:00:00Z", "2026-10-06 00:00:00"},
	} {
		var toPG string
		mirrored := make(chan domain.UpdateProblemRequest, 1)
		repo := &stubProblemRepo{
			updateProblemFields: func(_ context.Context, req domain.UpdateProblemRequest, _ string) (time.Time, error) {
				toPG = strOrEmpty(req.TargetResolutionDate)
				return time.Now(), nil
			},
			getProblem: problemDetailStub,
		}
		mirror := &stubMirrorProblemService{
			updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
				mirrored <- req
				return domain.UpdateProblemResponse{}, nil
			},
		}
		svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, TargetResolutionDate: strp(in)}); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if toPG != want[0] {
			t.Errorf("%s: Postgres got %q, want %q", in, toPG, want[0])
		}
		select {
		case req := <-mirrored:
			if strOrEmpty(req.TargetResolutionDate) != want[1] {
				t.Errorf("%s: ServiceNow got %q, want %q", in, strOrEmpty(req.TargetResolutionDate), want[1])
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: never mirrored", in)
		}
	}
}

// A transition carrying a date sends ServiceNow its format too.
func TestUpdateProblem_DualWriteTransitionSendsServiceNowDateFormat(t *testing.T) {
	toSNCh := make(chan string, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			toSNCh <- strOrEmpty(req.TargetResolutionDate)
			return domain.UpdateProblemResponse{}, nil
		},
	}
	repo := &stubProblemRepo{
		getProblem: problemDetailStub,
		applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
			return time.Now(), nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{
		ID: testDeploymentUUID, Transition: strp("confirm"), TargetResolutionDate: strp("2026-10-06 08:15:00"),
	}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	select {
	case toSN := <-toSNCh:
		if toSN != "2026-10-06 08:15:00" {
			t.Errorf("ServiceNow got %q", toSN)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the transition was never mirrored")
	}
}

// A blank value in the request is what the save would write, so it must not
// pass on the strength of what the problem already has: resolve with
// fixNotes "" would clear the stored notes and resolve without any.
func TestUpdateProblem_BlankRequestValueOverridesStoredOne(t *testing.T) {
	svc := NewProblemService(&stubProblemRepo{getProblem: problemDetailStub}) // has an assignee and fix notes
	_, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{
		ID: testDeploymentUUID, Transition: strp("resolve"), FixNotes: strp(""),
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "resolve needs fix notes") {
		t.Errorf("err = %v, want the fix-notes ValidationError", err)
	}
}

// problemInState answers GetProblem with a problem in the given state,
// owned or not, for the implied-assess tests.
func problemInState(state string, owned bool) func(context.Context, string) (domain.ProblemDetail, error) {
	return func(context.Context, string) (domain.ProblemDetail, error) {
		id := testDeploymentUUID
		d := domain.ProblemDetail{ID: &id, State: &state}
		if owned {
			d.AssignedTo = &domain.EntityRef{ID: testUUID, Name: "Jane Doe"}
		}
		return d, nil
	}
}

// Dual-write: ServiceNow moves an unowned NEW problem to Assess once an
// owner is set, so the owner-only request is an implied assess: one Postgres
// transition first (from-state enforced) carrying the owner, then a
// fields-only mirror (ServiceNow makes the move itself).
func TestUpdateProblem_DualWrite_ImpliedAssessOnFirstOwner(t *testing.T) {
	mirrored := make(chan domain.UpdateProblemRequest, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirrored <- req
			return domain.UpdateProblemResponse{}, nil
		},
	}
	var pgMoves int
	repo := &stubProblemRepo{
		applyProblemTransition: func(_ context.Context, req domain.UpdateProblemRequest, tr repository.ProblemTransition, enforce bool, _ string) (time.Time, error) {
			pgMoves++
			if tr != problemTransitions["assess"] || !enforce || strOrEmpty(req.AssignedToID) != testUUID {
				t.Errorf("Postgres got %+v %+v enforce=%v, want assess with the owner, enforced", tr, req, enforce)
			}
			return time.Now(), nil
		},
		getProblem: problemInState("NEW", false),
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: strp(testUUID)}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	if pgMoves != 1 {
		t.Errorf("Postgres moves = %d, want 1", pgMoves)
	}
	select {
	case req := <-mirrored:
		if strOrEmpty(req.AssignedToID) != testUUID || req.Transition != nil {
			t.Errorf("mirror got %+v, want the owner and no explicit transition", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the implied assess was never mirrored")
	}
}

// Dual-write: a ServiceNow failure on the implied assess is only logged.
func TestUpdateProblem_DualWrite_ImpliedAssessServiceNowFailureIsNotReturned(t *testing.T) {
	attempted := make(chan struct{}, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			attempted <- struct{}{}
			return domain.UpdateProblemResponse{}, errors.New("upstream unavailable")
		},
	}
	repo := &stubProblemRepo{
		applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
			return time.Now(), nil
		},
		getProblem: problemInState("NEW", false),
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: strp(testUUID)}); err != nil {
		t.Fatalf("err = %v, want success", err)
	}
	select {
	case <-attempted:
	case <-time.After(2 * time.Second):
		t.Fatal("the mirror was never attempted")
	}
}

// Dual-write: Postgres failing on the implied assess is returned.
func TestUpdateProblem_DualWrite_ImpliedAssessPostgresFailureIsReturned(t *testing.T) {
	boom := errors.New("connection reset")
	repo := &stubProblemRepo{
		applyProblemTransition: func(context.Context, domain.UpdateProblemRequest, repository.ProblemTransition, bool, string) (time.Time, error) {
			return time.Time{}, boom
		},
		getProblem: problemInState("NEW", false),
	}
	mirror := &stubMirrorProblemService{
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			t.Error("ServiceNow must not be called when Postgres fails")
			return domain.UpdateProblemResponse{}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: strp(testUUID)}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the Postgres failure", err)
	}
	time.Sleep(100 * time.Millisecond)
}

// Dual-write: everything that is not "first owner on a NEW problem" stays on
// the fields-only path (Postgres first, async mirror; no synchronous
// ServiceNow call, no transition).
func TestUpdateProblem_DualWrite_NoImpliedAssess(t *testing.T) {
	tests := []struct {
		name   string
		get    func(context.Context, string) (domain.ProblemDetail, error)
		assign *string
	}{
		{"already owned NEW problem", problemInState("NEW", true), strp(testUUID)},
		{"problem past NEW", problemInState("ASSESS", false), strp(testUUID)},
		{"unowned NEW problem, no assignee in the request", problemInState("NEW", false), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var fieldsWrites int
			mirrored := make(chan domain.UpdateProblemRequest, 1)
			mirror := &stubMirrorProblemService{
				updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
					mirrored <- req
					return domain.UpdateProblemResponse{}, nil
				},
			}
			repo := &stubProblemRepo{
				updateProblemFields: func(context.Context, domain.UpdateProblemRequest, string) (time.Time, error) {
					fieldsWrites++
					return time.Now(), nil
				},
				getProblem: tc.get,
			}
			svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
			if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: tc.assign, FixNotes: strp("notes")}); err != nil {
				t.Fatalf("UpdateProblem: %v", err)
			}
			if fieldsWrites != 1 {
				t.Errorf("fields writes = %d, want 1 (the fields-only path)", fieldsWrites)
			}
			select {
			case <-mirrored:
			case <-time.After(2 * time.Second):
				t.Fatal("fields-only path never mirrored to ServiceNow")
			}
		})
	}
}

// Postgres-only: an owner on a NEW problem is a plain field write. No read
// of the current problem, no transition (the stub repo panics on either).
func TestUpdateProblem_PlainPostgres_OwnerOnNewProblemStaysFieldsOnly(t *testing.T) {
	var fieldsWrites int
	repo := &stubProblemRepo{
		updateProblemFields: func(context.Context, domain.UpdateProblemRequest, string) (time.Time, error) {
			fieldsWrites++
			return time.Now(), nil
		},
	}
	svc := NewProblemService(repo)
	repo.getProblem = func(context.Context, string) (domain.ProblemDetail, error) {
		// Only the post-write re-read may call this, once.
		return problemInState("NEW", true)(context.Background(), "")
	}
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: strp(testUUID)}); err != nil {
		t.Fatalf("UpdateProblem: %v", err)
	}
	if fieldsWrites != 1 {
		t.Errorf("fields writes = %d, want 1", fieldsWrites)
	}
}

// A blank assignee never reaches the implied-assess check: it is refused as a
// non-UUID before any read or write (the stub repo panics on either).
func TestUpdateProblem_DualWrite_BlankAssigneeNeverImpliesAssess(t *testing.T) {
	mirror := &stubMirrorProblemService{}
	svc := NewProblemServiceWithSNMirror(&stubProblemRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, AssignedToID: strp("  ")}); err == nil {
		t.Error("want a validation error for a blank assignee")
	}
}
