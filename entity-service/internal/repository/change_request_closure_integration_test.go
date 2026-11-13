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
	"os"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// change_request.closed_on / closed_by_user_id: stamped by every writer that moves
// a change request into Closed or Rollback, never by Canceled, never moved by a
// resend; and migration 0288's backfill from event_outbox. Same harness as the
// other crFlow tests (CHANGE_REQUEST_TEST_DSN, run as a superuser and as csm_app).
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestClosureIntegration -p 1

func (f *crFlow) closure(id string) (closedOn *time.Time, closedBy *string) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys,
		`SELECT closed_on, closed_by_user_id::text FROM change_request WHERE id = $1`, id).Scan(&closedOn, &closedBy); err != nil {
		f.t.Fatalf("read closure: %v", err)
	}
	return closedOn, closedBy
}

func (f *crFlow) wantClosure(what, id string, wantStamped bool, wantBy string) {
	f.t.Helper()
	on, by := f.closure(id)
	if !wantStamped {
		if on != nil || by != nil {
			f.t.Fatalf("%s: closed_on = %v, closed_by = %v, want both empty", what, on, by)
		}
		return
	}
	if on == nil || time.Since(*on) > time.Minute {
		f.t.Fatalf("%s: closed_on = %v, want now", what, on)
	}
	got := ""
	if by != nil {
		got = *by
	}
	if got != wantBy {
		f.t.Fatalf("%s: closed_by_user_id = %q, want %q", what, got, wantBy)
	}
}

func TestChangeRequestClosureIntegration_Writers(t *testing.T) {
	f := newCustomerGroupFlow(t)

	t.Run("staff close", func(t *testing.T) {
		id := f.transitionFixture("REVIEW", false, false)
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		f.wantClosure("REVIEW -> CLOSED", id, true, crFlowCreatorID)
		// A resend of Closed keeps the first stamp.
		on, _ := f.closure(id)
		if _, err := f.patchState(id, domain.ChangeRequestStateClosed); err != nil {
			t.Fatalf("resend closed: %v", err)
		}
		if again, _ := f.closure(id); again == nil || !again.Equal(*on) {
			t.Fatalf("resend moved closed_on from %v to %v", on, again)
		}
	})
	t.Run("staff rollback", func(t *testing.T) {
		id := f.transitionFixture("REVIEW", false, false)
		f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
		f.wantClosure("REVIEW -> ROLLBACK", id, true, crFlowCreatorID)
	})
	t.Run("cancel stamps nothing", func(t *testing.T) {
		id := f.transitionFixture("IMPLEMENT", false, false)
		f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
		f.wantClosure("IMPLEMENT -> CANCELED", id, false, "")
	})
	t.Run("other moves stamp nothing", func(t *testing.T) {
		id := f.transitionFixture("SCHEDULED", false, false)
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.wantClosure("SCHEDULED -> IMPLEMENT", id, false, "")
	})
	t.Run("the customer's review closes it", func(t *testing.T) {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.setState(id, "SCHEDULED")
		f.driveToCustomerReview(id)
		if _, err := f.reviewAs(id, crScopeUserA1, true); err != nil {
			t.Fatalf("customer review: %v", err)
		}
		f.wantState(id, "CLOSED")
		f.wantClosure("Customer Review approved", id, true, crScopeUserA1)
	})
	t.Run("the customer's failed review rolls it back", func(t *testing.T) {
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, true)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.setState(id, "SCHEDULED")
		f.driveToCustomerReview(id)
		if _, err := f.reviewAs(id, crScopeUserA2, false); err != nil {
			t.Fatalf("customer review: %v", err)
		}
		f.wantState(id, "ROLLBACK")
		f.wantClosure("Customer Review rejected", id, true, crScopeUserA2)
	})
	t.Run("the GitHub sync closes it with no closer", func(t *testing.T) {
		gh := repository.NewGithubMutationRepository(f.scoped)
		id := f.transitionFixture("REVIEW", false, false)
		if changed, err := gh.SetState(f.sys, id, "CLOSED"); err != nil || !changed {
			t.Fatalf("SetState(CLOSED) = %v, %v", changed, err)
		}
		f.wantClosure("GitHub sync REVIEW -> CLOSED", id, true, "")
	})
}

