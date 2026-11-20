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

// Characterisation (golden-value) tests for the SLA clock engine's storage
// layer, written BEFORE the engine is refactored (calendar abstraction,
// generic subject core, sla_policy.applies_to) so that a later change can
// prove it altered nothing. They pin what the code does today, including
// behaviour that looks odd; they are not a statement that the behaviour is
// right. Anything surprising is marked "QUIRK" next to the assertion.
//
// Every test runs against a real PostgreSQL (the SQL is the thing under test)
// and is skipped unless ENTITY_TEST_DATABASE_URL is set; apply every
// migration first:
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run SLACharacterisation ./internal/repository/
//
// How time is made deterministic. The flat (24 x 7) paths read the database's
// NOW() and the 'WSO2 Business days' paths read Go's time.Now(), so a stored
// value that depends on "now" can only be asserted with a computed
// expectation and a tolerance (slaCharTolerance, which only has to cover the
// milliseconds between a test's own UPDATE and the engine call that follows).
// Wherever a value can be made independent of "now" it is asserted exactly
// instead:
//   - a clock with an OPEN pause segment at completion/cancellation finishes
//     with elapsed = (paused_on - start_on) - pauses, because the open segment
//     cancels the "now" term (NOW() - start - (NOW() - paused_on)); the same
//     holds for business days, since businessdays.Elapsed is additive over
//     adjacent intervals. Those tests seed fixed instants and assert exact
//     values;
//   - an idempotent re-pause freezes at the stored paused_on, never at now;
//   - the sla_live view is read inside one transaction, in which NOW() is a
//     constant that the test reads back, so the view's output is exact.
//
// Rows are seeded under unique ids (a fresh work_item per test, policies named
// with slaCharPolicyPrefix) and removed afterwards; the database is shared.

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	slaCharPolicyPrefix = "SLA characterisation "
	slaCharActor        = "sla-char-test"
	slaCharBusinessDays = "WSO2 Business days"
	// slaCharTolerance is the slack allowed between a test's own setup
	// statement and the engine call that reads NOW()/time.Now() right after
	// it. Local round trips are milliseconds; this is deliberately generous.
	slaCharTolerance = 5 * time.Second
)

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

type slaCharQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type slaCharExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type slaCharEnv struct {
	t          *testing.T
	ctx        context.Context
	pool       *pgxpool.Pool
	scoped     *repository.Scoped
	repo       repository.SLAEngineRepository
	workItemID string
	policyIDs  []string
	seq        int
}

// slaCharPool connects or skips. It is the only place the env var is read.
func slaCharPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database SLA characterisation tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

// newSLACharEnv creates one work_item of its own and removes it, its sla rows
// and every policy it created when the test ends.
func newSLACharEnv(t *testing.T) *slaCharEnv {
	t.Helper()
	pool := slaCharPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	e := &slaCharEnv{t: t, ctx: ctx, pool: pool, scoped: scoped, repo: repository.NewSLAEngineRepository(scoped, "")}

	// Remove anything a crashed earlier run left behind.
	if _, err := scoped.Exec(ctx, `DELETE FROM sla_policy WHERE name LIKE $1 AND created_by = $2`, slaCharPolicyPrefix+"%", slaCharActor); err != nil {
		t.Fatalf("pre-clean policies: %v", err)
	}
	if err := scoped.QueryRow(ctx, `
		INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1,
		        'SLACHAR' || substr(md5(random()::text), 1, 10), 'SLACHAR-' || substr(md5(random()::text), 1, 10),
		        'sla characterisation test case', 'CASE')
		RETURNING id::text`, slaCharActor).Scan(&e.workItemID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM sla WHERE work_item_id = $1::uuid`, e.workItemID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1::uuid`, e.workItemID)
		for _, id := range e.policyIDs {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, id)
		}
	})
	return e
}

// insertSLACharPolicy inserts a source='CSM' policy (so it never shows up in
// ListCandidatePolicies, which reads SERVICENOW rows only) and returns the
// SLAPolicyRef the engine would be handed for it.
func insertSLACharPolicy(ctx context.Context, q slaCharQuerier, name, target string, duration time.Duration, schedule string) (repository.SLAPolicyRef, error) {
	var id string
	err := q.QueryRow(ctx, `
		INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, schedule, source)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, TRUE, $3::sla_policy_target_enum, $4::interval, NULLIF($5, ''), 'CSM'::sla_source_enum)
		RETURNING id::text`, slaCharActor, name, target, intervalLiteral(duration), schedule).Scan(&id)
	if err != nil {
		return repository.SLAPolicyRef{}, err
	}
	return repository.SLAPolicyRef{ID: id, Name: name, Target: target, Duration: duration, Schedule: schedule}, nil
}

func (e *slaCharEnv) policy(target string, duration time.Duration, schedule string) repository.SLAPolicyRef {
	e.t.Helper()
	e.seq++
	p, err := insertSLACharPolicy(e.ctx, e.scoped, fmt.Sprintf("%s%s #%d", slaCharPolicyPrefix, e.workItemID, e.seq), target, duration, schedule)
	if err != nil {
		e.t.Fatalf("seed sla_policy: %v", err)
	}
	e.policyIDs = append(e.policyIDs, p.ID)
	return p
}

func (e *slaCharEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.scoped.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatalf("exec failed: %v\nsql: %s", err, sql)
	}
}

// register runs RegisterClock with the zero start (= now) and requires that it inserted.
func (e *slaCharEnv) register(p repository.SLAPolicyRef) {
	e.t.Helper()
	ok, err := e.repo.RegisterClock(e.ctx, e.workItemID, p, time.Time{})
	if err != nil || !ok {
		e.t.Fatalf("RegisterClock(%s) = %v, %v; want true, nil", p.Name, ok, err)
	}
}

// startAgo moves a registered clock's start_on to ago before the database's NOW().
func (e *slaCharEnv) startAgo(p repository.SLAPolicyRef, ago time.Duration) {
	e.t.Helper()
	e.exec(`UPDATE sla SET start_on = NOW() - $3::interval WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid`,
		e.workItemID, p.ID, intervalLiteral(ago))
}

// slaSeed describes a row inserted directly, bypassing RegisterClock, so a
// test can start from an exact stored state.
type slaSeed struct {
	Stage            string
	IsActive         bool
	HasBreached      bool
	StartOn          *time.Time
	PausedOn         *time.Time
	PauseDur         *time.Duration
	BusinessPauseDur *time.Duration
	Pct              *float64
	BusinessDur      *time.Duration
	RemainingDur     *time.Duration
}

func ivl(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return intervalLiteral(*d)
}

func durp(d time.Duration) *time.Duration { return &d }
func timep(t time.Time) *time.Time        { return &t }
func f64p(f float64) *float64             { return &f }

func insertSLACharSeed(ctx context.Context, x slaCharExecer, workItemID any, p repository.SLAPolicyRef, s slaSeed) error {
	_, err := x.Exec(ctx, `
		INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, schedule,
		                 is_active, stage, has_breached, start_on, paused_on, duration, pause_duration,
		                 business_pause_duration, business_elapsed_percentage, business_duration, remaining_business_duration)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2::uuid, $3::uuid, NULLIF($4, ''),
		        $5, $6::sla_stage_enum, $7, $8::timestamptz, $9::timestamptz, $10::interval, $11::interval,
		        $12::interval, $13::numeric, $14::interval, $15::interval)`,
		slaCharActor, workItemID, p.ID, p.Schedule,
		s.IsActive, s.Stage, s.HasBreached, s.StartOn, s.PausedOn, intervalLiteral(p.Duration), ivl(s.PauseDur),
		ivl(s.BusinessPauseDur), s.Pct, ivl(s.BusinessDur), ivl(s.RemainingDur))
	return err
}

func (e *slaCharEnv) seed(p repository.SLAPolicyRef, s slaSeed) {
	e.t.Helper()
	if err := insertSLACharSeed(e.ctx, e.scoped, e.workItemID, p, s); err != nil {
		e.t.Fatalf("seed sla row: %v", err)
	}
}

// slaCharRow is one stored sla row, durations in seconds, NULL kept as nil.
type slaCharRow struct {
	Stage            string
	IsActive         *bool
	HasBreached      *bool
	StartOn          time.Time
	EndOn            *time.Time
	PausedOn         *time.Time
	Schedule         *string
	Duration         *float64
	PauseDur         *float64
	BusinessPauseDur *float64
	BusinessDur      *float64
	RemainingDur     *float64
	Pct              *float64
	CreatedBy        string
	UpdatedBy        string
}

