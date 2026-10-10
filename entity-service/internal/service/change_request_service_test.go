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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubChangeRequestRepo is a minimal repository.ChangeRequestRepository
// whose unconfigured methods panic if called -- same convention as
// stubIncidentRepo (incident_service_test.go).
type stubChangeRequestRepo struct {
	createChangeRequest               func(ctx context.Context, req domain.CreateChangeRequestRequest, createdBy string) (domain.CreateChangeRequestResponse, error)
	createChangeRequestFromServiceNow func(ctx context.Context, req domain.CreateChangeRequestRequest, id, number, createdBy string) (domain.CreateChangeRequestResponse, error)
	patchChangeRequest                func(ctx context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, error)
	getChangeRequestByID              func(ctx context.Context, id string) (domain.ChangeRequest, error)
	getChangeRequestApprovals         func(ctx context.Context, id string) (domain.ChangeRequestApprovals, error)
	decideChangeRequestApproval       func(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, error)
	validateChangeRequestLinks        func(ctx context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error)
	getChangeRequestLinkOptions       func(ctx context.Context, req domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error)
	// The two state-reporting variants the dual-write service calls. When
	// unset they fall back to patchChangeRequest / decideChangeRequestApproval
	// with no state move reported, which is what every test written before
	// the state rule existed means.
	patchChangeRequestStates          func(ctx context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, repository.ChangeRequestStates, error)
	decideChangeRequestApprovalStates func(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, repository.ChangeRequestStates, error)
}

func (s *stubChangeRequestRepo) PatchChangeRequestStates(ctx context.Context, id string, req domain.PatchChangeRequestRequest, email string) (domain.ChangeRequest, repository.ChangeRequestStates, error) {
	if s.patchChangeRequestStates != nil {
		return s.patchChangeRequestStates(ctx, id, req, email)
	}
	cr, err := s.PatchChangeRequest(ctx, id, req, email)
	return cr, repository.ChangeRequestStates{}, err
}

func (s *stubChangeRequestRepo) DecideChangeRequestApprovalStates(ctx context.Context, id, approverUserID, decision, actorEmail string) (string, repository.ChangeRequestStates, error) {
	if s.decideChangeRequestApprovalStates != nil {
		return s.decideChangeRequestApprovalStates(ctx, id, approverUserID, decision, actorEmail)
	}
	approvalID, err := s.DecideChangeRequestApproval(ctx, id, approverUserID, decision, actorEmail)
	return approvalID, repository.ChangeRequestStates{}, err
}

// ValidateChangeRequestLinks defaults to accepting everything: most tests do
// not exercise the customer-scope fields.
func (s *stubChangeRequestRepo) ValidateChangeRequestLinks(ctx context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
	if s.validateChangeRequestLinks != nil {
		return s.validateChangeRequestLinks(ctx, sel)
	}
	return domain.ChangeRequestLinkSet{}, nil
}

