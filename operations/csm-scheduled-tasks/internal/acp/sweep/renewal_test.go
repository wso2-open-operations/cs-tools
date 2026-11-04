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
	"encoding/json"
	"errors"
	"testing"
	"time"
)

var renewalNow = time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

// renewalProject is an Open project whose saved suspensionProcessState is
// given per track. No opportunity links are mocked by default, so the
// invoice track has no due invoice.
func renewalProject(endDateInDays int, subEvent, invoiceEvent string) project {
	endDate := renewalNow.AddDate(0, 0, endDateInDays)
	return project{
		ID:                         "p1",
		Name:                       "Test Project",
		Account:                    &projectAccountRef{ID: "a1"},
		EndDate:                    &endDate,
		ClosureState:               strPtr("Open"),
		EndDateClosureState:        strPtr("Open"),
		InvoiceDueDateClosureState: strPtr("Open"),
		SuspensionProcessState: json.RawMessage(`{
			"based_on_subscription_end_date": {"event_type": "` + subEvent + `", "actionSendEmailNotification": "SUCCESSFUL"},
			"based_on_due_invoices": {"event_type": "` + invoiceEvent + `"}
		}`),
	}
}

// savedEventTypes returns the event_type of every section in the last
// suspensionProcessState write, or nil if none was written.
func savedEventTypes(t *testing.T, calls []updateCall) map[string]string {
	t.Helper()
	var last map[string]string
	for _, c := range calls {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.body, &body); err != nil {
			t.Fatalf("update body is not JSON: %v", err)
		}
		raw, ok := body["suspensionProcessState"]
		if !ok {
			continue
		}
		var state map[string]struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatalf("suspensionProcessState is not JSON: %v", err)
		}
		last = map[string]string{}
		for k, v := range state {
			last[k] = v.EventType
		}
	}
	return last
}

func runRenewal(t *testing.T, reader *mockEntityReader, proj project) (*mockProjectUpdater, *mockNotifier) {
	t.Helper()
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}
	if err := processProject(context.Background(), reader, updater, ntf, renewalNow, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 0 {
		t.Errorf("sent %d notices, want 0 (a reset never emails): %+v", len(ntf.sent), ntf.sent)
	}
	return updater, ntf
}

// TestProcessProject_RenewedAfterReminder_ResetsEndDateToOpen: the customer
// renewed after the 30-day reminder, so the end date is more than 90 days
// away again. Legacy saved "open" here; without it the next term's 90, 60
// and 30-day reminders are skipped.
func TestProcessProject_RenewedAfterReminder_ResetsEndDateToOpen(t *testing.T) {
	updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(390, "30_days_notice", "open"))

	got := savedEventTypes(t, updater.calls)
	if got["based_on_subscription_end_date"] != "open" {
		t.Errorf("end-date event_type = %q, want open; calls: %+v", got["based_on_subscription_end_date"], updater.calls)
	}
	if got["based_on_due_invoices"] != "open" {
		t.Errorf("invoice event_type = %q, want it kept as open", got["based_on_due_invoices"])
	}
	if len(updater.calls) != 1 {
		t.Errorf("update calls = %d, want 1", len(updater.calls))
	}
	if wroteField(updater.calls, "endDateClosureState") || len(updater.sfSuspendCalls) != 0 {
		t.Error("a reset must not touch the closure state or Salesforce")
	}
}

func TestProcessProject_RenewedAfterSuspensionAndReopen_ResetsEndDateToOpen(t *testing.T) {
	updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(390, "suspend", "open"))

	if got := savedEventTypes(t, updater.calls)["based_on_subscription_end_date"]; got != "open" {
		t.Errorf("end-date event_type = %q, want open", got)
	}
}

func TestProcessProject_AlreadyOpen_WritesNothing(t *testing.T) {
	updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(390, "open", "open"))

	if len(updater.calls) != 0 {
		t.Errorf("update calls = %d, want 0 (nothing to reset): %+v", len(updater.calls), updater.calls)
	}
}

// TestProcessProject_ResetBoundary: 91 days left is a new term (legacy's
// "open"), 90 days is still inside the 90-day window.
func TestProcessProject_ResetBoundary(t *testing.T) {
	tests := []struct {
		days      int
		wantReset bool
	}{
		{91, true},
		{90, false},
		{75, false},
	}
	for _, tt := range tests {
		updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(tt.days, "90_days_notice", "open"))
		reset := savedEventTypes(t, updater.calls)["based_on_subscription_end_date"] == "open"
		if reset != tt.wantReset {
			t.Errorf("%d days left: reset = %v, want %v; calls: %+v", tt.days, reset, tt.wantReset, updater.calls)
		}
	}
}

