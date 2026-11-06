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

package businessdays

import (
	"testing"
	"time"
)

func mustNY(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation("2006-01-02T15:04:05", s, location)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

// TestAddBusinessDuration_WithinOneWeekday mirrors entity-service's own
// TestElapsed_WithinOneWeekday, inverted: adding the same 8h to the same
// Monday 09:00 start must land back on 17:00 the same day.
func TestAddBusinessDuration_WithinOneWeekday(t *testing.T) {
	start := mustNY(t, "2024-01-01T09:00:00") // Monday
	want := mustNY(t, "2024-01-01T17:00:00")

	got := AddBusinessDuration(start, 8*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestAddBusinessDuration_CrossesOneWeekend mirrors
// TestElapsed_CrossingOneWeekend: 18h of business time from Friday 15:00
// must skip the whole weekend and land on Monday 09:00.
func TestAddBusinessDuration_CrossesOneWeekend(t *testing.T) {
	start := mustNY(t, "2024-01-05T15:00:00") // Friday
	want := mustNY(t, "2024-01-08T09:00:00")  // Monday

	got := AddBusinessDuration(start, 18*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestAddBusinessDuration_CrossesMultipleWeekends: 10 full weekdays (240h)
// from Monday midnight must land on the Saturday after the second business
// week -- the EARLIEST instant at which 240h of business time has elapsed,
// which is Jan 13 00:00, not Jan 15 (entity-service's own
// TestElapsed_CrossingMultipleWeekends measures the SAME 240h between Jan 1
// and Jan 15 too, since the extra weekend in between contributes zero
// business time either way -- Elapsed is many-to-one at a weekend boundary,
// AddBusinessDuration always returns the earliest matching instant, which is
// the correct "due" semantics: an SLA is due as soon as its business time is
// up, not delayed through a weekend that happens to follow).
func TestAddBusinessDuration_CrossesMultipleWeekends(t *testing.T) {
	start := mustNY(t, "2024-01-01T00:00:00") // Monday
	want := mustNY(t, "2024-01-13T00:00:00")  // Saturday, after two 5-day business weeks

	got := AddBusinessDuration(start, 10*24*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestAddBusinessDuration_DSTFallBack spans the "fall back" transition (Nov
// 3, 2024, a Sunday this calendar never counts anyway) -- a day-boundary
// calculation that drifted by the transition's extra hour would land on the
// wrong instant.
func TestAddBusinessDuration_DSTFallBack(t *testing.T) {
	start := mustNY(t, "2024-11-01T10:00:00") // Friday, EDT (UTC-4)
	want := mustNY(t, "2024-11-04T10:00:00")  // Monday, EST (UTC-5) after fall-back

	got := AddBusinessDuration(start, 24*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (DST fall-back day boundary likely drifted)", got, want)
	}
}

// TestAddBusinessDuration_DSTSpringForward is the same shape across the
// "spring forward" transition (Mar 10, 2024, a 23-hour Sunday).
func TestAddBusinessDuration_DSTSpringForward(t *testing.T) {
	start := mustNY(t, "2024-03-08T10:00:00") // Friday, EST (UTC-5)
	want := mustNY(t, "2024-03-11T10:00:00")  // Monday, EDT (UTC-4) after spring-forward

	got := AddBusinessDuration(start, 24*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (DST spring-forward day boundary likely drifted)", got, want)
	}
}

// TestAddBusinessDuration_ZeroOrNegativeReturnsStartUnchanged.
func TestAddBusinessDuration_ZeroOrNegativeReturnsStartUnchanged(t *testing.T) {
	start := mustNY(t, "2024-01-01T09:00:00")
	if got := AddBusinessDuration(start, 0); !got.Equal(start) {
		t.Fatalf("d=0: got %v, want %v unchanged", got, start)
	}
	if got := AddBusinessDuration(start, -time.Hour); !got.Equal(start) {
		t.Fatalf("d<0: got %v, want %v unchanged", got, start)
	}
}

// TestAddBusinessDuration_StartingOnAWeekendSkipsToMonday: a start instant
// that itself falls on a weekend (can happen if a case is somehow created
// then, or from a reconciliation anchored on an odd historical row) must
// still skip straight to the next business day without counting any of the
// weekend itself.
func TestAddBusinessDuration_StartingOnAWeekendSkipsToMonday(t *testing.T) {
	start := mustNY(t, "2024-01-06T12:00:00") // Saturday noon
	want := mustNY(t, "2024-01-08T01:00:00")  // Monday 01:00 -- 1h of real business time

	got := AddBusinessDuration(start, time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestAddBusinessDuration_InputInDifferentZoneIsNormalized mirrors
// TestElapsed_InputInDifferentZoneIsNormalized -- callers here pass UTC
// timestamps (entity-service's StartedOn scans into UTC time.Time), and the
// result must still be computed against America/New_York day boundaries.
func TestAddBusinessDuration_InputInDifferentZoneIsNormalized(t *testing.T) {
	// 2024-01-05 23:00 UTC is 2024-01-05 18:00 America/New_York (Friday,
	// EST = UTC-5) -- still Friday locally, 2h of same-day business time
	// lands at 20:00 NY = 2024-01-06 01:00 UTC.
	start := time.Date(2024, 1, 5, 23, 0, 0, 0, time.UTC)
	want := time.Date(2024, 1, 6, 1, 0, 0, 0, time.UTC)

	got := AddBusinessDuration(start, 2*time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("result location = %v, want the start's own UTC location preserved", got.Location())
	}
}

// TestAddBusinessDuration_IsElapsedsInverse checks the round trip against
// entity-service's own Elapsed formula directly, for a handful of spans
// that cross zero, one and several weekends -- the two functions must
// genuinely be inverses of one another, not just individually plausible.
func TestAddBusinessDuration_IsElapsedsInverse(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
		d     time.Duration
	}{
		{"same day", mustNY(t, "2024-01-02T09:00:00"), 3 * time.Hour},
		{"one weekend", mustNY(t, "2024-01-05T15:00:00"), 20 * time.Hour},
		{"two weekends", mustNY(t, "2024-01-01T00:00:00"), 9*24*time.Hour + 5*time.Hour},
		{"starts on weekend", mustNY(t, "2024-01-07T08:00:00"), 5 * time.Hour}, // Sunday
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			due := AddBusinessDuration(c.start, c.d)
			if got := businessDaysElapsed(c.start, due); got != c.d {
				t.Errorf("Elapsed(start, AddBusinessDuration(start, d)) = %v, want %v", got, c.d)
			}
		})
	}
}

// businessDaysElapsed is a local copy of entity-service's own
// businessdays.Elapsed, kept here only so this test file can check the
// round trip without importing across module boundaries (the two services
// are separate Go modules). Any drift between this copy and the real
// function would only make this one test less meaningful, never affect
// AddBusinessDuration's own production behavior.
func businessDaysElapsed(start, end time.Time) time.Duration {
	if !end.After(start) {
		return 0
	}
	start = start.In(location)
	end = end.In(location)

	var total time.Duration
	cur := start
	for cur.Before(end) {
		nextDay := startOfNextDay(cur)
		segmentEnd := nextDay
		if end.Before(segmentEnd) {
			segmentEnd = end
		}
		if isBusinessDay(cur) {
			if d := segmentEnd.Sub(cur); d > 0 {
				total += d
			}
		}
		cur = segmentEnd
	}
	return total
}
