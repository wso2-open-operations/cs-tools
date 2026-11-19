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
	for _, s := range []string{"8-5 weekdays", "wso2 business days", " ", "CS 12x5 IST"} {
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
	if len(names) != 1 || names[0] != WSO2BusinessDays {
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
