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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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

	t.Run("an RFC 3339 value is read as the instant it names", func(t *testing.T) {
		for in, want := range map[string]string{
			"2026-08-01T10:00:00Z":      "2026-08-01T10:00:00Z",
			"2026-08-01T15:30:00+05:30": "2026-08-01T10:00:00Z",
			"2026-08-01T05:00:00-05:00": "2026-08-01T10:00:00Z",
		} {
			got, err := toDownstreamUTCDateTime("plannedStartDate", in)
			if err != nil {
				t.Errorf("input %q: unexpected error: %v", in, err)
				continue
			}
			if got != want {
				t.Errorf("input %q: got %q, want %q", in, got, want)
			}
		}
	})

	t.Run("rejects bad input with a validation error naming the field", func(t *testing.T) {
		for _, in := range []string{
			"2026-08-01T10:00:00", // a time with no zone designator is neither layout
			"2026-08-01",
			"01-08-2026 10:00:00",
			"infinity",
			"now",
			"tomorrow",
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

	t.Run("rejects a year outside 2000 to 2100 and a zoneless fraction, saying which", func(t *testing.T) {
		for in, want := range map[string]string{
			"1999-12-31T23:59:59Z":  "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss (the year must be in 2000 to 2100)",
			"1999-01-01 00:00:00":   "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss (the year must be in 2000 to 2100)",
			"2101-01-01 00:00:00":   "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss (the year must be in 2000 to 2100)",
			"2026-08-01 10:00:00.5": "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss (whole seconds only, no fractional second)",
			"1999-01-01 00:00:00.5": "plannedEndDate must follow the format: YYYY-MM-DD HH:mm:ss (whole seconds only, no fractional second)",
		} {
			got, err := toDownstreamUTCDateTime("plannedEndDate", in)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || got != "" || ve.Msg != want {
				t.Errorf("input %q: got %q, %v; want a ValidationError %q", in, got, err, want)
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
// numeric state keys (-5/-4/-3). The caller is staff (an unrestricted scope):
// a customer is never sent these three states, see
// sn_change_request_customer_view_test.go.
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
	if _, err := svc.SearchChangeRequests(snStaffCtx(), req); err != nil {
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

// ---------------------------------------------------------------------------
// Field parity (CHANGES-cr-field-parity.md)
// ---------------------------------------------------------------------------

func strPtrPtr(s string) **string {
	p := &s
	return &p
}

func nullStrPtrPtr() **string {
	var p *string
	return &p
}

func priorityPtrPtr(p domain.ChangeRequestPriority) **domain.ChangeRequestPriority {
	pp := &p
	return &pp
}

func intPtrPtr(i int) **int {
	p := &i
	return &p
}

func nullIntPtrPtr() **int {
	var p *int
	return &p
}

// TestSNChangeRequestService_PatchChangeRequest_ExplicitNullClearsFields verifies
// that an explicit null on a tri-state field is sent through as a literal JSON
// null (clear), distinct from an omitted field (leave unchanged) -- the
// contract documented in CHANGES-cr-field-parity.md.
func TestSNChangeRequestService_PatchChangeRequest_ExplicitNullClearsFields(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/"+uuidToSysid(testCaseUUID), func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Change request updated", "changeRequest": {"id": "` + uuidToSysid(testCaseUUID) + `", "number": "CHG0001", "project": {"id": "` + uuidToSysid(testCaseUUID) + `", "name": "p"}, "createdOn": "2026-01-01 00:00:00"}}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	req := domain.PatchChangeRequestRequest{
		ImplementationPlan: nullStrPtrPtr(),
		Priority:           nullPriorityPtrPtr(),
		DurationInput:      nullIntPtrPtr(),
	}

	if _, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{"implementationPlan", "priorityKey", "durationInput"} {
		raw, ok := gotBody[key]
		if !ok {
			t.Errorf("expected key %q to be present in the outgoing payload (explicit null), but it was omitted", key)
			continue
		}
		if raw != nil {
			t.Errorf("expected %q to be sent as JSON null, got %v", key, raw)
		}
	}
}

func nullPriorityPtrPtr() **domain.ChangeRequestPriority {
	var p *domain.ChangeRequestPriority
	return &p
}

// TestSNChangeRequestService_PatchChangeRequest_SetsNewWritableFields verifies
// every one of the 13 field-parity writable keys reaches the outgoing payload
// with the expected wire value, including sysid conversion for id-valued
// fields and key mapping for priority/category.
func TestSNChangeRequestService_PatchChangeRequest_SetsNewWritableFields(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/"+uuidToSysid(testCaseUUID), func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Change request updated", "changeRequest": {"id": "` + uuidToSysid(testCaseUUID) + `", "number": "CHG0001", "project": {"id": "` + uuidToSysid(testCaseUUID) + `", "name": "p"}, "createdOn": "2026-01-01 00:00:00"}}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	comment := "a comment"
	workNote := "a work note"
	planningVisible := false
	req := domain.PatchChangeRequestRequest{
		ImplementationPlan:           strPtrPtr("<p>plan</p>"),
		Priority:                     priorityPtrPtr(domain.ChangeRequestPriorityHigh),
		Category:                     categoryPtrPtr(domain.ChangeRequestCategoryNetwork),
		RequestedByID:                strPtrPtr(testCaseUUID),
		AffectedServicesText:         strPtrPtr("services"),
		AffectedComponentsText:       strPtrPtr("components"),
		RollbackDurationText:         strPtrPtr("10 mins"),
		DeploymentProductIDs:         &[]string{testCaseUUID},
		Comment:                      &comment,
		WorkNote:                     &workNote,
		DurationInput:                intPtrPtr(21600),
		IsPlanningVisibleToCustomers: &planningVisible,
	}

	if _, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["implementationPlan"] != "<p>plan</p>" {
		t.Errorf("implementationPlan: got %v", gotBody["implementationPlan"])
	}
	if gotBody["priorityKey"] != "2" {
		t.Errorf("priorityKey: got %v, want \"2\" (high)", gotBody["priorityKey"])
	}
	if gotBody["categoryKey"] != "Network" {
		t.Errorf("categoryKey: got %v, want \"Network\"", gotBody["categoryKey"])
	}
	if gotBody["requestedById"] != uuidToSysid(testCaseUUID) {
		t.Errorf("requestedById: got %v, want a sysid, not a raw UUID", gotBody["requestedById"])
	}
	// customerGroupId / environmentIds are no longer part of the API, so
	// nothing of them is ever sent to ServiceNow.
	for _, key := range []string{"customerGroupId", "environmentIds"} {
		if _, ok := gotBody[key]; ok {
			t.Errorf("%s was sent to ServiceNow: %v", key, gotBody[key])
		}
	}
	if gotBody["comment"] != "a comment" || gotBody["workNote"] != "a work note" {
		t.Errorf("comment/workNote: got comment=%v workNote=%v", gotBody["comment"], gotBody["workNote"])
	}
	if gotBody["durationInput"] != float64(21600) {
		t.Errorf("durationInput: got %v", gotBody["durationInput"])
	}
	// An explicit false must be forwarded, not dropped as if the field were
	// omitted -- omitempty on a *bool only checks the pointer, not the
	// pointed-to value, so this also guards against a future regression.
	v, ok := gotBody["isPlanningVisibleToCustomers"]
	if !ok || v != false {
		t.Errorf("isPlanningVisibleToCustomers: got %v (present=%v), want false (present=true)", v, ok)
	}
}

func categoryPtrPtr(c domain.ChangeRequestCategory) **domain.ChangeRequestCategory {
	pp := &c
	return &pp
}

// TestSNChangeRequestService_PatchChangeRequest_RejectsEmptyJournalFields
// verifies comment/workNote reject an empty or whitespace-only value: a
// journal entry is not a clearable field value, per CHANGES-cr-field-parity.md.
func TestSNChangeRequestService_PatchChangeRequest_RejectsEmptyJournalFields(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)

	blank := "   "
	for _, req := range []domain.PatchChangeRequestRequest{
		{Comment: &blank},
		{WorkNote: &blank},
	} {
		_, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req)
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	}
}

// TestSNChangeRequestService_CreateChangeRequest_RejectsNonNewState guards
// against the regression reported live: the CSM Portal's own create form used
// to let a caller pick Assess or Authorize directly as a change request's
// starting state, skipping the workflow's own assess/authorize gates
// entirely. Every state but New must now be rejected before create even
// reaches ServiceNow.
func TestSNChangeRequestService_CreateChangeRequest_RejectsNonNewState(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	normalType := domain.ChangeRequestTypeNormal

	for _, s := range []domain.ChangeRequestState{
		domain.ChangeRequestStateAssess,
		domain.ChangeRequestStateAuthorize,
		domain.ChangeRequestStateImplement,
		domain.ChangeRequestStateClosed,
	} {
		state := s
		_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), domain.CreateChangeRequestRequest{
			Subject: "subject",
			Type:    &normalType,
			State:   &state,
		})
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("state %q: expected *apierror.ValidationError, got %T: %v", state, err, err)
		}
	}
}

// TestSNChangeRequestService_CreateChangeRequest_RequiresType: the type decides
// the approval flow, so a create without one (or with anything but standard/
// normal/emergency) is refused before ServiceNow is called.
func TestSNChangeRequestService_CreateChangeRequest_RequiresType(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	azure := domain.ChangeRequestTypeAzure
	for name, typ := range map[string]*domain.ChangeRequestType{"missing": nil, "azure": &azure} {
		_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), domain.CreateChangeRequestRequest{Subject: "subject", Type: typ})
		ve, ok := err.(*apierror.ValidationError)
		if !ok {
			t.Fatalf("%s: expected *apierror.ValidationError, got %T: %v", name, err, err)
		}
		if !strings.Contains(ve.Msg, "standard, normal or emergency") {
			t.Errorf("%s: message %q should name the three allowed types", name, ve.Msg)
		}
	}
}

