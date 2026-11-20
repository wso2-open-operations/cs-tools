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
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The product-issue SLA engine end to end against PostgreSQL with the three
// policies migration 0307 seeds. Work items, events and clocks are inserted
// with fixed instants, "now" is injected, and the stored sla rows are asserted
// exactly. Skipped without ENTITY_TEST_DATABASE_URL.

const piSLAWorkItemID = "5a1a0000-0000-4000-8000-0000000000a1"

const piH = 3600.0

type piSLAEnv struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	svc  *slaEngineService
	now  time.Time
	seq  int
}

func newPISLAEnv(t *testing.T) *piSLAEnv {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	e := &piSLAEnv{t: t, pool: pool, ctx: repository.WithSystemIdentity(context.Background())}
	var n int
	if err := pool.QueryRow(e.ctx, `SELECT count(*) FROM sla_policy WHERE applies_to = 'PRODUCT_ISSUE' AND name LIKE 'Product Issue P%'`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("migration 0307 policies not applied (count %d, err %v)", n, err)
	}
	e.svc = newSLAEngineService(repository.NewSLAEngineRepository(repository.NewScoped(pool), ""), func() time.Time { return e.now })
	e.cleanup()
	t.Cleanup(e.cleanup)
	return e
}

func (e *piSLAEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatalf("exec failed: %v\n%s", err, sql)
	}
}

