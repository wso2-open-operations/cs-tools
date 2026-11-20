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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacalendar"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// The conditions migration 0307 seeds, verbatim.
const (
	piSeedStartP1 = "priority=P1^stateNOT INRESOLVED,CLOSED"
	piSeedStartP2 = "priority=P2^stateNOT INRESOLVED,CLOSED"
	piSeedStartP3 = "priority=P3^stateNOT INRESOLVED,CLOSED"
	piSeedStop    = "stateINRESOLVED,CLOSED"
	piSeedPause   = "waits_on=CS"
)

// TestProductIssueSLA_MigrationConditionsParse: the SQL file carries exactly
// these strings, each parses with no unknown clause, and each evaluates as the
// policy intends on sample facts.
func TestProductIssueSLA_MigrationConditionsParse(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0307_product_issue_sla_policies.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, c := range []string{piSeedStartP1, piSeedStartP2, piSeedStartP3, piSeedStop, piSeedPause} {
		if !strings.Contains(sql, "'"+c+"'") {
			t.Errorf("migration does not contain the condition %q verbatim", c)
		}
		if unknown := slacondition.Parse(c).UnknownClauses(); len(unknown) != 0 {
			t.Errorf("condition %q has unknown clauses %v", c, unknown)
		}
	}

	f := func(priority, state, waits string) productIssueFacts {
		return productIssueFacts{priority: priority, state: state, waitsOn: waits, githubState: "OPEN"}
	}
	for _, tc := range []struct {
		cond  string
		facts productIssueFacts
		want  bool
	}{
		{piSeedStartP1, f("P1", "OPEN", "PRODUCT_TEAM"), true},
		{piSeedStartP1, f("P1", "WAITING_ON_CS", "CS"), true},
		{piSeedStartP1, f("P1", "OTHER", "PRODUCT_TEAM"), true},
		{piSeedStartP1, f("P1", "RESOLVED", "NOBODY"), false},
		{piSeedStartP1, f("P1", "CLOSED", "NOBODY"), false},
		{piSeedStartP1, f("P2", "OPEN", "PRODUCT_TEAM"), false},
		{piSeedStartP2, f("P2", "IN_PROGRESS", "PRODUCT_TEAM"), true},
		{piSeedStartP3, f("P3", "REOPENED", "PRODUCT_TEAM"), true},
		{piSeedStartP3, f("P3", "RESOLVED", "NOBODY"), false},
		{piSeedStartP1, f("", "OPEN", "PRODUCT_TEAM"), false},
		{piSeedStartP1, f("Priority/High", "OPEN", "PRODUCT_TEAM"), false},
		{piSeedStop, f("P1", "RESOLVED", "NOBODY"), true},
		{piSeedStop, f("P1", "CLOSED", "NOBODY"), true},
		{piSeedStop, f("P1", "WAITING_ON_CS", "CS"), false},
		{piSeedStop, f("P1", "OTHER", "PRODUCT_TEAM"), false},
		{piSeedPause, f("P1", "WAITING_ON_CS", "CS"), true},
		{piSeedPause, f("P1", "WAITING_ON_PRODUCT_TEAM", "PRODUCT_TEAM"), false},
		{piSeedPause, f("P1", "OTHER", "PRODUCT_TEAM"), false},
	} {
		if got := slacondition.Parse(tc.cond).Evaluate(tc.facts); got != tc.want {
			t.Errorf("%q on %v = %v, want %v", tc.cond, tc.facts, got, tc.want)
		}
	}
}

