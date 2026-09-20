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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testUUID = "99999999-0000-4000-8000-000000000001"

func str(s string) *string { return &s }
func num(n int) *int       { return &n }

// The repositories are deliberately nil: every case below must be rejected
// before any repository call, so a nil dereference would itself fail the test.
func TestCallRequestService_CreateValidation(t *testing.T) {
	svc := &callRequestService{}
	ok := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	tests := []struct {
		name string
		mod  func(*domain.CreateCallRequestRequest)
		want any
	}{
		{"missing caseId", func(r *domain.CreateCallRequestRequest) { r.CaseID = "" }, &apierror.ValidationError{}},
		{"malformed caseId", func(r *domain.CreateCallRequestRequest) { r.CaseID = "nope" }, &apierror.ValidationError{}},
		{"missing reason", func(r *domain.CreateCallRequestRequest) { r.Reason = "" }, &apierror.ValidationError{}},
		{"no utcTimes", func(r *domain.CreateCallRequestRequest) { r.UTCTimes = nil }, &apierror.ValidationError{}},
		{"non-positive duration", func(r *domain.CreateCallRequestRequest) { r.DurationMinutes = 0 }, &apierror.ValidationError{}},
		{"no caller token", func(r *domain.CreateCallRequestRequest) {}, &apierror.UnauthorizedError{}},
	}
	for _, tt := range tests {
		req := ok
		tt.mod(&req)
		_, err := svc.CreateCallRequest(context.Background(), req)
		requireErrKind(t, tt.name, err, tt.want)
	}
}

func TestCallRequestService_SearchValidation(t *testing.T) {
	svc := &callRequestService{}
	ctx := context.Background()

	_, err := svc.SearchCallRequests(ctx, domain.SearchCallRequestsRequest{})
	requireErrKind(t, "search without caseId", err, &apierror.ValidationError{})

	_, err = svc.SearchCallRequests(ctx, domain.SearchCallRequestsRequest{
		CaseID: testUUID, Filters: &domain.SearchCallRequestsFilters{States: []domain.CallRequestStateType{"bogus"}},
	})
	requireErrKind(t, "search invalid state", err, &apierror.ValidationError{})

	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		Filters: domain.SearchAllCallRequestsFilters{AssignmentTeamIDs: []string{testUUID}},
	})
	requireErrKind(t, "search-all assignmentTeamIds is unsupported, not silently ignored", err, &apierror.ValidationError{})

	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		Filters: domain.SearchAllCallRequestsFilters{CaseStates: []domain.CaseState{"bogus"}},
	})
	requireErrKind(t, "search-all invalid caseState", err, &apierror.ValidationError{})

	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		SortBy: domain.CallRequestSort{Field: "bogus"},
	})
	requireErrKind(t, "search-all invalid sort field", err, &apierror.ValidationError{})
}

func TestCallRequestService_UpdateValidation(t *testing.T) {
	svc := &callRequestService{}
	ctx := context.Background()
	scheduled := domain.CallRequestStateScheduled

	tests := []struct {
		name string
		req  domain.UpdateCallRequestRequest
	}{
		{"malformed id", domain.UpdateCallRequestRequest{ID: "x", State: scheduled}},
		{"invalid state", domain.UpdateCallRequestRequest{ID: testUUID, State: "bogus"}},
		{"scheduled needs meetingDate", domain.UpdateCallRequestRequest{ID: testUUID, State: scheduled, DurationMinutes: num(30)}},
		{"scheduled needs duration", domain.UpdateCallRequestRequest{ID: testUUID, State: scheduled, MeetingDate: str("2026-10-02T10:00:00Z")}},
		{"meetingDate must be RFC3339", domain.UpdateCallRequestRequest{ID: testUUID, State: scheduled, MeetingDate: str("tomorrow"), DurationMinutes: num(30)}},
		{"concluded needs notes", domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateConcluded}},
		{"non-positive actual duration", domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateNotesPending, ActualDurationMin: num(0)}},
		{"empty utcTimes when provided", domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateCanceled, UTCTimes: []string{}}},
	}
	for _, tt := range tests {
		_, err := svc.UpdateCallRequest(ctx, tt.req)
		requireErrKind(t, tt.name, err, &apierror.ValidationError{})
	}

	// Valid input still needs an authenticated caller.
	_, err := svc.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateCanceled})
	requireErrKind(t, "update without token", err, &apierror.UnauthorizedError{})
}

func TestCatalogService_Validation(t *testing.T) {
	svc := &catalogService{}
	ctx := context.Background()

	_, err := svc.SearchCatalogs(ctx, domain.SearchCatalogsRequest{})
	requireErrKind(t, "catalogs/search without deployedProductId", err, &apierror.ValidationError{})
	_, err = svc.SearchCatalogs(ctx, domain.SearchCatalogsRequest{DeployedProductID: "nope"})
	requireErrKind(t, "catalogs/search malformed deployedProductId", err, &apierror.ValidationError{})

	_, err = svc.GetCatalogItemVariables(ctx, "", testUUID)
	requireErrKind(t, "variables without catalogId", err, &apierror.ValidationError{})
	_, err = svc.GetCatalogItemVariables(ctx, testUUID, "")
	requireErrKind(t, "variables without catalogItemId", err, &apierror.ValidationError{})
	_, err = svc.GetCatalogItemVariables(ctx, "nope", testUUID)
	requireErrKind(t, "variables malformed catalogId", err, &apierror.ValidationError{})
}

func requireErrKind(t *testing.T, name string, err error, want any) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: got nil error, want %T", name, want)
		return
	}
	var ok bool
	switch want.(type) {
	case *apierror.ValidationError:
		var e *apierror.ValidationError
		ok = errors.As(err, &e)
	case *apierror.UnauthorizedError:
		var e *apierror.UnauthorizedError
		ok = errors.As(err, &e)
	}
	if !ok {
		t.Errorf("%s: got %T (%v), want %T", name, err, err, want)
	}
}

func TestCallRequestService_UpdateRejectsCancellationReason(t *testing.T) {
	svc := &callRequestService{}
	// Rejected before any caller/repository work, and for every state: silently
	// dropping a supplied reason would report success while losing it.
	for _, st := range []domain.CallRequestStateType{domain.CallRequestStateCanceled, domain.CallRequestStateCustomerRejected, domain.CallRequestStateWSO2Rejected} {
		_, err := svc.UpdateCallRequest(context.Background(), domain.UpdateCallRequestRequest{
			ID: testUUID, State: st, CancellationReason: str("no longer needed"),
		})
		requireErrKind(t, "cancellationReason on "+string(st), err, &apierror.ValidationError{})
	}
}
