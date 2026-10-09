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

package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// "Mark as completed" (digiops-cs#3350) concludes a call with no post-call notes, in
// one click. Nothing else in UpdateCallRequest looks at the call's current state, so
// the repository guards exactly this transition: it only applies to a call that is
// scheduled or notes pending. These tests run against a real Postgres with the same
// fixtures and prerequisites as call_request_search_all_integration_test.go
// (CALL_REQUEST_TEST_DSN, a non-superuser non-BYPASSRLS login, checked below).
//
//	CALL_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run CallRequestMarkCompleted

type callRow struct {
	state     string
	notes     *string
	updatedBy *string
}

func readCall(t *testing.T, scoped *repository.Scoped, id string) callRow {
	t.Helper()
	var r callRow
	ctx := repository.WithSystemIdentity(context.Background())
	if err := scoped.QueryRow(ctx,
		`SELECT state::text, all_notes, updated_by FROM customer_call WHERE id = $1::text::uuid`, id,
	).Scan(&r.state, &r.notes, &r.updatedBy); err != nil {
		t.Fatalf("read call %s: %v", id, err)
	}
	return r
}

func setCall(t *testing.T, scoped *repository.Scoped, id, state string, notes *string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := scoped.Exec(ctx,
		`UPDATE customer_call SET state = $2::text::customer_call_state_enum, all_notes = $3 WHERE id = $1::text::uuid`,
		id, state, notes); err != nil {
		t.Fatalf("set call %s to %s: %v", id, state, err)
	}
}

func TestCallRequestMarkCompletedIntegration(t *testing.T) {
	pool := callRequestTestPool(t)
	seedCallRequestSearchFixtures(t, pool)
	callRequestRLSPrecondition(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewCallRequestRepository(scoped)
	ctx := repository.WithSystemIdentity(context.Background())
	const who = "engineer@example.com"
	keep := "notes already on the call"

	complete := func(id string, notes *string) error {
		_, err := repo.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: id, State: domain.CallRequestStateConcluded, Notes: notes}, nil, who)
		return err
	}

	t.Run("allowed from scheduled and notes pending, notes untouched", func(t *testing.T) {
		for _, from := range []string{"SCHEDULED", "NOTES_PENDING"} {
			setCall(t, scoped, crCallPendingX, from, &keep)
			if err := complete(crCallPendingX, nil); err != nil {
				t.Fatalf("complete from %s: %v", from, err)
			}
			got := readCall(t, scoped, crCallPendingX)
			if got.state != "CONCLUDED" {
				t.Errorf("from %s: state = %s, want CONCLUDED", from, got.state)
			}
			if got.notes == nil || *got.notes != keep {
				t.Errorf("from %s: notes = %v, want them left as %q", from, got.notes, keep)
			}
			if got.updatedBy == nil || *got.updatedBy != who {
				t.Errorf("from %s: updated_by = %v, want %s", from, got.updatedBy, who)
			}
		}
	})

	t.Run("blank notes are no notes: they neither pass the guard nor erase the real ones", func(t *testing.T) {
		blank := "   "
		setCall(t, scoped, crCallPendingX, "SCHEDULED", &keep)
		if err := complete(crCallPendingX, &blank); err != nil {
			t.Fatalf("complete with blank notes: %v", err)
		}
		if got := readCall(t, scoped, crCallPendingX); got.state != "CONCLUDED" || got.notes == nil || *got.notes != keep {
			t.Errorf("after a blank-notes conclude: state=%s notes=%v, want CONCLUDED with %q kept", got.state, got.notes, keep)
		}
		setCall(t, scoped, crCallPendingX, "PENDING_ON_WSO2", nil)
		var conflict *apierror.ConflictError
		if err := complete(crCallPendingX, &blank); !errors.As(err, &conflict) {
			t.Errorf("blank notes from pending_on_wso2: got %v, want a ConflictError (blank notes must not slip past the guard)", err)
		}
	})

	t.Run("refused from every other state, and the call is left as it was", func(t *testing.T) {
		for from, label := range map[string]string{
			"PENDING_ON_WSO2":     "Pending on WSO2",
			"PENDING_ON_CUSTOMER": "Pending on Customer",
			"CUSTOMER_REJECTED":   "Customer Rejected",
			"WSO2_REJECTED":       "WSO2 Rejected",
			"CANCELED":            "Canceled",
			"CONCLUDED":           "Concluded", // already completed: a second click must not rewrite it
		} {
			setCall(t, scoped, crCallPendingX, from, nil)
			err := complete(crCallPendingX, nil)
			var conflict *apierror.ConflictError
			if !errors.As(err, &conflict) {
				t.Errorf("from %s: got %v, want a ConflictError", from, err)
				continue
			}
			if !strings.Contains(conflict.Msg, label) {
				t.Errorf("from %s: message %q does not name the current state %q", from, conflict.Msg, label)
			}
			if got := readCall(t, scoped, crCallPendingX); got.state != from {
				t.Errorf("from %s: a refused conclude changed the state to %s", from, got.state)
			}
		}
	})

	t.Run("concluding WITH notes is unchanged: no guard, whatever the state", func(t *testing.T) {
		n := "real post-call notes"
		for _, from := range []string{"PENDING_ON_WSO2", "NOTES_PENDING", "CANCELED"} {
			setCall(t, scoped, crCallPendingX, from, nil)
			if err := complete(crCallPendingX, &n); err != nil {
				t.Errorf("from %s with notes: %v", from, err)
				continue
			}
			if got := readCall(t, scoped, crCallPendingX); got.state != "CONCLUDED" || got.notes == nil || *got.notes != n {
				t.Errorf("from %s with notes: state=%s notes=%v", from, got.state, got.notes)
			}
		}
	})

	t.Run("a call with no state at all is refused as unknown, not completed", func(t *testing.T) {
		if _, err := scoped.Exec(ctx, `UPDATE customer_call SET state = NULL WHERE id = $1::text::uuid`, crCallPendingX); err != nil {
			t.Fatalf("clear state: %v", err)
		}
		var conflict *apierror.ConflictError
		err := complete(crCallPendingX, nil)
		if !errors.As(err, &conflict) || !strings.Contains(conflict.Msg, "unknown state") {
			t.Errorf("NULL state: got %v, want a ConflictError saying the state is unknown", err)
		}
		var state *string
		if e := scoped.QueryRow(ctx, `SELECT state::text FROM customer_call WHERE id = $1::text::uuid`, crCallPendingX).Scan(&state); e != nil || state != nil {
			t.Errorf("a refused conclude wrote a state: %v (err %v)", state, e)
		}
	})

	t.Run("the other post-call fields are still written when no notes are given", func(t *testing.T) {
		setCall(t, scoped, crCallPendingX, "SCHEDULED", &keep)
		plan, actual, blank := "agreed next steps", 25, "  "
		_, err := repo.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{
			ID: crCallPendingX, State: domain.CallRequestStateConcluded, Notes: &blank, Plan: &plan, ActualDurationMin: &actual,
		}, nil, who)
		if err != nil {
			t.Fatalf("complete with a plan and an actual duration: %v", err)
		}
		var state, notes, gotPlan, gotActual *string
		if e := scoped.QueryRow(ctx,
			`SELECT state::text, all_notes, plan, actual_call_duration FROM customer_call WHERE id = $1::text::uuid`, crCallPendingX,
		).Scan(&state, &notes, &gotPlan, &gotActual); e != nil {
			t.Fatalf("read: %v", e)
		}
		if state == nil || *state != "CONCLUDED" || notes == nil || *notes != keep ||
			gotPlan == nil || *gotPlan != plan || gotActual == nil || *gotActual != "25" {
			t.Errorf("state=%v notes=%v plan=%v actual=%v; want CONCLUDED, the old notes kept, the plan and 25 minutes written", state, notes, gotPlan, gotActual)
		}
	})

	t.Run("an unknown call, or one on another case, is not found rather than a conflict", func(t *testing.T) {
		setCall(t, scoped, crCallPendingX, "SCHEDULED", nil)
		var notFound *apierror.NotFoundError
		if err := complete("ca000000-0000-0000-0000-0000000000fe", nil); !errors.As(err, &notFound) {
			t.Errorf("unknown id: got %v, want a NotFoundError", err)
		}
		_, err := repo.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: crCallPendingX, CaseID: crCaseY1, State: domain.CallRequestStateConcluded}, nil, who)
		if !errors.As(err, &notFound) {
			t.Errorf("right call, wrong case: got %v, want a NotFoundError", err)
		}
		if got := readCall(t, scoped, crCallPendingX); got.state != "SCHEDULED" {
			t.Errorf("a not-found conclude changed the state to %s", got.state)
		}
	})
}

