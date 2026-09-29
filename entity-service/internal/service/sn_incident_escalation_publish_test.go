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
	"net/http"
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// The publisher half of the incident call-escalation ladder.
//
// csm-notification-service's engine is tested against events; these test that
// this service actually produces them, and produces them with the fields the
// ladder cannot run without — the priority that keys the whole timing table,
// the team that selects recipients, and the report time every call is an
// offset from.

// incidentEnrichmentBody is the GET /incidents/{id} response
// publishIncidentCreated reads the escalation fields from. ServiceNow derives
// priority from impact and urgency, so neither the create request nor its
// response carries it — this read is the only place it exists.
// The GET returns the incident object directly, not wrapped — see
// snGetIncidentResponse.
const incidentEnrichmentBody = `{
	"id": "` + testIncidentSysid + `",
	"number": "INC0001",
	"openedOn": "2026-09-09 04:30:00",
	"subject": "Gateway returning 500s",
	"priority": {"id": 1, "label": "1 - Critical"},
	"state": {"id": 1, "label": "New"},
	"assignmentGroup": {"id": "` + testIncidentSysid + `", "name": "Atlas"}
}`

// newTestIncidentEnrichmentClient stubs both calls the create path makes: the
// POST that creates the incident and the GET that resolves its priority.
func newTestIncidentEnrichmentClient(t *testing.T, enrichment string, enrichmentStatus int) *integrationservice.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		if enrichmentStatus != http.StatusOK {
			w.WriteHeader(enrichmentStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(enrichment))
	})
	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident created successfully.",
			"incident": {"id": "` + testIncidentSysid + `", "number": "INC0042", "createdOn": "2026-09-09 04:30:00", "createdBy": "engineer@example.com"}
		}`))
	})
	return newTestSNClient(t, mux)
}

// findPublished returns the first published event of a type.
func findPublished(t *testing.T, calls []mockPublishCall, typ events.Type) mockPublishCall {
	t.Helper()
	call, ok := findPublishCall(calls, typ)
	if !ok {
		t.Fatalf("no %s published; got %v", typ, publishedTypes(calls))
	}
	return call
}

// Without the priority the ladder has no timing table to key on, so this is
// the field the whole feature rests on.
func TestPublishIncidentCreated_CarriesTheEscalationFields(t *testing.T) {
	client := newTestIncidentEnrichmentClient(t, incidentEnrichmentBody, http.StatusOK)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), validCreateIncidentRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentCreated).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.Priority != "CRITICAL" {
		t.Errorf("priority = %q, want CRITICAL — the ladder keys its whole timing table on this", p.Priority)
	}
	if p.Team != "Atlas" {
		t.Errorf("team = %q, want Atlas — section 5.0 routes on the assigned team", p.Team)
	}
	if p.Number != "INC0042" {
		t.Errorf("number = %q, want INC0042", p.Number)
	}
	// openedOn, not createdOn: the ladder's offsets run from when the
	// incident was reported.
	if p.ReportedAt != "2026-09-09T04:30:00Z" {
		t.Errorf("reportedAt = %q, want the incident's openedOn in RFC3339", p.ReportedAt)
	}
}

// The enrichment read is best-effort. If it fails the event must still go out
// with what it always carried — losing the Chat alert and the direct call
// would be strictly worse than losing the ladder.
func TestPublishIncidentCreated_EnrichmentFailureStillPublishes(t *testing.T) {
	client := newTestIncidentEnrichmentClient(t, "", http.StatusInternalServerError)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), validCreateIncidentRequest()); err != nil {
		t.Fatalf("a failed enrichment must not fail CreateIncident: %v", err)
	}

	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentCreated).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.Title == "" || p.ShortDescription == "" {
		t.Error("the event lost the fields it carried before the ladder existed")
	}
	if p.Priority != "" {
		t.Errorf("priority = %q; a failed read must publish nothing rather than a guess", p.Priority)
	}
}

// Account and ABT eligibility are declared on the payload but have no source
// in this domain model. Publishing a fabricated value would be worse than
// publishing none: the consumer routes on ABT eligibility.
func TestPublishIncidentCreated_LeavesUnsourcedFieldsEmpty(t *testing.T) {
	client := newTestIncidentEnrichmentClient(t, incidentEnrichmentBody, http.StatusOK)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), validCreateIncidentRequest()); err != nil {
		t.Fatal(err)
	}
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentCreated).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.Account != "" {
		t.Errorf("account = %q; incidents have no account field to read one from", p.Account)
	}
	// Absent, not false: this service has no product-to-BU mapping to derive
	// it from, and sending false would claim an answer nobody gave. The
	// consumer treats an absence as "unknown" and says so rather than routing
	// as though someone had decided.
	if p.ABTEligible != nil {
		t.Errorf("abtEligible = %v; it must be absent, not an answer", *p.ABTEligible)
	}
}

// newTestIncidentCommentClient stubs the generic comment create path.
func newTestIncidentCommentClient(t *testing.T) *integrationservice.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Comment created successfully.",
			"comment": {"id": "` + testIncidentSysid + `", "createdOn": "2026-09-09 05:00:00", "createdBy": "engineer@example.com"}
		}`))
	})
	return newTestSNClient(t, mux)
}

