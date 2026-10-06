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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const escalationTestCaseID = "11111111-1111-1111-1111-111111111111"

// fakeEscalationRepoForService is a minimal repository.EscalationRepository
// fake for escalationService's request-validation tests -- it never touches
// a real database. It records the last CreateEscalation call (and whether it
// was called at all, which the IDOR-scoping tests assert on directly: an
// out-of-scope case must never reach here) so a test can assert what
// escalationService normalized/forwarded.
type fakeEscalationRepoForService struct {
	called         bool
	lastAction     domain.EscalationAction
	lastReason     *string
	lastActorEmail string
	createErr      error
	createResp     domain.CreatedEscalation
}

func (f *fakeEscalationRepoForService) SearchEscalations(context.Context, []string, []int, string, string, int, int) ([]domain.Escalation, int, error) {
	panic("fakeEscalationRepoForService.SearchEscalations: not expected to be called by these tests")
}

func (f *fakeEscalationRepoForService) CreateEscalation(_ context.Context, _ string, action domain.EscalationAction, reason *string, actorEmail string) (domain.CreatedEscalation, error) {
	f.called = true
	f.lastAction = action
	f.lastReason = reason
	f.lastActorEmail = actorEmail
	if f.createErr != nil {
		return domain.CreatedEscalation{}, f.createErr
	}
	return f.createResp, nil
}

// fakeUserRepoForEscalationService resolves exactly one known email.
type fakeUserRepoForEscalationService struct {
	knownEmail string
	user       domain.User
}

func (f *fakeUserRepoForEscalationService) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	if email == f.knownEmail {
		return f.user, nil
	}
	return domain.User{}, &apierror.NotFoundError{Msg: "no user found with email: " + email}
}
func (f *fakeUserRepoForEscalationService) GetUsersByIDs(context.Context, []string) ([]domain.User, error) {
	panic("fakeUserRepoForEscalationService.GetUsersByIDs: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) SearchUsers(context.Context, domain.SearchUsersRequest) ([]domain.User, int, error) {
	panic("fakeUserRepoForEscalationService.SearchUsers: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) GetUserRoles(context.Context, string) ([]string, error) {
	panic("fakeUserRepoForEscalationService.GetUserRoles: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) GetUserDetail(context.Context, string) (domain.UserDetail, error) {
	panic("fakeUserRepoForEscalationService.GetUserDetail: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) GetUserProjectAccess(context.Context, string) ([]domain.UserContactAccess, error) {
	panic("fakeUserRepoForEscalationService.GetUserProjectAccess: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) GetUserGroups(context.Context, string) ([]domain.UserGroupRef, error) {
	panic("fakeUserRepoForEscalationService.GetUserGroups: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) CreateUser(context.Context, domain.CreateUserRequest, string) (domain.User, error) {
	panic("fakeUserRepoForEscalationService.CreateUser: not expected to be called by these tests")
}
func (f *fakeUserRepoForEscalationService) UpdateUserTimeZone(context.Context, string, string) (time.Time, error) {
	panic("fakeUserRepoForEscalationService.UpdateUserTimeZone: not expected to be called by these tests")
}

// caseFoundInScopeRepo is the default stubCaseRepo.GetCaseByID for tests
// unrelated to authorization: the case is always found and always in scope,
// so those tests aren't coupled to the new access-scoping check (see
// stubCaseRepo/alwaysUnrestrictedAccess, both already defined in
// case_service_test.go and reused here as-is -- same package, same shape
// the IDOR fix's own doc comment says to copy).
func caseFoundInScopeRepo() *stubCaseRepo {
	return &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: escalationTestCaseID}, nil
		},
	}
}

func newTestEscalationService(repo *fakeEscalationRepoForService) (EscalationService, *fakeUserRepoForEscalationService) {
	return newTestEscalationServiceWithCaseAccess(repo, caseFoundInScopeRepo(), alwaysUnrestrictedAccess{})
}

func newTestEscalationServiceWithCaseAccess(repo *fakeEscalationRepoForService, caseRepo repository.CaseRepository, access AccessService) (EscalationService, *fakeUserRepoForEscalationService) {
	userRepo := &fakeUserRepoForEscalationService{
		knownEmail: "engineer@example.com",
		user:       domain.User{ID: "user-engineer", Email: "engineer@example.com"},
	}
	return NewEscalationService(repo, userRepo, caseRepo, access), userRepo
}

func TestEscalationService_CreateEscalation_InvalidCaseID(t *testing.T) {
	svc, _ := newTestEscalationService(&fakeEscalationRepoForService{})
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{CaseID: "not-a-uuid"})
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("got %v (%T), want *apierror.ValidationError", err, err)
	}
}

func TestEscalationService_CreateEscalation_InvalidAction(t *testing.T) {
	svc, _ := newTestEscalationService(&fakeEscalationRepoForService{})
	badAction := domain.EscalationAction("SIDEWAYS")
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Action: &badAction,
	})
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("got %v (%T), want *apierror.ValidationError", err, err)
	}
}

