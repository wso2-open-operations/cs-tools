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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

func TestDispatcher_Handle_IncidentCreated_RegistersSLAClocks(t *testing.T) {
	sla := &mockSLAEngine{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithSLAEngine(sla)

	record := eventbus.Record{Value: []byte(`{"type":"incident.created","entityId":"INC-ID-1","payload":{"title":"Router 5xx","shortDescription":"desc","callTo":"","number":"INC0091536","priority":"CRITICAL","team":"Artemis SRE Group","service":"Choreo","reportedAt":"2026-11-03T07:19:53Z"}}`)}
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(sla.incidentCalls) != 1 {
		t.Fatalf("RegisterIncidentClocks calls = %d, want 1", len(sla.incidentCalls))
	}
	got := sla.incidentCalls[0]
	want := mockIncidentClockCall{"INC-ID-1", "CRITICAL", "INC0091536", "Router 5xx", "Artemis SRE Group", "Choreo", time.Date(2026, 11, 3, 7, 19, 53, 0, time.UTC)}
	if got.incidentID != want.incidentID || got.priority != want.priority || got.number != want.number || got.title != want.title ||
		got.assignmentGroup != want.assignmentGroup || got.service != want.service || !got.createdAt.Equal(want.createdAt) {
		t.Errorf("RegisterIncidentClocks args = %+v, want %+v", got, want)
	}
	if len(sla.registerCalls) != 0 {
		t.Errorf("RegisterClocks (case) calls = %d, want none for an incident", len(sla.registerCalls))
	}
}

func TestDispatcher_Handle_IncidentPriorityElevated_RegistersSLAClocks(t *testing.T) {
	sla := &mockSLAEngine{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithSLAEngine(sla)

	record := eventbus.Record{Value: []byte(`{"type":"incident.priority_elevated","entityId":"INC-ID-2","payload":{"oldPriority":"MODERATE","newPriority":"HIGH","title":"Latency","number":"INC0094000","team":"Apollo SRE Group","elevatedAt":"2026-11-10T09:00:00Z"}}`)}
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(sla.incidentCalls) != 1 {
		t.Fatalf("RegisterIncidentClocks calls = %d, want 1", len(sla.incidentCalls))
	}
	got := sla.incidentCalls[0]
	if got.incidentID != "INC-ID-2" || got.priority != "HIGH" || got.number != "INC0094000" || got.assignmentGroup != "Apollo SRE Group" {
		t.Errorf("RegisterIncidentClocks args = %+v, want the elevated incident's id, new priority, number and group", got)
	}
}

// The event belongs to the paging ladder: with no SLA engine it is still
// acknowledged, as before.
func TestDispatcher_Handle_IncidentPriorityElevated_NoSLAEngine(t *testing.T) {
	record := eventbus.Record{Value: []byte(`{"type":"incident.priority_elevated","entityId":"INC-ID-4","payload":{"oldPriority":"LOW","newPriority":"CRITICAL"}}`)}
	if err := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).Handle(context.Background(), record); err != nil {
		t.Errorf("Handle() with no SLA engine error = %v, want nil", err)
	}
}

// Registration reads clocks from entity-service (and retries when none is
// found), so it runs after the page: the on-call call is placed first.
func TestDispatcher_Handle_IncidentCreated_PagesBeforeRegisteringSLAClocks(t *testing.T) {
	call := &mockCallSender{}
	sla := &mockSLAEngine{}
	callsAtRegistration := -1
	sla.onIncidentRegister = func() {
		call.mu.Lock()
		callsAtRegistration = len(call.calls)
		call.mu.Unlock()
	}
	d := NewDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, call, &mockLinkResolver{}, true, false, nil, true, "", nil).WithSLAEngine(sla)

	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(validIncidentRecord)}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if callsAtRegistration != 1 {
		t.Errorf("calls placed when SLA registration began = %d, want 1 (page first)", callsAtRegistration)
	}
}
