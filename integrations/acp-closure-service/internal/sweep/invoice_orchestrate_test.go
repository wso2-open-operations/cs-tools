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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
)

// invoiceLinkedReader builds a mockEntityReader wired to resolve a single
// due invoice: one linked opportunity (eligible, EULA 3.4 — no grace
// period, so dueDate is the anchor as-is), one eligible invoice on it,
// firing the terminal (day-0) window as of `now`.
func invoiceLinkedReader(t *testing.T, now time.Time) *mockEntityReader {
	t.Helper()
	dueDate := now.Format("2006-01-02")
	return &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"opp1","name":"Opp One","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"invoices":[{
				"id":"inv1","invoiceDate":"2026-01-01","invoicedDueDate":"` + dueDate + `",
				"opportunity":{"id":"opp1","name":"Opp One"}
			}]}`), nil
		},
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"hasPrimaryPartner":false}`), nil
		},
	}
}

func TestProcessProject_InvoiceCascade_FiresNotifyAndSuspendOnDueDate(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := invoiceLinkedReader(t, now)
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{sendFn: func(ctx context.Context, n notify.Notice) (bool, error) { return true, nil }}

	proj := project{
		ID:      "p1",
		Name:    "Test Project",
		Account: &projectAccountRef{ID: "a1"},
		// No EndDate: this test isolates the invoice cascade from the
		// subscription one entirely.
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) == 0 {
		t.Fatal("ntf.sent is empty, want at least the internal invoice notice")
	}
	found := false
	for _, n := range ntf.sent {
		if n.Subject == "[ACP] Project Suspension Notice of Test Project" {
			found = true
		}
	}
	if !found {
		t.Errorf("no invoice-cascade internal notice found among sent notices: %+v", ntf.sent)
	}

	// Both the notify-record write and the suspend write should have
	// happened, touching invoiceDueDateClosureState / based_on_due_invoices
	// — never endDateClosureState / based_on_subscription_end_date, since
	// this project has no EndDate at all.
	var sawInvoiceSuspend, sawInvoiceRecord bool
	for _, c := range updater.calls {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.body, &body); err != nil {
			t.Fatalf("parse update body: %v", err)
		}
		if _, ok := body["invoiceDueDateClosureState"]; ok {
			sawInvoiceSuspend = true
		}
		if raw, ok := body["suspensionProcessState"]; ok {
			var state struct {
				BasedOnDueInvoices struct {
					EventType string `json:"event_type"`
				} `json:"based_on_due_invoices"`
			}
			json.Unmarshal(raw, &state)
			if state.BasedOnDueInvoices.EventType == "suspend" {
				sawInvoiceRecord = true
			}
		}
	}
	if !sawInvoiceSuspend {
		t.Errorf("no update call wrote invoiceDueDateClosureState; calls: %+v", updater.calls)
	}
	if !sawInvoiceRecord {
		t.Errorf("no update call recorded based_on_due_invoices.event_type = suspend; calls: %+v", updater.calls)
	}
}

