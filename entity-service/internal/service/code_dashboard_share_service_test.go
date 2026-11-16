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

// fakeCodeShareRepo records the owner or user each call was made for, and
// whether a write was made.
type fakeCodeShareRepo struct {
	written            bool
	gotOwner, gotOther string

	owned, removed bool
	shares         []domain.CustomDashboardShare
	teamShares     []domain.CustomDashboardTeamShare
	link           *domain.CustomDashboardLink
	disableFound   bool
	openID         string
	openOwn        bool
	sharedWithMe   []domain.SharedCustomDashboardSummary
	sharedOne      *domain.SharedCodeDashboard
	err            error
}

func (f *fakeCodeShareRepo) ListShares(_ context.Context, ownerID, _ string) ([]domain.CustomDashboardShare, error) {
	f.gotOwner = ownerID
	return f.shares, f.err
}
func (f *fakeCodeShareRepo) AddShare(_ context.Context, ownerID, _, userID string) (bool, error) {
	f.written, f.gotOwner, f.gotOther = true, ownerID, userID
	return f.owned, f.err
}
func (f *fakeCodeShareRepo) RemoveShare(_ context.Context, ownerID, _, userID string) (bool, error) {
	f.written, f.gotOwner, f.gotOther = true, ownerID, userID
	return f.removed, f.err
}
func (f *fakeCodeShareRepo) ListTeamShares(_ context.Context, ownerID, _ string) ([]domain.CustomDashboardTeamShare, error) {
	f.gotOwner = ownerID
	return f.teamShares, f.err
}
func (f *fakeCodeShareRepo) AddTeamShare(_ context.Context, ownerID, _, teamID string) (bool, error) {
	f.written, f.gotOwner, f.gotOther = true, ownerID, teamID
	return f.owned, f.err
}
func (f *fakeCodeShareRepo) RemoveTeamShare(_ context.Context, ownerID, _, teamID string) (bool, error) {
	f.written, f.gotOwner, f.gotOther = true, ownerID, teamID
	return f.removed, f.err
}
func (f *fakeCodeShareRepo) GetLink(_ context.Context, ownerID, _ string) (*domain.CustomDashboardLink, error) {
	f.gotOwner = ownerID
	return f.link, f.err
}
func (f *fakeCodeShareRepo) EnableLink(_ context.Context, ownerID, _ string) (*domain.CustomDashboardLink, error) {
	f.written, f.gotOwner = true, ownerID
	return f.link, f.err
}
func (f *fakeCodeShareRepo) DisableLink(_ context.Context, ownerID, _ string) (bool, error) {
	f.written, f.gotOwner = true, ownerID
	return f.disableFound, f.err
}
func (f *fakeCodeShareRepo) OpenLink(_ context.Context, userID, linkID string) (string, bool, error) {
	f.written, f.gotOwner, f.gotOther = true, userID, linkID
	return f.openID, f.openOwn, f.err
}
func (f *fakeCodeShareRepo) ListSharedWithUser(_ context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error) {
	f.gotOther = userID
	return f.sharedWithMe, f.err
}
func (f *fakeCodeShareRepo) GetSharedWithUser(_ context.Context, userID, _ string) (*domain.SharedCodeDashboard, error) {
	f.gotOther = userID
	return f.sharedOne, f.err
}

const testCodeShareTeamID = "5e636a12-0000-4000-8000-0000000000dd"

