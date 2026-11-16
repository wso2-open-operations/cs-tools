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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

func announcementRecord(eventType, caseType string) eventbus.Record {
	var payload string
	if eventType == "case.created" {
		payload = fmt.Sprintf(`{"reporterName":"Jane Doe","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Planned maintenance","caseType":%q,"createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}`, caseType)
	} else {
		payload = fmt.Sprintf(`{"name":"Jane Doe","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Planned maintenance","caseComment":"update","commentId":"C-1","caseType":%q,"recipients":["test-recipient@example.com"]}`, caseType)
	}
	return eventbus.Record{Value: []byte(fmt.Sprintf(`{"type":%q,"entityId":"CASE-1","payload":%s}`, eventType, payload))}
}

func TestDispatcher_Handle_AnnouncementEmailsHideAuthorName(t *testing.T) {
	for _, eventType := range []string{"case.created", "case.comment_added"} {
		for _, ct := range []string{"ANNOUNCEMENT", "announcement"} {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
			if err := d.Handle(context.Background(), announcementRecord(eventType, ct)); err != nil {
				t.Fatalf("%s/%s: Handle() error = %v", eventType, ct, err)
			}
			if len(mock.calls) != 1 {
				t.Fatalf("%s/%s: %d emails, want 1", eventType, ct, len(mock.calls))
			}
			if strings.Contains(mock.calls[0].htmlBody, "Jane Doe") || strings.Contains(mock.calls[0].subject, "Jane Doe") {
				t.Errorf("%s/%s: email shows the author name", eventType, ct)
			}
		}
	}
}

func TestDispatcher_Handle_NonAnnouncementEmailsKeepAuthorName(t *testing.T) {
	for _, eventType := range []string{"case.created", "case.comment_added"} {
		for _, ct := range []string{"CASE", ""} {
			if eventType == "case.created" && ct == "" {
				continue // Validate rejects an empty caseType on case.created
			}
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
			if err := d.Handle(context.Background(), announcementRecord(eventType, ct)); err != nil {
				t.Fatalf("%s/%q: Handle() error = %v", eventType, ct, err)
			}
			if len(mock.calls) != 1 || !strings.Contains(mock.calls[0].htmlBody, "Jane Doe") {
				t.Errorf("%s/%q: author name missing from the email", eventType, ct)
			}
		}
	}
}
