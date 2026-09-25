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
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubChangeRequestRepo is a minimal repository.ChangeRequestRepository
// whose unconfigured methods panic if called -- same convention as
// stubIncidentRepo (incident_service_test.go).
type stubChangeRequestRepo struct {
	createChangeRequestFromServiceNow func(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error)
	patchChangeRequest                func(ctx context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, error)
	searchChangeRequests              func(ctx context.Context, req domain.SearchChangeRequestsRequest) ([]domain.SearchChangeRequestView, int, error)
	aggregateChangeRequests           func(ctx context.Context, req domain.AggregateChangeRequestsRequest) (domain.AggregateResponse, error)
	getChangeRequestByID              func(ctx context.Context, id string) (domain.ChangeRequest, error)
}

func (s *stubChangeRequestRepo) SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest, _, _ *time.Time, _ *string, _ []string) ([]domain.SearchChangeRequestView, int, error) {
	if s.searchChangeRequests != nil {
		return s.searchChangeRequests(ctx, req)
	}
	panic("SearchChangeRequests called unexpectedly")
}
func (s *stubChangeRequestRepo) AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest, _ string, _ int, _, _ *time.Time, _ *string) (domain.AggregateResponse, error) {
	if s.aggregateChangeRequests != nil {
		return s.aggregateChangeRequests(ctx, req)
	}
	panic("AggregateChangeRequests called unexpectedly")
}
func (s *stubChangeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	if s.getChangeRequestByID != nil {
		return s.getChangeRequestByID(ctx, id)
	}
	panic("GetChangeRequestByID called unexpectedly")
}
func (s *stubChangeRequestRepo) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, error) {
	if s.patchChangeRequest != nil {
		return s.patchChangeRequest(ctx, id, req, email)
	}
	panic("not implemented")
}
func (s *stubChangeRequestRepo) CreateChangeRequestFromServiceNow(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error) {
	if s.createChangeRequestFromServiceNow != nil {
		return s.createChangeRequestFromServiceNow(ctx, req, id, number, createdBy)
	}
	panic("CreateChangeRequestFromServiceNow called unexpectedly: Postgres must stay untouched when ServiceNow never accepts the change request")
}

// stubMirrorChangeRequestService embeds ChangeRequestService (nil) and
// overrides only CreateChangeRequest -- same convention as
// stubMirrorIncidentService (incident_service_test.go). Any other method
// being called would panic on the nil embedded interface, which is the
// point: this pilot's change request mode only ever calls the mirror's
// CreateChangeRequest.
type stubMirrorChangeRequestService struct {
	ChangeRequestService
	createChangeRequest func(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error)
	patchChangeRequest  func(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error)
}

func (s *stubMirrorChangeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	return s.createChangeRequest(ctx, req)
}

func (s *stubMirrorChangeRequestService) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	return s.patchChangeRequest(ctx, id, req)
}

func validCreateChangeRequestRequest() domain.CreateChangeRequestRequest {
	return domain.CreateChangeRequestRequest{Subject: "subject"}
}

// TestChangeRequestService_CreateChangeRequest_SNFailureLeavesPostgresUntouched
// is the pilot's core regression guard for change request CREATE: a single SN
// failure returns an error immediately (no internal retry -- retrying risks
// creating a duplicate ServiceNow record if the create actually succeeded but
// the response was lost) and the Postgres repository must never be called at
// all -- no row, no orphan.
func TestChangeRequestService_CreateChangeRequest_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateChangeRequestResponse{}, errors.New("sn downstream unreachable")
		},
	}
	// No createChangeRequestFromServiceNow override -- stubChangeRequestRepo
	// panics if it's ever called, which is exactly the assertion: Postgres
	// must stay untouched.
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	_, err := svc.CreateChangeRequest(context.Background(), validCreateChangeRequestRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the change request")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// TestChangeRequestService_CreateChangeRequest_RejectsUnsupportedTypeBeforeSN
// is the regression guard for a CodeRabbit finding on PR #1930: a type with
// no changeRequestTypeToChangeModel entry (e.g. the pre-000055
// site_reliability_ops value) must be rejected before the ServiceNow call,
// not after. Rejecting it only in CreateChangeRequestFromServiceNow (after
// ServiceNow already accepted the create) would leave ServiceNow holding an
// orphan record with no Postgres row, and would duplicate it if the caller
// retried.
func TestChangeRequestService_CreateChangeRequest_RejectsUnsupportedTypeBeforeSN(t *testing.T) {
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			t.Fatal("ServiceNow must never be called for an unsupported type")
			return domain.CreateChangeRequestResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	req := validCreateChangeRequestRequest()
	unsupported := domain.ChangeRequestTypeSiteReliabilityOps
	req.Type = &unsupported

	_, err := svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for unsupported type, got %T: %v", err, err)
	}
}

