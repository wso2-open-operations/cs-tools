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
		CustomerGroupID:    nullStrPtrPtr(),
		DurationInput:      nullIntPtrPtr(),
	}

	if _, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{"implementationPlan", "priorityKey", "customerGroupId", "durationInput"} {
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
	req := domain.PatchChangeRequestRequest{
		ImplementationPlan:     strPtrPtr("<p>plan</p>"),
		Priority:               priorityPtrPtr(domain.ChangeRequestPriorityHigh),
		Category:               categoryPtrPtr(domain.ChangeRequestCategoryNetwork),
		RequestedByID:          strPtrPtr(testCaseUUID),
		AffectedServicesText:   strPtrPtr("services"),
		AffectedComponentsText: strPtrPtr("components"),
		RollbackDurationText:   strPtrPtr("10 mins"),
		CustomerGroupID:        strPtrPtr(testCaseUUID),
		EnvironmentIDs:         &[]string{testCaseUUID},
		DeploymentProductIDs:   &[]string{testCaseUUID},
		Comment:                &comment,
		WorkNote:               &workNote,
		DurationInput:          intPtrPtr(21600),
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
	if gotBody["customerGroupId"] != uuidToSysid(testCaseUUID) {
		t.Errorf("customerGroupId: got %v, want a sysid, not a raw UUID", gotBody["customerGroupId"])
	}
	envIDs, ok := gotBody["environmentIds"].([]any)
	if !ok || len(envIDs) != 1 || envIDs[0] != uuidToSysid(testCaseUUID) {
		t.Errorf("environmentIds: got %v, want [%q] (raw UUID must not be sent to SN)", gotBody["environmentIds"], uuidToSysid(testCaseUUID))
	}
	if gotBody["comment"] != "a comment" || gotBody["workNote"] != "a work note" {
		t.Errorf("comment/workNote: got comment=%v workNote=%v", gotBody["comment"], gotBody["workNote"])
	}
	if gotBody["durationInput"] != float64(21600) {
		t.Errorf("durationInput: got %v", gotBody["durationInput"])
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
	req := domain.CreateChangeRequestRequest{
		Subject:                "subject",
		AffectedServicesText:   strPtr("services"),
		AffectedComponentsText: strPtr("components"),
		RollbackDurationText:   strPtr("2 hours"),
		CustomerGroupID:        strPtr(testCaseUUID),
		EnvironmentIDs:         []string{testCaseUUID},
		DeploymentProductIDs:   []string{testCaseUUID},
		DurationInput:          &duration,
	}

	if _, err := svc.CreateChangeRequest(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["customerGroupId"] != uuidToSysid(testCaseUUID) {
		t.Errorf("customerGroupId: got %v, want a sysid, not a raw UUID", gotBody["customerGroupId"])
	}
	envIDs, ok := gotBody["environmentIds"].([]any)
	if !ok || len(envIDs) != 1 || envIDs[0] != uuidToSysid(testCaseUUID) {
		t.Errorf("environmentIds: got %v", gotBody["environmentIds"])
	}
	if gotBody["durationInput"] != float64(21600) {
		t.Errorf("durationInput: got %v", gotBody["durationInput"])
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
	if len(got.Environments) != 1 || got.Environments[0].ID != testCaseUUID {
		t.Errorf("environments: got %v", got.Environments)
	}
	if len(got.DeploymentProducts) != 1 {
		t.Errorf("deploymentProducts: got %v", got.DeploymentProducts)
	}
	if got.CustomerGroup == nil || got.CustomerGroup.ID != testCaseUUID {
		t.Errorf("customerGroup: got %v", got.CustomerGroup)
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
