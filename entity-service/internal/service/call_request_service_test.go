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
	createCallRequest               func(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error)
	createCallRequestFromServiceNow func(ctx context.Context, req domain.CreateCallRequestRequest, id, createdBy string, createdOn time.Time, callerID string) (domain.CreateCallRequestResponse, error)
	updateCallRequest               func(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string) (domain.UpdateCallRequestResponse, error)
	setCallRequestSNSysID           func(ctx context.Context, id, snSysID string) error
	getCallRequestSNSysID           func(ctx context.Context, id string) (*string, error)
	searchAllCallRequests           func(ctx context.Context, f domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, p domain.Pagination) ([]domain.CallRequestView, int, error)
}

func (s *stubCallRequestRepo) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error) {
	if s.createCallRequest != nil {
		return s.createCallRequest(ctx, req, callerID, callerEmail)
	}
	panic("not implemented")
}
func (s *stubCallRequestRepo) CreateCallRequestFromServiceNow(ctx context.Context, req domain.CreateCallRequestRequest, id, createdBy string, createdOn time.Time, callerID string) (domain.CreateCallRequestResponse, error) {
	if s.createCallRequestFromServiceNow != nil {
		return s.createCallRequestFromServiceNow(ctx, req, id, createdBy, createdOn, callerID)
	}
	panic("not implemented")
}
func (s *stubCallRequestRepo) SearchCallRequests(context.Context, string, []domain.CallRequestStateType, domain.Pagination) ([]domain.CallRequestView, int, error) {
	panic("not implemented")
}
func (s *stubCallRequestRepo) SearchAllCallRequests(ctx context.Context, f domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, p domain.Pagination) ([]domain.CallRequestView, int, error) {
	if s.searchAllCallRequests != nil {
		return s.searchAllCallRequests(ctx, f, sortBy, p)
	}
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
// UpdateCallRequest plus the narrow callRequestSNCreator method
// createCallRequestSNFirst type-asserts against -- same convention as
// stubMirrorDeploymentService (deployment_service_test.go). CreateCallRequest
// itself is deliberately NOT overridden: createCallRequestSNFirst calls
// createCallRequestSNFirstDetailsFn instead, never the public
// CreateCallRequest, so leaving it unset (embedded CallRequestService is nil)
// doubles as an assertion that it's never reached.
type stubMirrorCallRequestService struct {
	CallRequestService
	createCallRequestSNFirstDetailsFn func(ctx context.Context, req domain.CreateCallRequestRequest) (id, createdBy string, createdOn time.Time, err error)
	updateCallRequest                 func(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error)
}

func (s *stubMirrorCallRequestService) createCallRequestSNFirstDetails(ctx context.Context, req domain.CreateCallRequestRequest) (id, createdBy string, createdOn time.Time, err error) {
	if s.createCallRequestSNFirstDetailsFn == nil {
		panic("stubMirrorCallRequestService: createCallRequestSNFirstDetailsFn not set")
	}
	return s.createCallRequestSNFirstDetailsFn(ctx, req)
}

func (s *stubMirrorCallRequestService) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
	return s.updateCallRequest(ctx, req)
}

