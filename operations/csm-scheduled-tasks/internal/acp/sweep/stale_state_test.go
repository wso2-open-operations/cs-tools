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

package sweep

import (
	"context"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
	"testing"
)

// staleNow is the evaluation instant of these tests (midnight UTC, so a date
// n days out has daysRemaining == n).
var staleNow = renewalNow

// invoiceDueReader gives the project one unpaid invoice (EULA 3.4, no primary
// partner, so the suspend date is the due date) due n days from staleNow.
func invoiceDueReader(t *testing.T, n int) *mockEntityReader {
	t.Helper()
	// invoiceLinkedReader sets the due date to its argument minus one day.
	return invoiceLinkedReader(t, staleNow.AddDate(0, 0, n+1))
}

// TestProcessProject_DateMovedOutPastRecordedWindow_EndDate: the end-date
// track's saved window is reset when the date has moved out past it, whatever
// the distance (legacy recomputed the event from the date on every run), and
// left alone otherwise.
func TestProcessProject_DateMovedOutPastRecordedWindow_EndDate(t *testing.T) {
	tests := []struct {
		name      string
		recorded  string
		days      int
		endState  string
		wantReset bool
	}{
		{"7-day notice, extended to +45", "7_days_notice", 45, "Open", true},
		{"7-day notice, extended to +60", "7_days_notice", 60, "Open", true},
		{"7-day notice, extended one day to +8", "7_days_notice", 8, "Open", true},
		{"90-day notice, extended to +120", "90_days_notice", 120, "Open", true},
		{"30-day notice, extended to +60", "30_days_notice", 60, "Open", true},
		{"suspend record (hand-reopened), date moved to +3", "suspend", 3, "Open", true},
		{"suspend_notice record, date moved to +3", "suspend_notice", 3, "Open", true},
		{"far renewal (> 90 days) still resets", "30_days_notice", 390, "Open", true},
		{"equal to the recorded window is not reset", "7_days_notice", 7, "Open", false},
		{"below the recorded window is not reset", "7_days_notice", 3, "Open", false},
		{"30-day notice, at +30 is not reset", "30_days_notice", 30, "Open", false},
		{"90-day notice, at +90 is not reset", "90_days_notice", 90, "Open", false},
		{"suspend record, date still past: not reset", "suspend", -5, "Open", false},
		{"suspend record, date today (day 0): not reset", "suspend", 0, "Open", false},
		{"track not Open (Closed by ACP): untouched", "7_days_notice", 45, "Closed", false},
		{"track not Open (Pending Suspended): untouched", "7_days_notice", 45, "Pending Suspended", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := renewalProject(tt.days, tt.recorded, "open")
			proj.EndDateClosureState = strPtr(tt.endState)
			updater := &mockProjectUpdater{}
			ntf := &mockNotifier{}
			// A decision that notifies or suspends may also act (a hand-reopened
			// project with the date in the past is covered by other tests); here
			// only the reset write is judged.
			if _, err := processProject(context.Background(), &mockEntityReader{}, updater, ntf, staleNow, proj); err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			got := savedEventTypes(t, updater.calls)["based_on_subscription_end_date"] == "open"
			if tt.recorded == "" {
				got = false
			}
			if got != tt.wantReset {
				t.Errorf("reset = %v, want %v; calls: %+v", got, tt.wantReset, updater.calls)
			}
			if !tt.wantReset && tt.days >= 0 && len(updater.calls) != 0 {
				t.Errorf("update calls = %d, want 0 (no churn): %+v", len(updater.calls), updater.calls)
			}
			if tt.wantReset && len(updater.calls) != 1 {
				t.Errorf("update calls = %d, want exactly 1", len(updater.calls))
			}
			if wroteField(updater.calls, "endDateClosureState") || len(updater.sfSuspendCalls) != 0 {
				t.Error("a reset must not touch the closure state or Salesforce")
			}
		})
	}
}

// TestProcessProject_DateMovedOutPastRecordedWindow_Invoice is the same for
// the invoice track: an extended due date resets the saved window.
func TestProcessProject_DateMovedOutPastRecordedWindow_Invoice(t *testing.T) {
	tests := []struct {
		name      string
		recorded  string
		dueInDays int
		invState  string
		wantReset bool
	}{
		{"7-day notice, due extended to +45", "7_days_notice", 45, "Open", true},
		{"7-day notice, due extended to +60", "7_days_notice", 60, "Open", true},
		{"90-day notice, due extended to +120", "90_days_notice", 120, "Open", true},
		{"suspend record (hand-reopened), due moved to +3", "suspend", 3, "Open", true},
		{"equal to the recorded window is not reset", "7_days_notice", 7, "Open", false},
		{"below the recorded window is not reset", "7_days_notice", 3, "Open", false},
		{"30-day notice at +30 is not reset", "30_days_notice", 30, "Open", false},
		{"track not Open (Suspended by ACP): untouched", "7_days_notice", 45, "Suspended", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := renewalProject(390, "open", tt.recorded)
			proj.InvoiceDueDateClosureState = strPtr(tt.invState)
			updater := &mockProjectUpdater{}
			if _, err := processProject(context.Background(), invoiceDueReader(t, tt.dueInDays), updater, &mockNotifier{}, staleNow, proj); err != nil {
				t.Fatalf("processProject() error = %v", err)
			}
			got := savedEventTypes(t, updater.calls)["based_on_due_invoices"] == "open"
			if got != tt.wantReset {
				t.Errorf("reset = %v, want %v; calls: %+v", got, tt.wantReset, updater.calls)
			}
			if !tt.wantReset && len(updater.calls) != 0 {
				t.Errorf("update calls = %d, want 0 (no churn): %+v", len(updater.calls), updater.calls)
			}
			if tt.wantReset && len(updater.calls) != 1 {
				t.Errorf("update calls = %d, want exactly 1", len(updater.calls))
			}
		})
	}
}

// TestMovedOut pins the comparison itself.
func TestMovedOut(t *testing.T) {
	for _, tt := range []struct {
		days, window int
		want         bool
	}{{8, 7, true}, {7, 7, false}, {6, 7, false}, {1, 0, true}, {0, 0, false}, {-3, 0, false}, {91, 90, true}, {90, 90, false}} {
		if got := movedOut(tt.days, closure.NoticeWindow(tt.window)); got != tt.want {
			t.Errorf("movedOut(%d, %d) = %v, want %v", tt.days, tt.window, got, tt.want)
		}
	}
}