// TestChangeRequestService_CreateChangeRequest_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the other half, mirroring
// TestIncidentService_CreateIncident_SNSuccessCreatesPostgresRowWithMatchingIdentity:
// on ServiceNow success, the Postgres insert must use EXACTLY the
// id/number/createdBy ServiceNow returned -- not anything generated locally.
func TestChangeRequestService_CreateChangeRequest_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID        = "55555555-5555-5555-5555-555555555555"
		snNumber    = "CHG0023001"
		snCreatedBy = "jane.doe@example.com"
	)
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(_ context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			resp := domain.CreateChangeRequestResponse{Message: "Change request created successfully."}
			resp.ChangeRequest.ID = snID
			resp.ChangeRequest.Number = snNumber
			resp.ChangeRequest.CreatedBy = snCreatedBy
			resp.ChangeRequest.CreatedOn = "2026-09-21T12:00:00Z"
			return resp, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotCreatedBy string
	repo := &stubChangeRequestRepo{
		createChangeRequestFromServiceNow: func(_ context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error) {
			mu.Lock()
			gotID, gotNumber, gotCreatedBy = id, number, createdBy
			mu.Unlock()
			resp := domain.CreateChangeRequestResponse{Message: "Change request created successfully."}
			resp.ChangeRequest.ID = id
			resp.ChangeRequest.Number = number
			resp.ChangeRequest.CreatedBy = createdBy
			resp.ChangeRequest.CreatedOn = "2026-09-21T12:00:00Z"
			return resp, nil
		},
	}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	resp, err := svc.CreateChangeRequest(context.Background(), validCreateChangeRequestRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotCreatedBy != snCreatedBy {
		t.Errorf("CreateChangeRequestFromServiceNow got (%q, %q, %q), want (%q, %q, %q)",
			gotID, gotNumber, gotCreatedBy, snID, snNumber, snCreatedBy)
	}
	if resp.ChangeRequest.ID != snID || resp.ChangeRequest.Number != snNumber || resp.ChangeRequest.CreatedBy != snCreatedBy {
		t.Errorf("CreateChangeRequest response = %+v, want identity matching ServiceNow's (%q, %q, %q)", resp.ChangeRequest, snID, snNumber, snCreatedBy)
	}
}

// TestChangeRequestService_CreateChangeRequest_DoesNotRetryValidationError
// covers the ValidationError path specifically: it must surface directly,
// with no Postgres write attempted, same as any other single-attempt SN
// failure.
func TestChangeRequestService_CreateChangeRequest_DoesNotRetryValidationError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "subject is required"}
		},
	}
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	_, err := svc.CreateChangeRequest(context.Background(), validCreateChangeRequestRequest())
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (validation errors are not retried), got %d", attempts)
	}
}