// TestCallRequestService_CreateCallRequest_SNFailureLeavesPostgresUntouched
// covers the SN-first path's safety property: if ServiceNow never accepts the
// call request, s.repo.CreateCallRequestFromServiceNow must never be called
// at all (stubCallRequestRepo panics if it's invoked without being
// configured, which doubles as the assertion) -- same convention as
// TestDeploymentService_CreateDeployment_SNFailureLeavesPostgresUntouched.
func TestCallRequestService_CreateCallRequest_SNFailureLeavesPostgresUntouched(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	mirror := &stubMirrorCallRequestService{
		createCallRequestSNFirstDetailsFn: func(context.Context, domain.CreateCallRequestRequest) (string, string, time.Time, error) {
			return "", "", time.Time{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCallRequestRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	if _, err := svc.CreateCallRequest(ctx, req); err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the call request")
	}
}

// TestCallRequestService_CreateCallRequest_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the SN-first path's happy case: the Postgres insert must use
// EXACTLY the id/createdBy/createdOn ServiceNow returned, plus the resolved
// caller's own id/email -- not anything generated locally.
func TestCallRequestService_CreateCallRequest_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID        = "33333333-3333-3333-3333-333333333333"
		snCreatedBy = "jane.doe@example.com"
	)
	createdOn := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}

	mirror := &stubMirrorCallRequestService{
		createCallRequestSNFirstDetailsFn: func(context.Context, domain.CreateCallRequestRequest) (string, string, time.Time, error) {
			return snID, snCreatedBy, createdOn, nil
		},
	}

	var gotID, gotCreatedBy, gotCallerID string
	var gotCreatedOn time.Time
	repo := &stubCallRequestRepo{
		createCallRequestFromServiceNow: func(_ context.Context, _ domain.CreateCallRequestRequest, id, createdBy string, createdOnArg time.Time, callerID string) (domain.CreateCallRequestResponse, error) {
			gotID, gotCreatedBy, gotCreatedOn, gotCallerID = id, createdBy, createdOnArg, callerID
			var resp domain.CreateCallRequestResponse
			resp.CallRequest.ID = id
			resp.CallRequest.CreatedBy = createdBy
			return resp, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, dispatcher, mirror)

	resp, err := svc.CreateCallRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotID != snID || gotCreatedBy != snCreatedBy || !gotCreatedOn.Equal(createdOn) || gotCallerID != testUUID {
		t.Errorf("CreateCallRequestFromServiceNow got (%q, %q, %v, %q), want (%q, %q, %v, %q)",
			gotID, gotCreatedBy, gotCreatedOn, gotCallerID, snID, snCreatedBy, createdOn, testUUID)
	}
	if resp.CallRequest.ID != snID {
		t.Errorf("response call request id = %q, want %q (the ServiceNow-assigned id)", resp.CallRequest.ID, snID)
	}
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

	// assignmentTeamIds is supported (the case's account CRE team), so only a
	// malformed id is refused here -- a well-formed one goes on to the repository.
	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		Filters: domain.SearchAllCallRequestsFilters{AssignmentTeamIDs: []string{"not-a-uuid"}},
	})
	requireErrKind(t, "search-all malformed assignmentTeamIds", err, &apierror.ValidationError{})

	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		Filters: domain.SearchAllCallRequestsFilters{CaseStates: []domain.CaseState{"bogus"}},
	})
	requireErrKind(t, "search-all invalid caseState", err, &apierror.ValidationError{})

	_, err = svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{
		SortBy: domain.CallRequestSort{Field: "bogus"},
	})
	requireErrKind(t, "search-all invalid sort field", err, &apierror.ValidationError{})
}

