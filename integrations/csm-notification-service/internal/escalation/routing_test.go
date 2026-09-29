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

package escalation

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// Section 6.0's matrix and section 5.0's rule table agree that LEVEL_0 on
// USA_WEEKEND depends on the business unit: R10 (ABT-eligible) has no
// notification level, while R12 and R14 (not ABT-eligible) do. Deciding this
// from the shift alone gave an ABT-eligible USA-weekend incident a LEVEL_0 the
// specification does not define, shifting every later level with it.
func TestHasNotificationLevel_MatchesTheRuleTable(t *testing.T) {
	tests := []struct {
		rule        string
		shift       Shift
		abtEligible bool
		want        bool
	}{
		{"R1 (business hours, ABT)", ShiftLK, true, false},
		{"R5 (business hours, sub team)", ShiftLK, false, false},
		{"R9 (USA business hours)", ShiftUSA, true, false},
		{"R2 (LK morning rotation, ABT)", ShiftLKMorning, true, true},
		{"R6 (LK morning rotation, sub team)", ShiftLKMorning, false, true},
		{"R4 (LK evening rotation)", ShiftLKEvening, true, true},
		{"R8 (LK weekend rotation)", ShiftLKWeekend, false, true},
		{"R10 (USA weekend, ABT-eligible)", ShiftUSAWeekend, true, false},
		{"R12 (USA weekend, not ABT-eligible)", ShiftUSAWeekend, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.rule, func(t *testing.T) {
			rc := RoutingContext{Shift: tc.shift, ABTEligible: abtFor(tc.abtEligible)}
			if got := rc.HasNotificationLevel(); got != tc.want {
				t.Errorf("%s: HasNotificationLevel() = %v, want %v", tc.rule, got, tc.want)
			}
		})
	}
}

// The rule above has to reach the plan, not just the predicate.
func TestBuildPlan_ABTEligibleUSAWeekendStartsAtLevel1(t *testing.T) {
	tr := testTrigger("P2", ShiftUSAWeekend)
	tr.Routing.ABTEligible = abtYes()
	plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, fullResolver())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Calls[0].Level != Level1 {
		t.Errorf("first call is %s, want LEVEL_1 (R10 defines no notification level)", plan.Calls[0].Level)
	}

	tr.Routing.ABTEligible = abtNo()
	plan, err = BuildPlan(context.Background(), tr, DefaultPolicy, fullResolver())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Calls[0].Level != Level0 {
		t.Errorf("first call is %s, want LEVEL_0 (R12 defines one)", plan.Calls[0].Level)
	}
}

// entity-service publishes ServiceNow's own incident priority enum, whose
// MODERATE is the spelling the case-severity vocabulary calls MEDIUM. Before
// this alias existed, a MODERATE incident resolved to no policy at all and got
// no ladder — two of the five incident priorities were silently uncovered.
func TestLookup_ResolvesIncidentPriorityVocabulary(t *testing.T) {
	for _, tc := range []struct {
		priority string
		want     string
	}{
		{"CRITICAL", "P1"}, // section 10.0's example reads "Critical (P1)"
		{"HIGH", "P2"},
		{"MODERATE", "P3"},
		{"MEDIUM", "P3"}, // the case-severity spelling still resolves
		{"LOW", "P4"},
		{"CATASTROPHIC", "P0"},
		{"P3", "P3"}, // P-notation passes through
	} {
		t.Run(tc.priority, func(t *testing.T) {
			got, ok := Lookup(DefaultPolicy, tc.priority)
			if !ok {
				t.Fatalf("Lookup(%q) found no policy", tc.priority)
			}
			if want := DefaultPolicy[tc.want]; got.InitialWait != want.InitialWait {
				t.Errorf("Lookup(%q) resolved to the wrong row: initial wait %v, want %v (%s)",
					tc.priority, got.InitialWait, want.InitialWait, tc.want)
			}
		})
	}
}

// PLANNING is deliberately absent: section 7.0 has no row below P4, so a
// planning-priority incident has no ladder at all rather than the wrong one.
func TestLookup_PlanningHasNoLadder(t *testing.T) {
	if _, ok := Lookup(DefaultPolicy, "PLANNING"); ok {
		t.Error("PLANNING resolved to a policy; section 7.0 defines no row for it")
	}
}

