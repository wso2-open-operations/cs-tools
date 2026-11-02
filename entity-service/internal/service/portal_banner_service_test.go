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

type recordingBannerRepo struct {
	filter repository.PortalBannerFilter
	rows   []domain.PortalBanner
	saved  domain.SavePortalBannerRequest
	actor  string
}

func (r *recordingBannerRepo) List(_ context.Context, filter repository.PortalBannerFilter) ([]domain.PortalBanner, error) {
	r.filter = filter
	return r.rows, nil
}

func (r *recordingBannerRepo) Create(_ context.Context, actor string, req domain.SavePortalBannerRequest) (domain.PortalBanner, error) {
	r.actor = actor
	r.saved = req
	return domain.PortalBanner{ID: "new", Status: req.Status, HTML: req.HTML, SortOrder: req.SortOrder}, nil
}

func (r *recordingBannerRepo) Update(_ context.Context, actor, id string, req domain.SavePortalBannerRequest) (*domain.PortalBanner, error) {
	r.actor = actor
	r.saved = req
	if id == "missing" {
		return nil, nil
	}
	row := domain.PortalBanner{ID: id, Status: req.Status, HTML: req.HTML}
	return &row, nil
}

const testCSMPortalBackendClientID = "csm-portal-backend-client-id"

// bannerAdminCtx is a generic admin identity (role-based), reused by several
// other services in this package (e.g. sla_status_service_test.go) whose own
// admin gate is still AUTH_ADMIN_ROLES-based -- kept exactly as it always was.
// portalBannerService itself no longer reads Roles at all; use
// bannerCSMPortalCtx for its own tests instead.
func bannerAdminCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@wso2.com", Roles: []string{"cs_admin"}})
}

// bannerCSMPortalCtx is what a real admin's banner change looks like by the
// time it reaches this service: csm-portal-backend's own client id (its
// PermAdmin gate has already run) plus the forwarded human's email.
func bannerCSMPortalCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{
		Validated: true, UserEmail: "admin@wso2.com", ClientID: testCSMPortalBackendClientID,
	})
}

func newTestPortalBannerService(repo repository.PortalBannerRepository) PortalBannerService {
	return NewPortalBannerService(repo, alwaysUnrestrictedAccess{}, testCSMPortalBackendClientID)
}

func TestPortalBannerListCustomerForcesTheCustomerAudience(t *testing.T) {
	repo := &recordingBannerRepo{rows: []domain.PortalBanner{{ID: "a", HTML: "<p>hi</p>", SortOrder: 0, Status: domain.PortalBannerPublished}}}
	svc := newTestPortalBannerService(repo)
	got, err := svc.ListCustomer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !repo.filter.Customer || repo.filter.CSM || repo.filter.Status != domain.PortalBannerPublished {
		t.Fatalf("filter = %+v", repo.filter)
	}
	if len(got.Banners) != 1 || got.Banners[0].HTML != "<p>hi</p>" {
		t.Fatalf("views = %+v", got.Banners)
	}
}

func TestPortalBannerCreateRequiresTheCSMPortalBackendClientID(t *testing.T) {
	repo := &recordingBannerRepo{}
	svc := newTestPortalBannerService(repo)
	// A real admin's own call, but not proxied through csm-portal-backend
	// (e.g. some other internal service, or a direct call) -- must still be
	// refused, since the admin check now lives entirely in csm-portal-backend.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Validated: true, UserEmail: "admin@wso2.com", ClientID: "some-other-client",
	})
	_, err := svc.Create(ctx, domain.SavePortalBannerRequest{Status: "draft", HTML: "<p>x</p>"})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v", err)
	}
	if repo.actor != "" {
		t.Fatal("repo was called")
	}
}

func TestPortalBannerCreateRequiresASignedInPerson(t *testing.T) {
	repo := &recordingBannerRepo{}
	svc := newTestPortalBannerService(repo)
	// The right client id, but no forwarded human at all (a pure machine
	// call) -- still refused; csm-portal-backend never calls this with an
	// empty UserEmail in practice, but the service doesn't trust that alone.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Validated: true, ClientID: testCSMPortalBackendClientID,
	})
	_, err := svc.Create(ctx, domain.SavePortalBannerRequest{Status: "draft", HTML: "<p>x</p>"})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v", err)
	}
	if repo.actor != "" {
		t.Fatal("repo was called")
	}
}

func TestPortalBannerCreateStoresTheAdmin(t *testing.T) {
	repo := &recordingBannerRepo{}
	svc := newTestPortalBannerService(repo)
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	_, err := svc.Create(bannerCSMPortalCtx(), domain.SavePortalBannerRequest{
		Status: " published ", HTML: "  <p>hi</p>  ", SortOrder: 1, StartsAt: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.actor != "admin@wso2.com" || repo.saved.Status != "published" || repo.saved.HTML != "<p>hi</p>" || repo.saved.SortOrder != 1 {
		t.Fatalf("saved %+v by %s", repo.saved, repo.actor)
	}
}

func TestPortalBannerRejectsABadSave(t *testing.T) {
	svc := newTestPortalBannerService(&recordingBannerRepo{})
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start := end.Add(time.Hour)
	for _, req := range []domain.SavePortalBannerRequest{
		{Status: "live", HTML: "<p>x</p>"},
		{Status: "draft", HTML: "  "},
		{Status: "draft", HTML: "<p>x</p>", SortOrder: -1},
		{Status: "draft", HTML: "<p>x</p>", StartsAt: &start, ExpiresAt: &end},
	} {
		if _, err := svc.Create(bannerCSMPortalCtx(), req); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
}