func (e *piSLAEnv) cleanup() {
	_, _ = e.pool.Exec(context.Background(), `DELETE FROM sla WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	_, _ = e.pool.Exec(context.Background(), `DELETE FROM work_item WHERE id = $1::uuid`, piSLAWorkItemID) // cascades to the product issue rows
}

// seed creates the product issue (work item, product_issue, github_issue_link)
// created at `created`, with the given priority (nil = none).
func (e *piSLAEnv) seed(priority *string, created time.Time) {
	e.t.Helper()
	e.cleanup()
	e.exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
	        VALUES ($1::uuid, $2, $2, 'sla-pi-test', 'sla-pi-test', next_work_item_number('PRODUCT_ISSUE'), 'sla product issue test', 'PRODUCT_ISSUE')`,
		piSLAWorkItemID, created.Add(-time.Hour)) // work_item.created_on differs on purpose: github_created_on wins
	e.exec(`INSERT INTO product_issue (id, priority) VALUES ($1::uuid, $2)`, piSLAWorkItemID, priority)
	e.exec(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, github_created_on)
	        VALUES ($1::uuid, 'sla-pi-test-org', 'sla-pi-test-repo', 9001, $2)`, piSLAWorkItemID, created)
	e.seq = 0
}

func piP(s string) *string { return &s }

type piEvent struct {
	to, waits, raw string // raw "" = a GitHub close/reopen event
	at             time.Time
}

func piBoardEv(to string, at time.Time) piEvent {
	waits := "PRODUCT_TEAM"
	switch to {
	case "WAITING_ON_CS":
		waits = "CS"
	case "RESOLVED":
		waits = "NOBODY"
	}
	raw := map[string]string{"OPEN": "Open", "IN_PROGRESS": "In Progress", "WAITING_ON_PRODUCT_TEAM": "WOW", "WAITING_ON_CS": "WOC", "RESOLVED": "Resolved", "REOPENED": "Reopened"}[to]
	return piEvent{to: to, waits: waits, raw: raw, at: at}
}

func piGHClose(at time.Time) piEvent  { return piEvent{to: "CLOSED", waits: "NOBODY", at: at} }
func piGHReopen(at time.Time) piEvent { return piEvent{to: "REOPENED", waits: "PRODUCT_TEAM", at: at} }

// deliver stores the event the way the webhook does (link columns move only for
// the newest event of their kind) and runs the engine with newest = the event
// is not older than any stored event.
func (e *piSLAEnv) deliver(ev piEvent) error {
	e.t.Helper()
	e.seq++
	if e.now.Before(ev.at) {
		e.now = ev.at // an event is never processed before it happened
	}
	var newestAll, newestOfKind bool
	if err := e.pool.QueryRow(e.ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM work_item_status_event WHERE work_item_id = $1::uuid AND occurred_at > $2),
		       NOT EXISTS (SELECT 1 FROM work_item_status_event WHERE work_item_id = $1::uuid AND occurred_at > $2
		                     AND (raw_status IS NOT NULL) = $3)`, piSLAWorkItemID, ev.at, ev.raw != "").Scan(&newestAll, &newestOfKind); err != nil {
		e.t.Fatal(err)
	}
	var rawArg any
	if ev.raw != "" {
		rawArg = ev.raw
	}
	e.exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
	        VALUES ($1::uuid, $2, $3, $4, 'WEBHOOK', $5, $6)`,
		piSLAWorkItemID, ev.to, ev.waits, ev.at, "sla-pi-test:"+strconv.Itoa(e.seq), rawArg)
	if newestOfKind {
		if ev.raw != "" {
			e.exec(`UPDATE github_issue_link SET board_status = $2, board_status_raw = $3 WHERE work_item_id = $1::uuid`, piSLAWorkItemID, ev.to, ev.raw)
		} else {
			state := "OPEN"
			if ev.to == "CLOSED" {
				state = "CLOSED"
			}
			e.exec(`UPDATE github_issue_link SET github_state = $2 WHERE work_item_id = $1::uuid`, piSLAWorkItemID, state)
		}
	}
	return e.svc.ApplyWorkItemStatusEffects(e.ctx, piSLAWorkItemID, ev.at, newestAll)
}

func (e *piSLAEnv) mustDeliver(ev piEvent) {
	e.t.Helper()
	if err := e.deliver(ev); err != nil {
		e.t.Fatalf("apply %s at %s: %v", ev.to, ev.at, err)
	}
}

func (e *piSLAEnv) register() {
	e.t.Helper()
	if err := e.svc.RegisterWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		e.t.Fatalf("register clocks: %v", err)
	}
}

// piRow is one stored sla row, in seconds.
type piRow struct {
	policy                 string
	stage                  string
	startOn                time.Time
	pausedOn, endOn        *time.Time
	pause, bpause, elapsed float64
	remaining, pct         float64
	breached               bool
	schedule               string
	isActive               bool
}

// rows returns every product-issue sla row of the work item, oldest first.
func (e *piSLAEnv) rows() []piRow {
	e.t.Helper()
	rs, err := e.pool.Query(e.ctx, `
		SELECT sp.name, s.stage::TEXT, s.start_on, s.paused_on, s.end_on,
		       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM s.business_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM s.remaining_business_duration), 0)::float8,
		       COALESCE(s.business_elapsed_percentage, 0)::float8, COALESCE(s.has_breached, FALSE),
		       COALESCE(s.schedule, ''), COALESCE(s.is_active, FALSE)
		  FROM sla s JOIN sla_policy sp ON sp.id = s.sla_policy_id
		 WHERE s.work_item_id = $1::uuid ORDER BY s.created_on, s.id`, piSLAWorkItemID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rs.Close()
	var out []piRow
	for rs.Next() {
		var r piRow
		if err := rs.Scan(&r.policy, &r.stage, &r.startOn, &r.pausedOn, &r.endOn, &r.pause, &r.bpause, &r.elapsed, &r.remaining, &r.pct, &r.breached, &r.schedule, &r.isActive); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func (e *piSLAEnv) onlyRow() piRow {
	e.t.Helper()
	rs := e.rows()
	if len(rs) != 1 {
		e.t.Fatalf("want exactly one sla row, got %d: %+v", len(rs), rs)
	}
	return rs[0]
}

func piNear(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func timePtrEq(a *time.Time, b time.Time) bool { return a != nil && a.Equal(b) }

// piWant is a fully specified expected row; nil times mean "must be NULL".
type piWant struct {
	policy, stage                     string
	start                             time.Time
	paused, end                       *time.Time
	pause, bpause, elapsed, remaining float64
	pct                               float64
	breached                          bool
}

func piTP(t time.Time) *time.Time { return &t }

func (r piRow) check(t *testing.T, label string, w piWant) {
	t.Helper()
	okTime := func(got, want *time.Time) bool {
		if want == nil {
			return got == nil
		}
		return got != nil && got.Equal(*want)
	}
	if r.policy != w.policy || r.stage != w.stage || !r.startOn.Equal(w.start) || !okTime(r.pausedOn, w.paused) || !okTime(r.endOn, w.end) ||
		!piNear(r.pause, w.pause) || !piNear(r.bpause, w.bpause) || !piNear(r.elapsed, w.elapsed) || !piNear(r.remaining, w.remaining) ||
		!piNear(r.pct, w.pct) || r.breached != w.breached {
		t.Errorf("%s:\n got  %+v\n want %+v", label, r, w)
	}
	if r.isActive != (w.stage != "ACHIEVED" && w.stage != "CANCELLED") {
		t.Errorf("%s: is_active = %v for stage %s", label, r.isActive, w.stage)
	}
}

func TestProductIssueSLAIntegration_LifecycleP1(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0) // Friday
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	if r := e.onlyRow(); r.stage != "IN_PROGRESS" || !r.startOn.Equal(created) || r.schedule != "CS 24x7" {
		t.Fatalf("registered row = %+v", r)
	}

	e.now = piAt(6, 12, 0)
	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 12, 0)))
	e.now = piAt(6, 14, 0)
	e.mustDeliver(piBoardEv("WAITING_ON_CS", piAt(6, 14, 0)))
	r := e.onlyRow()
	r.check(t, "paused", piWant{policy: "Product Issue P1", stage: "PAUSED", start: created, paused: piTP(piAt(6, 14, 0)), elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})

	e.mustDeliver(piBoardEv("WAITING_ON_PRODUCT_TEAM", piAt(7, 2, 0))) // Saturday: continuous clock
	e.onlyRow().check(t, "resumed", piWant{policy: "Product Issue P1", stage: "IN_PROGRESS", start: created, pause: 12 * piH, bpause: 12 * piH, elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})

	e.mustDeliver(piBoardEv("RESOLVED", piAt(7, 8, 0)))
	e.onlyRow().check(t, "resolved", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(7, 8, 0)),
		pause: 12 * piH, bpause: 12 * piH, elapsed: 10 * piH, remaining: 14 * piH, pct: 10.0 / 24 * 100})

	// A rebuild from history says the same.
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	e.onlyRow().check(t, "rebuilt", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(7, 8, 0)),
		pause: 12 * piH, bpause: 12 * piH, elapsed: 10 * piH, remaining: 14 * piH, pct: 10.0 / 24 * 100})
}

func TestProductIssueSLAIntegration_LifecycleP2_WeekendAnd21ISTBoundary(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(6, 15, 0) // Friday 15:00 IST
	e.seed(piP("P2"), created)
	e.now = created
	e.register()
	if r := e.onlyRow(); r.schedule != "CS 12x5 IST" || !r.startOn.Equal(created) {
		t.Fatalf("registered row = %+v", r)
	}
	e.mustDeliver(piBoardEv("IN_PROGRESS", piIST(6, 17, 0)))
	e.mustDeliver(piBoardEv("WAITING_ON_CS", piIST(6, 19, 0)))
	// Paused Friday 19:00 IST: business 15-19 = 4h consumed.
	e.onlyRow().check(t, "paused", piWant{policy: "Product Issue P2", stage: "PAUSED", start: created, paused: piTP(piIST(6, 19, 0)), elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})
	e.mustDeliver(piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 11, 0))) // Monday 11:00 IST
	// Wall pause Fri 19:00 -> Mon 11:00 = 64h; business pause Fri 19-21 + Mon 9-11 = 4h.
	e.onlyRow().check(t, "resumed", piWant{policy: "Product Issue P2", stage: "IN_PROGRESS", start: created, pause: 64 * piH, bpause: 4 * piH, elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})
	e.mustDeliver(piBoardEv("RESOLVED", piIST(9, 22, 0))) // Monday 22:00 IST, after the 21:00 close
	// Business Fri 15-21 = 6h + Mon 9-21 = 12h = 18h; minus the 4h pause = 14h.
	e.onlyRow().check(t, "resolved", piWant{policy: "Product Issue P2", stage: "ACHIEVED", start: created, end: piTP(piIST(9, 22, 0)),
		pause: 64 * piH, bpause: 4 * piH, elapsed: 14 * piH, remaining: 10 * piH, pct: 14.0 / 24 * 100})
}

func TestProductIssueSLAIntegration_LifecycleP3_Breach(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(2, 10, 0) // Monday 10:00 IST
	e.seed(piP("P3"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("IN_PROGRESS", piIST(2, 11, 0)))
	e.mustDeliver(piBoardEv("WAITING_ON_CS", piIST(2, 14, 0)))
	e.mustDeliver(piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(3, 10, 0)))
	e.mustDeliver(piBoardEv("RESOLVED", piIST(6, 22, 0))) // Friday 22:00 IST
	// Business: Mon 11h + Tue-Thu 36h + Fri 12h = 59h; pause Mon 14-21 (7h) + Tue 9-10 (1h) = 8h; consumed 51h of 48h.
	e.onlyRow().check(t, "breached", piWant{policy: "Product Issue P3", stage: "ACHIEVED", start: created, end: piTP(piIST(6, 22, 0)),
		pause: 20 * piH, bpause: 8 * piH, elapsed: 51 * piH, remaining: 0, pct: 51.0 / 48 * 100, breached: true})
}

func TestProductIssueSLAIntegration_PriorityChange(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(2, 10, 0) // Monday 10:00 IST
	e.seed(piP("P2"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("WAITING_ON_CS", piIST(2, 12, 0)))
	e.mustDeliver(piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(3, 10, 0)))

	at := piIST(3, 14, 0)
	e.now = piIST(3, 16, 0)
	e.exec(`UPDATE product_issue SET priority = 'P1' WHERE id = $1::uuid`, piSLAWorkItemID)
	if err := e.svc.ReviseWorkItemClocks(e.ctx, piSLAWorkItemID, at); err != nil {
		t.Fatal(err)
	}
	rs := e.rows()
	if len(rs) != 2 {
		t.Fatalf("want 2 rows, got %+v", rs)
	}
	// The P2 clock, cancelled at the change: business Mon 11h + Tue 5h = 16h, pause 10h.
	rs[0].check(t, "old P2", piWant{policy: "Product Issue P2", stage: "CANCELLED", start: created, end: piTP(at),
		pause: 22 * piH, bpause: 10 * piH, elapsed: 6 * piH, remaining: 18 * piH, pct: 25})
	// The new P1 clock: back-dated to creation, pause rebuilt from history (WOC Mon 12:00 -> Tue 10:00 IST = 22h wall).
	rs[1].check(t, "new P1", piWant{policy: "Product Issue P1", stage: "IN_PROGRESS", start: created,
		pause: 22 * piH, bpause: 22 * piH, elapsed: 8 * piH, remaining: 16 * piH, pct: 100.0 / 3})
	// Same call again: nothing new.
	if err := e.svc.ReviseWorkItemClocks(e.ctx, piSLAWorkItemID, at); err != nil {
		t.Fatal(err)
	}
	if got := len(e.rows()); got != 2 {
		t.Errorf("a second revise left %d rows", got)
	}
}

// reopen scenarios: a P1 issue resolved Friday 14:00Z, back in progress Saturday 10:00Z.
func piCheckReopened(t *testing.T, e *piSLAEnv, created time.Time) {
	t.Helper()
	rs := e.rows()
	if len(rs) != 2 {
		t.Fatalf("want the earlier ACHIEVED row and a new one, got %+v", rs)
	}
	rs[0].check(t, "earlier clock stays", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(6, 14, 0)),
		elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})
	// 26h wall from creation to now (Saturday 12:00Z) minus the 20h resolved interval.
	rs[1].check(t, "new clock", piWant{policy: "Product Issue P1", stage: "IN_PROGRESS", start: created,
		pause: 20 * piH, bpause: 20 * piH, elapsed: 6 * piH, remaining: 18 * piH, pct: 25})
}

func TestProductIssueSLAIntegration_ReopenBoard(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.mustDeliver(piBoardEv("RESOLVED", piAt(6, 14, 0)))
	e.now = piAt(7, 12, 0)
	e.mustDeliver(piBoardEv("REOPENED", piAt(7, 10, 0)))
	piCheckReopened(t, e, created)
	// Applying the reopening again changes nothing.
	e.mustDeliver(piBoardEv("REOPENED", piAt(7, 10, 0)))
	piCheckReopened(t, e, created)
}

func TestProductIssueSLAIntegration_ReopenGitHubCloseThenReopen(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.mustDeliver(piGHClose(piAt(6, 14, 0)))
	if r := e.onlyRow(); r.stage != "ACHIEVED" || !timePtrEq(r.endOn, piAt(6, 14, 0)) {
		t.Fatalf("a GitHub close must stop the clock at the close: %+v", r)
	}
	e.now = piAt(7, 12, 0)
	e.mustDeliver(piGHReopen(piAt(7, 10, 0)))
	piCheckReopened(t, e, created)
}

func TestProductIssueSLAIntegration_IdempotentEvents(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("WAITING_ON_CS", piAt(6, 14, 0)))
	before := e.rows()
	// The newest event's inline effect again, at a later processing time.
	e.now = piAt(9, 0, 0)
	if err := e.svc.ApplyWorkItemStatusEffects(e.ctx, piSLAWorkItemID, piAt(6, 14, 0), true); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	after := e.rows()
	if len(after) != 1 || len(before) != 1 {
		t.Fatalf("rows before %d after %d", len(before), len(after))
	}
	a, b := before[0], after[0]
	if a.stage != b.stage || !a.startOn.Equal(b.startOn) || !timePtrEq(b.pausedOn, *a.pausedOn) || a.pause != b.pause || a.bpause != b.bpause ||
		!piNear(a.elapsed, b.elapsed) || a.breached != b.breached {
		t.Errorf("a repeated event changed the clock:\n before %+v\n after  %+v", a, b)
	}
	// Resolved, then the same resolve again.
	e.mustDeliver(piBoardEv("RESOLVED", piAt(7, 8, 0)))
	done := e.onlyRow()
	e.mustDeliver(piBoardEv("RESOLVED", piAt(7, 8, 0)))
	again := e.onlyRow()
	if done.stage != "ACHIEVED" || again.stage != "ACHIEVED" || !again.endOn.Equal(*done.endOn) || again.pause != done.pause || !piNear(again.elapsed, done.elapsed) {
		t.Errorf("repeated resolve changed the clock:\n %+v\n %+v", done, again)
	}
}

func TestProductIssueSLAIntegration_NoPriorityNoClock(t *testing.T) {
	for name, priority := range map[string]*string{"none": nil, "unrecognised label": piP("Priority/High"), "P4": piP("P4")} {
		t.Run(name, func(t *testing.T) {
			e := newPISLAEnv(t)
			e.seed(priority, piAt(6, 10, 0))
			e.now = piAt(6, 10, 0)
			e.register()
			e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
			e.mustDeliver(piBoardEv("WAITING_ON_CS", piAt(6, 12, 0)))
			if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
				t.Fatal(err)
			}
			if rs := e.rows(); len(rs) != 0 {
				t.Errorf("want no clock, got %+v", rs)
			}
		})
	}
}

type piOutOfOrderScenario struct {
	name   string
	events []piEvent
	// oneRow: the lifecycle has no close/reopen, so every order ends in exactly
	// one clock. With a GitHub close in the middle, in-order delivery leaves an
	// earlier ACHIEVED row plus a new one while a shuffled delivery can leave
	// one rebuilt row, so only the newest row is compared.
	oneRow bool
}

func piSameClock(a, b piRow) bool {
	if a.stage != b.stage || !a.startOn.Equal(b.startOn) || a.pause != b.pause || a.bpause != b.bpause || a.breached != b.breached ||
		!piNear(a.elapsed, b.elapsed) || !piNear(a.pct, b.pct) {
		return false
	}
	for _, p := range [][2]*time.Time{{a.pausedOn, b.pausedOn}, {a.endOn, b.endOn}} {
		if (p[0] == nil) != (p[1] == nil) || (p[0] != nil && !p[0].Equal(*p[1])) {
			return false
		}
	}
	return true
}

// TestProductIssueSLAIntegration_OutOfOrderEqualsInOrder delivers the same events
// in shuffled orders; the stored clock must equal the in-order result.
func TestProductIssueSLAIntegration_OutOfOrderEqualsInOrder(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(6, 15, 0) // Friday 15:00 IST, P2
	scenarios := []piOutOfOrderScenario{
		{"resolved at the end", []piEvent{
			piBoardEv("IN_PROGRESS", piIST(6, 17, 0)), piBoardEv("WAITING_ON_CS", piIST(6, 19, 0)),
			piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 11, 0)), piBoardEv("WAITING_ON_CS", piIST(9, 13, 0)),
			piBoardEv("IN_PROGRESS", piIST(10, 10, 0)), piBoardEv("RESOLVED", piIST(10, 22, 0)),
		}, true},
		{"paused at the end", []piEvent{
			piBoardEv("IN_PROGRESS", piIST(6, 17, 0)), piBoardEv("WAITING_ON_CS", piIST(6, 19, 0)),
			piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 11, 0)), piBoardEv("WAITING_ON_CS", piIST(10, 13, 0)),
			piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(10, 15, 0)), piBoardEv("WAITING_ON_CS", piIST(10, 16, 0)),
		}, true},
		{"GitHub close and reopen interleaved with board moves", []piEvent{
			piBoardEv("IN_PROGRESS", piIST(6, 17, 0)), piGHClose(piIST(6, 20, 0)), piBoardEv("WAITING_ON_CS", piIST(9, 10, 0)),
			piGHReopen(piIST(9, 12, 0)), piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 15, 0)), piBoardEv("RESOLVED", piIST(10, 12, 0)),
		}, false},
	}
	orders := [][]int{
		{0, 1, 2, 3, 4, 5}, // in order
		{5, 4, 3, 2, 1, 0},
		{2, 0, 4, 1, 5, 3},
		{1, 3, 5, 0, 2, 4},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			var reference piRow
			for oi, order := range orders {
				e.seed(piP("P2"), created)
				e.now = created
				e.register()
				e.now = piIST(12, 12, 0) // processing happens well after every event
				for _, i := range order {
					e.mustDeliver(sc.events[i])
				}
				rows := e.rows()
				if sc.oneRow && len(rows) != 1 {
					t.Fatalf("order %v: want one row, got %+v", order, rows)
				}
				got := rows[len(rows)-1]
				if oi == 0 {
					reference = got
					continue
				}
				if !piSameClock(got, reference) {
					t.Errorf("order %v differs from in-order:\n got  %+v\n want %+v", order, got, reference)
				}
			}
		})
	}
}

// TestProductIssueSLAIntegration_OutOfOrderKnownResult: shuffled deliveries end
// in the exact hand-computed state, with no explicit recalculation.
func TestProductIssueSLAIntegration_OutOfOrderKnownResult(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(6, 15, 0)
	events := []piEvent{
		piBoardEv("IN_PROGRESS", piIST(6, 17, 0)), piBoardEv("WAITING_ON_CS", piIST(6, 19, 0)),
		piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 11, 0)), piBoardEv("RESOLVED", piIST(9, 22, 0)),
	}
	want := piWant{policy: "Product Issue P2", stage: "ACHIEVED", start: created, end: piTP(piIST(9, 22, 0)),
		pause: 64 * piH, bpause: 4 * piH, elapsed: 14 * piH, remaining: 10 * piH, pct: 14.0 / 24 * 100}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 3, 0, 2}, {2, 1, 3, 0}} {
		e.seed(piP("P2"), created)
		e.now = created
		e.register()
		e.now = piIST(12, 12, 0)
		for _, i := range order {
			e.mustDeliver(events[i])
		}
		e.onlyRow().check(t, "order "+strconv.Itoa(order[0])+strconv.Itoa(order[1])+strconv.Itoa(order[2])+strconv.Itoa(order[3]), want)
	}
}

// TestProductIssueSLAIntegration_RegisteredAfterHistory: a clock registered
// after the issue already has history counts that history.
func TestProductIssueSLAIntegration_RegisteredAfterHistory(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	for _, ev := range []piEvent{piBoardEv("IN_PROGRESS", piAt(6, 11, 0)), piBoardEv("WAITING_ON_CS", piAt(6, 14, 0)), piBoardEv("WAITING_ON_PRODUCT_TEAM", piAt(6, 18, 0))} {
		e.seq++
		e.exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
		        VALUES ($1::uuid, $2, $3, $4, 'BACKFILL', $5, $6)`, piSLAWorkItemID, ev.to, ev.waits, ev.at, "sla-pi-test:"+strconv.Itoa(e.seq), ev.raw)
	}
	e.exec(`UPDATE github_issue_link SET board_status = 'WAITING_ON_PRODUCT_TEAM' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	e.now = piAt(6, 20, 0)
	e.register()
	// 10h wall since creation, 4h waiting on CS.
	e.onlyRow().check(t, "registered late", piWant{policy: "Product Issue P1", stage: "IN_PROGRESS", start: created,
		pause: 4 * piH, bpause: 4 * piH, elapsed: 6 * piH, remaining: 18 * piH, pct: 25})
	e.register() // and again: still one row
	if n := len(e.rows()); n != 1 {
		t.Errorf("registering twice left %d rows", n)
	}
}

// TestProductIssueSLAIntegration_Isolation: the seeded policies are never
// listed for cases or incidents, and a product issue never lists a case policy.
func TestProductIssueSLAIntegration_Isolation(t *testing.T) {
	e := newPISLAEnv(t)
	repo := repository.NewSLAEngineRepository(repository.NewScoped(e.pool), "")
	for _, target := range []string{"RESPONSE", "WORKAROUND", "RESOLUTION"} {
		legacy, err := repo.ListCandidatePolicies(e.ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range legacy {
			if p.AppliesTo != "" {
				t.Errorf("legacy listing for %s returned %q (applies_to %q)", target, p.Name, p.AppliesTo)
			}
		}
		fam, err := repo.ListFamilyCandidatePolicies(e.ctx, target, "PRODUCT_ISSUE")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range fam {
			if p.AppliesTo != "PRODUCT_ISSUE" || p.Source != "CSM" {
				t.Errorf("family listing for %s returned %+v", target, p)
			}
		}
		if want := map[string]int{"RESOLUTION": 3}[target]; len(fam) != want {
			t.Errorf("family listing for %s has %d policies, want %d", target, len(fam), want)
		}
	}
	// A product issue resolves to its own family only, and to exactly one policy.
	e.seed(piP("P2"), piAt(6, 10, 0))
	subj, err := e.svc.loadProductIssueSubject(e.ctx, piSLAWorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	got, lookupFailed := e.svc.resolveApplicablePolicies(e.ctx, subj)
	if lookupFailed || len(got) != 1 || got[0].Name != "Product Issue P2" || got[0].AppliesTo != "PRODUCT_ISSUE" {
		t.Errorf("resolved %+v (lookupFailed %v)", got, lookupFailed)
	}
}

// store inserts an event without running the engine, as a backfill does.
func (e *piSLAEnv) store(ev piEvent) {
	e.t.Helper()
	e.seq++
	var rawArg any
	if ev.raw != "" {
		rawArg = ev.raw
	}
	e.exec(`INSERT INTO work_item_status_event (work_item_id, to_status, waits_on, occurred_at, source, dedupe_key, raw_status)
	        VALUES ($1::uuid, $2, $3, $4, 'BACKFILL', $5, $6)`, piSLAWorkItemID, ev.to, ev.waits, ev.at, "sla-pi-test:"+strconv.Itoa(e.seq), rawArg)
}

// TestProductIssueSLAIntegration_BackfilledResolvedIssueGetsAClock: an issue
// imported already resolved (open -> WOC -> resolved) gets exactly one ACHIEVED
// clock with the right consumed time and breach flag, and recalculating again
// adds nothing.
func TestProductIssueSLAIntegration_BackfilledResolvedIssueGetsAClock(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(2, 10, 0) // Monday 10:00 IST, P3 (48h)
	e.seed(piP("P3"), created)
	for _, ev := range []piEvent{
		piBoardEv("IN_PROGRESS", piIST(2, 11, 0)), piBoardEv("WAITING_ON_CS", piIST(2, 14, 0)),
		piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(3, 10, 0)), piBoardEv("RESOLVED", piIST(6, 22, 0)),
	} {
		e.store(ev)
	}
	e.exec(`UPDATE github_issue_link SET board_status = 'RESOLVED' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	e.now = piIST(12, 12, 0)
	want := piWant{policy: "Product Issue P3", stage: "ACHIEVED", start: created, end: piTP(piIST(6, 22, 0)),
		pause: 20 * piH, bpause: 8 * piH, elapsed: 51 * piH, remaining: 0, pct: 51.0 / 48 * 100, breached: true}
	e.register()
	e.onlyRow().check(t, "backfilled", want)
	for i := 0; i < 2; i++ {
		if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
			t.Fatal(err)
		}
		e.onlyRow().check(t, "recalculated again", want)
	}
	// A within-target backfill: resolved quickly, met.
	e.seed(piP("P1"), piAt(6, 10, 0))
	e.store(piBoardEv("RESOLVED", piAt(6, 12, 0)))
	e.exec(`UPDATE github_issue_link SET board_status = 'RESOLVED' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	e.register()
	e.onlyRow().check(t, "met", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: piAt(6, 10, 0), end: piTP(piAt(6, 12, 0)),
		elapsed: 2 * piH, remaining: 22 * piH, pct: 100.0 / 12})
}

// TestProductIssueSLAIntegration_ReopenGitHubWhileCardStaysResolved: the card
// is still on Resolved when GitHub reopens the issue; the reopen restarts the
// clock, and moving the card on keeps it running.
func TestProductIssueSLAIntegration_ReopenGitHubWhileCardStaysResolved(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.mustDeliver(piBoardEv("RESOLVED", piAt(6, 14, 0)))
	e.mustDeliver(piGHClose(piAt(6, 15, 0)))
	e.now = piAt(7, 12, 0)
	e.mustDeliver(piGHReopen(piAt(7, 10, 0))) // board still RESOLVED
	piCheckReopened(t, e, created)

	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(7, 11, 0)))
	piCheckReopened(t, e, created) // still running, same figures
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	piCheckReopened(t, e, created)
}

// TestProductIssueSLAIntegration_ConcurrentOperationsAreSerialised runs
// recalculations and inline applies of one issue in parallel: one consistent
// row set, equal to the known sequential result.
func TestProductIssueSLAIntegration_ConcurrentOperationsAreSerialised(t *testing.T) {
	e := newPISLAEnv(t)
	created := piIST(6, 15, 0)
	events := []piEvent{
		piBoardEv("IN_PROGRESS", piIST(6, 17, 0)), piBoardEv("WAITING_ON_CS", piIST(6, 19, 0)),
		piBoardEv("WAITING_ON_PRODUCT_TEAM", piIST(9, 11, 0)), piBoardEv("RESOLVED", piIST(9, 22, 0)),
	}
	want := piWant{policy: "Product Issue P2", stage: "ACHIEVED", start: created, end: piTP(piIST(9, 22, 0)),
		pause: 64 * piH, bpause: 4 * piH, elapsed: 14 * piH, remaining: 10 * piH, pct: 14.0 / 24 * 100}
	for round := 0; round < 5; round++ {
		e.seed(piP("P2"), created)
		e.now = created
		e.register()
		e.now = piIST(12, 12, 0)
		for _, ev := range events {
			e.store(ev)
		}
		e.exec(`UPDATE github_issue_link SET board_status = 'RESOLVED' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)

		errs := make(chan error, 6)
		start := make(chan struct{})
		run := func(f func() error) {
			go func() { <-start; errs <- f() }()
		}
		for i := 0; i < 3; i++ {
			run(func() error { return e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID) })
		}
		run(func() error { return e.svc.ApplyWorkItemStatusEffects(e.ctx, piSLAWorkItemID, piIST(9, 22, 0), true) })
		run(func() error { return e.svc.ApplyWorkItemStatusEffects(e.ctx, piSLAWorkItemID, piIST(9, 11, 0), false) })
		run(func() error { return e.svc.RegisterWorkItemClocks(e.ctx, piSLAWorkItemID) })
		close(start)
		for i := 0; i < 6; i++ {
			if err := <-errs; err != nil {
				t.Errorf("round %d: concurrent operation failed: %v", round, err)
			}
		}
		e.onlyRow().check(t, "concurrent result", want)
	}
}

