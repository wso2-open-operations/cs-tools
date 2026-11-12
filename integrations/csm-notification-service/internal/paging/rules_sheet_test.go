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
	"sort"
	"testing"
	"time"
)

// The CRE rules sheet ("[Updated] CRE Notification Escalation rules", the
// 5 Oct 2026 export of the live Google Sheet) transcribed by hand. This is
// the test's own copy of the spec: it must never be derived from rules.go or
// policy.go, or the test would compare the code against itself.
//
// Twillio calls tab: calls per level, Level 0..Level 4.
var sheetCalls = []struct {
	rule  string
	shift Shift
	team  string // "" = not assigned to an ABT
	calls [5]int
	note  string
}{
	{"R1a", ShiftLKMorning, "vega", [5]int{2, 1, 3, 1, 1}, "LK_MORNING, on an ABT"},
	{"R1a", ShiftLKMorning, "", [5]int{2, 1, 3, 1, 1}, "LK_MORNING, unassigned"},
	{"R1b", ShiftLKWeekend, "vega", [5]int{3, 1, 3, 1, 1}, "LK_WEEKEND"},
	{"R2", ShiftLK, "vega", [5]int{3, 1, 3, 1, 1}, "LK, on an ABT"},
	// The sheet's own cell says 3 (total 9); the product owner ruled it is one
	// nominee from each of the 7 ABTs (7, total 13), as the Rules tab's "for
	// all ABT" wording says. 7 is what this pins.
	{"R3", ShiftLK, "", [5]int{7, 1, 3, 1, 1}, "LK, unassigned (sheet cell 3, ruled 7)"},
	{"R4a", ShiftLKEvening, "vega", [5]int{2, 1, 3, 1, 1}, "LK_EVENING, on an ABT"},
	{"R4b", ShiftLKEvening, "", [5]int{7, 1, 3, 1, 1}, "LK_EVENING, unassigned"},
	{"R5", ShiftUSA, "vega", [5]int{3, 3, 1, 1, 1}, "AMERICAS"},
	{"R5", ShiftUSA, "", [5]int{3, 3, 1, 1, 1}, "AMERICAS, unassigned"},
	{"R6", ShiftUSAWeekend, "vega", [5]int{4, 3, 1, 1, 1}, "AMERICAS_WEEKEND"},
	{"R6", ShiftUSAWeekend, "", [5]int{4, 3, 1, 1, 1}, "AMERICAS_WEEKEND, unassigned"},
}

// Escalation ladder tab: minutes from the report to each level opening.
var sheetOpens = map[string][5]int{
	"S0": {0, 1, 4, 8, 12},
	"S1": {6, 9, 18, 28, 38},
	"S2": {9, 15, 30, 45, 60},
	"S3": {12, 20, 40, 65, 90},
	"S4": {15, 23, 53, 83, 113},
}

// sheetSchedule is a Team Schedule shaped like the real one: seven ABTs each
// with a lead, three nominees and two more engineers; Americas with three
// nominees, three Team leads and the America Team lead; the two heads in
// cre-leadership. Rota members on duty are what each shift really rosters.
func sheetSchedule(abts []string, shift Shift) *stubScheduleReader {
	var ms []teamMember
	for _, team := range abts {
		ms = append(ms,
			member(team, team+".lead@example.com", "lead", ""),
			member(team, team+".t1@example.com", "engineer", "T1"),
			member(team, team+".t2@example.com", "engineer", "T2"),
			member(team, team+".t3@example.com", "engineer", "T3"),
			member(team, team+".e4@example.com", "engineer", ""),
			member(team, team+".e5@example.com", "engineer", ""))
	}
	ms = append(ms,
		member("americas", "am.t1@example.com", "engineer", "T1"),
		member("americas", "am.t2@example.com", "engineer", "T2"),
		member("americas", "am.t3@example.com", "engineer", "T3"),
		member("americas", "am.lead1@example.com", "lead", ""),
		member("americas", "am.lead2@example.com", "lead", ""),
		member("americas", "am.lead3@example.com", "lead", ""),
		member("americas", "am.americalead@example.com", "americas_team_lead", ""),
		member("cre-leadership", "cre.head@example.com", "cre_head", ""),
		member("cre-leadership", "cs.head@example.com", "cs_head", ""))

	on := func(i int, team, code string) onDutyAssignment {
		a := onDutyFor(fmt.Sprintf("u%d", i), fmt.Sprintf("%s.rota%d@example.com", team, i), team)
		a.ShiftCode = code
		return a
	}
	var duty []onDutyAssignment
	switch shift {
	case ShiftLKMorning: // Twillio calls: 2 rota members
		duty = []onDutyAssignment{on(1, "vega", "CRE_MORNING"), on(2, "atlas", "CRE_MORNING_OC")}
	case ShiftLKWeekend: // 3
		duty = []onDutyAssignment{on(1, "vega", "CRE_WEEKEND"), on(2, "atlas", "CRE_WEEKEND"), on(3, "draco", "CRE_WEEKEND")}
	case ShiftLKEvening: // 7, one per ABT
		for i, team := range abts {
			duty = append(duty, on(i+1, team, "CRE_EVENING"))
		}
	case ShiftUSAWeekend: // the rostered Americas weekend member, and the on-call beside it
		duty = []onDutyAssignment{on(1, "americas", "CRE_WEEKEND_NIGHT"), on(2, "draco", "CRE_WEEKEND_NIGHT_OC")}
	}
	return &stubScheduleReader{members: ms, onDuty: duty}
}

