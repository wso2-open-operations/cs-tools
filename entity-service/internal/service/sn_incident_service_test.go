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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(client, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
	svc := NewServiceNowIncidentService(nil, nil)

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
