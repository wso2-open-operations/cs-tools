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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
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
	repo   repository.ProblemRepository
	access AccessService
	// snMirror is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// (config.DataSourcePostgresServiceNowDualWrite) -- see
	// NewProblemServiceWithSNMirror's own doc comment. When set,
	// CreateProblem delegates to createProblemSNFirst instead of the plain
	// Postgres path's ServiceUnavailableError below, mirroring
	// incidentService's identical snMirror-gated branch for CreateIncident.
	snMirror ProblemService
}

// NewProblemService constructs a ProblemService backed by Postgres.
func NewProblemService(repo repository.ProblemRepository, access AccessService) ProblemService {
	return &problemService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- same reasoning as incidentService's own copy: problem rows have no
// project association at all (work_item.project_id is NULL for every real
// problem, confirmed live against a real database copy), and in practice
// only csm-portal-backend (WSO2-internal) calls these endpoints.
func (s *problemService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "problems are only available to internal services"}
	}
	return nil
}

// NewProblemServiceWithSNMirror is NewProblemService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs for problem CREATE: a
// synchronous, ServiceNow-first creation path -- see
// createProblemSNFirst's own doc comment for the full reasoning (identical
// to incidentService.createIncidentSNFirst's: a Postgres-first async create
// could leave a permanent orphan). This mode has no problem UPDATE mirror --
// UpdateProblem stays exactly as unsupported here as it is in every other
// mode; only CREATE is in scope for this pilot extension.
//
// mirror is the ServiceNow-backed ProblemService (from
// NewServiceNowProblemService) whose CreateProblem performs the real
// ServiceNow POST. It is never made the active ProblemService here -- reads
// always stay on Postgres in this mode.
func NewProblemServiceWithSNMirror(repo repository.ProblemRepository, access AccessService, mirror ProblemService) ProblemService {
	return &problemService{repo: repo, access: access, snMirror: mirror}
}

// SearchProblems implements ProblemService.
func (s *problemService) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
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
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.AggregateResponse{}, err
	}
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
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ProblemDetail{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ProblemDetail{}, err
	}
	return s.repo.GetProblem(ctx, id)
}

// CreateProblem implements ProblemService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createProblemSNFirst instead of the plain Postgres path's
// ServiceUnavailableError below -- see that method's own doc comment.
func (s *problemService) CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ProblemDetail{}, err
	}
	if s.snMirror != nil {
		return s.createProblemSNFirst(ctx, req)
	}
	// CreateProblem is not supported for the plain PostgreSQL data source:
	// like CaseRepository.CreateCase, work_item.number has no DB default and
	// no backing sequence anywhere in migrations/.
	return domain.ProblemDetail{}, &apierror.ServiceUnavailableError{
		Msg: "creating a problem is not available on this data source: work_item.number has no generation strategy defined here",
	}
}

// createProblemSNFirst implements CreateProblem's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly mirroring incidentService.createIncidentSNFirst's
// reasoning -- see that method's own doc comment for why CREATE must be
// ServiceNow-first rather than Postgres-first-and-async.
//
// The ServiceNow call is made exactly once, with no internal retry: retrying
// here risks creating a second, duplicate ServiceNow record if ServiceNow's
// create actually succeeded but the HTTP response back to entity-service was
// lost (timeout/network blip) -- entity-service has no way to distinguish
// that from a real failure, and retry policy for that case belongs to the
// caller, not this layer.
//
// On success, id/number come from ServiceNow's own response and are used
// AS-IS for the Postgres insert
// (ProblemRepository.CreateProblemFromServiceNow) rather than generated.
// Unlike case/incident/change_request, createdBy is NOT taken from
// ServiceNow's response: ServiceNow's problem create endpoint
// (snProblemDetailResponse) returns no createdBy/createdOn field at all.
// Instead, createdBy is resolved from the requesting user's own JWT email
// claim -- the same middleware.UserIDTokenFromContext + emailFromJWT chain
// caseService.CreateCase already uses when req.CreatedBy is empty -- since
// the calling user's identity is the only real signal for who actually
// created the problem. An UnauthorizedError/ValidationError from that
// resolution is returned before ever calling ServiceNow, since without a
// createdBy there would be nothing valid to insert even if ServiceNow
// accepted the create.
func (s *problemService) createProblemSNFirst(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.ProblemDetail{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	createdBy, err := emailFromJWT(token)
	if err != nil {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}

	snResp, err := s.snMirror.CreateProblem(ctx, req)
	if err != nil {
		// ServiceNow never accepted the problem -- nothing is written to
		// Postgres at all, by construction
		// (s.repo.CreateProblemFromServiceNow is simply never called on
		// this path). No orphan gets created.
		return domain.ProblemDetail{}, err
	}

	id := ""
	if snResp.ID != nil {
		id = *snResp.ID
	}
	number := ""
	if snResp.Number != nil {
		number = *snResp.Number
	}

	resp, err := s.repo.CreateProblemFromServiceNow(ctx, req, id, number, createdBy, snResp.State)
	if err != nil {
		// ServiceNow already has the problem at this point -- this is now
		// real drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather
		// than only returned, same convention as
		// incidentService.createIncidentSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create problem: ServiceNow problem created but the Postgres insert failed",
			"problemId", id, "snNumber", number, "error", err)
		return domain.ProblemDetail{}, err
	}
	return resp, nil
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