type everyoneHasANumber struct{}

func (everyoneHasANumber) MobileNumber(_ context.Context, email string) (string, error) {
	return "+9477" + fmt.Sprintf("%07d", len(email)*7919%10000000), nil
}

// TestRulesSheet builds every rule's plan the way cmd/server does -- the
// shipped paging-alert.yaml, the Team Schedule resolver, wrapped by the profile
// phone lookup -- and holds it to the sheet: the rule matched, how many people
// each level calls, and when each level opens, for every priority.
func TestRulesSheet(t *testing.T) {
	cfg, err := LoadConfig("../../../../scripts/csm-compose/paging-alert.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cre, ok := cfg.For(LadderKeyCRE)
	if !ok {
		t.Fatal("shipped paging-alert.yaml has no enabled cre ladder")
	}
	// The sheet is the rule table, so a case on no ABT is held to its R3/R4b
	// rows; by default it goes to the shift's leads (unassigned_leads_test.go).
	cre.Teams.UnassignedLeads = "rules"
	reported := time.Date(2026, 10, 10, 16, 30, 0, 0, time.UTC) // any instant; the shift is given

	for _, row := range sheetCalls {
		for prio, opens := range sheetOpens {
			t.Run(fmt.Sprintf("%s/%s/%s", row.rule, row.note, prio), func(t *testing.T) {
				stub := sheetSchedule(cre.Teams.ABTs, row.shift)
				resolver := NewProfilePhoneResolver(
					NewTeamScheduleResolver(stub, cre.Teams, cre.Rules).
						WithHeads(cre.Heads).
						WithAlertDuty(cre.AlertTiers(), cre.AlertDuty.PerTeam),
					everyoneHasANumber{})
				trig := Trigger{IncidentID: "inc-1", Number: "INC0012345", Priority: prio, At: reported,
					Routing: RoutingContext{Shift: row.shift, AssignedCRETeam: row.team, At: reported}}
				plan, err := BuildPlan(context.Background(), trig, DefaultPolicy, resolver, ChannelCall)
				if err != nil {
					t.Fatal(err)
				}
				if got := plan.Trigger.Routing.RuleID; got != row.rule {
					t.Fatalf("rule = %q, want %s", got, row.rule)
				}
				people := map[Level]map[string]bool{}
				first := map[Level]time.Time{}
				for _, c := range plan.Calls {
					if people[c.Level] == nil {
						people[c.Level] = map[string]bool{}
					}
					people[c.Level][c.Recipient.Email] = true
					if f, ok := first[c.Level]; !ok || c.At.Before(f) {
						first[c.Level] = c.At
					}
				}
				for lvl := Level0; lvl <= Level4; lvl++ {
					if got, want := len(people[lvl]), row.calls[lvl]; got != want {
						t.Errorf("%s calls %d people %v, sheet says %d", lvl, got, keys(people[lvl]), want)
					}
					if got, want := first[lvl].Sub(reported), time.Duration(opens[lvl])*time.Minute; got != want {
						t.Errorf("%s opens at +%s, sheet says +%s", lvl, got, want)
					}
				}
				for _, issue := range plan.Issues {
					t.Errorf("unexpected plan issue %s at %s", issue.Reason, issue.Level)
				}
				// Who, not only how many, where the sheet names a person.
				if !people[Level3]["cre.head@example.com"] || !people[Level4]["cs.head@example.com"] {
					t.Errorf("heads = %v / %v", keys(people[Level3]), keys(people[Level4]))
				}
				if row.rule == "R6" {
					if !people[Level0]["americas.rota1@example.com"] || people[Level0]["draco.rota2@example.com"] {
						t.Errorf("R6 LEVEL_0 = %v; want the Americas weekend rota member, never the on-call", keys(people[Level0]))
					}
				}
				if row.rule == "R5" || row.rule == "R6" {
					if !people[Level2]["am.americalead@example.com"] {
						t.Errorf("night LEVEL_2 = %v, want the America Team lead", keys(people[Level2]))
					}
				}
			})
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
