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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// pgProjectUpdateService implements ProjectUpdateService against Postgres,
// for DATA_SOURCE=postgres and DATA_SOURCE=postgres-servicenow-dual-write.
//
// It covers the subset of the ServiceNow-mode PATCH /projects/{id} contract
// (domain.ProjectUpdateRequest, see snProjectUpdateService.UpdateProject's
// own doc comment for that contract) that has a real Postgres column to
// write to:
//   - EndDateClosureState/InvoiceDueDateClosureState/
//     ComplianceViolationClosureState -- project table columns of the same
//     name (migration 000009).
//   - HasAgent/HasKbReferences -- the linked account's
//     ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
//     columns (migration 000008), the same mapping
//     ProjectRepository.GetProjectByID already reads from (see that file's
//     own doc comment for why the name doesn't match).
//
// SuspensionProcessState has no Postgres column anywhere for project:
// confirmed against every migration file and cross-checked against a live
// local dev schema on 2026-09-24. account.suspension_process_state
// (migration 000015) is a same-named but unrelated column for the
// account's own ACP suspension flow, not project's (ServiceNow's project
// suspensionProcessState is a project-level field there, per
// sn_project_service.go). A request naming it is rejected with a
// ValidationError rather than silently dropped.
//
// ClosureState (the derived overall status) is left untouched: it is
// recomputed in ServiceNow by an SN Flow Designer business rule this
// codebase does not reimplement, so this mode never writes
// project.wso2_closure_state and the response never claims a freshly
// derived ClosureState value (always nil here, unlike ServiceNow mode's
// response).
type pgProjectUpdateService struct {
	repo     repository.ProjectRepository
	userRepo repository.UserRepository
	// snWriteback/snMirror back UpdateProject's best-effort, asynchronous
	// ServiceNow mirror write under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode (plain DATA_SOURCE=postgres never
	// touches ServiceNow at all). Set only via
	// NewProjectUpdateServiceWithSNWriteback. snMirror's UpdateProject
	// (snProjectUpdateService, sn_project_service.go) is already a bare
	// PATCH with no read-before-write or side effects beyond the SN PATCH
	// itself (see that method's own doc comment) -- unlike case's
	// patchCaseFields/CreateBareCaseComment, no separate narrow "bare"
	// variant was needed here.
	snWriteback *SNWritebackDispatcher
	snMirror    ProjectUpdateService
}

// NewProjectUpdateService constructs a ProjectUpdateService backed by
// Postgres alone (DATA_SOURCE=postgres) -- it never touches ServiceNow.
func NewProjectUpdateService(repo repository.ProjectRepository, userRepo repository.UserRepository) ProjectUpdateService {
	return &pgProjectUpdateService{repo: repo, userRepo: userRepo}
}

// NewProjectUpdateServiceWithSNWriteback is NewProjectUpdateService plus the
// wiring DATA_SOURCE=postgres-servicenow-dual-write needs: after a
// successful Postgres write, UpdateProject dispatches a best-effort,
// asynchronous mirror write onto mirror through dispatcher (see
// SNWritebackDispatcher's own doc comment). mirror is never read from or
// otherwise made the active path -- reads stay on Postgres in this mode, the
// same as every other dual-write pilot (case/incident/change_request/
// problem).
func NewProjectUpdateServiceWithSNWriteback(repo repository.ProjectRepository, userRepo repository.UserRepository, dispatcher *SNWritebackDispatcher, mirror ProjectUpdateService) ProjectUpdateService {
	return &pgProjectUpdateService{repo: repo, userRepo: userRepo, snWriteback: dispatcher, snMirror: mirror}
}

// UpdateProject implements ProjectUpdateService.
func (s *pgProjectUpdateService) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	// SuspensionProcessState is rejected outright rather than silently
	// dropped -- see this type's own doc comment for why no Postgres column
	// exists for it.
	if req.SuspensionProcessState != nil {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "suspensionProcessState is not supported for this data source"}
	}
	if req.HasAgent == nil && req.HasKbReferences == nil && req.EndDateClosureState == nil &&
		req.InvoiceDueDateClosureState == nil && req.ComplianceViolationClosureState == nil {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	// updated_by is the caller's own resolved email, the same
	// token->email->user-lookup convention CommentService.CreateComment uses
	// for comment.created_by (see that method's own doc comment) -- never a
	// caller-supplied value, so this can't be used to attribute a change to
	// someone else.
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.ProjectUpdateResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	result, err := s.repo.UpdateProject(ctx, id, req, user.Email)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-
	// dual-write only (snWriteback/snMirror are both nil otherwise). Postgres
	// has already committed by this point; this fires after, asynchronously,
	// and never affects this response -- the same UPDATE-stays-Postgres-
	// first/async shape as caseService.UpdateCase's own mirror (see that
	// method's own doc comment). req is forwarded as-is: SuspensionProcessState
	// is already known nil (rejected above), so the mirror can never carry a
	// field this mode didn't actually persist.
	if s.snWriteback != nil && s.snMirror != nil {
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "project", id, "update", mirrorReq, func(writeCtx context.Context) error {
			_, err := s.snMirror.UpdateProject(writeCtx, id, mirrorReq)
			return err
		})
	}

	return domain.ProjectUpdateResponse{
		Message: "Project updated successfully",
		Project: result,
	}, nil
}