// TestChangeRequestService_PatchChangeRequest_MirrorsToServiceNow covers the
// writeback wiring: on a successful Postgres patch, the mirror's
// PatchChangeRequest is dispatched asynchronously and does not block or
// affect the response.
func TestChangeRequestService_PatchChangeRequest_MirrorsToServiceNow(t *testing.T) {
	title := "new title"
	req := domain.PatchChangeRequestRequest{Title: &title}

	called := make(chan domain.PatchChangeRequestRequest, 1)
	mirror := &stubMirrorChangeRequestService{
		patchChangeRequest: func(_ context.Context, id string, mirrorReq domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			called <- mirrorReq
			return domain.PatchChangeRequestResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.Title == nil || *got.Title != title {
			t.Errorf("mirror got title %v, want %q", got.Title, title)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.PatchChangeRequest was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestChangeRequestService_PatchChangeRequest_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestChangeRequestService_PatchChangeRequest_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	title := "new title"
	req := domain.PatchChangeRequestRequest{Title: &title}

	mirror := &stubMirrorChangeRequestService{
		patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			return domain.PatchChangeRequestResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestChangeRequestService_SearchChangeRequests_ScopesToCallerProjects is the
// core regression guard for the change-request authorization gap: before
// this fix, filters.projectIds was purely a caller-supplied, optional
// narrowing with nothing tying it to the caller's own AccessScope -- an
// external caller omitting it (or asking for someone else's project) saw
// every project's change requests. Confirmed live against a real database
// copy: a real single-project customer contact could see all 3,279 change
// requests in the system this way, not just their own 182.
func TestChangeRequestService_SearchChangeRequests_ScopesToCallerProjects(t *testing.T) {
	tests := []struct {
		name            string
		scope           AccessScope
		requestedFilter []string
		wantRepoCalled  bool
		wantFilter      []string
	}{
		{
			name:            "external caller with no filter is narrowed to their own projects",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: nil,
			wantRepoCalled:  true,
			wantFilter:      []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		},
		{
			name:            "external caller's own filter is intersected with scope, not unioned",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
			wantRepoCalled:  true,
			wantFilter:      []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		},
		{
			name:            "external caller asking for an out-of-scope project alone gets zero results, not everyone's",
			scope:           AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},
			requestedFilter: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
			wantRepoCalled:  false,
		},
		{
			name:            "external caller with no registered projects at all gets zero results",
			scope:           AccessScope{ProjectIDs: nil},
			requestedFilter: nil,
			wantRepoCalled:  false,
		},
		{
			name:            "internal (unrestricted) caller's request passes through untouched",
			scope:           AccessScope{Unrestricted: true},
			requestedFilter: nil,
			wantRepoCalled:  true,
			wantFilter:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var gotFilter []string
			repo := &stubChangeRequestRepo{
				searchChangeRequests: func(_ context.Context, req domain.SearchChangeRequestsRequest) ([]domain.SearchChangeRequestView, int, error) {
					called = true
					gotFilter = req.Filters.ProjectIDs
					return []domain.SearchChangeRequestView{}, 0, nil
				},
			}
			svc := NewChangeRequestService(repo, stubAccess{scope: tc.scope})

			_, err := svc.SearchChangeRequests(context.Background(), domain.SearchChangeRequestsRequest{
				Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: tc.requestedFilter},
				Pagination: domain.Pagination{Limit: 20},
			})
			if err != nil {
				t.Fatalf("SearchChangeRequests: %v", err)
			}
			if called != tc.wantRepoCalled {
				t.Fatalf("repo called = %v, want %v", called, tc.wantRepoCalled)
			}
			if !called {
				return
			}
			if len(gotFilter) != len(tc.wantFilter) {
				t.Fatalf("repo received filters.projectIds = %v, want %v", gotFilter, tc.wantFilter)
			}
			for i := range gotFilter {
				if gotFilter[i] != tc.wantFilter[i] {
					t.Fatalf("repo received filters.projectIds = %v, want %v", gotFilter, tc.wantFilter)
				}
			}
		})
	}
}

// TestChangeRequestService_AggregateChangeRequests_ScopesToCallerProjects
// mirrors the search test's fail-closed short-circuit: an external caller
// whose scope resolves to zero usable projects must get an empty aggregate,
// not the repo's real (unscoped) answer.
func TestChangeRequestService_AggregateChangeRequests_ScopesToCallerProjects(t *testing.T) {
	called := false
	repo := &stubChangeRequestRepo{
		aggregateChangeRequests: func(context.Context, domain.AggregateChangeRequestsRequest) (domain.AggregateResponse, error) {
			called = true
			return domain.AggregateResponse{TotalRecords: 999}, nil
		},
	}
	svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})

	resp, err := svc.AggregateChangeRequests(context.Background(), domain.AggregateChangeRequestsRequest{
		Filters: domain.SearchChangeRequestsFilters{ProjectIDs: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}}, // out of scope
		GroupBy: "state",
	})
	if err != nil {
		t.Fatalf("AggregateChangeRequests: %v", err)
	}
	if called {
		t.Fatal("repo.AggregateChangeRequests must not be called once scoping resolves to zero projects")
	}
	if resp.TotalRecords != 0 || len(resp.Groups) != 0 {
		t.Errorf("resp = %+v, want an empty aggregate", resp)
	}
}

// changeRequestWithProject builds a minimal domain.ChangeRequest with the
// given project id -- everything else is zero-valued, which is fine since
// these tests only assert on scoping/authorization, not field mapping.
func changeRequestWithProject(id, projectID string) domain.ChangeRequest {
	return domain.ChangeRequest{
		SearchChangeRequestView: domain.SearchChangeRequestView{
			ID:      id,
			Project: domain.EntityRef{ID: projectID},
		},
	}
}

