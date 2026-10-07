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
func parseProblemFieldFiltersPostgres(filters []domain.ProblemFieldFilter) (states, assignedUserIDs, assignmentGroupIDs []string, err error) {
	for _, f := range filters {
		if !problemFilterFieldSet[f.Field] {
			return nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !problemFilterOpSet[f.Op] {
			return nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}
		if err := requireProblemFilterValues(f); err != nil {
			return nil, nil, nil, err
		}

		switch f.Field {
		case "state":
			for _, v := range f.Values {
				state := domain.ProblemState(v)
				if !validProblemState[state] {
					return nil, nil, nil, &apierror.ValidationError{Msg: "filters: state contains invalid value: " + v}
				}
				states = append(states, string(state))
			}
		case "assignmentGroupId":
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return nil, nil, nil, err
			}
			assignmentGroupIDs = append(assignmentGroupIDs, f.Values...)
		case "assignedUserId":
			if err := validateUUIDs("filters: assignedUserId", f.Values); err != nil {
				return nil, nil, nil, err
			}
			assignedUserIDs = append(assignedUserIDs, f.Values...)
		}
	}
	return states, assignedUserIDs, assignmentGroupIDs, nil
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
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}

// SearchProblems implements ProblemService.
func (s *problemService) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	states, assignedUserIDs, assignmentGroupIDs, err := parseProblemFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	views, total, err := s.repo.SearchProblems(ctx, req, states, assignedUserIDs, assignmentGroupIDs)
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
	states, assignedUserIDs, assignmentGroupIDs, err := parseProblemFieldFiltersPostgres(req.Filters.Filters)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	searchReq := domain.SearchProblemsRequest{Filters: req.Filters}
	return s.repo.AggregateProblems(ctx, searchReq, states, assignedUserIDs, assignmentGroupIDs, req.GroupBy, req.MaxGroups)
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
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.ProblemDetail{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	createdBy, err := emailFromJWT(token)
	if err != nil {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	if utf8.RuneCountInString(req.Subject) > maxWorkItemSubjectLength {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: fmt.Sprintf("subject cannot exceed %d characters", maxWorkItemSubjectLength)}
	}
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" && !validProblemCategoryPG[strings.ToUpper(strings.TrimSpace(*req.Category))] {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "category contains invalid value: " + *req.Category}
	}
	// ServiceNow's problem defaults are impact and urgency 3 - Low, and the
	// CSM API never sets them; its "Priority Problem Lookup" then derives the
	// priority from them (discovery scripts 63-65).
	return s.repo.CreateProblem(ctx, req, createdBy, newProblemPriorityFields())
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

	resp, err := s.repo.CreateProblemFromServiceNow(ctx, req, id, number, createdBy, snResp.State, problemPriorityFromServiceNow(snResp.Priority))
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

// problemTransitions is ServiceNow's ProblemUtils._PROBLEM_TRANSITIONS: the
// five forward moves of the problem state model its API allows, keyed by the
// request's transition name. One table for both data sources, so Postgres
// and ServiceNow can never disagree about what a move means.
var problemTransitions = map[string]repository.ProblemTransition{
	"assess":  {Name: "assess", From: "NEW", To: "ASSESS"},
	"confirm": {Name: "confirm", From: "ASSESS", To: "ROOT_CAUSE_ANALYSIS"},
	"fix":     {Name: "fix", From: "ROOT_CAUSE_ANALYSIS", To: "FIX_IN_PROGRESS"},
	"resolve": {Name: "resolve", From: "FIX_IN_PROGRESS", To: "RESOLVED"},
	"close":   {Name: "close", From: "RESOLVED", To: "CLOSED"},
}

// checkProblemTransitionRequirements refuses a move ServiceNow would refuse
// for a missing field, before anything is written. ServiceNow's state model
// (ProblemStateUtils, discovery script 61) wants an assignee to move to
// Assess and fix notes to move to Resolved -- the fields of its own "Assess"
// and "Resolve" dialogs (resolve's resolution code is set by the move
// itself). A value in the request decides; only an absent one falls back to
// the value already on the problem. Without this, dual-write gets ServiceNow's misleading 409 ("a
// populated 'Assigned to' is a confirmed live cause") and Postgres-only mode
// would move a problem ServiceNow never would.
func checkProblemTransitionRequirements(t repository.ProblemTransition, req domain.UpdateProblemRequest, current domain.ProblemDetail) error {
	// A value in the request decides, blank or not: the same save writes it,
	// so a blank one would clear what the problem has. Only an absent value
	// falls back to the problem's own.
	has := func(fromReq *string, onProblem bool) bool {
		if fromReq != nil {
			return strings.TrimSpace(*fromReq) != ""
		}
		return onProblem
	}
	switch t.Name {
	case "assess":
		if !has(req.AssignedToID, current.AssignedTo != nil && current.AssignedTo.ID != "") {
			return &apierror.ValidationError{Msg: "assess needs an assignee: send assignedToId, or assign the problem first"}
		}
	case "resolve":
		if !has(req.FixNotes, current.FixNotes != nil && strings.TrimSpace(*current.FixNotes) != "") {
			return &apierror.ValidationError{Msg: "resolve needs fix notes: send fixNotes, or add them to the problem first"}
		}
	}
	return nil
}