// piPolicyByName reads a seeded product-issue policy.
func (e *piSLAEnv) policyByName(name string) repository.SLAPolicyRef {
	e.t.Helper()
	ps, err := e.svc.repo.ListFamilyCandidatePolicies(e.ctx, "RESOLUTION", "PRODUCT_ISSUE")
	if err != nil {
		e.t.Fatal(err)
	}
	for _, p := range ps {
		if p.Name == name {
			return p
		}
	}
	e.t.Fatalf("policy %s not found", name)
	return repository.SLAPolicyRef{}
}

// TestProductIssueSLAIntegration_BackfilledClosedIssueWithoutCloseEvent: the link
// says CLOSED (github_closed_on set) but the timeline ends at In Progress, as a
// Phase 1 backfill leaves it: one ACHIEVED clock ending at github_closed_on.
func TestProductIssueSLAIntegration_BackfilledClosedIssueWithoutCloseEvent(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.store(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.exec(`UPDATE github_issue_link SET board_status = 'IN_PROGRESS', github_state = 'CLOSED', github_closed_on = $2 WHERE work_item_id = $1::uuid`, piSLAWorkItemID, piAt(6, 16, 0))
	e.now = piAt(20, 0, 0)
	e.register()
	want := piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(6, 16, 0)), elapsed: 6 * piH, remaining: 18 * piH, pct: 25}
	e.onlyRow().check(t, "closed backfill", want)
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	e.onlyRow().check(t, "recalculated", want)
}

