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
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	childNewID    = "c1000000-0000-4000-8000-000000000001"
	childOnHoldID = "c1000000-0000-4000-8000-000000000002"
)

// resolveWithChildrenRepo resolves the parent (In Progress before, Resolved after) and reports
// the cascade resolved two children: one that was New with no code, one On Hold that already
// carried the parent's code.
func resolveWithChildrenRepo() *stubIncidentRepo {
	before := incidentViewWith("IN_PROGRESS", nil)
	after := incidentViewWith("RESOLVED", nil)
	after.ResolutionCode = strPtr("SOLVED_PERMANENTLY")
	repo := updateRecordingRepo(before, after)
	repo.resolvedChildren = []repository.ResolvedChildIncident{
		{ID: childNewID, Number: "INC0000101", PreviousState: "NEW", ResolutionCode: strPtr("SOLVED_PERMANENTLY")},
		{ID: childOnHoldID, Number: "INC0000102", PreviousState: "ON_HOLD",
			PreviousResolutionCode: strPtr("SOLVED_PERMANENTLY"), ResolutionCode: strPtr("SOLVED_PERMANENTLY")},
	}
	return repo
}

func resolveRequest() domain.UpdateIncidentRequest {
	state := domain.IncidentStateResolved
	code := domain.IncidentResolutionCodeSolvedPermanently
	notes := "restarted the gateway"
	return domain.UpdateIncidentRequest{ID: testDeploymentUUID, State: &state, ResolutionCode: &code, ResolutionNotes: &notes}
}

// TestIncidentService_UpdateIncident_ChildrenResolvedWithParent: each child the cascade resolved
// gets its own state (and, when it moved, resolution code) activity rows, attributed to the
// parent's actor, and a child that left New gets incident.acknowledged so its paging stops. A
// child that was not New, and the parent that was already In Progress, send none.
func TestIncidentService_UpdateIncident_ChildrenResolvedWithParent(t *testing.T) {
	repo := resolveWithChildrenRepo()
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithPublisher(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, publisher)

	if _, err := svc.UpdateIncident(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), resolveRequest()); err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}

	byIncident := map[string]recordedIncidentChanges{}
	for _, rec := range repo.recordedChanges {
		if rec.actorEmail != "jane.doe@example.com" {
			t.Errorf("activity for %s attributed to %q, want jane.doe@example.com", rec.incidentID, rec.actorEmail)
		}
		byIncident[rec.incidentID] = rec
	}
	if len(repo.recordedChanges) != 3 {
		t.Fatalf("RecordIncidentFieldChanges calls = %d (%+v), want the parent and both children", len(repo.recordedChanges), repo.recordedChanges)
	}
	for id, want := range map[string][]repository.IncidentFieldChange{
		childNewID: {
			{Field: "state", OldValue: "New", NewValue: "Resolved"},
			{Field: "resolution_code", OldValue: "", NewValue: "Solved Permanently"},
		},
		childOnHoldID: {
			{Field: "state", OldValue: "On Hold", NewValue: "Resolved"},
		},
	} {
		if got := byIncident[id].changes; !reflect.DeepEqual(got, want) {
			t.Errorf("child %s activity = %+v, want %+v", id, got, want)
		}
	}

	var acked []string
	for _, c := range publisher.calls {
		if c.eventType != events.TypeIncidentAcknowledged {
			continue
		}
		acked = append(acked, c.entityID)
		var p events.IncidentAcknowledgedPayload
		if err := json.Unmarshal(c.payload, &p); err != nil || p.PreviousState != "NEW" || p.NewState != "RESOLVED" {
			t.Errorf("incident.acknowledged payload = %+v (err %v), want NEW -> RESOLVED", p, err)
		}
	}
	if !reflect.DeepEqual(acked, []string{childNewID}) {
		t.Errorf("incident.acknowledged sent for %v, want only the child that left New (%s)", acked, childNewID)
	}
}