// TestCallRequestService_SearchAll_PassesTeamAndAssigneeFiltersToTheRepo guards the
// "Calls To Attend" and "My Call Requests" dashboard widgets (digiops-cs#3314). A
// valid assignmentTeamIds used to be refused with a 400 before it ever reached the
// repository, which the widget showed as "Could not load this widget"; the
// validation-error test above passes against that old behaviour too (it also
// returned a ValidationError), so this one asserts the positive path in the default
// suite, which has no database: the filters the widgets send arrive at the
// repository unchanged, and the service does not turn them into an error.
func TestCallRequestService_SearchAll_PassesTeamAndAssigneeFiltersToTheRepo(t *testing.T) {
	const otherUUID = "22222222-2222-4222-8222-222222222222"
	var got domain.SearchAllCallRequestsFilters
	calls := 0
	svc := &callRequestService{repo: &stubCallRequestRepo{
		searchAllCallRequests: func(_ context.Context, f domain.SearchAllCallRequestsFilters, _ domain.CallRequestSort, _ domain.Pagination) ([]domain.CallRequestView, int, error) {
			calls++
			got = f
			return []domain.CallRequestView{{ID: testUUID}}, 1, nil
		},
	}}

	want := domain.SearchAllCallRequestsFilters{
		AssignedUserIDs:   []string{testUUID},
		AssignmentTeamIDs: []string{otherUUID},
		ExcludeCaseStates: []domain.CaseState{domain.CaseStateClosed},
		States:            []domain.CallRequestStateType{domain.CallRequestStatePendingOnWSO2, domain.CallRequestStateNotesPending},
	}
	resp, err := svc.SearchAllCallRequests(context.Background(), domain.SearchAllCallRequestsRequest{Filters: want})
	if err != nil {
		t.Fatalf("a valid team + assignee filter must not be refused: %v", err)
	}
	if calls != 1 {
		t.Fatalf("repository called %d times, want exactly 1", calls)
	}
	if len(resp.CallRequests) != 1 || resp.Total != 1 {
		t.Errorf("response = %+v, want the repository's one row and total 1", resp)
	}
	if len(got.AssignmentTeamIDs) != 1 || got.AssignmentTeamIDs[0] != otherUUID {
		t.Errorf("assignmentTeamIds reaching the repository = %v, want [%s]", got.AssignmentTeamIDs, otherUUID)
	}
	if len(got.AssignedUserIDs) != 1 || got.AssignedUserIDs[0] != testUUID {
		t.Errorf("assignedUserIds reaching the repository = %v, want [%s]", got.AssignedUserIDs, testUUID)
	}
	if len(got.States) != 2 || len(got.ExcludeCaseStates) != 1 {
		t.Errorf("states / excludeCaseStates were altered on the way: %+v", got)
	}

	// A malformed id is still refused up front and never reaches the repository.
	calls = 0
	_, err = svc.SearchAllCallRequests(context.Background(), domain.SearchAllCallRequestsRequest{
		Filters: domain.SearchAllCallRequestsFilters{AssignmentTeamIDs: []string{otherUUID, "not-a-uuid"}},
	})
	requireErrKind(t, "one malformed team id among valid ones", err, &apierror.ValidationError{})
	if calls != 0 {
		t.Errorf("a refused request still reached the repository %d time(s)", calls)
	}
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

// TestCallRequestService_UpdateCallRequest_ConcludeWithoutNotes covers the one-click
// "Mark as completed" (digiops-cs#3350): a conclude with no post-call notes is not a
// validation error any more, reaches the repository as sent, and a conflict the
// repository reports (the call is not scheduled / notes pending) is passed on as a
// conflict rather than turned into a 400 or a 500.
func TestCallRequestService_UpdateCallRequest_ConcludeWithoutNotes(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	blank := "   "
	for name, notes := range map[string]*string{"nil notes": nil, "whitespace-only notes": &blank} {
		t.Run(name, func(t *testing.T) {
			var got domain.UpdateCallRequestRequest
			calls := 0
			svc := &callRequestService{repo: &stubCallRequestRepo{
				updateCallRequest: func(_ context.Context, r domain.UpdateCallRequestRequest, _ *string, _ string) (domain.UpdateCallRequestResponse, error) {
					calls++
					got = r
					return domain.UpdateCallRequestResponse{Message: "ok"}, nil
				},
			}}
			if _, err := svc.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateConcluded, Notes: notes}); err != nil {
				t.Fatalf("a conclude without notes must pass validation: %v", err)
			}
			if calls != 1 || got.State != domain.CallRequestStateConcluded {
				t.Errorf("repository calls = %d, state = %q; want 1 call concluding the request", calls, got.State)
			}
			if got.Notes != nil {
				t.Errorf("notes reaching the repository = %q, want none (blank notes must not be written)", *got.Notes)
			}
		})
	}

	t.Run("a repository conflict stays a conflict", func(t *testing.T) {
		svc := &callRequestService{repo: &stubCallRequestRepo{
			updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest, *string, string) (domain.UpdateCallRequestResponse, error) {
				return domain.UpdateCallRequestResponse{}, &apierror.ConflictError{Msg: "only a scheduled call"}
			},
		}}
		_, err := svc.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateConcluded})
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) {
			t.Errorf("got %T (%v), want the repository's *apierror.ConflictError passed through", err, err)
		}
	})
}

