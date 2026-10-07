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

package paging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// ScheduleCatalogue answers with the SRE windows as migration 0153 seeds
// them, plus one CRE window, whatever the stub holds otherwise. The windows
// are reference data rather than something a test varies.
func (s *stubScheduleReader) ScheduleCatalogue(context.Context) (scheduleCatalogue, error) {
	var cat scheduleCatalogue
	for _, t := range []struct{ key, family string }{
		{"apollo", "SRE"}, {"artemis", "SRE"}, {"atlas", "CRE"},
	} {
		cat.Teams = append(cat.Teams, struct {
			Key    string `json:"key"`
			Name   string `json:"name"`
			Family string `json:"family"`
		}{t.key, t.key, t.family})
	}
	str := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	for _, sh := range []struct{ code, family, zone, tier string }{
		{"SRE_TZ1_L1", "SRE", "TZ1", "L1"},
		{"SRE_TZ1", "SRE", "TZ1", ""},
		{"SRE_TZ2_L1", "SRE", "TZ2", "L1"},
		{"SRE_TZ2", "SRE", "TZ2", ""},
		{"SRE_TZ3", "SRE", "TZ3", ""},
		{"CRE_MORNING", "CRE", "", ""},
	} {
		cat.Shifts = append(cat.Shifts, struct {
			Code     string  `json:"code"`
			Family   string  `json:"family"`
			ZoneCode *string `json:"zoneCode,omitempty"`
			Tier     *string `json:"tier,omitempty"`
		}{sh.code, sh.family, str(sh.zone), str(sh.tier)})
	}
	return cat, nil
}

// sreTeams is the SRE half of the configuration: the two SRE-ABTs.
var sreTeams = TeamKeys{
	ABTs:       []string{"castor", "draco", "vega", "sirius", "atlas", "phoenix", "rigel"},
	Leadership: "cre-leadership",
	SRE:        []string{"apollo", "artemis"},
	Aliases:    map[string]string{"SRE - Apollo": "apollo"},
}

func sreResolver(s *stubScheduleReader) TeamScheduleResolver {
	return NewTeamScheduleResolver(s, sreTeams, nil)
}

// held is one on-duty turn. tier "" takes the window's own.
func held(user, team, shift, tier string) onDutyAssignment {
	a := onDutyFor(user, user+"@example.com", team)
	a.Engineer.Name = user
	a.ShiftCode = shift
	if tier != "" {
		a.Tier = &tier
	}
	return a
}

// morningRota is 10:00 on a weekday: TZ1 alone, both teams on it.
func morningRota() *stubScheduleReader {
	return &stubScheduleReader{
		onDuty: []onDutyAssignment{
			held("a-l1", "apollo", "SRE_TZ1_L1", ""),
			held("r-l1", "artemis", "SRE_TZ1_L1", ""),
			held("a-l2", "apollo", "SRE_TZ1", "L2"),
			held("r-l2", "artemis", "SRE_TZ1", "L2"),
			held("r-l3", "artemis", "SRE_TZ1", "L3"), // apollo has no L3 today
			held("c-l1", "atlas", "CRE_MORNING", "L1"),
		},
		members: []teamMember{
			member("apollo", "a-lead@example.com", roleLead, ""),
			member("artemis", "r-lead@example.com", roleLead, ""),
			// Atlas's alert-duty nominee and lead, so a CRE ladder on the
			// same rota has somebody to reach.
			member("atlas", "atlas-t1@example.com", "engineer", "T1"),
			member("atlas", "atlas-lead@example.com", roleLead, ""),
		},
	}
}

func resolveSRE(t *testing.T, r TeamScheduleResolver, level Level, team string) []string {
	t.Helper()
	got, err := r.Resolve(context.Background(), level, RoutingContext{AssignedCRETeam: team, Ladder: LadderSRE, At: testClock})
	if err != nil {
		t.Fatalf("resolve %s: %v", level, err)
	}
	return emails(got)
}

func TestSRETiming_DefaultsAndOverrides(t *testing.T) {
	def := SREPolicy(false)
	var at []time.Duration
	for _, a := range Schedule(def, true) {
		at = append(at, a.After)
	}
	if len(at) != 3 || at[0] != 0 || at[1] != 5*time.Minute || at[2] != 10*time.Minute {
		t.Fatalf("default SRE clock = %v, want [0 5m 10m]: L1 at once, 5 minutes between two calls", at)
	}
	if n := len(Schedule(SREPolicy(true), true)); n != 4 {
		t.Fatalf("with L4: %d calls, want 4", n)
	}
	custom := SRETiming{InitialWait: Duration(time.Minute), Interval: Duration(2 * time.Minute)}.Policy()
	last := Schedule(custom, true)[2].After
	if last != 5*time.Minute {
		t.Fatalf("1m wait + two 2m gaps: L3 at %v, want 5m", last)
	}
}

// No priority gate: a PLANNING incident has no CRE ladder, but an SRE one
// still gets its clock.
func TestPolicyFor_SREHasNoPriorityGate(t *testing.T) {
	if _, ok := PolicyFor(DefaultPolicy, Trigger{Priority: "PLANNING"}); ok {
		t.Fatal("a PLANNING CRE incident must still have no ladder")
	}
	p, ok := PolicyFor(DefaultPolicy, Trigger{Priority: "PLANNING", Routing: RoutingContext{Ladder: LadderSRE}})
	if !ok || p.Levels[Level1].NotificationInterval != 5*time.Minute {
		t.Fatalf("SRE PLANNING incident: ok=%v policy=%+v; want the SRE clock", ok, p)
	}
}