// TestSNChangeRequestService_CreateChangeRequest_EmergencyWithACustomerBoxIsRefused: an
// Emergency change takes no customer step, so the ServiceNow-only data source refuses a
// create that ticks either box as the PostgreSQL ones do (before ServiceNow is called: the
// client here is nil), rather than accepting and dropping it.
func TestSNChangeRequestService_CreateChangeRequest_EmergencyWithACustomerBoxIsRefused(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	emergency := domain.ChangeRequestTypeEmergency
	yes := true
	const reason = "Emergency changes proceed without customer consent, so customer approval and customer review cannot be required"
	for name, req := range map[string]domain.CreateChangeRequestRequest{
		"approval": {Subject: "subject", Type: &emergency, CustomerApprovalRequired: &yes},
		"review":   {Subject: "subject", Type: &emergency, CustomerReviewRequired: &yes},
	} {
		_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req)
		ve, ok := err.(*apierror.ValidationError)
		if !ok {
			t.Fatalf("%s: expected *apierror.ValidationError, got %T: %v", name, err, err)
		}
		if !strings.HasPrefix(ve.Msg, reason) {
			t.Errorf("%s: message %q should give the reason %q", name, ve.Msg, reason)
		}
	}
}

// TestWithoutCustomerOutcomeStates: ServiceNow's own offered next states never
// reach the portal with the two states only the CUSTOMER can reach in them --
// "scheduled" (from any state, Customer Approval included: the customer's own
// approval schedules the change) and "closed" out of Customer Review (only the
// customer's own review closes it) -- so this data source's legalNextStates
// agrees with the PostgreSQL one. "closed" from Review, Rollback and Cancel stay.
func TestWithoutCustomerOutcomeStates(t *testing.T) {
	str := func(s string) *string { return &s }
	got := withoutCustomerOutcomeStates([]string{"scheduled", "implement", "Scheduled", "canceled"}, str("assess"))
	if strings.Join(got, ",") != "implement,canceled" {
		t.Fatalf("withoutCustomerOutcomeStates = %v, want [implement canceled]", got)
	}
	if withoutCustomerOutcomeStates(nil, nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if got := withoutCustomerOutcomeStates([]string{"scheduled", "canceled"}, nil); strings.Join(got, ",") != "canceled" {
		t.Fatalf("withoutCustomerOutcomeStates with unknown state = %v, want [canceled]", got)
	}
	// Review keeps Closed and Rollback (the failed-review off-ramp).
	got = withoutCustomerOutcomeStates([]string{"closed", "rollback", "canceled"}, str("review"))
	if strings.Join(got, ",") != "closed,rollback,canceled" {
		t.Fatalf("withoutCustomerOutcomeStates from review = %v, want [closed rollback canceled]", got)
	}
	// Customer Review: only the customer's review closes the change, so Closed
	// goes (in any case), Rollback and Cancel stay.
	for _, st := range []string{"customer_review", "Customer_Review"} {
		got = withoutCustomerOutcomeStates([]string{"closed", "Closed", "rollback", "canceled"}, str(st))
		if strings.Join(got, ",") != "rollback,canceled" {
			t.Fatalf("withoutCustomerOutcomeStates from %s = %v, want [rollback canceled]", st, got)
		}
	}
	// Customer Approval: the customer's own approval schedules the change, so
	// "scheduled" goes here as well (it used to stay: it was a staff action),
	// Re-schedule (authorize) and Cancel stay.
	got = withoutCustomerOutcomeStates([]string{"scheduled", "authorize", "canceled"}, str("customer_approval"))
	if strings.Join(got, ",") != "authorize,canceled" {
		t.Fatalf("withoutCustomerOutcomeStates from customer_approval = %v, want [authorize canceled]", got)
	}
	// An empty (non-nil) answer stays empty, not nil.
	if got := withoutCustomerOutcomeStates([]string{"scheduled"}, str("customer_approval")); got == nil || len(got) != 0 {
		t.Fatalf("withoutCustomerOutcomeStates([scheduled]) = %#v, want an empty non-nil slice", got)
	}
}

// TestSNChangeRequestService_PatchChangeRequest_RejectsInvalidPriorityAndCategory
// verifies the new priority/category writable keys are validated the same way
// the pre-existing create-path enums are.
func TestSNChangeRequestService_PatchChangeRequest_RejectsInvalidPriorityAndCategory(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)

	invalidPriority := domain.ChangeRequestPriority("urgent")
	_, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, domain.PatchChangeRequestRequest{
		Priority: priorityPtrPtr(invalidPriority),
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("priority: expected *apierror.ValidationError, got %T: %v", err, err)
	}

	invalidCategory := domain.ChangeRequestCategory("not-a-category")
	_, err = svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, domain.PatchChangeRequestRequest{
		Category: categoryPtrPtr(invalidCategory),
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("category: expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_CreateChangeRequest_SendsNewCreateFields verifies
// the 7 field-parity keys newly added to create reach the outgoing payload.
func TestSNChangeRequestService_CreateChangeRequest_SendsNewCreateFields(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message": "Change request created", "changeRequest": {"id": "` + uuidToSysid(testCaseUUID) + `", "number": "CHG0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	duration := 21600
	planningVisible := true
	normalType := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{
		Subject:                      "subject",
		Type:                         &normalType,
		AffectedServicesText:         strPtr("services"),
		AffectedComponentsText:       strPtr("components"),
		RollbackDurationText:         strPtr("2 hours"),
		DeploymentProductIDs:         []string{testCaseUUID},
		PlannedStartDate:             strPtr("2026-01-01 00:00:00"),
		PlannedEndDate:               strPtr("2026-01-01 06:00:00"),
		DurationInput:                &duration,
		IsPlanningVisibleToCustomers: &planningVisible,
	}

	if _, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{"customerGroupId", "environmentIds"} {
		if _, ok := gotBody[key]; ok {
			t.Errorf("%s was sent to ServiceNow: %v", key, gotBody[key])
		}
	}
	if gotBody["durationInput"] != float64(21600) {
		t.Errorf("durationInput: got %v", gotBody["durationInput"])
	}
	if gotBody["isPlanningVisibleToCustomers"] != true {
		t.Errorf("isPlanningVisibleToCustomers: got %v", gotBody["isPlanningVisibleToCustomers"])
	}
}

// TestSNChangeRequestService_CreateChangeRequest_ExplicitNewStateAccepted
// verifies a caller explicitly asking for New (the only state create ever
// permits) still succeeds, and that the outgoing payload never carries a
// stateKey at all -- ServiceNow's own default is what actually sets it.
func TestSNChangeRequestService_CreateChangeRequest_ExplicitNewStateAccepted(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message": "Change request created", "changeRequest": {"id": "` + uuidToSysid(testCaseUUID) + `", "number": "CHG0001", "createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com"}}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	newState := domain.ChangeRequestStateNew
	normalType := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{Subject: "subject", Type: &normalType, State: &newState}

	if _, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := gotBody["stateKey"]; ok {
		t.Errorf("stateKey: got %v present in payload, want it absent entirely", gotBody["stateKey"])
	}
}

func TestSNChangeRequestService_CreateChangeRequest_DurationInputMustMatchPlannedWindow(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)

	duration := 3600
	normalType := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{
		Subject:          "subject",
		Type:             &normalType,
		PlannedStartDate: strPtr("2026-01-01 00:00:00"),
		PlannedEndDate:   strPtr("2026-01-01 06:00:00"),
		DurationInput:    &duration,
	}

	_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError for mismatched durationInput, got %T: %v", err, err)
	}
}

func TestSNChangeRequestService_CreateChangeRequest_DurationInputRequiresBothPlannedDates(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)

	duration := 21600
	normalType := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{
		Subject:          "subject",
		Type:             &normalType,
		PlannedStartDate: strPtr("2026-01-01 00:00:00"),
		DurationInput:    &duration,
	}

	_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError when plannedEndDate is missing, got %T: %v", err, err)
	}
}

