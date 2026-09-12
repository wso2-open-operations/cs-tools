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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProblemService_SearchProblems_NumberFilterPassedThrough verifies the
// exact-match Number filter reaches the outgoing payload under the "number" key
// unchanged, alongside the untouched free-text searchQuery.
func TestSNProblemService_SearchProblems_NumberFilterPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"problems": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	req := domain.SearchProblemsRequest{
		Filters: domain.SearchProblemsFilters{Number: strPtr("PRB0010001")},
	}
	if _, err := svc.SearchProblems(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["number"] != "PRB0010001" {
		t.Fatalf("filters.number: got %v, want %q", gotFilters["number"], "PRB0010001")
	}
	if _, hasSearchQuery := gotFilters["searchQuery"]; hasSearchQuery {
		t.Fatalf("filters.searchQuery: expected omitted (empty), got %v", gotFilters["searchQuery"])
	}
}

// TestSNProblemService_SearchProblems_RejectsOverlongNumber verifies an
// oversized exact-match Number filter is rejected as a *apierror.ValidationError
// before the ServiceNow client is ever called.
func TestSNProblemService_SearchProblems_RejectsOverlongNumber(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/search", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"problems": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	req := domain.SearchProblemsRequest{
		Filters: domain.SearchProblemsFilters{Number: strPtr(strings.Repeat("x", maxExactNumberLen+1))},
	}
	_, err := svc.SearchProblems(contextWithUserIDToken("token"), req)

	var valErr *apierror.ValidationError
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	if ok := asValidationError(err, &valErr); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
	if called {
		t.Fatal("ServiceNow client was called despite the invalid filter")
	}
}

const (
	testProblemUUID  = "77777777-7777-7777-7777-777777777777"
	testProblemSysid = "77777777777777777777777777777777"
)

// TestSNProblemService_UpdateProblem_TransitionPassedThroughUnvalidated verifies a
// transition value reaches the outgoing PATCH payload verbatim -- this layer must
// never re-validate it as a closed enum, so an invalid value's actionable error from
// the data source reaches the caller unchanged instead of being swallowed here.
func TestSNProblemService_UpdateProblem_TransitionPassedThroughUnvalidated(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/"+testProblemSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Problem updated successfully",
			"problem": {
				"id": "` + testProblemSysid + `",
				"updatedOn": "2026-08-23 00:00:00",
				"updatedBy": "engineer@example.com",
				"state": "ASSESS",
				"resolutionCode": null,
				"assignedTo": {"id": "` + testProblemSysid + `", "name": "Jane Doe"},
				"assignmentGroup": null
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	resp, err := svc.UpdateProblem(contextWithUserIDToken("token"), domain.UpdateProblemRequest{
		ID:           testProblemUUID,
		Transition:   strPtr("teleport"), // deliberately invalid -- must still be forwarded as-is
		AssignedToID: strPtr(testProblemUUID),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["transition"] != "teleport" {
		t.Fatalf("transition: got %v, want %q (unvalidated passthrough)", gotBody["transition"], "teleport")
	}
	if gotBody["assignedToId"] != testProblemSysid {
		t.Fatalf("assignedToId: got %v, want %q (UUID must be converted to sysid)", gotBody["assignedToId"], testProblemSysid)
	}
	if resp.Problem.State == nil || *resp.Problem.State != "ASSESS" {
		t.Fatalf("response state: got %v, want ASSESS (the real post-write state)", resp.Problem.State)
	}
	if resp.Problem.AssignedTo == nil || resp.Problem.AssignedTo.ID != testProblemUUID {
		t.Fatalf("response assignedTo: got %+v, want id %q (sysid converted back to UUID)", resp.Problem.AssignedTo, testProblemUUID)
	}
}

// TestSNProblemService_UpdateProblem_RequiresAtLeastOneField verifies an empty
// request is rejected as a *apierror.ValidationError before the client is called.
func TestSNProblemService_UpdateProblem_RequiresAtLeastOneField(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/"+testProblemSysid, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	_, err := svc.UpdateProblem(contextWithUserIDToken("token"), domain.UpdateProblemRequest{ID: testProblemUUID})

	var valErr *apierror.ValidationError
	if !asValidationError(err, &valErr) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
	if called {
		t.Fatal("ServiceNow client was called despite an empty update request")
	}
}

// TestSNProblemService_UpdateProblem_InvalidAssignedToID verifies a malformed
// assignedToId is rejected as a validation error before the client is called.
func TestSNProblemService_UpdateProblem_InvalidAssignedToID(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowProblemService(nil)

	_, err := svc.UpdateProblem(contextWithUserIDToken("token"), domain.UpdateProblemRequest{
		ID:           testProblemUUID,
		AssignedToID: strPtr("not-a-uuid"),
	})

	var valErr *apierror.ValidationError
	if !asValidationError(err, &valErr) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNProblemService_UpdateProblem_ConflictMapped verifies a 409 from the data
// source (a rejected/reverted state transition) surfaces as a *apierror.ConflictError.
func TestSNProblemService_UpdateProblem_ConflictMapped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/problems/"+testProblemSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code": 409, "message": "State transition rejected"}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProblemService(client)

	_, err := svc.UpdateProblem(contextWithUserIDToken("token"), domain.UpdateProblemRequest{
		ID:         testProblemUUID,
		Transition: strPtr("assess"),
	})

	var confErr *apierror.ConflictError
	if !errors.As(err, &confErr) {
		t.Fatalf("expected *apierror.ConflictError, got %T: %v", err, err)
	}
}