// TestProcessProject_StillSuspended_NotReset: renewed but nobody reopened
// the project yet. Its record stays as is; reopening is a separate step.
func TestProcessProject_StillSuspended_NotReset(t *testing.T) {
	proj := renewalProject(390, "suspend", "suspend")
	proj.ClosureState = strPtr("Suspended")
	proj.EndDateClosureState = strPtr("Closed")
	proj.InvoiceDueDateClosureState = strPtr("Suspended")

	updater, _ := runRenewal(t, &mockEntityReader{}, proj)

	if len(updater.calls) != 0 {
		t.Errorf("update calls = %d, want 0 for a still-suspended project: %+v", len(updater.calls), updater.calls)
	}
}

// TestProcessProject_InvoicePaid_ResetsInvoiceToOpen: the invoice that drove
// the reminders is paid, so the project has no due invoice left.
func TestProcessProject_InvoicePaid_ResetsInvoiceToOpen(t *testing.T) {
	updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(390, "open", "7_days_notice"))

	got := savedEventTypes(t, updater.calls)
	if got["based_on_due_invoices"] != "open" {
		t.Errorf("invoice event_type = %q, want open; calls: %+v", got["based_on_due_invoices"], updater.calls)
	}
	if wroteField(updater.calls, "invoiceDueDateClosureState") {
		t.Error("a reset must not touch invoiceDueDateClosureState")
	}
}

func TestProcessProject_InvoiceDueMoreThan90DaysAway_ResetsInvoiceToOpen(t *testing.T) {
	reader := invoiceLinkedReader(t, renewalNow.AddDate(0, 0, 200))
	updater, _ := runRenewal(t, reader, renewalProject(390, "open", "30_days_notice"))

	if got := savedEventTypes(t, updater.calls)["based_on_due_invoices"]; got != "open" {
		t.Errorf("invoice event_type = %q, want open", got)
	}
}

func TestProcessProject_InvoiceStillWithin90Days_NotReset(t *testing.T) {
	reader := invoiceLinkedReader(t, renewalNow.AddDate(0, 0, 40))
	updater, _ := runRenewal(t, reader, renewalProject(390, "open", "30_days_notice"))

	if len(updater.calls) != 0 {
		t.Errorf("update calls = %d, want 0: %+v", len(updater.calls), updater.calls)
	}
}

// TestProcessProject_PartnerWithoutPrimaryPartner_ResetsInvoiceToOpen: the
// invoice track no longer applies to the account (legacy computed "open").
func TestProcessProject_PartnerWithoutPrimaryPartner_ResetsInvoiceToOpen(t *testing.T) {
	isPartner := true
	proj := renewalProject(390, "open", "15_days_notice")
	proj.Account.IsPartner = &isPartner
	reader := invoiceLinkedReader(t, renewalNow.AddDate(0, 0, 5))

	updater, _ := runRenewal(t, reader, proj)

	if got := savedEventTypes(t, updater.calls)["based_on_due_invoices"]; got != "open" {
		t.Errorf("invoice event_type = %q, want open", got)
	}
}

// TestProcessProject_BothTracksRenewed_BothSaved: the second reset builds on
// the first, so neither overwrites the other with a stale copy.
func TestProcessProject_BothTracksRenewed_BothSaved(t *testing.T) {
	updater, _ := runRenewal(t, &mockEntityReader{}, renewalProject(390, "30_days_notice", "15_days_notice"))

	got := savedEventTypes(t, updater.calls)
	if got["based_on_subscription_end_date"] != "open" || got["based_on_due_invoices"] != "open" {
		t.Errorf("last saved state = %v, want both tracks open; calls: %+v", got, updater.calls)
	}
}

// TestProcessProject_InvoiceLookupFails_InvoiceNotReset: a failed invoice
// lookup is not "no due invoice", so the invoice track must not be reset.
func TestProcessProject_InvoiceLookupFails_InvoiceNotReset(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return nil, errors.New("sales-entity-service unavailable")
		},
	}
	updater := &mockProjectUpdater{}
	err := processProject(context.Background(), reader, updater, &mockNotifier{}, renewalNow, renewalProject(390, "30_days_notice", "15_days_notice"))
	if err == nil {
		t.Fatal("processProject() error = nil, want the invoice lookup error")
	}

	got := savedEventTypes(t, updater.calls)
	if got["based_on_subscription_end_date"] != "open" {
		t.Errorf("end-date event_type = %q, want open (the end-date track does not depend on invoices)", got["based_on_subscription_end_date"])
	}
	if got["based_on_due_invoices"] != "15_days_notice" {
		t.Errorf("invoice event_type = %q, want it kept as 15_days_notice", got["based_on_due_invoices"])
	}
}

func TestProcessProject_ResetWriteFails_ReturnsError(t *testing.T) {
	updater := &mockProjectUpdater{
		updateProjectFn: func(ctx context.Context, id string, body []byte) ([]byte, error) {
			return nil, errors.New("csm down")
		},
	}
	err := processProject(context.Background(), &mockEntityReader{}, updater, &mockNotifier{}, renewalNow, renewalProject(390, "30_days_notice", "open"))
	if err == nil {
		t.Error("processProject() error = nil, want the failed reset write")
	}
}
