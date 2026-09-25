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
	createCallRequest     func(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string, authProjectIDs []string) (domain.CreateCallRequestResponse, error)
	searchCallRequests    func(ctx context.Context, caseID string, states []domain.CallRequestStateType, authProjectIDs []string, pagination domain.Pagination) ([]domain.CallRequestView, int, error)
	searchAllCallRequests func(ctx context.Context, f domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, authProjectIDs []string, pagination domain.Pagination) ([]domain.CallRequestView, int, error)
	updateCallRequest     func(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string, authProjectIDs []string) (domain.UpdateCallRequestResponse, error)
}

func (s *stubCallRequestRepo) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string, authProjectIDs []string) (domain.CreateCallRequestResponse, error) {
	if s.createCallRequest != nil {
		return s.createCallRequest(ctx, req, callerID, callerEmail, authProjectIDs)
	}
	panic("not implemented")
}
func (s *stubCallRequestRepo) SearchCallRequests(ctx context.Context, caseID string, states []domain.CallRequestStateType, authProjectIDs []string, pagination domain.Pagination) ([]domain.CallRequestView, int, error) {
	if s.searchCallRequests != nil {
		return s.searchCallRequests(ctx, caseID, states, authProjectIDs, pagination)
	}
	panic("SearchCallRequests called unexpectedly")
}
func (s *stubCallRequestRepo) SearchAllCallRequests(ctx context.Context, f domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, authProjectIDs []string, pagination domain.Pagination) ([]domain.CallRequestView, int, error) {
	if s.searchAllCallRequests != nil {
		return s.searchAllCallRequests(ctx, f, sortBy, authProjectIDs, pagination)
	}
	panic("SearchAllCallRequests called unexpectedly")
}
func (s *stubCallRequestRepo) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string, authProjectIDs []string) (domain.UpdateCallRequestResponse, error) {
	if s.updateCallRequest != nil {
		return s.updateCallRequest(ctx, req, assigneeID, callerEmail, authProjectIDs)
	}
	panic("UpdateCallRequest called unexpectedly")
}

// stubMirrorCallRequestService embeds CallRequestService (nil) and overrides
// only CreateCallRequest -- same convention as stubMirrorProblemService.
type stubMirrorCallRequestService struct {
	CallRequestService
	createCallRequest func(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error)
}

