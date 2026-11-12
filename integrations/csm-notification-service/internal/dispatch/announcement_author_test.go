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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

func TestWithoutAnnouncementAuthor(t *testing.T) {
	list := []string{"jane.doe@example.com", "john.roe@example.com"}
	tests := []struct {
		name, caseType, author string
		want                   []string
	}{
		{"announcement removes author", "ANNOUNCEMENT", "jane.doe@example.com", []string{"john.roe@example.com"}},
		{"case and whitespace insensitive", " announcement ", "  Jane.Doe@Example.com ", []string{"john.roe@example.com"}},
		{"non-announcement untouched", "CASE", "jane.doe@example.com", list},
		{"empty author is a no-op", "ANNOUNCEMENT", "", list},
		{"blank author is a no-op", "ANNOUNCEMENT", "   ", list},
		{"author not in list", "ANNOUNCEMENT", "nobody@example.com", list},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withoutAnnouncementAuthor(list, tt.caseType, tt.author)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func caseCreatedRecord(caseType, reporterEmail string, recipients ...string) eventbus.Record {
	list := ""
	for i, r := range recipients {
		if i > 0 {
			list += ","
		}
		list += fmt.Sprintf("%q", r)
	}
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Jane Doe","reporterEmail":%q,"projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Planned maintenance","caseType":%q,"createdAt":"2026-01-01","description":"desc","recipients":[%s]}}`, reporterEmail, caseType, list))}
}

func commentAddedRecord(caseType, authorEmail string, recipients ...string) eventbus.Record {
	list := ""
	for i, r := range recipients {
		if i > 0 {
			list += ","
		}
		list += fmt.Sprintf("%q", r)
	}
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Jane Doe","authorEmail":%q,"projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Planned maintenance","caseComment":"update","commentId":"C-1","caseType":%q,"recipients":[%s]}}`, authorEmail, caseType, list))}
}

func sentTo(mock *mockEmailSender) []string {
	var out []string
	for _, c := range mock.calls {
		out = append(out, c.to...)
	}
	return out
}

func TestDispatcher_Handle_CaseCreated_AnnouncementExcludesReporter(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := caseCreatedRecord("ANNOUNCEMENT", " Test-Recipient@Example.com ", testRecipient, "john.roe@example.com")
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{"john.roe@example.com"}) {
		t.Errorf("sent to %v, want only john.roe@example.com", got)
	}
}

func TestDispatcher_Handle_CaseCreated_NonAnnouncementKeepsReporter(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := caseCreatedRecord("SERVICE_REQUEST", testRecipient, testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{testRecipient}) {
		t.Errorf("sent to %v, want [%s]", got, testRecipient)
	}
}

func TestDispatcher_Handle_CaseCreated_AnnouncementEmptyReporterEmailKeepsAll(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := caseCreatedRecord("ANNOUNCEMENT", "", testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{testRecipient}) {
		t.Errorf("sent to %v, want [%s]", got, testRecipient)
	}
}

func TestDispatcher_Handle_CaseCreated_AnnouncementReporterOnlyRecipientSendsNothingNoError(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := caseCreatedRecord("ANNOUNCEMENT", testRecipient, testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(mock.calls) != 0 {
		t.Errorf("sent %d emails, want none", len(mock.calls))
	}
}

func TestDispatcher_Handle_CommentAdded_AnnouncementExcludesAuthor(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := commentAddedRecord("ANNOUNCEMENT", "TEST-RECIPIENT@example.com", testRecipient, "john.roe@example.com")
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{"john.roe@example.com"}) {
		t.Errorf("sent to %v, want only john.roe@example.com", got)
	}
}

func TestDispatcher_Handle_CommentAdded_NonAnnouncementKeepsAuthor(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := commentAddedRecord("CASE", testRecipient, testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{testRecipient}) {
		t.Errorf("sent to %v, want [%s]", got, testRecipient)
	}
}

func TestDispatcher_Handle_CommentAdded_AnnouncementEmptyAuthorEmailKeepsAll(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := commentAddedRecord("ANNOUNCEMENT", "", testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := sentTo(mock); !reflect.DeepEqual(got, []string{testRecipient}) {
		t.Errorf("sent to %v, want [%s]", got, testRecipient)
	}
}

func TestDispatcher_Handle_CommentAdded_AnnouncementAuthorOnlyRecipientSendsNothingNoError(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := commentAddedRecord("ANNOUNCEMENT", testRecipient, testRecipient)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(mock.calls) != 0 {
		t.Errorf("sent %d emails, want none", len(mock.calls))
	}
}
