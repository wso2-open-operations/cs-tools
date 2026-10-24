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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubProjectUpdateRepo is a minimal repository.ProjectRepository whose
// UpdateProject records the call and returns a canned result -- used to
// exercise pgProjectUpdateService without a real Postgres connection.
type stubProjectUpdateRepo struct {
	updateProject func(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error)

	called    bool
	gotID     string
	gotReq    domain.ProjectUpdateRequest
	gotByWhom string
}

func (r *stubProjectUpdateRepo) SearchProjects(context.Context, domain.SearchProjectsRequest, repository.SearchScope) ([]domain.Project, int, error) {
	panic("SearchProjects: not exercised by these tests")
}

func (r *stubProjectUpdateRepo) GetProjectByID(context.Context, string, repository.SearchScope) (domain.ProjectDetailsView, error) {
	panic("GetProjectByID: not exercised by these tests")
}

func (r *stubProjectUpdateRepo) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error) {
	r.called = true
	r.gotID = id
	r.gotReq = req
	r.gotByWhom = updatedBy
	if r.updateProject != nil {
		return r.updateProject(ctx, id, req, updatedBy)
	}
	return domain.ProjectUpdateResult{ID: id, UpdatedBy: updatedBy, UpdatedOn: time.Now()}, nil
}

func (r *stubProjectUpdateRepo) GetProjectConsumptionStatus(context.Context, string) (domain.ProjectConsumptionStatus, error) {
	panic("GetProjectConsumptionStatus: not exercised by these tests")
}

func (r *stubProjectUpdateRepo) UpdateProjectConsumption(context.Context, string, domain.ProjectConsumptionUpdateRequest, string) error {
	panic("UpdateProjectConsumption: not exercised by these tests")
}

// stubProjectMirror is a ProjectUpdateService whose UpdateProject records the
// call and returns either a canned error or a canned response -- used as the
// dual-write mode's ServiceNow mirror, standing in for the real
// snProjectUpdateService so these tests never make a live ServiceNow call.
type stubProjectMirror struct {
	err error

	calls  int
	gotID  string
	gotReq domain.ProjectUpdateRequest
}

func (m *stubProjectMirror) UpdateProject(_ context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	m.calls++
	m.gotID = id
	m.gotReq = req
	if m.err != nil {
		return domain.ProjectUpdateResponse{}, m.err
	}
	return domain.ProjectUpdateResponse{Message: "mirrored"}, nil
}

func testProjectUserRepo() stubUserRepo {
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "u-1", Email: email}, nil
		},
	}
}

// TestPgProjectUpdateService_StoresSuspensionProcessState pins that ACP's state reaches the
// repository unchanged, alone or alongside a sub-state.
func TestPgProjectUpdateService_StoresSuspensionProcessState(t *testing.T) {
	open := "Open"
	state := []byte(`{"based_on_due_invoices":{"event_type":"open"}}`)
	for name, req := range map[string]domain.ProjectUpdateRequest{
		"alone":            {SuspensionProcessState: state},
		"with a sub-state": {SuspensionProcessState: state, EndDateClosureState: &open},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &stubProjectUpdateRepo{}
			svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

			if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", req); err != nil {
				t.Fatalf("UpdateProject() error = %v, want success", err)
			}
			if !repo.called || string(repo.gotReq.SuspensionProcessState) != string(state) {
				t.Fatalf("repo called = %v with suspensionProcessState %s, want %s", repo.called, repo.gotReq.SuspensionProcessState, state)
			}
		})
	}
}

