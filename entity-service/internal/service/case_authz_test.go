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

const authzCaseID = "22222222-2222-2222-2222-222222222222"

// customerScopeAccess is an AccessService whose caller is a registered
// customer contact of one project: scoped, never Unrestricted.
var customerScopeAccess = stubAccess{scope: AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: "customer@example.com"}}

func requireForbidden(t *testing.T, name string, err error) {
	t.Helper()
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("%s: got %T (%v), want *apierror.ForbiddenError", name, err, err)
	}
}

func customerUserRepo() stubUserRepo {
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "user-c", Email: email, UserType: domain.UserTypeCustomer}, nil
		},
	}
}

// TestCaseService_UpdateCase_InternalOnlyFieldsRefusedForCustomerScope proves
// every engineer-side field is refused before any branch runs when the
// caller's scope is not Unrestricted: the repository stub has no update
// functions wired, so reaching one would panic.
func TestCaseService_UpdateCase_InternalOnlyFieldsRefusedForCustomerScope(t *testing.T) {
	str := func(s string) *string { return &s }
	boolean := func(b bool) *bool { return &b }
	state := func(s domain.CaseState) *domain.CaseState { return &s }
	sev := domain.CaseSeverityHigh
	ws := domain.CaseWorkStateOngoing

	cases := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{"assigneeEmail", domain.UpdateCaseRequest{ID: authzCaseID, AssigneeEmail: []byte(`"eng@example.com"`)}},
		{"acknowledge", domain.UpdateCaseRequest{ID: authzCaseID, Acknowledge: boolean(true)}},
		{"parentId", domain.UpdateCaseRequest{ID: authzCaseID, ParentID: str(testDeploymentUUID)}},
		{"markFixIssued", domain.UpdateCaseRequest{ID: authzCaseID, MarkFixIssued: boolean(true)}},
		{"severity", domain.UpdateCaseRequest{ID: authzCaseID, Severity: &sev}},
		{"workState", domain.UpdateCaseRequest{ID: authzCaseID, WorkState: &ws}},
		{"engineer-side state", domain.UpdateCaseRequest{ID: authzCaseID, State: state(domain.CaseStateWorkInProgress)}},
		{"fix eta", domain.UpdateCaseRequest{ID: authzCaseID, BestCaseFixEta: str("2026-10-10")}},
		{"relatedCaseId", domain.UpdateCaseRequest{ID: authzCaseID, RelatedCaseID: str(testDeploymentUUID)}},
		{"workaroundProvided", domain.UpdateCaseRequest{ID: authzCaseID, WorkaroundProvided: boolean(true)}},
		{"subject combined with fix eta", domain.UpdateCaseRequest{ID: authzCaseID, Subject: str("s"), WorstCaseFixEta: str("2026-10-10")}},
	}
	svc := NewCaseService(&stubCaseRepo{}, customerUserRepo(), nil, customerScopeAccess, &stubProjectContactRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))
	for _, tc := range cases {
		_, err := svc.UpdateCase(ctx, tc.req)
		requireForbidden(t, tc.name, err)
	}
}

