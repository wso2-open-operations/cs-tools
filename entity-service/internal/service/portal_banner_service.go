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
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const maxPortalBannerHTML = 100000

type portalBannerService struct {
	repo       repository.PortalBannerRepository
	access     AccessService
	adminRoles []string
}

// NewPortalBannerService constructs a PortalBannerService. adminRoles is AUTH_ADMIN_ROLES.
func NewPortalBannerService(repo repository.PortalBannerRepository, access AccessService, adminRoles []string) PortalBannerService {
	return &portalBannerService{repo: repo, access: access, adminRoles: adminRoles}
}

func (s *portalBannerService) ListCustomer(ctx context.Context) (domain.PortalBannerViewList, error) {
	return s.listViews(ctx, repository.PortalBannerFilter{Status: domain.PortalBannerPublished, Customer: true})
}

func (s *portalBannerService) ListCSM(ctx context.Context) (domain.PortalBannerViewList, error) {
	if err := RequireInternalCaller(ctx, s.access, "CSM banners are only available to internal callers"); err != nil {
		return domain.PortalBannerViewList{}, err
	}
	return s.listViews(ctx, repository.PortalBannerFilter{Status: domain.PortalBannerPublished, CSM: true})
}

func (s *portalBannerService) listViews(ctx context.Context, filter repository.PortalBannerFilter) (domain.PortalBannerViewList, error) {
	rows, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PortalBannerViewList{}, err
	}
	out := make([]domain.PortalBannerView, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.View())
	}
	return domain.PortalBannerViewList{Banners: out}, nil
}

func (s *portalBannerService) ListAdmin(ctx context.Context) (domain.PortalBannerList, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return domain.PortalBannerList{}, err
	}
	rows, err := s.repo.List(ctx, repository.PortalBannerFilter{})
	if err != nil {
		return domain.PortalBannerList{}, err
	}
	if rows == nil {
		rows = []domain.PortalBanner{}
	}
	return domain.PortalBannerList{Banners: rows}, nil
}

func (s *portalBannerService) Create(ctx context.Context, req domain.SavePortalBannerRequest) (domain.PortalBanner, error) {
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return domain.PortalBanner{}, err
	}
	clean, err := normalizePortalBanner(req)
	if err != nil {
		return domain.PortalBanner{}, err
	}
	return s.repo.Create(ctx, actor, clean)
}

func (s *portalBannerService) Update(ctx context.Context, id string, req domain.SavePortalBannerRequest) (domain.PortalBanner, error) {
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return domain.PortalBanner{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.PortalBanner{}, err
	}
	clean, err := normalizePortalBanner(req)
	if err != nil {
		return domain.PortalBanner{}, err
	}
	row, err := s.repo.Update(ctx, actor, id, clean)
	if err != nil {
		return domain.PortalBanner{}, err
	}
	if row == nil {
		return domain.PortalBanner{}, &apierror.NotFoundError{Msg: "banner not found"}
	}
	return *row, nil
}

func (s *portalBannerService) requireAdmin(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, "banners are only available to internal callers"); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" || !holdsAnyRole(id.Roles, s.adminRoles) {
		return "", &apierror.ForbiddenError{Msg: "only a CSM admin can change banners"}
	}
	return id.UserEmail, nil
}

func normalizePortalBanner(req domain.SavePortalBannerRequest) (domain.SavePortalBannerRequest, error) {
	status := strings.TrimSpace(req.Status)
	switch status {
	case domain.PortalBannerDraft, domain.PortalBannerPublished, domain.PortalBannerUnpublished:
	default:
		return req, &apierror.ValidationError{Msg: "status must be draft, published, or unpublished"}
	}
	if req.SortOrder < 0 {
		return req, &apierror.ValidationError{Msg: "sortOrder must be 0 or more"}
	}
	html := strings.TrimSpace(req.HTML)
	if html == "" {
		return req, &apierror.ValidationError{Msg: "html is required"}
	}
	if utf8.RuneCountInString(html) > maxPortalBannerHTML {
		return req, &apierror.ValidationError{Msg: "html is too long"}
	}
	if req.StartsAt != nil && req.ExpiresAt != nil && !req.ExpiresAt.After(*req.StartsAt) {
		return req, &apierror.ValidationError{Msg: "expiresAt must be after startsAt"}
	}
	req.Status = status
	req.HTML = html
	return req, nil
}