// TestPgProjectUpdateService_CallerResolution pins decision 4: an allow-listed internal
// client may PATCH without a user token; any other tokenless caller is refused.
func TestPgProjectUpdateService_CallerResolution(t *testing.T) {
	open := "Open"
	m2mCtx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "csm-integration"})
	cases := []struct {
		name    string
		ctx     context.Context
		access  AccessService
		wantBy  string
		wantErr bool
	}{
		{"internal client without token", m2mCtx, stubAccess{scope: AccessScope{Unrestricted: true}}, "csm-integration", false},
		// ProjectIDs deliberately includes the project this test actually
		// requests, so this case reaches (and still exercises) the
		// identity-resolution check this test is named for, rather than
		// being rejected earlier by authorizeProject's own project-scope
		// check for an unrelated reason.
		{"restricted caller without token", m2mCtx, stubAccess{scope: AccessScope{ProjectIDs: []string{"11111111-1111-1111-1111-111111111111"}}}, "", true},
		{"no access service wired", m2mCtx, nil, "", true},
		// access IS now consulted here too -- authorizeProject runs for every
		// caller, token or not (that's the whole point of the IDOR fix this
		// pins). What this case still proves: resolveUpdatedBy itself
		// resolves identity from the user token, not from access -- gotBy
		// comes back as the token's own email regardless of what access's
		// scope contains, as long as it's broad enough to authorize the
		// project at all.
		{"user token unchanged", contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), stubAccess{scope: AccessScope{Unrestricted: true}}, "jane.doe@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubProjectUpdateRepo{}
			svc := NewProjectUpdateService(repo, testProjectUserRepo(), tc.access)
			_, err := svc.UpdateProject(tc.ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{EndDateClosureState: &open})
			if tc.wantErr {
				var unauth *apierror.UnauthorizedError
				if !errors.As(err, &unauth) || repo.called {
					t.Fatalf("err = %v, repo.called = %v; want UnauthorizedError and no write", err, repo.called)
				}
				return
			}
			if err != nil || repo.gotByWhom != tc.wantBy {
				t.Fatalf("err = %v, updatedBy = %q; want nil, %q", err, repo.gotByWhom, tc.wantBy)
			}
		})
	}
}

// TestPgProjectUpdateService_RejectsCallerOutsideProjectScope is the
// regression guard for a real IDOR caught in review: granting a
// user-token-authenticated caller (customer_admin, specifically) the
// projects:update permission meant they could update ANY project's
// settings just by knowing its UUID, since nothing checked whether the
// caller actually belonged to that project -- a write has no WHERE-clause
// scope predicate the way a scoped list/by-id read does, so this has to be
// checked explicitly (see authorizeProject's own doc comment). A caller
// restricted to a different project must be refused as NotFoundError
// (never Forbidden, matching every other by-id authorization check in this
// codebase -- a 403 would confirm the project exists to someone not
// entitled to know that), and the repository must never be reached.
func TestPgProjectUpdateService_RejectsCallerOutsideProjectScope(t *testing.T) {
	open := "Open"
	repo := &stubProjectUpdateRepo{}
	access := stubAccess{scope: AccessScope{ProjectIDs: []string{"22222222-2222-2222-2222-222222222222"}}}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), access)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{EndDateClosureState: &open})

	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("UpdateProject() error = %v, want *apierror.NotFoundError", err)
	}
	if repo.called {
		t.Fatal("UpdateProject() reached the repository for a project outside the caller's scope")
	}
}

// TestPgProjectUpdateService_RequiresAtLeastOneField mirrors the
// ServiceNow-mode contract's own "at least one field must be provided" rule.
func TestPgProjectUpdateService_RequiresAtLeastOneField(t *testing.T) {
	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{})

	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("UpdateProject() error = %v, want *apierror.ValidationError", err)
	}
}

