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

func TestLastNoticeWindow(t *testing.T) {
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
			raw:  json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"90_days_notice"}}`),
			want: &w90,
		},
		{
			name: "7_days_notice maps to NoticeWindow7",
			raw:  json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"7_days_notice"}}`),
			want: &w7,
		},
		{
			name: "suspend maps to the terminal NoticeWindow0",
			raw:  json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
			want: &w0,
		},
		{
			name: "open means no prior notice",
			raw:  json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"open"}}`),
			want: nil,
		},
		{
			name: "missing based_on_subscription_end_date key means no prior notice",
			raw:  json.RawMessage(`{"based_on_due_invoices":{"event_type":"7_days_notice"}}`),
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
			got, err := LastNoticeWindow(tt.raw)
			if err != nil {
				t.Fatalf("LastNoticeWindow() error = %v, want nil", err)
			}
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("LastNoticeWindow() = %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Errorf("LastNoticeWindow() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

// TestWithSubscriptionEndDateState_FirstEverWrite covers a project that has
// never had any suspensionProcessState recorded (nil blob) — the common case
// for a project's very first notice.
func TestWithSubscriptionEndDateState_FirstEverWrite(t *testing.T) {
	got, err := WithSubscriptionEndDateState(nil, closure.NoticeWindow90, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err != nil {
		t.Fatalf("WithSubscriptionEndDateState() error = %v, want nil", err)
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal(got, &blob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	section, ok := blob[subscriptionEndDateKey]
	if !ok {
		t.Fatalf("output missing key %q", subscriptionEndDateKey)
	}

	var state struct {
		EventType                   string `json:"event_type"`
		ActionSendEmailNotification string `json:"actionSendEmailNotification"`
	}
	if err := json.Unmarshal(section, &state); err != nil {
		t.Fatalf("parse %s: %v", subscriptionEndDateKey, err)
	}
	if state.EventType != "90_days_notice" {
		t.Errorf("event_type = %q, want %q", state.EventType, "90_days_notice")
	}
	if state.ActionSendEmailNotification != "SUCCESSFUL" {
		t.Errorf("actionSendEmailNotification = %q, want %q", state.ActionSendEmailNotification, "SUCCESSFUL")
	}
}

// TestWithSubscriptionEndDateState_PreservesOtherSectionsByteForByte is the
// dedicated, thorough test for the one behavior that could silently corrupt
// real production data on the next write: based_on_due_invoices and
// based_on_compliance must survive untouched when only
// based_on_subscription_end_date changes. Uses the real shape confirmed via
// a live write against the dedicated test project
// e3e87599-1bc7-6650-182c-0dc5604bcb68 — based_on_due_invoices with a real
// event_type and multiple already-populated action results, not a trivial
// empty case.
func TestWithSubscriptionEndDateState_PreservesOtherSectionsByteForByte(t *testing.T) {
	input := json.RawMessage(`{
		"based_on_subscription_end_date": {
			"event_type": "30_days_notice",
			"actionSendEmailNotification": "SUCCESSFUL"
		},
		"based_on_due_invoices": {
			"event_type": "7_days_notice",
			"actionSendEmailNotification": "SUCCESSFUL",
			"actionServicePortalAnnouncement": "SUCCESSFUL"
		},
		"based_on_compliance": {
			"event_type": "open"
		}
	}`)

	got, err := WithSubscriptionEndDateState(input, closure.NoticeWindow7, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err != nil {
		t.Fatalf("WithSubscriptionEndDateState() error = %v, want nil", err)
	}

	var gotBlob map[string]json.RawMessage
	if err := json.Unmarshal(got, &gotBlob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	var wantBlob map[string]json.RawMessage
	if err := json.Unmarshal(input, &wantBlob); err != nil {
		t.Fatalf("test input is not valid JSON: %v", err)
	}

	for _, key := range []string{"based_on_due_invoices", "based_on_compliance"} {
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

	// Sanity check: based_on_subscription_end_date did change, to the
	// expected window — otherwise this test would trivially pass by
	// comparing two blobs that both left everything untouched.
	changed, ok := gotBlob[subscriptionEndDateKey]
	if !ok {
		t.Fatalf("output missing key %q", subscriptionEndDateKey)
	}
	var state subscriptionEndDateState
	if err := json.Unmarshal(changed, &state); err != nil {
		t.Fatalf("parse %s: %v", subscriptionEndDateKey, err)
	}
	if state.EventType != "7_days_notice" {
		t.Errorf("based_on_subscription_end_date.event_type = %q, want %q", state.EventType, "7_days_notice")
	}
}

// TestWithSubscriptionEndDateState_RejectsUnmappedWindow verifies that a
// closure.NoticeWindow with no entry in windowToEventType (any value other
// than the six confirmed constants) is rejected with an error rather than
// silently written as event_type:"". A silent empty write would later be
// read back by LastNoticeWindow as "no prior notice" (since "" isn't in
// eventTypeToWindow either), risking a wrong re-notification cascade.
func TestWithSubscriptionEndDateState_RejectsUnmappedWindow(t *testing.T) {
	const unmapped closure.NoticeWindow = 5

	_, err := WithSubscriptionEndDateState(nil, unmapped, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err == nil {
		t.Fatal("WithSubscriptionEndDateState() error = nil, want an error for an unmapped window")
	}
}

// TestWithSubscriptionEndDateState_PreservesHTMLSensitiveCharactersLiterally
// covers a real gap normalizeJSON-based equivalence checks can't see
// (CodeRabbit, PR #1657 review): encoding/json's default marshaling
// HTML-escapes '<', '>', and '&' in *any* output it produces — including
// bytes coming from an untouched json.RawMessage value that was never
// semantically changed. A byte-for-byte preservation claim has to survive
// this literally, not just survive a semantic-equivalence check.
func TestWithSubscriptionEndDateState_PreservesHTMLSensitiveCharactersLiterally(t *testing.T) {
	input := json.RawMessage(`{
		"based_on_subscription_end_date": {"event_type": "open"},
		"based_on_due_invoices": {"event_type": "open", "note": "R&D <Team> \"Alpha\""}
	}`)

	got, err := WithSubscriptionEndDateState(input, closure.NoticeWindow90, map[string]string{
		"actionSendEmailNotification": "SUCCESSFUL",
	})
	if err != nil {
		t.Fatalf("WithSubscriptionEndDateState() error = %v, want nil", err)
	}

	if !bytes.Contains(got, []byte(`R&D <Team> \"Alpha\"`)) {
		t.Errorf("untouched section's literal characters were escaped instead of preserved:\ngot: %s", got)
	}
}

// normalizeJSON re-marshals a JSON value through Go's canonical encoding so
// two semantically-identical values that differ only in whitespace compare
// equal. The values under test here are never re-serialized by
// WithSubscriptionEndDateState itself (they're carried as untouched
// json.RawMessage) — this normalization exists solely so the test isn't
// sensitive to incidental formatting in how the two literals were typed.
func normalizeJSON(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("normalizeJSON: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("normalizeJSON: %v", err)
	}
	return out
}
