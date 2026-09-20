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

package suspensionstate

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
)

// TestLastNoticeWindowForInvoices mirrors TestLastNoticeWindow, but reads
// based_on_due_invoices instead — Phase 2's own idempotency track, entirely
// separate from the subscription end-date one. The event_type vocabulary is
// identical (confirmed against the live calculateEventTypeFromDate/
// eulaCalculateEventType legacy functions — the vestigial restrict/
// pending_restriction states in oldCalculateEventTypeFromDate are dead code,
// not part of this vocabulary).
func TestLastNoticeWindowForInvoices(t *testing.T) {
	w90 := closure.NoticeWindow90
	w7 := closure.NoticeWindow7
	w0 := closure.NoticeWindow0

	tests := []struct {
		name string
		raw  json.RawMessage
		want *closure.NoticeWindow
	}{
		{
			name: "90_days_notice maps to NoticeWindow90",
			raw:  json.RawMessage(`{"based_on_due_invoices":{"event_type":"90_days_notice"}}`),
			want: &w90,
		},
		{
			name: "7_days_notice maps to NoticeWindow7",
			raw:  json.RawMessage(`{"based_on_due_invoices":{"event_type":"7_days_notice"}}`),
			want: &w7,
		},
		{
			name: "suspend maps to the terminal NoticeWindow0",
			raw:  json.RawMessage(`{"based_on_due_invoices":{"event_type":"suspend"}}`),
			want: &w0,
		},
		{
			name: "open means no prior notice",
			raw:  json.RawMessage(`{"based_on_due_invoices":{"event_type":"open"}}`),
			want: nil,
		},
		{
			name: "missing based_on_due_invoices key means no prior notice, even if based_on_subscription_end_date is present",
			raw:  json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"7_days_notice"}}`),
			want: nil,
		},
		{
			name: "nil blob means no prior notice",
			raw:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LastNoticeWindowForInvoices(tt.raw)
			if err != nil {
				t.Fatalf("LastNoticeWindowForInvoices() error = %v, want nil", err)
			}
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("LastNoticeWindowForInvoices() = %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Errorf("LastNoticeWindowForInvoices() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

// TestWithDueInvoicesState_FirstEverWrite mirrors
// TestWithSubscriptionEndDateState_FirstEverWrite for a project that has
// never had any suspensionProcessState recorded.
func TestWithDueInvoicesState_FirstEverWrite(t *testing.T) {
	got, err := WithDueInvoicesState(nil, closure.NoticeWindow90, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err != nil {
		t.Fatalf("WithDueInvoicesState() error = %v, want nil", err)
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal(got, &blob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	section, ok := blob[dueInvoicesKey]
	if !ok {
		t.Fatalf("output missing key %q", dueInvoicesKey)
	}

	var state struct {
		EventType                   string `json:"event_type"`
		ActionSendEmailNotification string `json:"actionSendEmailNotification"`
	}
	if err := json.Unmarshal(section, &state); err != nil {
		t.Fatalf("parse %s: %v", dueInvoicesKey, err)
	}
	if state.EventType != "90_days_notice" {
		t.Errorf("event_type = %q, want %q", state.EventType, "90_days_notice")
	}
	if state.ActionSendEmailNotification != "SUCCESSFUL" {
		t.Errorf("actionSendEmailNotification = %q, want %q", state.ActionSendEmailNotification, "SUCCESSFUL")
	}
}

// TestWithDueInvoicesState_PreservesOtherSectionsByteForByte is the
// invoice-side mirror of the dedicated preservation test on the
// subscription end-date path — this time based_on_subscription_end_date and
// based_on_compliance must survive untouched when only based_on_due_invoices
// changes. This is the direction Phase 1's own preservation test didn't
// cover: Phase 1 only ever proved it doesn't clobber based_on_due_invoices,
// never that a due-invoices write doesn't clobber the subscription state.
func TestWithDueInvoicesState_PreservesOtherSectionsByteForByte(t *testing.T) {
	input := json.RawMessage(`{
		"based_on_subscription_end_date": {
			"event_type": "30_days_notice",
			"actionSendEmailNotification": "SUCCESSFUL"
		},
		"based_on_due_invoices": {
			"event_type": "15_days_notice",
			"actionSendEmailNotification": "SUCCESSFUL"
		},
		"based_on_compliance": {
			"event_type": "open"
		}
	}`)

	got, err := WithDueInvoicesState(input, closure.NoticeWindow7, map[string]string{
		"actionSendEmailNotification":     "SUCCESSFUL",
		"actionServicePortalAnnouncement": "SUCCESSFUL",
	})
	if err != nil {
		t.Fatalf("WithDueInvoicesState() error = %v, want nil", err)
	}

	var gotBlob map[string]json.RawMessage
	if err := json.Unmarshal(got, &gotBlob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	var wantBlob map[string]json.RawMessage
	if err := json.Unmarshal(input, &wantBlob); err != nil {
		t.Fatalf("test input is not valid JSON: %v", err)
	}

	for _, key := range []string{subscriptionEndDateKey, "based_on_compliance"} {
		gotSection, ok := gotBlob[key]
		if !ok {
			t.Fatalf("output missing key %q", key)
		}
		wantSection, ok := wantBlob[key]
		if !ok {
			t.Fatalf("test input missing key %q", key)
		}
		if !bytes.Equal(normalizeJSON(t, gotSection), normalizeJSON(t, wantSection)) {
			t.Errorf("%s changed:\n got  = %s\n want = %s", key, gotSection, wantSection)
		}
	}

	// Sanity check: based_on_due_invoices did change, to the expected
	// window — otherwise this test would trivially pass by comparing two
	// blobs that both left everything untouched.
	changed, ok := gotBlob[dueInvoicesKey]
	if !ok {
		t.Fatalf("output missing key %q", dueInvoicesKey)
	}
	var state dueInvoicesState
	if err := json.Unmarshal(changed, &state); err != nil {
		t.Fatalf("parse %s: %v", dueInvoicesKey, err)
	}
	if state.EventType != "7_days_notice" {
		t.Errorf("based_on_due_invoices.event_type = %q, want %q", state.EventType, "7_days_notice")
	}
}

// TestWithDueInvoicesState_RejectsUnmappedWindow mirrors
// TestWithSubscriptionEndDateState_RejectsUnmappedWindow — same rationale,
// same shared windowToEventType map.
func TestWithDueInvoicesState_RejectsUnmappedWindow(t *testing.T) {
	const unmapped closure.NoticeWindow = 5

	_, err := WithDueInvoicesState(nil, unmapped, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err == nil {
		t.Fatal("WithDueInvoicesState() error = nil, want an error for an unmapped window")
	}
}
