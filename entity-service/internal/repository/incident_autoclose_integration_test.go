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

// Integration tests for the incident auto-close (IncidentAutocloseRepository
// and the incident service it closes through) and for the
// closed_on / closed_by_id a manual close now records (migration 0273),
// against a real Postgres with every migration applied. Skipped unless
// INCIDENT_AUTOCLOSE_TEST_DSN is set.
//
//	INCIDENT_AUTOCLOSE_TEST_DSN=postgres://... go test ./internal/repository/ -run IncidentAutoclose
//
// The sweep is database-wide, so these tests use a window no other fixture
// reaches (incidents resolved 4000 days ago, closed by resolvedForDays
// 3650) and assert on their own incidents only.
package repository_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	acDays    = 3650 // resolvedForDays every sweep here uses
	acDue     = 4000 // days ago a due incident was resolved
	acNotDue  = 3000 // days ago a not-yet-due incident was resolved
	acActorID = icCallerID
)

func incidentAutoclosePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_AUTOCLOSE_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_AUTOCLOSE_TEST_DSN not set")
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

func acCtx() context.Context { return repository.WithSystemIdentity(context.Background()) }

// acResolved creates an incident, resolves it with ic-engineer as resolver
// and backdates resolved_on and the work item's updated_on to daysAgo.
// nullResolvedOn clears resolved_on, as on synced rows that lack it.
func acResolved(t *testing.T, pool *pgxpool.Pool, daysAgo int, nullResolvedOn bool) string {
	t.Helper()
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	resp, err := repo.CreateIncident(acCtx(), icRequest(), "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	id := resp.Incident.ID
	state, code, notes, resolver := "RESOLVED", "SOLVED_PERMANENTLY", "fixed", icEngineerID
	if _, err := repo.UpdateIncidentLifecycle(acCtx(), id, repository.IncidentLifecycleUpdate{
		State: &state, ResolutionCode: &code, ResolutionNotes: &notes, ResolvedByID: &resolver,
	}, "ic-engineer@test.local"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	scoped := repository.NewScoped(pool)
	if _, err := scoped.Exec(acCtx(), `
		UPDATE incident SET resolved_on = CASE WHEN $2 THEN NULL ELSE NOW() - make_interval(days => $3::int) END
		WHERE id = $1`, id, nullResolvedOn, daysAgo); err != nil {
		t.Fatalf("backdate resolved_on: %v", err)
	}
	if _, err := scoped.Exec(acCtx(), `UPDATE work_item SET updated_on = NOW() - make_interval(days => $2::int) WHERE id = $1`, id, daysAgo); err != nil {
		t.Fatalf("backdate updated_on: %v", err)
	}
	return id
}

type acRow struct {
	state       string
	closedOn    *time.Time
	closedBy    *string
	resolvedBy  *string
	updatedBy   string
	comments    int
	activityNew []string
}

func acRead(t *testing.T, pool *pgxpool.Pool, id string) acRow {
	t.Helper()
	var r acRow
	scoped := repository.NewScoped(pool)
	if err := scoped.QueryRow(acCtx(), `
		SELECT inc.state::text, inc.closed_on, inc.closed_by_id::text, inc.resolved_by_id::text, wi.updated_by,
		       (SELECT COUNT(*) FROM comment c WHERE c.work_item_id = inc.id)
		FROM incident inc JOIN work_item wi ON wi.id = inc.id WHERE inc.id = $1`, id).
		Scan(&r.state, &r.closedOn, &r.closedBy, &r.resolvedBy, &r.updatedBy, &r.comments); err != nil {
		t.Fatalf("read incident: %v", err)
	}
	rows, err := scoped.Query(acCtx(), `SELECT new_value FROM work_item_activity WHERE work_item_id = $1 AND field_name = 'state' ORDER BY created_on`, id)
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		r.activityNew = append(r.activityNew, v)
	}
	return r
}

// acSweep runs the auto-close as routes.go wires it on the Postgres data
// source: the listing, and the incident service's own update to close, as the
// system user acSystemEmail ("" = the default "System user").
func acSweep(t *testing.T, pool *pgxpool.Pool, req domain.IncidentAutocloseRequest) domain.IncidentAutocloseResponse {
	t.Helper()
	return acSweepAs(t, pool, req, "")
}

func acSweepAs(t *testing.T, pool *pgxpool.Pool, req domain.IncidentAutocloseRequest, systemEmail string) domain.IncidentAutocloseResponse {
	t.Helper()
	scoped := repository.NewScoped(pool)
	incidents := service.NewIncidentServiceWithPublisher(repository.NewIncidentRepository(scoped), repository.NewUserRepository(pool), nil)
	svc := service.NewIncidentAutocloseService(repository.NewIncidentAutocloseRepository(scoped), incidents, rrUnrestrictedAccess{}, systemEmail)
	resp, err := svc.Autoclose(acCtx(), req)
	if err != nil {
		t.Fatalf("Autoclose: %v", err)
	}
	return resp
}

func acDaysPtr() *int { d := acDays; return &d }

func acClosedIDs(resp domain.IncidentAutocloseResponse) map[string]bool {
	out := map[string]bool{}
	for _, c := range resp.Closed {
		out[c.ID] = true
	}
	return out
}

// TestIncidentAutoclose_ClosesDueIncidentsOnly: a due RESOLVED incident is
// closed through the incident service with closed_on now, by the system user
// (the default "System user", which has no user row, so closed_by_id is NULL,
// as case auto-closure leaves closed_by_user_id), with one Resolved -> Closed
// activity row and no comment or work note; one with no close notes (synced
// rows can lack them) closes too; a not-yet-due one, and incidents in other
// states, are untouched; an incident with no resolved_on is measured from its
// last update.
func TestIncidentAutoclose_ClosesDueIncidentsOnly(t *testing.T) {
	pool := incidentAutoclosePool(t)
	due := acResolved(t, pool, acDue, false)
	notDue := acResolved(t, pool, acNotDue, false)
	nullDue := acResolved(t, pool, acDue, true)
	nullNotDue := acResolved(t, pool, acNotDue, true)
	inProgress := acResolved(t, pool, acDue, false)
	if _, err := repository.NewScoped(pool).Exec(acCtx(), `UPDATE incident SET state = 'IN_PROGRESS' WHERE id = $1`, inProgress); err != nil {
		t.Fatal(err)
	}
	noNotes := acResolved(t, pool, acDue, false)
	if _, err := repository.NewScoped(pool).Exec(acCtx(), `UPDATE incident SET close_notes = NULL WHERE id = $1`, noNotes); err != nil {
		t.Fatal(err)
	}
	before := acRead(t, pool, due)

	start := time.Now()
	resp := acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()})
	closed := acClosedIDs(resp)
	if !closed[due] || !closed[nullDue] || !closed[noNotes] || closed[notDue] || closed[nullNotDue] || closed[inProgress] || len(resp.Failed) != 0 {
		t.Fatalf("closed = %v, failed = %+v", resp.Closed, resp.Failed)
	}
	for _, c := range resp.Closed {
		if c.ID == nullDue && c.ResolvedOn != nil || c.ID == due && c.ResolvedOn == nil {
			t.Errorf("resolvedOn of %s = %v", c.Number, c.ResolvedOn)
		}
	}

	got := acRead(t, pool, due)
	if got.state != "CLOSED" || got.closedOn == nil || got.closedOn.Before(start.Add(-time.Minute)) {
		t.Errorf("due incident: state %s closed_on %v", got.state, got.closedOn)
	}
	if got.closedBy != nil || deref(got.resolvedBy) != icEngineerID {
		t.Errorf("closed_by_id = %s, resolved_by_id = %s, want NULL / %s", deref(got.closedBy), deref(got.resolvedBy), icEngineerID)
	}
	if got.updatedBy != domain.AutoclosureDefaultActor {
		t.Errorf("work_item.updated_by = %q, want the system user %q", got.updatedBy, domain.AutoclosureDefaultActor)
	}
	if got.comments != before.comments {
		t.Errorf("comments %d -> %d: auto-close must add no comment or work note", before.comments, got.comments)
	}
	if n := len(got.activityNew); n == 0 || got.activityNew[n-1] != "Closed" {
		t.Errorf("state activity = %v, want it to end with Closed", got.activityNew)
	}
	for name, id := range map[string]string{"null resolved_on": nullDue, "no close notes": noNotes} {
		if r := acRead(t, pool, id); r.state != "CLOSED" || r.closedBy != nil {
			t.Errorf("%s incident: %+v, want closed by nobody", name, r)
		}
	}
	for name, id := range map[string]string{"not due": notDue, "null not due": nullNotDue} {
		if r := acRead(t, pool, id); r.state != "RESOLVED" || r.closedOn != nil || r.closedBy != nil {
			t.Errorf("%s: %+v, want untouched", name, r)
		}
	}
	if r := acRead(t, pool, inProgress); r.state != "IN_PROGRESS" || r.closedOn != nil {
		t.Errorf("in progress: %+v, want untouched", r)
	}

	// Reading it back: the detail view carries closedOn, and no closedBy for a
	// system user with no user row.
	v, err := repository.NewIncidentRepository(repository.NewScoped(pool)).GetIncidentByID(acCtx(), due)
	if err != nil || v.ClosedOn == nil || v.ClosedBy != nil {
		t.Errorf("GetIncidentByID closedOn %v closedBy %v err %v", v.ClosedOn, v.ClosedBy, err)
	}

	// A second sweep finds nothing of ours left to close.
	again := acClosedIDs(acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()}))
	if again[due] || again[nullDue] {
		t.Errorf("second sweep closed an already-closed incident again")
	}
}