// TestProductIssueSLAIntegration_TerminalLinkWithEmptyTimeline: board RESOLVED on
// the link, GitHub open, no events: ACHIEVED, never running.
func TestProductIssueSLAIntegration_TerminalLinkWithEmptyTimeline(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.exec(`UPDATE github_issue_link SET board_status = 'RESOLVED', github_updated_on = $2 WHERE work_item_id = $1::uuid`, piSLAWorkItemID, piAt(6, 13, 0))
	e.now = piAt(20, 0, 0)
	e.register()
	e.onlyRow().check(t, "terminal link", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(6, 13, 0)),
		elapsed: 3 * piH, remaining: 21 * piH, pct: 12.5})
	// Without any usable instant it ends at creation.
	e.seed(piP("P1"), created)
	e.exec(`UPDATE github_issue_link SET board_status = 'RESOLVED' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	e.register()
	e.onlyRow().check(t, "no instant", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(created), remaining: 24 * piH})
}

// TestProductIssueSLAIntegration_LinkWaitingOnCSButTimelineEndsInProgress: PAUSED
// from the last event time.
func TestProductIssueSLAIntegration_LinkWaitingOnCSButTimelineEndsInProgress(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.store(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.exec(`UPDATE github_issue_link SET board_status = 'WAITING_ON_CS' WHERE work_item_id = $1::uuid`, piSLAWorkItemID)
	e.now = piAt(6, 20, 0)
	e.register()
	e.onlyRow().check(t, "paused from the last event", piWant{policy: "Product Issue P1", stage: "PAUSED", start: created, paused: piTP(piAt(6, 11, 0)),
		elapsed: 1 * piH, remaining: 23 * piH, pct: 100.0 / 24})
}

// TestProductIssueSLAIntegration_StaleRebuildDoesNotLoseAnInlineEvent: a rebuild
// that read the timeline before an event committed and wrote after the event's
// inline apply clobbers it; the next apply of that event's final rebuild repairs it.
func TestProductIssueSLAIntegration_StaleRebuildDoesNotLoseAnInlineEvent(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("IN_PROGRESS", piAt(6, 11, 0)))
	e.now = piAt(6, 20, 0)

	pol := e.policyByName("Product Issue P1")
	var rowID string
	var stale repository.ProductIssueClockState
	lock := func(f func(tx repository.ProductIssueSLATx) error) {
		t.Helper()
		if err := e.svc.pi.WithProductIssueLock(e.ctx, piSLAWorkItemID, func(_ context.Context, tx repository.ProductIssueSLATx) error { return f(tx) }); err != nil {
			t.Fatal(err)
		}
	}
	lock(func(tx repository.ProductIssueSLATx) error { // the stale rebuild reads the timeline now
		f, err := tx.Facts(e.ctx)
		if err != nil {
			return err
		}
		events, err := tx.Timeline(e.ctx)
		if err != nil {
			return err
		}
		row, _, err := tx.LatestClockRow(e.ctx, pol.ID)
		if err != nil {
			return err
		}
		rowID = row.ID
		stale = rebuildProductIssueClock(pol, f.CreatedOn, e.now, productIssueSegments(productIssueFactsFrom(f), events))
		return nil
	})
	wocAt := piAt(6, 14, 0)
	e.mustDeliver(piBoardEv("WAITING_ON_CS", wocAt))
	paused := piWant{policy: "Product Issue P1", stage: "PAUSED", start: created, paused: piTP(wocAt), elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6}
	e.onlyRow().check(t, "after the event", paused)

	lock(func(tx repository.ProductIssueSLATx) error { return tx.RewriteClock(e.ctx, rowID, stale) }) // the stale write lands late
	if r := e.onlyRow(); r.stage != "IN_PROGRESS" {
		t.Fatalf("expected the stale write to clobber the event, got %+v", r)
	}
	if err := e.svc.ApplyWorkItemStatusEffects(e.ctx, piSLAWorkItemID, wocAt, true); err != nil {
		t.Fatal(err)
	}
	e.onlyRow().check(t, "repaired", paused)
}

// TestProductIssueSLAIntegration_LatestClockRowPrefersTheOpenRow: created_on is a
// transaction start time, so a reopen row can sort before an older ACHIEVED row.
func TestProductIssueSLAIntegration_LatestClockRowPrefersTheOpenRow(t *testing.T) {
	e := newPISLAEnv(t)
	created := piAt(6, 10, 0)
	e.seed(piP("P1"), created)
	e.now = created
	e.register()
	e.mustDeliver(piBoardEv("RESOLVED", piAt(6, 12, 0)))
	e.exec(`UPDATE sla SET created_on = NOW() + INTERVAL '1 hour' WHERE work_item_id = $1::uuid`, piSLAWorkItemID) // the old row now sorts last
	e.now = piAt(7, 12, 0)
	e.mustDeliver(piBoardEv("REOPENED", piAt(7, 10, 0)))

	pol := e.policyByName("Product Issue P1")
	if err := e.svc.pi.WithProductIssueLock(e.ctx, piSLAWorkItemID, func(_ context.Context, tx repository.ProductIssueSLATx) error {
		row, ok, err := tx.LatestClockRow(e.ctx, pol.ID)
		if err != nil || !ok || row.Stage != "IN_PROGRESS" {
			t.Errorf("latest row = %+v ok=%v err=%v, want the open IN_PROGRESS row", row, ok, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RecalculateWorkItemClocks(e.ctx, piSLAWorkItemID); err != nil {
		t.Fatal(err)
	}
	for _, r := range e.rows() {
		switch r.stage {
		case "ACHIEVED":
			r.check(t, "old row untouched", piWant{policy: "Product Issue P1", stage: "ACHIEVED", start: created, end: piTP(piAt(6, 12, 0)), elapsed: 2 * piH, remaining: 22 * piH, pct: 100.0 / 12})
		default:
			r.check(t, "open row", piWant{policy: "Product Issue P1", stage: "IN_PROGRESS", start: created, pause: 22 * piH, bpause: 22 * piH, elapsed: 4 * piH, remaining: 20 * piH, pct: 100.0 / 6})
		}
	}
}
