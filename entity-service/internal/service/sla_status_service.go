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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	defaultSLAStatusLimit = 500
	// maxSLAStatusLimit is far above the generic 50-row cap other search
	// endpoints use: this endpoint has exactly one real caller
	// (integrations/csm-notification-service, polling periodically for
	// currently-active clocks — around 5,500 rows checked live), not a
	// human paging through a UI list, so a low cap would only turn one
	// intended round trip into over a hundred for no benefit to anyone.
	maxSLAStatusLimit = 2000
)

// normalizeSLAStatusPagination applies this endpoint's own defaults/cap —
// see maxSLAStatusLimit's own doc comment for why they differ from
// normalizePagination's generic ones.
func normalizeSLAStatusPagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultSLAStatusLimit
	}
	if p.Limit > maxSLAStatusLimit {
		return &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", maxSLAStatusLimit)}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

type slaStatusService struct {
	repo   repository.SLAStatusRepository
	access AccessService
}

// NewSLAStatusService constructs an SLAStatusService backed by the given
// repository. access gates every call to internal callers
// (AUTH_INTERNAL_CLIENT_IDS) -- see requireInternalCaller's own doc comment
// for why: unlike every other Postgres-backed read, this endpoint has no
// per-project/per-case filtering of its own to scope by (it returns every
// currently-active clock across every case in one bulk list, for its one
// real caller, integrations/csm-notification-service's poller), so there is
// no scope short of "internal service" that would be safe to hand this out
// under.
func NewSLAStatusService(repo repository.SLAStatusRepository, access AccessService) SLAStatusService {
	return &slaStatusService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- mirrors onboarding_step_service.go's own helper of the same name and
// same reasoning; kept as its own unexported copy rather than a shared one
// since AccessService itself has no natural home for a helper this small.
func (s *slaStatusService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "sla status is only available to internal services"}
	}
	return nil
}

// SearchActiveSLAStatuses implements SLAStatusService.
func (s *slaStatusService) SearchActiveSLAStatuses(ctx context.Context, req domain.Pagination) (domain.SearchSLAStatusResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	if err := normalizeSLAStatusPagination(&req); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	statuses, total, err := s.repo.SearchActiveSLAStatuses(ctx, req)
	if err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	return domain.SearchSLAStatusResponse{
		Statuses: statuses,
		Total:    total,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}, nil
}
