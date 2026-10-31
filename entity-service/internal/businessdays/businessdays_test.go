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

// TestElapsed_WithinOneWeekday: a span that never leaves a single Monday.
func TestElapsed_WithinOneWeekday(t *testing.T) {
	start := mustNY(t, "2024-01-01T09:00:00") // Monday
	end := mustNY(t, "2024-01-01T17:00:00")

	got := Elapsed(start, end)
	want := 8 * time.Hour
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestElapsed_CrossingOneWeekend: Friday afternoon to Monday morning --
// Saturday/Sunday must contribute nothing.
func TestElapsed_CrossingOneWeekend(t *testing.T) {
	start := mustNY(t, "2024-01-05T15:00:00") // Friday
	end := mustNY(t, "2024-01-08T09:00:00")   // Monday

	got := Elapsed(start, end)
	// Friday 15:00 -> midnight = 9h, weekend = 0, midnight -> 09:00 Monday = 9h.
	want := 18 * time.Hour
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestElapsed_CrossingMultipleWeekends: exactly two full business weeks,
// aligned on midnight boundaries so the expected total is exact.
func TestElapsed_CrossingMultipleWeekends(t *testing.T) {
	start := mustNY(t, "2024-01-01T00:00:00") // Monday
	end := mustNY(t, "2024-01-15T00:00:00")   // Monday, two weeks later

	got := Elapsed(start, end)
	want := 10 * 24 * time.Hour // 10 weekdays (Jan 1-5, Jan 8-12), two weekends excluded
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestElapsed_DSTFallBack: spans the "fall back" transition (Nov 3, 2024,
// a Sunday -- the US transition always lands on a Sunday, which this
// calendar never counts anyway). A day-boundary calculation that drifts by
// the transition's extra hour (e.g. by advancing with Add(24*time.Hour)
// instead of real calendar arithmetic) would land on the wrong instant for
// Monday's own midnight and mismeasure the Monday segment by exactly one
// hour -- this pins the total to the one correct value.
func TestElapsed_DSTFallBack(t *testing.T) {
	start := mustNY(t, "2024-11-01T10:00:00") // Friday, EDT (UTC-4)
	end := mustNY(t, "2024-11-04T10:00:00")   // Monday, EST (UTC-5) after fall-back

	got := Elapsed(start, end)
	// Friday 10:00 -> midnight = 14h, weekend (incl. the 25-hour Sunday) = 0,
	// midnight -> Monday 10:00 = 10h.
	want := 24 * time.Hour
	if got != want {
		t.Fatalf("got %v, want %v (DST fall-back day boundary likely drifted)", got, want)
	}
}

// TestElapsed_DSTSpringForward: same shape, across the "spring forward"
// transition (Mar 10, 2024, a 23-hour Sunday).
func TestElapsed_DSTSpringForward(t *testing.T) {
	start := mustNY(t, "2024-03-08T10:00:00") // Friday, EST (UTC-5)
	end := mustNY(t, "2024-03-11T10:00:00")   // Monday, EDT (UTC-4) after spring-forward

	got := Elapsed(start, end)
	want := 24 * time.Hour
	if got != want {
		t.Fatalf("got %v, want %v (DST spring-forward day boundary likely drifted)", got, want)
	}
}

func TestElapsed_EndBeforeOrEqualStartIsZero(t *testing.T) {
	start := mustNY(t, "2024-01-01T09:00:00")
	end := mustNY(t, "2024-01-01T08:00:00")
	if got := Elapsed(start, end); got != 0 {
		t.Fatalf("expected 0 for end before start, got %v", got)
	}
	if got := Elapsed(start, start); got != 0 {
		t.Fatalf("expected 0 for end == start, got %v", got)
	}
}

// TestElapsed_PureWeekendIsZero: a span entirely inside Saturday/Sunday.
func TestElapsed_PureWeekendIsZero(t *testing.T) {
	start := mustNY(t, "2024-01-06T00:00:00") // Saturday
	end := mustNY(t, "2024-01-08T00:00:00")   // Monday midnight (exclusive)
	if got := Elapsed(start, end); got != 0 {
		t.Fatalf("expected a full weekend to contribute 0, got %v", got)
	}
}

// TestElapsed_InputInDifferentZoneIsNormalized makes sure callers passing
// UTC timestamps (as every caller in this codebase does -- Postgres
// TIMESTAMPTZ scans into UTC time.Time) still get the right America/
// New_York day boundaries.
func TestElapsed_InputInDifferentZoneIsNormalized(t *testing.T) {
	// 2024-01-05 23:00 UTC is 2024-01-05 18:00 America/New_York (Friday,
	// EST = UTC-5) -- still Friday locally, must count.
	start := time.Date(2024, 1, 5, 23, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 6, 1, 0, 0, 0, time.UTC) // 2024-01-05 20:00 NY, still Friday

	got := Elapsed(start, end)
	want := 2 * time.Hour
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}
