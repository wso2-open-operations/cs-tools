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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testIncidentWatcherUUID1  = "44444444-4444-4444-4444-444444444444"
	testIncidentWatcherUUID2  = "55555555-5555-5555-5555-555555555555"
	testIncidentWatcherSysid1 = "44444444444444444444444444444444"
	testIncidentWatcherSysid2 = "55555555555555555555555555555555"
	testIncidentUUID          = "66666666-6666-6666-6666-666666666666"
	testIncidentSysid         = "66666666666666666666666666666666"
)

// validCreateIncidentRequest returns a minimally valid CreateIncidentRequest so
// that only the field under test needs to be overridden per case.
func validCreateIncidentRequest() domain.CreateIncidentRequest {
	return domain.CreateIncidentRequest{
		CallerID:  testCaseUUID,
		Category:  domain.IncidentCategoryInquiry,
		ServiceID: testCaseUUID,
		Impact:    domain.IncidentImpactLow,
		Urgency:   domain.IncidentUrgencyLow,
		Subject:   "subject",
	}
}

// TestSNIncidentService_CreateIncident_WatchListResolvedToEmails verifies that
// every watchList UUID is resolved to the watcher's email address before it
// reaches the outgoing payload: the backing service's incident-create payload
// declares the watch list as emails, so forwarding ids -- raw or reformatted --
// silently drops every watcher.
func TestSNIncidentService_CreateIncident_WatchListResolvedToEmails(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", watchListUserSearchStub(t))
	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident created successfully.",
			"incident": {"id": "` + testIncidentSysid + `", "number": "INC0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	req := validCreateIncidentRequest()
	req.WatchList = []string{testIncidentWatcherUUID1, testIncidentWatcherUUID2}

	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotWatchList, ok := gotBody["watchList"].([]any)
	if !ok {
		t.Fatalf("expected watchList array in payload, got %+v", gotBody["watchList"])
	}
	want := []string{testWatcherEmail1, testWatcherEmail2}
	if len(gotWatchList) != len(want) {
		t.Fatalf("watchList length: got %d, want %d", len(gotWatchList), len(want))
	}
	for i, w := range want {
		if gotWatchList[i] != w {
			t.Fatalf("watchList[%d]: got %v, want %q (an id must never be sent as a watcher)", i, gotWatchList[i], w)
		}
	}
}

// TestSNIncidentService_CreateIncident_WatchList_InvalidUUID verifies a malformed
// watchList entry is rejected with a clean validation error before any SN call.
func TestSNIncidentService_CreateIncident_WatchList_InvalidUUID(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := validCreateIncidentRequest()
	req.WatchList = []string{"not-a-uuid"}

	_, err := svc.CreateIncident(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_UpdateIncident_WatchListResolvedToUserNames mirrors the
// create-path coverage above for the PATCH /incidents/{id} path, which declares
// its watch list as usernames rather than the emails its create counterpart
// takes.
func TestSNIncidentService_UpdateIncident_WatchListResolvedToUserNames(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/search", watchListUserSearchStub(t))
	mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident updated successfully.",
			"incident": {"id": "` + testIncidentSysid + `", "number": "INC0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	watchList := []string{testIncidentWatcherUUID1, testIncidentWatcherUUID2}
	_, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:        testIncidentUUID,
		WatchList: &watchList,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotWatchList, ok := gotBody["watchList"].([]any)
	if !ok {
		t.Fatalf("expected watchList array in payload, got %+v", gotBody["watchList"])
	}
	want := []string{testWatcherUserName1, testWatcherUserName2}
	if len(gotWatchList) != len(want) {
		t.Fatalf("watchList length: got %d, want %d", len(gotWatchList), len(want))
	}
	for i, w := range want {
		if gotWatchList[i] != w {
			t.Fatalf("watchList[%d]: got %v, want %q (an id must never be sent as a watcher)", i, gotWatchList[i], w)
		}
	}
}

// TestSNIncidentService_UpdateIncident_WatchList_InvalidUUID verifies a malformed
// watchList entry is rejected with a clean validation error before any SN call.
func TestSNIncidentService_UpdateIncident_WatchList_InvalidUUID(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	watchList := []string{"not-a-uuid"}
	_, err := svc.UpdateIncident(contextWithUserIDToken("token"), domain.UpdateIncidentRequest{
		ID:        testIncidentUUID,
		WatchList: &watchList,
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_NumberFilterPassedThrough verifies the
// exact-match Number filter reaches the outgoing payload under the "number" key
// unchanged, alongside the untouched free-text searchQuery.
func TestSNIncidentService_SearchIncidents_NumberFilterPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"incidents": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{Number: strPtr("INC0010001")},
	}
	if _, err := svc.SearchIncidents(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["number"] != "INC0010001" {
		t.Fatalf("filters.number: got %v, want %q", gotFilters["number"], "INC0010001")
	}
	if _, hasSearchQuery := gotFilters["searchQuery"]; hasSearchQuery {
		t.Fatalf("filters.searchQuery: expected omitted (empty), got %v", gotFilters["searchQuery"])
	}
}

// TestSNIncidentService_SearchIncidents_NewFiltersPassedThrough verifies the
// generic filters array's state/assignmentGroupId/businessServiceId predicates
// reach the outgoing payload under the exact wire names Ballerina accepts,
// with the domain enum state values translated to SN numeric keys and the two
// UUID arrays converted to ServiceNow sysids.
func TestSNIncidentService_SearchIncidents_NewFiltersPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"incidents": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "state", Op: "in", Values: []string{"NEW", "CLOSED"}},
				{Field: "assignmentGroupId", Op: "in", Values: []string{testIncidentWatcherUUID1}},
				{Field: "businessServiceId", Op: "in", Values: []string{testIncidentWatcherUUID2}},
			},
		},
	}
	if _, err := svc.SearchIncidents(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}

	gotStateKeys, ok := gotFilters["stateKeys"].([]any)
	if !ok || len(gotStateKeys) != 2 || gotStateKeys[0] != float64(1) || gotStateKeys[1] != float64(7) {
		t.Fatalf("filters.stateKeys: got %v, want [1, 7]", gotFilters["stateKeys"])
	}

	gotAssignmentGroupIDs, ok := gotFilters["assignmentGroupIds"].([]any)
	if !ok || len(gotAssignmentGroupIDs) != 1 || gotAssignmentGroupIDs[0] != testIncidentWatcherSysid1 {
		t.Fatalf("filters.assignmentGroupIds: got %v, want [%q] (raw UUID must not be sent to SN)", gotFilters["assignmentGroupIds"], testIncidentWatcherSysid1)
	}

	gotBusinessServiceIDs, ok := gotFilters["businessServiceIds"].([]any)
	if !ok || len(gotBusinessServiceIDs) != 1 || gotBusinessServiceIDs[0] != testIncidentWatcherSysid2 {
		t.Fatalf("filters.businessServiceIds: got %v, want [%q] (raw UUID must not be sent to SN)", gotFilters["businessServiceIds"], testIncidentWatcherSysid2)
	}
}

// TestSNIncidentService_SearchIncidents_SlaViolatedAndProductNamePassedThrough
// verifies the generic filters array's slaViolated/productName predicates
// reach the outgoing payload under the exact wire names Ballerina accepts.
func TestSNIncidentService_SearchIncidents_SlaViolatedAndProductNamePassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"incidents": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "slaViolated", Op: "eq", Values: []string{"true"}},
				{Field: "productName", Op: "in", Values: []string{"API Manager", "Choreo"}},
			},
		},
	}
	if _, err := svc.SearchIncidents(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}

	if gotFilters["slaViolated"] != true {
		t.Fatalf("filters.slaViolated: got %v, want true", gotFilters["slaViolated"])
	}

	gotProductNames, ok := gotFilters["productNames"].([]any)
	if !ok || len(gotProductNames) != 2 || gotProductNames[0] != "API Manager" || gotProductNames[1] != "Choreo" {
		t.Fatalf("filters.productNames: got %v, want [API Manager, Choreo]", gotFilters["productNames"])
	}
}

