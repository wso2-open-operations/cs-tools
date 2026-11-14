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
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

const incidentCreatedWithTeam = `{"type":"incident.created","entityId":"inc-1","payload":{"title":"ContainerRecentlyOOMKilled","shortDescription":"ContainerRecentlyOOMKilled","callTo":"+15551234567","number":"INC0088718","priority":"CRITICAL","team":"Artemis SRE Group","category":"SERVICE_INTERRUPTION","state":"NEW","service":"wso2-cloud","environment":"central"}}`

const incidentAssignedWithTeam = `{"type":"incident.assigned","entityId":"inc-1","payload":{"assigneeId":"u-1","assigneeName":"Jane Doe","number":"INC0088718","team":"Artemis SRE Group","updatedBy":"jane.doe@example.com"}}`

func artemisSpace(audience string) bool {
	return audience == notifications.IncidentAudience("Artemis SRE Group")
}

func TestDispatcher_Handle_IncidentCreated_PostsCardToTheGroupSpace(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: artemisSpace}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, true, false, nil, true, "", nil)

	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(incidentCreatedWithTeam)}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(call.calls) != 1 {
		t.Errorf("calls = %d, want the call still placed", len(call.calls))
	}
	if len(chat.incidentCreatedCalls) != 1 {
		t.Fatalf("incident cards = %d, want 1", len(chat.incidentCreatedCalls))
	}
	got := chat.incidentCreatedCalls[0]
	want := notifications.IncidentCreatedAlert{
		IncidentID: "inc-1", Number: "INC0088718", Team: "Artemis SRE Group", Priority: "CRITICAL",
		ShortDescription: "ContainerRecentlyOOMKilled", Service: "wso2-cloud", Environment: "central",
		Category: "SERVICE_INTERRUPTION", State: "NEW", IncidentLink: "https://csm.example/operations/incidents/inc-1",
	}
	if got.audience != notifications.IncidentAudience("Artemis SRE Group") || got.alert != want {
		t.Errorf("card = %+v", got)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

func TestDispatcher_Handle_IncidentCreated_NoCardWithoutAGroupSpace(t *testing.T) {
	for name, body := range map[string]string{
		"group not configured": incidentCreatedWithTeam,
		"no group at all":      validIncidentRecord,
	} {
		t.Run(name, func(t *testing.T) {
			chat := &mockGoogleChatSender{}
			call := &mockCallSender{}
			d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, true, false, nil, true, "", nil)
			if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(body)}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(chat.incidentCreatedCalls) != 0 || len(call.calls) != 1 {
				t.Errorf("cards = %d, calls = %d; want 0 and 1", len(chat.incidentCreatedCalls), len(call.calls))
			}
		})
	}
}

func TestDispatcher_Handle_IncidentCreated_ChatRetryDoesNotRepeatTheCall(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: artemisSpace, err: errors.New("chat down")}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, true, false, nil, true, "", nil)
	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 7, Value: []byte(incidentCreatedWithTeam)}

	for attempt := 1; attempt <= 3; attempt++ {
		record.NoMoreRetries = attempt == 3
		if err := d.Handle(context.Background(), record); err == nil {
			t.Fatalf("attempt %d: expected the chat error to propagate", attempt)
		}
	}
	if len(call.calls) != 1 {
		t.Errorf("calls = %d across 3 attempts, want 1 (a succeeded call is not repeated)", len(call.calls))
	}
	if len(chat.incidentCreatedCalls) != 3 {
		t.Errorf("card attempts = %d, want 3 (the failing channel keeps retrying)", len(chat.incidentCreatedCalls))
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the final attempt, has %d entries", len(d.done))
	}
}

func TestDispatcher_Handle_IncidentAssigned_RepliesInTheGroupSpace(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: artemisSpace}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil)

	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(incidentAssignedWithTeam)}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.incidentAssignedCalls) != 1 {
		t.Fatalf("replies = %d, want 1", len(chat.incidentAssignedCalls))
	}
	want := notifications.IncidentAssignedAlert{IncidentID: "inc-1", Number: "INC0088718", AssigneeName: "Jane Doe \U0001F3A7", UpdatedBy: "jane.doe@example.com",
		IncidentLink: "https://csm.example/operations/incidents/inc-1"}
	if got := chat.incidentAssignedCalls[0]; got.alert != want || got.audience != notifications.IncidentAudience("Artemis SRE Group") {
		t.Errorf("reply = %+v", got)
	}
}

func TestDispatcher_Handle_IncidentCreated_NoNumberStillPostsTheCard(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: artemisSpace}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, false, "", nil)
	record := eventbus.Record{Value: []byte(`{"type":"incident.created","entityId":"inc-1","payload":{"title":"t","shortDescription":"t","team":"Artemis SRE Group"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.incidentCreatedCalls) != 1 {
		t.Errorf("cards = %d, want 1 even without a number", len(chat.incidentCreatedCalls))
	}
}

func TestDispatcher_Handle_IncidentAssigned_OlderPublisherPostsNothing(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: artemisSpace}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil)
	record := eventbus.Record{Value: []byte(`{"type":"incident.assigned","entityId":"inc-1","payload":{"assigneeId":"u-1"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.incidentAssignedCalls) != 0 {
		t.Errorf("replies = %d, want 0 without a number", len(chat.incidentAssignedCalls))
	}
}
