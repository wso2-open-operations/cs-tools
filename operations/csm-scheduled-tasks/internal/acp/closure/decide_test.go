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

package closure

import (
	"testing"
	"time"
)

// TestNoticeWindow_IsTerminal covers the single canonical "is this the
// terminal/day-0 window" predicate — added so callers needing this check
// (e.g. sweep.go's subject/body builders) share one definition instead of
// each independently comparing against NoticeWindow0 (Sajith Ekanayake:
// that re-derivation had already gone wrong once).
func TestNoticeWindow_IsTerminal(t *testing.T) {
	tests := []struct {
		window NoticeWindow
		want   bool
	}{
		{NoticeWindow90, false},
		{NoticeWindow60, false},
		{NoticeWindow30, false},
		{NoticeWindow15, false},
		{NoticeWindow7, false},
		{NoticeWindow0, true},
	}
	for _, tt := range tests {
		if got := tt.window.IsTerminal(); got != tt.want {
			t.Errorf("NoticeWindow(%d).IsTerminal() = %v, want %v", tt.window, got, tt.want)
		}
	}
}

func TestDecide_FiresCorrectWindowWhenNoPriorNotice(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		endDate    time.Time
		wantWindow NoticeWindow
		wantFires  bool
	}{
		{
			name:       "89 days remaining fires the 90-day window",
			endDate:    now.AddDate(0, 0, 89),
			wantWindow: NoticeWindow90,
			wantFires:  true,
		},
		{
			name:       "60 days remaining fires the 60-day window",
			endDate:    now.AddDate(0, 0, 60),
			wantWindow: NoticeWindow60,
			wantFires:  true,
		},
		{
			name:       "91 days remaining fires nothing",
			endDate:    now.AddDate(0, 0, 91),
			wantWindow: 0,
			wantFires:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(now, tt.endDate, nil)

			if got.Fires != tt.wantFires {
				t.Errorf("Fires = %v, want %v", got.Fires, tt.wantFires)
			}
			if tt.wantFires && got.Window != tt.wantWindow {
				t.Errorf("Window = %v, want %v", got.Window, tt.wantWindow)
			}
		})
	}
}

// TestDecide_MissedRunCatchesUpToNearestWindow covers the scenario the
// cascading (<=) design exists for: the component didn't run on the day a
// threshold was first crossed (e.g. it was down, or this is the very first
// run for a long-lived project). It must still fire the nearest window that
// daysRemaining has now reached — not skip ahead silently, and not re-fire a
// window already recorded in lastNoticeWindow.
func TestDecide_MissedRunCatchesUpToNearestWindow(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)

	w90 := NoticeWindow90

	tests := []struct {
		name             string
		daysRemaining    int
		lastNoticeWindow *NoticeWindow
		wantWindow       NoticeWindow
		wantFires        bool
	}{
		{
			name:             "never notified, jumped straight to 45 days remaining: catches up to 60, not 90",
			daysRemaining:    45,
			lastNoticeWindow: nil,
			wantWindow:       NoticeWindow60,
			wantFires:        true,
		},
		{
			name:             "already notified at 90, jumped to 10 days remaining: catches up to 15, not re-firing 90",
			daysRemaining:    10,
			lastNoticeWindow: &w90,
			wantWindow:       NoticeWindow15,
			wantFires:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endDate := now.AddDate(0, 0, tt.daysRemaining)

			got := Decide(now, endDate, tt.lastNoticeWindow)

			if got.Fires != tt.wantFires {
				t.Errorf("Fires = %v, want %v", got.Fires, tt.wantFires)
			}
			if tt.wantFires && got.Window != tt.wantWindow {
				t.Errorf("Window = %v, want %v", got.Window, tt.wantWindow)
			}
		})
	}
}

// TestDecide_Day0ReportsNotifyAndSuspendIndependently covers the day-0
// contract: Decide() only ever reports *what's* due, never sequencing.
// ShouldSuspend has no idempotency signal of its own in this package (the
// real signal, closureStatus, lives on the entity-service Project and isn't
// a parameter here) so it must report true on every day-0 evaluation,
// regardless of lastNoticeWindow — the caller is responsible for checking
// closureStatus before actually invoking suspend. ShouldNotify, by contrast,
// is gated by lastNoticeWindow reaching the terminal NoticeWindow0 value.
func TestDecide_Day0ReportsNotifyAndSuspendIndependently(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due

	w7 := NoticeWindow7
	w0 := NoticeWindow0

	tests := []struct {
		name              string
		lastNoticeWindow  *NoticeWindow
		wantShouldNotify  bool
		wantShouldSuspend bool
	}{
		{
			name:              "day-0 email not yet sent: notify and suspend both due",
			lastNoticeWindow:  &w7,
			wantShouldNotify:  true,
			wantShouldSuspend: true,
		},
		{
			name:              "day-0 email already sent (terminal marker recorded): skip notify, suspend still due",
			lastNoticeWindow:  &w0,
			wantShouldNotify:  false,
			wantShouldSuspend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(now, endDate, tt.lastNoticeWindow)

			if got.ShouldNotify != tt.wantShouldNotify {
				t.Errorf("ShouldNotify = %v, want %v", got.ShouldNotify, tt.wantShouldNotify)
			}
			if got.ShouldSuspend != tt.wantShouldSuspend {
				t.Errorf("ShouldSuspend = %v, want %v", got.ShouldSuspend, tt.wantShouldSuspend)
			}
			if got.Window != NoticeWindow0 {
				t.Errorf("Window = %v, want %v", got.Window, NoticeWindow0)
			}
		})
	}
}