// TestPgProjectUpdateService_PlainPostgresUpdatesFieldsAndNeverCallsSN proves
// a plain DATA_SOURCE=postgres pgProjectUpdateService (constructed via
// NewProjectUpdateService, with no dispatcher/mirror at all) applies the
// requested fields through the repository and returns the repository's
// result untouched -- and, since there is no snWriteback/snMirror wiring in
// this mode, there is no way for it to reach ServiceNow.
func TestPgProjectUpdateService_PlainPostgresUpdatesFieldsAndNeverCallsSN(t *testing.T) {
	endDateState := "NOTIFIED"
	hasAgent := true
	wantResult := domain.ProjectUpdateResult{
		ID:                  "11111111-1111-1111-1111-111111111111",
		UpdatedBy:           "jane.doe@example.com",
		UpdatedOn:           time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
		EndDateClosureState: &endDateState,
	}
	repo := &stubProjectUpdateRepo{
		updateProject: func(_ context.Context, _ string, _ domain.ProjectUpdateRequest, _ string) (domain.ProjectUpdateResult, error) {
			return wantResult, nil
		},
	}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.UpdateProject(ctx, wantResult.ID, domain.ProjectUpdateRequest{
		HasAgent:            &hasAgent,
		EndDateClosureState: &endDateState,
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if !repo.called {
		t.Fatal("UpdateProject() never called the repository")
	}
	if repo.gotByWhom != "jane.doe@example.com" {
		t.Fatalf("repo.UpdateProject() updatedBy = %q, want the caller's own resolved email", repo.gotByWhom)
	}
	if repo.gotReq.HasAgent == nil || !*repo.gotReq.HasAgent {
		t.Fatalf("repo.UpdateProject() req.HasAgent = %v, want true", repo.gotReq.HasAgent)
	}
	if resp.Project.ID != wantResult.ID || resp.Project.UpdatedBy != wantResult.UpdatedBy ||
		!resp.Project.UpdatedOn.Equal(wantResult.UpdatedOn) ||
		resp.Project.EndDateClosureState == nil || *resp.Project.EndDateClosureState != *wantResult.EndDateClosureState {
		t.Fatalf("UpdateProject() response.Project = %+v, want the repository's own result untouched: %+v", resp.Project, wantResult)
	}
}

// TestPgProjectUpdateService_AcceptsVerificationEnabledOnly proves a request
// setting only VerificationEnabled (the field the Pending Verification
// feature's Settings toggle sends) is not rejected by the "at least one
// field must be provided" guard, and reaches the repository and back
// unchanged -- locking in the fix made when this field was folded into the
// shared pgProjectUpdateService alongside HasAgent/HasKbReferences/the
// closure-state fields.
func TestPgProjectUpdateService_AcceptsVerificationEnabledOnly(t *testing.T) {
	verificationEnabled := true
	wantResult := domain.ProjectUpdateResult{
		ID:                  "11111111-1111-1111-1111-111111111111",
		UpdatedBy:           "jane.doe@example.com",
		UpdatedOn:           time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
		VerificationEnabled: &verificationEnabled,
	}
	repo := &stubProjectUpdateRepo{
		updateProject: func(_ context.Context, _ string, _ domain.ProjectUpdateRequest, _ string) (domain.ProjectUpdateResult, error) {
			return wantResult, nil
		},
	}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.UpdateProject(ctx, wantResult.ID, domain.ProjectUpdateRequest{
		VerificationEnabled: &verificationEnabled,
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if !repo.called {
		t.Fatal("UpdateProject() never called the repository")
	}
	if repo.gotReq.VerificationEnabled == nil || !*repo.gotReq.VerificationEnabled {
		t.Fatalf("repo.UpdateProject() req.VerificationEnabled = %v, want true", repo.gotReq.VerificationEnabled)
	}
	if resp.Project.VerificationEnabled == nil || !*resp.Project.VerificationEnabled {
		t.Fatalf("UpdateProject() response.Project.VerificationEnabled = %v, want true", resp.Project.VerificationEnabled)
	}
}

// TestPgProjectUpdateService_AcceptsClosureStateOverride pins that a request
// naming only ClosureState (the admin-only direct override -- fine-grained
// authorization for it lives in the BFF, not here) is recognized as having
// something to store and reaches the repository, the same as any other
// stored field.
func TestPgProjectUpdateService_AcceptsClosureStateOverride(t *testing.T) {
	closureState := "SUSPENDED"
	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{ClosureState: &closureState}); err != nil {
		t.Fatalf("UpdateProject() error = %v, want success", err)
	}
	if !repo.called || repo.gotReq.ClosureState == nil || *repo.gotReq.ClosureState != closureState {
		t.Fatalf("repo called = %v with closureState %v, want %q", repo.called, repo.gotReq.ClosureState, closureState)
	}
}

// TestPgProjectUpdateService_ClosureStateOnlyNeverDispatchesSNMirror pins
// hasSNMirrorableProjectFields's whole reason for existing: ClosureState has
// no ServiceNow equivalent (it's a Postgres-only direct override), so a
// dual-write request naming only it must not dispatch a mirror write at all
// -- one would only ever fail against snProjectUpdateService's own "at least
// one field must be provided" check, since none of the fields it knows about
// would be set.
func TestPgProjectUpdateService_ClosureStateOnlyNeverDispatchesSNMirror(t *testing.T) {
	closureState := "READ_ONLY"
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{ClosureState: &closureState}); err != nil {
		t.Fatalf("UpdateProject() error = %v, want success", err)
	}
	if !repo.called {
		t.Fatal("UpdateProject() never called the repository")
	}
	// No background goroutine to race against here (mirror dispatch never
	// happens), so a short, direct check is enough -- unlike the "dispatches
	// exactly one" test below, which has to waitFor an async failure record.
	if mirror.calls != 0 {
		t.Fatalf("ServiceNow mirror UpdateProject called %d times, want 0 for a ClosureState-only request", mirror.calls)
	}
}

// TestPgProjectUpdateService_VerificationEnabledOnlyNeverDispatchesSNMirror
// pins the identical reasoning as ClosureStateOnlyNeverDispatchesSNMirror,
// for the other Postgres-only field this request carries: without this,
// a dual-write request naming only VerificationEnabled would dispatch a
// mirror write that snProjectUpdateService doesn't recognize, recording a
// spurious sn_writeback_failures entry for a field that was never meant to
// reach ServiceNow.
func TestPgProjectUpdateService_VerificationEnabledOnlyNeverDispatchesSNMirror(t *testing.T) {
	verificationEnabled := true
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{VerificationEnabled: &verificationEnabled}); err != nil {
		t.Fatalf("UpdateProject() error = %v, want success", err)
	}
	if !repo.called {
		t.Fatal("UpdateProject() never called the repository")
	}
	// No background goroutine to race against here (mirror dispatch never
	// happens), so a short, direct check is enough -- same reasoning as the
	// ClosureState sibling test above.
	if mirror.calls != 0 {
		t.Fatalf("ServiceNow mirror UpdateProject called %d times, want 0 for a VerificationEnabled-only request", mirror.calls)
	}
}

// TestPgProjectUpdateService_DualWriteDispatchesExactlyOneMirrorCallOnSuccess
// proves DATA_SOURCE=postgres-servicenow-dual-write mode dispatches exactly
// one best-effort ServiceNow mirror write after a successful Postgres write,
// and that a mirror failure neither fails the caller's response nor retries
// -- it lands in sn_writeback_failures via SNWritebackDispatcher.run, which
// this test doesn't need to reach directly since SNWritebackDispatcher's own
// test already covers that recording behavior; this test only needs to prove
// pgProjectUpdateService dispatches the mirror call and tolerates it failing.
func TestPgProjectUpdateService_DualWriteDispatchesExactlyOneMirrorCallOnSuccess(t *testing.T) {
	endDateState := "NOTIFIED"
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{err: errors.New("sn unreachable")}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	id := "11111111-1111-1111-1111-111111111111"
	resp, err := svc.UpdateProject(ctx, id, domain.ProjectUpdateRequest{EndDateClosureState: &endDateState})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v -- a failed/slow ServiceNow mirror must never fail the Postgres-backed response", err)
	}
	if resp.Message == "" {
		t.Fatal("UpdateProject() returned an empty response despite a successful Postgres write")
	}

	// The mirror is configured to fail (mirror.err set above) -- proves a
	// mirror failure both doesn't retry (exactly one call) and lands in
	// sn_writeback_failures via SNWritebackDispatcher.run.
	waitFor(t, func() bool { return failures.count() == 1 })
	if mirror.calls != 1 {
		t.Fatalf("ServiceNow mirror UpdateProject called %d times, want exactly 1", mirror.calls)
	}
	if mirror.gotID != id {
		t.Fatalf("ServiceNow mirror UpdateProject id = %q, want %q", mirror.gotID, id)
	}
}