func TestLadderFor_ConfiguredTeamsAndAliases(t *testing.T) {
	r := sreResolver(&stubScheduleReader{})
	for team, want := range map[string]Ladder{
		"Apollo": LadderSRE, "SRE - Apollo": LadderSRE, "artemis": LadderSRE,
		"Atlas": LadderCRE, "": LadderCRE, "nobody-knows": LadderCRE,
	} {
		got, err := r.LadderFor(context.Background(), RoutingContext{AssignedCRETeam: team})
		if err != nil || got != want {
			t.Errorf("LadderFor(%q) = %q, %v; want %q", team, got, err, want)
		}
	}
}

// With no SRE list configured the catalogue's family still answers, so a
// deployment without the file classifies.
func TestLadderFor_FallsBackToTheCatalogue(t *testing.T) {
	r := NewTeamScheduleResolver(&stubScheduleReader{}, TeamKeys{}, nil)
	if got, _ := r.LadderFor(context.Background(), RoutingContext{AssignedCRETeam: "apollo"}); got != LadderSRE {
		t.Fatalf("apollo = %q, want SRE from the catalogue", got)
	}
	if got, _ := r.LadderFor(context.Background(), RoutingContext{AssignedCRETeam: "atlas"}); got != LadderCRE {
		t.Fatalf("atlas = %q, want CRE", got)
	}
}

func TestResolveSRE_OnePersonPerRungOwnTeamFirst(t *testing.T) {
	r := sreResolver(morningRota())
	for level, want := range map[Level]string{
		Level0: "a-l1@example.com",
		Level1: "a-l2@example.com",
		// apollo has nobody on L3: another SRE team's holder answers.
		Level2: "r-l3@example.com",
		Level3: "a-lead@example.com",
	} {
		got := resolveSRE(t, r, level, "Apollo")
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want exactly [%s]", level, got, want)
		}
	}
	if got := resolveSRE(t, r, Level4, "Apollo"); len(got) != 0 {
		t.Errorf("LEVEL_4 = %v; the SRE ladder has no fifth rung", got)
	}
}

// Weekdays 12:00-15:00 both zones' escalation windows are live. The SRE team
// confirmed one person per rung: the zone whose L1 block is live owns it.
func TestResolveSRE_OverlapCallsOnlyTheL1Zone(t *testing.T) {
	overlap := func(l1Shift string) *stubScheduleReader {
		return &stubScheduleReader{onDuty: []onDutyAssignment{
			held("a-l1", "apollo", l1Shift, ""),
			held("a-tz1-l2", "apollo", "SRE_TZ1", "L2"),
			held("a-tz2-l2", "apollo", "SRE_TZ2", "L2"),
		}}
	}
	if got := resolveSRE(t, sreResolver(overlap("SRE_TZ1_L1")), Level1, "apollo"); len(got) != 1 || got[0] != "a-tz1-l2@example.com" {
		t.Errorf("12:00-13:30 (TZ1 holds L1): L2 = %v, want only TZ1's", got)
	}
	if got := resolveSRE(t, sreResolver(overlap("SRE_TZ2_L1")), Level1, "apollo"); len(got) != 1 || got[0] != "a-tz2-l2@example.com" {
		t.Errorf("13:30-15:00 (TZ2 holds L1): L2 = %v, want only TZ2's", got)
	}
}

// A CRE P0 on the SRE ladder belongs to no SRE team: one person still, in the
// configured team order, and L4 is the lead of whoever took L1.
func TestResolveSRE_CREIncidentStillReachesOnePerson(t *testing.T) {
	r := sreResolver(morningRota())
	if got := resolveSRE(t, r, Level0, "Atlas"); len(got) != 1 || got[0] != "a-l1@example.com" {
		t.Errorf("L1 = %v, want apollo's (first in teams.sre)", got)
	}
	if got := resolveSRE(t, r, Level3, "Atlas"); len(got) != 1 || got[0] != "a-lead@example.com" {
		t.Errorf("L4 = %v, want the lead of the team that took L1", got)
	}
	if got := resolveSRE(t, r, Level0, "Atlas"); strings.Contains(strings.Join(got, ","), "c-l1") {
		t.Error("a CRE window must never answer an SRE rung")
	}
}

func TestResolveSRE_NobodyOnTheTierClimbs(t *testing.T) {
	got := resolveSRE(t, sreResolver(&stubScheduleReader{}), Level0, "apollo")
	if len(got) != 0 {
		t.Fatalf("got %v; want nobody and no error, so the ladder climbs", got)
	}
}

// handoverReader is a rota that changes hands at one instant: before it the
// stub's own holders are on duty, from it the after set.
type handoverReader struct {
	*stubScheduleReader
	handover time.Time
	after    []onDutyAssignment
}

func (h handoverReader) OnDutyAt(ctx context.Context, at time.Time) ([]onDutyAssignment, error) {
	if at.Before(h.handover) {
		return h.stubScheduleReader.OnDutyAt(ctx, at)
	}
	return h.after, nil
}