func TestProductIssueSLA_FactsDerivation(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, tc := range []struct {
		name                string
		in                  repository.ProductIssueSLAFacts
		state, waits, ghStr string
	}{
		{"no link, no board", repository.ProductIssueSLAFacts{}, "OPEN", "PRODUCT_TEAM", "OPEN"},
		{"open on board", repository.ProductIssueSLAFacts{GitHubState: "OPEN", BoardStatus: s("IN_PROGRESS"), BoardWaitsOn: "PRODUCT_TEAM"}, "IN_PROGRESS", "PRODUCT_TEAM", "OPEN"},
		{"waiting on CS", repository.ProductIssueSLAFacts{GitHubState: "OPEN", BoardStatus: s("WAITING_ON_CS"), BoardWaitsOn: "PRODUCT_TEAM"}, "WAITING_ON_CS", "CS", "OPEN"},
		{"github closed wins", repository.ProductIssueSLAFacts{GitHubState: "CLOSED", BoardStatus: s("WAITING_ON_CS")}, "CLOSED", "NOBODY", "CLOSED"},
		{"resolved", repository.ProductIssueSLAFacts{GitHubState: "OPEN", BoardStatus: s("RESOLVED")}, "RESOLVED", "NOBODY", "OPEN"},
		{"status map's own waits_on", repository.ProductIssueSLAFacts{GitHubState: "OPEN", BoardStatus: s("OTHER"), BoardWaitsOn: "CUSTOMER"}, "OTHER", "CUSTOMER", "OPEN"},
	} {
		got := productIssueFactsFrom(tc.in)
		if got.state != tc.state || got.waitsOn != tc.waits || got.githubState != tc.ghStr {
			t.Errorf("%s: got state=%s waits_on=%s github_state=%s", tc.name, got.state, got.waitsOn, got.githubState)
		}
	}
	// A priority label the facts do not have makes = false and != true.
	f := productIssueFactsFrom(repository.ProductIssueSLAFacts{})
	if slacondition.Parse("priority=P1").Evaluate(f) || !slacondition.Parse("priority!=P1").Evaluate(f) {
		t.Error("an absent priority must fail = and pass !=")
	}
}

func TestProductIssueSLASubject_Shape(t *testing.T) {
	var subj slaSubject = productIssueSLASubject{id: "x"}
	if subj.Kind() != "product_issue" || subj.Family() != "PRODUCT_ISSUE" || subj.TerminalNow() || subj.logs().idKey != "workItemId" {
		t.Errorf("unexpected subject shape: %s %s %v %s", subj.Kind(), subj.Family(), subj.TerminalNow(), subj.logs().idKey)
	}
	if got := subj.ConditionText(repository.SLAPolicyRef{Source: "CSM"}, "waits_on=CS"); got != "waits_on=CS" {
		t.Errorf("condition text = %q", got)
	}
	if _, timed := subj.(slaEventTimedSubject); timed {
		t.Error("the plain product issue subject must not be event-timed")
	}
}

// eventTimeRecorder records which repository methods the core calls.
type eventTimeRecorder struct {
	repository.SLAEngineRepository
	active []repository.SLAPolicyRef
	calls  []string
}

func (r *eventTimeRecorder) ActiveClockPolicies(_ context.Context, _, target string) ([]repository.SLAPolicyRef, error) {
	if target != "RESOLUTION" {
		return nil, nil
	}
	return r.active, nil
}
func (r *eventTimeRecorder) CompleteClockByPolicy(context.Context, string, string, string) (bool, error) {
	r.calls = append(r.calls, "complete")
	return true, nil
}
func (r *eventTimeRecorder) CancelClockByPolicy(context.Context, string, string, string) (bool, error) {
	r.calls = append(r.calls, "cancel")
	return true, nil
}
func (r *eventTimeRecorder) SetPausedByPolicy(_ context.Context, _, _, _ string, paused bool) (bool, error) {
	r.calls = append(r.calls, map[bool]string{true: "pause", false: "resume"}[paused])
	return true, nil
}
func (r *eventTimeRecorder) CompleteClockByPolicyAt(_ context.Context, _, _, _ string, at time.Time) (bool, error) {
	r.calls = append(r.calls, "completeAt "+at.Format(time.RFC3339))
	return true, nil
}
func (r *eventTimeRecorder) CancelClockByPolicyAt(_ context.Context, _, _, _ string, at time.Time) (bool, error) {
	r.calls = append(r.calls, "cancelAt "+at.Format(time.RFC3339))
	return true, nil
}
func (r *eventTimeRecorder) SetPausedByPolicyAt(_ context.Context, _, _, _ string, paused bool, at time.Time) (bool, error) {
	r.calls = append(r.calls, map[bool]string{true: "pauseAt ", false: "resumeAt "}[paused]+at.Format(time.RFC3339))
	return true, nil
}

