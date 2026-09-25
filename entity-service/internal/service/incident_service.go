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
	"log/slog"
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
	repo   repository.IncidentRepository
	access AccessService
	// snMirror is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// (config.DataSourcePostgresServiceNowDualWrite) -- see
	// NewIncidentServiceWithSNMirror's own doc comment. When set, CreateIncident
	// delegates to createIncidentSNFirst instead of the plain Postgres path's
	// ServiceUnavailableError below, mirroring caseService's identical
	// snMirror-gated branch for CreateCase.
	snMirror IncidentService
	// eventPublisher is nil in every mode except
	// DATA_SOURCE=postgres-servicenow-dual-write. createIncidentSNFirst
	// publishes incident.created itself, after CreateIncidentFromServiceNow
	// succeeds -- the mirror IncidentService above is always constructed
	// with its own publisher=nil in this mode, specifically so it never
	// publishes prematurely (before the Postgres insert this mode's reads
	// actually depend on has even been attempted). See
	// publishIncidentCreatedEvent's doc comment for the full reasoning.
	eventPublisher EventPublisherService
}

// NewIncidentService constructs an IncidentService backed by Postgres.
func NewIncidentService(repo repository.IncidentRepository, access AccessService) IncidentService {
	return &incidentService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- mirrors slaStatusService/onboardingStepService's own helper of the same
// name and same reasoning. Incidents (unlike case/change_request/
// conversation) have no project association at all: work_item.project_id is
// NULL for every real incident/incident_task/problem row (confirmed live
// against a real database copy, 100% NULL across 83,198 real incidents) --
// these are internal ITIL/ops records, not customer-project-scoped data, and
// in practice only csm-portal-backend (WSO2-internal) calls these endpoints
// today. There is no scope short of "internal caller" that would be safe to
// hand this out under.
func (s *incidentService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "incidents are only available to internal services"}
	}
	return nil
}

// NewIncidentServiceWithSNMirror is NewIncidentService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs for incident CREATE: a
// synchronous, ServiceNow-first creation path -- see createIncidentSNFirst's
// own doc comment for the full reasoning (identical to
// caseService.createCaseSNFirst's: a Postgres-first async create could leave
// a permanent orphan). Unlike case, this mode has no incident UPDATE mirror
// at all yet -- UpdateIncident stays exactly as unsupported here as it is in
// every other mode (see UpdateIncident's own doc comment); only CREATE is in
// scope for this pilot extension.
//
// mirror is the ServiceNow-backed IncidentService (from
// NewServiceNowIncidentService) whose CreateIncident performs the real
// ServiceNow POST, including its own side effects (publishIncidentCreated).
// It is never made the active IncidentService here -- reads always stay on
// Postgres in this mode.
func NewIncidentServiceWithSNMirror(repo repository.IncidentRepository, access AccessService, mirror IncidentService, eventPublisher EventPublisherService) IncidentService {
	return &incidentService{repo: repo, access: access, snMirror: mirror, eventPublisher: eventPublisher}
}

// SearchIncidents implements IncidentService.
func (s *incidentService) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
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
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.AggregateResponse{}, err
	}
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
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.IncidentView{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentView{}, err
	}
	return s.repo.GetIncidentByID(ctx, id)
}

// SearchIncidentActivities implements IncidentService.
func (s *incidentService) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) (domain.SearchIncidentActivitiesResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}
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

// CreateIncident implements IncidentService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createIncidentSNFirst instead of the plain Postgres path's
// ServiceUnavailableError below -- see that method's own doc comment.
func (s *incidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	if s.snMirror != nil {
		// ConfigurationItemID/AssignmentGroupID have no backing column on
		// this data source at all (unlike Subcategory/AssignedEngineerID/
		// WatchList/AdditionalComments/WorkNotes, which are accepted but
		// silently not persisted -- a separate, tracked follow-up per
		// CodeRabbit's finding on PR #1922). Rejecting these two explicitly
		// is strictly better than the alternative: ServiceNow would already
		// have accepted and stored them by the time Postgres is ever
		// touched, so silently dropping them here would mean the caller's
		// request appears to succeed while quietly losing data they
		// explicitly asked to set.
		if req.ConfigurationItemID != nil {
			return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "configurationItemId is not supported for this data source"}
		}
		if req.AssignmentGroupID != nil {
			return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "assignmentGroupId is not supported for this data source"}
		}
		return s.createIncidentSNFirst(ctx, req)
	}
	// CreateIncident is not supported for the plain PostgreSQL data source:
	// like CaseRepository.CreateCase, work_item.number has no DB default and
	// no backing sequence anywhere in migrations/.
	return domain.CreateIncidentResponse{}, &apierror.ServiceUnavailableError{
		Msg: "creating an incident is not available on this data source: work_item.number has no generation strategy defined here",
	}
}

// createIncidentSNFirst implements CreateIncident's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly mirroring caseService.createCaseSNFirst's reasoning
// -- see that method's own doc comment for why CREATE must be ServiceNow
// -first rather than Postgres-first-and-async: a Postgres row with no
// ServiceNow counterpart would be a PERMANENT orphan (ServiceNow is still
// the real backing store this platform proxies most writes onto), while an
// async-after-commit UPDATE has no equivalent failure mode.
//
// This call is made exactly once: no internal retry. If ServiceNow's HTTP
// response is lost after it actually created the record server-side, an
// internal retry here would create a second, duplicate ServiceNow record --
// worse than a request that surfaces the error and lets the caller decide
// whether to retry. Retry policy is the caller's responsibility.
//
// On success, id/number/createdBy come from ServiceNow's own response and
// are used AS-IS for the Postgres insert
// (IncidentRepository.CreateIncidentFromServiceNow) rather than generated --
// see that method's own doc comment for why there is no wso2ID parameter
// here, unlike case's equivalent. This is also what makes incident creation
// possible on Postgres at all in this mode, for the same reason case's own
// pilot did: IncidentRepository's own doc comment explains why plain
// CreateIncident can't generate work_item.number itself.
func (s *incidentService) createIncidentSNFirst(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	snResp, err := s.snMirror.CreateIncident(ctx, req)
	if err != nil {
		// ServiceNow never accepted the incident -- nothing is written to
		// Postgres at all, by construction (s.repo.CreateIncidentFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateIncidentResponse{}, err
	}

	resp, err := s.repo.CreateIncidentFromServiceNow(ctx, req, snResp.Incident.ID, snResp.Incident.Number, snResp.Incident.CreatedBy)
	if err != nil {
		// ServiceNow already has the incident at this point -- this is now
		// real drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather
		// than only returned, same convention as
		// caseService.createCaseSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create incident: ServiceNow incident created but the Postgres insert failed",
			"incidentId", snResp.Incident.ID, "snNumber", snResp.Incident.Number, "error", err)
		return domain.CreateIncidentResponse{}, err
	}
	// Only now -- Postgres has confirmed the row this mode's reads actually
	// depend on -- is it safe to publish. See publishIncidentCreatedEvent's
	// doc comment for why this can't just be snIncidentService's own
	// automatic publish (that fires right after the ServiceNow POST, before
	// this Postgres insert was even attempted).
	publishIncidentCreatedEvent(ctx, s.eventPublisher, req, resp.Incident.ID)
	return resp, nil
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