// TestChangeRequestService_GetChangeRequest_DeniesOutOfScopeProject is the
// by-id counterpart of the search test: today, GetChangeRequestByID has no
// project check at all, so any caller who knows/guesses a UUID can read a
// change request from any project. Confirmed live: a real registered
// customer contact could fetch CHG0038748, a real change request from a
// completely unrelated project, with nothing stopping them.
func TestChangeRequestService_GetChangeRequest_DeniesOutOfScopeProject(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	repo := &stubChangeRequestRepo{
		getChangeRequestByID: func(context.Context, string) (domain.ChangeRequest, error) {
			return changeRequestWithProject(id, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
		},
	}

	t.Run("external caller outside the project gets NotFound, not the row", func(t *testing.T) {
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		_, err := svc.GetChangeRequest(context.Background(), id)
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("external caller inside the project succeeds", func(t *testing.T) {
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}}})
		cr, err := svc.GetChangeRequest(context.Background(), id)
		if err != nil {
			t.Fatalf("GetChangeRequest: %v", err)
		}
		if cr.ID != id {
			t.Errorf("cr.ID = %q, want %q", cr.ID, id)
		}
	})

	t.Run("internal caller succeeds regardless of project", func(t *testing.T) {
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
		if _, err := svc.GetChangeRequest(context.Background(), id); err != nil {
			t.Fatalf("GetChangeRequest: %v", err)
		}
	})
}

// TestChangeRequestService_PatchChangeRequest_DeniesOutOfScopeProject checks
// the mutation path specifically: the authorization check must happen
// BEFORE repo.PatchChangeRequest is ever called, not after -- an
// out-of-scope caller must be refused outright, never allowed to mutate the
// row and have only the response withheld.
func TestChangeRequestService_PatchChangeRequest_DeniesOutOfScopeProject(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	title := "new title"

	t.Run("external caller outside the project is refused before any write", func(t *testing.T) {
		patchCalled := false
		repo := &stubChangeRequestRepo{
			getChangeRequestByID: func(context.Context, string) (domain.ChangeRequest, error) {
				return changeRequestWithProject(id, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
			},
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest, string) (domain.ChangeRequest, error) {
				patchCalled = true
				return domain.ChangeRequest{}, nil
			},
		}
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		_, err := svc.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{Title: &title})
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
		if patchCalled {
			t.Error("repo.PatchChangeRequest must never be called for an out-of-scope change request")
		}
	})

	t.Run("external caller inside the project succeeds", func(t *testing.T) {
		patchCalled := false
		repo := &stubChangeRequestRepo{
			getChangeRequestByID: func(context.Context, string) (domain.ChangeRequest, error) {
				return changeRequestWithProject(id, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), nil
			},
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest, string) (domain.ChangeRequest, error) {
				patchCalled = true
				return changeRequestWithProject(id, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), nil
			},
		}
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		if _, err := svc.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{Title: &title}); err != nil {
			t.Fatalf("PatchChangeRequest: %v", err)
		}
		if !patchCalled {
			t.Error("repo.PatchChangeRequest should have been called for an in-scope change request")
		}
	})

	t.Run("reassigning to an out-of-scope project is refused even though the current project is in scope", func(t *testing.T) {
		otherProject := "cccccccc-cccc-cccc-cccc-cccccccccccc"
		patchCalled := false
		repo := &stubChangeRequestRepo{
			getChangeRequestByID: func(context.Context, string) (domain.ChangeRequest, error) {
				return changeRequestWithProject(id, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), nil
			},
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest, string) (domain.ChangeRequest, error) {
				patchCalled = true
				return domain.ChangeRequest{}, nil
			},
		}
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		_, err := svc.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ProjectID: &otherProject})
		var nf *apierror.NotFoundError
		if !asNotFoundError(err, &nf) {
			t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
		}
		if patchCalled {
			t.Error("repo.PatchChangeRequest must never be called when reassigning to an out-of-scope project")
		}
	})

	t.Run("internal caller succeeds without a scope check", func(t *testing.T) {
		repo := &stubChangeRequestRepo{
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest, string) (domain.ChangeRequest, error) {
				return changeRequestWithProject(id, "dddddddd-dddd-dddd-dddd-dddddddddddd"), nil
			},
		}
		svc := NewChangeRequestService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

		if _, err := svc.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{Title: &title}); err != nil {
			t.Fatalf("PatchChangeRequest: %v", err)
		}
	})
}