// TestIncidentAutoclose_CascadesOpenTasks: the incident's open tasks are
// closed as on a manual close (CLOSED_INCOMPLETE with the cascade's work note); a
// task already closed is left alone.
func TestIncidentAutoclose_CascadesOpenTasks(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	open, _ := addTask(t, pool, inc, strp("OPEN"))
	done, _ := addTask(t, pool, inc, strp("CLOSED_COMPLETE"))

	if !acClosedIDs(acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()}))[inc] {
		t.Fatal("incident not closed")
	}
	want := "Incident Task is Closed Incomplete based on closure of " + incidentNumber(t, pool, inc) + "."
	if r := readTask(t, pool, open); deref(r.state) != "CLOSED_INCOMPLETE" || r.active || r.closedOn == nil || len(r.workNotes) != 1 || r.workNotes[0] != want {
		t.Errorf("open task: state=%s active=%v closed_on=%v notes=%q", deref(r.state), r.active, r.closedOn, r.workNotes)
	}
	if r := readTask(t, pool, done); deref(r.state) != "CLOSED_COMPLETE" || len(r.workNotes) != 0 {
		t.Errorf("closed task changed: state=%s notes=%q", deref(r.state), r.workNotes)
	}
}

// TestIncidentAutoclose_DryRunChangesNothing: a dry run lists the due
// incident and leaves it, its tasks and its activity as they were.
func TestIncidentAutoclose_DryRunChangesNothing(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	task, _ := addTask(t, pool, inc, strp("OPEN"))
	before := acRead(t, pool, inc)

	resp := acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr(), DryRun: true})
	if !resp.DryRun || !acClosedIDs(resp)[inc] {
		t.Fatalf("dry run did not list the due incident: %+v", resp)
	}
	after := acRead(t, pool, inc)
	if after.state != "RESOLVED" || after.closedOn != nil || after.updatedBy != before.updatedBy || len(after.activityNew) != len(before.activityNew) {
		t.Errorf("dry run changed the incident: %+v -> %+v", before, after)
	}
	if r := readTask(t, pool, task); deref(r.state) != "OPEN" || !r.active {
		t.Errorf("dry run changed the task: %s", deref(r.state))
	}
}