// rows returns every sla row of (work item, policy), keyed by stage.
func (e *slaCharEnv) rows(p repository.SLAPolicyRef) map[string]slaCharRow {
	e.t.Helper()
	rs, err := e.scoped.Query(e.ctx, `
		SELECT stage::text, is_active, has_breached, start_on, end_on, paused_on, schedule,
		       EXTRACT(EPOCH FROM duration)::float8, EXTRACT(EPOCH FROM pause_duration)::float8,
		       EXTRACT(EPOCH FROM business_pause_duration)::float8, EXTRACT(EPOCH FROM business_duration)::float8,
		       EXTRACT(EPOCH FROM remaining_business_duration)::float8, business_elapsed_percentage::float8,
		       created_by, updated_by
		FROM sla WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid`, e.workItemID, p.ID)
	if err != nil {
		e.t.Fatalf("read sla rows: %v", err)
	}
	defer rs.Close()
	out := map[string]slaCharRow{}
	for rs.Next() {
		var r slaCharRow
		if err := rs.Scan(&r.Stage, &r.IsActive, &r.HasBreached, &r.StartOn, &r.EndOn, &r.PausedOn, &r.Schedule,
			&r.Duration, &r.PauseDur, &r.BusinessPauseDur, &r.BusinessDur, &r.RemainingDur, &r.Pct,
			&r.CreatedBy, &r.UpdatedBy); err != nil {
			e.t.Fatalf("scan sla row: %v", err)
		}
		if _, dup := out[r.Stage]; dup {
			e.t.Fatalf("two %s rows for policy %s", r.Stage, p.Name)
		}
		out[r.Stage] = r
	}
	if err := rs.Err(); err != nil {
		e.t.Fatalf("sla rows: %v", err)
	}
	return out
}

// one returns the only row of the policy and fails if there is not exactly one.
func (e *slaCharEnv) one(p repository.SLAPolicyRef) slaCharRow {
	e.t.Helper()
	rs := e.rows(p)
	if len(rs) != 1 {
		e.t.Fatalf("policy %s: %d rows %v, want exactly 1", p.Name, len(rs), stagesOf(rs))
	}
	for _, r := range rs {
		return r
	}
	panic("unreachable")
}

func stagesOf(m map[string]slaCharRow) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	return s
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

func chkStage(t *testing.T, label string, r slaCharRow, want string, active bool) {
	t.Helper()
	if r.Stage != want {
		t.Errorf("%s: stage = %s, want %s", label, r.Stage, want)
	}
	chkBool(t, label+" is_active", r.IsActive, active)
}

func chkBool(t *testing.T, label string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want %v", label, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %v, want %v", label, *got, want)
	}
}

// chkSecs asserts a stored interval (seconds) is within tol of want.
func chkSecs(t *testing.T, label string, got *float64, want, tol time.Duration) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want %v", label, want)
		return
	}
	if math.Abs(*got-want.Seconds()) > tol.Seconds()+1e-6 {
		t.Errorf("%s = %vs, want %v (+/- %v)", label, *got, want, tol)
	}
}

func chkNull(t *testing.T, label string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %vs, want NULL", label, *got)
	}
}

// chkPct asserts the stored business_elapsed_percentage (numeric(12,2), so
// already rounded to two decimals) is within tol of want.
func chkPct(t *testing.T, label string, got *float64, want, tol float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want %v", label, want)
		return
	}
	if math.Abs(*got-want) > tol+0.0051 {
		t.Errorf("%s = %v, want %v (+/- %v)", label, *got, want, tol)
	}
}

func chkTime(t *testing.T, label string, got *time.Time, want time.Time, tol time.Duration) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want %v", label, want)
		return
	}
	if d := got.Sub(want); d < -tol || d > tol {
		t.Errorf("%s = %v, want %v (+/- %v)", label, got.UTC(), want.UTC(), tol)
	}
}

func chkNullTime(t *testing.T, label string, got *time.Time) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %v, want NULL", label, got.UTC())
	}
}

// pctTol is the percentage slack that corresponds to a time slack on a
// clock of the given duration.
func pctTol(tol, duration time.Duration) float64 {
	return tol.Seconds()/duration.Seconds()*100 + 0.01
}

func slaCharNY(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}
	tm, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm.UTC()
}

