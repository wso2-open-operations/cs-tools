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

// TestUsesGracePeriod covers the three-way precedence ported from the
// legacy ACPMainProcess.js: an account with a primary partner always gets
// the EULA-3.3-style grace period, regardless of the invoice's own EULA
// version; otherwise it's decided purely by whether the EULA version is
// 3.3 or below.
func TestUsesGracePeriod(t *testing.T) {
	tests := []struct {
		name              string
		eulaVersion       float64
		hasPrimaryPartner bool
		want              bool
	}{
		{"EULA 3.3 uses the grace period", 3.3, false, true},
		{"EULA 3.2 (below 3.3) uses the grace period", 3.2, false, true},
		{"EULA 3.4 does not use the grace period", 3.4, false, false},
		{"EULA 3.5 does not use the grace period", 3.5, false, false},
		{"primary partner forces the grace period even at EULA 3.4", 3.4, true, true},
		{"eula version unset (zero) does not use the grace period on its own", 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usesGracePeriod(tt.eulaVersion, tt.hasPrimaryPartner); got != tt.want {
				t.Errorf("usesGracePeriod(%v, %v) = %v, want %v", tt.eulaVersion, tt.hasPrimaryPartner, got, tt.want)
			}
		})
	}
}

// TestInvoiceSuspendDate covers the EULA-3.3-style grace period math: the
// account stays active for a minimum of 60 days from the invoice date, but
// never past the real due date once the credit period (due date minus
// invoice date) already exceeds 60 days — suspending at day 60 in that case
// would be unfair, since the invoice genuinely isn't due yet.
func TestInvoiceSuspendDate(t *testing.T) {
	invoiceDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		invoiceDueDate    time.Time
		eulaVersion       float64
		hasPrimaryPartner bool
		want              time.Time
	}{
		{
			// Invoice date Jan 1, due date Jan 30 — a 29-day credit period,
			// under 60. The due date is ignored; the account stays active
			// until day 60 from the invoice date instead.
			name:           "EULA 3.3, short credit period: grace period wins, suspend at day 60",
			invoiceDueDate: invoiceDate.AddDate(0, 0, 29),
			eulaVersion:    3.3,
			want:           invoiceDate.AddDate(0, 0, 60),
		},
		{
			// Invoice date Jan 1, due date Apr 1 — a 90-day credit period,
			// over 60. Suspending at day 60 would be unfair (not due yet),
			// so the real due date wins instead.
			name:           "EULA 3.3, long credit period: real due date wins",
			invoiceDueDate: invoiceDate.AddDate(0, 0, 90),
			eulaVersion:    3.3,
			want:           invoiceDate.AddDate(0, 0, 90),
		},
		{
			name:           "EULA 3.4: no grace period at all, due date always wins even when short",
			invoiceDueDate: invoiceDate.AddDate(0, 0, 10),
			eulaVersion:    3.4,
			want:           invoiceDate.AddDate(0, 0, 10),
		},
		{
			name:              "account has a primary partner: forces grace-period math even at EULA 3.4",
			invoiceDueDate:    invoiceDate.AddDate(0, 0, 10),
			eulaVersion:       3.4,
			hasPrimaryPartner: true,
			want:              invoiceDate.AddDate(0, 0, 60),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := invoiceSuspendDate(invoiceDate, tt.invoiceDueDate, tt.eulaVersion, tt.hasPrimaryPartner)
			if !got.Equal(tt.want) {
				t.Errorf("invoiceSuspendDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDecideInvoice_CascadeFiresRelativeToComputedSuspendDate covers the
// combined-scenario example: invoice issued Jun 1, due Jun 30, EULA 3.4 (no
// grace period, so the due date is the anchor as-is). On Jun 23 — exactly 7
// days before the due date — the 7-day window fires; a week earlier, with
// no milestone reached yet, nothing does.
func TestDecideInvoice_CascadeFiresRelativeToComputedSuspendDate(t *testing.T) {
	invoiceDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	invoiceDueDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		now        time.Time
		wantFires  bool
		wantWindow NoticeWindow
	}{
		{
			name:      "91 days before the due date, no milestone reached: fires nothing",
			now:       invoiceDueDate.AddDate(0, 0, -91),
			wantFires: false,
		},
		{
			name:       "exactly 7 days before the due date: fires the 7-day window",
			now:        time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC),
			wantFires:  true,
			wantWindow: NoticeWindow7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideInvoice(tt.now, invoiceDate, invoiceDueDate, 3.4, false, nil)

			if got.Fires != tt.wantFires {
				t.Errorf("Fires = %v, want %v", got.Fires, tt.wantFires)
			}
			if tt.wantFires && got.Window != tt.wantWindow {
				t.Errorf("Window = %v, want %v", got.Window, tt.wantWindow)
			}
		})
	}
}

// TestDecideInvoice_GracePeriodShiftsTheAnchorDate confirms DecideInvoice
// actually uses invoiceSuspendDate's computed date as its cascade anchor,
// not the raw due date — an EULA 3.3 invoice with a short credit period
// (due date already passed) must not suspend early if day 60 from the
// invoice date hasn't arrived yet.
func TestDecideInvoice_GracePeriodShiftsTheAnchorDate(t *testing.T) {
	invoiceDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	invoiceDueDate := invoiceDate.AddDate(0, 0, 10) // 10-day credit period, well under 60
	now := invoiceDate.AddDate(0, 0, 30)            // due date long passed, day 60 has not arrived

	got := DecideInvoice(now, invoiceDate, invoiceDueDate, 3.3, false, nil)

	if got.ShouldSuspend {
		t.Error("ShouldSuspend = true, want false — day 60 from the invoice date hasn't arrived yet, so the grace period should still protect this project")
	}
}

// TestDecideInvoice_ReusesTheSameIdempotencyContractAsDecide confirms the
// invoice cascade shares Decide's exact lastNoticeWindow semantics — a
// window already recorded doesn't refire, and day-0 notify/suspend are
// still reported independently. This is a thin check on top of decide.go's
// own thorough coverage of that shared behavior, not a re-test of it.
func TestDecideInvoice_ReusesTheSameIdempotencyContractAsDecide(t *testing.T) {
	invoiceDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	invoiceDueDate := invoiceDate.AddDate(0, 0, 90) // no grace period involved
	now := invoiceDueDate.AddDate(0, 0, 3)          // 3 days past due

	w0 := NoticeWindow0

	got := DecideInvoice(now, invoiceDate, invoiceDueDate, 3.4, false, &w0)

	if got.ShouldNotify {
		t.Error("ShouldNotify = true, want false — day-0 notice already recorded, should not refire")
	}
	if !got.ShouldSuspend {
		t.Error("ShouldSuspend = false, want true — suspend has no idempotency signal here, must still report true")
	}
}
