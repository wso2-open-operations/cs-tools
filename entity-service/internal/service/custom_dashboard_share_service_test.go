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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testShareDashboardID = "5e636a12-0000-4000-8000-000000000001"
	testRecipientID      = "5e636a12-0000-4000-8000-0000000000aa"
)

type fakeShareRepo struct {
	addCalled, removeCalled bool
	gotOwner, gotUser       string
	addOwned                bool
	removeFound             bool
	shares                  []domain.CustomDashboardShare
	sharedWithMe            []domain.SharedCustomDashboardSummary
	sharedOne               *domain.SharedCustomDashboard
	err                     error
}

func (f *fakeShareRepo) ListShares(_ context.Context, ownerID, _ string) ([]domain.CustomDashboardShare, error) {
	f.gotOwner = ownerID
	return f.shares, f.err
}
func (f *fakeShareRepo) AddShare(_ context.Context, ownerID, _, userID string) (bool, error) {
	f.addCalled, f.gotOwner, f.gotUser = true, ownerID, userID
	return f.addOwned, f.err
}
func (f *fakeShareRepo) RemoveShare(_ context.Context, ownerID, _, userID string) (bool, error) {
	f.removeCalled, f.gotOwner, f.gotUser = true, ownerID, userID
	return f.removeFound, f.err
}
func (f *fakeShareRepo) ListSharedWithUser(_ context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error) {
	f.gotUser = userID
	return f.sharedWithMe, f.err
}
func (f *fakeShareRepo) GetSharedWithUser(_ context.Context, userID, _ string) (*domain.SharedCustomDashboard, error) {
	f.gotUser = userID
	return f.sharedOne, f.err
}

// shareUsers resolves every email to testPlatformUserID and answers
// GetUsersByIDs from byID.
type shareUsers struct {
	stubUserRepo
	byID map[string]domain.User
}

func (u shareUsers) GetUsersByIDs(_ context.Context, ids []string) ([]domain.User, error) {
	var out []domain.User
	for _, id := range ids {
		if user, ok := u.byID[id]; ok {
			out = append(out, user)
		}
	}
	return out, nil
}

func shareSvc(t *testing.T, shares *fakeShareRepo, owned bool, recipients map[string]domain.User) (CustomDashboardShareService, context.Context) {
	t.Helper()
	dashboards := &fakeCustomDashboardRepo{}
	if owned {
		dashboards.get = &domain.CustomDashboard{ID: testShareDashboardID}
	}
	users := shareUsers{stubUserRepo: savedFilterViewUsers().(stubUserRepo), byID: recipients}
	return NewCustomDashboardShareService(shares, dashboards, users), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func internalRecipient() map[string]domain.User {
	return map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeInternal}}
}

// --- owner side

func TestCustomDashboardShareService_ListShares(t *testing.T) {
	t.Run("not the caller's dashboard", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{}, false, nil)
		_, err := svc.ListShares(ctx, testShareDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("non-UUID id", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{}, true, nil)
		_, err := svc.ListShares(ctx, "abc")
		wantValidationError(t, err)
	})
	t.Run("lists the owner's shares, never null", func(t *testing.T) {
		repo := &fakeShareRepo{}
		svc, ctx := shareSvc(t, repo, true, nil)
		got, err := svc.ListShares(ctx, testShareDashboardID)
		if err != nil || got.Shares == nil || len(got.Shares) != 0 {
			t.Fatalf("ListShares = %#v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.gotOwner, testPlatformUserID)
		}
	})
}

func TestCustomDashboardShareService_AddShare(t *testing.T) {
	t.Run("shares with internal staff and answers the new list", func(t *testing.T) {
		repo := &fakeShareRepo{addOwned: true, shares: []domain.CustomDashboardShare{{User: domain.UserReference{Name: "Alice"}}}}
		svc, ctx := shareSvc(t, repo, true, internalRecipient())
		got, err := svc.AddShare(ctx, testShareDashboardID, testRecipientID)
		if err != nil || len(got.Shares) != 1 {
			t.Fatalf("AddShare = %+v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotUser != testRecipientID {
			t.Errorf("AddShare(owner %q, user %q)", repo.gotOwner, repo.gotUser)
		}
	})

	refused := []struct {
		name       string
		userID     string
		owned      bool
		recipients map[string]domain.User
		want       string // "validation" or "notfound"
	}{
		{"non-UUID user id", "abc", true, internalRecipient(), "validation"},
		{"not the caller's dashboard", testRecipientID, false, internalRecipient(), "notfound"},
		{"yourself", testPlatformUserID, true, map[string]domain.User{testPlatformUserID: {ID: testPlatformUserID, UserType: domain.UserTypeInternal}}, "validation"},
		{"unknown user", testRecipientID, true, nil, "notfound"},
		{"a customer", testRecipientID, true, map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeCustomer}}, "validation"},
		{"an external contact", testRecipientID, true, map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeExternal}}, "validation"},
		{"a system user", testRecipientID, true, map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeSystem}}, "validation"},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			repo := &fakeShareRepo{addOwned: true}
			svc, ctx := shareSvc(t, repo, tc.owned, tc.recipients)
			_, err := svc.AddShare(ctx, testShareDashboardID, tc.userID)
			if tc.want == "validation" {
				wantValidationError(t, err)
			} else {
				wantNotFoundError(t, err)
			}
			if repo.addCalled {
				t.Error("the share was written")
			}
		})
	}

	t.Run("dashboard gone before the insert", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{addOwned: false}, true, internalRecipient())
		_, err := svc.AddShare(ctx, testShareDashboardID, testRecipientID)
		wantNotFoundError(t, err)
	})
}