// TestSLACore_EventTimeOnlyForTimedSubjects: a timed subject drives the *At
// methods with its own time; case and incident subjects keep the NOW() methods.
func TestSLACore_EventTimeOnlyForTimedSubjects(t *testing.T) {
	policy := repository.SLAPolicyRef{
		ID: "p", Name: "p", Target: "RESOLUTION", Source: "CSM", WhenToCancel: "NEVER", WhenToResume: "PAUSE_CONDITIONS_ARE_NOT_MET",
		StopCondition: piSeedStop, PauseCondition: piSeedPause,
	}
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	timed := func(state, waits string) productIssueTimedSLASubject {
		return productIssueTimedSLASubject{
			productIssueSLASubject: productIssueSLASubject{id: "w", facts: productIssueFacts{priority: "P1", state: state, waitsOn: waits}},
			at:                     at,
		}
	}
	for _, tc := range []struct {
		state, waits, want string
	}{
		{"WAITING_ON_CS", "CS", "pauseAt 2026-03-02T10:00:00Z"},
		{"IN_PROGRESS", "PRODUCT_TEAM", "resumeAt 2026-03-02T10:00:00Z"},
		{"RESOLVED", "NOBODY", "completeAt 2026-03-02T10:00:00Z"},
	} {
		rec := &eventTimeRecorder{active: []repository.SLAPolicyRef{policy}}
		svc := &slaEngineService{repo: rec}
		if err := svc.applyEffectsToOpenClocks(context.Background(), timed(tc.state, tc.waits)); err != nil {
			t.Fatal(err)
		}
		if len(rec.calls) != 1 || rec.calls[0] != tc.want {
			t.Errorf("%s: calls = %v, want [%s]", tc.state, rec.calls, tc.want)
		}
	}

	// A case subject on the same repository: the old methods, never the At ones.
	caseSubj := caseSLASubject{id: "c", facts: repository.CaseFacts{State: "CLOSED", Severity: "S1", CaseType: "case"}}
	if _, ok := slaSubject(caseSubj).(slaEventTimedSubject); ok {
		t.Fatal("caseSLASubject must not be event-timed")
	}
	if _, ok := slaSubject(incidentSLASubject{}).(slaEventTimedSubject); ok {
		t.Fatal("incidentSLASubject must not be event-timed")
	}
	casePolicy := repository.SLAPolicyRef{ID: "cp", Name: "cp", Target: "RESOLUTION", Source: "CSM", WhenToCancel: "NEVER", StopCondition: "stateINCLOSED"}
	rec := &eventTimeRecorder{active: []repository.SLAPolicyRef{casePolicy}}
	svc := &slaEngineService{repo: rec}
	svc.applyEffectsForPolicy(context.Background(), caseSubj, "RESOLUTION", casePolicy)
	if len(rec.calls) != 1 || rec.calls[0] != "complete" {
		t.Errorf("case subject calls = %v, want [complete] (the NOW() method)", rec.calls)
	}
}

func piAt(day, hour, min int) time.Time {
	return time.Date(2026, 3, day, hour, min, 0, 0, time.UTC)
}

func piIST(day, hour, min int) time.Time {
	return time.Date(2026, 3, day, hour, min, 0, 0, time.FixedZone("IST", 5*3600+30*60))
}

func piPolicy(schedule string, d time.Duration) repository.SLAPolicyRef {
	return repository.SLAPolicyRef{Schedule: schedule, Duration: d, StopCondition: piSeedStop, PauseCondition: piSeedPause}
}

func piBoard(to string, at time.Time) repository.ProductIssueStatusEvent {
	waits := "PRODUCT_TEAM"
	switch to {
	case "WAITING_ON_CS":
		waits = "CS"
	case "RESOLVED":
		waits = "NOBODY"
	}
	return repository.ProductIssueStatusEvent{ToStatus: to, WaitsOn: waits, OccurredAt: at, IsBoardMove: true}
}

func piGH(to string, at time.Time) repository.ProductIssueStatusEvent {
	return repository.ProductIssueStatusEvent{ToStatus: to, WaitsOn: "NOBODY", OccurredAt: at}
}

