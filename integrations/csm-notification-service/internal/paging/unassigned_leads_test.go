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
	"fmt"
	"slices"
	"testing"
	"time"
)

// abtNames are the seven CRE ABTs as deployed.
var abtNames = []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}

// unassignedStub holds a lead and an engineer for each of the seven ABTs, the
// Americas team's three leads, the Australia team's lead and the two heads.
// Every lead and head has a number.
func unassignedStub() *stubScheduleReader {
	var ms []teamMember
	phone := 0
	add := func(m teamMember) {
		phone++
		ms = append(ms, m)
	}
	for _, team := range abtNames {
		add(member(team, team+".lead@example.com", roleLead, ""))
		add(member(team, team+".eng@example.com", "engineer", "T1"))
	}
	for i := 1; i <= 3; i++ {
		add(member("americas", fmt.Sprintf("americas.lead%d@example.com", i), roleLead, ""))
	}
	add(teamMember{TeamKey: "australia", TeamType: "cre", Role: roleLead,
		Email: "australia.lead@example.com", Name: "australia.lead@example.com", UserID: "australia.lead@example.com"})
	add(member("cre-leadership", "cre.head@example.com", roleCREHead, ""))
	add(member("cre-leadership", "cs.head@example.com", roleCSHead, ""))
	return &stubScheduleReader{members: ms}
}

// shiftLeadsTeams is the deployed teams with teams.unassignedLeads switched on.
func shiftLeadsTeams(abts []string) TeamKeys {
	return TeamKeys{
		ABTs:            abts,
		ABTType:         "cre-abt",
		Americas:        "americas",
		Leadership:      "cre-leadership",
		TeamLeadsToCall: 3,
		UnassignedLeads: "shift_leads",
	}
}

func abtLeadEmails() []string {
	out := make([]string, 0, len(abtNames))
	for _, team := range abtNames {
		out = append(out, team+".lead@example.com")
	}
	return out
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// Unset, a case on no ABT goes to the shift's leads (U1 by IST day, U2 at
// night); "rules" brings the rule table back (R3 and the others).
func TestUnassignedLeads_DefaultIsTheShiftsLeads(t *testing.T) {
	unset := NewTeamScheduleResolver(unassignedStub(), TeamKeys{ABTs: abtNames, Americas: "americas"}, nil)
	rules := NewTeamScheduleResolver(unassignedStub(), TeamKeys{ABTs: abtNames, Americas: "americas", UnassignedLeads: "rules"}, nil)
	for shift, want := range map[Shift][2]string{
		ShiftLK: {RuleUnassignedDay, "R3"}, ShiftLKEvening: {RuleUnassignedDay, "R4b"},
		ShiftLKMorning: {RuleUnassignedDay, "R1a"}, ShiftLKWeekend: {RuleUnassignedDay, "R1b"},
		ShiftUSA: {RuleUnassignedNight, "R5"}, ShiftUSAWeekend: {RuleUnassignedNight, "R6"},
	} {
		if rule, _ := unset.RuleFor(RoutingContext{Shift: shift}); rule.ID != want[0] {
			t.Errorf("%s, nothing set: routed by %s, want %s", shift, rule.ID, want[0])
		}
		if rule, _ := rules.RuleFor(RoutingContext{Shift: shift}); rule.ID != want[1] {
			t.Errorf("%s, rules: routed by %s, want %s", shift, rule.ID, want[1])
		}
	}
	for v, want := range map[string]string{"": unassignedLeadsShift, "bogus": unassignedLeadsShift, " Rules ": unassignedLeadsRules} {
		if got := unassignedLeadsMode(v); got != want {
			t.Errorf("unassignedLeadsMode(%q) = %q, want %q", v, got, want)
		}
	}
}

// With shift_leads, every shift sends a case on no ABT to U1 (IST day) or U2
// (night); a case on an ABT keeps its own row.
func TestUnassignedLeads_Routing(t *testing.T) {
	r := NewTeamScheduleResolver(unassignedStub(), shiftLeadsTeams(abtNames), nil)
	for _, tc := range []struct {
		shift Shift
		team  string
		want  string
	}{
		{ShiftLK, "", RuleUnassignedDay},
		{ShiftLKMorning, "", RuleUnassignedDay},
		{ShiftLKEvening, "", RuleUnassignedDay},
		{ShiftLKWeekend, "", RuleUnassignedDay},
		{ShiftUSA, "", RuleUnassignedNight},
		{ShiftUSAWeekend, "", RuleUnassignedNight},
		// A team that is not an ABT (a renamed or unmapped one) is no ABT.
		{ShiftLK, "Some Old Team", RuleUnassignedDay},
		// On an ABT: the rule table, unchanged.
		{ShiftLK, "Atlas", "R2"},
		{ShiftLKEvening, "Atlas", "R4a"},
		{ShiftLKMorning, "Atlas", "R1a"},
		{ShiftUSA, "Atlas", "R5"},
	} {
		rule, ok := r.RuleFor(RoutingContext{Shift: tc.shift, AssignedCRETeam: tc.team})
		if !ok || rule.ID != tc.want {
			t.Errorf("%s team %q: routed by %s, want %s", tc.shift, tc.team, rule.ID, tc.want)
		}
	}
}

// Who each rung reaches: L1 is every ABT lead through the IST day (not
// teamLeadsToCall of them), plus the Australia lead on the morning, and the
// Americas leads at night; L0 and L2 reach nobody; L3/L4 are the heads.
func TestUnassignedLeads_WhoIsCalled(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 11, 12, 8, 7, 48, 0, time.UTC)
	for _, cfg := range []struct {
		name string
		abts []string
	}{
		{"abts listed", abtNames},
		// The case this exists for: no ABT list saved anywhere. The ABTs are
		// found by team type, so nobody has to type their names in.
		{"abts found by type", nil},
	} {
		t.Run(cfg.name, func(t *testing.T) {
			r := NewTeamScheduleResolver(unassignedStub(), shiftLeadsTeams(cfg.abts), nil)
			day := sorted(abtLeadEmails())
			for _, tc := range []struct {
				shift Shift
				want  []string
			}{
				{ShiftLK, day},
				{ShiftLKEvening, day},
				{ShiftLKWeekend, day},
				{ShiftLKMorning, sorted(append(abtLeadEmails(), "australia.lead@example.com"))},
				{ShiftUSA, []string{"americas.lead1@example.com", "americas.lead2@example.com", "americas.lead3@example.com"}},
				{ShiftUSAWeekend, []string{"americas.lead1@example.com", "americas.lead2@example.com", "americas.lead3@example.com"}},
			} {
				rc := RoutingContext{Shift: tc.shift, At: at}
				l1, err := r.Resolve(ctx, Level1, rc)
				if err != nil {
					t.Fatal(err)
				}
				if got := sorted(emails(l1)); !slices.Equal(got, tc.want) {
					t.Errorf("%s L1 = %v, want %v", tc.shift, got, tc.want)
				}
				for _, l := range []Level{Level0, Level2} {
					if got, _ := r.Resolve(ctx, l, rc); len(got) != 0 {
						t.Errorf("%s %s = %v, want nobody", tc.shift, l, emails(got))
					}
				}
				if got, _ := r.Resolve(ctx, Level3, rc); len(got) != 1 || got[0].Email != "cre.head@example.com" {
					t.Errorf("%s L3 = %v, want the CRE head", tc.shift, emails(got))
				}
			}
		})
	}
}