func hours(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

// ---------------------------------------------------------------------------
// 1. RegisterClock
// ---------------------------------------------------------------------------

// TestSLACharacterisation_RegisterClock pins the row RegisterClock inserts,
// for every schedule value. The register path is schedule-blind: the same
// columns are written whatever the schedule, and sla.schedule just echoes
// policy.Schedule (NULL when empty).
func TestSLACharacterisation_RegisterClock(t *testing.T) {
	for _, schedule := range []string{"", "24 x 7", slaCharBusinessDays, "8-5 Weekdays"} {
		t.Run("schedule="+schedule, func(t *testing.T) {
			e := newSLACharEnv(t)
			p := e.policy("RESPONSE", 4*time.Hour, schedule)

			before := time.Now()
			ok, err := e.repo.RegisterClock(e.ctx, e.workItemID, p, time.Time{})
			if err != nil || !ok {
				t.Fatalf("RegisterClock = %v, %v; want true, nil", ok, err)
			}
			r := e.one(p)
			chkStage(t, "registered", r, "IN_PROGRESS", true)
			chkBool(t, "has_breached", r.HasBreached, false)
			chkTime(t, "start_on", &r.StartOn, before, slaCharTolerance)
			chkNullTime(t, "end_on", r.EndOn)
			chkNullTime(t, "paused_on", r.PausedOn)
			chkNull(t, "pause_duration", r.PauseDur)
			chkNull(t, "business_pause_duration", r.BusinessPauseDur)
			chkSecs(t, "duration", r.Duration, 4*time.Hour, 0)
			chkSecs(t, "business_duration", r.BusinessDur, 0, 0)
			chkSecs(t, "remaining_business_duration", r.RemainingDur, 4*time.Hour, 0)
			chkPct(t, "business_elapsed_percentage", r.Pct, 0, 0)
			if schedule == "" {
				if r.Schedule != nil {
					t.Errorf("schedule column = %q, want NULL for an empty policy schedule", *r.Schedule)
				}
			} else if r.Schedule == nil || *r.Schedule != schedule {
				t.Errorf("schedule column = %v, want %q echoed verbatim", r.Schedule, schedule)
			}
			if r.CreatedBy != "sla-engine" || r.UpdatedBy != "sla-engine" {
				t.Errorf("created_by/updated_by = %q/%q, want sla-engine/sla-engine", r.CreatedBy, r.UpdatedBy)
			}

			// A second registration for the same (work item, policy) while the
			// first is active inserts nothing and reports false.
			ok, err = e.repo.RegisterClock(e.ctx, e.workItemID, p, time.Time{})
			if err != nil || ok {
				t.Errorf("second RegisterClock = %v, %v; want false, nil", ok, err)
			}
			e.one(p)
		})
	}
}

// TestSLACharacterisation_RegisterClock_ExplicitStartOn pins the retroactive
// path: a non-zero startOn is stored verbatim and everything else is as for
// an ordinary registration.
func TestSLACharacterisation_RegisterClock_ExplicitStartOn(t *testing.T) {
	e := newSLACharEnv(t)
	p := e.policy("RESPONSE", 4*time.Hour, "")
	start := time.Date(2024, 3, 8, 17, 30, 15, 0, time.UTC)
	if ok, err := e.repo.RegisterClock(e.ctx, e.workItemID, p, start); err != nil || !ok {
		t.Fatalf("RegisterClock = %v, %v", ok, err)
	}
	r := e.one(p)
	if !r.StartOn.Equal(start) {
		t.Errorf("start_on = %v, want exactly %v", r.StartOn.UTC(), start)
	}
	chkStage(t, "registered", r, "IN_PROGRESS", true)
	chkSecs(t, "remaining_business_duration", r.RemainingDur, 4*time.Hour, 0)
}

// TestSLACharacterisation_RegisterClock_BlockingStages pins which existing
// row for the SAME (work item, policy) blocks a fresh registration, per
// target: an active row (IN_PROGRESS/PAUSED) always blocks; ACHIEVED and
// COMPLETED always block; BREACHED blocks RESPONSE but NOT
// WORKAROUND/RESOLUTION; CANCELLED never blocks.
func TestSLACharacterisation_RegisterClock_BlockingStages(t *testing.T) {
	type tc struct {
		target, stage string
		wantInserted  bool
	}
	var cases []tc
	for _, target := range []string{"RESPONSE", "WORKAROUND", "RESOLUTION"} {
		for _, stage := range []string{"IN_PROGRESS", "PAUSED", "BREACHED", "ACHIEVED", "COMPLETED", "CANCELLED"} {
			inserted := stage == "CANCELLED" || (stage == "BREACHED" && target != "RESPONSE")
			cases = append(cases, tc{target, stage, inserted})
		}
	}
	e := newSLACharEnv(t)
	start := time.Date(2024, 3, 4, 9, 0, 0, 0, time.UTC)
	for _, c := range cases {
		p := e.policy(c.target, 8*time.Hour, "")
		e.seed(p, slaSeed{Stage: c.stage, IsActive: c.stage == "IN_PROGRESS" || c.stage == "PAUSED" || c.stage == "BREACHED", StartOn: &start})
		ok, err := e.repo.RegisterClock(e.ctx, e.workItemID, p, time.Time{})
		if err != nil {
			t.Fatalf("%s/%s: RegisterClock: %v", c.target, c.stage, err)
		}
		if ok != c.wantInserted {
			t.Errorf("%s with an existing %s row: RegisterClock = %v, want %v", c.target, c.stage, ok, c.wantInserted)
		}
		rs := e.rows(p)
		if c.wantInserted {
			// The old row stays as it was and a fresh IN_PROGRESS row joins it.
			if _, fresh := rs["IN_PROGRESS"]; !fresh || len(rs) != 2 {
				t.Errorf("%s/%s: rows = %v, want the old row plus a fresh IN_PROGRESS", c.target, c.stage, stagesOf(rs))
			}
		} else if len(rs) != 1 {
			t.Errorf("%s/%s: rows = %v, want the single existing row", c.target, c.stage, stagesOf(rs))
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Flat (24 x 7) lifecycle: pause, resume, complete, cancel
// ---------------------------------------------------------------------------

// TestSLACharacterisation_Flat_PauseResumeComplete walks one flat clock
// (24h policy, started 10h ago) through pause -> resume -> complete and pins
// every stored column after each step. Flat paths never touch
// business_pause_duration (it stays NULL).
func TestSLACharacterisation_Flat_PauseResumeComplete(t *testing.T) {
	const dur = 24 * time.Hour
	for _, schedule := range []string{"", "24 x 7", "8-5 Weekdays"} {
		t.Run("schedule="+schedule, func(t *testing.T) {
			e := newSLACharEnv(t)
			p := e.policy("RESOLUTION", dur, schedule)
			e.register(p)
			e.startAgo(p, 10*time.Hour)
			tol := slaCharTolerance
			ptol := pctTol(tol, dur)

			// --- pause: freezes the snapshot at the pause instant.
			now := time.Now()
			if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", true); err != nil || !ok {
				t.Fatalf("SetPaused(true) = %v, %v", ok, err)
			}
			r := e.one(p)
			chkStage(t, "after pause", r, "PAUSED", true)
			chkTime(t, "paused_on", r.PausedOn, now, tol)
			chkNull(t, "pause_duration (a first pause writes nothing to it)", r.PauseDur)
			chkNull(t, "business_pause_duration", r.BusinessPauseDur)
			chkBool(t, "has_breached", r.HasBreached, false)
			chkSecs(t, "business_duration (frozen)", r.BusinessDur, 10*time.Hour, tol)
			chkSecs(t, "remaining_business_duration (frozen)", r.RemainingDur, 14*time.Hour, tol)
			chkPct(t, "business_elapsed_percentage (frozen)", r.Pct, 10.0/24*100, ptol)
			chkNullTime(t, "end_on", r.EndOn)

			// --- resume after 3h paused.
			e.exec(`UPDATE sla SET paused_on = paused_on - INTERVAL '3 hours' WHERE work_item_id = $1::uuid`, e.workItemID)
			if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", false); err != nil || !ok {
				t.Fatalf("SetPaused(false) = %v, %v", ok, err)
			}
			r = e.one(p)
			chkStage(t, "after resume", r, "IN_PROGRESS", true)
			chkNullTime(t, "paused_on", r.PausedOn)
			chkSecs(t, "pause_duration (the finished segment)", r.PauseDur, 3*time.Hour, tol)
			chkNull(t, "business_pause_duration", r.BusinessPauseDur)
			// QUIRK: resume leaves the frozen snapshot columns as the pause wrote them.
			chkSecs(t, "business_duration (stale after resume)", r.BusinessDur, 10*time.Hour, tol)
			chkSecs(t, "remaining_business_duration (stale after resume)", r.RemainingDur, 14*time.Hour, tol)
			chkPct(t, "business_elapsed_percentage (stale after resume)", r.Pct, 10.0/24*100, ptol)

			// --- complete: elapsed = now - start_on - pause_duration = 10h - 3h.
			now = time.Now()
			if ok, err := e.repo.CompleteClock(e.ctx, e.workItemID, "RESOLUTION"); err != nil || !ok {
				t.Fatalf("CompleteClock = %v, %v", ok, err)
			}
			r = e.one(p)
			chkStage(t, "after complete", r, "ACHIEVED", false)
			chkTime(t, "end_on", r.EndOn, now, tol)
			chkNullTime(t, "paused_on", r.PausedOn)
			chkSecs(t, "pause_duration", r.PauseDur, 3*time.Hour, tol)
			chkNull(t, "business_pause_duration", r.BusinessPauseDur)
			chkSecs(t, "business_duration", r.BusinessDur, 7*time.Hour, tol)
			chkSecs(t, "remaining_business_duration", r.RemainingDur, 17*time.Hour, tol)
			chkPct(t, "business_elapsed_percentage", r.Pct, 7.0/24*100, ptol)
			chkBool(t, "has_breached", r.HasBreached, false)
			if r.UpdatedBy != "sla-engine" {
				t.Errorf("updated_by = %q, want sla-engine", r.UpdatedBy)
			}

			// --- a terminal clock is untouched by every later call.
			for name, fn := range map[string]func() (bool, error){
				"SetPaused(true)":  func() (bool, error) { return e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", true) },
				"SetPaused(false)": func() (bool, error) { return e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", false) },
				"CompleteClock":    func() (bool, error) { return e.repo.CompleteClock(e.ctx, e.workItemID, "RESOLUTION") },
				"CancelClock":      func() (bool, error) { return e.repo.CancelClock(e.ctx, e.workItemID, "RESOLUTION") },
			} {
				if ok, err := fn(); err != nil || ok {
					t.Errorf("%s on an ACHIEVED clock = %v, %v; want false, nil", name, ok, err)
				}
			}
			if got := e.one(p); got.Stage != "ACHIEVED" {
				t.Errorf("stage after the no-op calls = %s, want ACHIEVED", got.Stage)
			}
		})
	}
}

// TestSLACharacterisation_Flat_DuplicatePauseKeepsFrozenSnapshot pins that a
// second pause call on an already-PAUSED clock re-derives the same frozen
// values from the stored paused_on and does not move paused_on.
func TestSLACharacterisation_Flat_DuplicatePauseKeepsFrozenSnapshot(t *testing.T) {
	e := newSLACharEnv(t)
	start := slaCharNY(t, "2024-03-04T09:00:00")
	pausedOn := start.Add(6 * time.Hour)
	p := e.policy("WORKAROUND", 8*time.Hour, "24 x 7")
	e.seed(p, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &start, PausedOn: &pausedOn, PauseDur: durp(time.Hour),
		Pct: f64p(1), BusinessDur: durp(time.Minute), RemainingDur: durp(time.Minute)})

	if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "WORKAROUND", true); err != nil || !ok {
		t.Fatalf("SetPaused(true) = %v, %v", ok, err)
	}
	r := e.one(p)
	chkStage(t, "after duplicate pause", r, "PAUSED", true)
	if r.PausedOn == nil || !r.PausedOn.Equal(pausedOn) {
		t.Errorf("paused_on = %v, want unchanged %v", r.PausedOn, pausedOn)
	}
	// elapsed = paused_on - start_on - pause_duration = 6h - 1h = 5h of an 8h clock. Exact.
	chkSecs(t, "business_duration", r.BusinessDur, 5*time.Hour, 0)
	chkSecs(t, "remaining_business_duration", r.RemainingDur, 3*time.Hour, 0)
	chkPct(t, "business_elapsed_percentage", r.Pct, 62.5, 0)
	chkSecs(t, "pause_duration", r.PauseDur, time.Hour, 0)
}

// TestSLACharacterisation_Flat_CompleteAndCancel pins CompleteClock and
// CancelClock on flat clocks that are running (a clock past its duration is
// finalised with has_breached = true, an uncapped percentage and zero
// remaining) and that are paused at the moment of the call (the open pause is
// finalised into pause_duration).
func TestSLACharacterisation_Flat_CompleteAndCancel(t *testing.T) {
	type op struct {
		name       string
		call       func(e *slaCharEnv) (bool, error)
		wantStage  string
		wantActive bool
	}
	ops := []op{
		{"complete", func(e *slaCharEnv) (bool, error) { return e.repo.CompleteClock(e.ctx, e.workItemID, "RESOLUTION") }, "ACHIEVED", false},
		{"cancel", func(e *slaCharEnv) (bool, error) { return e.repo.CancelClock(e.ctx, e.workItemID, "RESOLUTION") }, "CANCELLED", false},
	}
	const dur = 24 * time.Hour
	for _, o := range ops {
		t.Run(o.name+"/running past its duration", func(t *testing.T) {
			e := newSLACharEnv(t)
			p := e.policy("RESOLUTION", dur, "")
			e.register(p)
			e.startAgo(p, 30*time.Hour)
			if ok, err := o.call(e); err != nil || !ok {
				t.Fatalf("call = %v, %v", ok, err)
			}
			r := e.one(p)
			chkStage(t, o.name, r, o.wantStage, o.wantActive)
			chkBool(t, "has_breached", r.HasBreached, true)
			chkSecs(t, "business_duration", r.BusinessDur, 30*time.Hour, slaCharTolerance)
			chkSecs(t, "remaining_business_duration (floored at 0)", r.RemainingDur, 0, 0)
			chkPct(t, "business_elapsed_percentage (uncapped)", r.Pct, 125, pctTol(slaCharTolerance, dur))
			chkTime(t, "end_on", r.EndOn, time.Now(), slaCharTolerance)
			chkNullTime(t, "paused_on", r.PausedOn)
			chkNull(t, "business_pause_duration", r.BusinessPauseDur)
		})
		t.Run(o.name+"/open pause is finalised", func(t *testing.T) {
			e := newSLACharEnv(t)
			p := e.policy("RESOLUTION", dur, "24 x 7")
			e.register(p)
			e.startAgo(p, 10*time.Hour)
			if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", true); err != nil || !ok {
				t.Fatalf("SetPaused = %v, %v", ok, err)
			}
			// paused for the last 4h: paused_on - start_on = 6h of running time.
			e.exec(`UPDATE sla SET paused_on = paused_on - INTERVAL '4 hours' WHERE work_item_id = $1::uuid`, e.workItemID)
			if ok, err := o.call(e); err != nil || !ok {
				t.Fatalf("call = %v, %v", ok, err)
			}
			r := e.one(p)
			chkStage(t, o.name, r, o.wantStage, o.wantActive)
			chkNullTime(t, "paused_on", r.PausedOn)
			chkSecs(t, "pause_duration", r.PauseDur, 4*time.Hour, slaCharTolerance)
			chkSecs(t, "business_duration", r.BusinessDur, 6*time.Hour, slaCharTolerance)
			chkSecs(t, "remaining_business_duration", r.RemainingDur, 18*time.Hour, slaCharTolerance)
			chkPct(t, "business_elapsed_percentage", r.Pct, 25, pctTol(slaCharTolerance, dur))
			chkBool(t, "has_breached", r.HasBreached, false)
		})
	}
}

// ---------------------------------------------------------------------------
// 3. 'WSO2 Business days' lifecycle
// ---------------------------------------------------------------------------

// Fixed instants used by the exact business-day tests. America/New_York:
//
//	start    Fri 2024-03-08 12:00 EST
//	pausedOn Mon 2024-03-11 10:00 EDT   (DST began on Sun 2024-03-10)
//
// Business time between them = Fri 12h + Mon 10h = 22h (the weekend, and the
// 23-hour Sunday, count for nothing); wall-clock time between them is 70h.
func slaCharBDFixedInstants(t *testing.T) (start, pausedOn time.Time) {
	t.Helper()
	return slaCharNY(t, "2024-03-08T12:00:00"), slaCharNY(t, "2024-03-11T10:00:00")
}

// TestSLACharacterisation_BusinessDays_IdempotentPauseIsExact pins the
// business-day pause freeze with exact values: a re-pause freezes at the
// stored paused_on, so the snapshot is
// businessdays.Elapsed(start, paused_on) - business_pause_duration.
func TestSLACharacterisation_BusinessDays_IdempotentPauseIsExact(t *testing.T) {
	e := newSLACharEnv(t)
	start, pausedOn := slaCharBDFixedInstants(t)
	p := e.policy("RESOLUTION", 40*time.Hour, slaCharBusinessDays)
	e.seed(p, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &start, PausedOn: &pausedOn,
		PauseDur: durp(5 * time.Hour), BusinessPauseDur: durp(2 * time.Hour),
		Pct: f64p(1), BusinessDur: durp(time.Minute), RemainingDur: durp(time.Minute)})

	if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", true); err != nil || !ok {
		t.Fatalf("SetPaused(true) = %v, %v", ok, err)
	}
	r := e.one(p)
	chkStage(t, "after re-pause", r, "PAUSED", true)
	if r.PausedOn == nil || !r.PausedOn.Equal(pausedOn) {
		t.Errorf("paused_on = %v, want unchanged %v", r.PausedOn, pausedOn)
	}
	chkSecs(t, "business_duration", r.BusinessDur, 20*time.Hour, 0) // 22h - 2h business pause
	chkSecs(t, "remaining_business_duration", r.RemainingDur, 20*time.Hour, 0)
	chkPct(t, "business_elapsed_percentage", r.Pct, 50, 0)
	chkSecs(t, "pause_duration (unchanged)", r.PauseDur, 5*time.Hour, 0)
	chkSecs(t, "business_pause_duration (unchanged)", r.BusinessPauseDur, 2*time.Hour, 0)
}

