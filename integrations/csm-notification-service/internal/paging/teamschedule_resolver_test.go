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
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// stubScheduleReader answers like the real endpoint: it filters the members it
// holds by the same three criteria the query does, so a test that asks for the
// wrong thing gets the wrong answer rather than everything.
type stubScheduleReader struct {
	members    []teamMember
	onDuty     []onDutyAssignment
	membersErr error
	onDutyErr  error

	gotTeamKeys []string
	gotRoles    []string
	gotTiers    []string
	gotTypes    []string
	gotAt       time.Time
	memberCalls int
	onDutyCalls int
}

func (s *stubScheduleReader) TeamMembers(_ context.Context, teamKeys, roles, tiers, types []string) ([]teamMember, error) {
	s.gotTeamKeys, s.gotRoles, s.gotTiers, s.gotTypes = teamKeys, roles, tiers, types
	s.memberCalls++
	if s.membersErr != nil {
		return nil, s.membersErr
	}
	var out []teamMember
	for _, m := range s.members {
		if len(teamKeys) > 0 && !inList(teamKeys, m.TeamKey) {
			continue
		}
		if len(types) > 0 && !inList(types, m.TeamType) {
			continue
		}
		if len(teamKeys) == 0 && len(types) == 0 {
			continue
		}
		if len(roles) > 0 && !inList(roles, m.Role) {
			continue
		}
		if len(tiers) > 0 && !inList(tiers, m.AlertTier) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *stubScheduleReader) OnDutyAt(_ context.Context, at time.Time) ([]onDutyAssignment, error) {
	s.gotAt = at
	s.onDutyCalls++
	return s.onDuty, s.onDutyErr
}

func inList(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func onDutyFor(userID, email, teamKey string) onDutyAssignment {
	var a onDutyAssignment
	a.Engineer.UserID, a.Engineer.Email, a.Engineer.Name = userID, email, email
	a.TeamKey = teamKey
	return a
}

func member(team, email, role, tier string) teamMember {
	// Every test team belongs to the CRE ABT unless a test says otherwise;
	// Americas is type cre, which is what makes it not an ABT.
	typ := "cre-abt"
	if team == "americas" {
		typ = "cre"
	}
	if team == "cre-leadership" {
		typ = "cre-leadership"
	}
	return teamMember{TeamKey: team, TeamType: typ, Email: email, Name: email,
		UserID: email, Role: role, AlertTier: tier}
}

// The seven ABTs plus Americas, as deployed.
var testTeams = TeamKeys{
	ABTs:       []string{"apollo", "artemis", "atlas", "castor", "draco", "phoenix", "vega"},
	Americas:   "americas",
	Leadership: "cre-leadership",
}

func testResolver(stub *stubScheduleReader) TeamScheduleResolver {
	return NewTeamScheduleResolver(stub, testTeams, nil)
}

func emails(rs []Recipient) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Email)
	}
	return out
}

// Every row of the sheet must be reachable, and reachable by exactly the
// (shift, is-assigned-to-an-ABT) pair it names. A row nothing routes to is a
// rule that silently does not exist.
func TestRuleFor_EveryRowIsReachable(t *testing.T) {
	cases := []struct {
		shift  Shift
		team   string
		wantID string
	}{
		{ShiftLKMorning, "vega", "R1a"},
		{ShiftLKMorning, "", "R1a"},
		{ShiftLKWeekend, "vega", "R1b"},
		{ShiftLK, "vega", "R2"},
		{ShiftLK, "not-an-abt", "R3"},
		{ShiftLKEvening, "vega", "R4a"},
		{ShiftLKEvening, "not-an-abt", "R4b"},
		{ShiftUSA, "vega", "R5"},
		{ShiftUSAWeekend, "vega", "R6"},
	}
	r := testResolver(&stubScheduleReader{})
	for _, tc := range cases {
		t.Run(tc.wantID+"/"+string(tc.shift), func(t *testing.T) {
			got, ok := r.RuleFor(RoutingContext{Shift: tc.shift, AssignedCRETeam: tc.team})
			if !ok {
				t.Fatalf("no rule matched shift %s, team %q", tc.shift, tc.team)
			}
			if got.ID != tc.wantID {
				t.Errorf("rule = %s, want %s", got.ID, tc.wantID)
			}
		})
	}
}