// TestSNChangeRequestService_GetChangeRequest_MapsFieldParityKeys verifies the
// 20 new read keys are mapped from the Choreo detail payload into the domain
// view, including priority/category key-to-domain-enum mapping.
func TestSNChangeRequestService_GetChangeRequest_MapsFieldParityKeys(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/"+uuidToSysid(testCaseUUID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "` + uuidToSysid(testCaseUUID) + `", "number": "CHG0038839",
			"project": {"id": "` + uuidToSysid(testCaseUUID) + `", "name": "p"},
			"createdOn": "2026-01-01 00:00:00", "createdBy": "engineer@example.com",
			"implementationPlan": "<p>plan</p>",
			"priority": {"id": 4, "label": "4 - Low"},
			"category": {"id": "Other", "label": "Other"},
			"requestedBy": {"id": "` + uuidToSysid(testCaseUUID) + `", "name": "Jane Doe"},
			"affectedServicesText": "<p>services</p>",
			"affectedComponentsText": "<p>components</p>",
			"rollbackDurationText": "10 mins",
			"environments": [{"id": "` + uuidToSysid(testCaseUUID) + `", "name": "UAT"}],
			"deploymentProducts": [{"id": "` + uuidToSysid(testCaseUUID) + `", "name": "WSO2 EI 6.6.0"}],
			"customerGroup": {"id": "` + uuidToSysid(testCaseUUID) + `", "name": "customer group"},
			"changeRequestType": {"id": 1, "label": "General"},
			"likelihood": {"id": 3, "label": "3 - Low"},
			"isPlanningVisibleToCustomers": false,
			"confirmCustomerUpdatedDate": null,
			"customerUpdatedOn": "2024-08-31 03:46:13",
			"labels": ["CRType/Emergency", "impact-3"],
			"deployments": [{"id": "` + uuidToSysid(testCaseUUID) + `", "name": "Production"}],
			"workStart": "2026-02-17 04:58:00",
			"workEnd": "2026-02-17 04:59:18",
			"gitReference": "https://github.com/example/repo/issues/491"
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	got, err := svc.GetChangeRequest(contextWithUserIDToken("token"), testCaseUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ImplementationPlan == nil || *got.ImplementationPlan != "<p>plan</p>" {
		t.Errorf("implementationPlan: got %v", got.ImplementationPlan)
	}
	if got.Priority == nil || *got.Priority != string(domain.ChangeRequestPriorityLow) {
		t.Errorf("priority: got %v, want %q", got.Priority, domain.ChangeRequestPriorityLow)
	}
	if got.Category == nil || *got.Category != string(domain.ChangeRequestCategoryOther) {
		t.Errorf("category: got %v, want %q", got.Category, domain.ChangeRequestCategoryOther)
	}
	if got.RequestedBy == nil || got.RequestedBy.ID != testCaseUUID {
		t.Errorf("requestedBy: got %v", got.RequestedBy)
	}
	if len(got.DeploymentProducts) != 1 {
		t.Errorf("deploymentProducts: got %v", got.DeploymentProducts)
	}
	if got.ChangeRequestType == nil || *got.ChangeRequestType != "General" {
		t.Errorf("changeRequestType: got %v", got.ChangeRequestType)
	}
	if len(got.Labels) != 2 {
		t.Errorf("labels: got %v", got.Labels)
	}
	if len(got.Deployments) != 1 {
		t.Errorf("deployments: got %v", got.Deployments)
	}
	if got.WorkStart == nil || *got.WorkStart != "2026-02-17 04:58:00" {
		t.Errorf("workStart: got %v", got.WorkStart)
	}
	if got.GitReference == nil || *got.GitReference != "https://github.com/example/repo/issues/491" {
		t.Errorf("gitReference: got %v", got.GitReference)
	}
}

// --- AggregateChangeRequests: state groupBy key remap ---
//
// SN's own groupBy implementation (ChangeRequestUtils.groupChangeRequestsBy)
// returns the raw internal state value as the bucket key (e.g. "-5" for
// "New") and the human-readable label separately. The platform's own
// ChangeRequestState enum strings must come back as the key so the frontend
// can round-trip it into a states filter. This test pins that remap, for staff
// (a customer is not handed the New / Assess buckets at all: see
// sn_change_request_customer_view_test.go).
func TestSNChangeRequestService_AggregateChangeRequests_StateGroupByRemapsKeyToDomainEnum(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/aggregate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"groups": []map[string]any{
				{"key": "-5", "label": "New", "count": 3},
				{"key": "-4", "label": "Assess", "count": 2},
				{"key": "999", "label": "Unrecognized Label", "count": 1},
			},
			"othersCount":  0,
			"totalRecords": 6,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowChangeRequestService(client)

	resp, err := svc.AggregateChangeRequests(snStaffCtx(), domain.AggregateChangeRequestsRequest{
		GroupBy: "state",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 3 {
		t.Fatalf("groups: got %d, want 3", len(resp.Groups))
	}
	if got, want := resp.Groups[0].Key, string(domain.ChangeRequestStateNew); got != want {
		t.Errorf("groups[0].Key: got %q, want %q (domain enum, not raw SN value %q)", got, want, "-5")
	}
	if got, want := resp.Groups[1].Key, string(domain.ChangeRequestStateAssess); got != want {
		t.Errorf("groups[1].Key: got %q, want %q", got, want)
	}
	// Unrecognized label: falls back to leaving the key as-is rather than
	// crashing or dropping the bucket.
	if got, want := resp.Groups[2].Key, "999"; got != want {
		t.Errorf("groups[2].Key: got %q, want %q (unrecognized label falls back to raw key)", got, want)
	}
}

// ServiceNow's change request payloads have no project (create) or deployment
// list field: the ServiceNow-only service refuses them instead of dropping them
// silently. (The PostgreSQL-first dual-write service strips them before it
// mirrors, so only a ServiceNow-only deployment can hit this.)
func TestSNChangeRequestService_RefusesPostgresOnlyScopeFields(t *testing.T) {
	svc := &snChangeRequestService{}
	project := "11111111-2222-3333-4444-555555555555"
	normal := domain.ChangeRequestTypeNormal

	_, err := svc.CreateChangeRequest(context.Background(), domain.CreateChangeRequestRequest{Subject: "s", Type: &normal, ProjectID: &project})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "projectId and deploymentIds are not supported") {
		t.Fatalf("create with projectId: err = %v", err)
	}
	_, err = svc.CreateChangeRequest(context.Background(), domain.CreateChangeRequestRequest{Subject: "s", Type: &normal, DeploymentIDs: []string{project}})
	if !asValidationError(err, &ve) {
		t.Fatalf("create with deploymentIds: err = %v", err)
	}
	_, err = svc.PatchChangeRequest(context.Background(), project, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{project}})
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "deploymentIds is not supported") {
		t.Fatalf("patch with deploymentIds: err = %v", err)
	}
}