func (s *stubChangeRequestRepo) GetChangeRequestLinkOptions(ctx context.Context, req domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error) {
	if s.getChangeRequestLinkOptions != nil {
		return s.getChangeRequestLinkOptions(ctx, req)
	}
	panic("not implemented")
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
func (s *stubChangeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	if s.getChangeRequestByID != nil {
		return s.getChangeRequestByID(ctx, id)
	}
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

func (s *stubChangeRequestRepo) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest, createdBy string) (domain.CreateChangeRequestResponse, error) {
	if s.createChangeRequest != nil {
		return s.createChangeRequest(ctx, req, createdBy)
	}
	panic("CreateChangeRequest called unexpectedly")
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
	normal := domain.ChangeRequestTypeNormal
	return domain.CreateChangeRequestRequest{Subject: "subject", Type: &normal}
}

// TestChangeRequestService_CreateChangeRequest_RequiresType: a change request
// must be created as standard, normal or emergency -- the type decides its
// whole approval flow. A missing or other type is refused before ServiceNow
// (dual-write) or Postgres (plain) is touched.
func TestChangeRequestService_CreateChangeRequest_RequiresType(t *testing.T) {
	azure := domain.ChangeRequestTypeAzure
	empty := domain.ChangeRequestType("")
	cases := map[string]*domain.ChangeRequestType{"missing": nil, "empty": &empty, "azure": &azure}
	for name, typ := range cases {
		t.Run("dual-write/"+name, func(t *testing.T) {
			mirror := &stubMirrorChangeRequestService{
				createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
					t.Fatal("ServiceNow must never be called without a valid type")
					return domain.CreateChangeRequestResponse{}, nil
				},
			}
			svc := NewChangeRequestServiceWithSNMirror(&stubChangeRequestRepo{}, stubUserRepo{}, mirror)
			_, err := svc.CreateChangeRequest(context.Background(), domain.CreateChangeRequestRequest{Subject: "subject", Type: typ})
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
			if !strings.Contains(ve.Msg, "standard, normal or emergency") {
				t.Errorf("message %q should name the three allowed types", ve.Msg)
			}
		})
		t.Run("postgres/"+name, func(t *testing.T) {
			svc := NewChangeRequestService(&stubChangeRequestRepo{}, stubUserRepo{})
			_, err := svc.CreateChangeRequest(contextWithUserIDToken(fakeJWTWithEmail(t, "a@example.com")), domain.CreateChangeRequestRequest{Subject: "subject", Type: typ})
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
	for _, ok := range domain.ChangeRequestCreatableTypes {
		ok := ok
		t.Run("accepts/"+string(ok), func(t *testing.T) {
			repo := &stubChangeRequestRepo{createChangeRequest: func(_ context.Context, req domain.CreateChangeRequestRequest, _ string) (domain.CreateChangeRequestResponse, error) {
				return domain.CreateChangeRequestResponse{Message: "ok"}, nil
			}}
			svc := NewChangeRequestService(repo, stubUserRepo{})
			if _, err := svc.CreateChangeRequest(contextWithUserIDToken(fakeJWTWithEmail(t, "a@example.com")), domain.CreateChangeRequestRequest{Subject: "subject", Type: &ok}); err != nil {
				t.Fatalf("type %s: %v", ok, err)
			}
		})
	}
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

// TestChangeRequestService_DecideChangeRequestApproval_NonMemberRefusalPropagates:
// a caller outside the customer group of a change waiting on its customer
// stage gets the repository's readable ForbiddenError unchanged, and nothing
// is mirrored to ServiceNow for a decision that was never recorded.
func TestChangeRequestService_DecideChangeRequestApproval_NonMemberRefusalPropagates(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "outsider@example.com"))
	const msg = `only members of the customer group "Artemis Customers" can approve or reject the customer's approval of this change request`
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorChangeRequestService{
		decideChangeRequestApproval: func(context.Context, string, string) (domain.ChangeRequestApprovalDecisionResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.ChangeRequestApprovalDecisionResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(context.Context, string, string, string, string) (string, error) {
			return "", &apierror.ForbiddenError{Msg: msg}
		},
	}
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "outsider@example.com"}, nil
		},
	}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))

	_, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved")
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || fe.Msg != msg {
		t.Fatalf("err = %v, want the repository's ForbiddenError %q", err, msg)
	}
	select {
	case <-mirrorCalled:
		t.Fatal("a refused decision was mirrored to ServiceNow")
	case <-time.After(200 * time.Millisecond):
	}
}

// TestChangeRequestService_DecideChangeRequestApproval_ExternalUserRefusalPropagates:
// a customer holding a row on an internal stage (approver pools are
// INTERNAL-only) gets the repository's readable ForbiddenError unchanged -- the
// service adds no mapping of its own, so the 403 and its reason reach the
// caller -- and nothing is mirrored to ServiceNow for a decision that was never
// recorded.
func TestChangeRequestService_DecideChangeRequestApproval_ExternalUserRefusalPropagates(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "dave.mendis@example.com"))
	const msg = "only active internal (WSO2) users can approve or reject the Peer Approval stage of a change request; external/customer users cannot"
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorChangeRequestService{
		decideChangeRequestApproval: func(context.Context, string, string) (domain.ChangeRequestApprovalDecisionResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.ChangeRequestApprovalDecisionResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApproval: func(_ context.Context, _, approverUserID, _, actorEmail string) (string, error) {
			if approverUserID != testUUID || actorEmail != "dave.mendis@example.com" {
				t.Errorf("repo got approver %q / actor %q, want the caller", approverUserID, actorEmail)
			}
			return "", &apierror.ForbiddenError{Msg: msg}
		},
	}
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: testUUID, Email: "dave.mendis@example.com"}, nil
		},
	}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))

	for _, decision := range []string{"approved", "rejected"} {
		_, err := svc.DecideChangeRequestApproval(ctx, testUUID, decision)
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) || fe.Msg != msg {
			t.Fatalf("%s: err = %v, want the repository's ForbiddenError %q", decision, err, msg)
		}
	}
	select {
	case <-mirrorCalled:
		t.Fatal("a refused decision was mirrored to ServiceNow")
	case <-time.After(200 * time.Millisecond):
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
// approver_user_id + state='REQUESTED') must propagate as-is, with no
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

// TestChangeRequestService_PatchChangeRequest_AcceptsFieldParityFieldsAlone
// proves a request carrying only one of the field-parity fields (previously
// rejected with "at least one field must be provided") reaches the
// repository, and that durationInput, the one field with no Postgres backing,
// is rejected rather than silently dropped.
func TestChangeRequestService_PatchChangeRequest_AcceptsFieldParityFieldsAlone(t *testing.T) {
	text := "2 hours"
	ptr := &text
	var got domain.PatchChangeRequestRequest
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, req domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			got = req
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	svc := NewChangeRequestService(repo, stubUserRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{RollbackDurationText: &ptr}); err != nil {
		t.Fatalf("rollbackDurationText alone: unexpected error: %v", err)
	}
	if got.RollbackDurationText == nil {
		t.Fatal("repository never saw RollbackDurationText")
	}

	// durationInput is the one field with no Postgres backing left.
	d := 60
	dd := &d
	_, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{DurationInput: &dd})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("durationInput: expected ValidationError, got %T: %v", err, err)
	}
}