// TestSNIncidentService_SearchIncidents_SlaViolatedInvalidValue verifies a
// non-boolean slaViolated filter value is rejected with a clean validation
// error before any SN call.
func TestSNIncidentService_SearchIncidents_SlaViolatedInvalidValue(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "slaViolated", Op: "eq", Values: []string{"not-a-bool"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_MadeSlaPassedThrough verifies the
// generic filters array's madeSla predicate reaches the outgoing payload
// under the exact wire name Ballerina accepts, as a sibling to (and
// independent of) slaViolated -- see domain.SearchIncidentsFilters Filters
// "madeSla" doc comment for why the two are kept distinct.
func TestSNIncidentService_SearchIncidents_MadeSlaPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"incidents": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "madeSla", Op: "eq", Values: []string{"false"}},
			},
		},
	}
	if _, err := svc.SearchIncidents(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}

	if gotFilters["madeSla"] != false {
		t.Fatalf("filters.madeSla: got %v, want false", gotFilters["madeSla"])
	}
	if _, present := gotFilters["slaViolated"]; present {
		t.Fatalf("filters.slaViolated: got present with value %v, want omitted since it was not requested", gotFilters["slaViolated"])
	}
}

// TestSNIncidentService_SearchIncidents_MadeSlaInvalidValue verifies a
// non-boolean madeSla filter value is rejected with a clean validation error
// before any SN call.
func TestSNIncidentService_SearchIncidents_MadeSlaInvalidValue(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "madeSla", Op: "eq", Values: []string{"not-a-bool"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_InvalidStateValue verifies an
// unrecognized state filter value is rejected with a clean validation error
// before any SN call.
func TestSNIncidentService_SearchIncidents_InvalidStateValue(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "state", Op: "in", Values: []string{"BOGUS"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_InvalidFilterField verifies an
// unsupported filters[] field name is rejected before any SN call.
func TestSNIncidentService_SearchIncidents_InvalidFilterField(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "notAField", Op: "in", Values: []string{"x"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_RejectsInvertedCreatedOnRange verifies
// a createdOn lte bound before its gte bound is rejected with a validation
// error before any SN call, instead of reaching ServiceNow and coming back
// as an empty (200) result indistinguishable from "no matching incidents".
func TestSNIncidentService_SearchIncidents_RejectsInvertedCreatedOnRange(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{"2026-08-10"}},
				{Field: "createdOn", Op: "lte", Values: []string{"2026-08-01"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_SearchIncidents_BusinessServiceIdInvalidUUID verifies a
// malformed businessServiceId filter value is rejected with a clean
// validation error before any SN call.
func TestSNIncidentService_SearchIncidents_BusinessServiceIdInvalidUUID(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	req := domain.SearchIncidentsRequest{
		Filters: domain.SearchIncidentsFilters{
			Filters: []domain.IncidentFieldFilter{
				{Field: "businessServiceId", Op: "in", Values: []string{"not-a-uuid"}},
			},
		},
	}
	_, err := svc.SearchIncidents(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNIncidentService_HandOffIncidentToSpecialist_PayloadAndResponseMapping
// verifies the outgoing payload carries reasonCode/escalationTeam/createGithubIssue
// under their exact wire names, and that every part of the response -- including
// the nested incident detail -- is mapped back to the domain representation, with
// previousAssignmentGroup/githubIssue/githubIssueError staying nil when the backing
// data source omits them rather than becoming a zero value.
func TestSNIncidentService_HandOffIncidentToSpecialist_PayloadAndResponseMapping(t *testing.T) {
	var gotBody map[string]any
	specialistGroupSysid := sysid32('7')
	taskSysid := sysid32('8')

	mux := http.NewServeMux()
	mux.HandleFunc("/incidents/"+testIncidentSysid+"/specialist-handoffs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Incident handed off to specialist group",
			"handoff": {
				"assignmentGroup": {"id": "` + specialistGroupSysid + `", "name": "Choreo APIM Special Ops"},
				"previousAssignmentGroup": null,
				"reasonCode": "no-runbook",
				"reasonDescription": "Runbook is not available",
				"escalationTeam": "choreo-apim-team",
				"task": {"id": "` + taskSysid + `", "number": "TASK0082504", "subject": "[Runbook Task] No entry available for INC0091926"},
				"githubIssue": null,
				"githubIssueError": null,
				"incident": {"id": "` + testIncidentSysid + `", "number": "INC0091926"}
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowIncidentService(client)

	createGithubIssue := false
	team := domain.IncidentSpecialistHandoffTeamChoreoAPIM
	req := domain.HandOffIncidentToSpecialistRequest{
		IncidentID:        testIncidentUUID,
		ReasonCode:        domain.IncidentSpecialistHandoffReasonNoRunbook,
		EscalationTeam:    &team,
		CreateGithubIssue: &createGithubIssue,
	}
	resp, err := svc.HandOffIncidentToSpecialist(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["reasonCode"] != "no-runbook" {
		t.Fatalf("reasonCode: got %v, want no-runbook", gotBody["reasonCode"])
	}
	if gotBody["escalationTeam"] != "choreo-apim-team" {
		t.Fatalf("escalationTeam: got %v, want choreo-apim-team", gotBody["escalationTeam"])
	}
	if gotBody["createGithubIssue"] != false {
		t.Fatalf("createGithubIssue: got %v, want false", gotBody["createGithubIssue"])
	}

	if resp.Message != "Incident handed off to specialist group" {
		t.Fatalf("Message = %q", resp.Message)
	}
	h := resp.Handoff
	if h.AssignmentGroup.ID != sysidToUUID(specialistGroupSysid) || h.AssignmentGroup.Name != "Choreo APIM Special Ops" {
		t.Fatalf("AssignmentGroup = %+v", h.AssignmentGroup)
	}
	if h.PreviousAssignmentGroup != nil {
		t.Fatalf("PreviousAssignmentGroup = %+v, want nil", h.PreviousAssignmentGroup)
	}
	if h.ReasonCode != domain.IncidentSpecialistHandoffReasonNoRunbook || h.ReasonDescription != "Runbook is not available" {
		t.Fatalf("ReasonCode/ReasonDescription = %v/%v", h.ReasonCode, h.ReasonDescription)
	}
	if h.EscalationTeam == nil || *h.EscalationTeam != domain.IncidentSpecialistHandoffTeamChoreoAPIM {
		t.Fatalf("EscalationTeam = %v", h.EscalationTeam)
	}
	if h.Task.ID != sysidToUUID(taskSysid) || h.Task.Number != "TASK0082504" ||
		h.Task.Subject != "[Runbook Task] No entry available for INC0091926" {
		t.Fatalf("Task = %+v", h.Task)
	}
	if h.GithubIssue != nil {
		t.Fatalf("GithubIssue = %+v, want nil", h.GithubIssue)
	}
	if h.GithubIssueError != nil {
		t.Fatalf("GithubIssueError = %v, want nil", *h.GithubIssueError)
	}
	if h.Incident.ID == nil || *h.Incident.ID != sysidToUUID(testIncidentSysid) || h.Incident.Number == nil || *h.Incident.Number != "INC0091926" {
		t.Fatalf("Incident = %+v", h.Incident)
	}
}

// TestSNIncidentService_HandOffIncidentToSpecialist_ValidatesInput verifies that
// a missing/invalid reasonCode or an invalid escalationTeam is rejected before any
// call reaches the backing data source.
func TestSNIncidentService_HandOffIncidentToSpecialist_ValidatesInput(t *testing.T) {
	invalidTeam := domain.IncidentSpecialistHandoffEscalationTeam("not-a-team")

	tests := []struct {
		name string
		req  domain.HandOffIncidentToSpecialistRequest
	}{
		{
			name: "missing reasonCode",
			req:  domain.HandOffIncidentToSpecialistRequest{IncidentID: testIncidentUUID},
		},
		{
			name: "invalid reasonCode",
			req: domain.HandOffIncidentToSpecialistRequest{
				IncidentID: testIncidentUUID,
				ReasonCode: domain.IncidentSpecialistHandoffReasonCode("bogus"),
			},
		},
		{
			name: "invalid escalationTeam",
			req: domain.HandOffIncidentToSpecialistRequest{
				IncidentID:     testIncidentUUID,
				ReasonCode:     domain.IncidentSpecialistHandoffReasonNoRunbook,
				EscalationTeam: &invalidTeam,
			},
		},
		{
			name: "invalid incident id",
			req: domain.HandOffIncidentToSpecialistRequest{
				IncidentID: "not-a-uuid",
				ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook,
			},
		},
	}

	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowIncidentService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.HandOffIncidentToSpecialist(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNIncidentService_GetIncidentByID_MapsSpecialistHandoff verifies the
// specialistHandoff block on incident detail is mapped through, and stays nil
// when the backing data source omits it (an incident never handed off).
func TestSNIncidentService_GetIncidentByID_MapsSpecialistHandoff(t *testing.T) {
	groupSysid := sysid32('9')

	tests := []struct {
		name           string
		responseJSON   string
		wantHandoffNil bool
	}{
		{
			name: "handed-off incident carries a summary",
			responseJSON: `{"id": "` + testIncidentSysid + `", "number": "INC0091926",
				"specialistHandoff": {
					"reasonCode": "no-runbook",
					"reasonDescription": "Runbook is not available",
					"escalationTeam": null,
					"handedOffAt": "2026-08-21 04:02:48",
					"handedOffBy": "sajithe@wso2.com",
					"assignmentGroup": {"id": "` + groupSysid + `", "name": "Choreo Special Ops"},
					"task": {"number": "TASK0082502", "subject": "[Runbook Task] No entry available for INC0091926", "state": "0", "stateLabel": "Open"},
					"githubIssueUrl": "https://github.com/wso2-enterprise/asgardeo-product/issues/36771"
				}}`,
		},
		{
			name:           "never handed off incident carries no summary",
			responseJSON:   `{"id": "` + testIncidentSysid + `", "number": "INC0091927", "specialistHandoff": null}`,
			wantHandoffNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/incidents/"+testIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responseJSON))
			})
			client := newTestSNClient(t, mux)
			svc := NewServiceNowIncidentService(client)

			view, err := svc.GetIncidentByID(contextWithUserIDToken("token"), testIncidentUUID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantHandoffNil {
				if view.SpecialistHandoff != nil {
					t.Fatalf("SpecialistHandoff = %+v, want nil", view.SpecialistHandoff)
				}
				return
			}
			if view.SpecialistHandoff == nil {
				t.Fatalf("SpecialistHandoff = nil, want non-nil")
			}
			sh := view.SpecialistHandoff
			if sh.ReasonCode != domain.IncidentSpecialistHandoffReasonNoRunbook || sh.ReasonDescription != "Runbook is not available" {
				t.Fatalf("ReasonCode/ReasonDescription = %v/%v", sh.ReasonCode, sh.ReasonDescription)
			}
			if sh.EscalationTeam != nil {
				t.Fatalf("EscalationTeam = %v, want nil", sh.EscalationTeam)
			}
			if sh.HandedOffAt != "2026-08-21 04:02:48" || sh.HandedOffBy == nil || *sh.HandedOffBy != "sajithe@wso2.com" {
				t.Fatalf("HandedOffAt/HandedOffBy = %v/%v", sh.HandedOffAt, sh.HandedOffBy)
			}
			if sh.AssignmentGroup.ID != sysidToUUID(groupSysid) || sh.AssignmentGroup.Name != "Choreo Special Ops" {
				t.Fatalf("AssignmentGroup = %+v", sh.AssignmentGroup)
			}
			if sh.Task.Number != "TASK0082502" || sh.Task.State == nil || *sh.Task.State != "0" ||
				sh.Task.StateLabel == nil || *sh.Task.StateLabel != "Open" {
				t.Fatalf("Task = %+v", sh.Task)
			}
			if sh.GithubIssueURL == nil || *sh.GithubIssueURL != "https://github.com/wso2-enterprise/asgardeo-product/issues/36771" {
				t.Fatalf("GithubIssueURL = %v", sh.GithubIssueURL)
			}
		})
	}
}
