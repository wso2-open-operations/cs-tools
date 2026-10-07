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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubProblemRepo is a minimal repository.ProblemRepository whose
// unconfigured methods panic if called -- same convention as
// stubIncidentRepo (incident_service_test.go).
type stubProblemRepo struct {
	createProblem               func(ctx context.Context, req domain.CreateProblemRequest, createdBy string) (domain.ProblemDetail, error)
	createProblemFromServiceNow func(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string) (domain.ProblemDetail, error)
	getProblem                  func(ctx context.Context, id string) (domain.ProblemDetail, error)
	// lastCreatePriority is what CreateProblemFromServiceNow was given.
	lastCreatePriority     string
	updateProblemFields    func(ctx context.Context, req domain.UpdateProblemRequest, actorEmail string) (time.Time, error)
	applyProblemTransition func(ctx context.Context, req domain.UpdateProblemRequest, t repository.ProblemTransition, enforceFrom bool, actorEmail string) (time.Time, error)
	linkWorkaroundProblem  func(ctx context.Context, problemID, incidentID string, groupID *string, actorEmail string) (*string, error)
}

func (s *stubProblemRepo) LinkWorkaroundProblem(ctx context.Context, problemID, incidentID string, groupID *string, actorEmail string) (*string, error) {
	if s.linkWorkaroundProblem != nil {
		return s.linkWorkaroundProblem(ctx, problemID, incidentID, groupID, actorEmail)
	}
	panic("LinkWorkaroundProblem called unexpectedly")
}

func (s *stubProblemRepo) ApplyProblemTransition(ctx context.Context, req domain.UpdateProblemRequest, t repository.ProblemTransition, enforceFrom bool, actorEmail string) (time.Time, error) {
	if s.applyProblemTransition != nil {
		return s.applyProblemTransition(ctx, req, t, enforceFrom, actorEmail)
	}
	panic("ApplyProblemTransition called unexpectedly")
}

func (s *stubProblemRepo) SearchProblems(context.Context, domain.SearchProblemsRequest, []string, []string, []string) ([]domain.SearchProblemView, int, error) {
	panic("not implemented")
}
func (s *stubProblemRepo) AggregateProblems(context.Context, domain.SearchProblemsRequest, []string, []string, []string, string, int) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubProblemRepo) GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error) {
	if s.getProblem != nil {
		return s.getProblem(ctx, id)
	}
	panic("not implemented")
}
func (s *stubProblemRepo) CreateProblemFromServiceNow(ctx context.Context, req domain.CreateProblemRequest, id, number, createdBy string, state *string, priority string) (domain.ProblemDetail, error) {
	s.lastCreatePriority = priority
	if s.createProblemFromServiceNow != nil {
		return s.createProblemFromServiceNow(ctx, req, id, number, createdBy, state)
	}
	panic("CreateProblemFromServiceNow called unexpectedly: Postgres must stay untouched when ServiceNow never accepts the problem")
}
func (s *stubProblemRepo) CreateProblem(ctx context.Context, req domain.CreateProblemRequest, createdBy string, _ repository.ProblemPriorityFields) (domain.ProblemDetail, error) {
	if s.createProblem != nil {
		return s.createProblem(ctx, req, createdBy)
	}
	panic("CreateProblem called unexpectedly")
}
func (s *stubProblemRepo) UpdateProblemFields(ctx context.Context, req domain.UpdateProblemRequest, actorEmail string) (time.Time, error) {
	if s.updateProblemFields != nil {
		return s.updateProblemFields(ctx, req, actorEmail)
	}
	panic("not implemented")
}

// stubMirrorProblemService embeds ProblemService (nil) and overrides only
// CreateProblem/UpdateProblem -- same convention as stubMirrorIncidentService
// (incident_service_test.go). Any other method being called would panic on
// the nil embedded interface, which is the point: this pilot's problem mode
// only ever calls the mirror's CreateProblem (synchronously) and
// UpdateProblem (via the async writeback dispatch).
type stubMirrorProblemService struct {
	ProblemService
	createProblem func(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error)
	updateProblem func(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error)
}

func (s *stubMirrorProblemService) CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	return s.createProblem(ctx, req)
}