// TestSLACharacterisation_BusinessDays_NewPause pins a first pause. The
// freeze instant is Go's time.Now(), so the expectation is computed from the
// stored paused_on (read back) with businessdays.Elapsed.
func TestSLACharacterisation_BusinessDays_NewPause(t *testing.T) {
	const dur = 1000 * time.Hour
	e := newSLACharEnv(t)
	p := e.policy("RESOLUTION", dur, slaCharBusinessDays)
	e.register(p)
	e.startAgo(p, 10*24*time.Hour)

	if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", true); err != nil || !ok {
		t.Fatalf("SetPaused(true) = %v, %v", ok, err)
	}
	r := e.one(p)
	chkStage(t, "after pause", r, "PAUSED", true)
	chkTime(t, "paused_on", r.PausedOn, time.Now(), slaCharTolerance)
	want := businessdays.Elapsed(r.StartOn, *r.PausedOn)
	chkSecs(t, "business_duration (frozen)", r.BusinessDur, want, time.Second)
	chkSecs(t, "remaining_business_duration (frozen)", r.RemainingDur, dur-want, time.Second)
	chkPct(t, "business_elapsed_percentage (frozen)", r.Pct, want.Seconds()/dur.Seconds()*100, 0.01)
	// QUIRK: unlike the flat path (which leaves them NULL), the business-day
	// path writes both pause totals as an explicit zero on a first pause.
	chkSecs(t, "pause_duration", r.PauseDur, 0, 0)
	chkSecs(t, "business_pause_duration", r.BusinessPauseDur, 0, 0)
	chkBool(t, "has_breached", r.HasBreached, false)
}

// TestSLACharacterisation_BusinessDays_Resume pins resume: both pause totals
// grow by the finished segment (flat wall-clock and business time), the
// frozen snapshot columns are left as they were, and paused_on is cleared.
func TestSLACharacterisation_BusinessDays_Resume(t *testing.T) {
	e := newSLACharEnv(t)
	start, pausedOn := slaCharBDFixedInstants(t)
	p := e.policy("RESOLUTION", 40*time.Hour, slaCharBusinessDays)
	e.seed(p, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &start, PausedOn: &pausedOn,
		PauseDur: durp(5 * time.Hour), BusinessPauseDur: durp(2 * time.Hour),
		Pct: f64p(12.34), BusinessDur: durp(time.Hour), RemainingDur: durp(9 * time.Hour)})

	before := time.Now()
	if ok, err := e.repo.SetPaused(e.ctx, e.workItemID, "RESOLUTION", false); err != nil || !ok {
		t.Fatalf("SetPaused(false) = %v, %v", ok, err)
	}
	after := time.Now()
	r := e.one(p)
	chkStage(t, "after resume", r, "IN_PROGRESS", true)
	chkNullTime(t, "paused_on", r.PausedOn)
	// The segment ran from the fixed paused_on to now, so these depend on now.
	mid := before.Add(after.Sub(before) / 2)
	chkSecs(t, "pause_duration", r.PauseDur, 5*time.Hour+mid.Sub(pausedOn), slaCharTolerance)
	chkSecs(t, "business_pause_duration", r.BusinessPauseDur, 2*time.Hour+businessdays.Elapsed(pausedOn, mid), slaCharTolerance)
	// Snapshot columns untouched by a resume.
	chkSecs(t, "business_duration (untouched)", r.BusinessDur, time.Hour, 0)
	chkSecs(t, "remaining_business_duration (untouched)", r.RemainingDur, 9*time.Hour, 0)
	chkPct(t, "business_elapsed_percentage (untouched)", r.Pct, 12.34, 0)
}

