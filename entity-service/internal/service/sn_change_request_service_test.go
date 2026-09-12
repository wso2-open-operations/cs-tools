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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestToDownstreamUTCDateTime covers the create-path datetime conversion: the
// platform's API accepts one datetime format everywhere, and the downstream
// create endpoint requires a different one than its own update endpoint.
func TestToDownstreamUTCDateTime(t *testing.T) {
	t.Parallel()

	t.Run("converts platform format to the downstream format", func(t *testing.T) {
		got, err := toDownstreamUTCDateTime("plannedStartDate", "2026-08-01 10:00:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "2026-08-01T10:00:00Z"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("rejects bad input with a validation error naming the field", func(t *testing.T) {
		for _, in := range []string{
			"2026-08-01T10:00:00Z", // already UTC form: not the platform's format
			"2026-08-01",
			"01-08-2026 10:00:00",
			"not a date",
			"",
		} {
			_, err := toDownstreamUTCDateTime("plannedEndDate", in)
			if err == nil {
				t.Errorf("input %q: expected an error, got none", in)
				continue
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("input %q: expected *apierror.ValidationError, got %T", in, err)
				continue
			}
			if want := "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss"; ve.Msg != want {
				t.Errorf("input %q: got msg %q, want %q", in, ve.Msg, want)
			}
		}
	})
}

// TestPatchResponseToleratesSlimReceipt pins the behaviour at the boundary where
// a committed write was being reported as a total failure. The downstream layer
// may answer a change-request write with a slim receipt (identifier plus a few
// fields) rather than the full detail payload. Decoding that must not fail, and
// mapping it must not panic on the absent fields.
func TestPatchResponseToleratesSlimReceipt(t *testing.T) {
	t.Parallel()

	const slimReceipt = `{
		"message": "Change request updated successfully.",
		"changeRequest": {
			"id": "0123456789abcdef0123456789abcdef",
			"state": {"label": "Assess"},
			"updatedOn": "2026-07-30 11:22:33",
			"updatedBy": "engineer@example.com"
		}
	}`

	var resp snPatchChangeRequestResponse
	if err := json.Unmarshal([]byte(slimReceipt), &resp); err != nil {
		t.Fatalf("slim receipt failed to decode: %v", err)
	}

	view := mapSNChangeRequestDetailToView(resp.ChangeRequest)

	if want := "01234567-89ab-cdef-0123-456789abcdef"; view.ID != want {
		t.Errorf("ID: got %q, want %q", view.ID, want)
	}
	if view.State == nil {
		t.Error("State: got nil, want a mapped value")
	}
	if want := "2026-07-30 11:22:33"; view.UpdatedOn != want {
		t.Errorf("UpdatedOn: got %q, want %q", view.UpdatedOn, want)
	}
	// Absent optional references must map to nil, not panic and not fabricate.
	if view.Case != nil || view.Deployment != nil || view.AssignedEngineer != nil || view.AssignedTeam != nil {
		t.Error("absent optional references should map to nil")
	}
	// An absent required-in-the-full-payload reference degrades to a zero value.
	if view.Project.ID != "" {
		t.Errorf("Project.ID: got %q, want empty", view.Project.ID)
	}
}

// TestNormalizePaginationCapMatchesDownstream pins the cap at the single choke
// point every search normalizes through. The downstream layer rejects a limit
// above 50 with an opaque error, so exceeding it must be caught here with a
// named validation error instead.
func TestNormalizePaginationCapMatchesDownstream(t *testing.T) {
	t.Parallel()

	if maxLimit != 50 {
		t.Fatalf("maxLimit is %d; the downstream layer rejects anything above 50", maxLimit)
	}
}

// TestSNChangeRequestService_SearchChangeRequests_NumberFilterPassedThrough verifies
// the exact-match Number filter reaches the outgoing payload under the "number" key
// unchanged, alongside the untouched free-text searchQuery.
func TestSNChangeRequestService_SearchChangeRequests_NumberFilterPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changeRequests": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{Number: strPtr("CHG0010001")},
	}
	if _, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["number"] != "CHG0010001" {
		t.Fatalf("filters.number: got %v, want %q", gotFilters["number"], "CHG0010001")
	}
	if _, hasSearchQuery := gotFilters["searchQuery"]; hasSearchQuery {
		t.Fatalf("filters.searchQuery: expected omitted (empty), got %v", gotFilters["searchQuery"])
	}
}

// TestSNChangeRequestService_SearchChangeRequests_NewAssessAuthorizeStatesAccepted
// verifies the New/Assess/Authorize states -- already fully wired end-to-end
// (domain enum, SN key mapping) except for validChangeRequestState -- no longer
// fail search validation and reach the outgoing payload with the correct SN
// numeric state keys (-5/-4/-3).
func TestSNChangeRequestService_SearchChangeRequests_NewAssessAuthorizeStatesAccepted(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changeRequests": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			States: []domain.ChangeRequestState{
				domain.ChangeRequestStateNew,
				domain.ChangeRequestStateAssess,
				domain.ChangeRequestStateAuthorize,
			},
		},
	}
	if _, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	gotStateKeys, ok := gotFilters["stateKeys"].([]any)
	if !ok || len(gotStateKeys) != 3 {
		t.Fatalf("filters.stateKeys: got %v, want [-5, -4, -3]", gotFilters["stateKeys"])
	}
	want := []float64{-5, -4, -3}
	for i, w := range want {
		if gotStateKeys[i] != w {
			t.Fatalf("filters.stateKeys[%d]: got %v, want %v", i, gotStateKeys[i], w)
		}
	}
}