func (s *stubMirrorCallRequestService) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
	return s.createCallRequest(ctx, req)
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
		createCallRequest: func(_ context.Context, req domain.CreateCallRequestRequest, _, _ string, _ []string) (domain.CreateCallRequestResponse, error) {
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
	}, stubAccess{scope: AccessScope{Unrestricted: true}}, dispatcher, mirror)

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
		createCallRequest: func(context.Context, domain.CreateCallRequestRequest, string, string, []string) (domain.CreateCallRequestResponse, error) {
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
	}, stubAccess{scope: AccessScope{Unrestricted: true}}, dispatcher, mirror)

	if _, err := svc.CreateCallRequest(ctx, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestCallRequestService_SearchCallRequests_ScopesToCallerProjects is the
// core regression guard for the call_request authorization gap:
// SearchCallRequests applied no authorization at all before this fix -- any
// authenticated caller could pass any existing caseID and read that case's
// call requests regardless of project.
func TestCallRequestService_SearchCallRequests_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		called := false
		var gotAuthProjectIDs []string
		repo := &stubCallRequestRepo{
			searchCallRequests: func(_ context.Context, _ string, _ []domain.CallRequestStateType, authProjectIDs []string, _ domain.Pagination) ([]domain.CallRequestView, int, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				return []domain.CallRequestView{}, 0, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		_, err := svc.SearchCallRequests(ctx, domain.SearchCallRequestsRequest{CaseID: testUUID, Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchCallRequests: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchCallRequests to be called")
		}
		if len(gotAuthProjectIDs) != 1 || gotAuthProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received authProjectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotAuthProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets zero results, repo never called", func(t *testing.T) {
		repo := &stubCallRequestRepo{}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		resp, err := svc.SearchCallRequests(ctx, domain.SearchCallRequestsRequest{CaseID: testUUID, Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchCallRequests: %v", err)
		}
		if len(resp.CallRequests) != 0 {
			t.Errorf("resp.CallRequests = %v, want empty", resp.CallRequests)
		}
	})

	t.Run("internal (unrestricted) caller passes no project filter", func(t *testing.T) {
		var gotAuthProjectIDs []string
		called := false
		repo := &stubCallRequestRepo{
			searchCallRequests: func(_ context.Context, _ string, _ []domain.CallRequestStateType, authProjectIDs []string, _ domain.Pagination) ([]domain.CallRequestView, int, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				return []domain.CallRequestView{}, 0, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{Unrestricted: true}})

		_, err := svc.SearchCallRequests(ctx, domain.SearchCallRequestsRequest{CaseID: testUUID, Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchCallRequests: %v", err)
		}
		if !called || gotAuthProjectIDs != nil {
			t.Errorf("called=%v gotAuthProjectIDs=%v, want called=true and nil", called, gotAuthProjectIDs)
		}
	})
}

// TestCallRequestService_SearchAllCallRequests_ScopesToCallerProjects is
// SearchCallRequests' identical guard for the cross-case search endpoint.
func TestCallRequestService_SearchAllCallRequests_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		called := false
		var gotAuthProjectIDs []string
		repo := &stubCallRequestRepo{
			searchAllCallRequests: func(_ context.Context, _ domain.SearchAllCallRequestsFilters, _ domain.CallRequestSort, authProjectIDs []string, _ domain.Pagination) ([]domain.CallRequestView, int, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				return []domain.CallRequestView{}, 0, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		_, err := svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchAllCallRequests: %v", err)
		}
		if !called {
			t.Fatal("expected repo.SearchAllCallRequests to be called")
		}
		if len(gotAuthProjectIDs) != 1 || gotAuthProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received authProjectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotAuthProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets zero results, repo never called", func(t *testing.T) {
		repo := &stubCallRequestRepo{}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		resp, err := svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchAllCallRequests: %v", err)
		}
		if len(resp.CallRequests) != 0 {
			t.Errorf("resp.CallRequests = %v, want empty", resp.CallRequests)
		}
	})

	t.Run("internal (unrestricted) caller passes no project filter", func(t *testing.T) {
		var gotAuthProjectIDs []string
		called := false
		repo := &stubCallRequestRepo{
			searchAllCallRequests: func(_ context.Context, _ domain.SearchAllCallRequestsFilters, _ domain.CallRequestSort, authProjectIDs []string, _ domain.Pagination) ([]domain.CallRequestView, int, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				return []domain.CallRequestView{}, 0, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{Unrestricted: true}})

		_, err := svc.SearchAllCallRequests(ctx, domain.SearchAllCallRequestsRequest{Pagination: domain.Pagination{Limit: 10}})
		if err != nil {
			t.Fatalf("SearchAllCallRequests: %v", err)
		}
		if !called || gotAuthProjectIDs != nil {
			t.Errorf("called=%v gotAuthProjectIDs=%v, want called=true and nil", called, gotAuthProjectIDs)
		}
	})
}