func incidentCommentRequest(commentType domain.CommentType) domain.CreateCommentRequest {
	return domain.CreateCommentRequest{
		ReferenceID:   testIncidentUUID,
		ReferenceType: domain.ReferenceTypeIncident,
		Type:          commentType,
		Content:       "Looking into this now",
	}
}

// A public comment is the specification's acknowledgement gesture for a
// priority elevation, and the only stop signal such a ladder has — an
// elevated incident has normally already left NEW, so incident.acknowledged
// can never fire for it again.
func TestPublishIncidentCommentAdded_PublicComment(t *testing.T) {
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCommentService(newTestIncidentCommentClient(t), publisher)

	if _, err := svc.CreateComment(contextWithUserIDToken("token"), incidentCommentRequest(domain.CommentTypeComment)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := findPublished(t, publisher.calls, events.TypeIncidentCommentAdded)
	if call.entityID != testIncidentUUID {
		t.Errorf("entityId = %q, want the incident's id so the consumer can find its ladder", call.entityID)
	}
	var p events.IncidentCommentAddedPayload
	if err := json.Unmarshal(call.payload, &p); err != nil {
		t.Fatal(err)
	}
	if !p.IsPublic {
		t.Error("a public comment must be published as public, or it will not stop a ladder")
	}
	if p.CommentID == "" {
		t.Error("commentId is empty; the execution summary records which comment stopped the ladder")
	}
}

// A work note is an internal jotting, not an acknowledgement. It is still
// published — the consumer decides, keeping the event a statement of fact —
// but flagged so it stops nothing.
func TestPublishIncidentCommentAdded_WorkNoteIsNotPublic(t *testing.T) {
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCommentService(newTestIncidentCommentClient(t), publisher)

	if _, err := svc.CreateComment(contextWithUserIDToken("token"), incidentCommentRequest(domain.CommentTypeWorkNote)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var p events.IncidentCommentAddedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentCommentAdded).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.IsPublic {
		t.Error("a work note must not be published as public; triaging would silence the pager")
	}
}

// A comment on anything else is none of the ladder's business.
func TestPublishIncidentCommentAdded_OnlyForIncidents(t *testing.T) {
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCommentService(newTestIncidentCommentClient(t), publisher)

	req := incidentCommentRequest(domain.CommentTypeComment)
	req.ReferenceType = domain.ReferenceTypeChangeRequest
	if _, err := svc.CreateComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, published := findPublishCall(publisher.calls, events.TypeIncidentCommentAdded); published {
		t.Error("a change-request comment published an incident event")
	}
}

// A deployment without an event bus still creates comments.
func TestPublishIncidentCommentAdded_NoPublisherConfigured(t *testing.T) {
	svc := NewServiceNowCommentService(newTestIncidentCommentClient(t), nil)
	if _, err := svc.CreateComment(contextWithUserIDToken("token"), incidentCommentRequest(domain.CommentTypeComment)); err != nil {
		t.Fatalf("a nil publisher must not fail CreateComment: %v", err)
	}
}

// newTestIncidentElevationClient stubs the baseline GET and the PATCH that an
// elevation makes: UpdateIncident reads the incident as it was before the
// change, so it can tell a genuine elevation from a no-op re-PATCH.
func newTestIncidentElevationClient(t *testing.T, beforePriority, afterPriority int) *integrationservice.Client {
	t.Helper()
	body := func(priority int) string {
		return `{
			"id": "` + testIncidentSysid + `",
			"number": "INC0042",
			"openedOn": "2026-09-09 04:30:00",
			"subject": "Gateway returning 500s",
			"priority": {"id": ` + strconv.Itoa(priority) + `, "label": "priority"},
			"state": {"id": 2, "label": "In Progress"},
			"assignmentGroup": {"id": "` + testIncidentSysid + `", "name": "Atlas"}
		}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			// The update response wraps the incident; the GET returns it
			// bare. Getting this wrong makes the baseline comparison read a
			// zero-valued incident and publish nothing at all.
			_, _ = w.Write([]byte(`{"message":"Incident updated successfully.","incident":` + body(afterPriority) + `}`))
			return
		}
		_, _ = w.Write([]byte(body(beforePriority)))
	})
	return newTestSNClient(t, mux)
}

// An elevation is the ladder's second trigger, and it carries the same
// recipient-selection fields the created event does — from the post-PATCH
// incident, so no extra read is needed.
func TestPublishIncidentPriorityElevated_CarriesTheEscalationFields(t *testing.T) {
	// 3 (Moderate) to 1 (Critical): a strict increase in urgency.
	client := newTestIncidentElevationClient(t, 3, 1)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	priority := domain.IncidentPriorityCritical
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:       testIncidentUUID,
		Priority: &priority,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p events.IncidentPriorityElevatedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentPriorityElevated).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.NewPriority != "CRITICAL" {
		t.Errorf("newPriority = %q, want CRITICAL — the ladder keys its timings on this one", p.NewPriority)
	}
	if p.OldPriority != "MODERATE" {
		t.Errorf("oldPriority = %q, want MODERATE", p.OldPriority)
	}
	if p.Number != "INC0042" {
		t.Errorf("number = %q, want INC0042", p.Number)
	}
	if p.Team != "Atlas" {
		t.Errorf("team = %q, want Atlas", p.Team)
	}
	if p.ElevatedAt == "" {
		t.Error("elevatedAt is empty; the ladder measures every call from it")
	}
}

// A downgrade is not an elevation, and must start nothing.
func TestPublishIncidentPriorityElevated_DowngradePublishesNothing(t *testing.T) {
	// 1 (Critical) down to 3 (Moderate).
	client := newTestIncidentElevationClient(t, 1, 3)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	priority := domain.IncidentPriorityModerate
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:       testIncidentUUID,
		Priority: &priority,
	}); err != nil {
		t.Fatal(err)
	}
	if _, published := findPublishCall(publisher.calls, events.TypeIncidentPriorityElevated); published {
		t.Error("a downgrade started a ladder")
	}
}

// ServiceNow derives priority from impact and urgency — CreateIncident
// requires both and accepts no priority at all. An update that raises urgency
// therefore raises the priority just as surely as one naming it, and gating
// the elevation check on req.Priority alone meant that update published
// nothing and no ladder ever started.
func TestPublishIncidentPriorityElevated_RaisedByUrgencyAlone(t *testing.T) {
	client := newTestIncidentElevationClient(t, 3, 1)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	urgency := domain.IncidentUrgencyHigh
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:      testIncidentUUID,
		Urgency: &urgency,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p events.IncidentPriorityElevatedPayload
	if err := json.Unmarshal(findPublished(t, publisher.calls, events.TypeIncidentPriorityElevated).payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.NewPriority != "CRITICAL" {
		t.Errorf("newPriority = %q, want CRITICAL", p.NewPriority)
	}
}

// The same for impact, the other half of the derivation.
func TestPublishIncidentPriorityElevated_RaisedByImpactAlone(t *testing.T) {
	client := newTestIncidentElevationClient(t, 3, 1)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	impact := domain.IncidentImpactHigh
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:     testIncidentUUID,
		Impact: &impact,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, published := findPublishCall(publisher.calls, events.TypeIncidentPriorityElevated); !published {
		t.Error("an impact change that raised the priority published nothing")
	}
}

// An impact or urgency change that leaves the derived priority alone must
// still publish nothing — the comparison is against the real priorities, not
// against which fields the request happened to name.
func TestPublishIncidentPriorityElevated_UrgencyWithoutElevationPublishesNothing(t *testing.T) {
	client := newTestIncidentElevationClient(t, 2, 2) // priority unchanged
	publisher := &mockEventPublisher{}
	svc := NewServiceNowIncidentService(client, publisher)

	urgency := domain.IncidentUrgencyHigh
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:      testIncidentUUID,
		Urgency: &urgency,
	}); err != nil {
		t.Fatal(err)
	}
	if _, published := findPublishCall(publisher.calls, events.TypeIncidentPriorityElevated); published {
		t.Error("an urgency change that did not raise the priority started a ladder")
	}
}

// A PATCH touching none of state, priority, impact or urgency must not pay
// for the baseline read at all.
func TestUpdateIncident_UnrelatedPatchSkipsTheBaselineFetch(t *testing.T) {
	var gets int
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			_, _ = w.Write([]byte(`{"message":"ok","incident":{"id":"` + testIncidentSysid + `","number":"INC0042"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + testIncidentSysid + `","number":"INC0042"}`))
	})
	svc := NewServiceNowIncidentService(newTestSNClient(t, mux), &mockEventPublisher{})

	subject := "A clearer subject"
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:      testIncidentUUID,
		Subject: &subject,
	}); err != nil {
		t.Fatal(err)
	}
	if gets != 0 {
		t.Errorf("a subject-only PATCH made %d baseline read(s); it should make none", gets)
	}
}