// The whole point of the new table: LEVEL_1 is the one lead of the incident's
// own team, LEVEL_2 is every ABT's lead. The previous model had these the
// other way round, so this is the assertion that pins the inversion.
func TestResolve_LeadRungsAreInverted(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("vega", "vega.lead@example.com", roleLead, ""),
		member("atlas", "atlas.lead@example.com", roleLead, ""),
		member("apollo", "apollo.lead@example.com", roleLead, ""),
		member("vega", "vega.sublead@example.com", roleSubLead, ""),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	one, err := r.Resolve(context.Background(), Level1, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(one); len(got) != 1 || got[0] != "vega.lead@example.com" {
		t.Errorf("LEVEL_1 = %v, want only the incident's own team lead", got)
	}

	all, err := r.Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(all); len(got) != 3 {
		t.Errorf("LEVEL_2 = %v, want every ABT lead", got)
	}
	// And a sub lead is nobody's rung any more.
	for _, e := range append(emails(one), emails(all)...) {
		if strings.Contains(e, "sublead") {
			t.Errorf("a sub lead was called (%s); the updated rules have no sub-lead rung", e)
		}
	}
}

func TestResolve_Level0PerRule(t *testing.T) {
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

	t.Run("R2 calls the incident's own ABT nominees", func(t *testing.T) {
		stub := &stubScheduleReader{members: []teamMember{
			member("vega", "v1@example.com", "engineer", "T1"),
			member("vega", "v2@example.com", "engineer", "T2"),
			member("vega", "v3@example.com", "engineer", "T3"),
			member("vega", "v9@example.com", "engineer", ""), // not nominated
			member("atlas", "a1@example.com", "engineer", "T1"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"v1@example.com", "v2@example.com", "v3@example.com"}
		if !equalStrings(emails(got), want) {
			t.Errorf("LEVEL_0 = %v, want %v", emails(got), want)
		}
	})

	t.Run("R3 calls one nominee from every ABT", func(t *testing.T) {
		var members []teamMember
		for _, team := range testTeams.ABTs {
			members = append(members,
				member(team, team+".t1@example.com", "engineer", "T1"),
				member(team, team+".t2@example.com", "engineer", "T2"))
		}
		stub := &stubScheduleReader{members: members}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLK, AssignedCRETeam: "not-an-abt", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(testTeams.ABTs) {
			t.Fatalf("LEVEL_0 reached %d people, want one per ABT (%d)", len(got), len(testTeams.ABTs))
		}
		// Lowest tier, so T1 rather than T2, and one per team not two.
		for _, e := range emails(got) {
			if !strings.Contains(e, ".t1@") {
				t.Errorf("%s was called; the lowest tier should answer first", e)
			}
		}
	})

	t.Run("R4a pairs the incident's own rota member with one other", func(t *testing.T) {
		stub := &stubScheduleReader{onDuty: []onDutyAssignment{
			onDutyFor("u1", "atlas.on@example.com", "atlas"),
			onDutyFor("u2", "vega.on@example.com", "vega"),
			onDutyFor("u3", "draco.on@example.com", "draco"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("LEVEL_0 = %v, want exactly two", emails(got))
		}
		if got[0].Email != "vega.on@example.com" {
			t.Errorf("first call = %s, want the incident's own team's rota member", got[0].Email)
		}
	})

	t.Run("R4b calls the whole evening rota when no ABT owns it", func(t *testing.T) {
		stub := &stubScheduleReader{onDuty: []onDutyAssignment{
			onDutyFor("u1", "a@example.com", "atlas"),
			onDutyFor("u2", "b@example.com", "vega"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "not-an-abt", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("LEVEL_0 = %v, want the whole rota", emails(got))
		}
	})

	t.Run("R5 calls the Americas nominees", func(t *testing.T) {
		stub := &stubScheduleReader{members: []teamMember{
			member("americas", "am1@example.com", "engineer", "T1"),
			member("americas", "am2@example.com", "engineer", "T2"),
			member("vega", "v1@example.com", "engineer", "T1"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(emails(got), []string{"am1@example.com", "am2@example.com"}) {
			t.Errorf("LEVEL_0 = %v, want only the Americas nominees", emails(got))
		}
	})

	// R6's rota member is the rostered Americas weekend member, never the
	// on-call shift beside it -- even when the on-call engineer sorts first.
	americasNominees := []teamMember{
		member("americas", "am1@example.com", "engineer", "T1"),
		member("americas", "am2@example.com", "engineer", "T2"),
		member("americas", "am3@example.com", "engineer", "T3"),
	}
	onShift := func(userID, email, team, code string) onDutyAssignment {
		a := onDutyFor(userID, email, team)
		a.ShiftCode = code
		return a
	}

	t.Run("R6 calls the Americas weekend rota member and the nominees", func(t *testing.T) {
		stub := &stubScheduleReader{members: americasNominees, onDuty: []onDutyAssignment{
			onShift("u1", "aaa.oncall@example.com", "draco", "CRE_WEEKEND_NIGHT_OC"),
			onShift("u2", "zzz.weekend@example.com", "americas", "CRE_WEEKEND_NIGHT"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftUSAWeekend, AssignedCRETeam: "sirius", At: at})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"zzz.weekend@example.com", "am1@example.com", "am2@example.com", "am3@example.com"}
		if !equalStrings(emails(got), want) {
			t.Errorf("LEVEL_0 = %v, want %v", emails(got), want)
		}
	})

	t.Run("R6 with nobody on the weekend rota calls the nominees, not the on-call", func(t *testing.T) {
		stub := &stubScheduleReader{members: americasNominees, onDuty: []onDutyAssignment{
			onShift("u1", "aaa.oncall@example.com", "draco", "CRE_WEEKEND_NIGHT_OC"),
		}}
		got, err := testResolver(stub).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftUSAWeekend, AssignedCRETeam: "sirius", At: at})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"am1@example.com", "am2@example.com", "am3@example.com"}
		if !equalStrings(emails(got), want) {
			t.Errorf("LEVEL_0 = %v, want %v", emails(got), want)
		}
	})

	t.Run("R6 rota shifts are configurable", func(t *testing.T) {
		stub := &stubScheduleReader{members: americasNominees[:1], onDuty: []onDutyAssignment{
			onShift("u1", "oncall@example.com", "draco", "CRE_WEEKEND_NIGHT_OC"),
			onShift("u2", "weekend@example.com", "americas", "CRE_WEEKEND_NIGHT"),
		}}
		teams := testTeams
		teams.AmericasWeekendRotaShifts = []string{"cre_weekend_night_oc"}
		got, err := NewTeamScheduleResolver(stub, teams, nil).Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftUSAWeekend, AssignedCRETeam: "sirius", At: at})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"oncall@example.com", "am1@example.com"}
		if !equalStrings(emails(got), want) {
			t.Errorf("LEVEL_0 = %v, want %v", emails(got), want)
		}
	})
}

// The evening pairing has to pick the same second person every time, or a
// retry reaches somebody the first attempt did not and the rung is untestable.
func TestResolve_RotaPairIsDeterministic(t *testing.T) {
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u3", "c@example.com", "draco"),
		onDutyFor("u1", "a@example.com", "atlas"),
		onDutyFor("u2", "b@example.com", "vega"),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: time.Now()}

	first, err := r.Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := r.Resolve(context.Background(), Level0, rc)
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(emails(first), emails(again)) {
			t.Fatalf("pass %d gave %v, first gave %v", i, emails(again), emails(first))
		}
	}
}

// The heads sit outside every ABT, in their own team.
func TestResolve_HeadsComeFromTheLeadershipTeam(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("cre-leadership", "cre.head@example.com", roleCREHead, ""),
		member("cre-leadership", "cs.head@example.com", roleCSHead, ""),
		member("vega", "vega.lead@example.com", roleLead, ""),
	}}
	r := testResolver(stub)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	for _, tc := range []struct {
		level Level
		want  string
	}{{Level3, "cre.head@example.com"}, {Level4, "cs.head@example.com"}} {
		got, err := r.Resolve(context.Background(), tc.level, rc)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Email != tc.want {
			t.Errorf("%s = %v, want %s", tc.level, emails(got), tc.want)
		}
	}
}

