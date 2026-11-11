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

package chataudience

import (
	"slices"
	"testing"
	"time"
)

// TestResolve is a table-driven unit test for the pure resolution function —
// the Americas/weekend clock rules can't be exercised deterministically
// through a full Handle() call (which reads the real wall clock), so
// they're tested directly here instead.
func TestResolve(t *testing.T) {
	configuredTeam := func(a string) bool { return a == "Castor" }
	noneConfigured := func(a string) bool { return false }
	configuredTeamAndOnboarding := func(a string) bool { return a == "Castor" || a == Onboarding }

	// A Monday at noon IST — outside both the Americas overnight window
	// and the weekend window, so it isolates whichever rule each subtest
	// is actually checking.
	weekdayNoon := time.Date(2026, time.January, 5, 6, 30, 0, 0, time.UTC) // 12:00 IST

	testCases := []struct {
		name             string
		team             string
		isEvaluation     bool
		onboardingStatus string
		now              time.Time
		hasAudience      func(string) bool
		want             []string
	}{
		{"recognized team", "Castor", false, "", weekdayNoon, configuredTeam, []string{"Castor"}},
		{"unrecognized team falls back", "Polaris", false, "", weekdayNoon, noneConfigured, []string{IncidentMonitor}},
		{"no team falls back", "", false, "", weekdayNoon, noneConfigured, []string{IncidentMonitor}},
		{"evaluation overrides team and onboarding", "Castor", true, "IN_PROGRESS", weekdayNoon, configuredTeamAndOnboarding, []string{Evaluation}},
		{"onboarding IN_PROGRESS routes to Onboarding alone", "Castor", false, "IN_PROGRESS", weekdayNoon, configuredTeamAndOnboarding, []string{Onboarding}},
		{"onboarding NOT_STARTED routes to Onboarding alone", "Castor", false, "NOT_STARTED", weekdayNoon, configuredTeamAndOnboarding, []string{Onboarding}},
		{"onboarding COMPLETED does not route to Onboarding", "Castor", false, "COMPLETED", weekdayNoon, configuredTeamAndOnboarding, []string{"Castor"}},
		{"onboarding IN_PROGRESS but no Onboarding space configured falls back to team", "Castor", false, "IN_PROGRESS", weekdayNoon, configuredTeam, []string{"Castor"}},
		{
			"9pm IST is inside the Americas window", "Castor", false, "",
			time.Date(2026, time.January, 5, 15, 30, 0, 0, time.UTC), // 21:00 IST
			configuredTeam, []string{"Castor", Americas},
		},
		{
			// Wednesday, not Monday: a pre-6am Monday instant also falls
			// inside the weekend tail (see the deliberate-overlap case
			// below) — this one isolates the Americas window alone.
			"5:59am IST on a weekday is inside the Americas window", "Castor", false, "",
			time.Date(2026, time.January, 7, 0, 29, 0, 0, time.UTC), // Wed 05:59 IST
			configuredTeam, []string{"Castor", Americas},
		},
		{
			"6:00am IST is outside the Americas window", "Castor", false, "",
			time.Date(2026, time.January, 5, 0, 30, 0, 0, time.UTC), // 06:00 IST
			configuredTeam, []string{"Castor"},
		},
		{
			"Saturday 6am IST starts the weekend window", "Castor", false, "",
			time.Date(2026, time.January, 3, 0, 30, 0, 0, time.UTC), // Sat 06:00 IST
			configuredTeam, []string{"Castor", IncidentMonitor},
		},
		{
			// Any pre-6am instant also falls inside the Americas overnight
			// window (it covers all of 00:00-06:00 IST as part of its own
			// wraparound), so "not yet weekend" is isolated with a Friday
			// evening instant instead, clear of both windows entirely.
			"Friday evening is before both the Americas and weekend windows", "Castor", false, "",
			time.Date(2026, time.January, 2, 14, 30, 0, 0, time.UTC), // Fri 20:00 IST
			configuredTeam, []string{"Castor"},
		},
		{
			"Sunday is inside the weekend window", "Castor", false, "",
			time.Date(2026, time.January, 4, 6, 30, 0, 0, time.UTC), // Sun 12:00 IST
			configuredTeam, []string{"Castor", IncidentMonitor},
		},
		{
			// Deliberately exercises the genuine overlap between the two
			// windows: a pre-6am Monday instant is simultaneously still
			// inside the weekend tail AND inside the Americas overnight
			// window (00:00-06:00 IST is common to both) — both audiences
			// are correctly added, deduped against the Incident Monitor
			// fallback (no configured team-of-its-own webhook contention
			// here since Castor is configured).
			"Monday before 6am IST overlaps the weekend and Americas windows", "Castor", false, "",
			time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC), // Mon 05:30 IST
			configuredTeam, []string{"Castor", Americas, IncidentMonitor},
		},
		{
			"Monday 6am IST ends the weekend window", "Castor", false, "",
			time.Date(2026, time.January, 5, 0, 30, 0, 0, time.UTC), // Mon 06:00 IST
			configuredTeam, []string{"Castor"},
		},
		{
			"no team during the weekend window dedupes to one Incident Monitor entry", "", false, "",
			time.Date(2026, time.January, 4, 6, 30, 0, 0, time.UTC), // Sun 12:00 IST
			noneConfigured, []string{IncidentMonitor},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(tc.team, tc.isEvaluation, tc.onboardingStatus, tc.now, tc.hasAudience)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Resolve(...) = %v, want %v", got, tc.want)
			}
		})
	}
}
