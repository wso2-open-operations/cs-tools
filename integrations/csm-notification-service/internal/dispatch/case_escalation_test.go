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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

const escalationTestCase = "11111111-1111-1111-1111-111111111111"

func caseEscalatedRecord(previous, current int) eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"case.escalated","entityId":"` + escalationTestCase + `","payload":{` +
		`"caseId":"` + escalationTestCase + `","caseNumber":"CS0448958","caseTitle":"Gateway down",` +
		`"severity":"MEDIUM","accountName":"Acme","product":"WSO2 Identity Server 7.1.0","environment":"Production",` +
		`"assignedEngineerEmail":"eng@wso2.com","escalationId":"e1",` +
		`"previousLevel":` + string(rune('0'+previous)) + `,"currentLevel":` + string(rune('0'+current)) + `,` +
		`"reason":"Down <b>all day</b>","actorEmail":"escalator@wso2.com","escalatedOn":"2026-10-08T09:03:17Z",` +
		`"recipients":["lead@wso2.com","tu@wso2.com"]}}`)}
}

// TestDispatcher_Handle_CaseEscalated_SNEmail pins ServiceNow's "Internal
// Escalation notification" email: To = the list, its subject, and both
// tables row for row (discovery script 82), with the time in Asia/Colombo --
// in this service's own email shell.
func TestDispatcher_Handle_CaseEscalated_SNEmail(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	if err := d.Handle(context.Background(), caseEscalatedRecord(3, 4)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(mock.calls))
	}
	got := mock.calls[0]
	if want := []string{"lead@wso2.com", "tu@wso2.com"}; !reflect.DeepEqual(got.to, want) {
		t.Errorf("to = %v, want %v", got.to, want)
	}
	if want := "Case Escalation To EL4"; got.subject != want {
		t.Errorf("subject = %q, want %q", got.subject, want)
	}
	for _, want := range []string{
		"Support Case Escalation", "Hi Team,", "requires your <strong>immediate attention</strong>",
		">Case ID<", ">CS0448958<", ">Escalated By<", ">escalator@wso2.com<", ">Customer Account<", ">Acme<",
		">Escalation Path<", ">EL3 -&gt; EL4<", ">Escalation Time<", ">2026-10-08 14:33:17<",
		">Escalation Reason<", ">Down &lt;b&gt;all day&lt;/b&gt;<",
		">Title<", ">Gateway down<", ">Product<", ">WSO2 Identity Server 7.1.0<", ">Priority<", ">Medium (P3)<",
		">Environment<", ">Production<", ">Assigned Engineer<", ">eng@wso2.com<",
		"Acknowledge the escalation.", ">\n                      View Case\n", "https://csm.example/cases/" + escalationTestCase,
		// The house email shell every other email here uses: header line,
		// footer and the WSO2 logo.
		"escalator@wso2.com <b>escalated</b>", "from <b>EL3</b> to <b>EL4</b>",
		"This message was sent by WSO2's support system.", "WSO2-Logo-Black.png",
	} {
		if !strings.Contains(got.htmlBody, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Contains(got.htmlBody, "Expected sent to") {
		t.Error("the debug-mode recipient line must not render outside debug mode")
	}
	if strings.Contains(got.htmlBody, "Acknowledge Escalation") {
		t.Error("the case link must not promise an acknowledgement nothing records")
	}
}

func TestDispatcher_Handle_CaseEscalated_DebugModeRedirects(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, true, true, []string{"debug@wso2.com"}, true, "", nil)

	if err := d.Handle(context.Background(), caseEscalatedRecord(0, 1)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	got := mock.calls[0]
	if want := []string{"debug@wso2.com"}; !reflect.DeepEqual(got.to, want) {
		t.Errorf("to = %v, want %v", got.to, want)
	}
	if !strings.Contains(got.htmlBody, "Expected sent to: lead@wso2.com, tu@wso2.com") {
		t.Error("debug mode must show who the mail was meant for")
	}
}

// TestDispatcher_Handle_CaseEscalated_DeescalationRejected: a de-escalation
// is never published (SN's flow mails only u_current_level != 0); one that
// arrives anyway is malformed and is not mailed.
func TestDispatcher_Handle_CaseEscalated_DeescalationRejected(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	if err := d.Handle(context.Background(), caseEscalatedRecord(2, 0)); err == nil {
		t.Fatal("a de-escalation payload must fail validation")
	}
	if len(mock.calls) != 0 {
		t.Errorf("sent %d emails for a de-escalation", len(mock.calls))
	}
}

func TestDispatcher_Handle_CaseEscalated_SendFailureRetries(t *testing.T) {
	mock := &mockEmailSender{err: errors.New("smtp down")}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	if err := d.Handle(context.Background(), caseEscalatedRecord(0, 1)); err == nil {
		t.Fatal("a failed send must return an error so the consumer retries")
	}
}

func TestDispatcher_Handle_CaseEscalated_DisabledSendsNothing(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, false, false, nil, true, "", nil)
	if err := d.Handle(context.Background(), caseEscalatedRecord(0, 1)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Errorf("sent %d emails with sending disabled", len(mock.calls))
	}
}
