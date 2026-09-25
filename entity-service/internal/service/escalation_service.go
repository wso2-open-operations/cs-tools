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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type escalationService struct {
	repo   repository.EscalationRepository
	access AccessService
}

// NewEscalationService constructs an EscalationService backed by Postgres.
// CreateEscalation always returns a ServiceUnavailableError -- see
// EscalationRepository's own doc comment for why (no defined level-
// transition or notification-recipient rule to derive from the schema
// alone).
func NewEscalationService(repo repository.EscalationRepository, access AccessService) EscalationService {
	return &escalationService{repo: repo, access: access}
}

// SearchEscalations implements EscalationService.
func (s *escalationService) SearchEscalations(ctx context.Context, req domain.SearchEscalationsRequest) (domain.SearchEscalationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEscalationsResponse{}, err
	}
	var caseIDs []string
	var currentLevels []int
	if req.Filters != nil {
		if err := validateUUIDs("filters.caseIds", req.Filters.CaseIDs); err != nil {
			return domain.SearchEscalationsResponse{}, err
		}
		caseIDs = req.Filters.CaseIDs
		for _, l := range req.Filters.CurrentLevels {
			// case_escalation_level_enum only spans EL0..EL5 (migration
			// 000053); an out-of-range value would otherwise reach
			// escalationLevelToEnum and trip a Postgres enum-cast error at
			// query time, surfacing as a 500 instead of a 400.
			if l < 0 || l > 5 {
				return domain.SearchEscalationsResponse{}, &apierror.ValidationError{Msg: "filters.currentLevels must be between 0 and 5"}
			}
		}
		currentLevels = req.Filters.CurrentLevels
	}
	sortField, sortOrder := "", ""
	if req.SortBy != nil {
		sortField = string(req.SortBy.Field)
		sortOrder = string(req.SortBy.Order)
	}

	// SearchEscalationsFilters has no projectIds field for a caller to
	// (mis)supply -- escalations are scoped purely from the caller's own
	// resolved AccessScope, never from request input. Escalations were
	// found with no authorization at all before this fix: escalationFromJoins
	// already joins work_item (for wi.number/subject), which HAS a real
	// project_id (case_escalation is 100% CASE-type in real data, unlike
	// incident/problem/incident_task), but nothing filtered on it.
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.SearchEscalationsResponse{}, err
	}
	var projectIDs []string
	if !scope.Unrestricted {
		if len(scope.ProjectIDs) == 0 {
			// Fail-closed short-circuit: an empty projectIDs slice reaching
			// the repo would be indistinguishable from "no filter" there
			// (same class of bug already caught and fixed for
			// change_request/conversation) -- stop here instead.
			return domain.SearchEscalationsResponse{
				Escalations: []domain.Escalation{},
				Limit:       req.Pagination.Limit,
				Offset:      req.Pagination.Offset,
			}, nil
		}
		projectIDs = scope.ProjectIDs
	}

	escalations, total, err := s.repo.SearchEscalations(ctx, caseIDs, currentLevels, projectIDs, sortField, sortOrder, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchEscalationsResponse{}, err
	}

	return domain.SearchEscalationsResponse{
		Escalations: escalations,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
	}, nil
}

// CreateEscalation implements EscalationService.
func (s *escalationService) CreateEscalation(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
	return domain.CreateEscalationResponse{}, &apierror.ServiceUnavailableError{
		Msg: "creating an escalation is not available on this data source: no defined rule for the next escalation level or notification recipients exists in this schema",
	}
}