func TestEscalationService_CreateEscalation_EscalateRequiresReason(t *testing.T) {
	svc, _ := newTestEscalationService(&fakeEscalationRepoForService{})
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{CaseID: escalationTestCaseID})
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("got %v (%T), want *apierror.ValidationError (reason required for the defaulted ESCALATE action)", err, err)
	}

	blank := "   "
	_, err = svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{CaseID: escalationTestCaseID, Reason: &blank})
	if !errors.As(err, &valErr) {
		t.Fatalf("blank reason: got %v (%T), want *apierror.ValidationError", err, err)
	}
}

func TestEscalationService_CreateEscalation_DeescalateDoesNotRequireReason(t *testing.T) {
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		PreviousLevel: domain.ChoiceListItem{Label: "2"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	svc, _ := newTestEscalationService(repo)
	deescalate := domain.EscalationActionDeescalate
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Action: &deescalate,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastAction != domain.EscalationActionDeescalate {
		t.Errorf("forwarded action = %q, want DEESCALATE", repo.lastAction)
	}
}

func TestEscalationService_CreateEscalation_NoUserIDTokenIsUnauthorized(t *testing.T) {
	svc, _ := newTestEscalationService(&fakeEscalationRepoForService{})
	reason := "customer escalation"
	_, err := svc.CreateEscalation(contextWithUserIDToken(""), domain.CreateEscalationRequest{CaseID: escalationTestCaseID, Reason: &reason})
	var unauthErr *apierror.UnauthorizedError
	if !errors.As(err, &unauthErr) {
		t.Fatalf("got %v (%T), want *apierror.UnauthorizedError", err, err)
	}
}

func TestEscalationService_CreateEscalation_NormalizesLowercaseActionAndForwardsActorEmail(t *testing.T) {
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	svc, _ := newTestEscalationService(repo)
	lowerEscalate := domain.EscalationAction("escalate")
	reason := "customer requested management involvement"
	resp, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Action: &lowerEscalate,
		Reason: &reason,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastAction != domain.EscalationActionEscalate {
		t.Errorf("forwarded action = %q, want the upper-cased ESCALATE", repo.lastAction)
	}
	if repo.lastActorEmail != "engineer@example.com" {
		t.Errorf("forwarded actor email = %q, want engineer@example.com", repo.lastActorEmail)
	}
	if resp.Message == "" {
		t.Error("expected a non-empty message")
	}
}

// TestEscalationService_CreateEscalation_OutOfScopeCaseIsNotFound is the
// IDOR regression guard: a caller whose AccessScope doesn't cover caseId
// must get exactly what GetCaseByID/SearchCases already give an out-of-scope
// by-id read -- a NotFoundError, indistinguishable from the case not
// existing at all (never a 403 that would confirm it exists) -- and
// repo.CreateEscalation must never be reached at all, not merely fail
// afterward.
func TestEscalationService_CreateEscalation_OutOfScopeCaseIsNotFound(t *testing.T) {
	caseRepo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			// Mirrors exactly what CaseRepository.GetCaseByID itself returns
			// for a case outside the caller's scope (its scopeClause simply
			// matches zero rows) -- see that method's own doc comment.
			return domain.CaseView{}, &apierror.NotFoundError{Msg: "case not found"}
		},
	}
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	svc, _ := newTestEscalationServiceWithCaseAccess(repo, caseRepo, alwaysUnrestrictedAccess{})

	reason := "trying to escalate a case outside my scope"
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Reason: &reason,
	})
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("got %v (%T), want *apierror.NotFoundError", err, err)
	}
	if repo.called {
		t.Error("repo.CreateEscalation must never be called for an out-of-scope case -- the IDOR this check closes")
	}
}

