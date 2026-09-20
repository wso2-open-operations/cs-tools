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

// parseProblemFieldFiltersPostgres translates SearchProblemsFilters.Filters
// for the Postgres data source: reuses problem_filters.go's field/op
// allow-lists, but "state" values are domain.ProblemState labels directly
// (they match problem_state_enum's own labels by identity -- unlike
// ParseProblemFieldFilters, which translates them into ServiceNow's raw
// problem_state numeric keys instead, per parsedProblemFilters' own doc
// comment). "assignmentGroupId" is accepted (validated as UUIDs) but never
// applied -- problem has no assignment-group column at all.
// "assignedUserId" IS applied: work_item.assigned_to_id is a real column.
func parseProblemFieldFiltersPostgres(filters []domain.ProblemFieldFilter) (states, assignedUserIDs []string, err error) {
	for _, f := range filters {
		if !problemFilterFieldSet[f.Field] {
			return nil, nil, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !problemFilterOpSet[f.Op] {
			return nil, nil, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}
		if err := requireProblemFilterValues(f); err != nil {
			return nil, nil, err
		}

		switch f.Field {
		case "state":
			for _, v := range f.Values {
				state := domain.ProblemState(v)
				if !validProblemState[state] {
					return nil, nil, &apierror.ValidationError{Msg: "filters: state contains invalid value: " + v}
				}
				states = append(states, string(state))
			}
		case "assignmentGroupId":
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return nil, nil, err
			}
			// Accepted, validated, but never applied -- no backing column.
		case "assignedUserId":
			if err := validateUUIDs("filters: assignedUserId", f.Values); err != nil {
				return nil, nil, err
			}
			assignedUserIDs = append(assignedUserIDs, f.Values...)
		}
	}
	return states, assignedUserIDs, nil
}

type problemService struct {
	repo repository.ProblemRepository
}

// NewProblemService constructs a ProblemService backed by Postgres.
func NewProblemService(repo repository.ProblemRepository) ProblemService {
	return &problemService{repo: repo}
}

// SearchProblems implements ProblemService.
func (s *problemService) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	states, assignedUserIDs, err := parseProblemFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	views, total, err := s.repo.SearchProblems(ctx, req, states, assignedUserIDs)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	return domain.SearchProblemsResponse{
		Problems: views,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// AggregateProblems implements ProblemService.
func (s *problemService) AggregateProblems(ctx context.Context, req domain.AggregateProblemsRequest) (domain.AggregateResponse, error) {
	if !validProblemAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	states, assignedUserIDs, err := parseProblemFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	searchReq := domain.SearchProblemsRequest{Filters: req.Filters}
	return s.repo.AggregateProblems(ctx, searchReq, states, assignedUserIDs, req.GroupBy, req.MaxGroups)
}

// GetProblem implements ProblemService.
func (s *problemService) GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ProblemDetail{}, err
	}
	return s.repo.GetProblem(ctx, id)
}

// CreateProblem is not supported for the PostgreSQL data source: like
// CaseRepository.CreateCase, work_item.number has no DB default and no
// backing sequence anywhere in migrations/.
func (s *problemService) CreateProblem(_ context.Context, _ domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	return domain.ProblemDetail{}, &apierror.ServiceUnavailableError{
		Msg: "creating a problem is not available on this data source: work_item.number has no generation strategy defined here",
	}
}

// UpdateProblem is not supported for the PostgreSQL data source: Transition
// is validated server-side by ServiceNow's own workflow engine, with no
// fixed, confirmed transition rule set (preconditions, side effects) to
// reimplement here -- see domain.UpdateProblemRequest's own doc comment for
// why this is deliberately not a closed enum this service could validate
// and apply itself.
func (s *problemService) UpdateProblem(_ context.Context, _ domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
	return domain.UpdateProblemResponse{}, &apierror.ServiceUnavailableError{
		Msg: "updating a problem is not available on this data source: no defined state-transition rule exists in this schema",
	}
}
