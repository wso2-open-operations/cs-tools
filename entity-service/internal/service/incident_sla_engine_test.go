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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudIncidentPolicyCandidates mirrors the real "Cloud Incident - High
// Priority" sla_policy row exactly (see
// incident_sla_condition_translate_test.go's own real-data constants) --
// the one real policy confirmed to exist for incidents today.
func cloudIncidentPolicyCandidates() map[string][]repository.SLAPolicyRef {
	return map[string][]repository.SLAPolicyRef{
		"RESOLUTION": {
			{
				ID:              "cloud-incident-high-priority",
				Name:            "Cloud Incident - High Priority",
				Target:          "RESOLUTION",
				Source:          "SERVICENOW",
				StartCondition:  realCloudIncidentStartCondition,
				StopCondition:   realCloudIncidentStopCondition,
				CancelCondition: realCloudIncidentCancelCondition,
				WhenToCancel:    "CANCEL_CONDITIONS_ARE_MET",
				WhenToResume:    "RESUME_CONDITIONS_ARE_MET",
			},
		},
	}
}

// choreoServiceID/asgardeoServiceID are the two business_service rows the
// real policy's conditions name -- aliases of snChoreoServiceID/
// snAsgardeoServiceID (sn_incident_service.go, this codebase's one existing
// source of truth for these two ids), not a second hardcoded copy.
const (
	choreoServiceID   = snChoreoServiceID
	asgardeoServiceID = snAsgardeoServiceID
)

