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

package dispatch

import (
	"context"
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

// Evaluation Subscription cases post their case.created, case.acknowledged
// and case.severity_changed cards to the Evaluation space, not Incident
// Monitor; every other case keeps going to Incident Monitor, and so does an
// Evaluation case when no Evaluation space is configured.
func TestDispatcher_Handle_EvaluationCaseCardsGoToEvaluationSpace(t *testing.T) {
	events := []struct {
		name     string
		payload  string // %t is isEvaluationAccount
		audience func(*mockGoogleChatSender) []string
	}{
		{
			name:    "case.created",
			payload: `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"R","projectName":"P","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"t","caseType":"CASE","priority":"HIGH","createdAt":"2026-01-01","description":"d","recipients":["test-recipient@example.com"],"isEvaluationAccount":%t}}`,
			audience: func(m *mockGoogleChatSender) []string {
				var out []string
				for _, c := range m.caseCreatedCalls {
					out = append(out, c.audience)
				}
				return out
			},
		},
		{
			name:    "case.created security_report_analysis",
			payload: `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"R","projectName":"P","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"t","caseType":"SECURITY_REPORT_ANALYSIS","priority":"","createdAt":"2026-01-01","description":"d","recipients":["test-recipient@example.com"],"isEvaluationAccount":%t}}`,
			audience: func(m *mockGoogleChatSender) []string {
				var out []string
				for _, c := range m.securityReportAnalysisCalls {
					out = append(out, c.audience)
				}
				return out
			},
		},
		{
			name:    "case.acknowledged",
			payload: `{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","severity":"HIGH","acknowledgerName":"Jane","isEvaluationAccount":%t}}`,
			audience: func(m *mockGoogleChatSender) []string {
				var out []string
				for _, c := range m.caseAcknowledgedCalls {
					out = append(out, c.audience)
				}
				return out
			},
		},
		{
			name:    "case.acknowledged security_report_analysis",
			payload: `{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","acknowledgerName":"Jane","caseType":"SECURITY_REPORT_ANALYSIS","isEvaluationAccount":%t}}`,
			audience: func(m *mockGoogleChatSender) []string {
				var out []string
				for _, c := range m.securityReportAnalysisAcknowledgedCalls {
					out = append(out, c.audience)
				}
				return out
			},
		},
		{
			name:    "case.severity_changed",
			payload: `{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"t","oldSeverity":"MEDIUM","newSeverity":"HIGH","recipients":["test-recipient@example.com"],"isEvaluationAccount":%t}}`,
			audience: func(m *mockGoogleChatSender) []string {
				var out []string
				for _, c := range m.severityChangedCalls {
					out = append(out, c.audience)
				}
				return out
			},
		},
	}
	cases := []struct {
		name               string
		isEvaluation       bool
		evaluationSpaceSet bool
		want               string
	}{
		{"evaluation case", true, true, chataudience.Evaluation},
		{"evaluation case without an evaluation space", true, false, chataudience.IncidentMonitor},
		{"other case", false, true, chataudience.IncidentMonitor},
	}

	for _, ev := range events {
		for _, tc := range cases {
			t.Run(ev.name+"/"+tc.name, func(t *testing.T) {
				evaluationSpaceSet := tc.evaluationSpaceSet
				chat := &mockGoogleChatSender{hasAudienceSpace: func(audience string) bool {
					return audience == chataudience.Evaluation && evaluationSpaceSet
				}}
				d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

				record := eventbus.Record{Value: []byte(fmt.Sprintf(ev.payload, tc.isEvaluation))}
				if err := d.Handle(context.Background(), record); err != nil {
					t.Fatalf("Handle() error = %v", err)
				}

				got := ev.audience(chat)
				if len(got) != 1 || got[0] != tc.want {
					t.Errorf("Chat card audiences = %q, want exactly [%q]", got, tc.want)
				}
			})
		}
	}
}