// An incident reported at 13:25 opens L2 at 13:30 and L3 at 13:35, both
// inside TZ2. Each rung must reach TZ2's holder, not whoever held the tier on
// TZ1 when the incident arrived -- and a tier TZ1 had nobody on must still
// find TZ2's holder rather than leave the rung empty.
func TestBuildPlan_SRERungsResolveWhenTheyOpen(t *testing.T) {
	reported := ist(2026, 10, 8, 13, 25)
	rota := handoverReader{
		stubScheduleReader: &stubScheduleReader{onDuty: []onDutyAssignment{
			held("tz1-l1", "apollo", "SRE_TZ1_L1", ""),
			held("tz1-l2", "apollo", "SRE_TZ1", "L2"),
			// TZ1 has no L3 today.
		}},
		handover: ist(2026, 10, 8, 13, 30),
		after: []onDutyAssignment{
			held("tz2-l1", "apollo", "SRE_TZ2_L1", ""),
			held("tz2-l2", "apollo", "SRE_TZ2", "L2"),
			held("tz2-l3", "apollo", "SRE_TZ2", "L3"),
		},
	}
	tr := Trigger{
		IncidentID: testIncidentID, Priority: "S0", Kind: TriggerNewIncident, At: reported,
		Routing: RoutingContext{AssignedCRETeam: "apollo", Ladder: LadderSRE, At: reported},
	}
	plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, NewTeamScheduleResolver(rota, sreTeams, nil), ChannelLog)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		level Level
		after time.Duration
		email string
	}{
		{Level0, 0, "tz1-l1@example.com"},
		{Level1, 5 * time.Minute, "tz2-l2@example.com"},
		{Level2, 10 * time.Minute, "tz2-l3@example.com"},
	}
	if len(plan.Calls) != len(want) || len(plan.Issues) != 0 {
		t.Fatalf("calls=%+v issues=%+v; want %d calls and no issues", plan.Calls, plan.Issues, len(want))
	}
	for i, w := range want {
		c := plan.Calls[i]
		if c.Level != w.level || c.At.Sub(reported) != w.after || c.Recipient.Email != w.email {
			t.Errorf("call %d = %s +%s %s; want %s +%s %s",
				i, c.Level, c.At.Sub(reported), c.Recipient.Email, w.level, w.after, w.email)
		}
	}
}

// atRecorder notes the instant each level was resolved for.
type atRecorder map[Level]time.Time

func (r atRecorder) Resolve(_ context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	r[level] = rc.At
	return []Recipient{{Name: "x", Email: "x@example.com", Phone: "+10000000000"}}, nil
}

// The CRE ladder keeps resolving every rung at the report instant: its rungs
// are fixed by the shift the incident arrived in.
func TestBuildPlan_CRERungsResolveAtTheReportInstant(t *testing.T) {
	rec := atRecorder{}
	tr := Trigger{
		IncidentID: testIncidentID, Priority: "P1", Kind: TriggerNewIncident, At: testClock,
		Routing: RoutingContext{AssignedCRETeam: "atlas", At: testClock},
	}
	if _, err := BuildPlan(context.Background(), tr, DefaultPolicy, rec, ChannelLog); err != nil {
		t.Fatal(err)
	}
	if len(rec) < 2 {
		t.Fatalf("resolved %d levels; want a multi-rung CRE ladder", len(rec))
	}
	for level, at := range rec {
		if !at.Equal(testClock) {
			t.Errorf("CRE %s resolved at %s; want the report instant %s", level, at, testClock)
		}
	}
}

func TestResolveSRE_PhoneBookFillsNumbers(t *testing.T) {
	r := sreResolver(morningRota()).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	got, err := r.Resolve(context.Background(), Level0, RoutingContext{AssignedCRETeam: "apollo", Ladder: LadderSRE, At: testClock})
	if err != nil || len(got) != 1 || got[0].Phone != "+94770000000" {
		t.Fatalf("got %+v, %v; want the phone book's number", got, err)
	}
}

func TestSRE_RolesAndInstruction(t *testing.T) {
	if got := Level1.RoleIn(LadderSRE); got != "L2 support" {
		t.Errorf("RoleIn = %q", got)
	}
	if got := Level1.RoleIn(LadderCRE); got != Level1.Role() {
		t.Errorf("CRE role changed: %q", got)
	}
	tr := Trigger{Kind: TriggerNewIncident, Routing: RoutingContext{Ladder: LadderSRE}}
	if !strings.Contains(tr.VoiceMessagePlain(), "Assign the incident to yourself") {
		t.Errorf("SRE voice message = %q", tr.VoiceMessagePlain())
	}
	if (RoutingContext{Ladder: LadderSRE}).Rule() != "SRE_TIERS" {
		t.Error("an SRE incident should report its own path")
	}
}

// --- the engines ------------------------------------------------------------

// ladderEngine is a chat-only engine of one kind over the morning rota. Chat
// needs no phone numbers, which is how the ladder is exercised without a
// telephony account.
func ladderEngine(kind Ladder, chat *fakeChat, store *memStore) *Engine {
	cfg := EngineConfig{CallSendingEnabled: true, Channel: ChannelChat, Kind: kind}
	policies := DefaultPolicy
	if kind == LadderSRE {
		policies = withSREPolicy(DefaultPolicy, cfg.Ladder.Timing.Policy())
	}
	return &Engine{
		policies:  policies,
		resolver:  sreResolver(morningRota()),
		notifiers: []notifier{chatNotifier{chat: chat, links: fakeLinks{}}},
		store:     store,
		notes:     &fakeNotes{},
		cfg:       cfg,
		clock:     func() time.Time { return testClock },
	}
}

