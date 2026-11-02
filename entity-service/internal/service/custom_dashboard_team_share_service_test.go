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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testShareTeamID = "5e636a12-0000-4000-8000-0000000000bb"

type fakeTeamShareRepo struct {
	addCalled, removeCalled bool
	gotOwner, gotTeam       string
	addOwned                bool
	removeFound             bool
	shares                  []domain.CustomDashboardTeamShare
	err                     error
}

func (f *fakeTeamShareRepo) ListTeamShares(_ context.Context, ownerID, _ string) ([]domain.CustomDashboardTeamShare, error) {
	f.gotOwner = ownerID
	return f.shares, f.err
}
func (f *fakeTeamShareRepo) AddTeamShare(_ context.Context, ownerID, _, teamID string) (bool, error) {
	f.addCalled, f.gotOwner, f.gotTeam = true, ownerID, teamID
	return f.addOwned, f.err
}
func (f *fakeTeamShareRepo) RemoveTeamShare(_ context.Context, ownerID, _, teamID string) (bool, error) {
	f.removeCalled, f.gotOwner, f.gotTeam = true, ownerID, teamID
	return f.removeFound, f.err
}

func teamShareSvc(t *testing.T, shares *fakeTeamShareRepo, owned bool) (CustomDashboardTeamShareService, context.Context) {
	t.Helper()
	dashboards := &fakeCustomDashboardRepo{}
	if owned {
		dashboards.get = &domain.CustomDashboard{ID: testShareDashboardID}
	}
	users := shareUsers{stubUserRepo: savedFilterViewUsers().(stubUserRepo)}
	return NewCustomDashboardTeamShareService(shares, dashboards, users), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func TestCustomDashboardTeamShareService_ListTeamShares(t *testing.T) {
	t.Run("not the caller's dashboard", func(t *testing.T) {
		svc, ctx := teamShareSvc(t, &fakeTeamShareRepo{}, false)
		_, err := svc.ListTeamShares(ctx, testShareDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("lists the owner's team shares, never null", func(t *testing.T) {
		repo := &fakeTeamShareRepo{}
		svc, ctx := teamShareSvc(t, repo, true)
		got, err := svc.ListTeamShares(ctx, testShareDashboardID)
		if err != nil || got.Shares == nil || len(got.Shares) != 0 {
			t.Fatalf("ListTeamShares = %#v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.gotOwner, testPlatformUserID)
		}
	})
}

func TestCustomDashboardTeamShareService_AddTeamShare(t *testing.T) {
	t.Run("shares with the team and answers the new list", func(t *testing.T) {
		repo := &fakeTeamShareRepo{addOwned: true, shares: []domain.CustomDashboardTeamShare{{Team: domain.CustomDashboardTeamShareTeam{Name: "Rigel"}}}}
		svc, ctx := teamShareSvc(t, repo, true)
		got, err := svc.AddTeamShare(ctx, testShareDashboardID, testShareTeamID)
		if err != nil || len(got.Shares) != 1 || got.Shares[0].Team.Name != "Rigel" {
			t.Fatalf("AddTeamShare = %+v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotTeam != testShareTeamID {
			t.Errorf("AddTeamShare(owner %q, team %q)", repo.gotOwner, repo.gotTeam)
		}
	})
	t.Run("refuses a non-UUID team id", func(t *testing.T) {
		repo := &fakeTeamShareRepo{addOwned: true}
		svc, ctx := teamShareSvc(t, repo, true)
		_, err := svc.AddTeamShare(ctx, testShareDashboardID, "rigel")
		wantValidationError(t, err)
		if repo.addCalled {
			t.Error("the share was written")
		}
	})
	t.Run("refuses another user's dashboard", func(t *testing.T) {
		repo := &fakeTeamShareRepo{addOwned: true}
		svc, ctx := teamShareSvc(t, repo, false)
		_, err := svc.AddTeamShare(ctx, testShareDashboardID, testShareTeamID)
		wantNotFoundError(t, err)
		if repo.addCalled {
			t.Error("the share was written")
		}
	})
	t.Run("dashboard gone before the insert", func(t *testing.T) {
		svc, ctx := teamShareSvc(t, &fakeTeamShareRepo{addOwned: false}, true)
		_, err := svc.AddTeamShare(ctx, testShareDashboardID, testShareTeamID)
		wantNotFoundError(t, err)
	})
}

func TestCustomDashboardTeamShareService_RemoveTeamShare(t *testing.T) {
	t.Run("removes the owner's team share", func(t *testing.T) {
		repo := &fakeTeamShareRepo{removeFound: true}
		svc, ctx := teamShareSvc(t, repo, true)
		if err := svc.RemoveTeamShare(ctx, testShareDashboardID, testShareTeamID); err != nil {
			t.Fatalf("RemoveTeamShare: %v", err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotTeam != testShareTeamID {
			t.Errorf("RemoveTeamShare(owner %q, team %q)", repo.gotOwner, repo.gotTeam)
		}
	})
	t.Run("not shared with the team", func(t *testing.T) {
		svc, ctx := teamShareSvc(t, &fakeTeamShareRepo{removeFound: false}, true)
		wantNotFoundError(t, svc.RemoveTeamShare(ctx, testShareDashboardID, testShareTeamID))
	})
	t.Run("not the caller's dashboard", func(t *testing.T) {
		repo := &fakeTeamShareRepo{removeFound: true}
		svc, ctx := teamShareSvc(t, repo, false)
		wantNotFoundError(t, svc.RemoveTeamShare(ctx, testShareDashboardID, testShareTeamID))
		if repo.removeCalled {
			t.Error("the share was removed")
		}
	})
	t.Run("non-UUID team id", func(t *testing.T) {
		svc, ctx := teamShareSvc(t, &fakeTeamShareRepo{}, true)
		wantValidationError(t, svc.RemoveTeamShare(ctx, testShareDashboardID, "abc"))
	})
}
