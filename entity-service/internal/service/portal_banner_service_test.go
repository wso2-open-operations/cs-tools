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

func bannerAdminCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@wso2.com", Roles: []string{"cs_admin"}})
}

func TestPortalBannerListCustomerForcesTheCustomerAudience(t *testing.T) {
	repo := &recordingBannerRepo{rows: []domain.PortalBanner{{ID: "a", HTML: "<p>hi</p>", SortOrder: 0, Status: domain.PortalBannerPublished}}}
	svc := NewPortalBannerService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
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

func TestPortalBannerCreateRequiresTheAdminRole(t *testing.T) {
	repo := &recordingBannerRepo{}
	svc := NewPortalBannerService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "eng@wso2.com", Roles: []string{"cs_engineer"}})
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
	svc := NewPortalBannerService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	_, err := svc.Create(bannerAdminCtx(), domain.SavePortalBannerRequest{
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
	svc := NewPortalBannerService(&recordingBannerRepo{}, alwaysUnrestrictedAccess{}, []string{"cs_admin"})
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start := end.Add(time.Hour)
	for _, req := range []domain.SavePortalBannerRequest{
		{Status: "live", HTML: "<p>x</p>"},
		{Status: "draft", HTML: "  "},
		{Status: "draft", HTML: "<p>x</p>", SortOrder: -1},
		{Status: "draft", HTML: "<p>x</p>", StartsAt: &start, ExpiresAt: &end},
	} {
		if _, err := svc.Create(bannerAdminCtx(), req); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
}