func created(t *testing.T, team, priority string) eventbus.Record {
	t.Helper()
	return record(t, events.TypeIncidentCreated, events.IncidentCreatedPayload{
		Title: "Latency alert on gateway", ShortDescription: "p95 latency above threshold", Number: "INC0099001",
		Priority: priority, Team: team, ReportedAt: testClock.Format(time.RFC3339),
	})
}

func assigned(t *testing.T) eventbus.Record {
	return record(t, events.TypeIncidentAssigned, events.IncidentAssignedPayload{AssigneeID: "a-l2", AssigneeName: "a-l2"})
}

// The whole SRE flow over chat: L1 at once, L2 at +5m, L3 at +10m, one card
// each, then stopped by an assignee.
func TestEngine_SRELadderStopsWhenAssigned(t *testing.T) {
	ctx := context.Background()
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderSRE, chat, store)

	if err := e.Handle(ctx, created(t, "Apollo", "HIGH")); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(ctx, testIncidentID)
	if !found || st.Plan.Trigger.Routing.Ladder != LadderSRE || len(st.Plan.Calls) != 3 {
		t.Fatalf("found=%v ladder=%q calls=%d; want an SRE ladder of 3", found, st.Plan.Trigger.Routing.Ladder, len(st.Plan.Calls))
	}
	for i, want := range []string{"L1 support", "L2 support"} {
		if err := e.Tick(ctx, testClock.Add(time.Duration(i)*5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if len(chat.posted) != i+1 || chat.posted[i].RungRole != want {
			t.Fatalf("after tick %d: %d cards; want %s", i, len(chat.posted), want)
		}
	}

	// A public comment is not an SRE acknowledgement.
	comment := record(t, events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{CommentID: "c1", IsPublic: true})
	if err := e.Handle(ctx, comment); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, testIncidentID); !found {
		t.Fatal("a public comment stopped the SRE ladder; only an assignee should")
	}

	if err := e.Handle(ctx, assigned(t)); err != nil {
		t.Fatal(err)
	}
	if err := e.Tick(ctx, testClock.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(chat.posted) != 2 {
		t.Fatalf("%d cards; L3 must not be reached once someone is assigned", len(chat.posted))
	}
	if _, found, _ := store.Get(ctx, testIncidentID); found {
		t.Fatal("a stopped ladder's state should be cleared")
	}
}

// Any priority, even one the CRE table has no row for.
func TestEngine_SRELadderRunsAtEveryPriority(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	if err := ladderEngine(LadderSRE, &fakeChat{}, store).Handle(ctx, created(t, "Apollo", "PLANNING")); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, testIncidentID); !found {
		t.Fatal("a PLANNING SRE incident got no ladder; the SRE ladder has no priority gate")
	}
}

// The agreed routing (Case Paging Rules.xlsx): a customer case S0 pages CRE
// and SRE, S1-S4 CRE only; an SRE incident pages SRE only; an incident on a
// CRE team pages nobody, since CRE work arrives as a case.
func TestEngines_RouteCasesAndIncidents(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		rec      func() eventbus.Record
		cre, sre bool
	}{
		{"SRE incident HIGH", func() eventbus.Record { return created(t, "Apollo", "HIGH") }, false, true},
		{"SRE incident CRITICAL", func() eventbus.Record { return created(t, "Apollo", "CRITICAL") }, false, true},
		{"CRE-team incident HIGH", func() eventbus.Record { return created(t, "Atlas", "HIGH") }, false, false},
		{"CRE-team incident CRITICAL", func() eventbus.Record { return created(t, "Atlas", "CRITICAL") }, false, false},
		{"case S0", func() eventbus.Record { return caseCreated(t, "Atlas", "CATASTROPHIC") }, true, true},
		{"case S1", func() eventbus.Record { return caseCreated(t, "Atlas", "CRITICAL") }, true, false},
		{"case S2", func() eventbus.Record { return caseCreated(t, "Atlas", "HIGH") }, true, false},
		{"case S3", func() eventbus.Record { return caseCreated(t, "Atlas", "MEDIUM") }, true, false},
		{"case S4, no priority gate", func() eventbus.Record { return caseCreated(t, "Atlas", "LOW") }, true, false},
		{"service request", func() eventbus.Record { return caseCreatedOfType(t, "SERVICE_REQUEST", "Atlas", "") }, false, false},
	} {
		creStore, sreStore := newMemStore(), newMemStore()
		cre, sre := ladderEngine(LadderCRE, &fakeChat{}, creStore), ladderEngine(LadderSRE, &fakeChat{}, sreStore)
		for _, e := range []*Engine{cre, sre} {
			if err := e.Handle(ctx, tc.rec()); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
		}
		_, gotCRE, _ := creStore.Get(ctx, testIncidentID)
		_, gotSRE, _ := sreStore.Get(ctx, testIncidentID)
		if gotCRE != tc.cre || gotSRE != tc.sre {
			t.Errorf("%s: CRE=%v SRE=%v; want CRE=%v SRE=%v", tc.name, gotCRE, gotSRE, tc.cre, tc.sre)
		}
	}
}