// TestPgProjectUpdateService_MirrorDropsOnboardingFieldsFromMixedRequest pins
// that a request mixing a mirrorable field with the Postgres-only onboarding
// fields still reaches the mirror, carrying only the mirrorable field.
func TestPgProjectUpdateService_MirrorDropsOnboardingFieldsFromMixedRequest(t *testing.T) {
	hasAgent := true
	status := "In-Progress"
	owner := "00000000-0000-0000-0000-000000000001"
	start, planned, golive, expiry := "2026-03-01", "2026-05-01", "2026-05-15", "2027-03-01"
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.ProjectUpdateRequest{
		HasAgent: &hasAgent, OnboardingStatus: &status, OnboardingOwner: &owner,
		OnboardingStartDate: &start, OnboardingGoLivePlanDate: &planned,
		OnboardingGoLiveDate: &golive, OnboardingExpiryDate: &expiry,
	}
	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", req); err != nil {
		t.Fatalf("UpdateProject() error = %v, want success", err)
	}
	if repo.gotReq.OnboardingStartDate == nil || repo.gotReq.OnboardingStatus == nil {
		t.Fatalf("repo must still receive the onboarding fields, got %+v", repo.gotReq)
	}
	time.Sleep(50 * time.Millisecond)
	if mirror.calls != 1 {
		t.Fatalf("mirror calls = %d, want 1", mirror.calls)
	}
	got := mirror.gotReq
	if got.HasAgent == nil || !*got.HasAgent {
		t.Fatalf("mirror request lost hasAgent: %+v", got)
	}
	if got.OnboardingStatus != nil || got.OnboardingOwner != nil || hasOnboardingDates(got) {
		t.Fatalf("mirror request still carries onboarding fields: %+v", got)
	}
}