// A failure to ask is an error; a rung with nobody on it is not.
func TestResolve_ErrorsOnlyWhenItCannotAsk(t *testing.T) {
	boom := errors.New("entity-service is down")
	stub := &stubScheduleReader{membersErr: boom}
	_, err := testResolver(stub).Resolve(context.Background(), Level1,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega"})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the underlying failure", err)
	}

	empty := &stubScheduleReader{}
	got, err := testResolver(empty).Resolve(context.Background(), Level1,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega"})
	if err != nil {
		t.Errorf("an unstaffed rung must not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("recipients = %v, want none", emails(got))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stubHistory is a call log the pairing rule can read.
type stubHistory struct {
	seen map[string]time.Time
	err  error
}

func (s stubHistory) LastCalled(_ context.Context, emails []string) (map[string]time.Time, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]time.Time{}
	for _, e := range emails {
		if at, ok := s.seen[e]; ok {
			out[e] = at
		}
	}
	return out, nil
}

// The evening pairing's second call rotates: whoever has gone longest without
// one goes next. Without this the same person takes every out-of-hours
// incident, which is the whole reason the rule is not "the first name".
func TestResolve_RotaPairRotatesBySinceLastCalled(t *testing.T) {
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "recent@example.com", "atlas"),
		onDutyFor("u3", "stale@example.com", "draco"),
	}}
	history := stubHistory{seen: map[string]time.Time{
		"recent@example.com": now.Add(-1 * time.Hour),
		"stale@example.com":  now.Add(-72 * time.Hour),
	}}

	r := testResolver(stub).WithCallHistory(history)
	got, err := r.Resolve(context.Background(), Level0,
		RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: now})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"own@example.com", "stale@example.com"}
	if !equalStrings(emails(got), want) {
		t.Errorf("LEVEL_0 = %v, want %v (the longest wait goes next)", emails(got), want)
	}
}

// Somebody never called has waited longest of all -- that is what brings a new
// person into the rotation rather than leaving them permanently unpicked.
func TestResolve_RotaPairPrefersSomebodyNeverCalled(t *testing.T) {
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "called@example.com", "atlas"),
		onDutyFor("u3", "never@example.com", "draco"),
	}}
	history := stubHistory{seen: map[string]time.Time{
		"called@example.com": now.Add(-99 * time.Hour),
	}}

	got, err := testResolver(stub).WithCallHistory(history).
		Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Email != "never@example.com" {
		t.Errorf("second call = %v, want never@example.com", emails(got))
	}
}

// A history read that fails must not fail the rung: fairness is worth less
// than the page going out.
func TestResolve_RotaPairSurvivesAHistoryFailure(t *testing.T) {
	stub := &stubScheduleReader{onDuty: []onDutyAssignment{
		onDutyFor("u1", "own@example.com", "vega"),
		onDutyFor("u2", "other@example.com", "atlas"),
	}}
	got, err := testResolver(stub).WithCallHistory(stubHistory{err: errors.New("redis down")}).
		Resolve(context.Background(), Level0,
			RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: time.Now()})
	if err != nil {
		t.Fatalf("a history failure must not fail the rung: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("recipients = %v, want the pair anyway", emails(got))
	}
}

