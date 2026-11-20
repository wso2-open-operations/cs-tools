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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeCaseEscalationAccessRepo struct {
	grants   []domain.CaseDeescalatorGrant
	internal bool
	gone     bool
	granted  []string
	revoked  []string
	listed   bool
}

func (f *fakeCaseEscalationAccessRepo) ListDeescalators(context.Context) ([]domain.CaseDeescalatorGrant, error) {
	f.listed = true
	return f.grants, nil
}

func (f *fakeCaseEscalationAccessRepo) GrantDeescalator(_ context.Context, userID, _ string) (bool, error) {
	f.granted = append(f.granted, userID)
	return true, nil
}

func (f *fakeCaseEscalationAccessRepo) RevokeDeescalator(_ context.Context, userID, _ string) (bool, error) {
	f.revoked = append(f.revoked, userID)
	return true, nil
}

func (f *fakeCaseEscalationAccessRepo) UserIsInternal(context.Context, string) (bool, error) {
	return f.internal, nil
}

func (f *fakeCaseEscalationAccessRepo) IsDeactivated(context.Context, string) (bool, error) {
	return f.gone, nil
}

const deescalatorUserID = "6d300000-0000-4000-8000-000000000001"

const caseEscalationTestClientID = "csm-portal-backend-client"

func newCaseEscalationAccess(repo *fakeCaseEscalationAccessRepo) CaseEscalationAccessService {
	return NewCaseEscalationAccessService(repo, alwaysUnrestrictedAccess{}, caseEscalationTestClientID)
}

// viaCSMPortal is what an admin's call looks like by the time it reaches this
// service: csm-portal-backend's own client id plus the forwarded person's email.
func viaCSMPortal(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, ClientID: caseEscalationTestClientID})
}

// The list is read and changed only through csm-portal-backend, which has
// already run its own admin check. A call from any other client, or with no
// signed-in person, is refused before anything is read or written, whatever
// roles the token carries.
func TestCaseEscalationAccessIsForTheCSMPortalBackendOnly(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true}
	svc := newCaseEscalationAccess(repo)
	for name, ctx := range map[string]context.Context{
		"another client, even with a role": auth.WithIdentity(context.Background(), auth.Identity{
			Validated: true, UserEmail: "admin@example.com", ClientID: "some-other-client", Roles: []string{"cs_admin"},
		}),
		"no client id":      callerWithRoles("admin@example.com", "cs_admin"),
		"no signed-in user": auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: caseEscalationTestClientID}),
	} {
		_, err := svc.ListDeescalators(ctx)
		wantForbidden(t, err, "list: "+name)
		wantForbidden(t, svc.GrantDeescalator(ctx, deescalatorUserID), "grant: "+name)
		wantForbidden(t, svc.RevokeDeescalator(ctx, deescalatorUserID), "revoke: "+name)
	}
	if repo.listed || len(repo.granted) != 0 || len(repo.revoked) != 0 {
		t.Fatalf("a refused caller reached the repository: %+v", repo)
	}
}

// The portal backend's caller needs no role in this service's own token: the
// admin decision was made upstream.
func TestCaseEscalationAccessTrustsTheCSMPortalBackend(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true}
	svc := newCaseEscalationAccess(repo)
	ctx := viaCSMPortal("admin@example.com")
	if _, err := svc.ListDeescalators(ctx); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := svc.GrantDeescalator(ctx, deescalatorUserID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := svc.RevokeDeescalator(ctx, deescalatorUserID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(repo.granted) != 1 || len(repo.revoked) != 1 {
		t.Fatalf("granted=%v revoked=%v, want one each", repo.granted, repo.revoked)
	}
}

// With no CSM_PORTAL_BACKEND_CLIENT_ID configured nobody is let through.
func TestCaseEscalationAccessRefusesEveryoneWithoutAConfiguredClientID(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true}
	svc := NewCaseEscalationAccessService(repo, alwaysUnrestrictedAccess{}, "")
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@example.com"})
	_, err := svc.ListDeescalators(ctx)
	wantForbidden(t, err, "list with no configured client id")
}

// A deactivated person is refused even through the portal backend.
func TestCaseEscalationAccessRefusesADeactivatedPerson(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true, gone: true}
	svc := newCaseEscalationAccess(repo)
	ctx := viaCSMPortal("admin@example.com")
	wantForbidden(t, svc.GrantDeescalator(ctx, deescalatorUserID), "grant by a deactivated person")
	_, err := svc.ListDeescalators(ctx)
	wantForbidden(t, err, "list by a deactivated person")
	if len(repo.granted) != 0 || repo.listed {
		t.Fatalf("a deactivated person reached the repository: %+v", repo)
	}
}

// Somebody who is not active internal staff cannot be granted: the grant would
// be stored and do nothing.
func TestCaseEscalationAccessRefusesSomebodyNotInternal(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: false}
	svc := newCaseEscalationAccess(repo)
	wantInvalid(t, svc.GrantDeescalator(viaCSMPortal("admin@example.com"), deescalatorUserID), "grant to a non-internal user")
	wantInvalid(t, svc.GrantDeescalator(callerWithRoles("admin@example.com", "cs_admin"), "not-a-uuid"), "grant with a bad id")
	if len(repo.granted) != 0 {
		t.Fatalf("granted %v, want nothing", repo.granted)
	}
}

type fakeDeescalatorReader struct{ grants []domain.CaseDeescalatorGrant }

func (f fakeDeescalatorReader) ListDeescalators(context.Context) ([]domain.CaseDeescalatorGrant, error) {
	return f.grants, nil
}

// A case's escalation history carries the named de-escalators, active ones
// only, so the portal can decide who may de-escalate from one response.
func TestSearchCaseEscalations_Deescalators(t *testing.T) {
	svc := WithCaseDeescalators(NewCaseEscalationService(oneEscalationHistory{}, nil), fakeDeescalatorReader{grants: []domain.CaseDeescalatorGrant{
		{UserID: "u-1", Name: "Active Person", Email: "active@example.com", GrantedOn: time.Now(), Active: true},
		{UserID: "u-2", Name: "Gone Person", Email: "gone@example.com", GrantedOn: time.Now(), Active: false},
	}})
	got, err := svc.SearchCaseEscalations(context.Background(), escalationTestCaseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Deescalators) != 1 || got.Deescalators[0].ID != "u-1" {
		t.Errorf("Deescalators = %+v, want only the active one", got.Deescalators)
	}
}

// Without the lookup (no database) Deescalators is an empty array, never null.
func TestSearchCaseEscalations_NoDeescalatorReader(t *testing.T) {
	got, err := NewCaseEscalationService(oneEscalationHistory{}, nil).SearchCaseEscalations(context.Background(), escalationTestCaseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Deescalators == nil || len(got.Deescalators) != 0 {
		t.Errorf("Deescalators = %#v, want an empty, non-nil array", got.Deescalators)
	}
}