// S4 is paged only when cre.trigger.priorities lists it.
func TestEngine_CaseS4IsOptional(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	cre := ladderEngine(LadderCRE, &fakeChat{}, store)
	cre.cfg.Ladder.Start.Priorities = []string{"S0", "S1", "S2", "S3"}
	if err := cre.Handle(ctx, caseCreated(t, "Atlas", "LOW")); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, testIncidentID); found {
		t.Fatal("an S4 case was paged with S4 left out of cre.trigger.priorities")
	}
	cre.cfg.Ladder.Start.Priorities = append(cre.cfg.Ladder.Start.Priorities, "S4")
	if err := cre.Handle(ctx, caseCreated(t, "Atlas", "LOW")); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, testIncidentID); !found {
		t.Fatal("an S4 case was not paged with S4 listed")
	}
}

// A case S0: any engineer assigned stops SRE; CRE needs the assignee's public
// comment too. A customer's comment stops neither. The summary goes onto the
// case.
func TestEngines_CaseS0StopRules(t *testing.T) {
	ctx := context.Background()
	creStore, sreStore := newMemStore(), newMemStore()
	cre, sre := ladderEngine(LadderCRE, &fakeChat{}, creStore), ladderEngine(LadderSRE, &fakeChat{}, sreStore)
	both := func(r eventbus.Record) {
		t.Helper()
		for _, e := range []*Engine{cre, sre} {
			if err := e.Handle(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
	}
	both(caseCreated(t, "Atlas", "CATASTROPHIC"))
	st, _, _ := sreStore.Get(ctx, testIncidentID)
	if st.Plan.Trigger.Routing.Ladder != LadderSRE || !st.Plan.Trigger.isCase() {
		t.Fatalf("SRE plan: ladder %q, record %q", st.Plan.Trigger.Routing.Ladder, st.Plan.Trigger.Record)
	}

	both(caseComment(t, "customer@acme.com", false, false))
	if _, found, _ := creStore.Get(ctx, testIncidentID); !found {
		t.Fatal("a customer's comment stopped the CRE chain")
	}
	if _, found, _ := sreStore.Get(ctx, testIncidentID); !found {
		t.Fatal("a customer's comment stopped the SRE chain")
	}

	both(caseAssigned(t, "eng@wso2.com"))
	if _, found, _ := sreStore.Get(ctx, testIncidentID); found {
		t.Fatal("the SRE chain kept paging after an engineer was assigned to the S0 case")
	}
	if st, found, _ := creStore.Get(ctx, testIncidentID); !found || st.Cancelled != nil {
		t.Fatal("assignment alone stopped the CRE chain; it needs the assignee's public comment too")
	}

	both(caseComment(t, "other@wso2.com", false, true))
	if _, found, _ := creStore.Get(ctx, testIncidentID); !found {
		t.Fatal("another engineer's comment stopped the CRE chain; only the assignee's counts")
	}
	both(caseComment(t, "eng@wso2.com", true, true))
	if _, found, _ := creStore.Get(ctx, testIncidentID); !found {
		t.Fatal("the assignee's work note stopped the CRE chain; only a public comment counts")
	}
	both(caseComment(t, "ENG@wso2.com", false, true))
	if _, found, _ := creStore.Get(ctx, testIncidentID); found {
		t.Fatal("the CRE chain kept paging after the assignee's public comment")
	}
	if notes := cre.notes.(*fakeNotes); len(notes.caseNotes) != 1 || len(notes.notes) != 0 {
		t.Fatalf("summary: %d on the case, %d on an incident; want 1 on the case", len(notes.caseNotes), len(notes.notes))
	}
}

// A comment written just before its author is assigned still counts.
func TestEngine_CaseCommentBeforeAssignmentCounts(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	cre := ladderEngine(LadderCRE, &fakeChat{}, store)
	for _, r := range []eventbus.Record{
		caseCreated(t, "Atlas", "HIGH"),
		caseComment(t, "eng@wso2.com", false, true),
		caseAssigned(t, "eng@wso2.com"),
	} {
		if err := cre.Handle(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, _ := store.Get(ctx, testIncidentID); found {
		t.Fatal("the chain kept paging: the comment came first, then its author was assigned")
	}
}

// Any severity change stops the running chain and starts a new one for the new
// severity, even after acknowledgement; S0 brings SRE in and losing it takes
// SRE out; S4 (off) pages nobody. A case already assigned when raised to S0
// stops both chains on the assignee's comment.
func TestEngines_CaseSeverityChanges(t *testing.T) {
	ctx := context.Background()
	creStore, sreStore := newMemStore(), newMemStore()
	cre, sre := ladderEngine(LadderCRE, &fakeChat{}, creStore), ladderEngine(LadderSRE, &fakeChat{}, sreStore)
	cre.cfg.Ladder.Start.Priorities = []string{"S0", "S1", "S2", "S3"}
	both := func(r eventbus.Record) {
		t.Helper()
		for _, e := range []*Engine{cre, sre} {
			if err := e.Handle(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
	}
	running := func() (string, bool) {
		st, found, _ := creStore.Get(ctx, testIncidentID)
		return NormalisePriority(st.Plan.Trigger.Priority), found
	}
	sreRunning := func() bool { _, found, _ := sreStore.Get(ctx, testIncidentID); return found }

	both(caseCreated(t, "Atlas", "MEDIUM"))
	both(severityChanged(t, "MEDIUM", "CRITICAL"))
	if p, ok := running(); !ok || p != "P1" || sreRunning() {
		t.Fatalf("S3 -> S1: CRE %v at %s, SRE %v; want a new S1 CRE chain and no SRE", ok, p, sreRunning())
	}

	both(severityChanged(t, "CRITICAL", "CATASTROPHIC"))
	if p, ok := running(); !ok || p != "P0" || !sreRunning() {
		t.Fatalf("S1 -> S0: CRE %v at %s, SRE %v; want both, CRE at S0", ok, p, sreRunning())
	}
	before, _, _ := sreStore.Get(ctx, testIncidentID)
	both(severityChanged(t, "CRITICAL", "CATASTROPHIC")) // redelivered
	after, _, _ := sreStore.Get(ctx, testIncidentID)
	if !after.Plan.Trigger.At.Equal(before.Plan.Trigger.At) {
		t.Fatal("a redelivered severity change restarted the SRE chain")
	}

	both(severityChanged(t, "CATASTROPHIC", "HIGH"))
	if p, ok := running(); !ok || p != "P2" || sreRunning() {
		t.Fatalf("S0 -> S2: CRE %v at %s, SRE %v; want a new S2 CRE chain and SRE stopped", ok, p, sreRunning())
	}

	// Acknowledged: assigned and the assignee's comment.
	both(caseAssigned(t, "eng@wso2.com"))
	both(caseComment(t, "eng@wso2.com", false, true))
	if _, ok := running(); ok {
		t.Fatal("the acknowledged S2 chain kept paging")
	}
	// Lowered after acknowledgement: still a new chain.
	both(severityChanged(t, "HIGH", "MEDIUM"))
	if p, ok := running(); !ok || p != "P3" {
		t.Fatalf("S2 -> S3 after acknowledgement: CRE %v at %s; want a new S3 chain", ok, p)
	}
	// Raised to S0 while assigned: both start; the assignee's comment stops both.
	both(severityChanged(t, "MEDIUM", "CATASTROPHIC"))
	if _, ok := running(); !ok || !sreRunning() {
		t.Fatal("S3 -> S0 on an assigned case did not start both chains")
	}
	both(caseComment(t, "eng@wso2.com", false, true))
	if _, ok := running(); ok || sreRunning() {
		t.Fatalf("the assignee's comment after the raise left CRE %v, SRE %v; want both stopped", ok, sreRunning())
	}

	// Lowered to S4 while S4 is off: nobody is paged.
	both(severityChanged(t, "CATASTROPHIC", "HIGH"))
	both(severityChanged(t, "HIGH", "LOW"))
	if _, ok := running(); ok || sreRunning() {
		t.Fatal("a case lowered to S4 (off) is still paging")
	}
}

// A closed case stops both chains.
func TestEngines_ClosedCaseStopsBoth(t *testing.T) {
	ctx := context.Background()
	creStore, sreStore := newMemStore(), newMemStore()
	cre, sre := ladderEngine(LadderCRE, &fakeChat{}, creStore), ladderEngine(LadderSRE, &fakeChat{}, sreStore)
	for _, r := range []eventbus.Record{
		caseCreated(t, "Atlas", "CATASTROPHIC"),
		caseStatus(t, "Work In Progress"),
	} {
		for _, e := range []*Engine{cre, sre} {
			if err := e.Handle(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, found, _ := creStore.Get(ctx, testIncidentID); !found {
		t.Fatal("a status change that is not a close stopped the CRE chain")
	}
	for _, e := range []*Engine{cre, sre} {
		if err := e.Handle(ctx, caseStatus(t, "Closed")); err != nil {
			t.Fatal(err)
		}
	}
	_, creUp, _ := creStore.Get(ctx, testIncidentID)
	_, sreUp, _ := sreStore.Get(ctx, testIncidentID)
	if creUp || sreUp {
		t.Fatalf("closed case: CRE %v, SRE %v still paging", creUp, sreUp)
	}
}

// An incident elevation never brings SRE in from a CRE team -- CRE work, and
// its S0, arrive as a case -- and never restarts an SRE team's ladder.
func TestEngine_SREElevations(t *testing.T) {
	ctx := context.Background()
	elevated := func(team, to string) eventbus.Record {
		return record(t, events.TypeIncidentPriorityElevated, events.IncidentPriorityElevatedPayload{
			Number: "INC0099001", Title: "Latency alert", OldPriority: "HIGH", NewPriority: to,
			Team: team, ElevatedAt: testClock.Format(time.RFC3339),
		})
	}
	for _, tc := range []struct{ team, to, why string }{
		{"Atlas", "P0", "a CRE-team incident elevated to P0 started the SRE ladder"},
		{"Apollo", "P1", "an elevation started an SRE team's ladder; its clock does not depend on priority"},
	} {
		store := newMemStore()
		if err := ladderEngine(LadderSRE, &fakeChat{}, store).Handle(ctx, elevated(tc.team, tc.to)); err != nil {
			t.Fatal(err)
		}
		if _, found, _ := store.Get(ctx, testIncidentID); found {
			t.Fatal(tc.why)
		}
	}
}

// --- case events ---------------------------------------------------------------

func caseCreated(t *testing.T, team, severity string) eventbus.Record {
	return caseCreatedOfType(t, "CASE", team, severity)
}

func caseCreatedOfType(t *testing.T, caseType, team, severity string) eventbus.Record {
	return record(t, events.TypeCaseCreated, events.CaseCreatedPayload{
		ReporterName: "Jane Customer", ProjectName: "Acme", ProjectID: "p-1", CaseID: testIncidentID,
		CaseNumber: "CS0099001", CaseTitle: "Gateway down", CaseType: caseType, Priority: severity,
		Team: team, CreatedAt: testClock.Format(time.RFC3339), Description: "everything fails",
		Recipients: []string{"watcher@wso2.com"},
	})
}

func severityChanged(t *testing.T, from, to string) eventbus.Record {
	return record(t, events.TypeSeverityChanged, events.SeverityChangedPayload{
		ProjectID: "p-1", CaseID: testIncidentID, CaseNumber: "CS0099001", CaseTitle: "Gateway down",
		OldSeverity: from, NewSeverity: to, Team: "Atlas", Recipients: []string{"watcher@wso2.com"},
	})
}

func caseAssigned(t *testing.T, email string) eventbus.Record {
	return record(t, events.TypeCaseAssigned, events.CaseAssignedPayload{
		AssigneeName: "An Engineer", AssigneeEmail: email, ProjectID: "p-1", CaseID: testIncidentID,
		Recipients: []string{"watcher@wso2.com"},
	})
}

func caseComment(t *testing.T, author string, workNote, engineer bool) eventbus.Record {
	return record(t, events.TypeCommentAdded, events.CommentAddedPayload{
		Name: "Someone", ProjectID: "p-1", CaseID: testIncidentID, CaseTitle: "Gateway down",
		CaseComment: "looking", CommentID: "c-1", IsInternalNote: workNote, AuthorEmail: author,
		IsSupportEngineerResponse: engineer, Recipients: []string{"watcher@wso2.com"},
	})
}

func caseStatus(t *testing.T, status string) eventbus.Record {
	return record(t, events.TypeStatusChanged, events.StatusChangedPayload{
		ProjectID: "p-1", CaseID: testIncidentID, NewStatus: status, Recipients: []string{"watcher@wso2.com"},
	})
}

// --- configuration -----------------------------------------------------------

func loadYAML(t *testing.T, body string) (Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "escalation.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(path)
}

func TestConfig_SRESection(t *testing.T) {
	cfg, err := loadYAML(t, `
enabled: true
sre:
  enabled: true
  channel: chat
  timing:
    interval: 2m
    includeL4: true
  teams:
    abtType: sre-abt
    abts: [apollo, artemis]
    aliases: {"SRE - Apollo": apollo}
`)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.SRE.Timing.Policy()
	if p.Levels[Level1].NotificationInterval != 2*time.Minute || p.Levels[Level3].NotificationCount != 1 {
		t.Fatalf("timing = %+v; want 2m rungs with L4", p)
	}
}

// --- routing: which ladders an incident climbs is configuration -------------

func TestRouting_DefaultRules(t *testing.T) {
	var r Routing // zero value: DefaultRouting
	for _, tc := range []struct {
		in       RouteInput
		cre, sre bool
	}{
		{RouteInput{Record: "incident", Team: "sre", Priority: "HIGH"}, false, true},                           // sheet "Yes" rows
		{RouteInput{Record: "incident", Team: "cre", Priority: "HIGH"}, false, false},                          // CRE work is a case
		{RouteInput{Record: "incident", Team: "cre", Priority: "CRITICAL"}, false, false},                      // not S0
		{RouteInput{Record: "incident", Team: "none", ContactType: "AZURE", Priority: "LOW"}, false, true},     // sheet "No" rows
		{RouteInput{Record: "incident", Team: "sre", ContactType: "SENTINEL"}, false, true},                    // monitoring, SRE team
		{RouteInput{Record: "incident", Team: "cre", ContactType: "SITE_24_7", Priority: "LOW"}, false, false}, // monitoring, CRE team
		{RouteInput{Record: "incident", Team: "none", ContactType: "EMAIL"}, false, false},                     // a person raised it
		{RouteInput{Record: "case", Team: "cre", Priority: "CATASTROPHIC"}, true, true},                        // S0: both
		{RouteInput{Record: "case", Team: "cre", Priority: "S0"}, true, true},                                  // label = code
		{RouteInput{Record: "case", Team: "none", Priority: "P0"}, true, true},                                 // S0 on no ABT
		{RouteInput{Record: "case", Team: "cre", Priority: "CRITICAL"}, true, false},                           // S1: CRE only
		{RouteInput{Record: "case", Team: "cre", Priority: "LOW"}, true, false},                                // S4: CRE (gate decides)
	} {
		_, cre := r.Match(tc.in, LadderKeyCRE)
		_, sre := r.Match(tc.in, LadderKeySRE)
		if cre != tc.cre || sre != tc.sre {
			t.Errorf("%+v: cre=%v sre=%v; want cre=%v sre=%v", tc.in, cre, sre, tc.cre, tc.sre)
		}
	}
}

// Routing is code, not configuration: a file carrying a routing section is
// refused rather than allowed to change who gets paged.
func TestConfig_RoutingSectionIsRefused(t *testing.T) {
	if _, err := loadYAML(t, "routing:\n  rules:\n    - name: x\n      ladders: [sre]\n"); err == nil {
		t.Fatal("a routing section loaded; routing is DefaultRouting, in code")
	}
}

// The engines follow the routing rules: a monitoring-raised incident with no
// team climbs the SRE ladder, past sre.trigger.requireKnownTeam.
func TestEngines_MonitoringRaisedIncidentClimbsSRE(t *testing.T) {
	ctx := context.Background()
	mk := func(team, priority, contact string) eventbus.Record {
		return record(t, events.TypeIncidentCreated, events.IncidentCreatedPayload{
			Title: "CPU alert", ShortDescription: "from monitoring", Number: "INC0099002",
			Priority: priority, Team: team, ContactType: contact, ReportedAt: testClock.Format(time.RFC3339),
		})
	}
	for _, tc := range []struct {
		name, team, priority, contact string
		cre, sre                      bool
	}{
		{"no team, AZURE", "", "LOW", "AZURE", false, true},
		// A CRE team's work arrives as a case; its monitoring incident pages
		// nobody.
		{"CRE team, SITE_247", "Atlas", "LOW", "SITE_247", false, false},
		{"SRE team, SENTINEL", "Apollo", "HIGH", "SENTINEL", false, true},
		{"no team, EMAIL", "", "LOW", "EMAIL", false, false},
	} {
		creStore, sreStore := newMemStore(), newMemStore()
		cre, sre := ladderEngine(LadderCRE, &fakeChat{}, creStore), ladderEngine(LadderSRE, &fakeChat{}, sreStore)
		// requireKnownTeam on both, as the local file sets it.
		cre.cfg.Ladder.Start.RequireKnownTeam = true
		sre.cfg.Ladder.Start.RequireKnownTeam = true
		for _, e := range []*Engine{cre, sre} {
			if err := e.Handle(ctx, mk(tc.team, tc.priority, tc.contact)); err != nil {
				t.Fatal(err)
			}
		}
		_, gotCRE, _ := creStore.Get(ctx, testIncidentID)
		st, gotSRE, _ := sreStore.Get(ctx, testIncidentID)
		if gotCRE != tc.cre || gotSRE != tc.sre {
			t.Errorf("%s: CRE=%v SRE=%v; want CRE=%v SRE=%v", tc.name, gotCRE, gotSRE, tc.cre, tc.sre)
			continue
		}
		if gotSRE && tc.team == "" && st.Plan.Calls[0].Recipient.Email != "a-l1@example.com" {
			t.Errorf("%s: L1 = %s; want the zone's L1, Apollo first", tc.name, st.Plan.Calls[0].Recipient.Email)
		}
	}
}

func TestConfig_RefusesMisplacedAndOverlappingSettings(t *testing.T) {
	for name, body := range map[string]string{
		"SRE timing under cre": "cre:\n  timing:\n    interval: 5m\n",
		"CRE rules under sre":  "sre:\n  rules:\n    - id: R1\n      shift: LK\n      assignedToABT: any\n      levels: [rota_members]\n",
		"bad duration":         "sre:\n  timing:\n    interval: five\n",
		"team in both lists":   "cre:\n  teams:\n    abts: [apollo]\nsre:\n  teams:\n    abts: [apollo]\n",
		"old crePriorities":    "sre:\n  trigger:\n    crePriorities: [P0]\n",
	} {
		if _, err := loadYAML(t, body); err == nil {
			t.Errorf("%s: loaded; want an error", name)
		}
	}
}

// The shipped local configuration must load: an invalid one disables both
// ladders on the stack everybody tests against.
func TestConfig_LocalComposeFileLoads(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("..", "..", "..", "..", "scripts", "csm-compose", "escalation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SRE.Teams.ABTs) != 2 || cfg.SRE.Timing.Policy().Levels[Level1].NotificationInterval != 5*time.Minute {
		t.Fatalf("sre section = %+v", cfg.SRE)
	}
}

// The log channel names each rung the way its own ladder does: an SRE rung is
// "L2 support", not the CRE ladder's name for LEVEL_1.
func TestLogNotifier_NamesTheRungByItsLadder(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	plan := Plan{Trigger: Trigger{Routing: RoutingContext{Ladder: LadderSRE}}}
	if _, err := (logNotifier{}).Deliver(context.Background(), plan, PlannedCall{Level: Level1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `role="L2 support"`) {
		t.Fatalf("log line = %s; want role=\"L2 support\"", buf.String())
	}
}

// A severity change refused only by a team or shift setting keeps the running
// chain: those settings decide whether a chain may start, not whether one
// should stop. A severity the ladder does not page still stops it.
func TestEngine_SeverityChangeRefusedByShiftKeepsTheChain(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	cre := ladderEngine(LadderCRE, &fakeChat{}, store)
	cre.cfg.Ladder.Start.Priorities = []string{"S0", "S1", "S2", "S3"}
	if err := cre.Handle(ctx, caseCreated(t, "Atlas", "MEDIUM")); err != nil {
		t.Fatal(err)
	}
	// testClock is the LK shift; from now on only the Americas night may start
	// a chain.
	cre.cfg.Ladder.Start.Shifts = []string{"USA"}
	if err := cre.Handle(ctx, severityChanged(t, "MEDIUM", "CRITICAL")); err != nil {
		t.Fatal(err)
	}
	st, found, _ := store.Get(ctx, testIncidentID)
	if !found || NormalisePriority(st.Plan.Trigger.Priority) != "P3" {
		t.Fatalf("S3 -> S1 refused by the shift setting: chain found=%v at %s; want the S3 chain still running", found, st.Plan.Trigger.Priority)
	}
	if err := cre.Handle(ctx, severityChanged(t, "CRITICAL", "LOW")); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get(ctx, testIncidentID); found {
		t.Fatal("lowered to S4 (not paged) and the chain is still running")
	}
}
