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

// The auto-closure workflow end to end on a real database: the real
// AutoclosureService over the real repositories and the real caseService, so a
// reminder is a real comment, the close is a real UpdateCase (including the
// Awaiting Info -> Closed move no staff table allows), and what is asserted is
// the rows left behind. Run as the application role (csm_app, row-level
// security enforced). Skipped unless ENTITY_TEST_DATABASE_URL is set.
//
//	ENTITY_TEST_DATABASE_URL=postgres://csm_app:...@host/db go test ./internal/service/ -run AutoclosureIntegration

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	acSysUserID   = "92100000-0000-4000-8000-0000000000a1"
	acStaffUserID = "92100000-0000-4000-8000-0000000000a2"
	acSysEmail    = "ac-system@test.local"
	acStaffEmail  = "ac-staff@test.local"
	acIDPrefix    = "92100000-0000-4000-8000-0000000000"
	acProjectOn   = "92100000-0000-4000-8000-0000000000b1" // Pending Verification switched on
	acProjectOff  = "92100000-0000-4000-8000-0000000000b2" // switched off
)

func acTicketID(n int) string { return fmt.Sprintf("%s%02d", acIDPrefix, n) }

type acEnv struct {
	t      *testing.T
	pool   *pgxpool.Pool
	ctx    context.Context
	scoped *repository.Scoped
	svc    AutoclosureService
	now    time.Time
}

func newACEnv(t *testing.T) *acEnv {
	t.Helper()
	pool := newCustomerReplyIntegrationPool(t)
	e := &acEnv{t: t, pool: pool, ctx: repository.WithSystemIdentity(context.Background()), scoped: repository.NewScoped(pool), now: time.Now().UTC()}

	cleanup := func() {
		for _, stmt := range []string{
			// pending_verification references work_item without a cascade, so its
			// rows must go first or they block the work items' own cleanup.
			`DELETE FROM pending_verification WHERE work_item_id::text LIKE '` + acIDPrefix + `%'`,
			`DELETE FROM work_item_activity WHERE work_item_id::text LIKE '` + acIDPrefix + `%'`,
			`DELETE FROM comment WHERE work_item_id::text LIKE '` + acIDPrefix + `%'`,
			`DELETE FROM work_item_watcher WHERE work_item_id::text LIKE '` + acIDPrefix + `%'`,
			`DELETE FROM work_item WHERE id::text LIKE '` + acIDPrefix + `%'`,
		} {
			_, _ = e.scoped.Exec(e.ctx, stmt)
		}
		_, _ = e.scoped.Exec(e.ctx, `DELETE FROM project WHERE id IN ($1, $2)`, acProjectOn, acProjectOff)
		_, _ = pool.Exec(e.ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, acSysUserID, acStaffUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, u := range []struct{ id, email, typ string }{{acSysUserID, acSysEmail, "SYSTEM"}, {acStaffUserID, acStaffEmail, "INTERNAL"}} {
		name := u.email
		if u.id == acSysUserID {
			name = "System (auto-closure test)"
		}
		e.exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active) VALUES ($1, $2, $2, $3, $4, $3, TRUE)`, u.id, e.now, u.email, name)
		e.exec(`UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, u.id, u.typ)
	}

	for _, pr := range []struct {
		id      string
		key     string
		enabled bool
	}{{acProjectOn, "ACVON", true}, {acProjectOff, "ACVOFF", false}} {
		e.exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, verification_enabled)
			VALUES ($1, $2, $2, 'test', 'test', $3, $3, $4)`, pr.id, e.now, pr.key, pr.enabled)
	}

	userRepo := repository.NewUserRepository(pool)
	caseSvc := NewCaseService(repository.NewCaseRepository(e.scoped), userRepo, nil, ctxIdentityAccess{}, nil)
	e.svc = NewAutoclosureService(repository.NewAutoclosureRepository(e.scoped), caseSvc, ctxIdentityAccess{}, acSysEmail)
	// As in routes.go: a case closed for inactivity is queued for the customer's own verification.
	WithPendingVerificationQueue(e.svc, repository.NewPendingVerificationRepository(e.scoped))
	return e
}

func (e *acEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.scoped.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatalf("seed (%.70s): %v", sql, err)
	}
}

func (e *acEnv) ago(days int) time.Time { return e.now.Add(-time.Duration(days) * 24 * time.Hour) }

// ticket seeds a parent "case" row. resolvedDaysAgo only matters for Solution Proposed.
func (e *acEnv) ticket(n int, state string, resolvedDaysAgo int, step *string, stateOn *time.Time) string {
	e.t.Helper()
	id := acTicketID(n)
	e.exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', $3, $3, 'autoclosure e2e', 'CASE'::work_item_type_enum)`, id, e.ago(90), fmt.Sprintf("ACE-%04d", n))
	var resolved *time.Time
	if resolvedDaysAgo > 0 {
		r := e.ago(resolvedDaysAgo)
		resolved = &r
	}
	e.exec(`INSERT INTO "case" (id, state, autoclosure_step, autoclosure_state_on, resolved_on) VALUES ($1, $2::case_state_enum, $3, $4, $5)`,
		id, state, step, stateOn, resolved)
	return id
}

