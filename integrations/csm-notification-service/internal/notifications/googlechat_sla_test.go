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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureSLABreachCard(t *testing.T) (*GoogleChatClient, func() chatCardMessage) {
	t.Helper()
	var msg chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "Team Nova", WebhookURL: srv.URL}}})
	return c, func() chatCardMessage { return msg }
}

// A security_report_analysis case never carries a severity (see
// dispatch.isSecurityReportAnalysisCaseType) -- the breach card must not
// render an empty "[]" bracket in the title, nor an "Unknown"/gray Priority
// line in the body.
func TestSendSLABreachAlert_NoSeverity_OmitsPriorityAndBracket(t *testing.T) {
	c, got := captureSLABreachCard(t)
	err := c.SendSLABreachAlert(context.Background(), "Team Nova", "response", "100", "CS0001", "WSO2-1", "Suspicious login reported", "SECURITY_REPORT_ANALYSIS", "api-manager", "Team Nova", "", "", "NEW", "2026-01-01 10:00:00 (UTC)", "https://csm.example/cases/1")
	if err != nil {
		t.Fatalf("SendSLABreachAlert: %v", err)
	}
	card := got().CardsV2[0].Card
	if strings.HasPrefix(card.Header.Title, "[") {
		t.Errorf("title = %q, want no empty severity bracket", card.Header.Title)
	}
	if !strings.Contains(card.Header.Title, "Response SLA Violation") {
		t.Errorf("title = %q, want the violation wording intact", card.Header.Title)
	}
	body := card.Sections[0].Widgets[0].TextParagraph.Text
	if strings.Contains(body, "Priority") {
		t.Errorf("body = %q, want no Priority line when severity is empty", body)
	}
}

// A regular case still shows its severity exactly as before -- this change
// must not affect the common case.
func TestSendSLABreachAlert_WithSeverity_ShowsBracketAndPriority(t *testing.T) {
	c, got := captureSLABreachCard(t)
	err := c.SendSLABreachAlert(context.Background(), "Team Nova", "response", "100", "CS0002", "WSO2-2", "Gateway returning 500s", "CASE", "api-manager", "Team Nova", "", "HIGH", "NEW", "2026-01-01 10:00:00 (UTC)", "https://csm.example/cases/2")
	if err != nil {
		t.Fatalf("SendSLABreachAlert: %v", err)
	}
	card := got().CardsV2[0].Card
	if !strings.HasPrefix(card.Header.Title, "[HIGH] Response SLA Violation") || !strings.Contains(card.Header.Title, "CS0002") {
		t.Errorf("title = %q, want the severity bracket and case ref intact", card.Header.Title)
	}
	body := card.Sections[0].Widgets[0].TextParagraph.Text
	if !strings.Contains(body, "Priority") || !strings.Contains(body, "High (P2)") {
		t.Errorf("body = %q, want the Priority line with its resolved label", body)
	}
}
