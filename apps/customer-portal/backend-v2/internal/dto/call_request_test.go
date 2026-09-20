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

package dto

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestBuildEntityUpdateCallRequestRequest_StateKeyTranslatesToEnum guards
// against reintroducing the bug where this backend expected a "state" string
// enum field the frontend never sends — the frontend (built against the old
// Ballerina backend) sends stateKey as a ServiceNow numeric choice-list key.
func TestBuildEntityUpdateCallRequestRequest_StateKeyTranslatesToEnum(t *testing.T) {
	got := BuildEntityUpdateCallRequestRequest(CallRequestUpdateRequest{StateKey: 6})

	if got.State != "canceled" {
		t.Fatalf("State = %q, want %q", got.State, "canceled")
	}
}

// TestBuildEntityUpdateCallRequestRequest_UnrecognizedStateKeyProducesEmptyState
// verifies an unrecognized (or absent, zero-value) stateKey translates to an
// empty State rather than panicking or forwarding the raw number —
// entity-service's own validCallRequestStates check then rejects an empty
// state with 400.
func TestBuildEntityUpdateCallRequestRequest_UnrecognizedStateKeyProducesEmptyState(t *testing.T) {
	got := BuildEntityUpdateCallRequestRequest(CallRequestUpdateRequest{StateKey: 999})

	if got.State != "" {
		t.Fatalf("State = %q, want empty string for an unrecognized stateKey", got.State)
	}
}

// TestBuildEntitySearchCallRequestsRequest_CaseIDFromPathAndStateKeysTranslated
// verifies caseID always comes from the path parameter (never the body,
// which the frontend never sends one in) and that filters.stateKeys
// translates to entity-service's own states string enum.
func TestBuildEntitySearchCallRequestsRequest_CaseIDFromPathAndStateKeysTranslated(t *testing.T) {
	req := CallRequestSearchRequest{
		Filters:    CallRequestSearchFilters{StateKeys: []int{3, 8, 999}},
		Pagination: entity.Pagination{Limit: 10, Offset: 0},
	}

	got := BuildEntitySearchCallRequestsRequest("case-1", req)

	if got.CaseID != toDashedID("case-1") {
		t.Fatalf("CaseID = %q, want %q", got.CaseID, toDashedID("case-1"))
	}
	if got.Filters == nil {
		t.Fatal("expected non-nil Filters")
	}
	want := []string{"scheduled", "concluded"}
	if !reflect.DeepEqual(got.Filters.States, want) {
		t.Fatalf("States = %+v, want %+v (999 has no mapping and must be dropped)", got.Filters.States, want)
	}
	if got.Pagination != req.Pagination {
		t.Fatalf("Pagination = %+v, want %+v", got.Pagination, req.Pagination)
	}
}

func TestBuildEntitySearchCallRequestsRequest_CaseIDNormalizedToDashedUUID(t *testing.T) {
	req := CallRequestSearchRequest{
		Pagination: entity.Pagination{Limit: 10, Offset: 0},
	}
	got := BuildEntitySearchCallRequestsRequest("26051dbc3baa8f5091404c6aa5e45a1c", req)
	if got.CaseID != "26051dbc-3baa-8f50-9140-4c6aa5e45a1c" {
		t.Fatalf("CaseID = %q, want %q", got.CaseID, "26051dbc-3baa-8f50-9140-4c6aa5e45a1c")
	}
}

// TestBuildEntitySearchCallRequestsRequest_NoStateKeysLeavesFiltersNil
// verifies an empty/absent stateKeys filter produces a nil Filters, matching
// entity.SearchCallRequestsRequest.Filters's omitempty/optional contract,
// rather than an empty-but-present filters object.
func TestBuildEntitySearchCallRequestsRequest_NoStateKeysLeavesFiltersNil(t *testing.T) {
	got := BuildEntitySearchCallRequestsRequest("case-1", CallRequestSearchRequest{})

	if got.Filters != nil {
		t.Fatalf("Filters = %+v, want nil", got.Filters)
	}
}

func TestCreateCallRequestResponse_UnmarshalChoiceListState(t *testing.T) {
	// Sample response with choice-list state structure
	raw := []byte(`{
		"message": "Call request created successfully.",
		"callRequest": {
			"id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
			"createdOn": "2026-01-01 10:00:00",
			"createdBy": "user@example.com",
			"state": {
				"id": 2,
				"label": "Pending on WSO2"
			},
			"scheduleTime": "2026-01-02 10:00:00"
		}
	}`)

	var resp entity.CreateCallRequestResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if resp.CallRequest.ID != "a1b2c3d4e5f60718293a4b5c6d7e8f90" {
		t.Errorf("ID = %q, want a1b2c3d4e5f60718293a4b5c6d7e8f90", resp.CallRequest.ID)
	}
	if resp.CallRequest.State.ID != "2" || resp.CallRequest.State.Label != "Pending on WSO2" {
		t.Errorf("State = %+v, want ID=\"2\", Label=\"Pending on WSO2\"", resp.CallRequest.State)
	}

	mapped := MapCallRequestCreate(resp)
	if mapped.ID != "a1b2c3d4e5f60718293a4b5c6d7e8f90" {
		t.Errorf("mapped.ID = %q, want a1b2c3d4e5f60718293a4b5c6d7e8f90", mapped.ID)
	}
	if mapped.State != "Pending on WSO2" {
		t.Errorf("mapped.State = %q, want \"Pending on WSO2\"", mapped.State)
	}
}

func TestSearchCallRequestsResponse_UnmarshalNumericStateID(t *testing.T) {
	raw := []byte(`{
		"callRequests": [
			{
				"id": "a1b2c3d4e5f60718293a4b5c6d7e8f90",
				"number": "CALL0000001",
				"case": {"id": "22222222222222222222222222222222", "name": "CS0000001"},
				"createdOn": "2026-01-01 10:00:00",
				"updatedOn": "2026-01-01 10:00:00",
				"state": {"id": 2, "label": "Pending on WSO2"},
				"preferredTimes": ["2026-01-02T10:00:00Z"],
				"durationMin": 30
			}
		],
		"total": 1,
		"offset": 0,
		"limit": 10
	}`)

	var resp entity.SearchCallRequestsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(resp.CallRequests) != 1 {
		t.Fatalf("expected 1 call request, got %d", len(resp.CallRequests))
	}
	if resp.CallRequests[0].State.ID != "2" {
		t.Errorf("State.ID = %q, want \"2\"", resp.CallRequests[0].State.ID)
	}

	mapped := MapSearchCallRequests(resp)
	if mapped.CallRequests[0].State.ID != "2" || mapped.CallRequests[0].State.Label != "Pending on WSO2" {
		t.Errorf("mapped State = %+v, want ID=\"2\", Label=\"Pending on WSO2\"", mapped.CallRequests[0].State)
	}
}