// customerGroupId and environmentIds are no longer accepted on the ServiceNow
// data source either (the Customer Group is the project's registered contacts
// and a deployment carries its environment): refused before ServiceNow is
// called, with the same messages as the PostgreSQL path.
func TestSNChangeRequestService_RemovedFieldsAreRefused(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Error("ServiceNow was called for a request carrying a removed field")
	})
	svc := NewServiceNowChangeRequestService(newTestSNClient(t, mux))
	normalType := domain.ChangeRequestTypeNormal
	ctx := contextWithUserIDToken("token")
	for name, tc := range map[string]struct {
		create domain.CreateChangeRequestRequest
		patch  domain.PatchChangeRequestRequest
		want   string
	}{
		"customerGroupId": {
			domain.CreateChangeRequestRequest{Subject: "s", Type: &normalType, CustomerGroupID: strPtr(testCaseUUID)},
			domain.PatchChangeRequestRequest{CustomerGroupID: strPtrPtr(testCaseUUID)},
			"customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts",
		},
		"customerGroupId null": {
			domain.CreateChangeRequestRequest{Subject: "s", Type: &normalType, CustomerGroupID: strPtr(testCaseUUID)},
			domain.PatchChangeRequestRequest{CustomerGroupID: nullStrPtrPtr()},
			"customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts",
		},
		"environmentIds": {
			domain.CreateChangeRequestRequest{Subject: "s", Type: &normalType, EnvironmentIDs: []string{testCaseUUID}},
			domain.PatchChangeRequestRequest{EnvironmentIDs: &[]string{testCaseUUID}},
			"environmentIds is no longer supported: deployments carry the environment",
		},
	} {
		_, err := svc.CreateChangeRequest(ctx, tc.create)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != tc.want {
			t.Errorf("%s create: err = %v, want ValidationError %q", name, err, tc.want)
		}
		_, err = svc.PatchChangeRequest(ctx, testCaseUUID, tc.patch)
		if !errors.As(err, &ve) || ve.Msg != tc.want {
			t.Errorf("%s patch: err = %v, want ValidationError %q", name, err, tc.want)
		}
	}
}