func (e *acEnv) staffComment(id string, daysAgo int) {
	e.exec(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		VALUES (gen_random_uuid(), $1, $2, 'COMMENT', $3, 'please send the logs')`, e.ago(daysAgo), acStaffEmail, id)
}

func (e *acEnv) advance(id string, stage domain.AutoclosureStage) domain.AutoclosureAdvanceResponse {
	e.t.Helper()
	resp, err := e.svc.Advance(e.ctx, id, domain.AutoclosureAdvanceRequest{Stage: stage})
	if err != nil {
		e.t.Fatalf("Advance(%s): %v", stage, err)
	}
	return resp
}

type acRow struct {
	state, resolution string
	step              *string
	closedBy          *string
	closedOn          *time.Time
	closeNotes        *string
}

func (e *acEnv) row(id string) acRow {
	e.t.Helper()
	var r acRow
	var res *string
	if err := e.scoped.QueryRow(e.ctx, `SELECT state::text, resolution_code::text, autoclosure_step, closed_by_user_id::text, closed_on, close_notes FROM "case" WHERE id = $1`, id).
		Scan(&r.state, &res, &r.step, &r.closedBy, &r.closedOn, &r.closeNotes); err != nil {
		e.t.Fatalf("read ticket: %v", err)
	}
	if res != nil {
		r.resolution = *res
	}
	return r
}

func (e *acEnv) systemComments(id string) []string {
	e.t.Helper()
	rows, err := e.scoped.Query(e.ctx, `SELECT content FROM comment WHERE work_item_id = $1 AND created_by = $2 AND type = 'COMMENT' ORDER BY created_on`, id, acSysEmail)
	if err != nil {
		e.t.Fatalf("read comments: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func (e *acEnv) events(id string) []string {
	e.t.Helper()
	rows, err := e.scoped.Query(e.ctx, `SELECT stage || ':' || outcome FROM case_autoclosure_event WHERE work_item_id = $1 ORDER BY created_on, id`, id)
	if err != nil {
		e.t.Fatalf("read events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (e *acEnv) setClock(id string, step string, daysAgo int) {
	e.exec(`UPDATE "case" SET autoclosure_step = $2, autoclosure_state_on = $3 WHERE id = $1`, id, step, e.ago(daysAgo))
}

// TestAutoclosureIntegration_AwaitingInfoFullLifecycle walks one Awaiting Info
// ticket from nothing to closed, asserting at each step what is on the rows,
// and that repeating a step changes nothing.
func TestAutoclosureIntegration_AwaitingInfoFullLifecycle(t *testing.T) {
	e := newACEnv(t)
	id := e.ticket(1, "AWAITING_INFO", 0, nil, nil)
	e.staffComment(id, 9)

	// 1. First reminder.
	if r := e.advance(id, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("first reminder: %+v", r)
	}
	if got := e.row(id); got.step == nil || *got.step != "FIRST_COMMENT" || got.state != "AWAITING_INFO" {
		t.Fatalf("after first reminder: %+v", got)
	}
	if c := e.systemComments(id); len(c) != 1 {
		t.Fatalf("system comments = %d, want 1", len(c))
	}
	// Repeating it must not post a second reminder.
	if r := e.advance(id, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("repeat first reminder: %+v, want skipped", r)
	}
	if c := e.systemComments(id); len(c) != 1 {
		t.Fatalf("a repeat posted another comment: %d", len(c))
	}
	// The reminder itself does not restart the clock, and nothing is due yet.
	if r := e.advance(id, domain.AutoclosureStageSecondReminder); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("second reminder one second later: %+v, want skipped", r)
	}

	// 2. A week later: second reminder.
	e.setClock(id, "FIRST_COMMENT", 8)
	if r := e.advance(id, domain.AutoclosureStageSecondReminder); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("second reminder: %+v", r)
	}
	if got := e.row(id); got.step == nil || *got.step != "SECOND_COMMENT" {
		t.Fatalf("after second reminder: %+v", got)
	}

	// 3. A week later: close.
	e.setClock(id, "SECOND_COMMENT", 8)
	if r := e.advance(id, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("close: %+v", r)
	}
	got := e.row(id)
	if got.state != "CLOSED" {
		t.Fatalf("state = %s, want CLOSED", got.state)
	}
	if got.resolution != "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS_THROUGH_AUTO_CLOSURE" {
		t.Errorf("resolution_code = %q, want the auto-closure code", got.resolution)
	}
	if got.closedBy == nil || *got.closedBy != acSysUserID {
		t.Errorf("closed_by_user_id = %v, want the system user", got.closedBy)
	}
	if got.closedOn == nil {
		t.Errorf("closed_on not set")
	}
	if got.closeNotes == nil || *got.closeNotes == "" {
		t.Errorf("close_notes not set")
	}
	if c := e.systemComments(id); len(c) != 3 {
		t.Errorf("system comments = %d, want 3 (first, second, closure)", len(c))
	}
	if ev := e.events(id); len(ev) != 3 || ev[0] != "first_reminder:ADVANCED" || ev[1] != "second_reminder:ADVANCED" || ev[2] != "close:ADVANCED" {
		t.Errorf("audit = %v", ev)
	}
	var activity int
	if err := e.scoped.QueryRow(e.ctx, `SELECT COUNT(*) FROM work_item_activity WHERE work_item_id = $1 AND field_name = 'state' AND old_value = 'Awaiting Info' AND new_value = 'Closed' AND user_email = $2`, id, acSysEmail).Scan(&activity); err != nil {
		t.Fatal(err)
	}
	if activity != 1 {
		t.Errorf("state activity entries by the system user = %d, want 1", activity)
	}
	// A closed ticket is never a candidate again.
	if r := e.advance(id, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Errorf("closing a closed ticket: %+v, want skipped", r)
	}
}

func TestAutoclosureIntegration_SolutionProposedClosesWithoutTheAwaitingInfoCode(t *testing.T) {
	e := newACEnv(t)
	id := e.ticket(2, "SOLUTION_PROPOSED", 20, nil, nil)
	e.advance(id, domain.AutoclosureStageFirstReminder)
	e.setClock(id, "SECOND_COMMENT", 8)
	if r := e.advance(id, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("close: %+v", r)
	}
	got := e.row(id)
	if got.state != "CLOSED" || got.resolution != "" {
		t.Errorf("state=%s resolution=%q, want CLOSED with no auto-closure resolution code", got.state, got.resolution)
	}
}

func TestAutoclosureIntegration_AnOpenChildBlocksEveryStageUntilItCloses(t *testing.T) {
	e := newACEnv(t)
	parent := e.ticket(3, "SOLUTION_PROPOSED", 20, nil, nil)
	child := e.ticket(4, "OPEN", 0, nil, nil)
	e.exec(`UPDATE work_item SET parent_id = $1 WHERE id = $2`, parent, child)

	if r := e.advance(parent, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("first reminder with an open child: %+v, want skipped", r)
	}
	if c := e.systemComments(parent); len(c) != 0 {
		t.Fatalf("a parent with an open child was reminded")
	}
	e.exec(`UPDATE "case" SET state = 'CLOSED' WHERE id = $1`, child)
	if r := e.advance(parent, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("first reminder once the child is closed: %+v, want advanced", r)
	}

	// A child that opens between the listing and the close is caught by the close itself.
	e.setClock(parent, "SECOND_COMMENT", 8)
	child2 := e.ticket(5, "OPEN", 0, nil, nil)
	e.exec(`UPDATE work_item SET parent_id = $1 WHERE id = $2`, parent, child2)
	if r := e.advance(parent, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("close with an open child: %+v, want skipped", r)
	}
	if got := e.row(parent); got.state == "CLOSED" {
		t.Fatalf("a parent with an open child was closed")
	}
}

func TestAutoclosureIntegration_HoldProtectsThenReleases(t *testing.T) {
	e := newACEnv(t)
	today := time.Date(e.now.Year(), e.now.Month(), e.now.Day(), 0, 0, 0, 0, time.UTC)
	held := e.ticket(6, "SOLUTION_PROPOSED", 30, ptrOf("ON_HOLD"), ptrTime(today.AddDate(0, 0, 14)))
	over := e.ticket(7, "SOLUTION_PROPOSED", 30, ptrOf("ON_HOLD"), ptrTime(today.AddDate(0, 0, -1)))

	// While held, no stage touches it.
	for _, st := range []domain.AutoclosureStage{domain.AutoclosureStageFirstReminder, domain.AutoclosureStageSecondReminder, domain.AutoclosureStageClose, domain.AutoclosureStageReleaseHold} {
		if r := e.advance(held, st); r.Outcome != domain.AutoclosureOutcomeSkipped {
			t.Errorf("held ticket, stage %s: %+v, want skipped", st, r)
		}
	}
	if c := e.systemComments(held); len(c) != 0 {
		t.Errorf("a held ticket was commented on")
	}

	// An expired hold goes back to DEFAULT with a fresh clock -- not an instant reminder.
	if r := e.advance(over, domain.AutoclosureStageReleaseHold); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("release: %+v", r)
	}
	if got := e.row(over); got.step == nil || *got.step != "DEFAULT" {
		t.Fatalf("after release: %+v", got)
	}
	if r := e.advance(over, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Errorf("first reminder right after a release: %+v, want skipped (the clock restarted)", r)
	}
	// ...and once a full interval has passed since the release, it is reminded.
	e.setClock(over, "DEFAULT", 8)
	if r := e.advance(over, domain.AutoclosureStageFirstReminder); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Errorf("first reminder a week after the release: %+v, want advanced", r)
	}
}

func TestAutoclosureIntegration_DailyLimitHoldsAcrossRetries(t *testing.T) {
	e := newACEnv(t)
	// Far older than anything else in the database, so they are the ones the
	// oldest-first budget picks whatever else is due in it.
	ids := []string{e.ticket(8, "SOLUTION_PROPOSED", 4000, nil, nil), e.ticket(9, "SOLUTION_PROPOSED", 3900, nil, nil), e.ticket(10, "SOLUTION_PROPOSED", 3800, nil, nil)}

	list := func(daily int) int {
		resp, err := e.svc.FindCandidates(e.ctx, domain.AutoclosureCandidatesRequest{Stage: domain.AutoclosureStageFirstReminder, DailyLimit: daily})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, c := range resp.Candidates {
			for _, id := range ids {
				if c.ID == id {
					n++
				}
			}
		}
		return n
	}
	// The table may hold other tickets and other events today, so count the delta.
	base, err := repository.NewAutoclosureRepository(e.scoped).CountAdvancedSince(e.ctx, time.Date(e.now.Year(), e.now.Month(), e.now.Day(), 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	limit := base + 2
	if got := list(limit); got != 2 {
		t.Fatalf("with 2 left of the budget, %d of my 3 due tickets were offered, want the 2 oldest", got)
	}
	e.advance(ids[0], domain.AutoclosureStageFirstReminder)
	e.advance(ids[1], domain.AutoclosureStageFirstReminder)
	// A retried run the same day finds the budget spent.
	if got := list(limit); got != 0 {
		t.Fatalf("after spending the budget, %d tickets were still offered", got)
	}
}

func ptrOf(s string) *string         { return &s }
func ptrTime(t time.Time) *time.Time { return &t }

func (e *acEnv) putInProject(caseID, projectID string) {
	e.exec(`UPDATE work_item SET project_id = $2 WHERE id = $1`, caseID, projectID)
}

type acVerification struct {
	reason, previous, addedBy, note string
	verified                        bool
}

func (e *acEnv) verifications(caseID string) []acVerification {
	e.t.Helper()
	rows, err := e.scoped.Query(e.ctx, `SELECT added_reason::text, COALESCE(previous_status::text, ''), added_by, COALESCE(note, ''), verified_on IS NOT NULL FROM pending_verification WHERE work_item_id = $1 ORDER BY added_on`, caseID)
	if err != nil {
		e.t.Fatalf("read pending verifications: %v", err)
	}
	defer rows.Close()
	var out []acVerification
	for rows.Next() {
		var v acVerification
		if err := rows.Scan(&v.reason, &v.previous, &v.addedBy, &v.note, &v.verified); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// TestAutoclosureIntegration_ClosedCaseIsQueuedForCustomerVerification is the
// join with the Pending Verification feature: a case closed for inactivity lands
// in the customer's queue (reason AUTO_CLOSED, with the state it was closed
// from), but only where the project has that feature on, and never twice.
func TestAutoclosureIntegration_ClosedCaseIsQueuedForCustomerVerification(t *testing.T) {
	e := newACEnv(t)

	// Awaiting Info on a project that uses the feature.
	ai := e.ticket(20, "AWAITING_INFO", 0, ptrOf("SECOND_COMMENT"), ptrTime(e.now.Add(-8*24*time.Hour)))
	e.putInProject(ai, acProjectOn)
	if r := e.advance(ai, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("close: %+v", r)
	}
	got := e.verifications(ai)
	if len(got) != 1 || got[0].reason != "AUTO_CLOSED" || got[0].previous != "AWAITING_INFO" || got[0].verified {
		t.Fatalf("queue = %+v, want one unverified AUTO_CLOSED entry from AWAITING_INFO", got)
	}
	// "Added by" is what the customer portal prints, so it is the system user's name, not its email.
	if got[0].addedBy != "System (auto-closure test)" {
		t.Errorf("addedBy = %q, want the system user's display name", got[0].addedBy)
	}
	if !strings.Contains(got[0].note, "no response to our request for information after two reminders") {
		t.Errorf("note = %q, want the explanation for an Awaiting Info close", got[0].note)
	}
	if ev := e.events(ai); len(ev) != 1 || ev[0] != "close:ADVANCED" {
		t.Errorf("audit = %v", ev)
	}

	// Solution Proposed records its own previous state.
	sp := e.ticket(21, "SOLUTION_PROPOSED", 30, ptrOf("SECOND_COMMENT"), ptrTime(e.now.Add(-8*24*time.Hour)))
	e.putInProject(sp, acProjectOn)
	e.advance(sp, domain.AutoclosureStageClose)
	if got := e.verifications(sp); len(got) != 1 || got[0].previous != "SOLUTION_PROPOSED" {
		t.Errorf("queue = %+v, want one entry from SOLUTION_PROPOSED", got)
	}

	// A project that has not turned the feature on: closed, not queued.
	off := e.ticket(22, "AWAITING_INFO", 0, ptrOf("SECOND_COMMENT"), ptrTime(e.now.Add(-8*24*time.Hour)))
	e.putInProject(off, acProjectOff)
	e.advance(off, domain.AutoclosureStageClose)
	if row := e.row(off); row.state != "CLOSED" {
		t.Errorf("state = %s, want CLOSED even where the feature is off", row.state)
	}
	if got := e.verifications(off); len(got) != 0 {
		t.Errorf("queued on a project with the feature off: %+v", got)
	}

	// A case that already has an unverified entry is still closed, and keeps exactly one.
	dup := e.ticket(23, "AWAITING_INFO", 0, ptrOf("SECOND_COMMENT"), ptrTime(e.now.Add(-8*24*time.Hour)))
	e.putInProject(dup, acProjectOn)
	e.exec(`INSERT INTO pending_verification (work_item_id, added_reason, added_by) VALUES ($1, 'MANUAL', 'someone@example.com')`, dup)
	if r := e.advance(dup, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("close with an entry already queued: %+v", r)
	}
	if row := e.row(dup); row.state != "CLOSED" {
		t.Errorf("state = %s, want CLOSED", row.state)
	}
	if got := e.verifications(dup); len(got) != 1 || got[0].reason != "MANUAL" {
		t.Errorf("queue = %+v, want the existing single MANUAL entry untouched", got)
	}

	// No project at all: closed, nothing queued, nothing breaks.
	none := e.ticket(24, "AWAITING_INFO", 0, ptrOf("SECOND_COMMENT"), ptrTime(e.now.Add(-8*24*time.Hour)))
	if r := e.advance(none, domain.AutoclosureStageClose); r.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("close of a case with no project: %+v", r)
	}
	if got := e.verifications(none); len(got) != 0 {
		t.Errorf("queued a case with no project: %+v", got)
	}
}

// The reference-time rules join on strings the real writers produce (the state
// activity's new_value label, the comment's created_by email). Seeding those
// by hand would prove nothing, so this drives the real case service: a staff
// state change and a staff comment, then ages the rows and asserts the
// listing's reference time is the comment's (not the updated_on fallback).
func TestAutoclosureIntegration_ReferenceTimeFollowsWhatTheRealWritersRecord(t *testing.T) {
	e := newACEnv(t)
	userRepo := repository.NewUserRepository(e.pool)
	caseSvc := NewCaseService(repository.NewCaseRepository(e.scoped), userRepo, nil, ctxIdentityAccess{}, nil)
	staffCtx := repository.WithSystemIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, acStaffEmail)))

	id := e.ticket(31, "WORK_IN_PROGRESS", 0, nil, nil)
	e.exec(`UPDATE work_item SET project_id = $2 WHERE id = $1`, id, acProjectOn)

	ai := domain.CaseStateAwaitingInfo
	if _, err := caseSvc.UpdateCase(staffCtx, domain.UpdateCaseRequest{ID: id, State: &ai}); err != nil {
		t.Fatalf("staff moves the case to Awaiting Info: %v", err)
	}
	if _, err := caseSvc.CreateCaseComment(staffCtx, domain.CreateCaseCommentRequest{CaseID: id, Type: domain.CommentTypeComment, Content: "please send the logs"}); err != nil {
		t.Fatalf("staff comment: %v", err)
	}
	if _, err := caseSvc.CreateCaseComment(staffCtx, domain.CreateCaseCommentRequest{CaseID: id, Type: domain.CommentTypeWorkNote, Content: "internal only"}); err != nil {
		t.Fatalf("work note: %v", err)
	}

	// Age what the writers recorded: state entry 22 days, public comment 20, work note 10 (newer, must be ignored).
	e.exec(`UPDATE work_item_activity SET created_on = $2 WHERE work_item_id = $1 AND field_name = 'state'`, id, e.ago(22))
	e.exec(`UPDATE comment SET created_on = $2 WHERE work_item_id = $1 AND type = 'COMMENT'`, id, e.ago(20))
	e.exec(`UPDATE comment SET created_on = $2 WHERE work_item_id = $1 AND type = 'WORK_NOTE'`, id, e.ago(10))
	e.exec(`UPDATE work_item SET updated_on = $2 WHERE id = $1`, id, e.ago(90))
	e.exec(`UPDATE "case" SET autoclosure_step = NULL, autoclosure_state_on = NULL WHERE id = $1`, id)

	resp, err := e.svc.FindCandidates(e.ctx, domain.AutoclosureCandidatesRequest{Stage: domain.AutoclosureStageFirstReminder, Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range resp.Candidates {
		if c.ID != id {
			continue
		}
		want := e.ago(20)
		if d := c.ReferenceTime.Sub(want); d < -time.Minute || d > time.Minute {
			t.Fatalf("reference time = %v, want the staff comment's %v (not the 90-day fallback, not the work note)", c.ReferenceTime, want)
		}
		return
	}
	t.Fatalf("a ticket whose real comment and state change are 20+ days old was not listed")
}

// A dry run decides and records, and leaves the ticket exactly as it was.
func TestAutoclosureIntegration_DryRunLeavesTheTicketAloneAndIsNotCounted(t *testing.T) {
	e := newACEnv(t)
	id := e.ticket(32, "SOLUTION_PROPOSED", 20, nil, nil)
	e.exec(`UPDATE work_item SET project_id = $2 WHERE id = $1`, id, acProjectOn)

	resp, err := e.svc.Advance(e.ctx, id, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != domain.AutoclosureOutcomeDryRun {
		t.Fatalf("outcome = %q, want dry_run", resp.Outcome)
	}
	if r := e.row(id); r.step != nil || r.state != "SOLUTION_PROPOSED" {
		t.Errorf("a dry run changed the ticket: step=%v state=%s", r.step, r.state)
	}
	if got := e.systemComments(id); len(got) != 0 {
		t.Errorf("a dry run posted %d comment(s)", len(got))
	}
	if ev := e.events(id); len(ev) != 1 || ev[0] != "first_reminder:DRY_RUN" {
		t.Errorf("audit rows = %v, want one first_reminder:DRY_RUN", ev)
	}
	n, err := repository.NewAutoclosureRepository(e.scoped).CountAdvancedSince(e.ctx, e.now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if again := e.advance(id, domain.AutoclosureStageFirstReminder); again.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Errorf("a real run after the dry run = %q, want advanced", again.Outcome)
	}
	m, _ := repository.NewAutoclosureRepository(e.scoped).CountAdvancedSince(e.ctx, e.now.Add(-24*time.Hour))
	if m != n+1 {
		t.Errorf("advanced count went %d -> %d; the dry run must not count and the real run must (+1)", n, m)
	}
}