// TestRebuildProductIssueClock covers the pure rebuild: terminal, paused and
// running clocks, a reopen, and closes interleaved with board moves.
func TestRebuildProductIssueClock(t *testing.T) {
	cur := productIssueFacts{priority: "P1"}
	hr := time.Hour
	p1 := piPolicy(slacalendar.CS24x7, 24*hr)
	created := piAt(6, 10, 0)

	t.Run("resolved after a pause", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(20, 0, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("IN_PROGRESS", piAt(6, 12, 0)), piBoard("WAITING_ON_CS", piAt(6, 14, 0)),
			piBoard("WAITING_ON_PRODUCT_TEAM", piAt(7, 2, 0)), piBoard("RESOLVED", piAt(7, 8, 0)),
		}))
		if st.Stage != "ACHIEVED" || st.EndOn == nil || !st.EndOn.Equal(piAt(7, 8, 0)) || st.PausedOn != nil {
			t.Fatalf("stage/end/paused = %s %v %v", st.Stage, st.EndOn, st.PausedOn)
		}
		if st.PauseDuration != 12*hr || st.BusinessPauseDuration != 12*hr || st.BusinessDuration != 10*hr || st.RemainingDuration != 14*hr || st.HasBreached {
			t.Errorf("figures = %+v", st)
		}
	})
	t.Run("resolved then closed: the clock ends at the first terminal segment", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(20, 0, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("RESOLVED", piAt(6, 14, 0)), piGH("CLOSED", piAt(7, 9, 0)),
		}))
		if st.Stage != "ACHIEVED" || !st.EndOn.Equal(piAt(6, 14, 0)) || st.PauseDuration != 0 || st.BusinessDuration != 4*hr {
			t.Errorf("got %+v end=%v", st, st.EndOn)
		}
	})
	t.Run("currently paused", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(20, 0, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("WAITING_ON_CS", piAt(6, 14, 0)),
		}))
		if st.Stage != "PAUSED" || st.PausedOn == nil || !st.PausedOn.Equal(piAt(6, 14, 0)) || st.EndOn != nil {
			t.Fatalf("stage/paused/end = %s %v %v", st.Stage, st.PausedOn, st.EndOn)
		}
		if st.PauseDuration != 0 || st.BusinessDuration != 4*hr || st.RemainingDuration != 20*hr {
			t.Errorf("figures = %+v", st)
		}
	})
	t.Run("running, reopened after a resolve: the resolved interval does not accrue", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(7, 12, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("RESOLVED", piAt(6, 14, 0)), piBoard("REOPENED", piAt(7, 10, 0)),
		}))
		if st.Stage != "IN_PROGRESS" || st.PauseDuration != 20*hr || st.BusinessDuration != 6*hr || st.ElapsedPercentage != 25 {
			t.Errorf("got %+v", st)
		}
	})
	t.Run("a close and reopen leave the board status as it was", func(t *testing.T) {
		segs := productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("IN_PROGRESS", piAt(6, 11, 0)), piGH("CLOSED", piAt(6, 14, 0)), piGH("REOPENED", piAt(7, 10, 0)),
		})
		last := segs[len(segs)-1].facts
		if last.state != "IN_PROGRESS" || last.githubState != "OPEN" {
			t.Errorf("last facts = %v", last)
		}
		if segs[3].facts.state != "IN_PROGRESS" || segs[2].facts.state != "CLOSED" {
			t.Errorf("segments = %+v", segs)
		}
	})
	t.Run("created already terminal: ended at start, nothing consumed", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(20, 0, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("RESOLVED", piAt(6, 9, 0)),
		}))
		if st.Stage != "ACHIEVED" || !st.EndOn.Equal(created) || st.BusinessDuration != 0 {
			t.Errorf("got %+v", st)
		}
	})
	t.Run("an unknown board status accrues", func(t *testing.T) {
		st := rebuildProductIssueClock(p1, created, piAt(6, 20, 0), productIssueSegments(cur, []repository.ProductIssueStatusEvent{
			piBoard("OTHER", piAt(6, 12, 0)),
		}))
		if st.Stage != "IN_PROGRESS" || st.PauseDuration != 0 || st.BusinessDuration != 10*hr {
			t.Errorf("got %+v", st)
		}
	})
}