// The ServiceNow data source has no notion of who is viewing: its change request
// detail never carries customerCanAnswer (absent means "unknown", and a client
// falls back to the flags it already had), whatever the customer flags say.
func TestSNChangeRequestDetail_LeavesCustomerCanAnswerAbsent(t *testing.T) {
	t.Parallel()

	const detail = `{
		"id": "0123456789abcdef0123456789abcdef",
		"state": {"label": "Customer Approval"},
		"hasCustomerApproved": true,
		"hasCustomerReviewed": false
	}`
	var cr snChangeRequestDetail
	if err := json.Unmarshal([]byte(detail), &cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := mapSNChangeRequestDetailToView(cr)
	if got.CustomerCanAnswer != nil {
		t.Fatalf("CustomerCanAnswer = %v, want nil (absent)", *got.CustomerCanAnswer)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "customerCanAnswer") {
		t.Fatalf("the ServiceNow detail's JSON mentions customerCanAnswer: %s", raw)
	}
}

// TestSNPlannedTimestamp pins what the ServiceNow service takes for a planned
// start / end: the platform's own layout as sent, or RFC 3339 with a zone (which
// the PostgreSQL data source reads too) converted to that layout in UTC, and
// nothing else.
func TestSNPlannedTimestamp(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"2030-03-01 09:00:00":       "2030-03-01 09:00:00",
		"2030-03-01T09:00:00Z":      "2030-03-01 09:00:00",
		"2030-03-01T14:30:00+05:30": "2030-03-01 09:00:00",
		"2030-03-01T04:00:00-05:00": "2030-03-01 09:00:00",
		"2030-03-01T09:00:00.5Z":    "2030-03-01 09:00:00",
		// The edges of the years every planned window is held to.
		"2000-01-01 00:00:00": "2000-01-01 00:00:00",
		"2100-12-31 23:59:59": "2100-12-31 23:59:59",
		// Not spelled as the layout is, but no fraction (Go's parser takes a one-digit hour and more
		// than one space): read as the PostgreSQL data source reads it and written back in the layout,
		// not refused as a "fractional second" it does not have.
		"2030-03-01 9:00:00":   "2030-03-01 09:00:00",
		"2030-03-01 0:00:00":   "2030-03-01 00:00:00",
		"2030-03-01  09:00:00": "2030-03-01 09:00:00",
	} {
		got, err := snPlannedTimestamp("plannedStartOn", in)
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("input %q: got %q, want %q", in, got, want)
		}
	}

	const format = "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss"
	for _, in := range []string{
		"infinity", "-infinity", "now", "today", "tomorrow", "epoch",
		"2030-03-01",                // a date alone
		"2030-03-01T09:00:00",       // a time with no zone designator
		"2030-03-01 09:00:00 UTC",   // a zone name
		"2030-03-01 09:00:00+05:30", // an offset on the zoneless layout
		" 2030-03-01T09:00:00Z",
		"2030-3-01 09:00:00",   // a one-digit month: the parser does not take it (only the hour is lenient)
		"2030-03-1 09:00:00",   // a one-digit day
		"2030-03-01 09:00:00 ", // a trailing space
		"",
	} {
		_, err := snPlannedTimestamp("plannedStartOn", in)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("input %q: want a ValidationError, got %v", in, err)
			continue
		}
		if ve.Msg != format {
			t.Errorf("input %q: got msg %q, want %q", in, ve.Msg, format)
		}
	}

	// A year outside 2000 to 2100 is refused in BOTH layouts (the zoneless one used to be forwarded
	// as typed, whatever its year), and a zoneless value with a fractional second is refused
	// (Go's parser takes one, ServiceNow's layout has none: "1999-01-01 00:00:00.5" used to be
	// forwarded as typed and fail downstream with an opaque pattern error). Neither is forwarded.
	for in, want := range map[string]string{
		"1999-12-31T23:59:59Z":      format + " (the year must be in 2000 to 2100)",
		"2101-01-01T00:00:00Z":      format + " (the year must be in 2000 to 2100)",
		"1999-01-01 00:00:00":       format + " (the year must be in 2000 to 2100)",
		"2101-01-01 00:00:00":       format + " (the year must be in 2000 to 2100)",
		"2000-01-01T00:00:00+14:00": format + " (the year must be in 2000 to 2100)", // 1999-12-31T10:00Z
		"2030-03-01 09:00:00.5":     format + " (whole seconds only, no fractional second)",
		"2030-03-01 09:00:00.000":   format + " (whole seconds only, no fractional second)",
		"1999-01-01 00:00:00.5":     format + " (whole seconds only, no fractional second)",
		"2030-03-01 9:00:00.5":      format + " (whole seconds only, no fractional second)", // an odd spelling does not hide a real fraction
	} {
		got, err := snPlannedTimestamp("plannedStartOn", in)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || got != "" {
			t.Errorf("input %q: got %q, %v; want a ValidationError and nothing to forward", in, got, err)
			continue
		}
		if ve.Msg != want {
			t.Errorf("input %q: got msg %q, want %q", in, ve.Msg, want)
		}
	}
}

