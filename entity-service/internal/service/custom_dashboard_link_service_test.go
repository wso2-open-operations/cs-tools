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

const testLinkID = "5e636a12-0000-4000-8000-0000000000cc"

type fakeDashboardLinkRepo struct {
	called                   bool
	gotOwner, gotUser, gotID string
	link                     *domain.CustomDashboardLink
	disableFound             bool
	openDashboardID          string
	openOwn                  bool
	err                      error
}

func (f *fakeDashboardLinkRepo) GetLink(_ context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	f.called, f.gotOwner, f.gotID = true, ownerID, dashboardID
	return f.link, f.err
}
func (f *fakeDashboardLinkRepo) EnableLink(_ context.Context, ownerID, dashboardID string) (*domain.CustomDashboardLink, error) {
	f.called, f.gotOwner, f.gotID = true, ownerID, dashboardID
	return f.link, f.err
}
func (f *fakeDashboardLinkRepo) DisableLink(_ context.Context, ownerID, dashboardID string) (bool, error) {
	f.called, f.gotOwner, f.gotID = true, ownerID, dashboardID
	return f.disableFound, f.err
}
func (f *fakeDashboardLinkRepo) OpenLink(_ context.Context, userID, linkID string) (string, bool, error) {
	f.called, f.gotUser, f.gotID = true, userID, linkID
	return f.openDashboardID, f.openOwn, f.err
}

func linkSvc(t *testing.T, links *fakeDashboardLinkRepo) (CustomDashboardLinkService, context.Context) {
	t.Helper()
	users := shareUsers{stubUserRepo: savedFilterViewUsers().(stubUserRepo)}
	return NewCustomDashboardLinkService(links, users), contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

func TestCustomDashboardLinkService_GetLink(t *testing.T) {
	t.Run("the owner's link", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{link: &domain.CustomDashboardLink{Enabled: true, LinkID: testLinkID, OpenedBy: 2}}
		svc, ctx := linkSvc(t, repo)
		got, err := svc.GetLink(ctx, testShareDashboardID)
		if err != nil || !got.Enabled || got.LinkID != testLinkID || got.OpenedBy != 2 {
			t.Fatalf("GetLink = %+v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotID != testShareDashboardID {
			t.Errorf("GetLink(owner %q, dashboard %q)", repo.gotOwner, repo.gotID)
		}
	})
	t.Run("not the caller's dashboard", func(t *testing.T) {
		svc, ctx := linkSvc(t, &fakeDashboardLinkRepo{})
		_, err := svc.GetLink(ctx, testShareDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("non-UUID id", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{}
		svc, ctx := linkSvc(t, repo)
		_, err := svc.GetLink(ctx, "abc")
		wantValidationError(t, err)
		if repo.called {
			t.Error("the repository was called")
		}
	})
	t.Run("no token", func(t *testing.T) {
		svc, _ := linkSvc(t, &fakeDashboardLinkRepo{})
		_, err := svc.GetLink(context.Background(), testShareDashboardID)
		var unauth *apierror.UnauthorizedError
		if !errors.As(err, &unauth) {
			t.Fatalf("err = %v, want UnauthorizedError", err)
		}
	})
}

func TestCustomDashboardLinkService_EnableLink(t *testing.T) {
	t.Run("turns the owner's link on", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{link: &domain.CustomDashboardLink{Enabled: true, LinkID: testLinkID}}
		svc, ctx := linkSvc(t, repo)
		got, err := svc.EnableLink(ctx, testShareDashboardID)
		if err != nil || !got.Enabled || got.LinkID != testLinkID {
			t.Fatalf("EnableLink = %+v, %v", got, err)
		}
		if repo.gotOwner != testPlatformUserID {
			t.Errorf("owner = %q, want %q", repo.gotOwner, testPlatformUserID)
		}
	})
	t.Run("not the caller's dashboard", func(t *testing.T) {
		svc, ctx := linkSvc(t, &fakeDashboardLinkRepo{})
		_, err := svc.EnableLink(ctx, testShareDashboardID)
		wantNotFoundError(t, err)
	})
	t.Run("non-UUID id", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{}
		svc, ctx := linkSvc(t, repo)
		_, err := svc.EnableLink(ctx, "abc")
		wantValidationError(t, err)
		if repo.called {
			t.Error("the link was turned on")
		}
	})
}

func TestCustomDashboardLinkService_DisableLink(t *testing.T) {
	t.Run("turns the owner's link off", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{disableFound: true}
		svc, ctx := linkSvc(t, repo)
		if err := svc.DisableLink(ctx, testShareDashboardID); err != nil {
			t.Fatalf("DisableLink: %v", err)
		}
		if repo.gotOwner != testPlatformUserID || repo.gotID != testShareDashboardID {
			t.Errorf("DisableLink(owner %q, dashboard %q)", repo.gotOwner, repo.gotID)
		}
	})
	t.Run("not the caller's dashboard", func(t *testing.T) {
		svc, ctx := linkSvc(t, &fakeDashboardLinkRepo{disableFound: false})
		wantNotFoundError(t, svc.DisableLink(ctx, testShareDashboardID))
	})
	t.Run("non-UUID id", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{disableFound: true}
		svc, ctx := linkSvc(t, repo)
		wantValidationError(t, svc.DisableLink(ctx, "abc"))
		if repo.called {
			t.Error("the link was turned off")
		}
	})
}

func TestCustomDashboardLinkService_OpenLink(t *testing.T) {
	t.Run("opens the dashboard as the caller", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{openDashboardID: testShareDashboardID}
		svc, ctx := linkSvc(t, repo)
		got, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
		if err != nil || got.DashboardID != testShareDashboardID || got.Own {
			t.Fatalf("OpenLink = %+v, %v", got, err)
		}
		if repo.gotUser != testPlatformUserID || repo.gotID != testLinkID {
			t.Errorf("OpenLink(user %q, link %q)", repo.gotUser, repo.gotID)
		}
	})
	t.Run("the owner's own dashboard", func(t *testing.T) {
		svc, ctx := linkSvc(t, &fakeDashboardLinkRepo{openDashboardID: testShareDashboardID, openOwn: true})
		got, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
		if err != nil || !got.Own {
			t.Fatalf("OpenLink = %+v, %v", got, err)
		}
	})
	t.Run("unknown or turned-off link", func(t *testing.T) {
		svc, ctx := linkSvc(t, &fakeDashboardLinkRepo{})
		_, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
		wantNotFoundError(t, err)
	})
	t.Run("a link id that is not a UUID is refused without quoting it", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{openDashboardID: testShareDashboardID}
		svc, ctx := linkSvc(t, repo)
		_, err := svc.OpenLink(ctx, domain.OpenCustomDashboardLinkRequest{LinkID: "not-a-link-secret"})
		wantValidationError(t, err)
		if strings.Contains(err.Error(), "not-a-link-secret") {
			t.Errorf("the error quotes the link id: %v", err)
		}
		if repo.called {
			t.Error("the link was opened")
		}
	})
	t.Run("no token", func(t *testing.T) {
		repo := &fakeDashboardLinkRepo{openDashboardID: testShareDashboardID}
		svc, _ := linkSvc(t, repo)
		_, err := svc.OpenLink(context.Background(), domain.OpenCustomDashboardLinkRequest{LinkID: testLinkID})
		var unauth *apierror.UnauthorizedError
		if !errors.As(err, &unauth) || repo.called {
			t.Fatalf("err = %v (repo called %v), want UnauthorizedError", err, repo.called)
		}
	})
}