// TestIncidentService_UpdateIncident_ChildrenNotMirrored: in dual-write only the parent's own
// update goes to ServiceNow; ServiceNow's rule resolves its children itself.
func TestIncidentService_UpdateIncident_ChildrenNotMirrored(t *testing.T) {
	repo := resolveWithChildrenRepo()
	mirrored := make(chan string, 4)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrored <- req.ID
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewIncidentServiceWithSNMirror(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, mirror, nil, dispatcher)

	if _, err := svc.UpdateIncident(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), resolveRequest()); err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}
	select {
	case id := <-mirrored:
		if id != testDeploymentUUID {
			t.Errorf("mirrored %s, want the parent %s", id, testDeploymentUUID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the parent's update was never mirrored")
	}
	select {
	case id := <-mirrored:
		t.Errorf("a second mirror write, for %s; children must not be mirrored", id)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestIncidentReport_ResolvedWithParentSkipsFollowUps: a child resolved by its parent's cascade
// still gets its incident report, but neither the workaround problem nor an alert task -- those
// are raised for the parent alone. A resolve of its own (false, or a snapshot recorded before
// migration 0256 without the column) still gets them.
func TestIncidentReport_ResolvedWithParentSkipsFollowUps(t *testing.T) {
	for name, tc := range map[string]struct {
		snapshot      map[string]any
		wantFollowUps bool
	}{
		"resolved with parent":    {map[string]any{"resolved_with_parent": true}, false},
		"resolved on its own":     {map[string]any{"resolved_with_parent": false}, true},
		"snapshot without column": {map[string]any{"state": "RESOLVED"}, true},
		"no snapshot":             {nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			for _, code := range []string{"SOLVED_WORK_AROUND", "FALSE_ALARM"} {
				tx := &fakeIncidentReportTx{src: resolvedOn(postResolutionServiceChoreo, code)}
				c := incidentStateChange("IN_PROGRESS", "RESOLVED", time.Now())
				c.Snapshot = tc.snapshot
				if err := newTestIncidentReportService(time.Now()).HandleChange(context.Background(), tx, c); err != nil {
					t.Fatalf("%s: HandleChange: %v", code, err)
				}
				if _, ok := tx.reports[incidentReportTestID]; !ok {
					t.Errorf("%s: incident report not written", code)
				}
				if got := len(tx.problems)+len(tx.tasks) > 0; got != tc.wantFollowUps {
					t.Errorf("%s: problems=%d tasks=%d, want follow-ups %v", code, len(tx.problems), len(tx.tasks), tc.wantFollowUps)
				}
			}
		})
	}
}

// TestSearchIncidents_ParentIncidentIDs: the Postgres service refuses a parentIncidentIds entry
// that is not a UUID before reaching the repository (whose stub panics), on search and
// aggregate alike; the ServiceNow service refuses the filter outright, having no parent_incident
// filter to map it onto. Neither client is reached.
func TestSearchIncidents_ParentIncidentIDs(t *testing.T) {
	bad := domain.SearchIncidentsFilters{ParentIncidentIDs: []string{"not-a-uuid"}}
	pg := NewIncidentServiceWithPublisher(&stubIncidentRepo{}, nil, nil)
	if _, err := pg.SearchIncidents(context.Background(), domain.SearchIncidentsRequest{Filters: bad}); !isValidationNaming(err, "filters.parentIncidentIds") {
		t.Errorf("postgres search: got %T: %v, want a ValidationError naming filters.parentIncidentIds", err, err)
	}
	if _, err := pg.AggregateIncidents(context.Background(), domain.AggregateIncidentsRequest{Filters: bad, GroupBy: "state"}); !isValidationNaming(err, "filters.parentIncidentIds") {
		t.Errorf("postgres aggregate: got %T: %v, want a ValidationError naming filters.parentIncidentIds", err, err)
	}

	good := domain.SearchIncidentsFilters{ParentIncidentIDs: []string{testDeploymentUUID}}
	sn := NewServiceNowIncidentService(nil, nil)
	if _, err := sn.SearchIncidents(contextWithUserIDToken("token"), domain.SearchIncidentsRequest{Filters: good}); !isValidationNaming(err, "filters.parentIncidentIds") {
		t.Errorf("servicenow search: got %T: %v, want a ValidationError naming filters.parentIncidentIds", err, err)
	}
	if _, err := sn.AggregateIncidents(contextWithUserIDToken("token"), domain.AggregateIncidentsRequest{Filters: good, GroupBy: "state"}); !isValidationNaming(err, "filters.parentIncidentIds") {
		t.Errorf("servicenow aggregate: got %T: %v, want a ValidationError naming filters.parentIncidentIds", err, err)
	}
}

func isValidationNaming(err error, field string) bool {
	var ve *apierror.ValidationError
	return errors.As(err, &ve) && strings.Contains(ve.Msg, field)
}