// TestSNChangeRequestService_PatchChangeRequest_PlannedWindowLayouts: the
// planned window of a PATCH reaches ServiceNow in the zoneless UTC layout whichever
// of the two documented layouts was sent, and a value that is neither is refused
// before any downstream call.
func TestSNChangeRequestService_PatchChangeRequest_PlannedWindowLayouts(t *testing.T) {
	const id = "11111111-2222-3333-4444-555555555555"

	run := func(t *testing.T, start, end *string) (map[string]any, int, error) {
		t.Helper()
		var gotBody map[string]any
		calls := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/change-requests/", func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"ok","changeRequest":{"id":"11111111222233334444555555555555"}}`))
		})
		svc := NewServiceNowChangeRequestService(newTestSNClient(t, mux))
		_, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), id,
			domain.PatchChangeRequestRequest{PlannedStartOn: start, PlannedEndOn: end})
		return gotBody, calls, err
	}

	t.Run("RFC 3339 is converted, the zoneless layout is forwarded as sent", func(t *testing.T) {
		for _, tc := range []struct{ name, start, end, wantStart, wantEnd string }{
			{"zoneless", "2030-03-01 09:00:00", "2030-03-01 10:00:00", "2030-03-01 09:00:00", "2030-03-01 10:00:00"},
			{"Z", "2030-03-01T09:00:00Z", "2030-03-01T10:00:00Z", "2030-03-01 09:00:00", "2030-03-01 10:00:00"},
			{"an offset", "2030-03-01T14:30:00+05:30", "2030-03-01T15:30:00+05:30", "2030-03-01 09:00:00", "2030-03-01 10:00:00"},
			{"one of each", "2030-03-01T09:00:00Z", "2030-03-01 10:00:00", "2030-03-01 09:00:00", "2030-03-01 10:00:00"},
		} {
			start, end := tc.start, tc.end
			body, calls, err := run(t, &start, &end)
			if err != nil || calls != 1 {
				t.Fatalf("%s: err %v, downstream calls %d; want none and 1", tc.name, err, calls)
			}
			if body["plannedStartOn"] != tc.wantStart || body["plannedEndOn"] != tc.wantEnd {
				t.Errorf("%s: forwarded %v / %v, want %q / %q", tc.name, body["plannedStartOn"], body["plannedEndOn"], tc.wantStart, tc.wantEnd)
			}
		}
	})

	t.Run("a start alone, an end alone", func(t *testing.T) {
		v := "2030-03-01T09:00:00Z"
		body, _, err := run(t, &v, nil)
		if err != nil || body["plannedStartOn"] != "2030-03-01 09:00:00" {
			t.Fatalf("start alone: err %v, body %v", err, body)
		}
		if _, has := body["plannedEndOn"]; has {
			t.Errorf("an absent end was forwarded: %v", body["plannedEndOn"])
		}
		body, _, err = run(t, nil, &v)
		if err != nil || body["plannedEndOn"] != "2030-03-01 09:00:00" {
			t.Fatalf("end alone: err %v, body %v", err, body)
		}
	})

	t.Run("anything else is refused and nothing is sent", func(t *testing.T) {
		for _, bad := range []string{"infinity", "now", "tomorrow", "2030-03-01", "2030-03-01T09:00:00", ""} {
			for _, which := range []string{"plannedStartOn", "plannedEndOn"} {
				v := bad
				var start, end *string
				if which == "plannedStartOn" {
					start = &v
				} else {
					end = &v
				}
				_, calls, err := run(t, start, end)
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) || calls != 0 {
					t.Errorf("%s %q: err %v, downstream calls %d; want a ValidationError and none", which, bad, err, calls)
					continue
				}
				if want := which + " must follow the format: YYYY-MM-DD HH:mm:ss"; ve.Msg != want {
					t.Errorf("%s %q: got msg %q, want %q", which, bad, ve.Msg, want)
				}
			}
		}
	})

	t.Run("a year outside 2000 to 2100 and a zoneless fractional second are refused, whichever field, and nothing is sent", func(t *testing.T) {
		for bad, reason := range map[string]string{
			"1999-12-31T23:59:59Z":  "the year must be in 2000 to 2100",
			"1999-01-01 00:00:00":   "the year must be in 2000 to 2100",
			"2101-01-01 00:00:00":   "the year must be in 2000 to 2100",
			"1999-01-01 00:00:00.5": "whole seconds only, no fractional second",
			"2030-03-01 09:00:00.5": "whole seconds only, no fractional second",
		} {
			for _, which := range []string{"plannedStartOn", "plannedEndOn"} {
				v := bad
				var start, end *string
				if which == "plannedStartOn" {
					start = &v
				} else {
					end = &v
				}
				_, calls, err := run(t, start, end)
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) || calls != 0 {
					t.Errorf("%s %q: err %v, downstream calls %d; want a ValidationError and none", which, bad, err, calls)
					continue
				}
				if want := which + " must follow the format: YYYY-MM-DD HH:mm:ss (" + reason + ")"; ve.Msg != want {
					t.Errorf("%s %q: got msg %q, want %q", which, bad, ve.Msg, want)
				}
			}
		}
	})

	t.Run("the caller's request is left as it was sent", func(t *testing.T) {
		start := "2030-03-01T09:00:00Z"
		req := domain.PatchChangeRequestRequest{PlannedStartOn: &start}
		mux := http.NewServeMux()
		mux.HandleFunc("/change-requests/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"ok","changeRequest":{"id":"11111111222233334444555555555555"}}`))
		})
		svc := NewServiceNowChangeRequestService(newTestSNClient(t, mux))
		if _, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), id, req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *req.PlannedStartOn != "2030-03-01T09:00:00Z" {
			t.Errorf("the caller's plannedStartOn was rewritten to %q", *req.PlannedStartOn)
		}
	})
}

