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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubProblemRepo is a minimal repository.ProblemRepository whose
// unconfigured methods panic if called -- same convention as
// stubIncidentRepo (incident_service_test.go).
type stubProblemRepo struct {
	createProblemFromServiceNow func(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error)
}

func (s *stubProblemRepo) SearchProblems(context.Context, domain.SearchProblemsRequest, []string, []string) ([]domain.SearchProblemView, int, error) {
	panic("not implemented")
}
func (s *stubProblemRepo) AggregateProblems(context.Context, domain.SearchProblemsRequest, []string, []string, string, int) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubProblemRepo) GetProblem(context.Context, string) (domain.ProblemDetail, error) {
	panic("not implemented")
}
func (s *stubProblemRepo) CreateProblemFromServiceNow(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error) {
	if s.createProblemFromServiceNow != nil {
		return s.createProblemFromServiceNow(ctx, req, id, number, createdBy, state)
	}
	panic("CreateProblemFromServiceNow called unexpectedly: Postgres must stay untouched when ServiceNow never accepts the problem")
}

// stubMirrorProblemService embeds ProblemService (nil) and overrides only
// CreateProblem -- same convention as stubMirrorIncidentService
// (incident_service_test.go). Any other method being called would panic on
// the nil embedded interface, which is the point: this pilot's problem mode
// only ever calls the mirror's CreateProblem.
type stubMirrorProblemService struct {
	ProblemService
	createProblem func(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error)
}

func (s *stubMirrorProblemService) CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	return s.createProblem(ctx, req)
}

func validCreateProblemRequest() domain.CreateProblemRequest {
	return domain.CreateProblemRequest{Subject: "subject"}
}

// TestProblemService_CreateProblem_SNFailureLeavesPostgresUntouched is the
// pilot's core regression guard for problem CREATE: a single SN failure
// returns an error immediately (no internal retry -- retrying risks creating
// a duplicate ServiceNow record if the create actually succeeded but the
// response was lost) and the Postgres repository must never be called at
// all -- no row, no orphan.
func TestProblemService_CreateProblem_SNFailureLeavesPostgresUntouched(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.ProblemDetail{}, errors.New("sn downstream unreachable")
		},
	}
	// No createProblemFromServiceNow override -- stubProblemRepo panics if
	// it's ever called, which is exactly the assertion: Postgres must stay
	// untouched.
	repo := &stubProblemRepo{}
	svc := NewProblemServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	_, err := svc.CreateProblem(ctx, validCreateProblemRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the problem")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// TestProblemService_CreateProblem_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the other half, mirroring
// TestIncidentService_CreateIncident_SNSuccessCreatesPostgresRowWithMatchingIdentity:
// on ServiceNow success, the Postgres insert must use EXACTLY the
// id/number ServiceNow returned, the confirmed state ServiceNow returned,
// AND createdBy resolved from the calling user's own JWT email claim (not a
// placeholder, and not anything from ServiceNow's response -- ServiceNow's
// problem create response has no createdBy field at all).
func TestProblemService_CreateProblem_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID         = "66666666-6666-6666-6666-666666666666"
		snNumber     = "PRB0023001"
		callerEmail  = "jane.doe@example.com"
		snReturnedID = snID
	)
	snState := "ASSESS"
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, callerEmail))

	mirror := &stubMirrorProblemService{
		createProblem: func(_ context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			id := snReturnedID
			number := snNumber
			return domain.ProblemDetail{ID: &id, Number: &number, State: &snState}, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotCreatedBy string
	var gotState *string
	repo := &stubProblemRepo{
		createProblemFromServiceNow: func(_ context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error) {
			mu.Lock()
			gotID, gotNumber, gotCreatedBy, gotState = id, number, createdBy, state
			mu.Unlock()
			return domain.ProblemDetail{ID: &id, Number: &number, State: state}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	resp, err := svc.CreateProblem(ctx, validCreateProblemRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber {
		t.Errorf("CreateProblemFromServiceNow got id/number (%q, %q), want (%q, %q)", gotID, gotNumber, snID, snNumber)
	}
	if gotCreatedBy != callerEmail {
		t.Errorf("CreateProblemFromServiceNow createdBy = %q, want the calling user's JWT email %q (not a placeholder, and not from ServiceNow's response)", gotCreatedBy, callerEmail)
	}
	if gotState == nil || *gotState != snState {
		t.Errorf("CreateProblemFromServiceNow state = %v, want ServiceNow's confirmed state %q", gotState, snState)
	}
	if resp.ID == nil || *resp.ID != snID {
		t.Errorf("CreateProblem response ID = %v, want %q", resp.ID, snID)
	}
}

// TestProblemService_CreateProblem_DoesNotRetryValidationError covers the
// ValidationError path specifically: it must surface directly, with no
// Postgres write attempted, same as any other single-attempt SN failure.
func TestProblemService_CreateProblem_DoesNotRetryValidationError(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "subject cannot be empty"}
		},
	}
	repo := &stubProblemRepo{}
	svc := NewProblemServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	_, err := svc.CreateProblem(ctx, validCreateProblemRequest())
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

// TestProblemService_CreateProblem_MissingUserIDTokenRejected guards the
// createdBy-resolution precondition that has no equivalent in
// incident/change_request's SN-first path: without a caller identity there
// is nothing valid to write to work_item.created_by, so this must fail
// before ever calling ServiceNow.
func TestProblemService_CreateProblem_MissingUserIDTokenRejected(t *testing.T) {
	ctx := contextWithUserIDToken("")

	mirrorCalled := false
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			mirrorCalled = true
			return domain.ProblemDetail{}, nil
		},
	}
	repo := &stubProblemRepo{}
	svc := NewProblemServiceWithSNMirror(repo, stubAccess{scope: AccessScope{Unrestricted: true}}, mirror)

	_, err := svc.CreateProblem(ctx, validCreateProblemRequest())
	if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
	if mirrorCalled {
		t.Error("ServiceNow must not be called when the caller's identity cannot be resolved")
	}
}