// TestCaseService_UpdateCase_CustomerWritableFieldsReachRepository proves the
// customer-writable side of the split is still open to a scoped caller: a
// close with resolution data, a hand-back transition, and the descriptive
// fields all reach the repository.
func TestCaseService_UpdateCase_CustomerWritableFieldsReachRepository(t *testing.T) {
	str := func(s string) *string { return &s }
	state := func(s domain.CaseState) *domain.CaseState { return &s }
	code := domain.CaseResolutionCodeSolvedByCustomer
	cause := domain.CaseCauseUserErrorConfiguration

	var updated, fieldsUpdated int
	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			updated++
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
		updateCaseFields: func(context.Context, domain.UpdateCaseRequest, string, string) (time.Time, error) {
			fieldsUpdated++
			return time.Now(), nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			open := domain.CaseStateOpen
			return domain.CaseView{ID: authzCaseID, State: &open, ProjectDetails: &domain.EntityRef{ID: "proj-1"}}, nil
		},
	}
	svc := NewCaseService(repo, customerUserRepo(), nil, customerScopeAccess, &stubProjectContactRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))

	reqs := []domain.UpdateCaseRequest{
		{ID: authzCaseID, State: state(domain.CaseStateClosed), ResolutionCode: &code, Cause: &cause, CloseNotes: str("done")},
		{ID: authzCaseID, State: state(domain.CaseStateWaitingOnWSO2)},
		{ID: authzCaseID, State: state(domain.CaseStateReopened)},
	}
	for _, req := range reqs {
		if _, err := svc.UpdateCase(ctx, req); err != nil {
			t.Fatalf("state %s: unexpected error: %v", *req.State, err)
		}
	}
	if _, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: authzCaseID, Subject: str("new subject"), Description: str("more detail")}); err != nil {
		t.Fatalf("descriptive fields: unexpected error: %v", err)
	}
	if updated != 3 || fieldsUpdated != 1 {
		t.Fatalf("repository calls: state updates=%d (want 3), field updates=%d (want 1)", updated, fieldsUpdated)
	}
}

// TestCaseService_UpdateCase_StateBranchRequiresKnownActorWhenTokenPresent
// proves the state branch no longer takes the actor best-effort: a user
// token that does not resolve to a platform user fails the update before the
// repository is reached.
func TestCaseService_UpdateCase_StateBranchRequiresKnownActorWhenTokenPresent(t *testing.T) {
	state := domain.CaseStateClosed
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{}, &apierror.NotFoundError{Msg: "user not found"}
	}}
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: authzCaseID}, nil
		},
	}
	svc := NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "ghost@example.com"))
	_, err := svc.UpdateCase(ctx, domain.UpdateCaseRequest{ID: authzCaseID, State: &state})
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("got %T (%v), want the actor lookup's NotFoundError", err, err)
	}
}

// TestCaseService_UpdateCase_StateBranchAcceptsInternalClientWithoutToken
// proves an allow-listed internal client (Unrestricted on the client
// credential alone, no user token) can still change state -- recorded with
// no user attribution rather than refused.
func TestCaseService_UpdateCase_StateBranchAcceptsInternalClientWithoutToken(t *testing.T) {
	state := domain.CaseStateClosed
	var called bool
	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			called = true
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: authzCaseID}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	if _, err := svc.UpdateCase(contextWithUserIDToken(""), domain.UpdateCaseRequest{ID: authzCaseID, State: &state}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("repository UpdateCase was not called")
	}
}

// TestCaseService_UpdateCase_WatchList_CustomerMayNotAddStaffOutsideProject
// proves a scoped caller can only add watchers who are registered contacts
// on the case's project: an internal user who is not one is refused, while
// an internal caller may still add them.
func TestCaseService_UpdateCase_WatchList_CustomerMayNotAddStaffOutsideProject(t *testing.T) {
	const staffID = "33333333-3333-3333-3333-333333333333"
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: authzCaseID, ProjectDetails: &domain.EntityRef{ID: "proj-1"}}, nil
		},
		setCaseWatchList: func(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			return []domain.WatchListUser{{ID: staffID}}, time.Now(), nil
		},
	}
	contacts := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{}, &apierror.NotFoundError{Msg: "not a contact"}
		},
	}
	userRepo := stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "u", Email: email}, nil
		},
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			return domain.UserDetail{ID: staffID, UserType: domain.UserTypeInternal, Email: "eng@example.com"}, nil
		},
	}
	watch := []string{staffID}

	customer := NewCaseService(repo, userRepo, nil, customerScopeAccess, contacts)
	_, err := customer.UpdateCase(contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com")), domain.UpdateCaseRequest{ID: authzCaseID, WatchList: &watch})
	requireForbidden(t, "customer adds staff", err)

	internal := NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, contacts)
	if _, err := internal.UpdateCase(contextWithUserIDToken(fakeJWTWithEmail(t, "eng@example.com")), domain.UpdateCaseRequest{ID: authzCaseID, WatchList: &watch}); err != nil {
		t.Fatalf("internal caller adds staff: unexpected error: %v", err)
	}
}