// Every rule in the updated table has a first rung, so every ladder must
// schedule one -- including the two LK rows, whose LEVEL_0 is the ABT's
// alert-duty nominees.
//
// This is a regression test with a real cause: the plan asked
// HasNotificationLevel ("rotation shifts only"), which was the PREVIOUS
// table's rule, so an incident during business hours silently skipped its
// fastest rung. Found by running a ladder, not by a test, which is why there
// is one now.
func TestBuildPlan_EveryRuleSchedulesLevel0(t *testing.T) {
	var members []teamMember
	for _, team := range append(append([]string{}, testTeams.ABTs...), testTeams.Americas) {
		members = append(members,
			member(team, team+".t1@example.com", "engineer", "T1"),
			member(team, team+".lead@example.com", roleLead, ""))
	}
	members = append(members,
		member("cre-leadership", "cre.head@example.com", roleCREHead, ""),
		member("cre-leadership", "cs.head@example.com", roleCSHead, ""))

	onDuty := []onDutyAssignment{
		onDutyFor("u1", "vega.on@example.com", "vega"),
		onDutyFor("u2", "atlas.on@example.com", "atlas"),
	}

	for _, rule := range DefaultRules {
		t.Run(rule.ID, func(t *testing.T) {
			if rule.Levels[Level0] == SourceNone {
				t.Skipf("%s has no LEVEL_0 source", rule.ID)
			}
			team := "vega"
			if rule.ABT == ABTNo {
				team = "not-an-abt"
			}
			r := testResolver(&stubScheduleReader{members: members, onDuty: onDuty})
			trigger := Trigger{
				IncidentID: "inc-1", Priority: "P1", At: time.Now(),
				Kind: TriggerNewIncident,
				Routing: RoutingContext{
					Shift: rule.Shift, AssignedCRETeam: team, At: time.Now(),
				},
			}
			plan, err := BuildPlan(context.Background(), trigger, DefaultPolicy, r, ChannelLog)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}
			if plan.Trigger.Routing.RuleID != rule.ID {
				t.Fatalf("routed by %s, wanted %s", plan.Trigger.Routing.RuleID, rule.ID)
			}
			var sawLevel0 bool
			for _, c := range plan.Calls {
				if c.Level == Level0 {
					sawLevel0 = true
					break
				}
			}
			if !sawLevel0 {
				t.Errorf("%s (%s) scheduled no LEVEL_0; its source is %s",
					rule.ID, rule.Shift, rule.Levels[Level0])
			}
		})
	}
}

// The rung label a card shows must come from the same table the routing does.
func TestRule_RoleAtNamesTheRealRung(t *testing.T) {
	r, ok := MatchRule(DefaultRules, ShiftUSA, true, true)
	if !ok {
		t.Fatal("no rule for the Americas shift")
	}
	if got, want := r.RoleAt(Level2), "Americas team lead"; got != want {
		t.Errorf("R5 LEVEL_2 = %q, want %q", got, want)
	}
	lk, _ := MatchRule(DefaultRules, ShiftLK, true, true)
	if got, want := lk.RoleAt(Level1), "Team lead"; got != want {
		t.Errorf("R2 LEVEL_1 = %q, want %q", got, want)
	}
	if got, want := lk.RoleAt(Level2), "Team leads"; got != want {
		t.Errorf("R2 LEVEL_2 = %q, want %q", got, want)
	}
}

// "Team leads" spans every ABT by default, and exactly the named teams when
// configuration names some -- which is how the resolver is made to agree with
// the spreadsheet's count of three without a release.
func TestResolve_TeamLeadsSpanIsConfigurable(t *testing.T) {
	var members []teamMember
	for _, team := range testTeams.ABTs {
		members = append(members, member(team, team+".lead@example.com", roleLead, ""))
	}
	stub := &stubScheduleReader{members: members}
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	all, err := testResolver(stub).Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(testTeams.ABTs) {
		t.Errorf("default LEVEL_2 reached %d, want every ABT lead (%d)", len(all), len(testTeams.ABTs))
	}

	narrowed := testTeams
	narrowed.TeamLeads = []string{"vega", "atlas", "apollo"}
	three, err := NewTeamScheduleResolver(stub, narrowed, nil).
		Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apollo.lead@example.com", "atlas.lead@example.com", "vega.lead@example.com"}
	if !equalStrings(emails(three), want) {
		t.Errorf("configured LEVEL_2 = %v, want %v", emails(three), want)
	}
}

// The night shift's LEVEL_1 has a second reading, selectable per rule: the
// Americas team's own leads rather than every ABT's.
func TestResolve_AmericasTeamLeadsIsSelectablePerRule(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("americas", "am.lead1@example.com", roleLead, ""),
		member("americas", "am.lead2@example.com", roleLead, ""),
		member("vega", "vega.lead@example.com", roleLead, ""),
	}}
	rules := []Rule{{
		ID: "R5", Shift: ShiftUSA, ABT: ABTAny,
		Levels: [5]LevelSource{SourceAlertDutyAmericas, SourceAmericasTeamLeads,
			SourceAmericasTeamLead, SourceCREHead, SourceCSHead},
	}}
	got, err := NewTeamScheduleResolver(stub, testTeams, rules).
		Resolve(context.Background(), Level1,
			RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"am.lead1@example.com", "am.lead2@example.com"}
	if !equalStrings(emails(got), want) {
		t.Errorf("LEVEL_1 = %v, want the Americas team's own leads %v", emails(got), want)
	}
}

