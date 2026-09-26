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
	getChangeRequestApprovals         func(ctx context.Context, id string) (domain.ChangeRequestApprovals, error)
	decideChangeRequestApproval       func(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error)
}

func (s *stubChangeRequestRepo) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	if s.getChangeRequestApprovals != nil {
		return s.getChangeRequestApprovals(ctx, id)
	}
	panic("not implemented")
}

func (s *stubChangeRequestRepo) DecideChangeRequestApproval(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error) {
	if s.decideChangeRequestApproval != nil {
		return s.decideChangeRequestApproval(ctx, id, approverUserID, decision, actorEmail)
	}
	panic("not implemented")
}

func (s *stubChangeRequestRepo) SearchChangeRequests(context.Context, domain.SearchChangeRequestsRequest, *time.Time, *time.Time, *string, []string) ([]domain.SearchChangeRequestView, int, error) {
	panic("not implemented")
}
func (s *stubChangeRequestRepo) AggregateChangeRequests(context.Context, domain.AggregateChangeRequestsRequest, string, int, *time.Time, *time.Time, *string) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubChangeRequestRepo) GetChangeRequestByID(context.Context, string) (domain.ChangeRequest, error) {
	panic("not implemented")
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
	createChangeRequest         func(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error)
	patchChangeRequest          func(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error)
	decideChangeRequestApproval func(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error)
}

func (s *stubMirrorChangeRequestService) DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
	return s.decideChangeRequestApproval(ctx, id, decision)
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
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)

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
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)

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
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)

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
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)

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
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, dispatcher)

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
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestChangeRequestService_GetChangeRequestApprovals_ValidatesID covers the
// service-layer UUID guard that runs before the repo is ever touched --
// same convention as GetChangeRequest's own id validation.
func TestChangeRequestService_GetChangeRequestApprovals_ValidatesID(t *testing.T) {
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestService(repo, stubUserRepo{})

	_, err := svc.GetChangeRequestApprovals(context.Background(), "not-a-uuid")
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for an invalid id, got %T: %v", err, err)
	}
}

// TestChangeRequestService_GetChangeRequestApprovals_ReadsAlwaysStayOnPostgres
// is the regression guard for this method's own doc comment: even with an
// snMirror configured (DATA_SOURCE=postgres-servicenow-dual-write), the read
// must go through s.repo, never s.snMirror -- the mirror here is left with
// no methods stubbed, so calling it at all would panic.
func TestChangeRequestService_GetChangeRequestApprovals_ReadsAlwaysStayOnPostgres(t *testing.T) {
	want := domain.ChangeRequestApprovals{Approvals: []domain.ChangeRequestApproval{{Stage: "Assess"}}}
	repo := &stubChangeRequestRepo{
		getChangeRequestApprovals: func(_ context.Context, id string) (domain.ChangeRequestApprovals, error) {
			if id != testUUID {
				t.Errorf("repo got id %q, want %q", id, testUUID)
			}
			return want, nil
		},
	}
	mirror := &stubMirrorChangeRequestService{}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)

	got, err := svc.GetChangeRequestApprovals(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Approvals) != 1 || got.Approvals[0].Stage != "Assess" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_RejectsInvalidDecision
// covers validation against the shared changeRequestApprovalDecisions map
// (sn_change_request_service.go) -- rejected before the id is even
// validated against the repo/currentUser, same "cheapest check first"
// ordering PatchChangeRequest already uses.
func TestChangeRequestService_DecideChangeRequestApproval_RejectsInvalidDecision(t *testing.T) {
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestService(repo, stubUserRepo{})

	_, err := svc.DecideChangeRequestApproval(context.Background(), testUUID, "maybe")
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for decision=%q, got %T: %v", "maybe", err, err)
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_RequiresUserIDToken
// covers currentUser's own UnauthorizedError path: with no x-user-id-token
// in context, the repo must never be reached.
func TestChangeRequestService_DecideChangeRequestApproval_RequiresUserIDToken(t *testing.T) {
	repo := &stubChangeRequestRepo{}
	svc := NewChangeRequestService(repo, stubUserRepo{})

	_, err := svc.DecideChangeRequestApproval(context.Background(), testUUID, "approved")
	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected *apierror.UnauthorizedError with no x-user-id-token, got %T: %v", err, err)
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_NoWritebackWhenSnWritebackNil
// covers the plain-Postgres path (no snMirror/snWriteback configured at
// all, DATA_SOURCE=postgres): the repo write still happens and the response
// still comes back, with no dispatch attempted -- stubMirrorChangeRequestService
// is never even constructed here, so any accidental dereference of a nil
// snMirror would panic instead of silently no-op-ing.
func TestChangeRequestService_DecideChangeRequestApproval_NoWritebackWhenSnWritebackNil(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(_ context.Context, id, approverUserID, decision, actorEmail string) (string, error) {
			if approverUserID != testUUID {
				t.Errorf("repo got approverUserID %q, want %q", approverUserID, testUUID)
			}
			if actorEmail != "jane.doe@example.com" {
				t.Errorf("repo got actorEmail %q, want %q", actorEmail, "jane.doe@example.com")
			}
			if decision != "approved" {
				t.Errorf("repo got decision %q, want %q", decision, "approved")
			}
			return "approval-record-id", nil
		},
	}
	svc := NewChangeRequestService(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	})

	resp, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "approval-record-id" || resp.State != "approved" {
		t.Errorf("got %+v, want ID=approval-record-id State=approved", resp)
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_MirrorsToServiceNow
// covers the writeback wiring: on a successful Postgres decide, the
// mirror's DecideChangeRequestApproval is dispatched asynchronously and
// does not block or affect the response -- same shape as
// TestChangeRequestService_PatchChangeRequest_MirrorsToServiceNow above.
func TestChangeRequestService_DecideChangeRequestApproval_MirrorsToServiceNow(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	called := make(chan string, 1)
	mirror := &stubMirrorChangeRequestService{
		decideChangeRequestApproval: func(_ context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
			called <- decision
			return domain.ChangeRequestApprovalDecisionResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(context.Context, string, string, string, string) (string, error) {
			return "approval-record-id", nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, mirror, dispatcher)

	if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, "rejected"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got != "rejected" {
			t.Errorf("mirror got decision %q, want %q", got, "rejected")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.DecideChangeRequestApproval was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already committed, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill -- Postgres-first, best-effort mirror, per this
// method's own doc comment.
func TestChangeRequestService_DecideChangeRequestApproval_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	mirror := &stubMirrorChangeRequestService{
		decideChangeRequestApproval: func(context.Context, string, string) (domain.ChangeRequestApprovalDecisionResponse, error) {
			return domain.ChangeRequestApprovalDecisionResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(context.Context, string, string, string, string) (string, error) {
			return "approval-record-id", nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, mirror, dispatcher)

	if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved"); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestChangeRequestService_DecideChangeRequestApproval_RepoNotFoundPropagates
// covers the "no pending approval for this caller" case: the repo's
// NotFoundError (no approval_stage_approver row matched work_item_id +
// approver_user_id + status='requested') must propagate as-is, with no
// mirror dispatch attempted -- stubMirrorChangeRequestService here has no
// decideChangeRequestApproval configured, so a dispatch attempt would
// panic.
func TestChangeRequestService_DecideChangeRequestApproval_RepoNotFoundPropagates(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	mirror := &stubMirrorChangeRequestService{}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(context.Context, string, string, string, string) (string, error) {
			return "", &apierror.NotFoundError{Msg: "no pending approval found for this change request and caller"}
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
		},
	}, mirror, dispatcher)

	_, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved")
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
	}
}