// TestSLACharacterisation_BusinessDays_CompleteAndCancelOpenPause pins
// completion/cancellation of a clock that is paused at that moment: elapsed
// is exactly Elapsed(start, paused_on) - business_pause_duration (the open
// segment cancels the "now" term), so these values are exact; only the two
// pause totals, which absorb the open segment, depend on now.
func TestSLACharacterisation_BusinessDays_CompleteAndCancelOpenPause(t *testing.T) {
	type op struct {
		name      string
		call      func(e *slaCharEnv) (bool, error)
		wantStage string
	}
	ops := []op{
		{"complete", func(e *slaCharEnv) (bool, error) { return e.repo.CompleteClock(e.ctx, e.workItemID, "RESOLUTION") }, "ACHIEVED"},
		{"cancel", func(e *slaCharEnv) (bool, error) { return e.repo.CancelClock(e.ctx, e.workItemID, "RESOLUTION") }, "CANCELLED"},
	}
	type dc struct {
		name         string
		duration     time.Duration
		wantBreached bool
		wantPct      float64
		wantRemain   time.Duration
	}
	durations := []dc{
		{"within duration", 40 * time.Hour, false, 50, 20 * time.Hour},
		{"past duration", 10 * time.Hour, true, 200, 0},
	}
	for _, o := range ops {
		for _, d := range durations {
			t.Run(o.name+"/"+d.name, func(t *testing.T) {
				e := newSLACharEnv(t)
				start, pausedOn := slaCharBDFixedInstants(t)
				p := e.policy("RESOLUTION", d.duration, slaCharBusinessDays)
				e.seed(p, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &start, PausedOn: &pausedOn,
					PauseDur: durp(5 * time.Hour), BusinessPauseDur: durp(2 * time.Hour),
					Pct: f64p(1), BusinessDur: durp(time.Minute), RemainingDur: durp(time.Minute)})

				before := time.Now()
				if ok, err := o.call(e); err != nil || !ok {
					t.Fatalf("call = %v, %v", ok, err)
				}
				after := time.Now()
				mid := before.Add(after.Sub(before) / 2)
				r := e.one(p)
				chkStage(t, o.name, r, o.wantStage, false)
				chkNullTime(t, "paused_on", r.PausedOn)
				chkTime(t, "end_on", r.EndOn, mid, slaCharTolerance)
				chkSecs(t, "business_duration", r.BusinessDur, 20*time.Hour, 0)
				chkSecs(t, "remaining_business_duration", r.RemainingDur, d.wantRemain, 0)
				chkPct(t, "business_elapsed_percentage", r.Pct, d.wantPct, 0)
				chkBool(t, "has_breached", r.HasBreached, d.wantBreached)
				chkSecs(t, "pause_duration", r.PauseDur, 5*time.Hour+mid.Sub(pausedOn), slaCharTolerance)
				chkSecs(t, "business_pause_duration", r.BusinessPauseDur, 2*time.Hour+businessdays.Elapsed(pausedOn, mid), slaCharTolerance)
			})
		}
	}
}

// TestSLACharacterisation_BusinessDays_CompleteAndCancelRunning pins
// completion/cancellation of a running business-day clock (10 days old, 1h of
// business pause already booked). Depends on now, so the expectation is
// computed with businessdays.Elapsed.
func TestSLACharacterisation_BusinessDays_CompleteAndCancelRunning(t *testing.T) {
	type op struct {
		name      string
		call      func(e *slaCharEnv) (bool, error)
		wantStage string
	}
	ops := []op{
		{"complete", func(e *slaCharEnv) (bool, error) { return e.repo.CompleteClock(e.ctx, e.workItemID, "RESOLUTION") }, "ACHIEVED"},
		{"cancel", func(e *slaCharEnv) (bool, error) { return e.repo.CancelClock(e.ctx, e.workItemID, "RESOLUTION") }, "CANCELLED"},
	}
	for _, o := range ops {
		for _, c := range []struct {
			name     string
			duration time.Duration
			breached bool
		}{{"within duration", 1000 * time.Hour, false}, {"past duration", 8 * time.Hour, true}} {
			t.Run(o.name+"/"+c.name, func(t *testing.T) {
				e := newSLACharEnv(t)
				p := e.policy("RESOLUTION", c.duration, slaCharBusinessDays)
				start := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
				e.seed(p, slaSeed{Stage: "IN_PROGRESS", IsActive: true, StartOn: &start, PauseDur: durp(time.Hour),
					BusinessPauseDur: durp(time.Hour), Pct: f64p(0), BusinessDur: durp(0), RemainingDur: durp(c.duration)})

				if ok, err := o.call(e); err != nil || !ok {
					t.Fatalf("call = %v, %v", ok, err)
				}
				now := time.Now()
				r := e.one(p)
				chkStage(t, o.name, r, o.wantStage, false)
				want := businessdays.Elapsed(start, now) - time.Hour
				chkSecs(t, "business_duration", r.BusinessDur, want, slaCharTolerance)
				remaining := c.duration - want
				if remaining < 0 {
					remaining = 0
				}
				chkSecs(t, "remaining_business_duration", r.RemainingDur, remaining, slaCharTolerance)
				chkPct(t, "business_elapsed_percentage", r.Pct, want.Seconds()/c.duration.Seconds()*100, pctTol(slaCharTolerance, c.duration))
				chkBool(t, "has_breached", r.HasBreached, c.breached)
				// No open pause: both totals are stored back unchanged.
				chkSecs(t, "pause_duration", r.PauseDur, time.Hour, 0)
				chkSecs(t, "business_pause_duration", r.BusinessPauseDur, time.Hour, 0)
			})
		}
	}
}

// ---------------------------------------------------------------------------
// 4. ReviseClocks (a severity change)
// ---------------------------------------------------------------------------