// Completing a call without notes is staff-only. The customer portal's backend
// forwards any state key it is given and cannot send notes, and RLS lets a project
// member update their own project's calls, so the old notes requirement was the only
// thing that kept a customer from concluding a call: it must stay out of their reach,
// on their own project's call as much as anyone else's, and the answer must not
// depend on the call (nothing about it, or whether it exists, is revealed).
func TestCallRequestMarkCompletedCustomerScopeIntegration(t *testing.T) {
	pool := callRequestTestPool(t)
	seedCallRequestSearchFixtures(t, pool)
	callRequestRLSPrecondition(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewCallRequestRepository(scoped)
	customer := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: crCustomerEmail})

	setCall(t, scoped, crCallPendingX, "SCHEDULED", nil)  // on the customer's own project
	setCall(t, scoped, crCallPendingY, "CANCELED", nil)   // on another project
	setCall(t, scoped, crCallHandedToA, "SCHEDULED", nil) // on another project
	complete := func(id string) error {
		_, err := repo.UpdateCallRequest(customer, domain.UpdateCallRequestRequest{ID: id, State: domain.CallRequestStateConcluded}, nil, crCustomerEmail)
		return err
	}

	want := map[string]string{crCallPendingX: "SCHEDULED", crCallPendingY: "CANCELED", crCallHandedToA: "SCHEDULED"}
	for id, state := range want {
		var forbidden *apierror.ForbiddenError
		if err := complete(id); !errors.As(err, &forbidden) {
			t.Errorf("call %s: a customer completing a call got %v, want a ForbiddenError", id, err)
		}
		if got := readCall(t, scoped, id); got.state != state {
			t.Errorf("call %s: a refused conclude changed the state from %s to %s", id, state, got.state)
		}
	}
	// A call that does not exist is refused the same way: no answer distinguishes it.
	var forbidden *apierror.ForbiddenError
	if err := complete("ca000000-0000-0000-0000-0000000000fe"); !errors.As(err, &forbidden) {
		t.Errorf("unknown call: got %v, want the same ForbiddenError", err)
	}

	// The staff path on the very same call still works, so the refusal is about who
	// asks, not about the call.
	staff := repository.WithSystemIdentity(context.Background())
	if _, err := repo.UpdateCallRequest(staff, domain.UpdateCallRequestRequest{ID: crCallPendingX, State: domain.CallRequestStateConcluded}, nil, "engineer@example.com"); err != nil {
		t.Fatalf("staff completing the same call: %v", err)
	}
	if got := readCall(t, scoped, crCallPendingX); got.state != "CONCLUDED" {
		t.Errorf("state after staff completion = %s, want CONCLUDED", got.state)
	}
}
