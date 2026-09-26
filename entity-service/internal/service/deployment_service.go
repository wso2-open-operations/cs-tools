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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type deploymentService struct {
	repo repository.DeploymentRepository
	// snWriteback/snMirror back CreateDeployment and UpdateDeployment under
	// DATA_SOURCE=postgres-servicenow-dual-write -- both nil in every other
	// mode. Set only via NewDeploymentServiceWithSNWriteback. Mirrors
	// caseService's identically-named fields and the same reasoning:
	// CreateDeployment calls snMirror directly, synchronously, BEFORE
	// writing to Postgres at all (see createDeploymentSNFirst's own doc
	// comment for why create is SN-first); UpdateDeployment writes Postgres
	// first and dispatches a best-effort, asynchronous mirror write onto
	// snMirror via snWriteback instead.
	snWriteback *SNWritebackDispatcher
	snMirror    DeploymentService
}

// NewDeploymentService constructs a DeploymentService backed by the given repository.
func NewDeploymentService(repo repository.DeploymentRepository) DeploymentService {
	return &deploymentService{repo: repo}
}

// NewDeploymentServiceWithSNWriteback constructs a DeploymentService for
// DATA_SOURCE=postgres-servicenow-dual-write -- see the snWriteback/snMirror
// fields' own doc comment for what each of CreateDeployment/UpdateDeployment
// does with them.
func NewDeploymentServiceWithSNWriteback(repo repository.DeploymentRepository, dispatcher *SNWritebackDispatcher, mirror DeploymentService) DeploymentService {
	return &deploymentService{repo: repo, snWriteback: dispatcher, snMirror: mirror}
}

// deploymentSNCreator is implemented by *snDeploymentService
// (createDeploymentSNFirstDetails, sn_deployment_service.go). A narrow
// interface — rather than adding this to the full DeploymentService
// interface, which every implementer (including the plain, Postgres-backed
// deploymentService itself) would then have to satisfy — named for exactly
// what createDeploymentSNFirst needs: the raw id/number/createdBy/createdOn
// ServiceNow assigned, not the wire-shaped domain.CreateDeploymentResponse
// the public CreateDeployment method returns (which has no Number field).
type deploymentSNCreator interface {
	createDeploymentSNFirstDetails(ctx context.Context, req domain.CreateDeploymentRequest) (id, number, createdBy string, createdOn time.Time, err error)
}

// CreateDeployment implements DeploymentService. Not supported for the plain
// PostgreSQL data source (s.snMirror == nil); SN-first under
// DATA_SOURCE=postgres-servicenow-dual-write -- see createDeploymentSNFirst.
func (s *deploymentService) CreateDeployment(ctx context.Context, req domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	if s.snMirror != nil {
		return s.createDeploymentSNFirst(ctx, req)
	}
	return domain.CreateDeploymentResponse{}, &apierror.ValidationError{Msg: "CreateDeployment is not supported for the PostgreSQL data source"}
}

// createDeploymentSNFirst implements CreateDeployment's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly like caseService.createCaseSNFirst and for the same
// reason — a Postgres-first create could leave a Postgres row with no
// ServiceNow counterpart if the async mirror write then failed, a PERMANENT
// orphan (deployed products and other data would go on attaching to a
// deployment ServiceNow never heard of). Calling ServiceNow first, and only
// writing to Postgres once that succeeds, makes that orphan impossible.
//
// On success, id/number/createdBy/createdOn come from ServiceNow's own
// response and are used AS-IS for the Postgres insert
// (DeploymentRepository.CreateDeploymentFromServiceNow) rather than
// generated — deployment.number is NOT NULL UNIQUE and Postgres has no
// generator for it, the exact same unresolved problem
// CreateCaseFromServiceNow already solves for case.number.
func (s *deploymentService) createDeploymentSNFirst(ctx context.Context, req domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	creator, ok := s.snMirror.(deploymentSNCreator)
	if !ok {
		// Cannot happen with the real constructor (routes.go always passes a
		// *snDeploymentService as mirror), but a fake mirror in a test that
		// doesn't implement this would otherwise panic on the type assertion
		// below instead of failing cleanly.
		return domain.CreateDeploymentResponse{}, fmt.Errorf("deployment SN mirror does not support create")
	}

	id, number, createdBy, createdOn, err := creator.createDeploymentSNFirstDetails(ctx, req)
	if err != nil {
		// ServiceNow never accepted the deployment — nothing is written to
		// Postgres at all, by construction (s.repo.CreateDeploymentFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateDeploymentResponse{}, err
	}

	created, err := s.repo.CreateDeploymentFromServiceNow(ctx, req, id, number, createdBy, createdOn)
	if err != nil {
		// ServiceNow already has the deployment at this point — this is now
		// real drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather
		// than only returned, since nothing else records this particular
		// failure shape -- same as createCaseSNFirst's equivalent
		// Postgres-insert-failure path.
		slog.ErrorContext(ctx, "sn create deployment: ServiceNow deployment created but the Postgres insert failed",
			"deploymentId", id, "number", number, "projectId", req.ProjectID, "error", err)
		return domain.CreateDeploymentResponse{}, err
	}

	return domain.CreateDeploymentResponse{
		Message:    "Deployment created successfully.",
		Deployment: created,
	}, nil
}

