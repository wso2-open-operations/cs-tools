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

package slacalendar

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
)

func TestForKnownAndUnknown(t *testing.T) {
	for _, s := range []string{"", "24 x 7", "WSO2 Business days"} {
		if _, known := For(s); !known {
			t.Errorf("For(%q) should be known", s)
		}
	}
	for _, s := range []string{"8-5 weekdays", "wso2 business days", " ", "cs 12x5 ist"} {
		cal, known := For(s)
		if known {
			t.Errorf("For(%q) should be unknown", s)
		}
		if cal == nil {
			t.Errorf("For(%q) must still return a flat calendar", s)
		}
	}
}

func TestFlatElapsedIsWallClock(t *testing.T) {
	start := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC) // Friday
	end := start.Add(60 * time.Hour)
	for _, s := range []string{"", "24 x 7", "unknown"} {
		cal, _ := For(s)
		if got := cal.Elapsed(start, end); got != 60*time.Hour {
			t.Errorf("%q: got %v want 60h", s, got)
		}
	}
}

func TestWSO2BusinessDaysDelegates(t *testing.T) {
	start := time.Date(2026, 3, 6, 15, 0, 0, 0, time.UTC) // Friday 10:00 ET
	end := start.Add(25 * time.Hour)
	cal, _ := For(WSO2BusinessDays)
	if got, want := cal.Elapsed(start, end), businessdays.Elapsed(start, end); got != want {
		t.Errorf("got %v want %v", got, want)
	}
	if cal.Elapsed(start, end) == end.Sub(start) {
		t.Error("business-days calendar must not equal flat across a weekend")
	}
}

func TestNonFlat(t *testing.T) {
	if _, ok := NonFlat(WSO2BusinessDays); !ok {
		t.Error("WSO2 Business days is non-flat")
	}
	for _, s := range []string{"", "24 x 7", "unknown"} {
		if _, ok := NonFlat(s); ok {
			t.Errorf("%q must not be non-flat", s)
		}
	}
	names := NonFlatSchedules()
	if len(names) != 3 || names[0] != WSO2BusinessDays || names[1] != CS12x5IST || names[2] != CS24x7 {
		t.Errorf("NonFlatSchedules = %v", names)
	}
	names[0] = "mutated"
	if NonFlatSchedules()[0] != WSO2BusinessDays {
		t.Error("NonFlatSchedules must return a copy")
	}
	for _, s := range NonFlatSchedules() {
		if _, ok := NonFlat(s); !ok {
			t.Errorf("%q listed but NonFlat says flat", s)
		}
	}
}

func TestCSCalendarsRegistered(t *testing.T) {
	for _, s := range []string{CS12x5IST, CS24x7} {
		if _, known := For(s); !known {
			t.Errorf("For(%q) should be known", s)
		}
		if _, ok := NonFlat(s); !ok {
			t.Errorf("NonFlat(%q) should be ok", s)
		}
		found := false
		for _, n := range NonFlatSchedules() {
			found = found || n == s
		}
		if !found {
			t.Errorf("NonFlatSchedules missing %q", s)
		}
	}
	if CS12x5IST != "CS 12x5 IST" || CS24x7 != "CS 24x7" {
		t.Error("schedule name constants changed")
	}
}

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.FixedZone("IST", 5*3600+30*60))
}

func TestCS12x5IST(t *testing.T) {
	cal, _ := For(CS12x5IST)
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-02 is a Monday; 2026-03-06 Friday; 03-07 Saturday.
	cases := []struct {
		name       string
		start, end time.Time
		want       time.Duration
	}{
		{"inside one window", ist(2026, 3, 3, 10, 0), ist(2026, 3, 3, 12, 30), 150 * time.Minute},
		{"whole window", ist(2026, 3, 3, 9, 0), ist(2026, 3, 3, 21, 0), 12 * time.Hour},
		{"across midnight", ist(2026, 3, 3, 20, 0), ist(2026, 3, 4, 10, 0), 2 * time.Hour},
		{"across weekend", ist(2026, 3, 6, 20, 0), ist(2026, 3, 9, 10, 0), 2 * time.Hour},
		{"saturday only", ist(2026, 3, 7, 10, 0), ist(2026, 3, 7, 18, 0), 0},
		{"sunday only", ist(2026, 3, 8, 10, 0), ist(2026, 3, 8, 18, 0), 0},
		{"weekday evening to morning", ist(2026, 3, 3, 22, 0), ist(2026, 3, 4, 8, 0), 0},
		{"before open", ist(2026, 3, 3, 5, 0), ist(2026, 3, 3, 9, 30), 30 * time.Minute},
		{"start after end", ist(2026, 3, 4, 12, 0), ist(2026, 3, 4, 10, 0), 0},
		{"zero length", ist(2026, 3, 4, 12, 0), ist(2026, 3, 4, 12, 0), 0},
		{"one week", ist(2026, 3, 2, 0, 0), ist(2026, 3, 9, 0, 0), 60 * time.Hour},
		{"three weeks", ist(2026, 3, 2, 9, 0), ist(2026, 3, 23, 9, 0), 180 * time.Hour},
	}
	for _, c := range cases {
		got := cal.Elapsed(c.start, c.end)
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
		// Same instants expressed in other zones give the same answer.
		for _, loc := range []*time.Location{time.UTC, ny} {
			if g := cal.Elapsed(c.start.In(loc), c.end.In(loc)); g != c.want {
				t.Errorf("%s in %s: got %v want %v", c.name, loc, g, c.want)
			}
		}
	}
}

func TestCS12x5ISTMultiWeekSpan(t *testing.T) {
	cal, _ := For(CS12x5IST)
	// Thu 2026-03-05 15:00 IST to Thu 2026-03-19 11:00 IST:
	// Thu 6h + Fri 12h + (Mon-Fri) 60h + (Mon-Wed) 36h + Thu 2h = 116h.
	got := cal.Elapsed(ist(2026, 3, 5, 15, 0), ist(2026, 3, 19, 11, 0))
	if want := 116 * time.Hour; got != want {
		t.Errorf("got %v want %v", got, want)
	}
	// An instant of another zone that is the same IST moment.
	utc := ist(2026, 3, 5, 15, 0).UTC()
	if got2 := cal.Elapsed(utc, ist(2026, 3, 19, 11, 0).UTC()); got2 != got {
		t.Errorf("UTC inputs: got %v want %v", got2, got)
	}
}

func TestCS24x7(t *testing.T) {
	cal, _ := For(CS24x7)
	start := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	if got := cal.Elapsed(start, start.Add(60*time.Hour)); got != 60*time.Hour {
		t.Errorf("got %v want 60h", got)
	}
	if got := cal.Elapsed(start, start.Add(-time.Hour)); got != 0 {
		t.Errorf("start after end: got %v want 0", got)
	}
	if got := cal.Elapsed(start, start); got != 0 {
		t.Errorf("zero length: got %v want 0", got)
	}
}