// UpdateProblem implements ProblemService for both Postgres data sources.
// A request carries a transition (a state move), plain fields, or both --
// the fields apply in the same save as the move, as ServiceNow's "Fix" UI
// action sets cause/fix notes while moving the state.
//
//   - DATA_SOURCE=postgres: Postgres is the only copy. A transition is
//     checked against the problem's current state (wrong state = 400 naming
//     it, as ServiceNow answers) and applied with its side effects and the
//     fields in one transaction (ProblemRepository.ApplyProblemTransition).
//   - DATA_SOURCE=postgres-servicenow-dual-write, transition: ServiceNow
//     FIRST, synchronously, with the whole request. ServiceNow runs its own
//     state model and business rules and can refuse or silently revert a
//     move (its "Update Problem State to Assess" rule forces Assess back
//     whenever the problem has an assignee; ProblemUtils reports that as a
//     409). Only once ServiceNow has accepted is the same move written to
//     Postgres, without the from-state check, since Postgres may lag
//     ServiceNow. A ServiceNow refusal returns as-is and Postgres is
//     untouched, so the two never disagree about the state.
//   - dual-write, fields only: Postgres first, then a best-effort async
//     ServiceNow mirror (snWriteback), as before. A failed mirror leaves
//     ServiceNow one field stale until retried, never a different state.
func (s *problemService) UpdateProblem(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateProblemResponse{}, err
	}
	var transition *repository.ProblemTransition
	if req.Transition != nil {
		t, ok := problemTransitions[strings.TrimSpace(*req.Transition)]
		if !ok {
			return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "transition must be one of: assess, confirm, fix, resolve, close"}
		}
		name := t.Name
		req.Transition = &name
		transition = &t
	}
	if transition == nil && req.AssignedToID == nil && req.AssignmentGroupID == nil && req.CauseNotes == nil &&
		req.FixNotes == nil && req.Workaround == nil && req.TargetResolutionDate == nil && req.ChangeRequestID == nil {
		return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "at least one of transition, assignedToId, assignmentGroupId, causeNotes, fixNotes, workaround, targetResolutionDate, or changeRequestId must be provided"}
	}
	// changeRequestId "" unlinks; anything else must be a change request's id.
	if req.ChangeRequestID != nil && *req.ChangeRequestID != "" {
		if err := validateUUIDs("changeRequestId", []string{*req.ChangeRequestID}); err != nil {
			return domain.UpdateProblemResponse{}, err
		}
	}
	for field, val := range map[string]*string{"assignedToId": req.AssignedToID, "assignmentGroupId": req.AssignmentGroupID} {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.UpdateProblemResponse{}, err
			}
		}
	}
	// One instant, two spellings: Postgres gets RFC3339 (what the repository
	// parses), ServiceNow its own "YYYY-MM-DD HH:mm:ss" (UTC), which is also
	// what the portal sends and this API documents.
	var snTargetDate *string
	if req.TargetResolutionDate != nil {
		t, err := parseProblemTargetDate(*req.TargetResolutionDate)
		if err != nil {
			return domain.UpdateProblemResponse{}, err
		}
		pg, sn := t.Format(time.RFC3339), t.Format(problemTargetDateLayout)
		req.TargetResolutionDate, snTargetDate = &pg, &sn
	}

	actorEmail, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.UpdateProblemResponse{}, err
	}

	if transition != nil {
		current, err := s.repo.GetProblem(ctx, req.ID)
		if err != nil {
			return domain.UpdateProblemResponse{}, err
		}
		if err := checkProblemTransitionRequirements(*transition, req, current); err != nil {
			return domain.UpdateProblemResponse{}, err
		}
	}

	var updatedOn time.Time
	switch {
	case transition != nil && s.snMirror != nil:
		snReq := req
		snReq.TargetResolutionDate = snTargetDate
		if _, err := s.snMirror.UpdateProblem(ctx, snReq); err != nil {
			return domain.UpdateProblemResponse{}, err
		}
		updatedOn, err = s.repo.ApplyProblemTransition(ctx, req, *transition, false, actorEmail)
		if err != nil {
			// ServiceNow has moved; Postgres has not. Loud, like
			// createProblemSNFirst's own drift branch: the next sync, or a
			// retry of the same request, brings Postgres level.
			slog.ErrorContext(ctx, "update problem: ServiceNow applied the transition but the Postgres write failed",
				"problemId", req.ID, "transition", transition.Name, "error", err)
			return domain.UpdateProblemResponse{}, err
		}
	case transition != nil:
		updatedOn, err = s.repo.ApplyProblemTransition(ctx, req, *transition, true, actorEmail)
		if err != nil {
			return domain.UpdateProblemResponse{}, err
		}
	default:
		updatedOn, err = s.repo.UpdateProblemFields(ctx, req, actorEmail)
		if err != nil {
			return domain.UpdateProblemResponse{}, err
		}
		s.mirrorProblemFields(ctx, req, snTargetDate)
	}

	// Re-read so State/ResolutionCode/AssignedTo show the real post-write
	// state, not a guess from req. A failure here only means this response
	// can't confirm the write, not that the write (or its mirror) is in
	// question -- logged, and returned as an error since
	// UpdateProblemResponse has no partial shape to fall back to.
	detail, err := s.repo.GetProblem(ctx, req.ID)
	if err != nil {
		slog.ErrorContext(ctx, "update problem: problem was updated but the post-write re-read failed",
			"problemId", req.ID, "error", err)
		return domain.UpdateProblemResponse{}, err
	}

	updatedOnStr := updatedOn.UTC().Format(time.RFC3339)
	return domain.UpdateProblemResponse{
		Message: "Problem updated successfully",
		Problem: domain.UpdateProblemView{
			ID:              detail.ID,
			UpdatedOn:       &updatedOnStr,
			UpdatedBy:       &actorEmail,
			State:           detail.State,
			ResolutionCode:  detail.ResolutionCode,
			AssignedTo:      detail.AssignedTo,
			AssignmentGroup: detail.AssignmentGroup,
		},
	}, nil
}