func TestSLAEngineService_RegisterIncidentClocks_CriticalChoreoServiceInterruptionMatches(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-1"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-1", &priority)

	want := []string{"incident-1|cloud-incident-high-priority"}
	if len(repo.registered) != len(want) || repo.registered[0] != want[0] {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_HighAsgardeoServiceInterruptionMatches(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-2"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: asgardeoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityHigh
	svc.RegisterIncidentClocks(context.Background(), "incident-2", &priority)

	want := []string{"incident-2|cloud-incident-high-priority"}
	if len(repo.registered) != len(want) || repo.registered[0] != want[0] {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_ModeratePriorityDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-3"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityModerate
	svc.RegisterIncidentClocks(context.Background(), "incident-3", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: Moderate priority is outside priorityIN1,2 (Critical/High)", repo.registered)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_OtherServiceDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-4"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: "11111111-2222-3333-4444-555555555555", // not Choreo or Asgardeo
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-4", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: the policy is scoped to Choreo/Asgardeo only", repo.registered)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_NonServiceInterruptionCategoryDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-5"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "INQUIRY",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-5", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: category=service_interruption is required", repo.registered)
	}
}

func TestSLAEngineService_ApplyIncidentStateEffects_CompletesOnResolved(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.activeByCaseTarget["incident-6|RESOLUTION"] = cloudIncidentPolicyCandidates()["RESOLUTION"][0]
	repo.factsByIncident["incident-6"] = repository.IncidentFacts{
		State:     "RESOLVED",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyIncidentStateEffects(context.Background(), "incident-6", domain.IncidentStateResolved)

	want := "incident-6|RESOLUTION"
	if len(repo.completed) != 1 || repo.completed[0] != want {
		t.Fatalf("completed = %v, want [%s]: RESOLVED is one of the policy's own stop_condition states (stateIN3,6,7 -> ON_HOLD,RESOLVED,CLOSED)", repo.completed, want)
	}
}

func TestSLAEngineService_ApplyIncidentStateEffects_StateDBLabelNormalizesCancelledTypo(t *testing.T) {
	// Regression test for the exact one-letter mismatch this package's own
	// doc comments warn about: domain.IncidentStateCancelled is spelled
	// "CANCELLED" (two L's) but the real stored incident_state_enum value,
	// and this translator's own output, is "CANCELED" (one L). Passing the
	// domain constant as ApplyIncidentStateEffects' stateOverride must still
	// correctly cancel the clock -- not silently fail to match by one
	// letter.
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.activeByCaseTarget["incident-7|RESOLUTION"] = cloudIncidentPolicyCandidates()["RESOLUTION"][0]
	repo.factsByIncident["incident-7"] = repository.IncidentFacts{
		State:     "IN_PROGRESS", // stale fresh-read value; override below takes precedence
		Category:  "SERVICE_INTERRUPTION",
		Priority:  "CRITICAL", // isolates the cancel_condition's OR'd "^NQ" groups to the state one
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyIncidentStateEffects(context.Background(), "incident-7", domain.IncidentStateCancelled)

	want := "incident-7|RESOLUTION"
	if len(repo.clockCancelled) != 1 || repo.clockCancelled[0] != want {
		t.Fatalf("clockCancelled = %v, want [%s]: domain.IncidentStateCancelled must normalize to the real \"CANCELED\" (one L) enum label",
			repo.clockCancelled, want)
	}
}

// TestIsIncidentPolicy_EscalationMarkerAlongsideBusinessServiceIsExcluded is
// the regression test for a real classification gap a review caught: an
// earlier revision's isIncidentPolicy only ever excluded
// slaPolicyCaseFieldMarkers, never slaPolicySkipFieldMarkers' own
// escalation-only entries (escalation_severity/source_table/
// active_escalation) despite its own doc comment claiming otherwise -- a
// hypothetical escalation row that also happened to carry "business_service"
// would have been misclassified as a genuine incident policy and reached
// RegisterIncidentClocks.
func TestIsIncidentPolicy_EscalationMarkerAlongsideBusinessServiceIsExcluded(t *testing.T) {
	for _, marker := range []string{"escalation_severity", "source_table", "active_escalation"} {
		t.Run(marker, func(t *testing.T) {
			conditions := "business_service=aaaa^" + marker + "=1"
			if isIncidentPolicy(conditions) {
				t.Errorf("isIncidentPolicy(%q) = true, want false: a %q marker must exclude a row even with business_service present", conditions, marker)
			}
		})
	}
}

func TestIsIncidentPolicy_BusinessServiceAndCategoryAloneStillMatch(t *testing.T) {
	// The real "Cloud Incident - High Priority" row's own shape: this must
	// keep matching -- it is NOT excluded by slaPolicySkipFieldMarkers'
	// "category=" entry, which is why isIncidentPolicy can't reuse that list
	// wholesale the way an earlier revision's doc comment incorrectly
	// claimed it did.
	if !isIncidentPolicy(realCloudIncidentStartCondition) {
		t.Errorf("isIncidentPolicy(%q) = false, want true", realCloudIncidentStartCondition)
	}
}

// TestIncidentService_UpdateIncident_NotesOnlyDoesNotReviseSLAClocks is the
// regression test for a real, confirmed bug a review caught: `before` is
// only fetched when the request has a lifecycle or record change, so a
// notes-only (WorkNotes/AdditionalComments) update left it zero-valued --
// and the SLA block's priority-change comparison, with no gate on whether a
// baseline was actually read, then read that zero-valued "" as "priority
// changed" against the freshly re-fetched view's real priority, cancelling
// and restarting a live RESOLUTION clock on every single work note.
func TestIncidentService_UpdateIncident_NotesOnlyDoesNotReviseSLAClocks(t *testing.T) {
	priority := "CRITICAL"
	repo := &stubIncidentRepo{
		createIncidentComment: func(_ context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: incidentID, Type: commentType, Content: content}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return domain.IncidentView{ID: &id, Priority: &priority}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := WithIncidentSLAEngine(
		NewIncidentServiceWithPublisher(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, nil),
		slaEngine,
	)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	workNotes := "investigating the repeat alert"
	if _, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &workNotes}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseIncidentCalls) != 0 {
		t.Errorf("reviseIncidentCalls = %v, want none: a notes-only update must never revise SLA clocks", slaEngine.reviseIncidentCalls)
	}
}

// TestIncidentService_UpdateIncident_ServiceIDOnlyChangeRevisesSLAClocks is
// the regression test for a second real gap a review caught alongside the
// notes-only one above: ReviseIncidentClocks used to run only on a genuine
// PRIORITY change, but the real "Cloud Incident - High Priority" policy's
// own start_condition also matches on business_service -- a ServiceID-only
// PATCH (priority unchanged) could move an incident onto, or off of, the
// Choreo/Asgardeo scope that policy requires, and the engine never
// re-resolved for it.
func TestIncidentService_UpdateIncident_ServiceIDOnlyChangeRevisesSLAClocks(t *testing.T) {
	priority := "CRITICAL"
	beforeService := &domain.EntityRef{ID: "11111111-1111-1111-1111-111111111111", Name: "Service A"}
	afterService := &domain.EntityRef{ID: "22222222-2222-2222-2222-222222222222", Name: "Service B"}
	calls := 0
	repo := &stubIncidentRepo{
		updateIncidentLifecycle: func(context.Context, string, repository.IncidentLifecycleUpdate, string) error { return nil },
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			calls++
			svc := beforeService
			if calls > 1 {
				svc = afterService
			}
			return domain.IncidentView{ID: &id, Priority: &priority, Service: svc}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := WithIncidentSLAEngine(
		NewIncidentServiceWithSNMirror(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, &stubMirrorIncidentService{
			updateIncident: func(context.Context, domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
				return domain.UpdateIncidentResponse{}, nil
			},
		}, nil, NewSNWritebackDispatcher(&recordingSNWritebackFailures{})),
		slaEngine,
	)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	newServiceID := afterService.ID
	if _, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, ServiceID: &newServiceID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseIncidentCalls) != 1 || slaEngine.reviseIncidentCalls[0].incidentID != testDeploymentUUID {
		t.Fatalf("reviseIncidentCalls = %v, want exactly one for incident %q: a ServiceID-only change must revise SLA clocks", slaEngine.reviseIncidentCalls, testDeploymentUUID)
	}
}

// TestIncidentService_UpdateIncident_CategoryOnlyChangeRevisesSLAClocks is
// CategoryOnlyChangeRevisesSLAClocks' sibling for the policy's other
// matched field, category.
func TestIncidentService_UpdateIncident_CategoryOnlyChangeRevisesSLAClocks(t *testing.T) {
	priority := "CRITICAL"
	beforeCategory := "INQUIRY"
	afterCategory := "SERVICE_INTERRUPTION"
	calls := 0
	repo := &stubIncidentRepo{
		updateIncidentLifecycle: func(context.Context, string, repository.IncidentLifecycleUpdate, string) error { return nil },
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			calls++
			cat := beforeCategory
			if calls > 1 {
				cat = afterCategory
			}
			return domain.IncidentView{ID: &id, Priority: &priority, Category: &cat}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := WithIncidentSLAEngine(
		NewIncidentServiceWithSNMirror(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, &stubMirrorIncidentService{
			updateIncident: func(context.Context, domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
				return domain.UpdateIncidentResponse{}, nil
			},
		}, nil, NewSNWritebackDispatcher(&recordingSNWritebackFailures{})),
		slaEngine,
	)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	newCategory := domain.IncidentCategoryServiceInterruption
	if _, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, Category: &newCategory}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseIncidentCalls) != 1 || slaEngine.reviseIncidentCalls[0].incidentID != testDeploymentUUID {
		t.Fatalf("reviseIncidentCalls = %v, want exactly one for incident %q: a Category-only change must revise SLA clocks", slaEngine.reviseIncidentCalls, testDeploymentUUID)
	}
}

// TestSNIncidentService_UpdateIncident_NonEscalationUpdateDoesNotReviseSLAClocks
// is sn_incident_service.go's own counterpart to the notes-only regression
// above: its pre-PATCH baseline is only fetched when
// incidentUpdateTouchesEscalation(req) is true, so a Subject-only PATCH (no
// state/priority/impact/urgency/assignee) skips it, leaving before.ID nil --
// without the before.ID != nil gate, comparing that zero-valued priority
// against the PATCH response's real one read as "changed" and revised a
// live clock for a plain subject edit.
func TestSNIncidentService_UpdateIncident_NonEscalationUpdateDoesNotReviseSLAClocks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident updated successfully.",
			"incident": {
				"id": "` + testIncidentSysid + `", "number": "INC0001",
				"createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com",
				"priority": {"id": 1, "label": "Critical"}
			}
		}`))
	})

	slaEngine := &fakeSLAEngineService{}
	svc := WithIncidentSLAEngine(NewServiceNowIncidentService(newTestSNClient(t, mux), nil), slaEngine)

	subject := "Updated subject text"
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:      testIncidentUUID,
		Subject: &subject,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseIncidentCalls) != 0 {
		t.Errorf("reviseIncidentCalls = %v, want none: a non-escalation update must never revise SLA clocks", slaEngine.reviseIncidentCalls)
	}
}

// TestSNIncidentService_UpdateIncident_ServiceIDOnlyChangeRevisesSLAClocks is
// sn_incident_service.go's own counterpart to
// TestIncidentService_UpdateIncident_ServiceIDOnlyChangeRevisesSLAClocks:
// incidentUpdateTouchesEscalation(req) alone used to gate BOTH the baseline
// fetch and the revision itself, and that function has no idea about
// ServiceID/Category -- so a ServiceID-only PATCH never even fetched a
// baseline, let alone revised the clock.
func TestSNIncidentService_UpdateIncident_ServiceIDOnlyChangeRevisesSLAClocks(t *testing.T) {
	const newServiceSysid = "22222222222222222222222222222222"
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{
				"id": "` + testIncidentSysid + `", "number": "INC0001",
				"createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com",
				"priority": {"id": 1, "label": "Critical"},
				"service": {"id": "11111111111111111111111111111111", "name": "Service A"}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"message": "Incident updated successfully.",
			"incident": {
				"id": "` + testIncidentSysid + `", "number": "INC0001",
				"createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com",
				"priority": {"id": 1, "label": "Critical"},
				"service": {"id": "` + newServiceSysid + `", "name": "Service B"}
			}
		}`))
	})

	slaEngine := &fakeSLAEngineService{}
	svc := WithIncidentSLAEngine(NewServiceNowIncidentService(newTestSNClient(t, mux), nil), slaEngine)

	newServiceID := sysidToUUID(newServiceSysid)
	if _, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:        testIncidentUUID,
		ServiceID: &newServiceID,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slaEngine.reviseIncidentCalls) != 1 || slaEngine.reviseIncidentCalls[0].incidentID != testIncidentUUID {
		t.Fatalf("reviseIncidentCalls = %v, want exactly one for incident %q: a ServiceID-only change must revise SLA clocks", slaEngine.reviseIncidentCalls, testIncidentUUID)
	}
}