// At night LEVEL_1 is the Americas team's three Team leads (role lead) and
// LEVEL_2 the one America Team lead above them (role americas_team_lead,
// migration 0185). Before that role existed all four were 'lead', LEVEL_1
// took all four, and LEVEL_2 had to guess.
func TestResolve_NightLevel1NeverIncludesTheAmericaLead(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("americas", "am.tl1@example.com", roleLead, ""),
		member("americas", "am.tl2@example.com", roleLead, ""),
		member("americas", "am.tl3@example.com", roleLead, ""),
		member("americas", "am.atl@example.com", roleAmericasTeamLead, ""),
	}}
	rules := []Rule{{
		ID: "R5", Shift: ShiftUSA, ABT: ABTAny,
		Levels: [5]LevelSource{SourceAlertDutyAmericas, SourceAmericasTeamLeads,
			SourceAmericasTeamLead, SourceCREHead, SourceCSHead},
	}}
	rc := RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: time.Now()}
	r := NewTeamScheduleResolver(stub, testTeams, rules)

	l1, err := r.Resolve(context.Background(), Level1, rc)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := r.Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"am.tl1@example.com", "am.tl2@example.com", "am.tl3@example.com"}; !equalStrings(emails(l1), want) {
		t.Errorf("LEVEL_1 = %v, want the three Team leads %v", emails(l1), want)
	}
	if want := []string{"am.atl@example.com"}; !equalStrings(emails(l2), want) {
		t.Errorf("LEVEL_2 = %v, want the America Team lead %v", emails(l2), want)
	}
}

// Nobody holding americas_team_lead is nobody to call at LEVEL_2 -- never one
// of the three Team leads picked by address, which would ring a LEVEL_1
// person twice and look like an escalation.
func TestResolve_NoAmericaTeamLeadIsNobodyNotAGuess(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("americas", "am.tl1@example.com", roleLead, ""),
		member("americas", "am.tl2@example.com", roleLead, ""),
	}}
	rules := []Rule{{
		ID: "R5", Shift: ShiftUSA, ABT: ABTAny,
		Levels: [5]LevelSource{SourceAlertDutyAmericas, SourceAmericasTeamLeads,
			SourceAmericasTeamLead, SourceCREHead, SourceCSHead},
	}}
	l2, err := NewTeamScheduleResolver(stub, testTeams, rules).Resolve(context.Background(), Level2,
		RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(l2) != 0 {
		t.Errorf("LEVEL_2 = %v, want nobody when no America Team lead is set", emails(l2))
	}
}

// The heads are two named people, not a team lookup -- so a deployment with no
// leadership team still reaches them, and entity-service is never asked.
func TestResolve_HeadsComeFromConfigurationWhenNamed(t *testing.T) {
	stub := &stubScheduleReader{}
	r := testResolver(stub).WithHeads(Heads{
		CRE: Person{Name: "CRE Head", Email: "cre.head@example.com", Phone: "+94770000001"},
		CS:  Person{Name: "CS Head", Email: "cs.head@example.com"},
	})
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	for _, tc := range []struct {
		level Level
		want  string
	}{
		{Level3, "cre.head@example.com"}, {Level4, "cs.head@example.com"},
	} {
		got, err := r.Resolve(context.Background(), tc.level, rc)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Email != tc.want {
			t.Errorf("%s = %v, want %s", tc.level, emails(got), tc.want)
		}
	}
	if stub.memberCalls != 0 {
		t.Errorf("entity-service was asked %d times; named heads need no lookup", stub.memberCalls)
	}
}

// perTeam is the other half of the 3-vs-7 question: all of a team's nominees,
// or one from each team.
func TestResolve_AlertDutyPerTeamIsConfigurable(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("vega", "v1@example.com", "engineer", "T1"),
		member("vega", "v2@example.com", "engineer", "T2"),
		member("vega", "v3@example.com", "engineer", "T3"),
	}}
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	all, err := testResolver(stub).Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("default LEVEL_0 = %v, want all three nominees", emails(all))
	}

	one, err := testResolver(stub).WithAlertDuty(nil, 1).
		Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].Email != "v1@example.com" {
		t.Errorf("perTeam=1 LEVEL_0 = %v, want the lowest tier only", emails(one))
	}
}

