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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
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
//     name (migration 0014).
//   - SuspensionProcessState -- project.suspension_process_state (migration
//     0202), stored as sent and returned by GET/search.
//   - HasAgent/HasKbReferences -- the linked account's
//     ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
//     columns (migration 0012), the same mapping
//     ProjectRepository.GetProjectByID already reads from (see that file's
//     own doc comment for why the name doesn't match).
//
// ClosureState is recomputed by the repository in the same transaction, from the
// rule ServiceNow's "Update WSO2 Closure State" business rule applies.
//
// An allow-listed internal client (AccessService.ResolveScope is Unrestricted)
// may call without a user token; updated_by is then its client id.
type pgProjectUpdateService struct {
	repo     repository.ProjectRepository
	userRepo repository.UserRepository
	access   AccessService
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
func NewProjectUpdateService(repo repository.ProjectRepository, userRepo repository.UserRepository, access AccessService) ProjectUpdateService {
	return &pgProjectUpdateService{repo: repo, userRepo: userRepo, access: access}
}

// NewProjectUpdateServiceWithSNWriteback is NewProjectUpdateService plus the
// wiring DATA_SOURCE=postgres-servicenow-dual-write needs: after a
// successful Postgres write, UpdateProject dispatches a best-effort,
// asynchronous mirror write onto mirror through dispatcher (see
// SNWritebackDispatcher's own doc comment). mirror is never read from or
// otherwise made the active path -- reads stay on Postgres in this mode, the
// same as every other dual-write pilot (case/incident/change_request/
// problem).
func NewProjectUpdateServiceWithSNWriteback(repo repository.ProjectRepository, userRepo repository.UserRepository, access AccessService, dispatcher *SNWritebackDispatcher, mirror ProjectUpdateService) ProjectUpdateService {
	return &pgProjectUpdateService{repo: repo, userRepo: userRepo, access: access, snWriteback: dispatcher, snMirror: mirror}
}

// UpdateProject implements ProjectUpdateService.
//
// Authorizes the caller against projectID before touching anything -- a
// write has no WHERE-clause scope predicate to fold the caller's
// AccessScope into the way a scoped list/by-id read does (see
// authorizeProject's own doc comment), so without this call any caller
// who merely holds the projects:update permission (customer_admin
// included, as of the AI Assistant settings toggle) could update any
// project's settings just by knowing its UUID -- a real IDOR, caught in
// review once that permission was first granted to an external-facing
// role.
func (s *pgProjectUpdateService) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	// A nil access (only possible via direct construction -- routes.go always
	// wires a real AccessService) can't resolve anyone's scope, so it must
	// fail closed for every caller here, same as resolveUpdatedBy's own
	// identical guard on its separate no-token/M2M branch below.
	if s.access == nil {
		return domain.ProjectUpdateResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if _, err := authorizeProject(ctx, s.access, id); err != nil {
		return domain.ProjectUpdateResponse{}, err
	}
	if !hasStoredProjectFields(req) {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	updatedBy, err := s.resolveUpdatedBy(ctx)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	result, err := s.repo.UpdateProject(ctx, id, req, updatedBy)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-
	// dual-write only (snWriteback/snMirror are both nil otherwise). Postgres
	// has already committed by this point; this fires after, asynchronously,
	// and never affects this response -- the same UPDATE-stays-Postgres-
	// first/async shape as caseService.UpdateCase's own mirror (see that
	// method's own doc comment).
	if s.snWriteback != nil && s.snMirror != nil && hasStoredProjectFields(req) {
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

// resolveUpdatedBy returns the user-token caller's email, or, with no user token,
// the client id of an allow-listed internal client.
func (s *pgProjectUpdateService) resolveUpdatedBy(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		if s.access == nil {
			return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		scope, err := s.access.ResolveScope(ctx)
		if err != nil {
			return "", err
		}
		if !scope.Unrestricted {
			return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		if clientID := auth.IdentityFromContext(ctx).ClientID; clientID != "" {
			return clientID, nil
		}
		return "internal-client", nil
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

// hasStoredProjectFields reports whether req sets a field Postgres stores.
func hasStoredProjectFields(req domain.ProjectUpdateRequest) bool {
	return req.HasAgent != nil || req.HasKbReferences != nil || req.EndDateClosureState != nil ||
		req.InvoiceDueDateClosureState != nil || req.ComplianceViolationClosureState != nil ||
		req.SuspensionProcessState != nil
}
