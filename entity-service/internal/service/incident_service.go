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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// incidentStateToEnum maps domain.IncidentState to incident_state_enum's
// real labels (migration 000058) -- identity for every value except
// "canceled", which the enum spells with one L ('CANCELED') where
// domain.IncidentStateCancelled has two ("CANCELLED").
func incidentStateToEnum(s domain.IncidentState) string {
	if s == domain.IncidentStateCancelled {
		return "CANCELED"
	}
	return string(s)
}

// incidentPriorityToEnum maps domain.IncidentPriority to
// incident_priority_enum's real labels. incident_priority_enum has no
// 'PLANNING' label at all (only CRITICAL/HIGH/MODERATE/LOW), so
// IncidentPriorityPlanning returns ok=false rather than a miscast value --
// callers must reject it with a ValidationError, not silently drop or bind
// it.
func incidentPriorityToEnum(p domain.IncidentPriority) (string, bool) {
	switch p {
	case domain.IncidentPriorityCritical, domain.IncidentPriorityHigh, domain.IncidentPriorityModerate, domain.IncidentPriorityLow:
		return string(p), true
	default:
		// Rejects IncidentPriorityPlanning (no incident_priority_enum
		// equivalent) and any other value JSON decoding let through --
		// domain.IncidentPriority is a plain string type with no decode-time
		// validation, so a caller-supplied value outside the enum's actual
		// four labels must be caught here, not left to fail as a raw
		// Postgres enum-cast error.
		return "", false
	}
}

// parseIncidentFieldFiltersPostgres translates SearchIncidentsFilters for
// the Postgres data source: reuses incident_filters.go's field/op
// allow-lists and its date-parsing/boolean-parsing helpers (both
// data-source-agnostic), but "state" values are mapped through
// incidentStateToEnum directly rather than ParseIncidentFieldFilters' own
// SN-raw-integer translation (parsedIncidentFilters.StateKeys). Also maps
// req.Filters.Priorities (a separate, top-level field, not part of the
// generic Filters array) the same way.
func parseIncidentFieldFiltersPostgres(f domain.SearchIncidentsFilters, now time.Time) (priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, err error) {
	for _, p := range f.Priorities {
		enumValue, ok := incidentPriorityToEnum(p)
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters.priorities contains a value with no equivalent on this data source: " + string(p)}
		}
		priorities = append(priorities, enumValue)
	}

	for _, f := range f.Filters {
		if !incidentFilterFieldSet[f.Field] {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentFilterOpSet[f.Op] {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			for _, v := range f.Values {
				state := domain.IncidentState(v)
				if !validIncidentState[state] {
					return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: state contains invalid value: " + v}
				}
				states = append(states, incidentStateToEnum(state))
			}
		case "assignmentGroupId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			// Accepted, validated, but never applied -- no backing column.
		case "businessServiceId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: businessServiceId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			serviceIDs = append(serviceIDs, f.Values...)
		case "assignedUserId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: assignedUserId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			assignedUserIDs = append(assignedUserIDs, f.Values...)
		case "createdOn":
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			t, err := parseIncidentFilterDate(f, f.Values[0], now)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			switch f.Op {
			case "gte":
				createdStartDate = t
			case "lte":
				createdEndDate = t
			default:
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
		case "slaViolated":
			if f.Op != "eq" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if len(f.Values) != 1 {
				return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: slaViolated eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			slaViolated = &b
		case "madeSla":
			if f.Op != "eq" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if len(f.Values) != 1 {
				return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: madeSla eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			madeSla = &b
		case "productName":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			// Accepted, validated, but never applied -- no confirmed
			// product-name-to-service mapping on this data source.
		}
	}
	return priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate, nil
}

type incidentService struct {
	repo repository.IncidentRepository
}

// NewIncidentService constructs an IncidentService backed by Postgres.
func NewIncidentService(repo repository.IncidentRepository) IncidentService {
	return &incidentService{repo: repo}
}

// SearchIncidents implements IncidentService.
func (s *incidentService) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := validateUUIDs("filters.parentIds", req.Filters.ParentIDs); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, err :=
		parseIncidentFieldFiltersPostgres(req.Filters, time.Now().UTC())
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	views, total, err := s.repo.SearchIncidents(ctx, req, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd)
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	return domain.SearchIncidentsResponse{
		Incidents: views,
		Total:     total,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

// AggregateIncidents implements IncidentService.
func (s *incidentService) AggregateIncidents(ctx context.Context, req domain.AggregateIncidentsRequest) (domain.AggregateResponse, error) {
	if !validIncidentAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, err :=
		parseIncidentFieldFiltersPostgres(req.Filters, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	searchReq := domain.SearchIncidentsRequest{Filters: req.Filters}
	return s.repo.AggregateIncidents(ctx, searchReq, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, req.GroupBy, req.MaxGroups)
}

// GetIncidentByID implements IncidentService.
func (s *incidentService) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentView{}, err
	}
	return s.repo.GetIncidentByID(ctx, id)
}

// SearchIncidentActivities implements IncidentService.
func (s *incidentService) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) (domain.SearchIncidentActivitiesResponse, error) {
	if err := validateUUIDs("incidentId", []string{req.IncidentID}); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	activity, total, err := s.repo.SearchIncidentActivities(ctx, req)
	if err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	return domain.SearchIncidentActivitiesResponse{
		Activity: activity,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// CreateIncident is not supported for the PostgreSQL data source: like
// CaseRepository.CreateCase, work_item.number has no DB default and no
// backing sequence anywhere in migrations/.
func (s *incidentService) CreateIncident(_ context.Context, _ domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	return domain.CreateIncidentResponse{}, &apierror.ServiceUnavailableError{
		Msg: "creating an incident is not available on this data source: work_item.number has no generation strategy defined here",
	}
}

// UpdateIncident is not supported for the PostgreSQL data source: several
// fields have no backing column at all (AssignmentGroupID,
// ConfigurationItemID, WatchList) and AdditionalComments/WorkNotes would
// need comment-table side effects mirroring caseService.UpdateCase's own
// comment-on-update behavior -- deferred as a unit rather than
// half-implemented.
func (s *incidentService) UpdateIncident(_ context.Context, _ domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	return domain.UpdateIncidentResponse{}, &apierror.ServiceUnavailableError{
		Msg: "updating an incident is not available on this data source yet",
	}
}

// HandOffIncidentToSpecialist is not supported for the PostgreSQL data
// source: this is an inherently ServiceNow-workflow-specific feature (moves
// the incident to a specialist group, opens a runbook-gap task, files a
// GitHub issue) with no assignment-group or handoff-tracking concept
// anywhere in this schema to derive an equivalent from.
func (s *incidentService) HandOffIncidentToSpecialist(_ context.Context, _ domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error) {
	return domain.HandOffIncidentToSpecialistResponse{}, &apierror.ServiceUnavailableError{
		Msg: "specialist handoff is not available on this data source: no assignment-group or handoff-tracking concept exists in this schema",
	}
}