// The ABT is a property of the team, not a list in a config file.
//
// team.type is where the grouping lives -- cre-abt holds seven teams, sre-abt
// two, and Americas is type cre and belongs to no ABT. Resolving from it means
// a team added to an ABT is reached with no config change, and a team moved
// between ABTs cannot leave a stale key behind.
func TestResolve_ABTResolvesFromTeamType(t *testing.T) {
	cre := []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}
	var members []teamMember
	for _, team := range cre {
		members = append(members,
			member(team, team+".lead@example.com", roleLead, ""),
			member(team, team+".t1@example.com", "engineer", "T1"))
	}
	// An SRE team and Americas must not be reached by the CRE ladder.
	members = append(members,
		teamMember{TeamKey: "apollo", TeamType: "sre-abt", Email: "apollo.lead@example.com",
			Name: "apollo.lead@example.com", UserID: "apollo.lead", Role: roleLead},
		member("americas", "am.lead@example.com", roleLead, ""))

	byType := TeamKeys{ABTType: "cre-abt", Americas: "americas"}
	r := NewTeamScheduleResolver(&stubScheduleReader{members: members}, byType, nil)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	// A team in the ABT routes by the assigned-to-an-ABT row.
	rule, ok := r.RuleFor(rc)
	if !ok || rule.ID != "R2" {
		t.Fatalf("vega routed by %v, want R2", rule.ID)
	}
	// Americas is type cre, so it is NOT an ABT team -- which is what sends a
	// night incident down R5 rather than R2.
	if rule, _ := r.RuleFor(RoutingContext{Shift: ShiftLK, AssignedCRETeam: "americas"}); rule.ID != "R3" {
		t.Errorf("americas routed by %s, want R3 (not an ABT team)", rule.ID)
	}

	leads, err := r.Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != len(cre) {
		t.Errorf("LEVEL_2 reached %d, want every CRE ABT lead (%d): %v", len(leads), len(cre), emails(leads))
	}
	for _, e := range emails(leads) {
		if strings.HasPrefix(e, "apollo") || strings.HasPrefix(e, "am.") {
			t.Errorf("%s was called; it is not in cre-abt", e)
		}
	}

	// And one nominee from each team in the ABT, not from outside it.
	nominees, err := r.Resolve(context.Background(), Level0,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "not-an-abt", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(nominees) != len(cre) {
		t.Errorf("R3 LEVEL_0 reached %d, want one per CRE ABT team (%d)", len(nominees), len(cre))
	}
}

// "Team leads" is at least three OUT OF the pool, and the pool is every ABT
// team's lead. The sheet's three and the ABT's seven were never in conflict --
// one is how many get called, the other how many there are to choose from.
func TestResolve_TeamLeadsCallsNFromThePool(t *testing.T) {
	cre := []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}
	var members []teamMember
	for _, team := range cre {
		members = append(members, member(team, team+".lead@example.com", roleLead, ""))
	}
	stub := &stubScheduleReader{members: members}
	teams := testTeams
	teams.ABTs = cre
	teams.TeamLeadsToCall = 3
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	got, err := NewTeamScheduleResolver(stub, teams, nil).
		Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("LEVEL_2 called %d, want 3 from a pool of %d: %v", len(got), len(cre), emails(got))
	}

	// Zero means the whole pool, which is the other reading.
	teams.TeamLeadsToCall = 0
	all, err := NewTeamScheduleResolver(stub, teams, nil).
		Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(cre) {
		t.Errorf("with no cap LEVEL_2 called %d, want the whole pool (%d)", len(all), len(cre))
	}
}

// The three rotate: whoever has gone longest without a call goes first, so the
// duty spreads across the pool rather than always landing on the same names.
func TestResolve_TeamLeadsRotateAcrossThePool(t *testing.T) {
	now := time.Date(2026, 9, 30, 11, 0, 0, 0, time.UTC)
	cre := []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}
	var members []teamMember
	for _, team := range cre {
		members = append(members, member(team, team+".lead@example.com", roleLead, ""))
	}
	// Everyone called recently except three.
	seen := map[string]time.Time{}
	for i, team := range cre {
		seen[team+".lead@example.com"] = now.Add(-time.Duration(i) * time.Hour)
	}
	teams := testTeams
	teams.ABTs = cre
	teams.TeamLeadsToCall = 3

	got, err := NewTeamScheduleResolver(&stubScheduleReader{members: members}, teams, nil).
		WithCallHistory(stubHistory{seen: seen}).
		Resolve(context.Background(), Level2,
			RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: now})
	if err != nil {
		t.Fatal(err)
	}
	// The three longest-waiting are the last three in that -i hour ordering.
	want := []string{"vega.lead@example.com", "sirius.lead@example.com", "rigel.lead@example.com"}
	if !equalStrings(emails(got), want) {
		t.Errorf("LEVEL_2 = %v, want the three longest without a call %v", emails(got), want)
	}
}

// An incident with no team is a definite "not on an ABT team", not an unknown.
//
// Regression test: reading it as unknown left every ABTYes/ABTNo row
// unmatchable, and LK and LK_EVENING have only that pair -- so an unassigned
// incident during business hours matched no rule, every rung resolved to
// nobody, and it was never paged. Not being on a team is what R3 and R4b are
// for.
func TestRuleFor_NoTeamMatchesTheNotAssignedRow(t *testing.T) {
	r := testResolver(&stubScheduleReader{})
	for _, tc := range []struct {
		shift Shift
		want  string
	}{
		{ShiftLK, "R3"},
		{ShiftLKEvening, "R4b"},
	} {
		rule, ok := r.RuleFor(RoutingContext{Shift: tc.shift, AssignedCRETeam: ""})
		if !ok {
			t.Fatalf("%s with no team matched no rule at all", tc.shift)
		}
		if rule.ID != tc.want {
			t.Errorf("%s with no team routed by %s, want %s", tc.shift, rule.ID, tc.want)
		}
	}
}

