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

package caseevents

import (
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/eventbus"
)

func TestHandle_ValidRecord_ReturnsNil(t *testing.T) {
	h := NewHandler(nil)
	record := eventbus.Record{
		Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"caseComment":"hello"}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
}

func TestHandle_MalformedRecord_ReturnsNilNotError(t *testing.T) {
	h := NewHandler(nil)
	record := eventbus.Record{Value: []byte("not json")}
	// A malformed record is logged and dropped, not retried — see Handle's
	// doc comment — so this must not return an error.
	if err := h.Handle(t.Context(), record); err != nil {
		t.Errorf("Handle() error = %v, want nil for a malformed record", err)
	}
}

func TestHandle_EmptyRecord_ReturnsNilNotError(t *testing.T) {
	h := NewHandler(nil)
	if err := h.Handle(t.Context(), eventbus.Record{}); err != nil {
		t.Errorf("Handle() error = %v, want nil for an empty record", err)
	}
}

type publishCall struct {
	caseID  string
	payload string
}

type mockHub struct {
	calls []publishCall
}

func (m *mockHub) Publish(caseID, payload string) {
	m.calls = append(m.calls, publishCall{caseID: caseID, payload: payload})
}

func TestHandle_CommentAdded_BroadcastsToHub(t *testing.T) {
	hub := &mockHub{}
	h := NewHandler(hub)
	record := eventbus.Record{
		Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"timestamp":"2026-08-13T00:00:00Z"}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(hub.calls) != 1 {
		t.Fatalf("hub.Publish called %d times, want 1", len(hub.calls))
	}
	if hub.calls[0].caseID != "CASE-1" {
		t.Errorf("Publish caseID = %q, want %q", hub.calls[0].caseID, "CASE-1")
	}
	if !strings.Contains(hub.calls[0].payload, `"caseId":"CASE-1"`) ||
		!strings.Contains(hub.calls[0].payload, `"type":"case.comment_added"`) {
		t.Errorf("Publish payload = %q, missing expected fields", hub.calls[0].payload)
	}
}

func TestHandle_StatusChanged_BroadcastsToHub(t *testing.T) {
	hub := &mockHub{}
	h := NewHandler(hub)
	record := eventbus.Record{
		Value: []byte(`{"type":"case.status_changed","entityId":"CASE-2","payload":{"timestamp":"2026-08-13T00:00:00Z","newStatus":"resolved"}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(hub.calls) != 1 {
		t.Fatalf("hub.Publish called %d times, want 1", len(hub.calls))
	}
	if hub.calls[0].caseID != "CASE-2" {
		t.Errorf("Publish caseID = %q, want %q", hub.calls[0].caseID, "CASE-2")
	}
}

func TestHandle_AssignedSeverityAcknowledged_NotBroadcast(t *testing.T) {
	for _, typ := range []string{"case.assigned", "case.severity_changed", "case.acknowledged"} {
		t.Run(typ, func(t *testing.T) {
			hub := &mockHub{}
			h := NewHandler(hub)
			record := eventbus.Record{
				Value: []byte(`{"type":"` + typ + `","entityId":"CASE-5","payload":{"timestamp":"2026-08-13T00:00:00Z","newSeverity":"S1"}}`),
			}
			if err := h.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error = %v, want nil", err)
			}
			if len(hub.calls) != 0 {
				t.Fatalf("hub.Publish called %d times, want 0", len(hub.calls))
			}
		})
	}
}

func TestHandle_OtherEventType_DoesNotBroadcast(t *testing.T) {
	for _, typ := range []string{"case.created", "incident.created"} {
		t.Run(typ, func(t *testing.T) {
			hub := &mockHub{}
			h := NewHandler(hub)
			record := eventbus.Record{
				Value: []byte(`{"type":"` + typ + `","entityId":"CASE-3","payload":{}}`),
			}
			if err := h.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error = %v, want nil", err)
			}
			if len(hub.calls) != 0 {
				t.Errorf("hub.Publish called %d times, want 0 for %s", len(hub.calls), typ)
			}
		})
	}
}

func TestHandle_NilHub_DoesNotPanic(t *testing.T) {
	h := NewHandler(nil)
	record := eventbus.Record{
		Value: []byte(`{"type":"case.comment_added","entityId":"CASE-4","payload":{}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
}

func TestHandle_CommentAdded_InternalNoteNotBroadcast(t *testing.T) {
	hub := &mockHub{}
	h := NewHandler(hub)
	record := eventbus.Record{
		Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"timestamp":"2026-08-13T00:00:00Z","isInternalNote":true}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(hub.calls) != 0 {
		t.Fatalf("hub.Publish called %d times for an internal note, want 0", len(hub.calls))
	}
}

func TestHandle_CommentAdded_NotInternalOrAbsentFlagBroadcast(t *testing.T) {
	cases := map[string]string{
		"explicit false": `{"timestamp":"2026-08-13T00:00:00Z","isInternalNote":false}`,
		"absent field":   `{"timestamp":"2026-08-13T00:00:00Z"}`,
		"empty payload":  `{}`,
		"null payload":   `null`,
		"malformed flag": `{"timestamp":"2026-08-13T00:00:00Z","isInternalNote":"yes"}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			hub := &mockHub{}
			h := NewHandler(hub)
			record := eventbus.Record{
				Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":` + payload + `}`),
			}
			if err := h.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error = %v, want nil", err)
			}
			if len(hub.calls) != 1 {
				t.Fatalf("hub.Publish called %d times, want 1", len(hub.calls))
			}
		})
	}
}

func TestHandle_StatusChangedIgnoresInternalNoteFlag(t *testing.T) {
	for _, typ := range []string{"case.status_changed"} {
		t.Run(typ, func(t *testing.T) {
			hub := &mockHub{}
			h := NewHandler(hub)
			record := eventbus.Record{
				Value: []byte(`{"type":"` + typ + `","entityId":"CASE-6","payload":{"timestamp":"2026-08-13T00:00:00Z","isInternalNote":true}}`),
			}
			if err := h.Handle(t.Context(), record); err != nil {
				t.Fatalf("Handle() error = %v, want nil", err)
			}
			if len(hub.calls) != 1 {
				t.Fatalf("hub.Publish called %d times, want 1", len(hub.calls))
			}
		})
	}
}
