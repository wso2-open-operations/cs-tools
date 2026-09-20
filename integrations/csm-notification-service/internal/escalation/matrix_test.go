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
	"fmt"
	"strings"
	"testing"
	"time"
)

// Every path through the specification, checked for shape.
//
// The other tests pick a rule and a priority and look closely at one thing.
// This one walks all fourteen rows of section 5.0 against all five rows of
// section 7.0 — seventy ladders — and asserts what the document says about
// each: which rungs it climbs, how many calls that is, when each rung opens,
// and when it reaches the Head of CRE. It is the evidence that "each
// escalation path works", one case per path, rather than a sample.
//
// Recipients are the fixed test roster, one person per rung, so the counts
// below are counts of attempts. A real resolver returning several people at a
// rung multiplies the calls at that rung, and nothing else.

// routingFor is every rule's inputs, as the table states them.
//
// The sub-team rows state eligibility explicitly rather than leaning on a zero
// value: "ABT: No" is an answer the table gives, and since the field became
// presence-aware, leaving it unset would mean "nobody said" — which routes to
// no row at all. The no-product rows (R7, R8, R13, R14) genuinely do not
// consult it, and leave it unset for that reason.
var routingFor = map[string]RoutingContext{
	"R1":  {Product: "WSO2 API Manager", ABTEligible: abtYes(), AssignedCRETeam: "Atlas", Shift: ShiftLK},
	"R2":  {Product: "WSO2 API Manager", ABTEligible: abtYes(), AssignedCRETeam: "Atlas", Shift: ShiftLKMorning},
	"R3":  {Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: ShiftLK},
	"R4":  {Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: ShiftLKEvening},
	"R5":  {Product: "WSO2 Identity Server", ABTEligible: abtNo(), Shift: ShiftLK},
	"R6":  {Product: "WSO2 Identity Server", ABTEligible: abtNo(), Shift: ShiftLKWeekend},
	"R7":  {Shift: ShiftLK},
	"R8":  {Shift: ShiftLKMorning},
	"R9":  {Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: ShiftUSA},
	"R10": {Product: "WSO2 API Manager", ABTEligible: abtYes(), Shift: ShiftUSAWeekend},
	"R11": {Product: "WSO2 Identity Server", ABTEligible: abtNo(), Shift: ShiftUSA},
	"R12": {Product: "WSO2 Identity Server", ABTEligible: abtNo(), Shift: ShiftUSAWeekend},
	"R13": {Shift: ShiftUSA},
	"R14": {Shift: ShiftUSAWeekend},
}

// hasLevel0 is which rows the table gives a notification level.
var hasLevel0 = map[string]bool{
	"R2": true, "R4": true, "R6": true, "R8": true, "R12": true, "R14": true,
}

func TestMatrix_EveryRuleTimesEveryPriority(t *testing.T) {
	priorities := []string{"P0", "P1", "P2", "P3", "P4"}
	rules := make([]string, 0, len(routingFor))
	for i := 1; i <= 14; i++ {
		rules = append(rules, fmt.Sprintf("R%d", i))
	}

	var report []string
	for _, rule := range rules {
		for _, priority := range priorities {
			name := rule + "/" + priority
			t.Run(name, func(t *testing.T) {
				rc := routingFor[rule]
				tr := testTrigger(priority, rc.Shift)
				tr.Routing = rc

				if got := rc.Rule(); got != rule {
					t.Fatalf("routing reports %s, want %s", got, rule)
				}
				policy, ok := Lookup(DefaultPolicy, priority)
				if !ok {
					t.Fatalf("no policy for %s", priority)
				}
				plan, err := BuildPlan(context.Background(), tr, DefaultPolicy, fullResolver())
				if err != nil {
					t.Fatal(err)
				}

				// 1. Which rungs, in order.
				wantLevels := []string{"LEVEL_1", "LEVEL_2", "LEVEL_3", "LEVEL_4"}
				if hasLevel0[rule] {
					wantLevels = append([]string{"LEVEL_0"}, wantLevels...)
				}
				if got := plan.LevelsClimbed(); strings.Join(got, ",") != strings.Join(wantLevels, ",") {
					t.Errorf("climbs %v, want %v", got, wantLevels)
				}

				// 2. How many calls: the sum of notification counts over the
				// rungs actually climbed.
				wantCalls := 0
				first := Level1
				if hasLevel0[rule] {
					first = Level0
				}
				for l := first; l <= Level4; l++ {
					wantCalls += policy.Levels[l].NotificationCount
				}
				if len(plan.Calls) != wantCalls {
					t.Errorf("%d calls, want %d", len(plan.Calls), wantCalls)
				}

				// 3. The ladder opens after the priority's initial wait.
				if !plan.Calls[0].At.Equal(tr.At.Add(policy.InitialWait)) {
					t.Errorf("first call at T+%v, want T+%v (initial wait)",
						plan.Calls[0].At.Sub(tr.At), policy.InitialWait)
				}

				// 4. Each rung opens when the previous one's duration ends —
				// the document's (count × interval) + escalation formula.
				opens := policy.InitialWait
				for l := first; l <= Level4; l++ {
					got, found := firstCallAt(plan, l)
					if !found {
						t.Errorf("%s never opens", l)
						continue
					}
					if got != opens {
						t.Errorf("%s opens at T+%v, want T+%v", l, got, opens)
					}
					// Attempts within the rung are one interval apart.
					lp := policy.Levels[l]
					for n := 1; n <= lp.NotificationCount; n++ {
						want := opens + time.Duration(n-1)*lp.NotificationInterval
						if at, ok := attemptAt(plan, l, n); !ok || at != want {
							t.Errorf("%s attempt %d at T+%v, want T+%v", l, n, at, want)
						}
					}
					opens += lp.Duration()
				}

				// 5. And the whole thing reaches the final rung when section
				// 7.0 says it does.
				if got, _ := firstCallAt(plan, Level4); got != TimeToFinalLevel(policy, hasLevel0[rule]) {
					t.Errorf("LEVEL_4 at T+%v, want T+%v", got, TimeToFinalLevel(policy, hasLevel0[rule]))
				}

				if !t.Failed() {
					last := plan.Calls[len(plan.Calls)-1].At.Sub(tr.At)
					report = append(report, fmt.Sprintf("%-4s %-3s %-38s %2d calls  LEVEL_4 at %-7v last call %v",
						rule, priority, strings.Join(plan.LevelsClimbed(), ","), len(plan.Calls),
						TimeToFinalLevel(policy, hasLevel0[rule]), last))
				}
			})
		}
	}
	if testing.Verbose() {
		t.Logf("\n%s", strings.Join(report, "\n"))
	}
}

func firstCallAt(p Plan, l Level) (time.Duration, bool) {
	for _, c := range p.Calls {
		if c.Level == l {
			return c.At.Sub(p.Trigger.At), true
		}
	}
	return 0, false
}

func attemptAt(p Plan, l Level, ordinal int) (time.Duration, bool) {
	for _, c := range p.Calls {
		if c.Level == l && c.Ordinal == ordinal {
			return c.At.Sub(p.Trigger.At), true
		}
	}
	return 0, false
}