// TestChangeRequestService_PatchChangeRequest_CustomerGateFlagsAlone: the
// creation form's two checkboxes (customerApprovalRequired /
// customerReviewRequired) are a patch on their own -- not rejected as "at least
// one field must be provided" -- and reach the repository unchanged.
func TestChangeRequestService_PatchChangeRequest_CustomerGateFlagsAlone(t *testing.T) {
	yes, no := true, false
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"customerApprovalRequired": {CustomerApprovalRequired: &yes},
		"customerReviewRequired":   {CustomerReviewRequired: &no},
		"both":                     {CustomerApprovalRequired: &no, CustomerReviewRequired: &yes},
	} {
		t.Run(name, func(t *testing.T) {
			var got domain.PatchChangeRequestRequest
			repo := &stubChangeRequestRepo{
				patchChangeRequest: func(_ context.Context, id string, r domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
					got = r
					return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
				},
			}
			svc := NewChangeRequestService(repo, stubUserRepo{})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
			if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CustomerApprovalRequired != req.CustomerApprovalRequired || got.CustomerReviewRequired != req.CustomerReviewRequired {
				t.Fatalf("repository saw %+v/%+v, want the request's own flags", got.CustomerApprovalRequired, got.CustomerReviewRequired)
			}
		})
	}
}

// TestChangeRequestService_PatchChangeRequest_CustomerGateFlagsStayOutOfTheMirror:
// ServiceNow's change request API has no field the service can name for the
// two checkboxes, so they are stripped from the dual-write mirror; a PATCH that
// carried nothing else is not mirrored at all (an empty ServiceNow PATCH would
// only record a writeback failure).
func TestChangeRequestService_PatchChangeRequest_CustomerGateFlagsStayOutOfTheMirror(t *testing.T) {
	yes := true
	title := "new title"
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("mixed with other fields: flags stripped, rest mirrored", func(t *testing.T) {
		called := make(chan domain.PatchChangeRequestRequest, 1)
		mirror := &stubMirrorChangeRequestService{
			patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
				called <- r
				return domain.PatchChangeRequestResponse{}, nil
			},
		}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{Title: &title, CustomerApprovalRequired: &yes, CustomerReviewRequired: &yes}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case got := <-called:
			if got.Title == nil || *got.Title != title {
				t.Errorf("mirror title = %v, want %q", got.Title, title)
			}
			if got.CustomerApprovalRequired != nil || got.CustomerReviewRequired != nil {
				t.Errorf("mirror saw the Postgres-only flags: %v/%v", got.CustomerApprovalRequired, got.CustomerReviewRequired)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("mirror.PatchChangeRequest was never called")
		}
	})

	t.Run("flags alone: nothing to mirror", func(t *testing.T) {
		mirror := &stubMirrorChangeRequestService{
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
				t.Error("mirror was called for a PATCH that only carried the Postgres-only flags")
				return domain.PatchChangeRequestResponse{}, nil
			},
		}
		failures := &recordingSNWritebackFailures{}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{CustomerApprovalRequired: &yes}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(150 * time.Millisecond) // the dispatch is asynchronous; give a wrongly-fired one time to show
		if got := failures.count(); got != 0 {
			t.Errorf("writeback failures = %d, want 0", got)
		}
	})
}