// TestIncidentAutoclose_LimitAndMore: limit caps a call, oldest first, and
// more says whether the page was full; calling again picks up the rest.
func TestIncidentAutoclose_LimitAndMore(t *testing.T) {
	pool := incidentAutoclosePool(t)
	oldest := acResolved(t, pool, acDue+2, false)
	middle := acResolved(t, pool, acDue+1, false)
	newest := acResolved(t, pool, acDue, false)

	first := acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr(), Limit: 2})
	if len(first.Closed) != 2 || !first.More || first.Closed[0].ID != oldest || first.Closed[1].ID != middle {
		t.Fatalf("first page = %+v more=%v, want oldest, middle and more", first.Closed, first.More)
	}
	if acRead(t, pool, newest).state != "RESOLVED" {
		t.Fatal("the third incident was closed beyond the limit")
	}
	second := acSweep(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr(), Limit: 2})
	if !acClosedIDs(second)[newest] || second.More {
		t.Fatalf("second page = %+v more=%v, want newest and no more", second.Closed, second.More)
	}
}

// TestIncidentAutoclose_SystemUserWithARow: with AUTOCLOSURE_SYSTEM_USER_EMAIL
// naming a user that has a row, the close is attributed to that user:
// closed_by_id is theirs and updated_by their email.
func TestIncidentAutoclose_SystemUserWithARow(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	acSweepAs(t, pool, domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()}, "ic-caller@test.local")
	if r := acRead(t, pool, inc); r.state != "CLOSED" || deref(r.closedBy) != acActorID || r.updatedBy != "ic-caller@test.local" {
		t.Fatalf("closed by %s, updated_by %q, state %s; want %s / ic-caller@test.local / CLOSED", deref(r.closedBy), r.updatedBy, r.state, acActorID)
	}
}