// TestProductIssueState_ReopenWithStaleBoardColumn: a GitHub reopen newer than
// the latest board event turns a RESOLVED/CLOSED card into REOPENED, a board
// move after it wins again, and GitHub CLOSED beats everything.
func TestProductIssueState_ReopenWithStaleBoardColumn(t *testing.T) {
	t0 := piAt(6, 10, 0)
	t1, t2 := t0.Add(time.Hour), t0.Add(2*time.Hour)
	for _, tc := range []struct {
		name            string
		gh, board       string
		boardAt, reopen *time.Time
		want            string
	}{
		{"reopen newer than the Resolved card", "OPEN", "RESOLVED", &t0, &t1, "REOPENED"},
		{"card moved after the reopen", "OPEN", "IN_PROGRESS", &t2, &t1, "IN_PROGRESS"},
		{"card back on Resolved after the reopen", "OPEN", "RESOLVED", &t2, &t1, "RESOLVED"},
		{"reopen but the card is in progress", "OPEN", "IN_PROGRESS", &t0, &t1, "IN_PROGRESS"},
		{"reopen, no board event at all, board Resolved", "OPEN", "RESOLVED", nil, &t1, "REOPENED"},
		{"closed again", "CLOSED", "RESOLVED", &t0, &t1, "CLOSED"},
		{"no reopen", "OPEN", "RESOLVED", &t0, nil, "RESOLVED"},
	} {
		if got := productIssueState(tc.gh, tc.board, tc.boardAt, tc.reopen); got != tc.want {
			t.Errorf("%s: state = %s, want %s", tc.name, got, tc.want)
		}
	}
	// The replay applies the same rule per segment.
	segs := productIssueSegments(productIssueFacts{priority: "P1"}, []repository.ProductIssueStatusEvent{
		piBoard("RESOLVED", t0), piGH("CLOSED", t0.Add(30*time.Minute)), piGH("REOPENED", t1), piBoard("IN_PROGRESS", t2),
	})
	var got []string
	for _, sg := range segs {
		got = append(got, sg.facts.state+"/"+sg.facts.waitsOn)
	}
	want := []string{"OPEN/PRODUCT_TEAM", "RESOLVED/NOBODY", "CLOSED/NOBODY", "REOPENED/PRODUCT_TEAM", "IN_PROGRESS/PRODUCT_TEAM"}
	if len(got) != len(want) {
		t.Fatalf("segments = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("segment %d = %s, want %s (all %v)", i, got[i], want[i], got)
		}
	}
}

// piFakeEngineRepo is eventTimeRecorder plus the family listing and a no-op register.
type piFakeEngineRepo struct {
	*eventTimeRecorder
	family []repository.SLAPolicyRef
}

func (r *piFakeEngineRepo) ListFamilyCandidatePolicies(_ context.Context, target, _ string) ([]repository.SLAPolicyRef, error) {
	if target != "RESOLUTION" {
		return nil, nil
	}
	return r.family, nil
}
func (r *piFakeEngineRepo) RegisterWorkItemClock(context.Context, string, repository.SLAPolicyRef, time.Time) (bool, error) {
	return false, nil
}

type piFakePIRepo struct {
	facts repository.ProductIssueSLAFacts
}

func (r piFakePIRepo) GetProductIssueSLAFacts(context.Context, string) (repository.ProductIssueSLAFacts, error) {
	return r.facts, nil
}
func (r piFakePIRepo) WithProductIssueLock(ctx context.Context, _ string, fn func(context.Context, repository.ProductIssueSLATx) error) error {
	return fn(ctx, piFakeTx(r))
}

type piFakeTx struct {
	facts repository.ProductIssueSLAFacts
}

func (t piFakeTx) Facts(context.Context) (repository.ProductIssueSLAFacts, error) {
	return t.facts, nil
}
func (t piFakeTx) Timeline(context.Context) ([]repository.ProductIssueStatusEvent, error) {
	return nil, nil
}
func (t piFakeTx) LatestClockRow(context.Context, string) (repository.ProductIssueClockRow, bool, error) {
	return repository.ProductIssueClockRow{}, false, nil
}
func (t piFakeTx) RegisterClock(context.Context, repository.SLAPolicyRef, time.Time) (bool, error) {
	return false, nil
}
func (t piFakeTx) RewriteClock(context.Context, string, repository.ProductIssueClockState) error {
	return nil
}