// TestChangeRequestService_CreateChangeRequest_EmergencyWithACustomerBoxIsRefusedBeforeAnyWrite:
// an Emergency change takes no customer step, so a create that ticks either box is a 400 --
// on the plain path before the repository is called, and on the dual-write path BEFORE
// the previous system is called (a refusal after it accepted the create would strand a
// record there with no PostgreSQL row). The same create with the boxes off, and a Normal
// change with them on, go through.
func TestChangeRequestService_CreateChangeRequest_EmergencyWithACustomerBoxIsRefusedBeforeAnyWrite(t *testing.T) {
	yes, no := true, false
	emergency := domain.ChangeRequestTypeEmergency
	build := func(approval, review *bool) domain.CreateChangeRequestRequest {
		req := validCreateChangeRequestRequest()
		req.Type = &emergency
		req.CustomerApprovalRequired, req.CustomerReviewRequired = approval, review
		return req
	}
	ok := func() domain.CreateChangeRequestResponse {
		resp := domain.CreateChangeRequestResponse{Message: "ok"}
		resp.ChangeRequest.ID = testUUID
		return resp
	}
	const reason = "Emergency changes proceed without customer consent, so customer approval and customer review cannot be required"
	for name, boxes := range map[string][2]*bool{"approval": {&yes, nil}, "review": {nil, &yes}, "both": {&yes, &yes}} {
		boxes := boxes
		t.Run("plain/"+name, func(t *testing.T) {
			repo := &stubChangeRequestRepo{
				createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest, string) (domain.CreateChangeRequestResponse, error) {
					t.Fatal("the repository must not be called for an Emergency change with a customer box")
					return domain.CreateChangeRequestResponse{}, nil
				},
			}
			svc := NewChangeRequestService(repo, stubUserRepo{})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
			_, err := svc.CreateChangeRequest(ctx, build(boxes[0], boxes[1]))
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) || !strings.HasPrefix(ve.Msg, reason) {
				t.Fatalf("err = %v (%T), want a ValidationError that gives the reason", err, err)
			}
		})
		t.Run("previous-system-first/"+name, func(t *testing.T) {
			mirror := &stubMirrorChangeRequestService{
				createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
					t.Fatal("the previous system must never be called for an Emergency change with a customer box")
					return domain.CreateChangeRequestResponse{}, nil
				},
			}
			repo := &stubChangeRequestRepo{
				createChangeRequestFromServiceNow: func(context.Context, domain.CreateChangeRequestRequest, string, string, string) (domain.CreateChangeRequestResponse, error) {
					t.Fatal("the repository must not be called for an Emergency change with a customer box")
					return domain.CreateChangeRequestResponse{}, nil
				},
			}
			svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
			_, err := svc.CreateChangeRequest(context.Background(), build(boxes[0], boxes[1]))
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) || !strings.HasPrefix(ve.Msg, reason) {
				t.Fatalf("err = %v (%T), want a ValidationError that gives the reason", err, err)
			}
		})
	}
	t.Run("an Emergency change with the boxes off, and a Normal one with them on, are created", func(t *testing.T) {
		repo := &stubChangeRequestRepo{
			createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest, string) (domain.CreateChangeRequestResponse, error) {
				return ok(), nil
			},
		}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		if _, err := svc.CreateChangeRequest(ctx, build(&no, &no)); err != nil {
			t.Fatalf("an Emergency change with both boxes off: %v", err)
		}
		if _, err := svc.CreateChangeRequest(ctx, build(nil, nil)); err != nil {
			t.Fatalf("an Emergency change with no box: %v", err)
		}
		normal := validCreateChangeRequestRequest()
		normal.CustomerApprovalRequired, normal.CustomerReviewRequired = &yes, &yes
		if _, err := svc.CreateChangeRequest(ctx, normal); err != nil {
			t.Fatalf("a Normal change with both boxes: %v", err)
		}
	})
}

// TestChangeRequestService_CreateChangeRequest_PassesCustomerGateFlags: both
// Postgres create paths (plain and ServiceNow-first) hand the checkboxes to the
// repository untouched.
func TestChangeRequestService_CreateChangeRequest_PassesCustomerGateFlags(t *testing.T) {
	yes, no := true, false
	want := validCreateChangeRequestRequest()
	want.CustomerApprovalRequired, want.CustomerReviewRequired = &yes, &no
	ok := func() domain.CreateChangeRequestResponse {
		resp := domain.CreateChangeRequestResponse{Message: "ok"}
		resp.ChangeRequest.ID = testUUID
		return resp
	}

	t.Run("plain", func(t *testing.T) {
		var got domain.CreateChangeRequestRequest
		repo := &stubChangeRequestRepo{
			createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest, _ string) (domain.CreateChangeRequestResponse, error) {
				got = r
				return ok(), nil
			},
		}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		if _, err := svc.CreateChangeRequest(ctx, want); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.CustomerApprovalRequired == nil || !*got.CustomerApprovalRequired || got.CustomerReviewRequired == nil || *got.CustomerReviewRequired {
			t.Fatalf("repository saw %v/%v, want true/false", got.CustomerApprovalRequired, got.CustomerReviewRequired)
		}
	})

	t.Run("servicenow-first", func(t *testing.T) {
		var got domain.CreateChangeRequestRequest
		mirror := &stubMirrorChangeRequestService{
			createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
				return ok(), nil
			},
		}
		repo := &stubChangeRequestRepo{
			createChangeRequestFromServiceNow: func(_ context.Context, r domain.CreateChangeRequestRequest, _, _, _ string) (domain.CreateChangeRequestResponse, error) {
				got = r
				return ok(), nil
			},
		}
		svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
		if _, err := svc.CreateChangeRequest(context.Background(), want); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.CustomerApprovalRequired == nil || !*got.CustomerApprovalRequired || got.CustomerReviewRequired == nil || *got.CustomerReviewRequired {
			t.Fatalf("repository saw %v/%v, want true/false", got.CustomerApprovalRequired, got.CustomerReviewRequired)
		}
	})
}