// TestIncidentAutoclose_RecheckSkipsAnIncidentNoLongerDue: an incident
// re-opened after it was listed is left alone by the auto-close update: under
// the row lock it is no longer due, and nothing is written.
func TestIncidentAutoclose_RecheckSkipsAnIncidentNoLongerDue(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	if _, err := repository.NewScoped(pool).Exec(acCtx(), `UPDATE incident SET state = 'IN_PROGRESS' WHERE id = $1`, inc); err != nil {
		t.Fatal(err)
	}
	closedState, days := "CLOSED", acDays
	_, err := repo.UpdateIncidentLifecycle(acCtx(), inc, repository.IncidentLifecycleUpdate{State: &closedState, AutocloseDueDays: &days}, "System user")
	if !errors.Is(err, repository.ErrIncidentNotDueForAutoclose) {
		t.Fatalf("err = %v, want ErrIncidentNotDueForAutoclose", err)
	}
	if r := acRead(t, pool, inc); r.state != "IN_PROGRESS" || r.closedOn != nil || r.updatedBy == "System user" {
		t.Errorf("incident changed: %+v", r)
	}
}

// TestIncidentManualClose_RecordsClosedOnAndBy: closing by PATCH stamps
// closed_on and closed_by_id with the acting user; a re-open keeps them; a
// close with no acting user records closed_by_id NULL.
func TestIncidentManualClose_RecordsClosedOnAndBy(t *testing.T) {
	pool := incidentAutoclosePool(t)
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	inc := acResolved(t, pool, 1, false)

	closedState, actor := "CLOSED", acActorID
	if _, err := repo.UpdateIncidentLifecycle(acCtx(), inc, repository.IncidentLifecycleUpdate{State: &closedState, DefaultResolvedByID: &actor}, "ic-caller@test.local"); err != nil {
		t.Fatalf("close: %v", err)
	}
	first := acRead(t, pool, inc)
	if first.closedOn == nil || deref(first.closedBy) != acActorID {
		t.Fatalf("after close: closed_on %v closed_by %s, want set / %s", first.closedOn, deref(first.closedBy), acActorID)
	}
	v, err := repo.GetIncidentByID(acCtx(), inc)
	if err != nil || v.ClosedOn == nil || v.ClosedBy == nil {
		t.Errorf("detail closedOn %v closedBy %v err %v", v.ClosedOn, v.ClosedBy, err)
	}

	reopen := "IN_PROGRESS"
	if _, err := repo.UpdateIncidentLifecycle(acCtx(), inc, repository.IncidentLifecycleUpdate{State: &reopen}, "ic-caller@test.local"); err != nil {
		t.Fatalf("re-open: %v", err)
	}
	if r := acRead(t, pool, inc); r.closedOn == nil || !r.closedOn.Equal(*first.closedOn) || deref(r.closedBy) != acActorID {
		t.Errorf("re-open cleared the close stamps: %+v", r)
	}

	// Closing again with nobody to attribute it to.
	if _, err := repo.UpdateIncidentLifecycle(acCtx(), inc, repository.IncidentLifecycleUpdate{State: &closedState}, "system-m2m@wso2.com"); err != nil {
		t.Fatalf("close again: %v", err)
	}
	if r := acRead(t, pool, inc); r.closedOn == nil || r.closedBy != nil {
		t.Errorf("close with no actor: closed_on %v closed_by %s, want set / NULL", r.closedOn, deref(r.closedBy))
	}

	// A save of an already-closed incident is not a new close.
	stamped := acRead(t, pool, inc).closedOn
	if _, err := repo.UpdateIncidentLifecycle(acCtx(), inc, repository.IncidentLifecycleUpdate{State: &closedState, DefaultResolvedByID: &actor}, "ic-caller@test.local"); err != nil {
		t.Fatalf("resend close: %v", err)
	}
	if r := acRead(t, pool, inc); !r.closedOn.Equal(*stamped) || r.closedBy != nil {
		t.Errorf("resent close restamped: %+v", r)
	}
}

