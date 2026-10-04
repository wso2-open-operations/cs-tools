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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	wlContactID = "11111111-1111-1111-1111-111111111111"
	wlContact2  = "22222222-2222-2222-2222-222222222222"
	wlStaffID   = "33333333-3333-3333-3333-333333333333"
	wlOtherID   = "44444444-4444-4444-4444-444444444444"
)

func registeredContact(email, id string) repository.ProjectContactRow {
	return repository.ProjectContactRow{Email: email, RegistrationState: "REGISTERED", ResolvedUserID: str(id), ResolvedEmail: str(email)}
}

// wlHarness drives a dual-write CreateCase with a given caller type, project
// contacts and user directory, recording what the upstream mirror and the
// watcher-row write received.
type wlHarness struct {
	callerType    domain.UserType
	callerToken   bool
	callerMissing bool
	contacts      func(offset int) ([]repository.ProjectContactRow, int)
	contactsErr   error
	users         []domain.User
	usersErr      error

	mirrorCalls   int
	mirrorList    []string
	setCalled     bool
	setIDs        []string
	contactsCalls int
	userSearches  int
	lastFilters   domain.SearchUsersFilters
}

func (h *wlHarness) run(t *testing.T, watchList []string) error {
	t.Helper()
	const caseID = "55555555-5555-5555-5555-555555555555"
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			h.mirrorCalls++
			h.mirrorList = req.WatchList
			return domain.CreateCaseResponse{Case: domain.CreateCaseDetails{ID: caseID, InternalID: "X-1", Number: "CS0000001", CreatedBy: "jane.doe@example.com", State: "Open"}}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, _ domain.CreateCaseRequest, id, number, wso2ID, createdBy, _ string) (domain.Case, error) {
			st := domain.CaseStateOpen
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, ProjectID: "proj-1", State: &st}, nil
		},
		setCaseWatchList: func(_ context.Context, _ string, ids []string, _ string) ([]domain.WatchListUser, time.Time, error) {
			h.setCalled = true
			h.setIDs = ids
			return nil, time.Time{}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			sev := domain.CaseSeverityHigh
			return domain.CaseView{ID: caseID, ProjectDetails: &domain.EntityRef{ID: "proj-1"}, Severity: &sev}, nil
		},
	}
	users := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			if h.callerMissing {
				return domain.User{}, &apierror.NotFoundError{Msg: "no user found with that email"}
			}
			return domain.User{ID: "caller", Email: "jane.doe@example.com", UserType: h.callerType}, nil
		},
		searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
			h.userSearches++
			h.lastFilters = req.Filters
			return h.users, len(h.users), h.usersErr
		},
	}
	contacts := &stubProjectContactRepo{
		searchProjectContacts: func(_ context.Context, _ string, req domain.SearchProjectContactsRequest) ([]repository.ProjectContactRow, int, error) {
			h.contactsCalls++
			if h.contactsErr != nil {
				return nil, 0, h.contactsErr
			}
			rows, total := h.contacts(req.Pagination.Offset)
			return rows, total, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, users, &mockEventPublisher{}, alwaysUnrestrictedAccess{}, contacts, dispatcher, mirror, nil, "")

	ctx := contextWithUserIDToken("")
	if h.callerToken {
		ctx = contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	}
	req := validCreateCaseRequest()
	req.WatchList = watchList
	_, err := svc.CreateCase(ctx, req)
	return err
}

func fixedContacts(rows ...repository.ProjectContactRow) func(int) ([]repository.ProjectContactRow, int) {
	return func(int) ([]repository.ProjectContactRow, int) { return rows, len(rows) }
}

func wantRejected(t *testing.T, err error, h *wlHarness) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a ValidationError", err)
	}
	if h.mirrorCalls != 0 || h.setCalled {
		t.Fatalf("a rejected watch list must create nothing: mirrorCalls=%d setCalled=%v", h.mirrorCalls, h.setCalled)
	}
	for _, leak := range []string{"@", wlContactID, wlOtherID, wlStaffID} {
		if strings.Contains(ve.Msg, leak) {
			t.Fatalf("message %q echoes an entry", ve.Msg)
		}
	}
}

func TestCreateWatchList_ExternalCaller_AllContactsAccepted_MirrorGetsEmails_PostgresGetsIDs(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID), registeredContact("john.roe@example.com", wlContact2))}
	// Case-insensitive, and the same contact listed twice by different case.
	if err := h.run(t, []string{"JANE.DOE@example.com", "john.roe@example.com", "Jane.Doe@Example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.mirrorList) != 2 || h.mirrorList[0] != "jane.doe@example.com" || h.mirrorList[1] != "john.roe@example.com" {
		t.Fatalf("mirror watchList = %v, want the 2 contacts' emails", h.mirrorList)
	}
	if len(h.setIDs) != 2 || h.setIDs[0] != wlContactID || h.setIDs[1] != wlContact2 {
		t.Fatalf("watcher rows = %v, want the 2 validated ids", h.setIDs)
	}
	if h.contactsCalls != 1 || h.userSearches != 0 {
		t.Fatalf("contactsCalls=%d userSearches=%d, want 1 and 0", h.contactsCalls, h.userSearches)
	}
}