// UpdateDeployment implements DeploymentService. Not supported for the plain
// PostgreSQL data source (s.snMirror == nil); Postgres-first with an
// asynchronous ServiceNow mirror under
// DATA_SOURCE=postgres-servicenow-dual-write.
//
// UPDATE stays Postgres-first/async — unlike CreateDeployment (see that
// method's own doc comment for why create is SN-first/synchronous instead):
// a failed async mirror write here just means ServiceNow's copy of an
// EXISTING, already-created deployment is stale on one field until retried,
// not a permanent orphan the way a failed async CREATE would be. Exactly the
// same reasoning as caseService.UpdateCase's own State/Severity/WorkState
// mirror.
//
// Unlike UpdateCase, no narrower method is needed for the mirror call: the
// full snDeploymentService.UpdateDeployment has no read-before-write or
// event-publish side effects to avoid duplicating (see its own doc
// comment) — the same request can be forwarded to it directly.
func (s *deploymentService) UpdateDeployment(ctx context.Context, req domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	if s.snMirror == nil {
		return domain.UpdateDeploymentResponse{}, &apierror.ValidationError{Msg: "UpdateDeployment is not supported for the PostgreSQL data source"}
	}

	if err := validateUpdateDeploymentRequest(req); err != nil {
		return domain.UpdateDeploymentResponse{}, err
	}

	actor, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.UpdateDeploymentResponse{}, err
	}

	updated, err := s.repo.UpdateDeploymentFields(ctx, req, actor)
	if err != nil {
		return domain.UpdateDeploymentResponse{}, err
	}

	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "deployment", req.ID, "update", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.UpdateDeployment(writeCtx, req)
				return err
			},
		)
	}

	return domain.UpdateDeploymentResponse{
		Message:    "Deployment updated successfully.",
		Deployment: updated,
	}, nil
}

// validateUpdateDeploymentRequest holds the validation rules for
// UpdateDeployment shared by both implementations -- originally only
// snDeploymentService.UpdateDeployment enforced these (UUID shape, exactly
// one of the detail fields or active, a known type, active can only be set
// to false); deploymentService.UpdateDeployment (the
// DATA_SOURCE=postgres-servicenow-dual-write path) skipped all of it and
// wrote straight to Postgres. Extracted here so both call the same checks
// rather than the dual-write path silently accepting a request ServiceNow
// mode would reject.
func validateUpdateDeploymentRequest(req domain.UpdateDeploymentRequest) error {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return err
	}

	hasDetailFields := req.Name != nil || req.Type != nil || req.Description != nil
	if !hasDetailFields && req.Active == nil {
		return &apierror.ValidationError{Msg: "at least one of name, type, description, or active must be provided"}
	}
	if hasDetailFields && req.Active != nil {
		return &apierror.ValidationError{Msg: "active must not be provided when updating deployment details"}
	}
	if req.Type != nil {
		if _, ok := validDeploymentTypes[*req.Type]; !ok {
			return &apierror.ValidationError{Msg: fmt.Sprintf("invalid type %q", *req.Type)}
		}
	}
	if req.Active != nil && *req.Active {
		return &apierror.ValidationError{Msg: "active can only be set to false"}
	}
	return nil
}

// resolveActorEmail resolves the caller's email from x-user-id-token, for
// stamping deployment.updated_by -- a plain VARCHAR audit string (an email,
// by this codebase's own convention; see DeploymentRepository's own doc
// comment), so unlike caseService.resolveActor this needs no UserRepository
// lookup to a full domain.User.
func (s *deploymentService) resolveActorEmail(ctx context.Context) (string, error) {
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

// SearchDeployments implements DeploymentService.
func (s *deploymentService) SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}
	if err := validateUUIDs("projectIds", req.ProjectIDs); err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}

	views, total, err := s.repo.SearchDeployments(ctx, req)
	if err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}

	return domain.SearchDeploymentsResponse{
		Deployments: views,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(views) < total,
	}, nil
}
