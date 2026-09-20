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
	"testing"
	"time"
)

// Each level's duration must equal (NC * NI) + TN — the formula section 7.0
// states and every row except one reproduces.
func TestLevelDurationsMatchTheSpecTable(t *testing.T) {
	want := map[string][5]int{ // minutes per level, from the document
		"P0": {1, 3, 4, 4, 4},
		"P1": {3, 9, 10, 10, 10},
		"P2": {6, 15, 15, 15, 15},
		"P3": {8, 20, 25, 25, 25},
		"P4": {8, 30, 30, 30, 30}, // document states 15 for LEVEL_0 — see below
	}
	for priority, mins := range want {
		p, ok := Lookup(DefaultPolicy, priority)
		if !ok {
			t.Fatalf("%s: missing from DefaultPolicy", priority)
		}
		for level := Level0; level <= Level4; level++ {
			got := p.Levels[level].Duration()
			if got != time.Duration(mins[level])*time.Minute {
				t.Errorf("%s %s: duration = %v, want %dm", priority, level, got, mins[level])
			}
		}
	}
}

// "Time to Reach Final Escalation Level" = initial wait + levels 0..3.
// This reproduces the document exactly for P0, P2 and P3. P1 and P4 are the
// two known inconsistencies and are asserted at their computed values, so a
// future correction to the source is a deliberate, visible change here.
func TestTimeToFinalLevel(t *testing.T) {
	tests := []struct {
		priority   string
		wantMins   int
		documented int
	}{
		{"P0", 12, 12},
		{"P1", 38, 40}, // document says 40; its own rows sum to 38
		{"P2", 60, 60},
		{"P3", 90, 90},
		{"P4", 113, 120}, // document's 120 only reconciles if LEVEL_0 is 15m, not 8m
	}
	for _, tc := range tests {
		t.Run(tc.priority, func(t *testing.T) {
			p, _ := Lookup(DefaultPolicy, tc.priority)
			got := TimeToFinalLevel(p, true)
			if got != time.Duration(tc.wantMins)*time.Minute {
				t.Fatalf("%s: got %v, want %dm", tc.priority, got, tc.wantMins)
			}
			if tc.wantMins != tc.documented {
				t.Logf("KNOWN SPEC DIVERGENCE %s: computed %dm, document states %dm",
					tc.priority, tc.wantMins, tc.documented)
			}
		})
	}
}

// A P0 is the tightest ladder: it must place its first call immediately, since
// its initial wait is 0% of a 15-minute SLA.
func TestP0StartsImmediately(t *testing.T) {
	p, _ := Lookup(DefaultPolicy, "P0")
	got := Schedule(p, true)
	if len(got) == 0 {
		t.Fatal("expected attempts")
	}
	if got[0].After != 0 {
		t.Fatalf("first P0 attempt at %v, want immediate", got[0].After)
	}
	if got[0].Level != Level0 {
		t.Fatalf("first attempt at %s, want LEVEL_0", got[0].Level)
	}
}

// Every recipient at a level is called NotificationCount times — the ladder is
// repeated calls per level, not one call per level.
func TestScheduleRepeatsCallsWithinALevel(t *testing.T) {
	p, _ := Lookup(DefaultPolicy, "P1")
	got := Schedule(p, true)

	perLevel := map[Level]int{}
	for _, a := range got {
		perLevel[a.Level]++
	}
	want := map[Level]int{Level0: 2, Level1: 3, Level2: 3, Level3: 3, Level4: 3}
	for level, n := range want {
		if perLevel[level] != n {
			t.Errorf("%s: %d attempts, want %d", level, perLevel[level], n)
		}
	}
	if len(got) != 14 {
		t.Fatalf("P1 total attempts = %d, want 14", len(got))
	}
}

// P1 concretely: initial wait 6m, LEVEL_0 two calls 1m apart, then LEVEL_1
// opens after LEVEL_0's full 3m duration.
func TestP1AttemptOffsets(t *testing.T) {
	p, _ := Lookup(DefaultPolicy, "P1")
	got := Schedule(p, true)

	wantFirst := []Attempt{
		{Level: Level0, Ordinal: 1, After: 6 * time.Minute},
		{Level: Level0, Ordinal: 2, After: 7 * time.Minute},
		{Level: Level1, Ordinal: 1, After: 9 * time.Minute},
		{Level: Level1, Ordinal: 2, After: 11 * time.Minute},
		{Level: Level1, Ordinal: 3, After: 13 * time.Minute},
	}
	for i, w := range wantFirst {
		if got[i] != w {
			t.Errorf("attempt %d = %+v, want %+v", i, got[i], w)
		}
	}
}

// Outside a rotation there is no notification level at all; the ladder must
// start at LEVEL_1 rather than idling through LEVEL_0's duration.
func TestScheduleSkipsLevel0OutsideRotations(t *testing.T) {
	p, _ := Lookup(DefaultPolicy, "P1")
	got := Schedule(p, false)

	for _, a := range got {
		if a.Level == Level0 {
			t.Fatal("LEVEL_0 must not be scheduled outside a rotation")
		}
	}
	if got[0].After != 6*time.Minute {
		t.Fatalf("first attempt at %v, want the 6m initial wait", got[0].After)
	}
	if got[0].Level != Level1 {
		t.Fatalf("first attempt at %s, want LEVEL_1", got[0].Level)
	}
}

// Attempts must come out in due order — the engine seeds wake entries from
// this slice directly.
func TestScheduleIsOrdered(t *testing.T) {
	for priority := range DefaultPolicy {
		p, _ := Lookup(DefaultPolicy, priority)
		got := Schedule(p, true)
		for i := 1; i < len(got); i++ {
			if got[i].After < got[i-1].After {
				t.Fatalf("%s: attempt %d (%v) precedes %d (%v)", priority, i, got[i].After, i-1, got[i-1].After)
			}
		}
	}
}

// Callers may pass either P-notation or the severity labels used elsewhere.
func TestLookupAcceptsSeverityLabels(t *testing.T) {
	byLabel, ok := Lookup(DefaultPolicy, "CRITICAL")
	if !ok {
		t.Fatal("CRITICAL should resolve")
	}
	byCode, _ := Lookup(DefaultPolicy, "P1")
	if byLabel.InitialWait != byCode.InitialWait {
		t.Fatal("CRITICAL must resolve to the same policy as P1")
	}
	if _, ok := Lookup(DefaultPolicy, "NOT_A_PRIORITY"); ok {
		t.Fatal("an unknown priority must not resolve")
	}
}

func TestLevelStringMatchesExecutionSummaryFormat(t *testing.T) {
	want := []string{"LEVEL_0", "LEVEL_1", "LEVEL_2", "LEVEL_3", "LEVEL_4"}
	for i, w := range want {
		if got := Level(i).String(); got != w {
			t.Errorf("Level(%d) = %q, want %q", i, got, w)
		}
	}
}