func TestCreateWatchList_ExternalCaller_NonContactRejected(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID))}
	wantRejected(t, h.run(t, []string{"jane.doe@example.com", "stranger@example.com"}), h)
	if h.userSearches != 0 {
		t.Fatal("an external caller must never trigger the internal-user lookup")
	}
}

func TestCreateWatchList_ExternalCaller_InternalUserRejected(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID)),
		users:    []domain.User{{ID: wlStaffID, Email: "staff@example.com", UserType: domain.UserTypeInternal}}}
	wantRejected(t, h.run(t, []string{wlStaffID}), h)
}

func TestCreateWatchList_InvitedAndDeactivatedContactsRejected(t *testing.T) {
	for _, state := range []string{"INVITED", "RE-INVITED", "DEACTIVATED"} {
		row := registeredContact("jane.doe@example.com", wlContactID)
		row.RegistrationState = state
		h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true, contacts: fixedContacts(row)}
		wantRejected(t, h.run(t, []string{"jane.doe@example.com"}), h)
	}
}

func TestCreateWatchList_InternalCaller_ContactAndInternalUserAccepted(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeInternal, callerToken: true,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID)),
		users:    []domain.User{{ID: wlStaffID, Email: "staff@example.com", UserType: domain.UserTypeInternal}}}
	if err := h.run(t, []string{wlContactID, wlStaffID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.mirrorList) != 2 || h.mirrorList[0] != "jane.doe@example.com" || h.mirrorList[1] != "staff@example.com" {
		t.Fatalf("mirror watchList = %v, want both emails (ids converted)", h.mirrorList)
	}
	if len(h.setIDs) != 2 || h.setIDs[0] != wlContactID || h.setIDs[1] != wlStaffID {
		t.Fatalf("watcher rows = %v", h.setIDs)
	}
	if h.userSearches != 1 || len(h.lastFilters.UserIDs) != 1 {
		t.Fatalf("userSearches=%d filters=%+v, want one batched lookup of the single non-contact", h.userSearches, h.lastFilters)
	}
}

func TestCreateWatchList_InternalCaller_ExternalNonContactRejected(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeInternal, callerToken: true,
		contacts: fixedContacts(),
		users:    []domain.User{{ID: wlOtherID, Email: "other@example.com", UserType: domain.UserTypeCustomer}}}
	wantRejected(t, h.run(t, []string{wlOtherID}), h)
}

func TestCreateWatchList_InternalCaller_EmailsBatchedInOneLookup(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeInternal, callerToken: true, contacts: fixedContacts(),
		users: []domain.User{
			{ID: wlStaffID, Email: "staff@example.com", UserType: domain.UserTypeInternal},
			{ID: wlContact2, Email: "staff2@example.com", UserType: domain.UserTypeInternal},
		}}
	if err := h.run(t, []string{"Staff@example.com", "staff2@example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.userSearches != 1 || len(h.lastFilters.Emails) != 2 {
		t.Fatalf("userSearches=%d filters=%+v, want one lookup of 2 emails", h.userSearches, h.lastFilters)
	}
}

func TestCreateWatchList_NoToken_RejectedBeforeAnyCreate(t *testing.T) {
	h := &wlHarness{callerToken: false, contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID))}
	err := h.run(t, []string{"jane.doe@example.com"})
	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) || h.mirrorCalls != 0 {
		t.Fatalf("err=%v mirrorCalls=%d, want Unauthorized and no create", err, h.mirrorCalls)
	}
}

func TestCreateWatchList_CallerWithoutPlatformRecordGetsExternalRule(t *testing.T) {
	h := &wlHarness{callerMissing: true, callerToken: true, contacts: fixedContacts(),
		users: []domain.User{{ID: wlStaffID, Email: "staff@example.com", UserType: domain.UserTypeInternal}}}
	wantRejected(t, h.run(t, []string{wlStaffID}), h)
}

func TestCreateWatchList_ContactsPaged(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true}
	h.contacts = func(offset int) ([]repository.ProjectContactRow, int) {
		total := watchListContactPageSize + 1
		if offset == 0 {
			rows := make([]repository.ProjectContactRow, watchListContactPageSize)
			for i := range rows {
				rows[i] = repository.ProjectContactRow{Email: "filler@example.com", RegistrationState: "REGISTERED"}
			}
			return rows, total
		}
		return []repository.ProjectContactRow{registeredContact("jane.doe@example.com", wlContactID)}, total
	}
	if err := h.run(t, []string{"jane.doe@example.com"}); err != nil {
		t.Fatalf("a contact on the second page must be found: %v", err)
	}
	if h.contactsCalls != 2 {
		t.Fatalf("contact pages = %d, want 2", h.contactsCalls)
	}
}