// LEVEL_2 on the night shift is ONE person, never the pool of Team leads
// LEVEL_1 just called: the named override when set, else the holder of
// americas_team_lead -- and with neither, nobody rather than a guess.
func TestResolve_AmericasTeamLeadIsOnePerson(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("americas", "am1@example.com", roleLead, ""),
		member("americas", "am2@example.com", roleLead, ""),
		member("americas", "am3@example.com", roleLead, ""),
	}}
	rc := RoutingContext{Shift: ShiftUSA, AssignedCRETeam: "vega", At: time.Now()}

	// Named outright, which is the intended configuration.
	named := testTeams
	named.AmericasLead = Person{Name: "Americas Lead", Email: "above@example.com"}
	got, err := NewTeamScheduleResolver(stub, named, nil).Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Email != "above@example.com" {
		t.Errorf("LEVEL_2 = %v, want the one named lead", emails(got))
	}

	// Unnamed and no americas_team_lead held: not the pool, and not a guess.
	fallback, err := NewTeamScheduleResolver(stub, testTeams, nil).Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(fallback) != 0 {
		t.Errorf("LEVEL_2 = %v, want nobody: the three Team leads are LEVEL_1's", emails(fallback))
	}
}

// The nominations are an order, not three interchangeable labels.
//
// T1 is called first, then T2, then T3 -- so the emails here are deliberately
// in the OPPOSITE alphabetical order to the tiers. alertDuty used to sort the
// recipients by email right after takePerTeam had put them in tier order,
// which silently handed the rung back T3-first whenever the addresses happened
// to sort that way. A real run against a seeded roster is how it was found:
// nominating T1/T2/T3 and watching LEVEL_0 call them in email order.
func TestResolve_AlertDutyKeepsNominationOrder(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("vega", "zara@example.com", "engineer", "T1"),
		member("vega", "mina@example.com", "engineer", "T2"),
		member("vega", "abel@example.com", "engineer", "T3"),
	}}
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "vega", At: time.Now()}

	want := []string{"zara@example.com", "mina@example.com", "abel@example.com"}

	// Both ways of configuring the rung: "all of them" (perTeam 0) and an
	// explicit span. perTeam 0 used to skip the sort entirely, so "all three
	// nominees" came back in whatever order entity-service listed them while
	// "the first two" came back ordered -- the same rung disagreeing with
	// itself depending on a number that only says how many to keep.
	for _, perTeam := range []int{0, 3} {
		got, err := testResolver(stub).WithAlertDuty(nil, perTeam).
			Resolve(context.Background(), Level0, rc)
		if err != nil {
			t.Fatalf("perTeam=%d: %v", perTeam, err)
		}
		if !slices.Equal(emails(got), want) {
			t.Errorf("perTeam=%d LEVEL_0 = %v, want T1 then T2 then T3 (%v)",
				perTeam, emails(got), want)
		}
	}

	// And a truncated rung takes the LOWEST tiers, not the first two
	// alphabetically.
	two, err := testResolver(stub).WithAlertDuty(nil, 2).
		Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(emails(two), want[:2]) {
		t.Errorf("perTeam=2 LEVEL_0 = %v, want %v", emails(two), want[:2])
	}
}