// TestProblemService_RequiresInternalCaller is the core regression guard
// for the problem authorization gap: SearchProblems, AggregateProblems,
// GetProblem, and CreateProblem applied no authorization at all before
// this fix. Confirmed live against a real database copy: 100% of real
// problem rows have project_id = NULL -- problems are internal ITIL/ops
// records, not customer-project-scoped data, so "internal caller only"
// (not project scoping) is the correct fix, mirroring incidentService's
// identical requireInternalCaller pattern.
func TestProblemService_RequiresInternalCaller(t *testing.T) {
	external := stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}}

	assertForbidden := func(t *testing.T, err error) {
		t.Helper()
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	}

	t.Run("SearchProblems", func(t *testing.T) {
		svc := NewProblemService(&stubProblemRepo{}, external)
		_, err := svc.SearchProblems(context.Background(), domain.SearchProblemsRequest{Pagination: domain.Pagination{Limit: 10}})
		assertForbidden(t, err)
	})

	t.Run("AggregateProblems", func(t *testing.T) {
		svc := NewProblemService(&stubProblemRepo{}, external)
		_, err := svc.AggregateProblems(context.Background(), domain.AggregateProblemsRequest{GroupBy: "state"})
		assertForbidden(t, err)
	})

	t.Run("GetProblem", func(t *testing.T) {
		svc := NewProblemService(&stubProblemRepo{}, external)
		_, err := svc.GetProblem(context.Background(), "11111111-1111-1111-1111-111111111111")
		assertForbidden(t, err)
	})

	t.Run("CreateProblem", func(t *testing.T) {
		svc := NewProblemService(&stubProblemRepo{}, external)
		_, err := svc.CreateProblem(context.Background(), domain.CreateProblemRequest{})
		assertForbidden(t, err)
	})

	t.Run("an internal caller is not blocked by the gate itself", func(t *testing.T) {
		svc := NewProblemService(&stubProblemRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected the unconfigured stub repo to be reached (and panic) for an internal caller")
			}
		}()
		_, _ = svc.GetProblem(context.Background(), "11111111-1111-1111-1111-111111111111")
	})
}