// TestChangeRequestService_GetChangeRequestApprovals_StampsViewerEmail is the
// regression guard for "Approve/Reject disabled for the approver themselves":
// an internal user resolved from the user token alone comes back from
// AccessService.ResolveScope Unrestricted with an EMPTY ViewerEmail, and the
// repository's markCanDecide leaves every canDecide false without one. The
// service must therefore hand the repo an identity naming the caller (taken
// from the same x-user-id-token DecideChangeRequestApproval uses). The scope
// here is produced by the REAL AccessService, exactly as
// callerIdentityMiddleware stamps it on the HTTP path.
func TestChangeRequestService_GetChangeRequestApprovals_StampsViewerEmail(t *testing.T) {
	const email = "jane.doe@example.com"

	var seen repository.SearchScope
	var seenOK bool
	repo := &stubChangeRequestRepo{
		getChangeRequestApprovals: func(ctx context.Context, _ string) (domain.ChangeRequestApprovals, error) {
			seen, seenOK = repository.CallerIdentityFromContext(ctx)
			return domain.ChangeRequestApprovals{}, nil
		},
	}
	svc := NewChangeRequestService(repo, stubUserRepo{})

	// The HTTP path: auth.Middleware validates the token, the identity
	// middleware resolves + stamps the scope, the handler calls the service.
	httpCtx := func(t *testing.T, access AccessService) context.Context {
		t.Helper()
		ctx := auth.WithIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, email)), auth.Identity{Validated: true, UserEmail: email})
		scope, err := access.ResolveScope(ctx)
		if err != nil {
			t.Fatalf("ResolveScope: %v", err)
		}
		return repository.WithCallerIdentity(ctx, scope)
	}

	t.Run("internal user resolved from the user token (no CSM-portal client config)", func(t *testing.T) {
		access := NewAccessService(&fakeAccessRepo{users: []repository.AccessUser{userOf("INTERNAL", true)}}, AccessClientConfig{})
		ctx := httpCtx(t, access)
		if pre, _ := repository.CallerIdentityFromContext(ctx); pre.ViewerEmail != "" {
			t.Fatalf("precondition: ResolveScope now sets ViewerEmail (%q); this regression test needs revisiting", pre.ViewerEmail)
		}
		seen, seenOK = repository.SearchScope{}, false
		if _, err := svc.GetChangeRequestApprovals(ctx, testUUID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !seenOK || seen.ViewerEmail != email || !seen.Unrestricted {
			t.Errorf("repo saw identity %+v (present=%v), want Unrestricted with ViewerEmail %q", seen, seenOK, email)
		}
	})

	t.Run("an identity that already names the viewer is left alone", func(t *testing.T) {
		ctx := repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "someone.else@example.com")),
			repository.SearchScope{Unrestricted: true, ViewerEmail: email})
		seen, seenOK = repository.SearchScope{}, false
		if _, err := svc.GetChangeRequestApprovals(ctx, testUUID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen.ViewerEmail != email {
			t.Errorf("ViewerEmail = %q, want it kept as %q", seen.ViewerEmail, email)
		}
	})

	t.Run("no caller identity on ctx stays absent (fails closed, nothing invented)", func(t *testing.T) {
		seen, seenOK = repository.SearchScope{}, true
		if _, err := svc.GetChangeRequestApprovals(contextWithUserIDToken(fakeJWTWithEmail(t, email)), testUUID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seenOK {
			t.Errorf("repo saw an identity %+v, want none", seen)
		}
	})
}

const (
	scopeTestProjectID    = "11111111-2222-3333-4444-555555555555"
	scopeTestDeploymentID = "66666666-7777-8888-9999-aaaaaaaaaaaa"
)

