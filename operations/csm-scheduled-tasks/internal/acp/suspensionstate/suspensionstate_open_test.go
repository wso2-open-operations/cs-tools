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
)

const renewedInput = `{
	"based_on_subscription_end_date": {
		"event_type": "30_days_notice",
		"actionSendEmailNotification": "SUCCESSFUL"
	},
	"based_on_due_invoices": {
		"event_type": "suspend",
		"actionSendEmailNotification": "SUCCESSFUL"
	},
	"based_on_compliance": {
		"event_type": "open"
	}
}`

// TestWithOpen covers both tracks' reset: the named section becomes exactly
// {"event_type":"open"} (the old action results are dropped, as legacy's
// fresh template did), LastNoticeWindow then reports no notice, and every
// other section is carried through unchanged.
func TestWithOpen(t *testing.T) {
	tests := []struct {
		name      string
		reset     func(json.RawMessage) (json.RawMessage, error)
		key       string
		untouched []string
	}{
		{"subscription end date", WithSubscriptionEndDateOpen, subscriptionEndDateKey, []string{dueInvoicesKey, "based_on_compliance"}},
		{"due invoices", WithDueInvoicesOpen, dueInvoicesKey, []string{subscriptionEndDateKey, "based_on_compliance"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.reset(json.RawMessage(renewedInput))
			if err != nil {
				t.Fatalf("reset error = %v, want nil", err)
			}

			var gotBlob, wantBlob map[string]json.RawMessage
			if err := json.Unmarshal(got, &gotBlob); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(renewedInput), &wantBlob); err != nil {
				t.Fatalf("test input is not valid JSON: %v", err)
			}

			if !bytes.Equal(normalizeJSON(t, gotBlob[tt.key]), normalizeJSON(t, json.RawMessage(`{"event_type":"open"}`))) {
				t.Errorf("%s = %s, want {\"event_type\":\"open\"}", tt.key, gotBlob[tt.key])
			}
			window, err := lastNoticeWindowForKey(got, tt.key)
			if err != nil || window != nil {
				t.Errorf("last notice window after reset = %v, %v, want nil, nil", window, err)
			}
			for _, key := range tt.untouched {
				if !bytes.Equal(normalizeJSON(t, gotBlob[key]), normalizeJSON(t, wantBlob[key])) {
					t.Errorf("%s changed:\n got  = %s\n want = %s", key, gotBlob[key], wantBlob[key])
				}
			}
		})
	}
}

func TestWithOpen_RejectsCorruptBlob(t *testing.T) {
	if _, err := WithSubscriptionEndDateOpen(json.RawMessage(`{not json`)); err == nil {
		t.Error("WithSubscriptionEndDateOpen(corrupt) error = nil, want an error")
	}
	if _, err := WithDueInvoicesOpen(json.RawMessage(`{not json`)); err == nil {
		t.Error("WithDueInvoicesOpen(corrupt) error = nil, want an error")
	}
}