// The roster's three tiers are alternatives in the rule table's own
// specificity order, not additive: merging them would call a team's own sub
// lead alongside every other team's pooled sub leads.
func TestRosterResolver_PrefersTheMostSpecificTier(t *testing.T) {
	r := Roster{
		ByTeam: map[string]LevelRoster{
			"Atlas": {"LEVEL_1": {rec("atlas.lead@wso2.com", "+94770000001")}},
		},
		ByShift: map[string]LevelRoster{
			"LK_MORNING": {"LEVEL_1": {rec("pool.lead@wso2.com", "+94770000002")}},
		},
		Default: LevelRoster{
			"LEVEL_1": {rec("fallback.lead@wso2.com", "+94770000003")},
			"LEVEL_3": {rec("bu.head@wso2.com", "+94770000004")},
		},
	}
	resolver := NewRosterResolver(r)
	ctx := context.Background()

	got, err := resolver.Resolve(ctx, Level1, RoutingContext{AssignedCRETeam: "Atlas", Shift: ShiftLKMorning})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Email != "atlas.lead@wso2.com" {
		t.Errorf("an assigned team must win over the shift pool, got %v", got)
	}

	// No team on the incident (rules R3/R4) falls through to the shift pool.
	got, _ = resolver.Resolve(ctx, Level1, RoutingContext{Shift: ShiftLKMorning})
	if len(got) != 1 || got[0].Email != "pool.lead@wso2.com" {
		t.Errorf("an unassigned team must fall back to the shift pool, got %v", got)
	}

	// Leadership levels live only in the default tier.
	got, _ = resolver.Resolve(ctx, Level3, RoutingContext{AssignedCRETeam: "Atlas", Shift: ShiftLKMorning})
	if len(got) != 1 || got[0].Email != "bu.head@wso2.com" {
		t.Errorf("leadership levels must come from the default tier, got %v", got)
	}

	// A level nobody is configured for resolves to nobody, which BuildPlan
	// records as an issue and skips rather than failing.
	if got, _ := resolver.Resolve(ctx, Level4, RoutingContext{Shift: ShiftLK}); len(got) != 0 {
		t.Errorf("an unconfigured level must resolve to nobody, got %v", got)
	}
}

