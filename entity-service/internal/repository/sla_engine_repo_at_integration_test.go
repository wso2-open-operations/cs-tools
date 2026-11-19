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

// Integration tests (ENTITY_TEST_DATABASE_URL / CASE_STATS_TEST_DSN) for the
// explicit-time "At" variants and RegisterWorkItemClock. They run on the
// non-flat calendars "CS 12x5 IST" and "CS 24x7" with fixed instants so every
// figure is exact.

package repository_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacalendar"
)

const (
	slaAtActor     = "sla-at-test"
	slaAtPolicy12  = "5c4b0000-0000-0000-0000-000000000001"
	slaAtPolicy247 = "5c4b0000-0000-0000-0000-000000000002"
	slaAtPolicyFlt = "5c4b0000-0000-0000-0000-000000000003"
)

type slaAtEnv struct {
	ctx    context.Context
	repo   repository.SLAEngineRepository
	scoped *repository.Scoped
}

func newSLAAtEnv(t *testing.T) *slaAtEnv {
	t.Helper()
	pool := caseStatsPool(t)
	seedSLAEngineWorkItem(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, id := range []string{slaAtPolicy12, slaAtPolicy247, slaAtPolicyFlt} {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla WHERE sla_policy_id = $1::uuid`, id)
			_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, id)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	return &slaAtEnv{ctx: ctx, repo: repository.NewSLAEngineRepository(scoped, ""), scoped: scoped}
}

// policy seeds a CSM / PRODUCT_ISSUE RESOLUTION policy with a 4 hour duration.
func (e *slaAtEnv) policy(t *testing.T, id, name, schedule string) repository.SLAPolicyRef {
	t.Helper()
	if _, err := e.scoped.Exec(e.ctx, `
		INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, schedule, source, applies_to)
		VALUES ($1::uuid, NOW(), NOW(), $2, $2, $3, TRUE, 'RESOLUTION', INTERVAL '4 hours', $4, 'CSM'::sla_source_enum, 'PRODUCT_ISSUE')`,
		id, slaAtActor, "SLA at test "+name, schedule); err != nil {
		t.Fatalf("seed policy %s: %v", name, err)
	}
	return repository.SLAPolicyRef{ID: id, Name: name, Target: "RESOLUTION", Duration: 4 * time.Hour, Schedule: schedule, Source: "CSM", AppliesTo: "PRODUCT_ISSUE"}
}

type slaAtRow struct {
	stage                               string
	hasBreached                         bool
	pausedOn, endOn                     *time.Time
	pauseSecs, businessPauseSecs        float64
	elapsedSecs, remainingSecs, percent float64
}

func (e *slaAtEnv) row(t *testing.T, policyID string) slaAtRow {
	t.Helper()
	var r slaAtRow
	if err := e.scoped.QueryRow(e.ctx, `
		SELECT stage::TEXT, COALESCE(has_breached, FALSE), paused_on, end_on,
		       COALESCE(EXTRACT(EPOCH FROM pause_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM business_pause_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM business_duration), 0)::float8,
		       COALESCE(EXTRACT(EPOCH FROM remaining_business_duration), 0)::float8,
		       COALESCE(business_elapsed_percentage, 0)::float8
		FROM sla WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid
		ORDER BY created_on DESC LIMIT 1`, slaEngineIntegrationWorkItemID, policyID).
		Scan(&r.stage, &r.hasBreached, &r.pausedOn, &r.endOn, &r.pauseSecs, &r.businessPauseSecs, &r.elapsedSecs, &r.remainingSecs, &r.percent); err != nil {
		t.Fatalf("read sla row: %v", err)
	}
	return r
}

func (e *slaAtEnv) count(t *testing.T, policyID string) int {
	t.Helper()
	var n int
	if err := e.scoped.QueryRow(e.ctx, `SELECT COUNT(*) FROM sla WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid`,
		slaEngineIntegrationWorkItemID, policyID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func (r slaAtRow) expect(t *testing.T, label, stage string, breached bool, pause, bpause, elapsed, remaining, pct float64) {
	t.Helper()
	if r.stage != stage || r.hasBreached != breached || !near(r.pauseSecs, pause) || !near(r.businessPauseSecs, bpause) ||
		!near(r.elapsedSecs, elapsed) || !near(r.remainingSecs, remaining) || !near(r.percent, pct) {
		t.Errorf("%s: got stage=%s breached=%v pause=%.0f bpause=%.0f elapsed=%.0f remaining=%.0f pct=%.2f; want stage=%s breached=%v pause=%.0f bpause=%.0f elapsed=%.0f remaining=%.0f pct=%.2f",
			label, r.stage, r.hasBreached, r.pauseSecs, r.businessPauseSecs, r.elapsedSecs, r.remainingSecs, r.percent,
			stage, breached, pause, bpause, elapsed, remaining, pct)
	}
}

func istAt(d, h, m int) time.Time {
	return time.Date(2026, 3, d, h, m, 0, 0, time.FixedZone("IST", 5*3600+30*60))
}

const hr = 3600.0

func TestSLAEngineIntegration_AtVariants_CS12x5IST(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicy12, "12x5", slacalendar.CS12x5IST)
	start := istAt(2, 10, 0) // Monday
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || !ok {
		t.Fatalf("register: %v %v", ok, err)
	}
	t1, t2, t3 := istAt(2, 12, 0), istAt(3, 10, 0), istAt(3, 12, 0) // Mon 12:00, Tue 10:00, Tue 12:00

	if ok, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, t1); err != nil || !ok {
		t.Fatalf("pause: %v %v", ok, err)
	}
	r := e.row(t, p.ID)
	r.expect(t, "paused", "PAUSED", false, 0, 0, 2*hr, 2*hr, 50)
	if r.pausedOn == nil || !r.pausedOn.Equal(t1) {
		t.Errorf("paused_on = %v want %v", r.pausedOn, t1)
	}

	if ok, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, false, t2); err != nil || !ok {
		t.Fatalf("resume: %v %v", ok, err)
	}
	// Flat pause Mon 12:00 -> Tue 10:00 = 22h; business pause = Mon 12-21 (9h) + Tue 9-10 (1h) = 10h.
	e.row(t, p.ID).expect(t, "resumed", "IN_PROGRESS", false, 22*hr, 10*hr, 2*hr, 2*hr, 50)

	if ok, err := e.repo.CompleteClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, t3); err != nil || !ok {
		t.Fatalf("complete: %v %v", ok, err)
	}
	// Business elapsed Mon 11h + Tue 3h = 14h, minus 10h pause = 4h = the whole duration.
	r = e.row(t, p.ID)
	r.expect(t, "completed", "ACHIEVED", true, 22*hr, 10*hr, 4*hr, 0, 100)
	if r.endOn == nil || !r.endOn.Equal(t3) {
		t.Errorf("end_on = %v want %v", r.endOn, t3)
	}
}

func TestSLAEngineIntegration_AtVariants_CS24x7(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicy247, "24x7", slacalendar.CS24x7)
	start := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC) // Saturday: wall clock ignores weekends
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || !ok {
		t.Fatalf("register: %v %v", ok, err)
	}
	t1, t2, t3 := start.Add(1*time.Hour), start.Add(3*time.Hour), start.Add(5*time.Hour)

	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, t1); err != nil {
		t.Fatal(err)
	}
	e.row(t, p.ID).expect(t, "paused", "PAUSED", false, 0, 0, 1*hr, 3*hr, 25)
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, false, t2); err != nil {
		t.Fatal(err)
	}
	e.row(t, p.ID).expect(t, "resumed", "IN_PROGRESS", false, 2*hr, 2*hr, 1*hr, 3*hr, 25)
	if ok, err := e.repo.CompleteClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, t3); err != nil || !ok {
		t.Fatalf("complete: %v %v", ok, err)
	}
	// 5h wall clock - 2h pause = 3h of 4h.
	e.row(t, p.ID).expect(t, "completed", "ACHIEVED", false, 2*hr, 2*hr, 3*hr, 1*hr, 75)
}

func TestSLAEngineIntegration_AtVariants_Cancel(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicy247, "24x7", slacalendar.CS24x7)
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	if _, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil {
		t.Fatal(err)
	}
	// Cancelled while paused: the open pause segment is finalized at `at`.
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	at := start.Add(2 * time.Hour)
	if ok, err := e.repo.CancelClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, at); err != nil || !ok {
		t.Fatalf("cancel: %v %v", ok, err)
	}
	r := e.row(t, p.ID)
	r.expect(t, "cancelled", "CANCELLED", false, 1*hr, 1*hr, 1*hr, 3*hr, 25)
	if r.endOn == nil || !r.endOn.Equal(at) || r.pausedOn != nil {
		t.Errorf("end_on=%v paused_on=%v", r.endOn, r.pausedOn)
	}
}

func TestSLAEngineIntegration_AtVariants_EarlierThanStartOrPause(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicy247, "24x7", slacalendar.CS24x7)
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	// Complete before the clock even started: elapsed clamps at 0.
	if _, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.repo.CompleteClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, start.Add(-time.Hour)); err != nil || !ok {
		t.Fatalf("complete: %v %v", ok, err)
	}
	e.row(t, p.ID).expect(t, "complete before start", "ACHIEVED", false, 0, 0, 0, 4*hr, 0)

	// Resume earlier than the pause began: the pause segment clamps at 0, never negative.
	if _, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, false, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	e.row(t, p.ID).expect(t, "resume before pause", "IN_PROGRESS", false, 0, 0, 2*hr, 2*hr, 50)

	// Pause earlier than start_on: frozen elapsed clamps at 0.
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, start.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	e.row(t, p.ID).expect(t, "pause before start", "PAUSED", false, 0, 0, 0, 4*hr, 0)
}

func TestSLAEngineIntegration_RegisterWorkItemClock(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicy247, "24x7", slacalendar.CS24x7)
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || !ok {
		t.Fatalf("first register: %v %v", ok, err)
	}
	// An open clock blocks a second one.
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || ok {
		t.Fatalf("register over open row: ok=%v err=%v, want false", ok, err)
	}
	// A paused clock is still open and still blocks.
	if _, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); ok {
		t.Fatal("register over paused row must not insert")
	}
	if _, err := e.repo.CompleteClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// RegisterClock still refuses to re-open a finished RESOLUTION clock...
	if ok, err := e.repo.RegisterClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || ok {
		t.Fatalf("RegisterClock after ACHIEVED: ok=%v err=%v, want false", ok, err)
	}
	// ...RegisterWorkItemClock registers a second row.
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, time.Time{}); err != nil || !ok {
		t.Fatalf("register after ACHIEVED: ok=%v err=%v, want true", ok, err)
	}
	if n := e.count(t, p.ID); n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
	// Cancelled earlier row does not block either.
	if _, err := e.repo.CancelClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || !ok {
		t.Fatalf("register after CANCELLED: ok=%v err=%v, want true", ok, err)
	}
	if n := e.count(t, p.ID); n != 3 {
		t.Errorf("rows = %d, want 3", n)
	}
}

func TestSLAEngineIntegration_AtVariants_LeaveFlatClockUntouched(t *testing.T) {
	e := newSLAAtEnv(t)
	p := e.policy(t, slaAtPolicyFlt, "flat", slacalendar.Flat24x7)
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	if ok, err := e.repo.RegisterWorkItemClock(e.ctx, slaEngineIntegrationWorkItemID, p, start); err != nil || !ok {
		t.Fatalf("register: %v %v", ok, err)
	}
	at := start.Add(time.Hour)
	if ok, err := e.repo.SetPausedByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, true, at); err != nil || ok {
		t.Errorf("pause on flat: ok=%v err=%v, want false", ok, err)
	}
	if ok, err := e.repo.CompleteClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, at); err != nil || ok {
		t.Errorf("complete on flat: ok=%v err=%v, want false", ok, err)
	}
	if ok, err := e.repo.CancelClockByPolicyAt(e.ctx, slaEngineIntegrationWorkItemID, "RESOLUTION", p.ID, at); err != nil || ok {
		t.Errorf("cancel on flat: ok=%v err=%v, want false", ok, err)
	}
	r := e.row(t, p.ID)
	r.expect(t, "flat untouched", "IN_PROGRESS", false, 0, 0, 0, 4*hr, 0)
	if r.pausedOn != nil || r.endOn != nil {
		t.Errorf("flat row modified: paused_on=%v end_on=%v", r.pausedOn, r.endOn)
	}
}