// The customer-scope fields are shape-checked (UUIDs, bounded) before anything
// is written or sent anywhere, on create (plain and ServiceNow-first) and PATCH.
func TestChangeRequestService_ScopeFields_ShapeValidation(t *testing.T) {
	many := make([]string, maxChangeRequestScopeIDs+1)
	for i := range many {
		many[i] = scopeTestDeploymentID
	}
	bad := "not-a-uuid"
	creates := map[string]func(*domain.CreateChangeRequestRequest){
		"projectId":            func(r *domain.CreateChangeRequestRequest) { r.ProjectID = &bad },
		"deploymentIds":        func(r *domain.CreateChangeRequestRequest) { r.DeploymentIDs = []string{bad} },
		"deploymentProductIds": func(r *domain.CreateChangeRequestRequest) { r.DeploymentProductIDs = []string{bad} },
		"too many":             func(r *domain.CreateChangeRequestRequest) { r.DeploymentIDs = many },
	}
	for name, mod := range creates {
		mod := mod
		t.Run("create/"+name, func(t *testing.T) {
			req := validCreateChangeRequestRequest()
			mod(&req)
			mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
				t.Fatal("ServiceNow was called for an invalid request")
				return domain.CreateChangeRequestResponse{}, nil
			}}
			for label, svc := range map[string]ChangeRequestService{
				"plain":            NewChangeRequestService(&stubChangeRequestRepo{}, stubUserRepo{}),
				"servicenow-first": NewChangeRequestServiceWithSNMirror(&stubChangeRequestRepo{}, stubUserRepo{}, mirror),
			} {
				_, err := svc.CreateChangeRequest(contextWithUserIDToken(fakeJWTWithEmail(t, "a@example.com")), req)
				var ve *apierror.ValidationError
				if !asValidationError(err, &ve) {
					t.Fatalf("%s: expected *apierror.ValidationError, got %T: %v", label, err, err)
				}
			}
		})
	}

	patches := map[string]domain.PatchChangeRequestRequest{
		"projectId":            {ProjectID: &bad},
		"deploymentIds":        {DeploymentIDs: &[]string{bad}},
		"deploymentProductIds": {DeploymentProductIDs: &[]string{bad}},
		"too many":             {DeploymentIDs: &many},
		"blank comment":        {Comment: func() *string { s := "  "; return &s }()},
		"blank work note":      {WorkNote: func() *string { s := ""; return &s }()},
	}
	for name, req := range patches {
		req := req
		t.Run("patch/"+name, func(t *testing.T) {
			svc := NewChangeRequestService(&stubChangeRequestRepo{}, stubUserRepo{}) // repo panics if reached
			_, err := svc.PatchChangeRequest(contextWithUserIDToken(fakeJWTWithEmail(t, "a@example.com")), testUUID, req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// PATCH accepts the scope fields and the journal entries (alone), hands them to
// the repository untouched, and rejects only durationInput.
func TestChangeRequestService_PatchChangeRequest_ScopeFieldsReachTheRepository(t *testing.T) {
	var got domain.PatchChangeRequestRequest
	repo := &stubChangeRequestRepo{patchChangeRequest: func(_ context.Context, id string, req domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
		got = req
		return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
	}}
	svc := NewChangeRequestService(repo, stubUserRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	comment, note := "hello", "internal"
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"deploymentIds":        {DeploymentIDs: &[]string{scopeTestDeploymentID}},
		"empty deploymentIds":  {DeploymentIDs: &[]string{}},
		"deploymentProductIds": {DeploymentProductIDs: &[]string{scopeTestDeploymentID}},
		"comment":              {Comment: &comment},
		"workNote":             {WorkNote: &note},
		"projectId":            {ProjectID: func() *string { s := scopeTestProjectID; return &s }()},
	} {
		got = domain.PatchChangeRequestRequest{}
		if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
			t.Fatalf("%s alone: %v", name, err)
		}
		if got.DeploymentIDs != req.DeploymentIDs || got.DeploymentProductIDs != req.DeploymentProductIDs ||
			got.Comment != req.Comment || got.WorkNote != req.WorkNote || got.ProjectID != req.ProjectID {
			t.Fatalf("%s: the repository saw a different request", name)
		}
	}
}

// The dual-write mirror gets only what ServiceNow models: projectId on PATCH,
// comment and workNote go through; deploymentIds and deploymentProductIds
// (Postgres-derived) are stripped; a
// PATCH carrying only those has nothing to mirror.
func TestChangeRequestService_PatchChangeRequest_ScopeFieldsStayOutOfTheMirror(t *testing.T) {
	repo := &stubChangeRequestRepo{patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
		return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
	}}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	project, comment := scopeTestProjectID, "mirrored comment"

	t.Run("mixed: scope lists stripped, project and journal mirrored", func(t *testing.T) {
		called := make(chan domain.PatchChangeRequestRequest, 1)
		mirror := &stubMirrorChangeRequestService{patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			called <- r
			return domain.PatchChangeRequestResponse{}, nil
		}}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		ids := []string{scopeTestDeploymentID}
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{
			ProjectID: &project, Comment: &comment, DeploymentIDs: &ids, DeploymentProductIDs: &ids,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case got := <-called:
			if got.DeploymentIDs != nil || got.DeploymentProductIDs != nil {
				t.Errorf("mirror saw the Postgres-only lists: %v/%v", got.DeploymentIDs, got.DeploymentProductIDs)
			}
			if got.ProjectID == nil || *got.ProjectID != project || got.Comment == nil || *got.Comment != comment {
				t.Errorf("mirror lost projectId/comment: %v/%v", got.ProjectID, got.Comment)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("mirror.PatchChangeRequest was never called")
		}
	})

	t.Run("scope lists alone: nothing to mirror", func(t *testing.T) {
		mirror := &stubMirrorChangeRequestService{patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			t.Error("mirror was called for a PATCH that only carried Postgres-only scope lists")
			return domain.PatchChangeRequestResponse{}, nil
		}}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		ids := []string{scopeTestDeploymentID}
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{DeploymentIDs: &ids}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(150 * time.Millisecond)
	})
}

// ServiceNow-first create: the scope selection is validated against PostgreSQL
// BEFORE ServiceNow is called (a refusal afterwards would strand the change
// request in ServiceNow); ServiceNow then receives the request without the
// fields it cannot model, while PostgreSQL receives all of them.
func TestChangeRequestService_CreateChangeRequest_ScopeValidatedBeforeSNAndStrippedFromIt(t *testing.T) {
	project := scopeTestProjectID
	category, comment, note := domain.ChangeRequestCategoryDevOps, "c", "w"
	req := validCreateChangeRequestRequest()
	req.ProjectID, req.DeploymentIDs = &project, []string{scopeTestDeploymentID}
	req.DeploymentProductIDs = []string{scopeTestDeploymentID}
	req.Category, req.Comment, req.WorkNote = &category, &comment, &note

	t.Run("refused by PostgreSQL: ServiceNow never called", func(t *testing.T) {
		mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			t.Fatal("ServiceNow was called although the scope selection is invalid")
			return domain.CreateChangeRequestResponse{}, nil
		}}
		repo := &stubChangeRequestRepo{validateChangeRequestLinks: func(context.Context, domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
			return domain.ChangeRequestLinkSet{}, &apierror.ValidationError{Msg: "deploymentIds contains a deployment that does not belong to the selected project: x"}
		}}
		svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
		_, err := svc.CreateChangeRequest(context.Background(), req)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "does not belong to the selected project") {
			t.Fatalf("err = %v, want the repository's ValidationError", err)
		}
	})

	t.Run("customerGroupId / environmentIds: refused before anything else, ServiceNow never called", func(t *testing.T) {
		mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			t.Fatal("ServiceNow was called although a removed field was sent")
			return domain.CreateChangeRequestResponse{}, nil
		}}
		repo := &stubChangeRequestRepo{validateChangeRequestLinks: func(context.Context, domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
			t.Fatal("the repository was consulted for a request carrying a removed field")
			return domain.ChangeRequestLinkSet{}, nil
		}}
		for name, tc := range map[string]struct {
			mod  func(*domain.CreateChangeRequestRequest)
			want string
		}{
			"customerGroupId": {func(r *domain.CreateChangeRequestRequest) { g := scopeTestProjectID; r.CustomerGroupID = &g },
				"customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"},
			"environmentIds": {func(r *domain.CreateChangeRequestRequest) { r.EnvironmentIDs = []string{scopeTestDeploymentID} },
				"environmentIds is no longer supported: deployments carry the environment"},
		} {
			r := req
			tc.mod(&r)
			for label, svc := range map[string]ChangeRequestService{
				"plain":            NewChangeRequestService(repo, stubUserRepo{}),
				"servicenow-first": NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror),
			} {
				_, err := svc.CreateChangeRequest(context.Background(), r)
				var ve *apierror.ValidationError
				if !asValidationError(err, &ve) || ve.Msg != tc.want {
					t.Fatalf("%s/%s: err = %v, want ValidationError %q", name, label, err, tc.want)
				}
			}
		}
	})

	t.Run("accepted: ServiceNow gets the modelled fields only, PostgreSQL gets everything", func(t *testing.T) {
		var sawSel domain.ChangeRequestLinkSelection
		var toSN, toPG domain.CreateChangeRequestRequest
		mirror := &stubMirrorChangeRequestService{createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			toSN = r
			resp := domain.CreateChangeRequestResponse{}
			resp.ChangeRequest.ID = testUUID
			return resp, nil
		}}
		repo := &stubChangeRequestRepo{
			validateChangeRequestLinks: func(_ context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
				sawSel = sel
				return domain.ChangeRequestLinkSet{}, nil
			},
			createChangeRequestFromServiceNow: func(_ context.Context, r domain.CreateChangeRequestRequest, id, _, _ string) (domain.CreateChangeRequestResponse, error) {
				toPG = r
				resp := domain.CreateChangeRequestResponse{}
				resp.ChangeRequest.ID = id
				return resp, nil
			},
		}
		svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
		if _, err := svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sawSel.ProjectID == nil || *sawSel.ProjectID != project || len(sawSel.DeploymentIDs) != 1 || len(sawSel.DeploymentProductIDs) != 1 {
			t.Errorf("validated selection = %+v", sawSel)
		}
		if toSN.ProjectID != nil || toSN.DeploymentIDs != nil || toSN.DeploymentProductIDs != nil {
			t.Errorf("ServiceNow saw the Postgres-only scope: %+v", toSN)
		}
		if toSN.Category == nil || toSN.Comment == nil || toSN.WorkNote == nil {
			t.Errorf("ServiceNow lost category/comment/workNote: %+v", toSN)
		}
		if toPG.ProjectID == nil || len(toPG.DeploymentIDs) != 1 || len(toPG.DeploymentProductIDs) != 1 || toPG.Comment == nil || toPG.WorkNote == nil {
			t.Errorf("PostgreSQL did not get the full request: %+v", toPG)
		}
	})
}