// acMirror stands in for the backing data source's incident service under
// DATA_SOURCE=postgres-servicenow-dual-write: it records each mirrored update
// and whether it carried a user token, and fails every call with failWith.
type acMirror struct {
	service.IncidentService
	mu       sync.Mutex
	reqs     []domain.UpdateIncidentRequest
	tokens   []string
	failWith error
}

func (m *acMirror) UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs = append(m.reqs, req)
	m.tokens = append(m.tokens, middleware.UserIDTokenFromContext(ctx))
	return domain.UpdateIncidentResponse{}, m.failWith
}

func (m *acMirror) snapshot() ([]domain.UpdateIncidentRequest, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.UpdateIncidentRequest(nil), m.reqs...), append([]string(nil), m.tokens...)
}

// acFailures records the write-back failures the dispatcher reports.
type acFailures struct {
	mu    sync.Mutex
	calls []domain.CreateSNWritebackFailureRequest
}

func (f *acFailures) Create(_ context.Context, req domain.CreateSNWritebackFailureRequest) (domain.SNWritebackFailure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	return domain.SNWritebackFailure{ID: "f1"}, nil
}

func (f *acFailures) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// acDualWriteService wires the auto-close as routes.go does under dual-write:
// the incident service with a mirror and a write-back dispatcher.
func acDualWriteService(pool *pgxpool.Pool, failWith error) (service.IncidentAutocloseService, *acMirror, *acFailures) {
	scoped := repository.NewScoped(pool)
	mirror := &acMirror{failWith: failWith}
	failures := &acFailures{}
	incidents := service.NewIncidentServiceWithSNMirror(repository.NewIncidentRepository(scoped), repository.NewUserRepository(pool),
		mirror, nil, service.NewSNWritebackDispatcher(failures))
	return service.NewIncidentAutocloseService(repository.NewIncidentAutocloseRepository(scoped), incidents, rrUnrestrictedAccess{}, ""), mirror, failures
}

func acWaitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

// TestIncidentAutoclose_DualWriteMirrorsTheClose: under dual-write the close is
// written back like any other incident update, as case auto-closure's is: the
// mirror gets exactly {state: closed} for the incident, under no user token (so
// as the integration's own user), and no write-back failure is recorded.
func TestIncidentAutoclose_DualWriteMirrorsTheClose(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	svc, mirror, failures := acDualWriteService(pool, nil)

	resp, err := svc.Autoclose(acCtx(), domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()})
	if err != nil {
		t.Fatalf("Autoclose: %v", err)
	}
	if !acClosedIDs(resp)[inc] {
		t.Fatalf("closed = %+v, want the due incident", resp.Closed)
	}
	if r := acRead(t, pool, inc); r.state != "CLOSED" {
		t.Fatalf("Postgres state = %s, want CLOSED", r.state)
	}
	var mine []domain.UpdateIncidentRequest
	var tokens []string
	acWaitFor(t, func() bool {
		reqs, toks := mirror.snapshot()
		mine, tokens = nil, nil
		for i, r := range reqs {
			if r.ID == inc {
				mine, tokens = append(mine, r), append(tokens, toks[i])
			}
		}
		return len(mine) == 1
	})
	got := mine[0]
	if got.State == nil || *got.State != domain.IncidentStateClosed {
		t.Errorf("mirrored state = %v, want CLOSED", got.State)
	}
	got.ID, got.State = "", nil
	if !reflect.DeepEqual(got, domain.UpdateIncidentRequest{}) {
		t.Errorf("mirrored request carries more than the state: %+v", got)
	}
	if tokens[0] != "" {
		t.Errorf("the mirrored close carried a user token; want none (the integration's own user)")
	}
	time.Sleep(50 * time.Millisecond)
	if n := failures.count(); n != 0 {
		t.Errorf("write-back failures = %d, want none", n)
	}
}

// TestIncidentAutoclose_DualWriteMirrorFailureNeitherFailsNorRepeats: a mirror
// that fails does not fail the close, and the next run does not close (or
// mirror) the incident again: Postgres is the record, and the failure is queued
// for replay, as for case auto-closure.
func TestIncidentAutoclose_DualWriteMirrorFailureNeitherFailsNorRepeats(t *testing.T) {
	pool := incidentAutoclosePool(t)
	inc := acResolved(t, pool, acDue, false)
	svc, mirror, failures := acDualWriteService(pool, errors.New("the backing data source is down"))

	resp, err := svc.Autoclose(acCtx(), domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()})
	if err != nil {
		t.Fatalf("Autoclose: %v", err)
	}
	if !acClosedIDs(resp)[inc] {
		t.Fatalf("closed = %+v, want the due incident closed although the mirror fails", resp.Closed)
	}
	for _, f := range resp.Failed {
		if f.ID == inc {
			t.Fatalf("the incident is reported failed: %+v", f)
		}
	}
	acWaitFor(t, func() bool { return failures.count() >= 1 })

	again, err := svc.Autoclose(acCtx(), domain.IncidentAutocloseRequest{ResolvedForDays: acDaysPtr()})
	if err != nil {
		t.Fatalf("second Autoclose: %v", err)
	}
	if acClosedIDs(again)[inc] {
		t.Fatalf("the second run closed the incident again")
	}
	time.Sleep(50 * time.Millisecond)
	reqs, _ := mirror.snapshot()
	n := 0
	for _, r := range reqs {
		if r.ID == inc {
			n++
		}
	}
	if n != 1 {
		t.Errorf("mirrored closes of the incident = %d, want 1", n)
	}
	if r := acRead(t, pool, inc); r.state != "CLOSED" {
		t.Errorf("Postgres state = %s, want CLOSED", r.state)
	}
}