func (s *stubMirrorProblemService) UpdateProblem(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
	return s.updateProblem(ctx, req)
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
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)

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
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)

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
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)

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
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)

	_, err := svc.CreateProblem(ctx, validCreateProblemRequest())
	if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
	if mirrorCalled {
		t.Error("ServiceNow must not be called when the caller's identity cannot be resolved")
	}
}

// TestProblemService_UpdateProblem_AtLeastOneFieldRequired guards the
// "nothing to do" rejection: a request setting none of the 5 supported
// fields (and neither of the 2 rejected ones) must fail validation rather
// than silently no-op a write.
func TestProblemService_UpdateProblem_AtLeastOneFieldRequired(t *testing.T) {
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProblemServiceWithSNMirror(&stubProblemRepo{}, &stubMirrorProblemService{}, dispatcher)

	_, err := svc.UpdateProblem(context.Background(), domain.UpdateProblemRequest{ID: testDeploymentUUID})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestProblemService_UpdateProblem_WritesOnlyNonNilFields covers the
// dynamic-SET partial-update contract: only the fields actually set on the
// request must reach ProblemRepository.UpdateProblemFields, and the mirror
// dispatch must carry that same narrow set (never a raw forward of req).
func TestProblemService_UpdateProblem_WritesOnlyNonNilFields(t *testing.T) {
	var gotUpdateReq domain.UpdateProblemRequest
	var gotActorEmail string
	updatedOn := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := &stubProblemRepo{
		updateProblemFields: func(_ context.Context, req domain.UpdateProblemRequest, actorEmail string) (time.Time, error) {
			gotUpdateReq = req
			gotActorEmail = actorEmail
			return updatedOn, nil
		},
		getProblem: func(_ context.Context, id string) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id}, nil
		},
	}

	mirrorCalled := make(chan domain.UpdateProblemRequest, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirrorCalled <- req
			return domain.UpdateProblemResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProblemServiceWithSNMirror(repo, mirror, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	fixNotes := "applied patch"
	resp, err := svc.UpdateProblem(ctx, domain.UpdateProblemRequest{ID: testDeploymentUUID, FixNotes: &fixNotes})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Problem.ID == nil || *resp.Problem.ID != testDeploymentUUID {
		t.Errorf("response problem ID = %v, want %q", resp.Problem.ID, testDeploymentUUID)
	}
	if resp.Problem.UpdatedBy == nil || *resp.Problem.UpdatedBy != "jane.doe@example.com" {
		t.Errorf("response UpdatedBy = %v, want %q", resp.Problem.UpdatedBy, "jane.doe@example.com")
	}

	if gotUpdateReq.ID != testDeploymentUUID || gotUpdateReq.FixNotes == nil || *gotUpdateReq.FixNotes != fixNotes {
		t.Errorf("UpdateProblemFields got %+v, want ID=%q FixNotes=%q", gotUpdateReq, testDeploymentUUID, fixNotes)
	}
	if gotUpdateReq.CauseNotes != nil || gotUpdateReq.Workaround != nil || gotUpdateReq.TargetResolutionDate != nil || gotUpdateReq.AssignedToID != nil {
		t.Errorf("UpdateProblemFields got extra fields set: %+v, want only FixNotes", gotUpdateReq)
	}
	if gotActorEmail != "jane.doe@example.com" {
		t.Errorf("UpdateProblemFields actorEmail = %q, want %q", gotActorEmail, "jane.doe@example.com")
	}

	select {
	case mirrorReq := <-mirrorCalled:
		if mirrorReq.ID != testDeploymentUUID {
			t.Errorf("mirror got ID %q, want %q", mirrorReq.ID, testDeploymentUUID)
		}
		if mirrorReq.FixNotes == nil || *mirrorReq.FixNotes != fixNotes {
			t.Errorf("mirror got FixNotes %v, want %q", mirrorReq.FixNotes, fixNotes)
		}
		if mirrorReq.CauseNotes != nil || mirrorReq.Workaround != nil || mirrorReq.TargetResolutionDate != nil || mirrorReq.AssignedToID != nil {
			t.Errorf("mirror got extra fields set: %+v, want only ID+FixNotes", mirrorReq)
		}
		if mirrorReq.Transition != nil || mirrorReq.AssignmentGroupID != nil {
			t.Errorf("mirror got Transition/AssignmentGroupID set: %+v, want both nil", mirrorReq)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateProblem was never called")
	}
}

// TestProblemService_UpdateProblem_InvalidTargetResolutionDateRejected
// guards the RFC3339 validation on TargetResolutionDate: a malformed value
// must be rejected before any repository write is attempted.
func TestProblemService_UpdateProblem_InvalidTargetResolutionDateRejected(t *testing.T) {
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProblemServiceWithSNMirror(&stubProblemRepo{}, &stubMirrorProblemService{}, dispatcher)

	bad := "not-a-date"
	_, err := svc.UpdateProblem(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), domain.UpdateProblemRequest{ID: testDeploymentUUID, TargetResolutionDate: &bad})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestProblemService_UpdateProblem_MirrorDispatchedEvenWhenReReadFails is
// the CodeRabbit-flagged regression guard on PR #2042: UpdateProblemFields
// has already committed the Postgres write by the time GetProblem runs, so
// a re-read failure must NOT skip the ServiceNow mirror dispatch. Skipping
// it would mean the caller gets an error for a write that actually
// succeeded, ServiceNow never gets the update, and -- since
// s.snWriteback.Dispatch itself would never have been called -- nothing
// would even land in sn_writeback_failures to flag the drift. This asserts
// the mirror fires (checked via the channel) even though GetProblem returns
// an error and UpdateProblem itself therefore also returns an error.
func TestProblemService_UpdateProblem_MirrorDispatchedEvenWhenReReadFails(t *testing.T) {
	repo := &stubProblemRepo{
		updateProblemFields: func(context.Context, domain.UpdateProblemRequest, string) (time.Time, error) {
			return time.Now(), nil
		},
		getProblem: func(context.Context, string) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{}, errors.New("re-read: connection reset")
		},
	}

	mirrorCalled := make(chan domain.UpdateProblemRequest, 1)
	mirror := &stubMirrorProblemService{
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirrorCalled <- req
			return domain.UpdateProblemResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProblemServiceWithSNMirror(repo, mirror, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	fixNotes := "applied patch"
	_, err := svc.UpdateProblem(ctx, domain.UpdateProblemRequest{ID: testDeploymentUUID, FixNotes: &fixNotes})
	if err == nil {
		t.Fatal("expected an error surfaced from the failed post-write re-read")
	}

	select {
	case mirrorReq := <-mirrorCalled:
		if mirrorReq.ID != testDeploymentUUID {
			t.Errorf("mirror got ID %q, want %q", mirrorReq.ID, testDeploymentUUID)
		}
		if mirrorReq.FixNotes == nil || *mirrorReq.FixNotes != fixNotes {
			t.Errorf("mirror got FixNotes %v, want %q", mirrorReq.FixNotes, fixNotes)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateProblem was never called despite the Postgres write succeeding -- the re-read failure must not skip the mirror dispatch")
	}
}

// TestProblemService_CreateProblem_RejectsBeforeServiceNowWhenPostgresWouldFail
// proves values the Postgres insert cannot store are rejected BEFORE the
// ServiceNow call, so no ServiceNow-only record is left behind.
func TestProblemService_CreateProblem_RejectsBeforeServiceNowWhenPostgresWouldFail(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	bad := "bogus"
	cases := map[string]func(*domain.CreateProblemRequest){
		"subject over 512 characters": func(r *domain.CreateProblemRequest) { r.Subject = strings.Repeat("a", 513) },
		"unknown category":            func(r *domain.CreateProblemRequest) { r.Category = &bad },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			mirror := &stubMirrorProblemService{
				createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
					t.Fatal("ServiceNow must not be called")
					return domain.ProblemDetail{}, nil
				},
			}
			svc := NewProblemServiceWithSNMirror(&stubProblemRepo{}, mirror, nil)
			req := validCreateProblemRequest()
			mutate(&req)
			_, err := svc.CreateProblem(ctx, req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
		})
	}
}