// Blank notes are only dropped for a conclude (so "Mark as completed" cannot erase the
// notes a call already has). Every other state keeps exactly what it was sent.
func TestCallRequestService_UpdateCallRequest_BlankNotesOnlyDroppedForConclude(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	blank := "   "
	var got domain.UpdateCallRequestRequest
	svc := &callRequestService{repo: &stubCallRequestRepo{
		updateCallRequest: func(_ context.Context, r domain.UpdateCallRequestRequest, _ *string, _ string) (domain.UpdateCallRequestResponse, error) {
			got = r
			return domain.UpdateCallRequestResponse{}, nil
		},
	}}
	req := domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateNotesPending, Notes: &blank}
	if _, err := svc.UpdateCallRequest(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Notes == nil || *got.Notes != blank {
		t.Errorf("notes for a non-conclude state = %v, want them passed through exactly as sent", got.Notes)
	}
}

// Under dual-write the Postgres change commits first and ServiceNow is only mirrored
// after it succeeds: a conclude the repository refuses must never reach the mirror,
// and one it accepts must reach it exactly as the repository saw it (blank notes
// already dropped), or ServiceNow could be told to blank notes Postgres kept.
func TestCallRequestService_UpdateCallRequest_ConcludeMirrorFollowsPostgres(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	blank := "  "
	newSvc := func(repoErr error) (*callRequestService, chan domain.UpdateCallRequestRequest) {
		mirrored := make(chan domain.UpdateCallRequestRequest, 1)
		repo := &stubCallRequestRepo{
			updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest, *string, string) (domain.UpdateCallRequestResponse, error) {
				return domain.UpdateCallRequestResponse{}, repoErr
			},
			getCallRequestSNSysID: func(context.Context, string) (*string, error) { return nil, nil },
		}
		mirror := &stubMirrorCallRequestService{
			updateCallRequest: func(_ context.Context, r domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
				mirrored <- r
				return domain.UpdateCallRequestResponse{}, nil
			},
		}
		svc := NewCallRequestServiceWithSNWriteback(repo, stubUserRepo{}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), mirror)
		return svc.(*callRequestService), mirrored
	}

	t.Run("a refused conclude is never mirrored", func(t *testing.T) {
		svc, mirrored := newSvc(&apierror.ConflictError{Msg: "not scheduled"})
		if _, err := svc.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateConcluded}); err == nil {
			t.Fatal("expected the repository's conflict")
		}
		select {
		case r := <-mirrored:
			t.Errorf("ServiceNow was sent %+v for a conclude Postgres refused", r)
		case <-time.After(300 * time.Millisecond):
		}
	})

	t.Run("an accepted conclude is mirrored with the blank notes already dropped", func(t *testing.T) {
		svc, mirrored := newSvc(nil)
		if _, err := svc.UpdateCallRequest(ctx, domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateConcluded, Notes: &blank}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case r := <-mirrored:
			if r.State != domain.CallRequestStateConcluded || r.Notes != nil {
				t.Errorf("mirror got state=%q notes=%v, want concluded with no notes", r.State, r.Notes)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the accepted conclude was never mirrored")
		}
	})
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

// TestCallRequestService_UpdateCallRequest_DerivesSysIDWhenNoneStored covers
// the new-style row case (CreateCallRequest is now ServiceNow-first -- see
// callRequestService's own doc comment): a NULL sn_sys_id column no longer
// means "skip, no mapping yet" -- it means "this row's id already IS the real
// sys_id" -- so the mirror must fire with mirrorReq.ID derived via
// sysidToUUID(uuidToSysid(req.ID)), i.e. req.ID unchanged.
func TestCallRequestService_UpdateCallRequest_DerivesSysIDWhenNoneStored(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCallRequestRequest{ID: testUUID, CaseID: testSNSysID2, State: domain.CallRequestStateScheduled, MeetingDate: str("2026-10-01T10:00:00Z"), DurationMinutes: num(30)}

	repo := &stubCallRequestRepo{
		updateCallRequest: func(context.Context, domain.UpdateCallRequestRequest, *string, string) (domain.UpdateCallRequestResponse, error) {
			return domain.UpdateCallRequestResponse{}, nil
		},
		getCallRequestSNSysID: func(context.Context, string) (*string, error) {
			return nil, nil // no mapping stored -- a row CreateCallRequest created SN-first
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
		if got.ID != testUUID {
			t.Errorf("mirror UpdateCallRequest ID = %q, want %q (derived via uuidToSysid(req.ID) round-tripped back through sysidToUUID)", got.ID, testUUID)
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
