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
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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
	// snMirror is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// (config.DataSourcePostgresServiceNowDualWrite) -- see
	// NewProblemServiceWithSNMirror's own doc comment. When set,
	// CreateProblem delegates to createProblemSNFirst instead of the plain
	// Postgres path's ServiceUnavailableError below, mirroring
	// incidentService's identical snMirror-gated branch for CreateIncident.
	snMirror ProblemService
	// snWriteback is nil in every mode except
	// DATA_SOURCE=postgres-servicenow-dual-write, same convention as
	// incidentService's identical field -- see
	// NewIncidentServiceWithSNMirror's own doc comment. Set only via
	// NewProblemServiceWithSNMirror. Backs UpdateProblem's best-effort async
	// ServiceNow mirror write.
	snWriteback *SNWritebackDispatcher
}

// NewProblemService constructs a ProblemService backed by Postgres.
func NewProblemService(repo repository.ProblemRepository) ProblemService {
	return &problemService{repo: repo}
}

// NewProblemServiceWithSNMirror is NewProblemService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs for problem CREATE and
// UPDATE: a synchronous, ServiceNow-first creation path -- see
// createProblemSNFirst's own doc comment for the full reasoning (identical
// to incidentService.createIncidentSNFirst's: a Postgres-first async create
// could leave a permanent orphan) -- plus a Postgres-first, best-effort async
// ServiceNow mirror for UPDATE (see UpdateProblem's own doc comment),
// identical in shape to incidentService's own snMirror/snWriteback pair.
//
// mirror is the ServiceNow-backed ProblemService (from
// NewServiceNowProblemService) whose CreateProblem performs the real
// ServiceNow POST and whose UpdateProblem is the async mirror's target. It
// is never made the active ProblemService here -- reads always stay on
// Postgres in this mode.
//
// dispatcher is the single shared *SNWritebackDispatcher constructed once in
// routes.go and reused across case/incident/problem's UPDATE mirrors -- see
// NewIncidentServiceWithSNMirror's own doc comment for why one shared
// instance is correct (a fixed background worker pool plus one
// sn_writeback_failures repository, nothing problem-specific about it).
func NewProblemServiceWithSNMirror(repo repository.ProblemRepository, mirror ProblemService, dispatcher *SNWritebackDispatcher) ProblemService {
	return &problemService{repo: repo, snMirror: mirror, snWriteback: dispatcher}
}

// resolveActorEmail resolves the caller's email from x-user-id-token, for
// stamping work_item.updated_by on UpdateProblem -- a plain VARCHAR audit
// string (an email, by this codebase's own convention), same shape as
// deploymentService.resolveActorEmail. No UserRepository lookup to a full
// domain.User is needed: unlike incidentService.UpdateIncident (which writes
// a comment.created_by FK-adjacent field), UpdateProblem writes no comment
// row at all.
func (s *problemService) resolveActorEmail(ctx context.Context) (string, error) {
	email, err := callerEmail(ctx)
	if err != nil {
		return "", err
	}
	return email, nil
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

// CreateProblem implements ProblemService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createProblemSNFirst instead of the plain Postgres path's
// ServiceUnavailableError below -- see that method's own doc comment.
func (s *problemService) CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	if s.snMirror != nil {
		return s.createProblemSNFirst(ctx, req)
	}
	return s.createProblemPortal(ctx, req)
}

