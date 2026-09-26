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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
)

// bothCascadesReader builds a mockEntityReader wired so both the
// subscription and invoice cascades have everything they need to fire for
// project "p1": one linked, eligible opportunity/invoice (EULA 3.4, no
// grace period). getProjectFn fails the test if it's ever called —
// processProject's cross-cascade "already closed" gate is tracked entirely
// in memory (see cascadeDecision/processProject), deliberately never a live
// GetProject re-fetch, since a real live test showed that re-fetch reading
// stale (pre-suspend) data from this backend and letting both cascades
// notify anyway.
func bothCascadesReader(t *testing.T, invoiceDueDate time.Time) *mockEntityReader {
	t.Helper()
	return &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"opp1","name":"Opp One","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"invoices":[{
				"id":"inv1","invoiceDate":"2025-01-01","invoicedDueDate":"` + invoiceDueDate.Format("2006-01-02") + `",
				"opportunity":{"id":"opp1","name":"Opp One"}
			}]}`), nil
		},
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"hasPrimaryPartner":false}`), nil
		},
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			t.Error("GetProject should not be called — the same-run closure gate is tracked in memory, not via a live re-fetch")
			return []byte(`{}`), nil
		},
	}
}

// TestProcessProject_BothCascadesFireSameRun_MoreOverdueCascadeNotifiesOnly
// is the regression test for the real duplicate-email bug: a project whose
// subscription end date is far more overdue than its invoice due date
// fired both cascades in the same live run, and both sent their own
// "Project Suspension Notice" (identical subject, since neither has a
// cascade-specific subject variant) plus their own byte-for-byte-identical
// no-business-contact nudge — legacy would only ever send the more urgent
// reason's notice, silently ignoring the other via a live re-read of the
// shared closure status. Subscription is the more-overdue reason here, so
// it must be the one and only cascade that notifies.
func TestProcessProject_BothCascadesFireSameRun_MoreOverdueCascadeNotifiesOnly(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -100)       // far overdue
	invoiceDueDate := now.AddDate(0, 0, -10) // less overdue

	reader := bothCascadesReader(t, invoiceDueDate)
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{sendFn: func(ctx context.Context, n notify.Notice) (bool, error) { return true, nil }}

	proj := project{
		ID:      "p1",
		Name:    "Test Project",
		Account: &projectAccountRef{ID: "a1"},
		EndDate: &endDate,
	}

	if err := processProject(context.Background(), reader, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	count := 0
	nudgeCount := 0
	for _, n := range ntf.sent {
		if n.Subject == "[ACP] Project Suspension Notice of Test Project" {
			count++
		}
		if n.Subject == "[Urgent] [ACP] No Business Contacts Specified for Project Test Project" {
			nudgeCount++
		}
	}
	if count != 1 {
		t.Errorf("sent %d \"Project Suspension Notice\" emails, want exactly 1 (only the more-overdue subscription cascade should notify); sent: %+v", count, ntf.sent)
	}
	// The no-business-contact nudge has no cascade-specific content at
	// all (see noBusinessContactBody) — sent twice, the two copies would
	// be byte-for-byte identical, not just same-subject like the
	// suspension notice pair above. notifyForWindow is skipped entirely
	// for the invoice cascade here (actInvoice's alreadyClosed check
	// short-circuits before it's ever called), so this must be 1, not 2.
	if nudgeCount != 1 {
		t.Errorf("sent %d no-business-contact nudge emails, want exactly 1 (the invoice cascade's notifyForWindow call — internal notice AND nudge alike — must be skipped entirely once already closed); sent: %+v", nudgeCount, ntf.sent)
	}
	if len(ntf.sent) != 2 {
		t.Errorf("ntf.sent = %d notices, want exactly 2 (subscription cascade's internal notice + nudge only); sent: %+v", len(ntf.sent), ntf.sent)
	}
	if len(ntf.sent) > 0 && strings.Contains(ntf.sent[0].Body, "Invoice Id:") {
		t.Errorf("first sent notice body contains \"Invoice Id:\" — the invoice cascade (less overdue) notified before/instead of subscription")
	}

	if !recordedIgnored(updater.calls, "based_on_due_invoices") {
		t.Errorf("no update call recorded based_on_due_invoices.actionSendEmailNotification = IGNORED; calls: %+v", updater.calls)
	}
	if !wroteField(updater.calls, "endDateClosureState") {
		t.Error("no update call wrote endDateClosureState — the more-urgent cascade must still suspend")
	}
	if !wroteField(updater.calls, "invoiceDueDateClosureState") {
		t.Error("no update call wrote invoiceDueDateClosureState — suspend is per-dimension and must not be suppressed by the notify guard")
	}
}

// TestProcessProject_BothCascadesFireSameRun_OrderingIsDynamicNotHardcoded
// flips the urgency from the test above — invoice far more overdue than
// subscription — to prove the ordering genuinely follows days_left rather
// than a hardcoded subscription-always-first sequence.
func TestProcessProject_BothCascadesFireSameRun_OrderingIsDynamicNotHardcoded(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -10)         // less overdue
	invoiceDueDate := now.AddDate(0, 0, -100) // far overdue

	reader := bothCascadesReader(t, invoiceDueDate)
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{sendFn: func(ctx context.Context, n notify.Notice) (bool, error) { return true, nil }}

	proj := project{
		ID:      "p1",
		Name:    "Test Project",
		Account: &projectAccountRef{ID: "a1"},
		EndDate: &endDate,
	}

	if err := processProject(context.Background(), reader, updater, ntf, now, proj); err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) == 0 {
		t.Fatal("ntf.sent is empty, want at least the invoice cascade's internal notice")
	}
	if !strings.Contains(ntf.sent[0].Body, "Invoice Id:") {
		t.Errorf("first sent notice body = %q, want the invoice cascade's body (contains \"Invoice Id:\") since it's more overdue and must go first", ntf.sent[0].Body)
	}

	if !recordedIgnored(updater.calls, "based_on_subscription_end_date") {
		t.Errorf("no update call recorded based_on_subscription_end_date.actionSendEmailNotification = IGNORED; calls: %+v", updater.calls)
	}
}

func recordedIgnored(calls []updateCall, section string) bool {
	for _, c := range calls {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.body, &body); err != nil {
			continue
		}
		raw, ok := body["suspensionProcessState"]
		if !ok {
			continue
		}
		var state map[string]struct {
			ActionSendEmailNotification string `json:"actionSendEmailNotification"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			continue
		}
		if s, ok := state[section]; ok && s.ActionSendEmailNotification == "IGNORED" {
			return true
		}
	}
	return false
}

func wroteField(calls []updateCall, field string) bool {
	for _, c := range calls {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.body, &body); err != nil {
			continue
		}
		if _, ok := body[field]; ok {
			return true
		}
	}
	return false
}