func TestCustomDashboardShareService_RemoveShare(t *testing.T) {
	t.Run("removes the owner's share", func(t *testing.T) {
		repo := &fakeShareRepo{removeFound: true}
		svc, ctx := shareSvc(t, repo, true, nil)
		if err := svc.RemoveShare(ctx, testShareDashboardID, testRecipientID); err != nil {
			t.Fatalf("RemoveShare: %v", err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotUser != testRecipientID {
			t.Errorf("RemoveShare(owner %q, user %q)", repo.gotOwner, repo.gotUser)
		}
	})
	t.Run("not shared with them", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{removeFound: false}, true, nil)
		wantNotFoundError(t, svc.RemoveShare(ctx, testShareDashboardID, testRecipientID))
	})
	t.Run("not the caller's dashboard", func(t *testing.T) {
		repo := &fakeShareRepo{removeFound: true}
		svc, ctx := shareSvc(t, repo, false, nil)
		wantNotFoundError(t, svc.RemoveShare(ctx, testShareDashboardID, testRecipientID))
		if repo.removeCalled {
			t.Error("the share was removed")
		}
	})
	t.Run("non-UUID user id", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{}, true, nil)
		wantValidationError(t, svc.RemoveShare(ctx, testShareDashboardID, "abc"))
	})
}

// --- recipient side

func TestCustomDashboardShareService_SharedWithMe(t *testing.T) {
	t.Run("lists what is shared with the caller, never null", func(t *testing.T) {
		repo := &fakeShareRepo{}
		svc, ctx := shareSvc(t, repo, false, nil)
		got, err := svc.ListSharedWithMe(ctx)
		if err != nil || got.Dashboards == nil || len(got.Dashboards) != 0 {
			t.Fatalf("ListSharedWithMe = %#v, %v", got, err)
		}
		if repo.gotUser != testPlatformUserID {
			t.Errorf("user = %q, want the caller", repo.gotUser)
		}
	})
	t.Run("gets one shared with the caller", func(t *testing.T) {
		repo := &fakeShareRepo{sharedOne: &domain.SharedCustomDashboard{ID: testShareDashboardID, Name: "SLA"}}
		svc, ctx := shareSvc(t, repo, false, nil)
		got, err := svc.GetSharedWithMe(ctx, testShareDashboardID)
		if err != nil || got.Name != "SLA" || repo.gotUser != testPlatformUserID {
			t.Fatalf("GetSharedWithMe = %+v, %v (user %q)", got, err, repo.gotUser)
		}
	})
	t.Run("not shared with the caller", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{}, false, nil)
		_, err := svc.GetSharedWithMe(ctx, testShareDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("non-UUID id", func(t *testing.T) {
		svc, ctx := shareSvc(t, &fakeShareRepo{}, false, nil)
		_, err := svc.GetSharedWithMe(ctx, "abc")
		wantValidationError(t, err)
	})
}

func TestCustomDashboardShareService_RequiresToken(t *testing.T) {
	svc := NewCustomDashboardShareService(&fakeShareRepo{}, &fakeCustomDashboardRepo{}, shareUsers{})
	_, err := svc.ListSharedWithMe(context.Background())
	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("got %T %v, want UnauthorizedError", err, err)
	}
}

// The id check accepts an upper-case UUID, while the caller's own id comes
// from the database in lower case; PostgreSQL would store both as the same
// UUID, so the upper-case copy of the caller's own id must be refused too.
func TestCustomDashboardShareService_AddShare_RefusesYourselfInAnyCase(t *testing.T) {
	const ownerID = "5e636a12-0000-4000-8000-0000000000bb"
	upper := strings.ToUpper(ownerID)
	users := shareUsers{
		stubUserRepo: stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: ownerID, Email: email}, nil
		}},
		// Were the check case-sensitive, this would let the share through.
		byID: map[string]domain.User{upper: {ID: upper, UserType: domain.UserTypeInternal}},
	}
	repo := &fakeShareRepo{addOwned: true}
	svc := NewCustomDashboardShareService(repo, &fakeCustomDashboardRepo{get: &domain.CustomDashboard{ID: testShareDashboardID}}, users)
	_, err := svc.AddShare(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), testShareDashboardID, upper)
	wantValidationError(t, err)
	if repo.addCalled {
		t.Error("the caller's own upper-case id was written as a share")
	}
}