// The form's lookup validates its input and is Postgres-only.
func TestChangeRequestService_GetChangeRequestLinkOptions(t *testing.T) {
	var got domain.ChangeRequestLinkOptionsRequest
	repo := &stubChangeRequestRepo{getChangeRequestLinkOptions: func(_ context.Context, r domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error) {
		got = r
		return domain.ChangeRequestLinkOptionsResponse{
			Deployments:      []domain.ChangeRequestDeploymentOption{{ID: "d"}},
			CustomerContacts: []domain.ChangeRequestCustomerContact{{ID: "c", Name: "Jane Doe", Email: "jane@example.com"}},
		}, nil
	}}
	svc := NewChangeRequestService(repo, stubUserRepo{})
	resp, err := svc.GetChangeRequestLinkOptions(context.Background(), domain.ChangeRequestLinkOptionsRequest{ProjectID: scopeTestProjectID, DeploymentIDs: []string{scopeTestDeploymentID}})
	if err == nil && (len(resp.CustomerContacts) != 1 || resp.CustomerContacts[0].ID != "c") {
		t.Fatalf("customerContacts not passed through: %+v", resp.CustomerContacts)
	}
	if err != nil || len(resp.Deployments) != 1 || got.ProjectID != scopeTestProjectID || len(got.DeploymentIDs) != 1 {
		t.Fatalf("valid request: resp=%+v err=%v got=%+v", resp, err, got)
	}
	for name, req := range map[string]domain.ChangeRequestLinkOptionsRequest{
		"missing projectId":   {},
		"bad projectId":       {ProjectID: "nope"},
		"bad deploymentIds":   {ProjectID: scopeTestProjectID, DeploymentIDs: []string{"nope"}},
		"too many deployment": {ProjectID: scopeTestProjectID, DeploymentIDs: make([]string, maxChangeRequestScopeIDs+1)},
	} {
		_, err := svc.GetChangeRequestLinkOptions(context.Background(), req)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("%s: expected *apierror.ValidationError, got %T: %v", name, err, err)
		}
	}
	// The ServiceNow-only service refuses.
	_, err = (&snChangeRequestService{}).GetChangeRequestLinkOptions(context.Background(), domain.ChangeRequestLinkOptionsRequest{ProjectID: scopeTestProjectID})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("servicenow data source: expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// customerGroupId / environmentIds on PATCH are refused with a clear 400 before
// the repository is reached (and so before anything is mirrored to ServiceNow),
// whatever else the request carries.
func TestChangeRequestService_PatchChangeRequest_RefusesRemovedFields(t *testing.T) {
	svc := NewChangeRequestService(&stubChangeRequestRepo{}, stubUserRepo{}) // repo panics if reached
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "a@example.com"))
	title := "t"
	group := scopeTestProjectID
	var none *string
	groupPtr := &group
	for name, tc := range map[string]struct {
		req  domain.PatchChangeRequestRequest
		want string
	}{
		"customerGroupId":      {domain.PatchChangeRequestRequest{Title: &title, CustomerGroupID: &groupPtr}, "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"},
		"customerGroupId null": {domain.PatchChangeRequestRequest{CustomerGroupID: &none}, "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"},
		"environmentIds":       {domain.PatchChangeRequestRequest{Title: &title, EnvironmentIDs: &[]string{scopeTestDeploymentID}}, "environmentIds is no longer supported: deployments carry the environment"},
	} {
		_, err := svc.PatchChangeRequest(ctx, testUUID, tc.req)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || ve.Msg != tc.want {
			t.Fatalf("%s: err = %v, want ValidationError %q", name, err, tc.want)
		}
	}
}