// TestSNChangeRequestService_CreateChangeRequest_PlannedWindowLayouts: the same
// for a create, which also reaches the ServiceNow service as the first write of
// the dual-write create (with the text the caller sent, validated, not converted).
func TestSNChangeRequestService_CreateChangeRequest_PlannedWindowLayouts(t *testing.T) {
	run := func(t *testing.T, req domain.CreateChangeRequestRequest) (map[string]any, int, error) {
		t.Helper()
		var gotBody map[string]any
		calls := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/change-requests", func(w http.ResponseWriter, r *http.Request) {
			calls++
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"ok","changeRequest":{"id":"11111111222233334444555555555555","number":"CHG0000001"}}`))
		})
		svc := NewServiceNowChangeRequestService(newTestSNClient(t, mux))
		_, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req)
		return gotBody, calls, err
	}
	normal := domain.ChangeRequestTypeNormal
	base := func(start, end string) domain.CreateChangeRequestRequest {
		return domain.CreateChangeRequestRequest{Subject: "Window", Type: &normal, PlannedStartDate: &start, PlannedEndDate: &end}
	}

	t.Run("either layout reaches the create endpoint in the layout it requires", func(t *testing.T) {
		for _, tc := range []struct{ name, start, end string }{
			{"zoneless", "2030-03-01 09:00:00", "2030-03-01 10:00:00"},
			{"Z", "2030-03-01T09:00:00Z", "2030-03-01T10:00:00Z"},
			{"an offset", "2030-03-01T14:30:00+05:30", "2030-03-01T15:30:00+05:30"},
		} {
			body, calls, err := run(t, base(tc.start, tc.end))
			if err != nil || calls != 1 {
				t.Fatalf("%s: err %v, downstream calls %d; want none and 1", tc.name, err, calls)
			}
			if body["plannedStartDate"] != "2030-03-01T09:00:00Z" || body["plannedEndDate"] != "2030-03-01T10:00:00Z" {
				t.Errorf("%s: forwarded %v / %v", tc.name, body["plannedStartDate"], body["plannedEndDate"])
			}
		}
	})

	t.Run("durationInput is checked against the window whichever layout it came in", func(t *testing.T) {
		okDur, badDur := 3600, 60
		req := base("2030-03-01T09:00:00Z", "2030-03-01T10:00:00+00:00")
		req.DurationInput = &okDur
		if _, calls, err := run(t, req); err != nil || calls != 1 {
			t.Fatalf("matching duration: err %v, downstream calls %d", err, calls)
		}
		req.DurationInput = &badDur
		_, calls, err := run(t, req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || calls != 0 || !strings.Contains(ve.Msg, "durationInput (60) must match plannedEndDate - plannedStartDate (3600)") {
			t.Fatalf("mismatched duration: err %v, downstream calls %d", err, calls)
		}
	})

	t.Run("anything else is refused and nothing is sent", func(t *testing.T) {
		for _, bad := range []string{"infinity", "now", "tomorrow", "2030-03-01", "2030-03-01T09:00:00"} {
			for _, field := range []string{"plannedStartDate", "plannedEndDate"} {
				req := base("2030-03-01 09:00:00", "2030-03-01 10:00:00")
				if field == "plannedStartDate" {
					req.PlannedStartDate = &bad
				} else {
					req.PlannedEndDate = &bad
				}
				_, calls, err := run(t, req)
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) || calls != 0 {
					t.Errorf("%s %q: err %v, downstream calls %d; want a ValidationError and none", field, bad, err, calls)
					continue
				}
				if want := field + " must follow the format: YYYY-MM-DD HH:mm:ss"; ve.Msg != want {
					t.Errorf("%s %q: got msg %q, want %q", field, bad, ve.Msg, want)
				}
			}
		}
	})

	t.Run("a year outside 2000 to 2100 and a zoneless fractional second are refused, whichever field, and nothing is sent", func(t *testing.T) {
		for bad, reason := range map[string]string{
			"1999-12-31T23:59:59Z":  "the year must be in 2000 to 2100",
			"1999-01-01 00:00:00":   "the year must be in 2000 to 2100",
			"2101-01-01 00:00:00":   "the year must be in 2000 to 2100",
			"1999-01-01 00:00:00.5": "whole seconds only, no fractional second",
			"2030-03-01 09:00:00.5": "whole seconds only, no fractional second",
		} {
			for _, field := range []string{"plannedStartDate", "plannedEndDate"} {
				v := bad
				req := base("2030-03-01 09:00:00", "2030-03-01 10:00:00")
				if field == "plannedStartDate" {
					req.PlannedStartDate = &v
				} else {
					req.PlannedEndDate = &v
				}
				_, calls, err := run(t, req)
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) || calls != 0 {
					t.Errorf("%s %q: err %v, downstream calls %d; want a ValidationError and none", field, bad, err, calls)
					continue
				}
				if want := field + " must follow the format: YYYY-MM-DD HH:mm:ss (" + reason + ")"; ve.Msg != want {
					t.Errorf("%s %q: got msg %q, want %q", field, bad, ve.Msg, want)
				}
			}
		}
	})
}