// Seven leads are called once, not three times: twenty-one calls would let a
// cap of twelve cut the heads off. Three Americas leads keep every attempt.
func TestUnassignedLeads_SevenLeadsCalledOnce(t *testing.T) {
	ctx := context.Background()
	r := NewTeamScheduleResolver(unassignedStub(), shiftLeadsTeams(abtNames), nil).
		WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	for _, tc := range []struct {
		shift   Shift
		wantL1  int
		attempt int
	}{
		{ShiftLK, 7, 1},
		{ShiftUSA, 3, 3},
	} {
		tr := testTrigger("HIGH", tc.shift)
		tr.Routing.AssignedCRETeam = ""
		plan, err := BuildPlan(ctx, tr, DefaultPolicy, r, ChannelCall)
		if err != nil {
			t.Fatal(err)
		}
		var l1 int
		ordinals := map[int]bool{}
		for _, c := range plan.Calls {
			if c.Level == Level1 {
				l1++
				ordinals[c.Ordinal] = true
			}
		}
		if l1 != tc.wantL1*tc.attempt || len(ordinals) != tc.attempt {
			t.Errorf("%s: %d L1 calls over %d attempts, want %d over %d", tc.shift, l1, len(ordinals), tc.wantL1*tc.attempt, tc.attempt)
		}
		// With a cap of twelve (the deployed value) the CRE head is always
		// reached. By day so is the CS head; at night three Americas leads
		// called three times leave the CS head to the cap, as before.
		e := &Engine{cfg: EngineConfig{Channel: ChannelCall, Ladder: LadderConfig{
			Safety: Safety{MaxCallsPerLadder: 12, CallWithoutVerifiedLeads: true}}}}
		e.applySafety(ctx, &plan)
		levels := levelsOf(plan.Calls)
		if !slices.Contains(levels, Level3) || (tc.shift == ShiftLK && !slices.Contains(levels, Level4)) {
			t.Errorf("%s: with a cap of 12 the calls are %v; a head is cut off", tc.shift, levels)
		}
	}
}