// TestEscalationService_CreateEscalation_InScopeCaseStillWorks is the
// positive-path counterpart: a case the caller's scope DOES cover must not
// be collaterally blocked by the new authorization check.
func TestEscalationService_CreateEscalation_InScopeCaseStillWorks(t *testing.T) {
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	svc, _ := newTestEscalationServiceWithCaseAccess(repo, caseFoundInScopeRepo(), alwaysUnrestrictedAccess{})

	reason := "customer requested management involvement"
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Reason: &reason,
	})
	if err != nil {
		t.Fatalf("unexpected error for an in-scope case: %v", err)
	}
	if !repo.called {
		t.Error("repo.CreateEscalation should have been called for an in-scope case")
	}
}

// stubMirrorEscalationService embeds EscalationService (nil) and overrides
// only CreateEscalation -- same convention as stubMirrorCallRequestService.
type stubMirrorEscalationService struct {
	EscalationService
	createEscalation func(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error)
}

func (s *stubMirrorEscalationService) CreateEscalation(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
	return s.createEscalation(ctx, req)
}

// TestEscalationService_CreateEscalation_MirrorsToServiceNow covers the
// writeback wiring added for fix 2: on a successful Postgres create, the
// mirror's CreateEscalation is dispatched asynchronously, forwarding req
// verbatim, and does not block or affect the response.
func TestEscalationService_CreateEscalation_MirrorsToServiceNow(t *testing.T) {
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		ID:            "escalation-1",
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	userRepo := &fakeUserRepoForEscalationService{
		knownEmail: "engineer@example.com",
		user:       domain.User{ID: "user-engineer", Email: "engineer@example.com"},
	}
	called := make(chan domain.CreateEscalationRequest, 1)
	mirror := &stubMirrorEscalationService{
		createEscalation: func(_ context.Context, mirrorReq domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
			called <- mirrorReq
			return domain.CreateEscalationResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewEscalationServiceWithSNWriteback(repo, userRepo, caseFoundInScopeRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)

	reason := "customer requested management involvement"
	req := domain.CreateEscalationRequest{CaseID: escalationTestCaseID, Reason: &reason}
	if _, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-called:
		if got.CaseID != req.CaseID {
			t.Errorf("mirror got CaseID %q, want %q", got.CaseID, req.CaseID)
		}
		if got.Reason == nil || *got.Reason != reason {
			t.Errorf("mirror got Reason %v, want %q", got.Reason, reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.CreateEscalation was never called")
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestEscalationService_CreateEscalation_MirrorFailureRecordsWritebackFailure
// covers the failure half: Postgres already succeeded, so the call must
// still report success, but the mirror error lands in sn_writeback_failures
// for manual backfill.
func TestEscalationService_CreateEscalation_MirrorFailureRecordsWritebackFailure(t *testing.T) {
	repo := &fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		ID:            "escalation-2",
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}}
	userRepo := &fakeUserRepoForEscalationService{
		knownEmail: "engineer@example.com",
		user:       domain.User{ID: "user-engineer", Email: "engineer@example.com"},
	}
	mirror := &stubMirrorEscalationService{
		createEscalation: func(context.Context, domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
			return domain.CreateEscalationResponse{}, errors.New("sn downstream unreachable")
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewEscalationServiceWithSNWriteback(repo, userRepo, caseFoundInScopeRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)

	reason := "customer requested management involvement"
	req := domain.CreateEscalationRequest{CaseID: escalationTestCaseID, Reason: &reason}
	if _, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), req); err != nil {
		t.Fatalf("expected the Postgres-side success to be reported despite the mirror failure, got %v", err)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
}

// TestEscalationService_CreateEscalation_NoMirrorOnPlainPostgres covers the
// plain-Postgres (non-dual-write) regression guard: with snWriteback/snMirror
// both nil (NewEscalationService, not the SNWriteback constructor),
// CreateEscalation must still succeed and must never touch any mirror.
func TestEscalationService_CreateEscalation_NoMirrorOnPlainPostgres(t *testing.T) {
	svc, _ := newTestEscalationService(&fakeEscalationRepoForService{createResp: domain.CreatedEscalation{
		PreviousLevel: domain.ChoiceListItem{Label: "0"},
		CurrentLevel:  domain.ChoiceListItem{Label: "1"},
	}})
	reason := "customer requested management involvement"
	_, err := svc.CreateEscalation(contextWithUserIDToken(fakeJWTWithEmail(t, "engineer@example.com")), domain.CreateEscalationRequest{
		CaseID: escalationTestCaseID,
		Reason: &reason,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
