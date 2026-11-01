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

package repository

import (
	"testing"
	"time"
)

// TestBusinessDayElapsedSnapshot_WithinOneWeekday covers the plain case: no
// pause, a span entirely inside one business day.
func TestBusinessDayElapsedSnapshot_WithinOneWeekday(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 1, 1, 9, 0, 0, 0, loc) // Monday
	at := time.Date(2024, 1, 1, 13, 0, 0, 0, loc)   // 4h later
	duration := 8 * time.Hour

	elapsed, remaining, pct, breached := businessDayElapsedSnapshot(start, at, 0, duration)
	if elapsed != 4*time.Hour {
		t.Errorf("elapsed = %v, want 4h", elapsed)
	}
	if remaining != 4*time.Hour {
		t.Errorf("remaining = %v, want 4h", remaining)
	}
	if pct != 50 {
		t.Errorf("pct = %v, want 50", pct)
	}
	if breached {
		t.Error("breached = true, want false")
	}
}

// TestBusinessDayElapsedSnapshot_SubtractsCompletedBusinessPause confirms a
// completed pause segment (already folded into businessPauseSoFar by the
// caller) is subtracted from the raw calendar elapsed time.
func TestBusinessDayElapsedSnapshot_SubtractsCompletedBusinessPause(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, loc) // Monday midnight
	at := time.Date(2024, 1, 2, 0, 0, 0, 0, loc)    // Tuesday midnight -- 24h of business time
	businessPause := 10 * time.Hour
	duration := 48 * time.Hour

	elapsed, _, _, _ := businessDayElapsedSnapshot(start, at, businessPause, duration)
	if elapsed != 14*time.Hour {
		t.Errorf("elapsed = %v, want 14h (24h calendar - 10h paused)", elapsed)
	}
}

// TestBusinessDayElapsedSnapshot_FloorsAtZero confirms a pause total that
// exceeds the raw calendar elapsed time never produces a negative elapsed.
func TestBusinessDayElapsedSnapshot_FloorsAtZero(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 1, 1, 9, 0, 0, 0, loc)
	at := time.Date(2024, 1, 1, 10, 0, 0, 0, loc)
	elapsed, remaining, pct, breached := businessDayElapsedSnapshot(start, at, 5*time.Hour, 10*time.Hour)
	if elapsed != 0 {
		t.Errorf("elapsed = %v, want 0", elapsed)
	}
	if remaining != 10*time.Hour {
		t.Errorf("remaining = %v, want the full duration", remaining)
	}
	if pct != 0 {
		t.Errorf("pct = %v, want 0", pct)
	}
	if breached {
		t.Error("breached = true, want false")
	}
}

// TestBusinessDayElapsedSnapshot_Breached confirms elapsed >= duration marks
// the clock breached and remaining floors at zero rather than going negative.
func TestBusinessDayElapsedSnapshot_Breached(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, loc)
	at := time.Date(2024, 1, 3, 0, 0, 0, 0, loc) // 48h calendar later
	duration := 24 * time.Hour

	elapsed, remaining, pct, breached := businessDayElapsedSnapshot(start, at, 0, duration)
	if !breached {
		t.Error("breached = false, want true")
	}
	if remaining != 0 {
		t.Errorf("remaining = %v, want 0", remaining)
	}
	if elapsed != 48*time.Hour {
		t.Errorf("elapsed = %v, want 48h (uncapped past 100%%)", elapsed)
	}
	if pct != 200 {
		t.Errorf("pct = %v, want 200 (uncapped)", pct)
	}
}

// TestBusinessDayElapsedSnapshot_SkipsWeekend confirms the underlying
// calendar (internal/businessdays) is genuinely consulted, not a flat
// wall-clock diff -- a span crossing a weekend elapses less than its
// absolute duration.
func TestBusinessDayElapsedSnapshot_SkipsWeekend(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	start := time.Date(2024, 1, 5, 0, 0, 0, 0, loc) // Friday midnight
	at := time.Date(2024, 1, 8, 0, 0, 0, 0, loc)    // Monday midnight (72h calendar later)
	duration := 24 * time.Hour

	elapsed, _, _, _ := businessDayElapsedSnapshot(start, at, 0, duration)
	if elapsed != 24*time.Hour {
		t.Errorf("elapsed = %v, want 24h (Friday only, weekend excluded)", elapsed)
	}
}