// codeShareSvc is the service as the caller (testPlatformUserID); owned says
// whether the caller owns testShareDashboardID.
func codeShareSvc(t *testing.T, repo *fakeCodeShareRepo, owned bool, recipients map[string]domain.User) (CodeDashboardShareService, context.Context) {
	t.Helper()
	dashboards := &fakeCodeDashboardRepo{}
	if owned {
		dashboards.get = &domain.CodeDashboard{ID: testShareDashboardID}
	}
	users := shareUsers{stubUserRepo: savedFilterViewUsers().(stubUserRepo), byID: recipients}
	return NewCodeDashboardShareService(repo, dashboards, users), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func TestCodeDashboardShareService_AddShare(t *testing.T) {
	t.Run("shares with internal staff and answers the new list", func(t *testing.T) {
		repo := &fakeCodeShareRepo{owned: true, shares: []domain.CustomDashboardShare{{User: domain.UserReference{Name: "Alice"}}}}
		svc, ctx := codeShareSvc(t, repo, true, internalRecipient())
		got, err := svc.AddShare(ctx, testShareDashboardID, testRecipientID)
		if err != nil || len(got.Shares) != 1 {
			t.Fatalf("AddShare = %+v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotOther != testRecipientID {
			t.Errorf("AddShare(owner %q, user %q)", repo.gotOwner, repo.gotOther)
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
		{"unknown user", testRecipientID, true, nil, "notfound"},
		{"a customer", testRecipientID, true, map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeCustomer}}, "validation"},
		{"an external contact", testRecipientID, true, map[string]domain.User{testRecipientID: {ID: testRecipientID, UserType: domain.UserTypeExternal}}, "validation"},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			repo := &fakeCodeShareRepo{owned: true}
			svc, ctx := codeShareSvc(t, repo, tc.owned, tc.recipients)
			_, err := svc.AddShare(ctx, testShareDashboardID, tc.userID)
			if tc.want == "validation" {
				wantValidationError(t, err)
			} else {
				wantNotFoundError(t, err)
			}
			if repo.written {
				t.Error("the share was written")
			}
		})
	}
	t.Run("dashboard gone before the insert", func(t *testing.T) {
		svc, ctx := codeShareSvc(t, &fakeCodeShareRepo{}, true, internalRecipient())
		_, err := svc.AddShare(ctx, testShareDashboardID, testRecipientID)
		wantNotFoundError(t, err)
	})
}

func TestCodeDashboardShareService_ListAndRemoveShares(t *testing.T) {
	svc, ctx := codeShareSvc(t, &fakeCodeShareRepo{}, true, nil)
	if got, err := svc.ListShares(ctx, testShareDashboardID); err != nil || got.Shares == nil {
		t.Errorf("ListShares = %+v, %v; want an empty list, not null", got, err)
	}
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{}, false, nil)
	_, err := svc.ListShares(ctx, testShareDashboardID)
	wantNotFoundError(t, err)

	repo := &fakeCodeShareRepo{removed: true}
	svc, ctx = codeShareSvc(t, repo, true, nil)
	if err := svc.RemoveShare(ctx, testShareDashboardID, testRecipientID); err != nil || repo.gotOwner != testPlatformUserID {
		t.Errorf("RemoveShare = %v as %q", err, repo.gotOwner)
	}
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{}, true, nil)
	wantNotFoundError(t, svc.RemoveShare(ctx, testShareDashboardID, testRecipientID))
	repo = &fakeCodeShareRepo{removed: true}
	svc, ctx = codeShareSvc(t, repo, false, nil)
	wantNotFoundError(t, svc.RemoveShare(ctx, testShareDashboardID, testRecipientID))
	wantValidationError(t, svc.RemoveShare(ctx, testShareDashboardID, "abc"))
	if repo.written {
		t.Error("a share of a dashboard that is not the caller's was removed")
	}
}

