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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubCallRequestRepo is a minimal repository.CallRequestRepository whose
// unconfigured methods panic if called -- same convention as
// stubProblemRepo (problem_service_test.go).
type stubCallRequestRepo struct {
	createCallRequest     func(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error)
	updateCallRequest     func(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string) (domain.UpdateCallRequestResponse, error)
	setCallRequestSNSysID func(ctx context.Context, id, snSysID string) error
	getCallRequestSNSysID func(ctx context.Context, id string) (*string, error)
}

func (s *stubCallRequestRepo) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error) {
	if s.createCallRequest != nil {
		return s.createCallRequest(ctx, req, callerID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCallRequestRepo) SearchCallRequests(context.Context, string, []domain.CallRequestStateType, domain.Pagination) ([]domain.CallRequestView, int, error) {
	panic("not implemented")
}
func (s *stubCallRequestRepo) SearchAllCallRequests(context.Context, domain.SearchAllCallRequestsFilters, domain.CallRequestSort, domain.Pagination) ([]domain.CallRequestView, int, error) {
	panic("not implemented")
}
func (s *stubCallRequestRepo) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string) (domain.UpdateCallRequestResponse, error) {
	if s.updateCallRequest != nil {
		return s.updateCallRequest(ctx, req, assigneeID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCallRequestRepo) SetCallRequestSNSysID(ctx context.Context, id, snSysID string) error {
	if s.setCallRequestSNSysID != nil {
		return s.setCallRequestSNSysID(ctx, id, snSysID)
	}
	return nil
}
func (s *stubCallRequestRepo) GetCallRequestSNSysID(ctx context.Context, id string) (*string, error) {
	if s.getCallRequestSNSysID != nil {
		return s.getCallRequestSNSysID(ctx, id)
	}
	return nil, nil
}

// stubMirrorCallRequestService embeds CallRequestService (nil) and overrides
// only CreateCallRequest -- same convention as stubMirrorProblemService.
type stubMirrorCallRequestService struct {
	CallRequestService
	createCallRequest func(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error)
	updateCallRequest func(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error)
}

func (s *stubMirrorCallRequestService) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
	return s.createCallRequest(ctx, req)
}

func (s *stubMirrorCallRequestService) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
	return s.updateCallRequest(ctx, req)
}

// TestCallRequestService_CreateCallRequest_MirrorsToServiceNow covers the
// writeback wiring: on a successful Postgres create, the mirror's
// CreateCallRequest is dispatched asynchronously and does not block or
// affect the response, and a mirror failure is recorded to
// sn_writeback_failures rather than failing the call.
func TestCallRequestService_CreateCallRequest_MirrorsToServiceNow(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	called := make(chan domain.CreateCallRequestRequest, 1)
	mirror := &stubMirrorCallRequestService{
		createCallRequest: func(_ context.Context, mirrorReq domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
			called <- mirrorReq
			return domain.CreateCallRequestResponse{}, nil
		},
	}
	repo := &stubCallRequestRepo{
		createCallRequest: func(_ context.Context, req domain.CreateCallRequestRequest, _, _ string) (domain.CreateCallRequestResponse, error) {
			var resp domain.CreateCallRequestResponse
			resp.CallRequest.ID = testUUID
			return resp, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateCallRequest(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.CaseID != req.CaseID || got.Reason != req.Reason {
			t.Errorf("mirror got %+v, want caseId/reason to match %+v", got, req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateCallRequest was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCallRequestService_CreateCallRequest_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestCallRequestService_CreateCallRequest_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	mirror := &stubMirrorCallRequestService{
		createCallRequest: func(context.Context, domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
			return domain.CreateCallRequestResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCallRequestRepo{
		createCallRequest: func(context.Context, domain.CreateCallRequestRequest, string, string) (domain.CreateCallRequestResponse, error) {
			var resp domain.CreateCallRequestResponse
			resp.CallRequest.ID = testUUID
			return resp, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateCallRequest(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

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
		_, err := svc.CreateCallRequest(contextWithUserIDToken(""), req)
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
	_, err := svc.UpdateCallRequest(contextWithUserIDToken(""), domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateCanceled})
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

// testSNSysID2 is a second canonical UUID, distinct from testUUID, used
// where a test needs both a Postgres row id and a ServiceNow-mapped id in
// play at once.
const testSNSysID2 = "88888888-0000-4000-8000-000000000002"

// TestCallRequestService_CreateCallRequest_MirrorSuccessPersistsSNSysID
// covers the id-mapping half of the CREATE mirror: on success, the
// ServiceNow-returned id (converted to its raw sys_id form) is persisted
// back onto the Postgres row via SetCallRequestSNSysID.
func TestCallRequestService_CreateCallRequest_MirrorSuccessPersistsSNSysID(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	mirror := &stubMirrorCallRequestService{
		createCallRequest: func(context.Context, domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
			var resp domain.CreateCallRequestResponse
			resp.CallRequest.ID = testSNSysID2 // the ServiceNow-side id, sysidToUUID-converted
			return resp, nil
		},
	}
	type setCall struct{ id, snSysID string }
	setCalls := make(chan setCall, 1)
	repo := &stubCallRequestRepo{
		createCallRequest: func(context.Context, domain.CreateCallRequestRequest, string, string) (domain.CreateCallRequestResponse, error) {
			var resp domain.CreateCallRequestResponse
			resp.CallRequest.ID = testUUID // the Postgres-side id
			return resp, nil
		},
		setCallRequestSNSysID: func(_ context.Context, id, snSysID string) error {
			setCalls <- setCall{id: id, snSysID: snSysID}
			return nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateCallRequest(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-setCalls:
		if got.id != testUUID {
			t.Errorf("SetCallRequestSNSysID id = %q, want %q (the Postgres row id)", got.id, testUUID)
		}
		wantSysID := uuidToSysid(testSNSysID2)
		if got.snSysID != wantSysID {
			t.Errorf("SetCallRequestSNSysID snSysID = %q, want %q", got.snSysID, wantSysID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SetCallRequestSNSysID was never called")
	}
}

// TestCallRequestService_UpdateCallRequest_MirrorsWithStoredSNSysID covers
// the UPDATE mirror's happy path: when a ServiceNow sys_id is already
// stored for this call request, the mirror fires against it (converted back
// to UUID form for the mirror's own uuidToSysid), and CaseID is cleared to
// avoid the mirror's GET-before-write case-membership verification.
func TestCallRequestService_UpdateCallRequest_MirrorsWithStoredSNSysID(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCallRequestRequest{ID: testUUID, CaseID: testSNSysID2, State: domain.CallRequestStateScheduled, MeetingDate: str("2026-10-01T10:00:00Z"), DurationMinutes: num(30)}

	storedSysID := uuidToSysid(testSNSysID2)
	repo := &stubCallRequestRepo{
		updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest, *string, string) (domain.UpdateCallRequestResponse, error) {
			return domain.UpdateCallRequestResponse{}, nil
		},
		getCallRequestSNSysID: func(context.Context, string) (*string, error) {
			return &storedSysID, nil
		},
	}
	called := make(chan domain.UpdateCallRequestRequest, 1)
	mirror := &stubMirrorCallRequestService{
		updateCallRequest: func(_ context.Context, mirrorReq domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
			called <- mirrorReq
			return domain.UpdateCallRequestResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{}, dispatcher, mirror)

	if _, err := svc.UpdateCallRequest(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.ID != testSNSysID2 {
			t.Errorf("mirror UpdateCallRequest ID = %q, want %q (sysidToUUID of the stored sys_id)", got.ID, testSNSysID2)
		}
		if got.CaseID != "" {
			t.Errorf("mirror UpdateCallRequest CaseID = %q, want empty (avoid GET-before-write verify)", got.CaseID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateCallRequest was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCallRequestService_UpdateCallRequest_SkipsMirrorWhenNoSNSysIDStored
// covers the pre-migration-row case: a NULL sn_sys_id (the CREATE mirror
// never ran, hasn't finished, or failed) must make the UPDATE mirror skip
// silently -- no mirror call, no sn_writeback_failures record, no error to
// the caller.
func TestCallRequestService_UpdateCallRequest_SkipsMirrorWhenNoSNSysIDStored(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateScheduled, MeetingDate: str("2026-10-01T10:00:00Z"), DurationMinutes: num(30)}

	repo := &stubCallRequestRepo{
		updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest, *string, string) (domain.UpdateCallRequestResponse, error) {
			return domain.UpdateCallRequestResponse{}, nil
		},
		getCallRequestSNSysID: func(context.Context, string) (*string, error) {
			return nil, nil // no mapping stored yet
		},
	}
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorCallRequestService{
		updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.UpdateCallRequestResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{}, dispatcher, mirror)

	if _, err := svc.UpdateCallRequest(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.UpdateCallRequest was called despite no ServiceNow mapping being stored")
	case <-time.After(300 * time.Millisecond):
		// expected: no mirror call
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a deliberate skip, got %d", got)
	}
}
