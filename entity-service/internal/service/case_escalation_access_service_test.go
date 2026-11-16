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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeCaseEscalationAccessRepo struct {
	grants   []domain.CaseDeescalatorGrant
	internal bool
	portal   bool
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

func (f *fakeCaseEscalationAccessRepo) IsPortalAdmin(context.Context, string) (bool, error) {
	return f.portal, nil
}

func (f *fakeCaseEscalationAccessRepo) IsDeactivated(context.Context, string) (bool, error) {
	return f.gone, nil
}

const deescalatorUserID = "6d300000-0000-4000-8000-000000000001"

func newCaseEscalationAccess(repo *fakeCaseEscalationAccessRepo) CaseEscalationAccessService {
	return NewCaseEscalationAccessService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
}

// Only a CSM admin (a token role, or the stored admin role) may read or change
// the list; everybody else is refused before anything is read or written.
func TestCaseEscalationAccessIsForAdminsOnly(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true}
	svc := newCaseEscalationAccess(repo)
	engineer := callerWithRoles("engineer@example.com", "cs_engineer")

	_, err := svc.ListDeescalators(engineer)
	wantForbidden(t, err, "list by a non-admin")
	wantForbidden(t, svc.GrantDeescalator(engineer, deescalatorUserID), "grant by a non-admin")
	wantForbidden(t, svc.RevokeDeescalator(engineer, deescalatorUserID), "revoke by a non-admin")
	if repo.listed || len(repo.granted) != 0 || len(repo.revoked) != 0 {
		t.Fatalf("a non-admin reached the repository: %+v", repo)
	}

	admin := callerWithRoles("admin@example.com", "cs_admin")
	if _, err := svc.ListDeescalators(admin); err != nil {
		t.Fatalf("list by an admin: %v", err)
	}
	if err := svc.GrantDeescalator(admin, deescalatorUserID); err != nil {
		t.Fatalf("grant by an admin: %v", err)
	}
	if err := svc.RevokeDeescalator(admin, deescalatorUserID); err != nil {
		t.Fatalf("revoke by an admin: %v", err)
	}
	if len(repo.granted) != 1 || len(repo.revoked) != 1 {
		t.Fatalf("granted=%v revoked=%v, want one each", repo.granted, repo.revoked)
	}
}

// A deactivated admin is refused whether the admin comes from a token role or
// the stored admin role.
func TestCaseEscalationAccessRefusesADeactivatedAdmin(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true, portal: true, gone: true}
	svc := newCaseEscalationAccess(repo)
	for name, ctx := range map[string]context.Context{
		"token admin role": callerWithRoles("admin@example.com", "cs_admin"),
		"stored admin":     callerWithRoles("lead@example.com"),
	} {
		wantForbidden(t, svc.GrantDeescalator(ctx, deescalatorUserID), name)
		_, err := svc.ListDeescalators(ctx)
		wantForbidden(t, err, name)
	}
	if len(repo.granted) != 0 || repo.listed {
		t.Fatalf("a deactivated admin reached the repository: %+v", repo)
	}
}

// The stored portal admin role counts as an admin too, as for rota access.
func TestCaseEscalationAccessAcceptsAStoredPortalAdmin(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: true, portal: true}
	svc := newCaseEscalationAccess(repo)
	if err := svc.GrantDeescalator(callerWithRoles("lead@example.com"), deescalatorUserID); err != nil {
		t.Fatalf("grant by a portal admin: %v", err)
	}
}

// Somebody who is not active internal staff cannot be granted: the grant would
// be stored and do nothing.
func TestCaseEscalationAccessRefusesSomebodyNotInternal(t *testing.T) {
	repo := &fakeCaseEscalationAccessRepo{internal: false}
	svc := newCaseEscalationAccess(repo)
	wantInvalid(t, svc.GrantDeescalator(callerWithRoles("admin@example.com", "cs_admin"), deescalatorUserID), "grant to a non-internal user")
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