// TestProcessProject_InvoiceCascade_IsPartnerWithoutPrimaryPartnerDisablesCascade
// covers legacy's actual precedence (ACPMainProcess.js's
// calculateEventTypeFromDate): hasPrimaryPartner is checked first,
// unconditionally; isPartner only disables the cascade when
// hasPrimaryPartner is false. So isPartner=true alone must NOT skip
// fetching invoice/account data up front — legacy fetches due invoices
// unconditionally too — it only means the cascade doesn't fire once both
// facts are known.
func TestProcessProject_InvoiceCascade_IsPartnerWithoutPrimaryPartnerDisablesCascade(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := invoiceLinkedReader(t, now) // getAccountFn defaults to hasPrimaryPartner:false
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	isPartner := true
	proj := project{
		ID:      "p1",
		Name:    "Test Project",
		Account: &projectAccountRef{ID: "a1", IsPartner: &isPartner},
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 — isPartner=true with hasPrimaryPartner=false must not fire", len(ntf.sent))
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

// TestProcessProject_InvoiceCascade_HasPrimaryPartnerOverridesIsPartner is
// the regression test for a real bug: a prior version of buildInvoiceCascade
// gated on isPartner alone, before hasPrimaryPartner was ever fetched — so a
// project whose account had BOTH isPartner=true and hasPrimaryPartner=true
// (a real, legal combination per legacy) had its invoice cascade disabled
// entirely, when legacy's calculateEventTypeFromDate would still fire it via
// the grace-period (parterLed) path. hasPrimaryPartner must be checked
// first, unconditionally, before isPartner is ever consulted.
func TestProcessProject_InvoiceCascade_HasPrimaryPartnerOverridesIsPartner(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := invoiceLinkedReader(t, now)
	reader.getAccountFn = func(ctx context.Context, id string) ([]byte, error) {
		return []byte(`{"hasPrimaryPartner":true}`), nil
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{sendFn: func(ctx context.Context, n notify.Notice) (bool, error) { return true, nil }}

	isPartner := true
	proj := project{
		ID:      "p1",
		Name:    "Test Project",
		Account: &projectAccountRef{ID: "a1", IsPartner: &isPartner},
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) == 0 {
		t.Fatal("ntf.sent is empty, want the invoice cascade to fire — hasPrimaryPartner=true must override isPartner=true")
	}
}

func TestProcessProject_InvoiceCascade_NoDueInvoiceIsNoOp(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := &mockEntityReader{} // default: no links, no due invoice
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	proj := project{ID: "p1", Name: "Test Project", Account: &projectAccountRef{ID: "a1"}}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

func TestProcessProject_InvoiceCascade_SkipsNotifyWhenAlreadyClosedForAnyReason(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := invoiceLinkedReader(t, now)
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	closureState := "Suspended" // already closed, e.g. by the subscription cascade
	proj := project{
		ID:           "p1",
		Name:         "Test Project",
		Account:      &projectAccountRef{ID: "a1"},
		ClosureState: &closureState,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 — already closed for another reason, must not notify", len(ntf.sent))
	}

	// The window still advances (recorded IGNORED), same contract as the
	// subscription cascade's equivalent test.
	var sawIgnored bool
	for _, c := range updater.calls {
		var body struct {
			SuspensionProcessState struct {
				BasedOnDueInvoices struct {
					ActionSendEmailNotification string `json:"actionSendEmailNotification"`
				} `json:"based_on_due_invoices"`
			} `json:"suspensionProcessState"`
		}
		json.Unmarshal(c.body, &body)
		if body.SuspensionProcessState.BasedOnDueInvoices.ActionSendEmailNotification == "IGNORED" {
			sawIgnored = true
		}
	}
	if !sawIgnored {
		t.Errorf("no update call recorded actionSendEmailNotification = IGNORED; calls: %+v", updater.calls)
	}
}

func TestProcessProject_InvoiceCascade_HasPrimaryPartnerForcesGracePeriod(t *testing.T) {
	// invoiceDate + 10 days due (short credit period), EULA 3.4 (no grace
	// on its own) — but the account HAS a primary partner, so the 60-day
	// grace period should still apply, keeping the account active well
	// past the raw due date.
	invoiceDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dueDate := invoiceDate.AddDate(0, 0, 10)
	now := dueDate.AddDate(0, 0, 5) // 5 days past the raw due date

	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"opp1","name":"Opp One","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"invoices":[{
				"id":"inv1","invoiceDate":"2026-01-01","invoicedDueDate":"` + dueDate.Format("2006-01-02") + `",
				"opportunity":{"id":"opp1","name":"Opp One"}
			}]}`), nil
		},
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"hasPrimaryPartner":true}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	proj := project{ID: "p1", Name: "Test Project", Account: &projectAccountRef{ID: "a1"}}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	for _, c := range updater.calls {
		var body map[string]json.RawMessage
		json.Unmarshal(c.body, &body)
		if _, ok := body["invoiceDueDateClosureState"]; ok {
			t.Error("invoiceDueDateClosureState should not be written yet — day 60 from the invoice date hasn't arrived, hasPrimaryPartner should still protect this project despite the raw due date having passed")
		}
	}
}

// TestProcessProject_InvoiceCascade_PassesInvoiceSfIDToInternalNoticeOnly
// covers the plumbing for the internal notice's "Open in Salesforce" link:
// the invoice's sfId from the API must reach the internal invoice notice,
// and no other notice (customer-facing or no-business-contact nudge) sent
// for the same window.
func TestProcessProject_InvoiceCascade_PassesInvoiceSfIDToInternalNoticeOnly(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	reader := invoiceLinkedReader(t, now)
	dueDate := now.Format("2006-01-02")
	reader.searchInvoicesFn = func(ctx context.Context, body []byte) ([]byte, error) {
		return []byte(`{"invoices":[{
			"id":"inv1","invoiceDate":"2026-01-01","invoicedDueDate":"` + dueDate + `",
			"opportunity":{"id":"opp1","name":"Opp One"},"sfId":"a0IE2000006XBu5MAG"
		}]}`), nil
	}
	ntf := &mockNotifier{sendFn: func(ctx context.Context, n notify.Notice) (bool, error) { return true, nil }}
	proj := project{ID: "p1", Name: "Test Project", Account: &projectAccountRef{ID: "a1"}}

	if err := processProject(context.Background(), reader, &mockProjectUpdater{}, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	sawInternal := false
	for _, n := range ntf.sent {
		if n.Subject == "[ACP] Project Suspension Notice of Test Project" {
			sawInternal = true
			if n.InvoiceSfID != "a0IE2000006XBu5MAG" {
				t.Errorf("internal notice InvoiceSfID = %q, want %q", n.InvoiceSfID, "a0IE2000006XBu5MAG")
			}
			continue
		}
		if n.InvoiceSfID != "" {
			t.Errorf("notice %q has InvoiceSfID = %q, want empty (internal notice only)", n.Subject, n.InvoiceSfID)
		}
	}
	if !sawInternal {
		t.Fatalf("no internal invoice notice sent; got %+v", ntf.sent)
	}
}
