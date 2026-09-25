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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// incidentTaskStateEnumSet is the real incident_task_state_enum label set
// (migration 000066). Unlike ParseIncidentTaskFieldFilters (which treats
// "state" filter values as raw ServiceNow integers -- see
// parsedIncidentTaskFilters.StateKeys' own doc comment for why that data
// source's state choice list has no unambiguous enum to translate through),
// Postgres's own enum has no such ambiguity, so this data source accepts the
// enum's own label strings directly instead of reusing that SN-shaped parse
// function.
var incidentTaskStateEnumSet = map[string]bool{
	"PENDING": true, "OPEN": true, "WORK_IN_PROGRESS": true,
	"CLOSED_COMPLETE": true, "CLOSED_INCOMPLETE": true, "CLOSED_SKIPPED": true,
}

// parseIncidentTaskFieldFiltersPostgres translates
// SearchIncidentTasksFilters.Filters for the Postgres data source: reuses
// incident_task_filters.go's field/op allow-lists, but "state" values are
// validated as incident_task_state_enum labels (case-insensitive) rather
// than SN raw integers. "assignmentGroupId" is accepted (validated as UUIDs)
// but never applied by the repository -- incident_task has no assignment-group
// column at all, same as change_request's own AssignedTeamID gap.
func parseIncidentTaskFieldFiltersPostgres(filters []domain.IncidentTaskFieldFilter) (states, incidentIDs []string, err error) {
	for _, f := range filters {
		if !incidentTaskFilterFieldSet[f.Field] {
			return nil, nil, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentTaskFilterOpSet[f.Op] {
			return nil, nil, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}
		if err := requireIncidentTaskFilterValues(f); err != nil {
			return nil, nil, err
		}

		switch f.Field {
		case "state":
			for _, v := range f.Values {
				upper := strings.ToUpper(v)
				if !incidentTaskStateEnumSet[upper] {
					return nil, nil, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q value %q is not a valid state", f.Field, v)}
				}
				states = append(states, upper)
			}
		case "assignmentGroupId":
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return nil, nil, err
			}
			// Accepted, validated, but never applied -- no backing column.
		case "incidentId":
			if err := validateUUIDs("filters: incidentId", f.Values); err != nil {
				return nil, nil, err
			}
			incidentIDs = append(incidentIDs, f.Values...)
		}
	}
	return states, incidentIDs, nil
}

type incidentTaskService struct {
	repo   repository.IncidentTaskRepository
	access AccessService
}

// NewIncidentTaskService constructs an IncidentTaskService backed by Postgres.
func NewIncidentTaskService(repo repository.IncidentTaskRepository, access AccessService) IncidentTaskService {
	return &incidentTaskService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- same reasoning as incidentService's own copy: incident_task rows have
// no project association at all (work_item.project_id is NULL for every
// real incident_task, confirmed live against a real database copy), and in
// practice only csm-portal-backend (WSO2-internal) calls these endpoints.
func (s *incidentTaskService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "incident tasks are only available to internal services"}
	}
	return nil
}

// SearchIncidentTasks implements IncidentTaskService.
func (s *incidentTaskService) SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest) (domain.SearchIncidentTasksResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}
	states, incidentIDs, err := parseIncidentTaskFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}

	tasks, total, err := s.repo.SearchIncidentTasks(ctx, req, states, incidentIDs)
	if err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}

	return domain.SearchIncidentTasksResponse{
		IncidentTasks: tasks,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

// AggregateIncidentTasks implements IncidentTaskService.
//
// validIncidentTaskAggregateField (sn_incident_task_service.go) is the wire
// contract's full groupBy allow-list ("state"/"assignmentGroup", matching
// openapi.yaml); this only checks the value is a legal groupBy at all. The
// repository rejects "assignmentGroup" specifically with its own,
// data-source-specific message (no assignment-group column exists here).
func (s *incidentTaskService) AggregateIncidentTasks(ctx context.Context, req domain.AggregateIncidentTasksRequest) (domain.AggregateResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.AggregateResponse{}, err
	}
	if !validIncidentTaskAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	states, incidentIDs, err := parseIncidentTaskFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	searchReq := domain.SearchIncidentTasksRequest{Filters: req.Filters}
	return s.repo.AggregateIncidentTasks(ctx, searchReq, states, incidentIDs, req.GroupBy, req.MaxGroups)
}

// GetIncidentTask implements IncidentTaskService.
func (s *incidentTaskService) GetIncidentTask(ctx context.Context, id string) (domain.IncidentTaskDetail, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.IncidentTaskDetail{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentTaskDetail{}, err
	}
	return s.repo.GetIncidentTask(ctx, id)
}