// TestSLACharacterisation_ReviseClocks pins one severity change over a work
// item that holds six clocks in different states, flat and business-day:
//
//	A RESPONSE   flat  IN_PROGRESS, 10h old, 8h policy     -> CANCELLED, breached; replaced
//	B WORKAROUND flat  PAUSED (fixed instants)             -> CANCELLED, exact; NOT in the new list, so not replaced
//	C RESOLUTION BD    PAUSED (fixed instants)             -> CANCELLED, exact; replaced
//	D WORKAROUND BD    BREACHED, 10 days old               -> CANCELLED (workaround/resolution cancel a BREACHED row); replaced
//	E RESPONSE   BD    BREACHED                            -> untouched (RESPONSE keeps BREACHED); NOT replaced
//	F RESPONSE   flat  ACHIEVED                            -> untouched; NOT replaced
func TestSLACharacterisation_ReviseClocks(t *testing.T) {
	e := newSLACharEnv(t)
	fixedStart := time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC)
	fixedPaused := fixedStart.Add(6 * time.Hour)
	bdStart, bdPaused := slaCharBDFixedInstants(t)
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)

	pA := e.policy("RESPONSE", 8*time.Hour, "")
	pB := e.policy("WORKAROUND", 8*time.Hour, "24 x 7")
	pC := e.policy("RESOLUTION", 40*time.Hour, slaCharBusinessDays)
	pD := e.policy("WORKAROUND", 8*time.Hour, slaCharBusinessDays)
	pE := e.policy("RESPONSE", 8*time.Hour, slaCharBusinessDays)
	pF := e.policy("RESPONSE", 8*time.Hour, "")

	e.register(pA)
	e.startAgo(pA, 10*time.Hour)
	e.seed(pB, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &fixedStart, PausedOn: &fixedPaused, PauseDur: durp(time.Hour)})
	e.seed(pC, slaSeed{Stage: "PAUSED", IsActive: true, StartOn: &bdStart, PausedOn: &bdPaused,
		PauseDur: durp(5 * time.Hour), BusinessPauseDur: durp(2 * time.Hour)})
	e.seed(pD, slaSeed{Stage: "BREACHED", IsActive: true, HasBreached: true, StartOn: &tenDaysAgo})
	e.seed(pE, slaSeed{Stage: "BREACHED", IsActive: true, HasBreached: true, StartOn: &tenDaysAgo,
		Pct: f64p(777), BusinessDur: durp(9 * time.Hour), RemainingDur: durp(0)})
	e.seed(pF, slaSeed{Stage: "ACHIEVED", IsActive: false, StartOn: &fixedStart,
		Pct: f64p(55), BusinessDur: durp(4 * time.Hour), RemainingDur: durp(4 * time.Hour)})

	before := time.Now()
	cancelled, err := e.repo.ReviseClocks(e.ctx, e.workItemID, []repository.SLAPolicyRef{pA, pC, pD, pE, pF})
	if err != nil {
		t.Fatalf("ReviseClocks: %v", err)
	}
	after := time.Now()
	mid := before.Add(after.Sub(before) / 2)
	if cancelled != 4 {
		t.Errorf("ReviseClocks cancelled = %d, want 4 (A, B, C, D)", cancelled)
	}

	// A: flat running clock, 10h into an 8h policy.
	rsA := e.rows(pA)
	if len(rsA) != 2 {
		t.Fatalf("A rows = %v, want CANCELLED + fresh IN_PROGRESS", stagesOf(rsA))
	}
	a := rsA["CANCELLED"]
	chkStage(t, "A old", a, "CANCELLED", false)
	chkBool(t, "A old has_breached", a.HasBreached, true)
	chkSecs(t, "A old business_duration", a.BusinessDur, 10*time.Hour, slaCharTolerance)
	chkSecs(t, "A old remaining_business_duration", a.RemainingDur, 0, 0)
	chkPct(t, "A old business_elapsed_percentage", a.Pct, 125, pctTol(slaCharTolerance, 8*time.Hour))
	chkTime(t, "A old end_on", a.EndOn, mid, slaCharTolerance)

	// B: flat PAUSED, fixed instants: elapsed = paused_on - start_on - pause_duration = 5h. Exact.
	rsB := e.rows(pB)
	if len(rsB) != 1 {
		t.Fatalf("B rows = %v, want only CANCELLED (B is not in the new policy list)", stagesOf(rsB))
	}
	b := rsB["CANCELLED"]
	chkStage(t, "B", b, "CANCELLED", false)
	chkSecs(t, "B business_duration", b.BusinessDur, 5*time.Hour, 0)
	chkSecs(t, "B remaining_business_duration", b.RemainingDur, 3*time.Hour, 0)
	chkPct(t, "B business_elapsed_percentage", b.Pct, 62.5, 0)
	chkBool(t, "B has_breached", b.HasBreached, false)
	chkSecs(t, "B pause_duration (open segment finalised)", b.PauseDur, time.Hour+mid.Sub(fixedPaused), slaCharTolerance)
	chkNullTime(t, "B paused_on", b.PausedOn)
	chkNull(t, "B business_pause_duration (flat path never writes it)", b.BusinessPauseDur)

	// C: business-day PAUSED, fixed instants. Exact.
	rsC := e.rows(pC)
	if len(rsC) != 2 {
		t.Fatalf("C rows = %v, want CANCELLED + fresh IN_PROGRESS", stagesOf(rsC))
	}
	c := rsC["CANCELLED"]
	chkStage(t, "C old", c, "CANCELLED", false)
	chkSecs(t, "C old business_duration", c.BusinessDur, 20*time.Hour, 0)
	chkSecs(t, "C old remaining_business_duration", c.RemainingDur, 20*time.Hour, 0)
	chkPct(t, "C old business_elapsed_percentage", c.Pct, 50, 0)
	chkBool(t, "C old has_breached", c.HasBreached, false)
	chkSecs(t, "C old pause_duration", c.PauseDur, 5*time.Hour+mid.Sub(bdPaused), slaCharTolerance)
	chkSecs(t, "C old business_pause_duration", c.BusinessPauseDur, 2*time.Hour+businessdays.Elapsed(bdPaused, mid), slaCharTolerance)

	// D: business-day BREACHED workaround clock is cancelled.
	rsD := e.rows(pD)
	if len(rsD) != 2 {
		t.Fatalf("D rows = %v, want CANCELLED + fresh IN_PROGRESS", stagesOf(rsD))
	}
	d := rsD["CANCELLED"]
	chkStage(t, "D old", d, "CANCELLED", false)
	chkBool(t, "D old has_breached", d.HasBreached, true)
	wantD := businessdays.Elapsed(tenDaysAgo, mid)
	chkSecs(t, "D old business_duration", d.BusinessDur, wantD, slaCharTolerance)
	chkSecs(t, "D old remaining_business_duration", d.RemainingDur, 0, 0)

	// E: RESPONSE BREACHED is never cancelled by a revision, and blocks re-registration.
	rsE := e.rows(pE)
	if len(rsE) != 1 {
		t.Fatalf("E rows = %v, want only the untouched BREACHED row", stagesOf(rsE))
	}
	eRow := rsE["BREACHED"]
	chkStage(t, "E", eRow, "BREACHED", true)
	chkPct(t, "E business_elapsed_percentage (untouched)", eRow.Pct, 777, 0)
	chkSecs(t, "E business_duration (untouched)", eRow.BusinessDur, 9*time.Hour, 0)

	// F: ACHIEVED is untouched and blocks re-registration.
	rsF := e.rows(pF)
	if len(rsF) != 1 {
		t.Fatalf("F rows = %v, want only the untouched ACHIEVED row", stagesOf(rsF))
	}
	f := rsF["ACHIEVED"]
	chkStage(t, "F", f, "ACHIEVED", false)
	chkPct(t, "F business_elapsed_percentage (untouched)", f.Pct, 55, 0)

	// Replacement clocks: IN_PROGRESS, started now, zeroed snapshot, full remaining.
	for name, pair := range map[string]struct {
		p   repository.SLAPolicyRef
		rem time.Duration
	}{"A": {pA, 8 * time.Hour}, "C": {pC, 40 * time.Hour}, "D": {pD, 8 * time.Hour}} {
		r := e.rows(pair.p)["IN_PROGRESS"]
		chkStage(t, name+" new", r, "IN_PROGRESS", true)
		chkTime(t, name+" new start_on", &r.StartOn, mid, slaCharTolerance)
		chkBool(t, name+" new has_breached", r.HasBreached, false)
		chkSecs(t, name+" new business_duration", r.BusinessDur, 0, 0)
		chkSecs(t, name+" new remaining_business_duration", r.RemainingDur, pair.rem, 0)
		chkPct(t, name+" new business_elapsed_percentage", r.Pct, 0, 0)
		chkNull(t, name+" new pause_duration", r.PauseDur)
		chkNull(t, name+" new business_pause_duration", r.BusinessPauseDur)
		chkNullTime(t, name+" new paused_on", r.PausedOn)
		chkNullTime(t, name+" new end_on", r.EndOn)
	}
}

// ---------------------------------------------------------------------------
// 5. sla_live and business_seconds_elapsed
// ---------------------------------------------------------------------------

// slaCharRollback aborts a transaction on purpose so nothing a test seeded
// inside it is kept.
var slaCharRollback = errors.New("roll back")

// TestSLACharacterisation_BusinessSecondsElapsed pins the SQL function the
// view uses, with fixed instants and exact values, and requires it to agree
// with businessdays.Elapsed (its documented Go twin) on every case.
func TestSLACharacterisation_BusinessSecondsElapsed(t *testing.T) {
	pool := slaCharPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	ny := func(s string) time.Time { return slaCharNY(t, s) }
	cases := []struct {
		name       string
		start, end time.Time
		want       time.Duration
	}{
		{"within one weekday", ny("2024-01-01T09:00:00"), ny("2024-01-01T17:00:00"), 8 * time.Hour},
		{"Friday 15:00 to Monday 09:00 skips the weekend", ny("2024-01-05T15:00:00"), ny("2024-01-08T09:00:00"), 18 * time.Hour},
		{"two full business weeks", ny("2024-01-01T00:00:00"), ny("2024-01-15T00:00:00"), 240 * time.Hour},
		{"across the fall-back Sunday (25h day, never counted)", ny("2024-11-01T10:00:00"), ny("2024-11-04T10:00:00"), 24 * time.Hour},
		{"across the spring-forward Sunday (23h day, never counted)", ny("2024-03-08T10:00:00"), ny("2024-03-11T10:00:00"), 24 * time.Hour},
		{"Friday midnight to Tuesday midnight over spring-forward", ny("2024-03-08T00:00:00"), ny("2024-03-12T00:00:00"), 48 * time.Hour},
		{"Sunday 23h day only", ny("2024-03-10T00:00:00"), ny("2024-03-11T00:00:00"), 0},
		{"pure weekend", ny("2024-01-06T00:00:00"), ny("2024-01-08T00:00:00"), 0},
		{"Saturday noon to Monday 06:00", ny("2024-01-06T12:00:00"), ny("2024-01-08T06:00:00"), 6 * time.Hour},
		{"day boundary is New York midnight, not UTC", time.Date(2024, 1, 5, 23, 0, 0, 0, time.UTC), time.Date(2024, 1, 6, 6, 0, 0, 0, time.UTC), 6 * time.Hour},
		{"end before start", ny("2024-01-01T09:00:00"), ny("2024-01-01T08:00:00"), 0},
		{"end equal to start", ny("2024-01-01T09:00:00"), ny("2024-01-01T09:00:00"), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got float64
			if err := scoped.QueryRow(ctx, `SELECT business_seconds_elapsed($1::timestamptz, $2::timestamptz)`, c.start, c.end).Scan(&got); err != nil {
				t.Fatalf("query: %v", err)
			}
			if got != c.want.Seconds() {
				t.Errorf("business_seconds_elapsed = %v, want %v", got, c.want.Seconds())
			}
			if goElapsed := businessdays.Elapsed(c.start, c.end); goElapsed != c.want {
				t.Errorf("businessdays.Elapsed = %v, want %v (Go and SQL must agree)", goElapsed, c.want)
			}
		})
	}
}