// Level 1 must reach somebody when the incident is on no ABT.
//
// R3, R4b and R1-with-no-team all route an incident that belongs to no ABT,
// and Level 1 is "the team lead" -- the incident's OWN ABT's lead, which does
// not exist for these. The rung resolved to nobody, BuildPlan recorded
// NO_RECIPIENTS and dropped it, and because the rungs above keep their own
// offsets the ladder went silent for Level 1's whole budget: a P0 placed its
// Level 0 calls at +0m and then nothing until Level 2 at +4m. Proved against
// a running Team Schedule before this existed -- the engine logged
// levels="[LEVEL_0 LEVEL_2 LEVEL_3 LEVEL_4]".
//
// Climbing past an EMPTY rung is still right; this is about a rung that could
// never fire for any unassigned incident, which is a different thing.
func TestResolve_UnassignedIncidentStillReachesATeamLead(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("vega", "vega.lead@example.com", roleLead, ""),
		member("castor", "castor.lead@example.com", roleLead, ""),
		member("atlas", "atlas.lead@example.com", roleLead, ""),
	}}
	// No AssignedCRETeam: the incident is on no ABT.
	rc := RoutingContext{Shift: ShiftLK, At: time.Now()}

	got, err := testResolver(stub).Resolve(context.Background(), Level1, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("LEVEL_1 = %v, want exactly one lead from the pool", emails(got))
	}
	if !strings.HasSuffix(got[0].Email, ".lead@example.com") {
		t.Errorf("LEVEL_1 = %v, want somebody from the team-lead pool", emails(got))
	}

	// One, not three -- Level 2 is the three-lead rung, and blurring them
	// would spend Level 2's people on Level 1's budget.
	three, err := testResolver(stub).Resolve(context.Background(), Level2, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(three) <= len(got) {
		t.Errorf("LEVEL_2 = %v, want more than LEVEL_1's %v", emails(three), emails(got))
	}

	// An incident that DOES name an ABT is unaffected: it still gets that
	// team's own lead, not the pool.
	own, err := testResolver(stub).Resolve(context.Background(), Level1,
		RoutingContext{Shift: ShiftLK, AssignedCRETeam: "castor", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != 1 || own[0].Email != "castor.lead@example.com" {
		t.Errorf("LEVEL_1 for castor = %v, want castor's own lead", emails(own))
	}
}

// A CRE rota rung reaches CRE people only.
//
// The Team Schedule's on-duty list covers every rota, the SRE teams' included,
// and the rota rungs used to take it whole: a real run's weekday-morning
// LEVEL_0 was ten Apollo and Artemis engineers and one CRE engineer, and each
// got a card in the room. SRE is reached through its own ladder, on a P0, one
// person per rung -- never as a CRE rota member.
func TestResolve_RotaRungsReachOnlyThisLaddersTeams(t *testing.T) {
	cre := TeamKeys{
		ABTs:       []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"},
		Americas:   "americas",
		Leadership: "cre-leadership",
	}
	onDuty := []onDutyAssignment{
		onDutyFor("u1", "apollo.on@example.com", "apollo"),       // SRE
		onDutyFor("u2", "artemis.on@example.com", "artemis"),     // SRE
		onDutyFor("u3", "vega.on@example.com", "vega"),           // CRE ABT
		onDutyFor("u4", "castor.on@example.com", "castor"),       // CRE ABT
		onDutyFor("u5", "americas.on@example.com", "americas"),   // night team
		onDutyFor("u6", "migration.on@example.com", "migration"), // type cre, not an ABT
	}
	want := map[string]bool{"vega.on@example.com": true, "castor.on@example.com": true, "americas.on@example.com": true}
	at := time.Now()

	// R1a: every rota member on duty -- of this ladder's teams.
	got, err := NewTeamScheduleResolver(&stubScheduleReader{onDuty: onDuty}, cre, nil).
		Resolve(context.Background(), Level0, RoutingContext{Shift: ShiftLKMorning, AssignedCRETeam: "vega", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("R1a LEVEL_0 = %v, want only %v", emails(got), want)
	}
	for _, r := range got {
		if !want[r.Email] {
			t.Errorf("R1a LEVEL_0 reached %s, who is not on this ladder's rota", r.Email)
		}
	}

	// R4a: own rota member first, then ONE other -- and the other is CRE too.
	pair, err := NewTeamScheduleResolver(&stubScheduleReader{onDuty: onDuty}, cre, nil).
		Resolve(context.Background(), Level0, RoutingContext{Shift: ShiftLKEvening, AssignedCRETeam: "vega", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if len(pair) != 2 || pair[0].Email != "vega.on@example.com" || !want[pair[1].Email] {
		t.Errorf("R4a LEVEL_0 = %v, want vega's own member then one other CRE member", emails(pair))
	}

	// An explicit rotaTeams list replaces the default.
	only := cre
	only.RotaTeams = []string{"Castor"}
	got, err = NewTeamScheduleResolver(&stubScheduleReader{onDuty: onDuty}, only, nil).
		Resolve(context.Background(), Level0, RoutingContext{Shift: ShiftLKMorning, AssignedCRETeam: "vega", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Email != "castor.on@example.com" {
		t.Errorf("rotaTeams [Castor]: LEVEL_0 = %v, want castor only", emails(got))
	}
}

// A case carries its team's name; a deployment whose team keys are not slugs of
// it (DEV: "rigel_abt_cre_team") maps the name through teams.aliases. Every CRE
// lookup has to honour that alias, as the SRE ladder's already does.
//
// Regression test: the CRE side slugified the name and skipped the alias, so on
// DEV "Rigel" became "rigel", a team that does not exist -- every ABT case
// routed by R3 and its own-team rungs reached nobody.
func TestResolve_CRETeamNameMapsThroughAlias(t *testing.T) {
	stub := &stubScheduleReader{members: []teamMember{
		member("rigel_abt_cre_team", "r1@example.com", "engineer", "T1"),
		member("rigel_abt_cre_team", "r2@example.com", "engineer", "T2"),
		member("rigel_abt_cre_team", "r3@example.com", "engineer", "T3"),
		member("rigel_abt_cre_team", "lead@example.com", roleLead, ""),
	}}
	teams := TeamKeys{
		ABTType:    "cre-abt",
		Americas:   "americas_cre_team",
		Leadership: "cre-leadership",
		Aliases:    map[string]string{"rigel": "rigel_abt_cre_team"},
	}
	r := NewTeamScheduleResolver(stub, teams, nil)
	rc := RoutingContext{Shift: ShiftLK, AssignedCRETeam: "Rigel", At: time.Now()}

	if rule, ok := r.RuleFor(rc); !ok || rule.ID != "R2" {
		t.Fatalf("a Rigel case in LK routed by %q (matched %v), want R2", rule.ID, ok)
	}
	tier1, err := r.Resolve(context.Background(), Level0, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(tier1); len(got) != 3 || got[0] != "r1@example.com" || got[2] != "r3@example.com" {
		t.Errorf("LEVEL_0 = %v, want Rigel's T1..T3", got)
	}
	tier2, err := r.Resolve(context.Background(), Level1, rc)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(tier2); len(got) != 1 || got[0] != "lead@example.com" {
		t.Errorf("LEVEL_1 = %v, want Rigel's own lead", got)
	}
}