// 0288 fills closed_on from the first outbox transition into Closed / Rollback,
// and the closer from the work item when it still carries that instant; a row
// with no transition, or one that already has closed_on, is left alone; running
// it twice changes nothing more.
func TestChangeRequestClosureIntegration_Backfill(t *testing.T) {
	f := newCustomerGroupFlow(t)
	sql, err := os.ReadFile("../../migrations/0288_change_request_closed_on_backfill.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	clear := func(id string) {
		f.execSQL(`UPDATE change_request SET closed_on = NULL, closed_by_user_id = NULL WHERE id = $1`, id)
	}

	// Closed by staff: the work item's updated_on is the outbox row's instant.
	closed := f.transitionFixture("REVIEW", false, false)
	f.step(closed, domain.ChangeRequestStateClosed, "CLOSED")
	wantOn, _ := f.closure(closed)
	clear(closed)

	// Rolled back, then a work note moved the work item on: time, no closer.
	rolled := f.transitionFixture("REVIEW", false, false)
	f.step(rolled, domain.ChangeRequestStateRollback, "ROLLBACK")
	clear(rolled)
	f.execSQL(`UPDATE work_item SET updated_on = NOW() + interval '1 minute' WHERE id = $1`, rolled)

	// Closed with no recorded transition (the outbox row is gone).
	noTrail := f.transitionFixture("REVIEW", false, false)
	f.step(noTrail, domain.ChangeRequestStateClosed, "CLOSED")
	clear(noTrail)
	f.execSQL(`DELETE FROM event_outbox WHERE entity_type = 'change_request' AND entity_id = $1`, noTrail)

	// Already stamped: kept.
	kept := f.transitionFixture("REVIEW", false, false)
	f.step(kept, domain.ChangeRequestStateClosed, "CLOSED")
	f.execSQL(`UPDATE change_request SET closed_on = '2020-01-01T00:00:00Z' WHERE id = $1`, kept)

	for run := 1; run <= 2; run++ {
		// As `make migrate` runs it: a plain session, nobody flagged internal (as
		// csm_app, which has no BYPASSRLS, FORCE row-level security would hide
		// every row from an unflagged UPDATE).
		conn, err := f.pool.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(context.Background(), string(sql)); err != nil {
			conn.Release()
			t.Fatalf("run %d of 0288: %v", run, err)
		}
		var flag string
		if err := conn.QueryRow(context.Background(), `SELECT COALESCE(current_setting('app.is_internal', true), '')`).Scan(&flag); err != nil || flag != "" {
			conn.Release()
			t.Fatalf("run %d: app.is_internal = %q after 0288 (%v), want it cleared", run, flag, err)
		}
		conn.Release()
		if on, by := f.closure(closed); on == nil || !on.Equal(*wantOn) || by == nil || *by != crFlowCreatorID {
			t.Fatalf("run %d: closed: closed_on = %v (want %v), closed_by = %v", run, on, wantOn, by)
		}
		if on, by := f.closure(rolled); on == nil || by != nil {
			t.Fatalf("run %d: rolled back: closed_on = %v, closed_by = %v, want a time and no closer", run, on, by)
		}
		if on, by := f.closure(noTrail); on != nil || by != nil {
			t.Fatalf("run %d: no transition: closed_on = %v, closed_by = %v, want both empty", run, on, by)
		}
		if on, _ := f.closure(kept); on == nil || on.UTC().Year() != 2020 {
			t.Fatalf("run %d: already stamped: closed_on = %v, want 2020 kept", run, on)
		}
	}
}