// TestPgProjectUpdateService_AcceptsOnboardingFields pins that an onboarding-only
// request counts as a stored field, reaches the repository, and is never
// dispatched to the dual-write mirror.
func TestPgProjectUpdateService_AcceptsOnboardingFields(t *testing.T) {
	status := "In-Progress"
	owner := "00000000-0000-0000-0000-000000000001"
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}, dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.ProjectUpdateRequest{OnboardingStatus: &status, OnboardingOwner: &owner}
	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", req); err != nil {
		t.Fatalf("UpdateProject() error = %v, want success", err)
	}
	if !repo.called || repo.gotReq.OnboardingStatus == nil || *repo.gotReq.OnboardingStatus != status ||
		repo.gotReq.OnboardingOwner == nil || *repo.gotReq.OnboardingOwner != owner {
		t.Fatalf("repo called = %v with %+v, want onboarding fields passed through", repo.called, repo.gotReq)
	}
	time.Sleep(50 * time.Millisecond)
	if mirror.calls != 0 {
		t.Fatalf("mirror calls = %d, want 0 for an onboarding-only request", mirror.calls)
	}
}

// TestPgProjectUpdateService_RejectsMalformedOnboardingOwner pins that a
// non-UUID owner is a 400 from the service, before the repository is called.
func TestPgProjectUpdateService_RejectsMalformedOnboardingOwner(t *testing.T) {
	owner := "not-a-uuid"
	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{OnboardingOwner: &owner})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("UpdateProject() error = %v, want *apierror.ValidationError", err)
	}
	if repo.called {
		t.Fatal("repository must not be called for a malformed onboardingOwner")
	}
}

// TestSNProjectUpdateService_RejectsOnboardingFields pins that the ServiceNow
// data source rejects the Postgres-only onboarding fields with a 400 before
// any upstream call (the nil client would panic if one were attempted).
func TestSNProjectUpdateService_RejectsOnboardingFields(t *testing.T) {
	status := "Completed"
	owner := "00000000-0000-0000-0000-000000000001"
	hasAgent := true
	date := "2026-03-01"
	svc := &snProjectUpdateService{}
	for name, req := range map[string]domain.ProjectUpdateRequest{
		"status":            {OnboardingStatus: &status},
		"owner":             {OnboardingOwner: &owner},
		"start date":        {OnboardingStartDate: &date},
		"go-live plan date": {OnboardingGoLivePlanDate: &date},
		"go-live date":      {OnboardingGoLiveDate: &date},
		"expiry date":       {OnboardingExpiryDate: &date},
		"with other fields": {HasAgent: &hasAgent, OnboardingStatus: &status},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.UpdateProject(context.Background(), "11111111-1111-1111-1111-111111111111", req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
		})
	}
}

// TestPgProjectUpdateService_OnboardingDates pins that date-only requests count
// as stored fields and reach the repository without a mirror dispatch, and that
// a malformed date or a go-live before the start is a 400 before the repository.
func TestPgProjectUpdateService_OnboardingDates(t *testing.T) {
	str := func(s string) *string { return &s }
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	const id = "11111111-1111-1111-1111-111111111111"

	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	req := domain.ProjectUpdateRequest{OnboardingStartDate: str("2026-03-01"), OnboardingGoLiveDate: str("2026-04-01")}
	if _, err := svc.UpdateProject(ctx, id, req); err != nil || !repo.called {
		t.Fatalf("valid dates: err = %v, repo called = %v", err, repo.called)
	}

	for name, bad := range map[string]domain.ProjectUpdateRequest{
		"malformed":      {OnboardingStartDate: str("01/03/2026")},
		"impossible":     {OnboardingExpiryDate: str("2026-02-30")},
		"timestamp":      {OnboardingGoLivePlanDate: str("2026-03-01T00:00:00Z")},
		"plan < start":   {OnboardingStartDate: str("2026-03-01"), OnboardingGoLivePlanDate: str("2026-02-28")},
		"expiry < start": {OnboardingStartDate: str("2026-03-01"), OnboardingExpiryDate: str("2026-01-01")},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &stubProjectUpdateRepo{}
			svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
			_, err := svc.UpdateProject(ctx, id, bad)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if repo.called {
				t.Fatal("repository must not be called")
			}
		})
	}
}
