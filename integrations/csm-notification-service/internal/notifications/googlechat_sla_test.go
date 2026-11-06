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

package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSLAStateLabel(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"open", "OPEN", "Open"},
		{"work in progress", "WORK_IN_PROGRESS", "Work In Progress"},
		{"awaiting info", "AWAITING_INFO", "Awaiting Info"},
		{"waiting on wso2", "WAITING_ON_WSO2", "Waiting on WSO2"},
		{"solution proposed", "SOLUTION_PROPOSED", "Solution Proposed"},
		{"reopened", "REOPENED", "Reopened"},
		{"closed", "CLOSED", "Closed"},
		{"unmapped state falls back to raw", "SOMETHING_NEW", "SOMETHING_NEW"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slaStateLabel(tt.in); got != tt.want {
				t.Errorf("slaStateLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSendSLABreachAlert_StateLineIsReadable verifies the State row shows the
// readable label, not the raw enum, and is omitted when state is empty.
func TestSendSLABreachAlert_StateLineIsReadable(t *testing.T) {
	tests := []struct {
		name, state, wantContains string
		wantState                 bool
	}{
		{"raw enum", "WORK_IN_PROGRESS", "<b>State :</b> Work In Progress", true},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured chatCardMessage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})
			err := c.SendSLABreachAlert(context.Background(), "api-manager", "response", "100",
				"CS0001001", "", "Sample case", "", "", "", "", "HIGH", tt.state, "", "")
			if err != nil {
				t.Fatalf("SendSLABreachAlert returned error: %v", err)
			}
			if len(captured.CardsV2) != 1 || len(captured.CardsV2[0].Card.Sections) == 0 {
				t.Fatalf("unexpected card shape: %+v", captured)
			}
			var text strings.Builder
			for _, s := range captured.CardsV2[0].Card.Sections {
				for _, w := range s.Widgets {
					if w.TextParagraph != nil {
						text.WriteString(w.TextParagraph.Text)
					}
				}
			}
			got := text.String()
			if strings.Contains(got, "WORK_IN_PROGRESS") {
				t.Errorf("card text %q still contains the raw enum", got)
			}
			if tt.wantState && !strings.Contains(got, tt.wantContains) {
				t.Errorf("card text %q missing %q", got, tt.wantContains)
			}
			if !tt.wantState && strings.Contains(got, "State :") {
				t.Errorf("card text %q has a State row, want none", got)
			}
		})
	}
}