// createProblemPortal implements CreateProblem's plain-Postgres path
// (s.snMirror == nil, no ServiceNow at all) -- unblocked by migration 0140's
// next_portal_work_item_number(), the same product decision that used to
// defer this (see CLAUDE.md, "CreateCase and case numbers"). Validates the
// same two fields createProblemSNFirst validates before ever reaching
// ServiceNow -- there is no ServiceNow call on this path at all, but the
// checks still belong before the repository call, same "validate before
// writing" convention every service method here follows. createdBy is
// resolved from the caller's own JWT email claim, same chain
// createProblemSNFirst already uses.
func (s *problemService) createProblemPortal(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	createdBy, err := callerEmail(ctx)
	if err != nil {
		return domain.ProblemDetail{}, err
	}
	if utf8.RuneCountInString(req.Subject) > maxWorkItemSubjectLength {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: fmt.Sprintf("subject cannot exceed %d characters", maxWorkItemSubjectLength)}
	}
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" && !validProblemCategoryPG[strings.ToUpper(strings.TrimSpace(*req.Category))] {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "category contains invalid value: " + *req.Category}
	}
	return s.repo.CreateProblem(ctx, req, createdBy)
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
// claim -- the same validated-identity lookup (callerEmail)
// caseService.CreateCase already uses when req.CreatedBy is empty -- since
// the calling user's identity is the only real signal for who actually
// created the problem. An UnauthorizedError/ValidationError from that
// resolution is returned before ever calling ServiceNow, since without a
// createdBy there would be nothing valid to insert even if ServiceNow
// accepted the create.
func (s *problemService) createProblemSNFirst(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	createdBy, err := callerEmail(ctx)
	if err != nil {
		return domain.ProblemDetail{}, err
	}

	// Validate everything the Postgres insert will later depend on BEFORE the
	// ServiceNow call: a failure after it leaves a ServiceNow record with no
	// Postgres row.
	if utf8.RuneCountInString(req.Subject) > maxWorkItemSubjectLength {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: fmt.Sprintf("subject cannot exceed %d characters", maxWorkItemSubjectLength)}
	}
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" && !validProblemCategoryPG[strings.ToUpper(strings.TrimSpace(*req.Category))] {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "category contains invalid value: " + *req.Category}
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