// TestApplyWorkItemStatusEffects_WritesAtTheEventTime: the inline update uses the
// event's own time, never the processing time (the final locked rebuild would
// otherwise hide a wrong inline time from the integration tests).
func TestApplyWorkItemStatusEffects_WritesAtTheEventTime(t *testing.T) {
	policy := repository.SLAPolicyRef{
		ID: "p1", Name: "Product Issue P1", Target: "RESOLUTION", Source: "CSM", AppliesTo: "PRODUCT_ISSUE",
		WhenToCancel: "NEVER", WhenToResume: "PAUSE_CONDITIONS_ARE_NOT_MET",
		StartCondition: piSeedStartP1, StopCondition: piSeedStop, PauseCondition: piSeedPause,
	}
	rec := &eventTimeRecorder{active: []repository.SLAPolicyRef{policy}}
	board := "WAITING_ON_CS"
	prio := "P1"
	pi := piFakePIRepo{facts: repository.ProductIssueSLAFacts{Priority: &prio, GitHubState: "OPEN", BoardStatus: &board, CreatedOn: piAt(6, 10, 0)}}
	processing := piAt(20, 0, 0)
	svc := &slaEngineService{repo: &piFakeEngineRepo{eventTimeRecorder: rec, family: []repository.SLAPolicyRef{policy}}, pi: pi, now: func() time.Time { return processing }}
	svc.resolver = newSLAPolicyResolver(svc.repo)

	if err := svc.ApplyWorkItemStatusEffects(context.Background(), "w", piAt(6, 14, 0), true); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 1 || rec.calls[0] != "pauseAt 2026-03-06T14:00:00Z" {
		t.Errorf("calls = %v, want one pauseAt at the event's time", rec.calls)
	}
}

// TestPiWithCurrentState: the synthetic final segment and its start instant.
func TestPiWithCurrentState(t *testing.T) {
	created := piAt(6, 10, 0)
	last := piAt(6, 12, 0)
	closedOn, early, updated, future := piAt(6, 16, 0), piAt(6, 9, 0), piAt(6, 13, 0), piAt(30, 0, 0)
	now := piAt(7, 0, 0)
	ev := []repository.ProductIssueStatusEvent{piBoard("IN_PROGRESS", last)}
	base := productIssueSegments(productIssueFacts{priority: "P1"}, ev)
	cur := func(state string) productIssueFacts {
		gh := "OPEN"
		if state == "CLOSED" {
			gh = "CLOSED"
		}
		return productIssueFacts{priority: "P1", state: state, githubState: gh, waitsOn: productIssueWaitsOn(state, "")}
	}
	for _, tc := range []struct {
		name        string
		cur         productIssueFacts
		closed, upd *time.Time
		wantExtra   bool
		wantFrom    time.Time
	}{
		{"same state: nothing added", cur("IN_PROGRESS"), nil, nil, false, time.Time{}},
		{"closed: github_closed_on", cur("CLOSED"), &closedOn, nil, true, closedOn},
		{"closed: closed_on before the last event is clamped", cur("CLOSED"), &early, nil, true, last},
		{"closed: no closed_on", cur("CLOSED"), nil, nil, true, last},
		{"board differs: last event", cur("WAITING_ON_CS"), nil, nil, true, last},
		{"board differs: updated_on when later", cur("RESOLVED"), nil, &updated, true, updated},
		{"board differs: updated_on in the future ignored", cur("RESOLVED"), nil, &future, true, last},
	} {
		got := piWithCurrentState(base, tc.cur, ev, created, now, tc.closed, tc.upd)
		if (len(got) == len(base)+1) != tc.wantExtra {
			t.Errorf("%s: %d segments", tc.name, len(got))
			continue
		}
		if tc.wantExtra && (!got[len(got)-1].from.Equal(tc.wantFrom) || got[len(got)-1].facts.state != tc.cur.state) {
			t.Errorf("%s: final segment from %v state %s, want %v %s", tc.name, got[len(got)-1].from, got[len(got)-1].facts.state, tc.wantFrom, tc.cur.state)
		}
	}
	// No events at all: not before creation.
	empty := piWithCurrentState(productIssueSegments(productIssueFacts{}, nil), cur("RESOLVED"), nil, created, now, nil, nil)
	if len(empty) != 2 || !empty[1].from.Equal(created) {
		t.Errorf("empty timeline: %+v", empty)
	}
}