func TestCodeDashboardShareService_TeamShares(t *testing.T) {
	repo := &fakeCodeShareRepo{owned: true, teamShares: []domain.CustomDashboardTeamShare{{Team: domain.CustomDashboardTeamShareTeam{ID: testCodeShareTeamID}}}}
	svc, ctx := codeShareSvc(t, repo, true, nil)
	if got, err := svc.AddTeamShare(ctx, testShareDashboardID, testCodeShareTeamID); err != nil || len(got.Shares) != 1 || repo.gotOther != testCodeShareTeamID {
		t.Fatalf("AddTeamShare = %+v, %v (team %q)", got, err, repo.gotOther)
	}

	repo = &fakeCodeShareRepo{owned: true}
	svc, ctx = codeShareSvc(t, repo, false, nil)
	_, err := svc.AddTeamShare(ctx, testShareDashboardID, testCodeShareTeamID)
	wantNotFoundError(t, err)
	_, err = svc.AddTeamShare(ctx, testShareDashboardID, "abc")
	wantValidationError(t, err)
	if repo.written {
		t.Error("a team share was written for a dashboard that is not the caller's")
	}
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{}, true, nil)
	_, err = svc.AddTeamShare(ctx, testShareDashboardID, testCodeShareTeamID)
	wantNotFoundError(t, err) // gone before the insert
	if got, err := svc.ListTeamShares(ctx, testShareDashboardID); err != nil || got.Shares == nil {
		t.Errorf("ListTeamShares = %+v, %v; want an empty list, not null", got, err)
	}
	wantNotFoundError(t, svc.RemoveTeamShare(ctx, testShareDashboardID, testCodeShareTeamID))
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{removed: true}, true, nil)
	if err := svc.RemoveTeamShare(ctx, testShareDashboardID, testCodeShareTeamID); err != nil {
		t.Errorf("RemoveTeamShare = %v", err)
	}
}

func TestCodeDashboardShareService_Link(t *testing.T) {
	on := &domain.CustomDashboardLink{Enabled: true, LinkID: testLinkID}
	repo := &fakeCodeShareRepo{link: on, disableFound: true}
	svc, ctx := codeShareSvc(t, repo, true, nil)
	if got, err := svc.GetLink(ctx, testShareDashboardID); err != nil || got.LinkID != testLinkID || repo.gotOwner != testPlatformUserID {
		t.Errorf("GetLink = %+v, %v as %q", got, err, repo.gotOwner)
	}
	if got, err := svc.EnableLink(ctx, testShareDashboardID); err != nil || !got.Enabled {
		t.Errorf("EnableLink = %+v, %v", got, err)
	}
	if err := svc.DisableLink(ctx, testShareDashboardID); err != nil {
		t.Errorf("DisableLink = %v", err)
	}

	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{}, true, nil)
	_, err := svc.GetLink(ctx, testShareDashboardID)
	wantNotFoundError(t, err)
	_, err = svc.EnableLink(ctx, testShareDashboardID)
	wantNotFoundError(t, err)
	wantNotFoundError(t, svc.DisableLink(ctx, testShareDashboardID))

	repo = &fakeCodeShareRepo{link: on, disableFound: true}
	svc, ctx = codeShareSvc(t, repo, true, nil)
	_, err = svc.EnableLink(ctx, "abc")
	wantValidationError(t, err)
	wantValidationError(t, svc.DisableLink(ctx, "abc"))
	if repo.written {
		t.Error("a link was changed for an id that is not a UUID")
	}
}

func TestCodeDashboardShareService_OpenLink(t *testing.T) {
	repo := &fakeCodeShareRepo{openID: testShareDashboardID}
	svc, ctx := codeShareSvc(t, repo, false, nil)
	got, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
	if err != nil || got.DashboardID != testShareDashboardID || got.Own || repo.gotOwner != testPlatformUserID || repo.gotOther != testLinkID {
		t.Fatalf("OpenLink = %+v, %v (user %q, link %q)", got, err, repo.gotOwner, repo.gotOther)
	}
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{openID: testShareDashboardID, openOwn: true}, false, nil)
	if got, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID}); err != nil || !got.Own {
		t.Errorf("own OpenLink = %+v, %v", got, err)
	}
	svc, ctx = codeShareSvc(t, &fakeCodeShareRepo{}, false, nil)
	_, err = svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
	wantNotFoundError(t, err)

	repo = &fakeCodeShareRepo{openID: testShareDashboardID}
	svc, ctx = codeShareSvc(t, repo, false, nil)
	_, err = svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: "not-a-link-secret"})
	wantValidationError(t, err)
	if strings.Contains(err.Error(), "not-a-link-secret") {
		t.Errorf("the error quotes the link id: %v", err)
	}
	if repo.written {
		t.Error("a link that is not a UUID was opened")
	}
}