// UpdateProblem supports exactly 5 fields under
// DATA_SOURCE=postgres-servicenow-dual-write (s.snWriteback != nil):
// AssignedToID, CauseNotes, FixNotes, Workaround, TargetResolutionDate --
// see ProblemRepository.UpdateProblemFields' own doc comment for their
// column mapping. Every other mode still returns the
// unconditional ServiceUnavailableError below.
//
// Transition and AssignmentGroupID are rejected outright, exactly like
// incidentService.UpdateIncident's own rejected-field list: Transition is
// validated server-side by ServiceNow's own workflow engine, with no fixed,
// confirmed transition rule set (preconditions, side effects) to reimplement
// here (see domain.UpdateProblemRequest's own doc comment); AssignmentGroupID
// has no backing column at all -- problem has no CMDB/assignment-group table
// anywhere in this schema (same gap ChangeRequestRepository's own package
// doc comment already documents for change_request.GroupID).
//
// A best-effort, async ServiceNow mirror write follows via s.snWriteback,
// exactly the Postgres-first/async-mirror shape
// incidentService.UpdateIncident already uses, for the identical reason: a
// failed mirror here just leaves ServiceNow's copy of an EXISTING problem
// stale on one field until retried by hand, not a permanent orphan the way
// a failed async CREATE would be.
//
// snProblemService.UpdateProblem (the mirror target) does no live pre-read
// either -- confirmed against its own doc comment, a straightforward
// validate-then-PATCH, so mirrorReq is passed to it directly with no
// narrower patcher interface needed, unlike case's snFieldsBundlePatcher
// indirection.
func (s *problemService) UpdateProblem(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
	if s.snWriteback == nil {
		return domain.UpdateProblemResponse{}, &apierror.ServiceUnavailableError{
			Msg: "updating a problem is not available on this data source: no defined state-transition rule exists in this schema",
		}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateProblemResponse{}, err
	}
	if req.Transition != nil || req.AssignmentGroupID != nil {
		return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "transition and assignmentGroupId are not supported by this data source"}
	}
	if req.AssignedToID == nil && req.CauseNotes == nil && req.FixNotes == nil &&
		req.Workaround == nil && req.TargetResolutionDate == nil {
		return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "at least one of assignedToId, causeNotes, fixNotes, workaround, or targetResolutionDate must be provided"}
	}
	if req.AssignedToID != nil {
		if err := validateUUIDs("assignedToId", []string{*req.AssignedToID}); err != nil {
			return domain.UpdateProblemResponse{}, err
		}
	}
	if req.TargetResolutionDate != nil {
		if _, err := time.Parse(time.RFC3339, *req.TargetResolutionDate); err != nil {
			return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "targetResolutionDate must be a valid RFC3339 timestamp"}
		}
	}

	actorEmail, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.UpdateProblemResponse{}, err
	}

	updatedOn, err := s.repo.UpdateProblemFields(ctx, req, actorEmail)
	if err != nil {
		return domain.UpdateProblemResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (guaranteed by the s.snWriteback == nil guard above). Dispatched
	// immediately once Postgres has committed -- BEFORE the GetProblem
	// re-read below, deliberately -- because the write has already
	// succeeded at this point regardless of whether the re-read that
	// follows does. Dispatching only after a successful re-read would mean
	// a re-read failure (e.g. a transient connection blip) skips the mirror
	// entirely: the caller gets an error for a write that actually
	// succeeded, ServiceNow never gets the update, and -- since
	// s.snWriteback.Dispatch itself was never called -- nothing lands in
	// sn_writeback_failures either, silent drift the dispatcher can't even
	// report on. Fires asynchronously and never affects this response.
	// mirrorReq carries only ID plus the field(s) this call actually set --
	// never forwards req itself -- so this can never accidentally carry
	// Transition/AssignmentGroupID (both already rejected above and
	// therefore always nil here) into the mirror call.
	mirrorReq := domain.UpdateProblemRequest{
		ID:                   req.ID,
		AssignedToID:         req.AssignedToID,
		CauseNotes:           req.CauseNotes,
		FixNotes:             req.FixNotes,
		Workaround:           req.Workaround,
		TargetResolutionDate: req.TargetResolutionDate,
	}
	payload := map[string]any{"id": req.ID}
	if req.AssignedToID != nil {
		payload["assignedToId"] = *req.AssignedToID
	}
	if req.CauseNotes != nil {
		payload["causeNotes"] = *req.CauseNotes
	}
	if req.FixNotes != nil {
		payload["fixNotes"] = *req.FixNotes
	}
	if req.Workaround != nil {
		payload["workaround"] = *req.Workaround
	}
	if req.TargetResolutionDate != nil {
		payload["targetResolutionDate"] = *req.TargetResolutionDate
	}
	s.snWriteback.Dispatch(ctx, "problem", req.ID, "update", payload,
		func(writeCtx context.Context) error {
			_, err := s.snMirror.UpdateProblem(writeCtx, mirrorReq)
			return err
		},
	)

	// Re-read via GetProblem, matching incidentService.UpdateIncident's own
	// GetIncidentByID re-read pattern, since UpdateProblemFields only
	// returns updated_on -- State/ResolutionCode/AssignedTo must reflect the
	// real post-write state, not be guessed at from req. The mirror above
	// has already been dispatched by this point regardless of whether this
	// re-read succeeds: a failure here only means this response can't
	// confirm the post-write view, not that the write itself (or its
	// mirror) is in question -- logged loudly, same convention as
	// createProblemSNFirst's own drift-logging branch, and returned as an
	// error since UpdateProblemResponse has no partial-view shape to fall
	// back to.
	detail, err := s.repo.GetProblem(ctx, req.ID)
	if err != nil {
		slog.ErrorContext(ctx, "update problem: problem was updated but the post-write re-read failed",
			"problemId", req.ID, "error", err)
		return domain.UpdateProblemResponse{}, err
	}

	updatedOnStr := updatedOn.UTC().Format(time.RFC3339)
	view := domain.UpdateProblemView{
		ID:             detail.ID,
		UpdatedOn:      &updatedOnStr,
		UpdatedBy:      &actorEmail,
		State:          detail.State,
		ResolutionCode: detail.ResolutionCode,
		AssignedTo:     detail.AssignedTo,
		// AssignmentGroup is always nil -- problem has no assignment-group
		// column anywhere (see this type's own package doc comment in
		// problem_repo.go).
	}

	return domain.UpdateProblemResponse{
		Message: "Problem updated successfully",
		Problem: view,
	}, nil
}

// maxWorkItemSubjectLength is work_item.subject's VARCHAR length (migration
// 0090). Longer values fail the Postgres insert.
const maxWorkItemSubjectLength = 512

// validProblemCategoryPG is problem_category_enum's label set (migration 0059).
var validProblemCategoryPG = map[string]bool{"SOFTWARE": true, "HARDWARE": true, "NETWORK": true, "DATABASE": true}