// TestCaseService_Tags_RequireInternalCaller proves tag edits are
// engineer-side on both the add and remove paths.
func TestCaseService_Tags_RequireInternalCaller(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, customerUserRepo(), nil, customerScopeAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))
	_, err := svc.AddCaseTag(ctx, authzCaseID, "billable")
	requireForbidden(t, "add tag", err)
	requireForbidden(t, "remove tag", svc.RemoveCaseTag(ctx, authzCaseID, testDeploymentUUID))
}

// TestCaseService_CreateCaseComment_WorkNoteRequiresInternalCaller proves a
// scoped caller may author a public comment but not a work note or an
// activity entry.
func TestCaseService_CreateCaseComment_WorkNoteRequiresInternalCaller(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "c1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: authzCaseID}, nil
		},
	}
	svc := NewCaseService(repo, customerUserRepo(), nil, customerScopeAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))

	for _, typ := range []domain.CommentType{domain.CommentTypeWorkNote, domain.CommentTypeActivity} {
		_, err := svc.CreateCaseComment(ctx, domain.CreateCaseCommentRequest{CaseID: authzCaseID, Type: typ, Content: "x"})
		requireForbidden(t, string(typ), err)
	}
	if _, err := svc.CreateCaseComment(ctx, domain.CreateCaseCommentRequest{CaseID: authzCaseID, Type: domain.CommentTypeComment, Content: "public"}); err != nil {
		t.Fatalf("public comment: unexpected error: %v", err)
	}
}

// TestCaseService_SearchCaseComments_HidesWorkNotesFromCustomerScope proves
// the service-side filter: work notes are dropped from a scoped caller's
// page, and asking for them explicitly is refused, while an Unrestricted
// caller sees the page as the repository returned it.
func TestCaseService_SearchCaseComments_HidesWorkNotesFromCustomerScope(t *testing.T) {
	page := []domain.CaseComment{
		{ID: "1", Type: domain.CommentTypeComment},
		{ID: "2", Type: domain.CommentTypeWorkNote},
		{ID: "3", Type: domain.CommentTypeComment},
	}
	repo := &stubCaseRepo{searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
		return append([]domain.CaseComment(nil), page...), len(page), nil
	}}
	req := domain.SearchCaseCommentsRequest{CaseID: authzCaseID}

	customer := NewCaseService(repo, customerUserRepo(), nil, customerScopeAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "customer@example.com"))
	resp, err := customer.SearchCaseComments(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 2 || resp.Comments[0].ID != "1" || resp.Comments[1].ID != "3" {
		t.Fatalf("customer page = %+v, want the two public comments", resp.Comments)
	}
	workNote := domain.CommentTypeWorkNote
	_, err = customer.SearchCaseComments(ctx, domain.SearchCaseCommentsRequest{CaseID: authzCaseID, Filters: &domain.CommentFilters{Type: &workNote}})
	requireForbidden(t, "explicit work_note filter", err)

	internal := NewCaseService(repo, customerUserRepo(), nil, alwaysUnrestrictedAccess{}, nil)
	resp, err = internal.SearchCaseComments(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 3 {
		t.Fatalf("internal page has %d comments, want 3", len(resp.Comments))
	}
}

func TestWithoutWorkNoteActivity(t *testing.T) {
	wn, c := domain.CommentTypeWorkNote, domain.CommentTypeComment
	in := []domain.CaseActivity{{ID: "a", CommentType: &wn}, {ID: "b", CommentType: &c}, {ID: "d"}}
	out := withoutWorkNoteActivity(in)
	if len(out) != 2 || out[0].ID != "b" || out[1].ID != "d" {
		t.Fatalf("got %+v", out)
	}
}