// TestChangeRequestService_GetChangeRequest_PassesTheViewerAnswerThrough: the
// detail's customerCanAnswer is the repository's, per viewer -- the service adds,
// drops and recomputes nothing, and keeps absent, false and true apart.
func TestChangeRequestService_GetChangeRequest_PassesTheViewerAnswerThrough(t *testing.T) {
	yes, no := true, false
	for name, want := range map[string]*bool{"absent": nil, "false": &no, "true": &yes} {
		repo := &stubChangeRequestRepo{
			getChangeRequestByID: func(_ context.Context, id string) (domain.ChangeRequest, error) {
				if id != testUUID {
					t.Errorf("repo got id %q, want %q", id, testUUID)
				}
				return domain.ChangeRequest{CustomerCanAnswer: want}, nil
			},
		}
		got, err := NewChangeRequestService(repo, stubUserRepo{}).GetChangeRequest(context.Background(), testUUID)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if (got.CustomerCanAnswer == nil) != (want == nil) || (want != nil && *got.CustomerCanAnswer != *want) {
			t.Errorf("%s: customerCanAnswer = %v, want %v", name, got.CustomerCanAnswer, want)
		}
	}

	_, err := NewChangeRequestService(&stubChangeRequestRepo{}, stubUserRepo{}).GetChangeRequest(context.Background(), "not-a-uuid")
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for an invalid id, got %T: %v", err, err)
	}
}