func TestCreateWatchList_LookupFailuresFailTheCreate(t *testing.T) {
	boom := errors.New("db unavailable")
	h := &wlHarness{callerType: domain.UserTypeCustomer, callerToken: true, contactsErr: boom}
	if err := h.run(t, []string{"jane.doe@example.com"}); !errors.Is(err, boom) || h.mirrorCalls != 0 {
		t.Fatalf("contacts failure: err=%v mirrorCalls=%d", err, h.mirrorCalls)
	}
	h = &wlHarness{callerType: domain.UserTypeInternal, callerToken: true, contacts: fixedContacts(), usersErr: boom}
	if err := h.run(t, []string{wlStaffID}); !errors.Is(err, boom) || h.mirrorCalls != 0 {
		t.Fatalf("user lookup failure: err=%v mirrorCalls=%d", err, h.mirrorCalls)
	}
}

func TestCreateWatchList_MixedListStillAValidationError(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeInternal, callerToken: true, contacts: fixedContacts()}
	wantRejected(t, h.run(t, []string{"jane.doe@example.com", wlStaffID}), h)
	wantRejected(t, h.run(t, []string{"not-an-email-or-id"}), h)
}

func TestCreateWatchList_EmptyListUntouched(t *testing.T) {
	// No token, no contacts stub: nothing may be looked up or required.
	h := &wlHarness{contactsErr: errors.New("must not be called")}
	if err := h.run(t, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.contactsCalls != 0 || h.userSearches != 0 || h.setCalled || h.mirrorCalls != 1 {
		t.Fatalf("contactsCalls=%d userSearches=%d setCalled=%v mirrorCalls=%d", h.contactsCalls, h.userSearches, h.setCalled, h.mirrorCalls)
	}
}

// pgOnlyRun drives CreateCase on the pure-Postgres path (no upstream mirror).
func (h *wlHarness) pgOnlyRun(t *testing.T, watchList []string) error {
	t.Helper()
	const caseID = "66666666-6666-6666-6666-666666666666"
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, _ domain.CreateCaseRequest) (domain.Case, error) {
			h.mirrorCalls++ // counts repo creates on this path
			st := domain.CaseStateOpen
			return domain.Case{ID: caseID, Number: "CS0000002", State: &st}, nil
		},
		setCaseWatchList: func(_ context.Context, _ string, ids []string, _ string) ([]domain.WatchListUser, time.Time, error) {
			h.setCalled = true
			h.setIDs = ids
			return nil, time.Time{}, nil
		},
	}
	users := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{ID: "caller", Email: "jane.doe@example.com", UserType: h.callerType}, nil
		},
		searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
			h.userSearches++
			return h.users, len(h.users), h.usersErr
		},
	}
	contacts := &stubProjectContactRepo{
		searchProjectContacts: func(_ context.Context, _ string, req domain.SearchProjectContactsRequest) ([]repository.ProjectContactRow, int, error) {
			h.contactsCalls++
			rows, total := h.contacts(req.Pagination.Offset)
			return rows, total, nil
		},
	}
	svc := NewCaseService(repo, users, nil, alwaysUnrestrictedAccess{}, contacts)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	req := validCreateCaseRequest()
	req.WatchList = watchList
	_, err := svc.CreateCase(ctx, req)
	return err
}

func TestCreateWatchList_PostgresOnly_ValidatedAndPersisted(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID), registeredContact("john.roe@example.com", wlContact2))}
	if err := h.pgOnlyRun(t, []string{"john.roe@example.com", "Jane.Doe@example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.mirrorCalls != 1 || !h.setCalled || len(h.setIDs) != 2 || h.setIDs[0] != wlContact2 || h.setIDs[1] != wlContactID {
		t.Fatalf("creates=%d setCalled=%v ids=%v, want 1 create and the 2 validated ids in order", h.mirrorCalls, h.setCalled, h.setIDs)
	}
}

func TestCreateWatchList_PostgresOnly_IneligibleWatcherCreatesNothing(t *testing.T) {
	h := &wlHarness{callerType: domain.UserTypeCustomer,
		contacts: fixedContacts(registeredContact("jane.doe@example.com", wlContactID))}
	err := h.pgOnlyRun(t, []string{"jane.doe@example.com", "stranger@example.com"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || h.mirrorCalls != 0 || h.setCalled {
		t.Fatalf("err=%v creates=%d setCalled=%v, want a ValidationError and nothing written", err, h.mirrorCalls, h.setCalled)
	}
}

func TestCreateWatchList_PostgresOnly_EmptyListUnchanged(t *testing.T) {
	h := &wlHarness{contactsErr: errors.New("must not be called")}
	if err := h.pgOnlyRun(t, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.mirrorCalls != 1 || h.setCalled || h.contactsCalls != 0 {
		t.Fatalf("creates=%d setCalled=%v contactsCalls=%d", h.mirrorCalls, h.setCalled, h.contactsCalls)
	}
}