func TestParseRoster(t *testing.T) {
	r, err := ParseRoster(`{"default":{"LEVEL_1":[{"Email":"a@wso2.com","Name":"A","Phone":"+94770000001"}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Default["LEVEL_1"]) != 1 {
		t.Fatalf("expected one LEVEL_1 recipient, got %v", r.Default)
	}
	if r.IsEmpty() {
		t.Error("a roster with recipients must not report empty")
	}

	empty, err := ParseRoster("")
	if err != nil {
		t.Fatalf("an empty roster must not be an error: %v", err)
	}
	if !empty.IsEmpty() {
		t.Error("an empty roster must report empty so main.go can refuse to start the engine")
	}
	if _, err := ParseRoster("{not json"); err == nil {
		t.Error("expected an error for malformed roster JSON")
	}
}

// Every row of section 5.0's table must be reachable and must be the one the
// document names, because this string is what a log line and the incident's
// own work note report as the path a call alert took.
func TestRule_CoversTheWholeTable(t *testing.T) {
	const (
		product = "WSO2 API Manager"
		team    = "Atlas"
	)
	tests := []struct {
		want string
		rc   RoutingContext
	}{
		// Integration / ABT-eligible, a CRE team assigned.
		{"R1", RoutingContext{Product: product, ABTEligible: abtYes(), AssignedCRETeam: team, Shift: ShiftLK}},
		{"R2", RoutingContext{Product: product, ABTEligible: abtYes(), AssignedCRETeam: team, Shift: ShiftLKMorning}},
		{"R2", RoutingContext{Product: product, ABTEligible: abtYes(), AssignedCRETeam: team, Shift: ShiftLKEvening}},
		{"R2", RoutingContext{Product: product, ABTEligible: abtYes(), AssignedCRETeam: team, Shift: ShiftLKWeekend}},
		// ABT-eligible, no team — the pooled rows.
		{"R3", RoutingContext{Product: product, ABTEligible: abtYes(), Shift: ShiftLK}},
		{"R4", RoutingContext{Product: product, ABTEligible: abtYes(), Shift: ShiftLKEvening}},
		// Sub-team model: the table stops consulting the team entirely, but
		// eligibility is still an explicit "No" rather than an absence.
		{"R5", RoutingContext{Product: product, ABTEligible: abtNo(), Shift: ShiftLK}},
		{"R5", RoutingContext{Product: product, ABTEligible: abtNo(), AssignedCRETeam: team, Shift: ShiftLK}},
		{"R6", RoutingContext{Product: product, ABTEligible: abtNo(), Shift: ShiftLKMorning}},
		// No product at all — section 12.0's erroneous scenario.
		{"R7", RoutingContext{Shift: ShiftLK}},
		{"R8", RoutingContext{Shift: ShiftLKWeekend}},
		{"R13", RoutingContext{Shift: ShiftUSA}},
		{"R14", RoutingContext{Shift: ShiftUSAWeekend}},
		// Americas.
		{"R9", RoutingContext{Product: product, ABTEligible: abtYes(), Shift: ShiftUSA}},
		{"R10", RoutingContext{Product: product, ABTEligible: abtYes(), Shift: ShiftUSAWeekend}},
		{"R11", RoutingContext{Product: product, ABTEligible: abtNo(), Shift: ShiftUSA}},
		{"R12", RoutingContext{Product: product, ABTEligible: abtNo(), Shift: ShiftUSAWeekend}},
	}
	seen := map[string]bool{}
	for _, tc := range tests {
		t.Run(tc.want+"/"+string(tc.rc.Shift), func(t *testing.T) {
			if got := tc.rc.Rule(); got != tc.want {
				t.Errorf("Rule() = %s, want %s (product=%q abt=%s team=%q shift=%s)",
					got, tc.want, tc.rc.Product, tc.rc.ABTEligibility(), tc.rc.AssignedCRETeam, tc.rc.Shift)
			}
		})
		seen[tc.want] = true
	}
	for i := 1; i <= 14; i++ {
		if rule := "R" + strconv.Itoa(i); !seen[rule] {
			t.Errorf("no case covers %s; the table has 14 rows and all must be reachable", rule)
		}
	}
}

// An unrecognised shift has no row, and reporting the wrong path is worse than
// admitting there is none.
func TestRule_UnknownShift(t *testing.T) {
	rc := RoutingContext{Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: Shift("MARS")}
	if got := rc.Rule(); got != "UNKNOWN" {
		t.Errorf("Rule() = %s, want UNKNOWN", got)
	}
}

// R10 and R12 differ only by ABT eligibility, and that difference is exactly
// the LEVEL_0 rule — so the reported path and the scheduled ladder must agree.
func TestRule_AgreesWithTheLevel0Rule(t *testing.T) {
	abt := RoutingContext{Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: ShiftUSAWeekend}
	iam := RoutingContext{Product: "WSO2 Identity Server", ABTEligible: abtNo(), Shift: ShiftUSAWeekend}

	if abt.Rule() != "R10" || abt.HasNotificationLevel() {
		t.Errorf("R10 must have no notification level: rule=%s level0=%t", abt.Rule(), abt.HasNotificationLevel())
	}
	if iam.Rule() != "R12" || !iam.HasNotificationLevel() {
		t.Errorf("R12 must have a notification level: rule=%s level0=%t", iam.Rule(), iam.HasNotificationLevel())
	}
}

// abtYes and abtNo state a definite eligibility. Tests that mean "nobody told
// us" leave the field nil, which is its own case.
func abtYes() *bool { yes := true; return &yes }
func abtNo() *bool  { no := false; return &no }

// abtFor turns a table-driven bool into the definite value the routing
// context now carries.
func abtFor(eligible bool) *bool { return &eligible }

// Nobody has told us whether the account is ABT-eligible — the situation in
// production today, since no publisher populates the field. That is a third
// state, not a "no": eligibility splits section 5.0's table in half, so the
// row genuinely cannot be named, and saying "R12" would read as a decision
// that was made rather than one that was missed.
func TestRule_UnknownEligibilityIsNotAnAnswer(t *testing.T) {
	rc := RoutingContext{Product: "WSO2 API Manager", AssignedCRETeam: "Atlas", Shift: ShiftLK}
	if got := rc.Rule(); got != "UNKNOWN_ABT" {
		t.Errorf("Rule() = %s, want UNKNOWN_ABT when nobody supplied eligibility", got)
	}
	if got := rc.ABTEligibility(); got != "unknown" {
		t.Errorf("ABTEligibility() = %q, want unknown", got)
	}

	// An explicit no is a different thing and still names its row.
	rc.ABTEligible = abtNo()
	if got := rc.Rule(); got != "R5" {
		t.Errorf("Rule() = %s, want R5 for an explicit sub-team answer", got)
	}
}

// With eligibility unknown on a USA_WEEKEND rotation, the notification level
// is included. The two errors are not symmetric: including it wrongly wakes
// one rotation member who was not on the hook and costs the ladder nothing,
// while omitting it wrongly removes the first and fastest rung from an
// unattended incident on a weekend night.
func TestHasNotificationLevel_UnknownEligibilityKeepsLevel0(t *testing.T) {
	unknown := RoutingContext{Product: "WSO2 API Manager", Shift: ShiftUSAWeekend}
	if !unknown.HasNotificationLevel() {
		t.Error("LEVEL_0 was dropped on an unclassified weekend-night incident")
	}
	// A definite yes is what removes it, per R10.
	unknown.ABTEligible = abtYes()
	if unknown.HasNotificationLevel() {
		t.Error("R10 defines no notification level; a definite yes must drop it")
	}
}

// The presence-aware field has to survive the wire, or the distinction it
// exists for is lost the moment the event is published.
func TestABTEligibility_SurvivesJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"omitted", `{}`, "unknown"},
		{"explicit true", `{"abtEligible":true}`, "yes"},
		{"explicit false", `{"abtEligible":false}`, "no"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p events.IncidentCreatedPayload
			if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
				t.Fatal(err)
			}
			rc := RoutingContext{ABTEligible: p.ABTEligible}
			if got := rc.ABTEligibility(); got != tc.want {
				t.Errorf("%s decoded to %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