// TestSNChangeRequestService_SearchChangeRequests_NewFiltersPassedThrough verifies
// the generic filters array's createdOn (gte/lte) and assignmentGroupId (in)
// predicates translate into createdStartDate/createdEndDate/assignmentGroupIds
// on the outgoing payload under the exact wire names Ballerina accepts,
// mirroring the existing closedStartDate/closedEndDate coverage.
func TestSNChangeRequestService_SearchChangeRequests_NewFiltersPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changeRequests": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{start.Format(time.RFC3339)}},
				{Field: "createdOn", Op: "lte", Values: []string{end.Format(time.RFC3339)}},
				{Field: "assignmentGroupId", Op: "in", Values: []string{testCaseUUID}},
			},
		},
	}
	if _, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["createdStartDate"] != formatSNDateTimeUTC(&start) {
		t.Fatalf("filters.createdStartDate: got %v, want %q", gotFilters["createdStartDate"], formatSNDateTimeUTC(&start))
	}
	if gotFilters["createdEndDate"] != formatSNDateTimeUTC(&end) {
		t.Fatalf("filters.createdEndDate: got %v, want %q", gotFilters["createdEndDate"], formatSNDateTimeUTC(&end))
	}
	gotAssignmentGroupIDs, ok := gotFilters["assignmentGroupIds"].([]any)
	if !ok || len(gotAssignmentGroupIDs) != 1 || gotAssignmentGroupIDs[0] != uuidToSysid(testCaseUUID) {
		t.Fatalf("filters.assignmentGroupIds: got %v, want [%q] (raw UUID must not be sent to SN)", gotFilters["assignmentGroupIds"], uuidToSysid(testCaseUUID))
	}
}

// TestSNChangeRequestService_SearchChangeRequests_ApprovalFilterPassedThrough
// verifies the generic filters array's "approval" predicate translates into
// filters.approval on the outgoing payload under the exact raw ServiceNow
// task.approval value, unchanged (no key/enum mapping).
func TestSNChangeRequestService_SearchChangeRequests_ApprovalFilterPassedThrough(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changeRequests": [], "totalRecords": 0, "offset": 0, "limit": 20}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "approval", Op: "eq", Values: []string{"approved"}},
			},
		},
	}
	if _, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotFilters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in payload, got %+v", gotBody["filters"])
	}
	if gotFilters["approval"] != "approved" {
		t.Fatalf("filters.approval: got %v, want %q", gotFilters["approval"], "approved")
	}
}

// TestSNChangeRequestService_SearchChangeRequests_ApprovalInvalidValueRejected
// verifies a malformed approval filter value is rejected with a clean
// validation error before any SN call.
func TestSNChangeRequestService_SearchChangeRequests_ApprovalInvalidValueRejected(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowChangeRequestService(nil)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "approval", Op: "eq", Values: []string{"maybe"}},
			},
		},
	}
	_, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_SearchChangeRequests_CreatedEndDateBeforeStart verifies
// a createdOn lte predicate earlier than its own gte predicate is rejected,
// mirroring the existing closedEndDate/closedStartDate ordering check.
func TestSNChangeRequestService_SearchChangeRequests_CreatedEndDateBeforeStart(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowChangeRequestService(nil)

	start := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{start.Format(time.RFC3339)}},
				{Field: "createdOn", Op: "lte", Values: []string{end.Format(time.RFC3339)}},
			},
		},
	}
	_, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_SearchChangeRequests_CreatedOnMultipleValuesRejected
// verifies a createdOn predicate carrying more than one value is rejected rather
// than silently using only Values[0] and discarding the rest.
func TestSNChangeRequestService_SearchChangeRequests_CreatedOnMultipleValuesRejected(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowChangeRequestService(nil)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{start.Format(time.RFC3339), end.Format(time.RFC3339)}},
			},
		},
	}
	_, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_SearchChangeRequests_InvalidFilterField verifies an
// unsupported filters[] field name is rejected before any SN call.
func TestSNChangeRequestService_SearchChangeRequests_InvalidFilterField(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowChangeRequestService(nil)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "notAField", Op: "in", Values: []string{"x"}},
			},
		},
	}
	_, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_SearchChangeRequests_AssignmentGroupIdInvalidUUID
// verifies a malformed assignmentGroupId filter value is rejected with a clean
// validation error before any SN call.
func TestSNChangeRequestService_SearchChangeRequests_AssignmentGroupIdInvalidUUID(t *testing.T) {
	// client is intentionally nil: validation must fail before touching it.
	svc := NewServiceNowChangeRequestService(nil)

	req := domain.SearchChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{
			Filters: []domain.ChangeRequestFieldFilter{
				{Field: "assignmentGroupId", Op: "in", Values: []string{"not-a-uuid"}},
			},
		},
	}
	_, err := svc.SearchChangeRequests(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}
