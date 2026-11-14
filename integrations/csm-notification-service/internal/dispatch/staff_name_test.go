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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

// The circled W a user's name carries in the data never reaches a chat card,
// and a support engineer's name gets the icon instead.
func TestDispatcher_Handle_CaseAcknowledged_StripsTheCircledWAndAddsTheIcon(t *testing.T) {
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","severity":"CRITICAL","product":"api-manager","acknowledgerName":"Jane Doe ⓦ"}}`)}
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.caseAcknowledgedCalls) != 1 {
		t.Fatalf("alerts = %d, want 1", len(chat.caseAcknowledgedCalls))
	}
	if got := chat.caseAcknowledgedCalls[0].acknowledgerName; got != "Jane Doe \U0001F3A7" {
		t.Errorf("acknowledgerName = %q, want the name without the circled W, then the icon", got)
	}
}

// A comment email shows the support engineer icon beside an internal author,
// by user type, never beside a customer (even one with a WSO2 address), and
// never when the type lookup fails. Neither name carries the circled W.
func TestDispatcher_Handle_CommentAdded_IconOnlyForInternalAuthors(t *testing.T) {
	cases := []struct {
		name     string
		lookup   *fakeUserSearcher
		wantIcon bool
	}{
		{"internal author", &fakeUserSearcher{users: []entity.UserRoleInfo{{Email: "author@example.test", UserType: "internal"}}}, true},
		{"customer author", &fakeUserSearcher{users: []entity.UserRoleInfo{{Email: "author@example.test", UserType: "external"}}}, false},
		{"lookup fails", &fakeUserSearcher{err: errors.New("entity-service down")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
			d.WithStaffLookup(NewStaffLookup(c.lookup, time.Minute))
			record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Jane Doe \u24e6","authorEmail":"author@example.test","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`)}
			if err := d.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(mock.calls) != 1 {
				t.Fatalf("emails = %d, want 1", len(mock.calls))
			}
			body := mock.calls[0].htmlBody
			if strings.Contains(body, "&#9446;") || strings.Contains(body, "&#9420;") {
				t.Error("the email still carries the circled W")
			}
			if got := strings.Contains(body, "&#127911;"); got != c.wantIcon {
				t.Errorf("icon present = %v, want %v", got, c.wantIcon)
			}
			if !strings.Contains(body, "Jane Doe") {
				t.Error("the email lost the author's name")
			}
		})
	}
}

// The assignee is a support engineer, and every email about the assignment
// shows the icon in place of the circled W, whoever receives it.
func TestDispatcher_Handle_CaseAssigned_EveryEmailShowsTheIconNotTheCircledW(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.assigned","entityId":"CASE-1","payload":{"assigneeName":"Jane Doe ⓦ","assigneeEmail":"jane.doe@example.com","projectId":"PROJ-1","caseId":"CASE-1","recipients":["customer@example.com","staff@example.com"]}}`)}
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) == 0 {
		t.Fatal("no email sent")
	}
	for i, call := range mock.calls {
		if strings.Contains(call.htmlBody, "&#9446;") || strings.Contains(call.htmlBody, "&#9420;") {
			t.Errorf("email %d still carries the circled W", i)
		}
		if !strings.Contains(call.htmlBody, "&#127911;") {
			t.Errorf("email %d has no support engineer icon", i)
		}
	}
}