// slaLiveCase is one row seeded inside the rolled-back transaction of
// TestSLACharacterisation_SLALive and the view output expected for it.
type slaLiveCase struct {
	name     string
	business bool // policy schedule 'WSO2 Business days' (else '24 x 7')
	stage    string
	startAgo *time.Duration // nil = start_on NULL
	pause    *time.Duration // pause_duration
	bpause   *time.Duration // business_pause_duration
	pausedOn *time.Duration // ago
	stored   struct {       // stored snapshot columns
		breached bool
		pct      *float64
		elapsed  *time.Duration
		remain   *time.Duration
	}
	// expectation. For a business-day row whose elapsed depends on the day of
	// the week, elapsed is derived from businessdays.Elapsed at the
	// transaction's NOW() and wantElapsed is ignored.
	wantStage    string
	wantBreached bool
	wantElapsed  time.Duration
	wantPct      float64
	wantRemain   time.Duration
	// bdComputed: elapsed = max(0, businessdays.Elapsed(start, txNow) - business_pause_duration).
	bdComputed bool
}

// TestSLACharacterisation_SLALive pins the sla_live view for flat and
// business-day policies and every stage. All rows are seeded and read inside
// one transaction, in which NOW() is a constant read back first, so the
// IN_PROGRESS/BREACHED values are exact; the transaction is rolled back.
//
// Policy duration is 10h throughout.
func TestSLACharacterisation_SLALive(t *testing.T) {
	pool := slaCharPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	const dur = 10 * time.Hour
	ago := func(d time.Duration) *time.Duration { return &d }
	mk := func(name string, business bool, stage string, startAgo, pause *time.Duration) slaLiveCase {
		return slaLiveCase{name: name, business: business, stage: stage, startAgo: startAgo, pause: pause}
	}

	var cases []slaLiveCase
	add := func(c slaLiveCase) { cases = append(cases, c) }

	// ---- flat, live stages: elapsed = now - start_on - pause_duration, exactly.
	c := mk("flat IN_PROGRESS inside the duration", false, "IN_PROGRESS", ago(4*time.Hour), ago(time.Hour))
	c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", 3*time.Hour, 30, 7*time.Hour
	add(c)
	c = mk("flat IN_PROGRESS with NULL pause_duration", false, "IN_PROGRESS", ago(5*time.Hour), nil)
	c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", 5*time.Hour, 50, 5*time.Hour
	add(c)
	c = mk("flat IN_PROGRESS past the duration is live-bumped to BREACHED", false, "IN_PROGRESS", ago(12*time.Hour), ago(time.Hour))
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "BREACHED", true, 11*time.Hour, 110, 0
	add(c)
	c = mk("flat IN_PROGRESS exactly at the duration is BREACHED (>=)", false, "IN_PROGRESS", ago(10*time.Hour), nil)
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "BREACHED", true, 10*time.Hour, 100, 0
	add(c)
	c = mk("flat IN_PROGRESS with a stored has_breached = true stays true while inside the duration", false, "IN_PROGRESS", ago(3*time.Hour), nil)
	c.stored.breached = true
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", true, 3*time.Hour, 30, 7*time.Hour
	add(c)
	c = mk("flat IN_PROGRESS pause_duration larger than elapsed floors elapsed at 0", false, "IN_PROGRESS", ago(2*time.Hour), ago(5*time.Hour))
	c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", 0, 0, dur
	add(c)
	c = mk("flat BREACHED past the duration", false, "BREACHED", ago(12*time.Hour), nil)
	c.stored.breached = true
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "BREACHED", true, 12*time.Hour, 120, 0
	add(c)
	// QUIRK: a stored BREACHED row keeps whatever has_breached holds, even false.
	c = mk("flat BREACHED with stored has_breached = false keeps false", false, "BREACHED", ago(12*time.Hour), nil)
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "BREACHED", false, 12*time.Hour, 120, 0
	add(c)
	c = mk("flat BREACHED still inside the duration keeps counting", false, "BREACHED", ago(2*time.Hour), nil)
	c.stored.breached = true
	c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "BREACHED", true, 2*time.Hour, 20, 8*time.Hour
	add(c)

	// ---- non-live stages pass the stored snapshot straight through, for both schedules.
	for _, business := range []bool{false, true} {
		label := map[bool]string{false: "flat", true: "business-day"}[business]
		c = mk(label+" PAUSED passes the stored snapshot through", business, "PAUSED", ago(5*time.Hour), ago(time.Hour))
		c.pausedOn = ago(2 * time.Hour)
		c.stored.pct, c.stored.elapsed, c.stored.remain = f64p(33.33), ago(3*time.Hour+20*time.Minute), ago(6*time.Hour+40*time.Minute)
		c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "PAUSED", 3*time.Hour+20*time.Minute, 33.33, 6*time.Hour+40*time.Minute
		add(c)
		for _, stage := range []string{"ACHIEVED", "COMPLETED", "CANCELLED"} {
			c = mk(label+" "+stage+" passes the stored snapshot through", business, stage, ago(8*time.Hour), nil)
			c.stored.pct, c.stored.elapsed, c.stored.remain = f64p(50), ago(5*time.Hour), ago(5*time.Hour)
			c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = stage, 5*time.Hour, 50, 5*time.Hour
			add(c)
		}
		c = mk(label+" ACHIEVED with stored has_breached = true reports it", business, "ACHIEVED", ago(20*time.Hour), nil)
		c.stored.breached = true
		c.stored.pct, c.stored.elapsed, c.stored.remain = f64p(150), ago(15*time.Hour), ago(0)
		c.wantStage, c.wantBreached, c.wantElapsed, c.wantPct, c.wantRemain = "ACHIEVED", true, 15*time.Hour, 150, 0
		add(c)
		c = mk(label+" IN_PROGRESS with NULL start_on passes the stored snapshot through", business, "IN_PROGRESS", nil, nil)
		c.stored.pct, c.stored.elapsed, c.stored.remain = f64p(7), ago(time.Hour), ago(9*time.Hour)
		c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", time.Hour, 7, 9*time.Hour
		add(c)
	}

	// ---- business-day live stages: elapsed = max(0, business time since start - business_pause_duration);
	// the flat pause_duration (99h here) is ignored.
	c = mk("business-day IN_PROGRESS ignores the flat pause_duration", true, "IN_PROGRESS", ago(4*time.Hour), ago(99*time.Hour))
	c.bpause, c.bdComputed = ago(time.Hour), true
	add(c)
	c = mk("business-day IN_PROGRESS 30 days old is BREACHED", true, "IN_PROGRESS", ago(30*24*time.Hour), nil)
	c.bdComputed, c.wantStage, c.wantBreached = true, "BREACHED", true
	add(c)
	c = mk("business-day IN_PROGRESS with business pause above elapsed floors at 0", true, "IN_PROGRESS", ago(2*time.Hour), nil)
	c.bpause = ago(5 * time.Hour)
	c.wantStage, c.wantElapsed, c.wantPct, c.wantRemain = "IN_PROGRESS", 0, 0, dur
	add(c)
	c = mk("business-day stored BREACHED, 30 days old", true, "BREACHED", ago(30*24*time.Hour), nil)
	c.stored.breached, c.bdComputed, c.wantStage, c.wantBreached = true, true, "BREACHED", true
	add(c)
	c = mk("business-day stored BREACHED with has_breached = false keeps false", true, "BREACHED", ago(30*24*time.Hour), nil)
	c.bdComputed, c.wantStage, c.wantBreached = true, "BREACHED", false
	add(c)

	type got struct {
		stage, liveStage        string
		liveBreached, breached  bool
		pct, elapsed, remaining *float64
	}
	results := make([]got, len(cases))
	var txNow time.Time
	policyF := repository.SLAPolicyRef{}
	policyB := repository.SLAPolicyRef{}

	err := scoped.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT NOW()`).Scan(&txNow); err != nil {
			return err
		}
		var err error
		if policyF, err = insertSLACharPolicy(ctx, tx, slaCharPolicyPrefix+"view flat", "RESOLUTION", dur, "24 x 7"); err != nil {
			return err
		}
		if policyB, err = insertSLACharPolicy(ctx, tx, slaCharPolicyPrefix+"view business", "RESOLUTION", dur, slaCharBusinessDays); err != nil {
			return err
		}
		ids := make([]string, len(cases))
		for i, c := range cases {
			p := policyF
			if c.business {
				p = policyB
			}
			s := slaSeed{Stage: c.stage, IsActive: true, HasBreached: c.stored.breached, PauseDur: c.pause,
				BusinessPauseDur: c.bpause, Pct: c.stored.pct, BusinessDur: c.stored.elapsed, RemainingDur: c.stored.remain}
			if c.startAgo != nil {
				s.StartOn = timep(txNow.Add(-*c.startAgo))
			}
			if c.pausedOn != nil {
				s.PausedOn = timep(txNow.Add(-*c.pausedOn))
			}
			if err := insertSLACharSeed(ctx, tx, nil, p, s); err != nil {
				return fmt.Errorf("seed %q: %w", c.name, err)
			}
			// Identify the row just inserted: the newest one for this policy
			// with this case's index recorded in updated_by.
			if err := tx.QueryRow(ctx, `UPDATE sla SET updated_by = $2 WHERE id = (
					SELECT id FROM sla WHERE sla_policy_id = $1::uuid AND updated_by = $3 LIMIT 1) RETURNING id::text`,
				p.ID, fmt.Sprintf("case-%d", i), slaCharActor).Scan(&ids[i]); err != nil {
				return fmt.Errorf("tag %q: %w", c.name, err)
			}
		}
		for i := range cases {
			var g got
			if err := tx.QueryRow(ctx, `
				SELECT stage::text, live_stage::text, live_has_breached, COALESCE(has_breached, FALSE),
				       live_elapsed_percentage::float8, EXTRACT(EPOCH FROM live_business_duration)::float8,
				       EXTRACT(EPOCH FROM live_remaining_business_duration)::float8
				FROM sla_live WHERE id = $1::uuid`, ids[i]).
				Scan(&g.stage, &g.liveStage, &g.liveBreached, &g.breached, &g.pct, &g.elapsed, &g.remaining); err != nil {
				return fmt.Errorf("read %q: %w", cases[i].name, err)
			}
			results[i] = g
		}
		return slaCharRollback
	})
	if !errors.Is(err, slaCharRollback) {
		t.Fatalf("view transaction: %v", err)
	}

	for i, c := range cases {
		g := results[i]
		t.Run(c.name, func(t *testing.T) {
			wantElapsed, wantPct, wantRemain := c.wantElapsed, c.wantPct, c.wantRemain
			if c.bdComputed {
				// The view's business-day formula, derived from the Go twin at the transaction's NOW().
				start := txNow.Add(-*c.startAgo)
				wantElapsed = businessdays.Elapsed(start, txNow)
				if c.bpause != nil {
					wantElapsed -= *c.bpause
				}
				if wantElapsed < 0 {
					wantElapsed = 0
				}
				wantPct = wantElapsed.Seconds() / dur.Seconds() * 100
				wantRemain = dur - wantElapsed
				if wantRemain < 0 {
					wantRemain = 0
				}
				if c.wantStage == "" {
					c.wantStage = c.stage
					if wantElapsed >= dur {
						c.wantStage = "BREACHED"
					}
					c.wantBreached = wantElapsed >= dur
				}
			}
			if g.stage != c.stage {
				t.Errorf("stored stage = %s, want %s", g.stage, c.stage)
			}
			if g.liveStage != c.wantStage {
				t.Errorf("live_stage = %s, want %s", g.liveStage, c.wantStage)
			}
			if g.liveBreached != c.wantBreached {
				t.Errorf("live_has_breached = %v, want %v", g.liveBreached, c.wantBreached)
			}
			if g.pct == nil || math.Abs(*g.pct-wantPct) > 1e-4 {
				t.Errorf("live_elapsed_percentage = %v, want %v", slaCharDeref(g.pct), wantPct)
			}
			if g.elapsed == nil || math.Abs(*g.elapsed-wantElapsed.Seconds()) > 1e-3 {
				t.Errorf("live_business_duration = %vs, want %vs", slaCharDeref(g.elapsed), wantElapsed.Seconds())
			}
			if g.remaining == nil || math.Abs(*g.remaining-wantRemain.Seconds()) > 1e-3 {
				t.Errorf("live_remaining_business_duration = %vs, want %vs", slaCharDeref(g.remaining), wantRemain.Seconds())
			}
		})
	}
}

func slaCharDeref(f *float64) any {
	if f == nil {
		return "NULL"
	}
	return *f
}

// ---------------------------------------------------------------------------
// 6. ListCandidatePolicies (the resolver's input)
// ---------------------------------------------------------------------------

// TestSLACharacterisation_ListCandidatePolicies pins which sla_policy rows the
// resolver is offered for a target: only source = 'SERVICENOW', active (or
// is_active NULL), with a positive duration, ordered by id; and how NULL
// columns come back (empty strings, retroactive false, default cancel/resume
// rules).
func TestSLACharacterisation_ListCandidatePolicies(t *testing.T) {
	pool := slaCharPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	repo := repository.NewSLAEngineRepository(scoped, "")

	const idPrefix = "5c4a0000-0000-0000-0000-00000000000"
	ids := make([]string, 0, 8)
	for i := 1; i <= 8; i++ {
		ids = append(ids, fmt.Sprintf("%s%d", idPrefix, i))
	}
	cleanup := func() {
		for _, id := range ids {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, id)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	insert := func(id, name, source, target string, active any, duration any, extra string) {
		t.Helper()
		if _, err := scoped.Exec(ctx, `
			INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, source)
			VALUES ($1::uuid, NOW(), NOW(), $2, $2, $3, $4, $5::sla_policy_target_enum, $6::interval, $7::sla_source_enum)`,
			id, slaCharActor, slaCharPolicyPrefix+name, active, target, duration, source); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		if extra != "" {
			if _, err := scoped.Exec(ctx, `UPDATE sla_policy SET `+extra+` WHERE id = $1::uuid`, id); err != nil {
				t.Fatalf("seed %s extra: %v", name, err)
			}
		}
	}
	insert(ids[0], "1 full", "SERVICENOW", "RESPONSE", true, "90 minutes",
		`schedule = 'WSO2 Business days', start_condition = 'state=OPEN', stop_condition = 'state=CLOSED',
		 pause_condition = 'state=AWAITING_INFO', resume_condition = 'state=WORK_IN_PROGRESS', cancel_condition = 'priority=S4',
		 retroactive = TRUE, when_to_cancel = 'START_CONDITIONS_ARE_NOT_MET', when_to_resume = 'PAUSE_CONDITIONS_ARE_NOT_MET'`)
	insert(ids[1], "2 nulls", "SERVICENOW", "RESPONSE", nil, "1 hour", "")
	insert(ids[2], "3 inactive", "SERVICENOW", "RESPONSE", false, "1 hour", "")
	insert(ids[3], "4 no duration", "SERVICENOW", "RESPONSE", true, nil, "")
	insert(ids[4], "5 zero duration", "SERVICENOW", "RESPONSE", true, "0 seconds", "")
	insert(ids[5], "6 csm source", "CSM", "RESPONSE", true, "1 hour", "")
	insert(ids[6], "7 other target", "SERVICENOW", "WORKAROUND", true, "2 hours", "")

	names := func(refs []repository.SLAPolicyRef) []string {
		var out []string
		for _, r := range refs {
			if len(r.Name) >= len(slaCharPolicyPrefix) && r.Name[:len(slaCharPolicyPrefix)] == slaCharPolicyPrefix {
				out = append(out, r.Name[len(slaCharPolicyPrefix):])
			}
		}
		return out
	}
	resp, err := repo.ListCandidatePolicies(ctx, "RESPONSE")
	if err != nil {
		t.Fatalf("ListCandidatePolicies(RESPONSE): %v", err)
	}
	if got := names(resp); fmt.Sprint(got) != "[1 full 2 nulls]" {
		t.Fatalf("RESPONSE candidates = %v, want [1 full 2 nulls] (inactive, no/zero duration and CSM-source rows excluded, ordered by id)", got)
	}
	full, nulls := resp[len(resp)-2], resp[len(resp)-1]

	want := repository.SLAPolicyRef{
		ID: ids[0], Name: slaCharPolicyPrefix + "1 full", Target: "RESPONSE", Duration: 90 * time.Minute,
		Schedule: slaCharBusinessDays, Source: "SERVICENOW",
		StartCondition: "state=OPEN", StopCondition: "state=CLOSED", PauseCondition: "state=AWAITING_INFO",
		ResumeCondition: "state=WORK_IN_PROGRESS", CancelCondition: "priority=S4",
		Retroactive: true, WhenToCancel: "START_CONDITIONS_ARE_NOT_MET", WhenToResume: "PAUSE_CONDITIONS_ARE_NOT_MET",
	}
	if full != want {
		t.Errorf("full row =\n  %+v\nwant\n  %+v", full, want)
	}
	wantNulls := repository.SLAPolicyRef{
		ID: ids[1], Name: slaCharPolicyPrefix + "2 nulls", Target: "RESPONSE", Duration: time.Hour, Source: "SERVICENOW",
		WhenToCancel: "CANCEL_CONDITIONS_ARE_MET", WhenToResume: "RESUME_CONDITIONS_ARE_MET",
	}
	if nulls != wantNulls {
		t.Errorf("NULL-column row =\n  %+v\nwant\n  %+v", nulls, wantNulls)
	}

	work, err := repo.ListCandidatePolicies(ctx, "WORKAROUND")
	if err != nil {
		t.Fatalf("ListCandidatePolicies(WORKAROUND): %v", err)
	}
	if got := names(work); fmt.Sprint(got) != "[7 other target]" {
		t.Errorf("WORKAROUND candidates = %v, want [7 other target]", got)
	}
}