// TestCallRequestService_CreateCallRequest_ScopesToCallerProjects covers the
// write path: CreateCallRequest applied no authorization at all before this
// fix -- a caller could open a call request against any case regardless of
// project.
func TestCallRequestService_CreateCallRequest_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.CreateCallRequestRequest{CaseID: testUUID, Reason: "r", UTCTimes: []string{"2026-10-01T10:00:00Z"}, DurationMinutes: 30}
	userRepo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}

	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		var gotAuthProjectIDs []string
		repo := &stubCallRequestRepo{
			createCallRequest: func(_ context.Context, _ domain.CreateCallRequestRequest, _, _ string, authProjectIDs []string) (domain.CreateCallRequestResponse, error) {
				gotAuthProjectIDs = authProjectIDs
				var resp domain.CreateCallRequestResponse
				resp.CallRequest.ID = testUUID
				return resp, nil
			},
		}
		svc := NewCallRequestService(repo, userRepo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		if _, err := svc.CreateCallRequest(ctx, req); err != nil {
			t.Fatalf("CreateCallRequest: %v", err)
		}
		if len(gotAuthProjectIDs) != 1 || gotAuthProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received authProjectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotAuthProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets NotFound, repo never called", func(t *testing.T) {
		repo := &stubCallRequestRepo{}
		svc := NewCallRequestService(repo, userRepo, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		_, err := svc.CreateCallRequest(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("internal caller passes no project filter", func(t *testing.T) {
		var gotAuthProjectIDs []string
		called := false
		repo := &stubCallRequestRepo{
			createCallRequest: func(_ context.Context, _ domain.CreateCallRequestRequest, _, _ string, authProjectIDs []string) (domain.CreateCallRequestResponse, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				var resp domain.CreateCallRequestResponse
				resp.CallRequest.ID = testUUID
				return resp, nil
			},
		}
		svc := NewCallRequestService(repo, userRepo, stubAccess{scope: AccessScope{Unrestricted: true}})

		if _, err := svc.CreateCallRequest(ctx, req); err != nil {
			t.Fatalf("CreateCallRequest: %v", err)
		}
		if !called || gotAuthProjectIDs != nil {
			t.Errorf("called=%v gotAuthProjectIDs=%v, want called=true and nil", called, gotAuthProjectIDs)
		}
	})
}

// TestCallRequestService_UpdateCallRequest_ScopesToCallerProjects covers the
// by-id write path: UpdateCallRequest applied no authorization at all before
// this fix -- a caller could update any call request by id regardless of
// project.
func TestCallRequestService_UpdateCallRequest_ScopesToCallerProjects(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := domain.UpdateCallRequestRequest{ID: testUUID, State: domain.CallRequestStateCanceled}

	t.Run("external caller's own resolved projects are passed to the repo", func(t *testing.T) {
		var gotAuthProjectIDs []string
		repo := &stubCallRequestRepo{
			updateCallRequest: func(_ context.Context, _ domain.UpdateCallRequestRequest, _ *string, _ string, authProjectIDs []string) (domain.UpdateCallRequestResponse, error) {
				gotAuthProjectIDs = authProjectIDs
				return domain.UpdateCallRequestResponse{}, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

		if _, err := svc.UpdateCallRequest(ctx, req); err != nil {
			t.Fatalf("UpdateCallRequest: %v", err)
		}
		if len(gotAuthProjectIDs) != 1 || gotAuthProjectIDs[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("repo received authProjectIDs = %v, want [aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa]", gotAuthProjectIDs)
		}
	})

	t.Run("external caller with no registered projects at all gets NotFound, repo never called", func(t *testing.T) {
		repo := &stubCallRequestRepo{}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{ProjectIDs: nil}})

		_, err := svc.UpdateCallRequest(ctx, req)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("internal caller passes no project filter", func(t *testing.T) {
		var gotAuthProjectIDs []string
		called := false
		repo := &stubCallRequestRepo{
			updateCallRequest: func(_ context.Context, _ domain.UpdateCallRequestRequest, _ *string, _ string, authProjectIDs []string) (domain.UpdateCallRequestResponse, error) {
				called = true
				gotAuthProjectIDs = authProjectIDs
				return domain.UpdateCallRequestResponse{}, nil
			},
		}
		svc := NewCallRequestService(repo, nil, stubAccess{scope: AccessScope{Unrestricted: true}})

		if _, err := svc.UpdateCallRequest(ctx, req); err != nil {
			t.Fatalf("UpdateCallRequest: %v", err)
		}
		if !called || gotAuthProjectIDs != nil {
			t.Errorf("called=%v gotAuthProjectIDs=%v, want called=true and nil", called, gotAuthProjectIDs)
		}
	})
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