func TestCodeDashboardShareService_SharedWithMe(t *testing.T) {
	repo := &fakeCodeShareRepo{}
	svc, ctx := codeShareSvc(t, repo, false, nil)
	if got, err := svc.ListSharedWithMe(ctx); err != nil || got.Dashboards == nil || repo.gotOther != testPlatformUserID {
		t.Errorf("ListSharedWithMe = %+v, %v as %q; want an empty list, not null", got, err, repo.gotOther)
	}
	_, err := svc.GetSharedWithMe(ctx, testShareDashboardID)
	wantNotFoundError(t, err)
	_, err = svc.GetSharedWithMe(ctx, "abc")
	wantValidationError(t, err)

	repo = &fakeCodeShareRepo{sharedOne: &domain.SharedCodeDashboard{ID: testShareDashboardID, Source: "function render() {}"}}
	svc, ctx = codeShareSvc(t, repo, false, nil)
	if got, err := svc.GetSharedWithMe(ctx, testShareDashboardID); err != nil || got.Source != "function render() {}" {
		t.Errorf("GetSharedWithMe = %+v, %v", got, err)
	}
	repo = &fakeCodeShareRepo{err: errors.New("db down")}
	svc, ctx = codeShareSvc(t, repo, false, nil)
	if _, err := svc.ListSharedWithMe(ctx); err == nil {
		t.Error("ListSharedWithMe swallowed a repository error")
	}
}

func TestCodeDashboardShareService_RequiresToken(t *testing.T) {
	svc := NewCodeDashboardShareService(&fakeCodeShareRepo{}, &fakeCodeDashboardRepo{}, shareUsers{})
	var ue *apierror.UnauthorizedError
	if _, err := svc.ListSharedWithMe(context.Background()); !errors.As(err, &ue) {
		t.Errorf("ListSharedWithMe: got %T %v, want UnauthorizedError", err, err)
	}
	if _, err := svc.OpenLink(context.Background(), domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID}); !errors.As(err, &ue) {
		t.Errorf("OpenLink: got %T %v, want UnauthorizedError", err, err)
	}
	if _, err := svc.AddShare(context.Background(), testShareDashboardID, testRecipientID); !errors.As(err, &ue) {
		t.Errorf("AddShare: got %T %v, want UnauthorizedError", err, err)
	}
}

// The caller's own id, in either case, is refused. The caller here has a UUID
// id (testPlatformUserID is not one, so the UUID check would refuse it first);
// the id check accepts an upper-case UUID while the database answers in lower
// case, and PostgreSQL would store both as the same UUID.
func TestCodeDashboardShareService_AddShare_RefusesYourselfInAnyCase(t *testing.T) {
	const ownerID = "5e636a12-0000-4000-8000-0000000000bb"
	for _, self := range []string{ownerID, strings.ToUpper(ownerID)} {
		users := shareUsers{
			stubUserRepo: stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
				return domain.User{ID: ownerID, Email: email}, nil
			}},
			// Were the check missing or case-sensitive, this would let the share through.
			byID: map[string]domain.User{self: {ID: self, UserType: domain.UserTypeInternal}},
		}
		repo := &fakeCodeShareRepo{owned: true}
		svc := NewCodeDashboardShareService(repo, &fakeCodeDashboardRepo{get: &domain.CodeDashboard{ID: testShareDashboardID}}, users)
		_, err := svc.AddShare(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), testShareDashboardID, self)
		wantValidationError(t, err)
		if repo.written {
			t.Errorf("shared with the caller's own id %q", self)
		}
	}
}
