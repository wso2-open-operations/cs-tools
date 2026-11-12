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
	"encoding/json"
	"testing"
)

func TestHasRecordedEvent(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want bool
	}{
		{"empty", ``, false},
		{"empty object", `{}`, false},
		{"both open", `{"based_on_subscription_end_date":{"event_type":"open"},"based_on_due_invoices":{"event_type":"open"}}`, false},
		{"no event type", `{"based_on_due_invoices":{}}`, false},
		{"end date notice", `{"based_on_subscription_end_date":{"event_type":"90_days_notice"}}`, true},
		{"invoice suspension", `{"based_on_due_invoices":{"event_type":"suspend"}}`, true},
		{"suspend notice", `{"based_on_subscription_end_date":{"event_type":"suspend_notice"}}`, true},
		{"unknown event type", `{"based_on_due_invoices":{"event_type":"new_kind"}}`, true},
		{"unparseable blob", `"text"`, true},
		{"unparseable section", `{"based_on_due_invoices":"text"}`, true},
		{"other keys only", `{"something_else":{"event_type":"suspend"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasRecordedEvent(json.RawMessage(tc.raw)); got != tc.want {
				t.Errorf("HasRecordedEvent(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
