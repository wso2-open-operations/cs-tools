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

package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// newTestCreateIncidentClient stubs the POST /incidents create call
// publishIncidentCreated triggers after, and nothing else — so the enrichment
// GET it also makes fails, and the event goes out carrying only the fields it
// had before the escalation ladder existed. That is the point here: these
// tests cover that older contract. The enrichment itself, and its own
// failure path, are covered in sn_incident_escalation_publish_test.go.
func newTestCreateIncidentClient(t *testing.T, incidentSysid string) *integrationservice.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident created successfully.",
			"incident": {"id": "` + incidentSysid + `", "number": "INC0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}
		}`))
	})
	return newTestSNClient(t, mux)
}

// TestSNIncidentService_CreateIncident_PublishesIncidentCreated verifies the
// happy path: Title/ShortDescription are built from req, with no enrichment
// call needed and no portal link involved — csm-notification-service builds
// the "Open in Portal" link itself from the event's EntityID, the same way
// it already does for case.created.
func TestSNIncidentService_CreateIncident_PublishesIncidentCreated(t *testing.T) {
	client := newTestCreateIncidentClient(t, testIncidentSysid)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	req := validCreateIncidentRequest()
	req.Subject = "Payment gateway down"
	comments := "Customers cannot check out"
	req.AdditionalComments = &comments

	resp, err := svc.CreateIncident(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeIncidentCreated {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeIncidentCreated)
	}
	if call.entityID != resp.Incident.ID {
		t.Errorf("entityID = %q, want the new incident's id %q", call.entityID, resp.Incident.ID)
	}

	var payload events.IncidentCreatedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.Title != "Payment gateway down" {
		t.Errorf("title = %q, want %q", payload.Title, "Payment gateway down")
	}
	if payload.ShortDescription != "Customers cannot check out" {
		t.Errorf("shortDescription = %q, want %q", payload.ShortDescription, "Customers cannot check out")
	}
}

// TestSNIncidentService_CreateIncident_ShortDescriptionFallsBackToSubject
// verifies that an incident created with no AdditionalComments still
// publishes a non-empty ShortDescription — required by
// csm-notification-service's events.Validate — by falling back to Subject.
func TestSNIncidentService_CreateIncident_ShortDescriptionFallsBackToSubject(t *testing.T) {
	client := newTestCreateIncidentClient(t, testIncidentSysid)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	req := validCreateIncidentRequest()
	req.Subject = "Payment gateway down"

	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	var payload events.IncidentCreatedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.ShortDescription != "Payment gateway down" {
		t.Errorf("shortDescription = %q, want it to fall back to the subject %q", payload.ShortDescription, "Payment gateway down")
	}
}

// TestSNIncidentService_CreateIncident_NoPublisherConfigured verifies that a
// nil publisher is a silent no-op, not a panic.
func TestSNIncidentService_CreateIncident_NoPublisherConfigured(t *testing.T) {
	client := newTestCreateIncidentClient(t, testIncidentSysid)
	svc := NewServiceNowIncidentService(client, nil)

	req := validCreateIncidentRequest()
	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSNIncidentService_CreateIncident_PublishFailureDoesNotFailCreateIncident
// verifies that a Publish error is logged, not returned — the incident
// already exists in ServiceNow by that point.
func TestSNIncidentService_CreateIncident_PublishFailureDoesNotFailCreateIncident(t *testing.T) {
	client := newTestCreateIncidentClient(t, testIncidentSysid)
	publisher := &mockEventPublisher{err: errors.New("event hub unreachable")}
	svc := NewServiceNowIncidentService(client, publisher)

	req := validCreateIncidentRequest()
	resp, err := svc.CreateIncident(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("CreateIncident must succeed even when publishing fails, got: %v", err)
	}
	if resp.Incident.Number != "INC0001" {
		t.Fatalf("unexpected incident number: %s", resp.Incident.Number)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("expected the publish attempt to still happen, got %d calls", len(publisher.calls))
	}
}