// mirrorProblemFields is the dual-write fields-only path's best-effort async
// ServiceNow write; a no-op on DATA_SOURCE=postgres. Dispatched once Postgres
// has committed and before the re-read, so a failed re-read can't skip it.
// mirrorReq carries only the fields this call set, never a transition (a
// transition goes to ServiceNow synchronously instead).
func (s *problemService) mirrorProblemFields(ctx context.Context, req domain.UpdateProblemRequest, snTargetDate *string) {
	if s.snWriteback == nil {
		return
	}
	mirrorReq := domain.UpdateProblemRequest{
		ID:                   req.ID,
		AssignedToID:         req.AssignedToID,
		AssignmentGroupID:    req.AssignmentGroupID,
		CauseNotes:           req.CauseNotes,
		FixNotes:             req.FixNotes,
		Workaround:           req.Workaround,
		TargetResolutionDate: snTargetDate,
		ChangeRequestID:      req.ChangeRequestID,
	}
	payload := map[string]any{"id": req.ID}
	for key, val := range map[string]*string{
		"assignedToId": req.AssignedToID, "assignmentGroupId": req.AssignmentGroupID,
		"causeNotes": req.CauseNotes, "fixNotes": req.FixNotes, "workaround": req.Workaround,
		"targetResolutionDate": snTargetDate,
		"changeRequestId":      req.ChangeRequestID,
	} {
		if val != nil {
			payload[key] = *val
		}
	}
	s.snWriteback.Dispatch(ctx, "problem", req.ID, "update", payload,
		func(writeCtx context.Context) error {
			_, err := s.snMirror.UpdateProblem(writeCtx, mirrorReq)
			return err
		},
	)
}

// maxWorkItemSubjectLength is work_item.subject's VARCHAR length (migration
// 0090). Longer values fail the Postgres insert.
const maxWorkItemSubjectLength = 512

// validProblemCategoryPG is problem_category_enum's label set (migration 0059).
var validProblemCategoryPG = map[string]bool{"SOFTWARE": true, "HARDWARE": true, "NETWORK": true, "DATABASE": true}

// problemTargetDateLayout is ServiceNow's date-time format, the one
// ProblemUtils requires for targetResolutionDate and the portal sends (UTC).
const problemTargetDateLayout = "2006-01-02 15:04:05"

// parseProblemTargetDate accepts targetResolutionDate as this API documents
// it -- "YYYY-MM-DD HH:mm:ss", UTC -- and as RFC3339, and returns it in UTC.
func parseProblemTargetDate(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if t, err := time.ParseInLocation(problemTargetDateLayout, v, time.UTC); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, &apierror.ValidationError{Msg: "targetResolutionDate must be YYYY-MM-DD HH:mm:ss (UTC) or an RFC3339 timestamp"}
}

// problemPriorityFromServiceNow turns the priority ServiceNow returns for a
// problem it just created -- its display value, "5 - Planning" -- into the
// problem_priority_enum label. ServiceNow gives every new problem 5 -
// Planning (discovery script 63), which is also the fallback when the
// response carries none or an unrecognised value.
func problemPriorityFromServiceNow(display *string) string {
	if display != nil {
		switch strings.TrimSpace(*display) {
		case "1", "1 - Critical":
			return "CRITICAL"
		case "2", "2 - High":
			return "HIGH"
		case "3", "3 - Moderate":
			return "MODERATE"
		case "4", "4 - Low":
			return "LOW"
		}
	}
	return "PLANNING"
}

// newProblemPriorityFields is a new problem's impact, urgency and priority:
// ServiceNow's defaults, 3 - Low and 3 - Low, and the priority its "Priority
// Problem Lookup" derives from them.
func newProblemPriorityFields() repository.ProblemPriorityFields {
	impact, urgency := "LOW", "LOW"
	return repository.ProblemPriorityFields{Impact: impact, Urgency: urgency, Priority: priorityFromImpactUrgency(impact, urgency)}
}
