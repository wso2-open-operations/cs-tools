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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// customerWritableCaseStates are the state transitions a customer contact may
// make on their own case: hand the case back to the engineers, reopen it, or
// close it. Every other state (open, work_in_progress, awaiting_info,
// solution_proposed) is an engineer-side transition.
var customerWritableCaseStates = map[domain.CaseState]bool{
	domain.CaseStateWaitingOnWSO2: true,
	domain.CaseStateReopened:      true,
	domain.CaseStateClosed:        true,
}

// internalOnlyCaseUpdateFields names the fields of req that only an internal
// user may change. The split is:
//
//   - customer-writable: subject, description, deploymentId,
//     deployedProductId, watchList (restricted further to registered project
//     contacts -- see validateWatchListProjectMembership), state limited to
//     customerWritableCaseStates together with the resolutionCode / cause /
//     closeNotes that accompany a close;
//   - internal-only: severity, workState, assigneeEmail, parentId,
//     acknowledge, markFixIssued, the three fix-ETA fields, relatedCaseId,
//     workaroundProvided, and any state outside customerWritableCaseStates.
//
// The returned names are the request's own JSON field names, so the 403 the
// caller builds from them reads like every other field-level error here.
func internalOnlyCaseUpdateFields(req domain.UpdateCaseRequest) []string {
	var fields []string
	if req.State != nil && !customerWritableCaseStates[*req.State] {
		fields = append(fields, "state "+string(*req.State))
	}
	if req.Severity != nil {
		fields = append(fields, "severity")
	}
	if req.WorkState != nil {
		fields = append(fields, "workState")
	}
	if len(req.AssigneeEmail) > 0 {
		fields = append(fields, "assigneeEmail")
	}
	if req.ParentID != nil {
		fields = append(fields, "parentId")
	}
	if req.Acknowledge != nil {
		fields = append(fields, "acknowledge")
	}
	if req.MarkFixIssued != nil {
		fields = append(fields, "markFixIssued")
	}
	if req.BestCaseFixEta != nil {
		fields = append(fields, "bestCaseFixEta")
	}
	if req.MostLikelyFixEta != nil {
		fields = append(fields, "mostLikelyFixEta")
	}
	if req.WorstCaseFixEta != nil {
		fields = append(fields, "worstCaseFixEta")
	}
	if req.RelatedCaseID != nil {
		fields = append(fields, "relatedCaseId")
	}
	if req.WorkaroundProvided != nil {
		fields = append(fields, "workaroundProvided")
	}
	return fields
}

// withoutWorkNotes drops work notes from a comment page served to a caller
// whose scope is not Unrestricted. The row-level policy on the comment table
// hides them at the source; this keeps the service correct on its own as
// well, so a read path never depends on the policy alone.
func withoutWorkNotes(comments []domain.CaseComment) []domain.CaseComment {
	out := comments[:0]
	for _, c := range comments {
		if c.Type == domain.CommentTypeWorkNote {
			continue
		}
		out = append(out, c)
	}
	return out
}

// withoutWorkNoteActivity is withoutWorkNotes for the activity feed, whose
// comment entries carry the comment type in CommentType.
func withoutWorkNoteActivity(activity []domain.CaseActivity) []domain.CaseActivity {
	out := activity[:0]
	for _, a := range activity {
		if a.CommentType != nil && *a.CommentType == domain.CommentTypeWorkNote {
			continue
		}
		out = append(out, a)
	}
	return out
}

// callerScope resolves the caller's AccessScope for this service (see
// resolveCallerScope).
func (s *caseService) callerScope(ctx context.Context) (AccessScope, error) {
	return resolveCallerScope(ctx, s.access)
}

// requireInternalCaller refuses a caller whose scope is not Unrestricted with
// forbiddenMsg. Used by the engineer-side case mutations that live outside
// UpdateCase (tag edits).
func (s *caseService) requireInternalCaller(ctx context.Context, forbiddenMsg string) error {
	scope, err := s.callerScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: forbiddenMsg}
	}
	return nil
}

// resolveMutationActorEmail returns the e-mail a state/severity/workState
// update is attributed to. A request that carries a user token must resolve
// to a known platform user (the same requirement every other UpdateCase
// branch has always had); a request with no user token is accepted only when
// its scope is Unrestricted on the client credential alone (an allow-listed
// internal client), and is then recorded with no user attribution -- never
// best-effort for anyone else.
func (s *caseService) resolveMutationActorEmail(ctx context.Context, scope AccessScope) (string, error) {
	if middleware.UserIDTokenFromContext(ctx) == "" {
		if !scope.Unrestricted {
			return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		return "", nil
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return "", err
	}
	return actor.Email, nil
}