// The guarantee this exists for: on every shift, for a case on an ABT and on
// none, in both modes, the heads are never the first people a CRE case calls.
// Whoever the plan reaches first sits below the heads.
func TestNoCRECaseCallsTheHeadsFirst(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"rules", "shift_leads"} {
		teams := shiftLeadsTeams(abtNames)
		teams.UnassignedLeads = mode
		stub := unassignedStub()
		// The rota rungs (R1a/R1b/R4b/R6) need somebody on duty.
		for _, team := range abtNames {
			stub.onDuty = append(stub.onDuty, onDutyFor(team+".eng@example.com", team+".eng@example.com", team))
		}
		r := NewTeamScheduleResolver(stub, teams, nil).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
		for _, shift := range allShifts {
			for _, team := range []string{"", "Atlas"} {
				for _, priority := range []string{"CATASTROPHIC", "CRITICAL", "HIGH", "MEDIUM"} {
					tr := testTrigger(priority, shift)
					tr.Routing.AssignedCRETeam = team
					plan, err := BuildPlan(ctx, tr, DefaultPolicy, r, ChannelCall)
					if err != nil {
						t.Fatal(err)
					}
					name := fmt.Sprintf("%s %s team %q %s", mode, shift, team, priority)
					if len(plan.Calls) == 0 {
						t.Errorf("%s: nobody is called", name)
						continue
					}
					if first := plan.Calls[0]; first.Level >= Level3 {
						t.Errorf("%s: the first call is %s (%s), a head", name, first.Level, first.Recipient.Email)
					}
				}
			}
		}
	}
}

// And when nobody below the heads can be found at all, the heads' calls are
// held rather than ringing first (Safety.CallHeadsWithoutLowerTiers, off by
// default): with shift_leads and no leads anywhere, the plan holds them.
func TestUnassignedLeads_NoLeadsHoldsTheHeads(t *testing.T) {
	ctx := context.Background()
	stub := &stubScheduleReader{members: []teamMember{
		member("cre-leadership", "cre.head@example.com", roleCREHead, ""),
		member("cre-leadership", "cs.head@example.com", roleCSHead, ""),
	}}
	r := NewTeamScheduleResolver(stub, shiftLeadsTeams(abtNames), nil).WithPhoneBook(PhoneBook{TestCallTo: "+94770000000"})
	tr := testTrigger("HIGH", ShiftLK)
	tr.Routing.AssignedCRETeam = ""
	plan, err := BuildPlan(ctx, tr, DefaultPolicy, r, ChannelCall)
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{cfg: EngineConfig{Channel: ChannelCall}}
	e.applySafety(ctx, &plan)
	if len(plan.Calls) != 0 {
		t.Errorf("calls = %v; the heads must not ring when nobody below them can be called", levelsOf(plan.Calls))
	}
}

// The L1 email of a case on no ABT goes to the leads its call reaches, with
// every ABT list copied by day and only the Americas list at night.
func TestUnassignedLeads_L1Email(t *testing.T) {
	ctx := context.Background()
	lists := map[string]string{"americas": "americas-list@example.test"}
	for _, team := range abtNames {
		lists[team] = team + "-list@example.test"
	}
	e := &Engine{cfg: EngineConfig{Ladder: LadderConfig{
		Teams: TeamKeys{ABTs: abtNames, Americas: "americas"},
		Email: RungEmail{TeamLists: lists},
	}}}
	leads := []Recipient{{Name: "Atlas Lead", Email: "atlas.lead@example.com"}, {Name: "Vega Lead", Email: "vega.lead@example.com"}}

	day := Trigger{Routing: RoutingContext{RuleID: RuleUnassignedDay}}
	to, greeting := e.rungEmailTo(ctx, day, Level1, false, leads)
	if len(to) != 2 || greeting != "Team Leads" {
		t.Errorf("day L1 email to %v (%q), want both leads as \"Team Leads\"", emails(to), greeting)
	}
	if cc := e.rungEmailLists(day, Level1, false); len(cc) != len(abtNames) || slices.Contains(cc, "americas-list@example.test") {
		t.Errorf("day L1 Cc = %v, want every ABT list and not Americas'", cc)
	}

	night := Trigger{Routing: RoutingContext{RuleID: RuleUnassignedNight}}
	if cc := e.rungEmailLists(night, Level1, true); !slices.Equal(cc, []string{"americas-list@example.test"}) {
		t.Errorf("night L1 Cc = %v, want only the Americas list", cc)
	}
}
