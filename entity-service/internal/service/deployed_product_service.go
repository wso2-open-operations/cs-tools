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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type deployedProductService struct {
	repo repository.DeployedProductRepository
	// snWriteback/snMirror back CreateDeployedProduct and UpdateDeployedProduct
	// under DATA_SOURCE=postgres-servicenow-dual-write -- both nil in every
	// other mode. Set only via NewDeployedProductServiceWithSNWriteback.
	// Mirrors deploymentService's identically-named fields and the same
	// reasoning: CreateDeployedProduct calls snMirror directly, synchronously,
	// BEFORE writing to Postgres at all (see createDeployedProductSNFirst's
	// own doc comment for why create is SN-first); UpdateDeployedProduct
	// writes Postgres first and dispatches a best-effort, asynchronous mirror
	// write onto snMirror via snWriteback instead.
	snWriteback *SNWritebackDispatcher
	snMirror    DeployedProductService
}

// NewDeployedProductService constructs a DeployedProductService backed by the given repository.
func NewDeployedProductService(repo repository.DeployedProductRepository) DeployedProductService {
	return &deployedProductService{repo: repo}
}

// NewDeployedProductServiceWithSNWriteback constructs a DeployedProductService
// for DATA_SOURCE=postgres-servicenow-dual-write -- see the
// snWriteback/snMirror fields' own doc comment for what each of
// CreateDeployedProduct/UpdateDeployedProduct does with them.
func NewDeployedProductServiceWithSNWriteback(repo repository.DeployedProductRepository, dispatcher *SNWritebackDispatcher, mirror DeployedProductService) DeployedProductService {
	return &deployedProductService{repo: repo, snWriteback: dispatcher, snMirror: mirror}
}

// deployedProductSNCreator is implemented by *snDeployedProductService
// (createDeployedProductSNFirstDetails, sn_deployed_product_service.go). A
// narrow interface -- rather than adding this to the full
// DeployedProductService interface, which every implementer (including the
// plain, Postgres-backed deployedProductService itself) would then have to
// satisfy -- named for exactly what createDeployedProductSNFirst needs: the
// raw id/number/createdBy/createdOn ServiceNow assigned, not the wire-shaped
// domain.CreateDeployedProductResponse the public CreateDeployedProduct
// method returns (which has no Number field). Mirrors deploymentSNCreator
// exactly (deployment_service.go).
type deployedProductSNCreator interface {
	createDeployedProductSNFirstDetails(ctx context.Context, req domain.CreateDeployedProductRequest) (id, number, createdBy string, createdOn time.Time, err error)
}

// SearchDeployedProducts implements DeployedProductService.
//
// Deliberately reads from Postgres even under DATA_SOURCE=postgres-servicenow-dual-write
// -- see deploymentService.SearchDeployments' own doc comment for the
// reasoning: only deployed products this Postgres mirror actually knows
// about should be selectable, so case creation (whose deployed_product_id
// FK requires a matching Postgres row) can never be offered one it would
// then fail to link. A deployed product created before dual-write launched
// won't appear here until Postgres is backfilled.
func (s *deployedProductService) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}
	if err := validateUUIDs("deploymentIds", req.DeploymentIDs); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	views, total, err := s.repo.SearchDeployedProducts(ctx, req)
	if err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	return domain.SearchDeployedProductsResponse{
		DeployedProducts: views,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
		HasMore:          req.Pagination.Offset+len(views) < total,
	}, nil
}

// SearchProjectsByProductVersion implements DeployedProductService.
// deployed_product.project_id (migration 0019) is a direct FK to project,
// so unlike the ServiceNow implementation this doesn't need to page through
// deployments platform-wide to resolve the join -- the repository does it
// in one query. The same mandatoryExcludeClosureStates/
// mandatoryExcludeSubscriptionTypes policy defined alongside the ServiceNow
// implementation (sn_deployed_product_service.go) is applied here too, so
// the "can't be turned off" EOL-audience exclusion guarantee holds
// identically regardless of data source.
func (s *deployedProductService) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest) (domain.SearchProjectsByProductVersionResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productId", []string{req.ProductID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productVersionId", []string{req.ProductVersionID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}

	projects, total, err := s.repo.SearchProjectsByProductVersion(ctx, req, mandatoryExcludeClosureStates, mandatoryExcludeSubscriptionTypes)
	if err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}

	return domain.SearchProjectsByProductVersionResponse{
		Projects: projects,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(projects) < total,
	}, nil
}

// CreateDeployedProduct implements DeployedProductService. Not supported for
// the plain PostgreSQL data source (s.snMirror == nil); SN-first under
// DATA_SOURCE=postgres-servicenow-dual-write -- see createDeployedProductSNFirst.
func (s *deployedProductService) CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	if s.snMirror != nil {
		return s.createDeployedProductSNFirst(ctx, req)
	}
	return domain.CreateDeployedProductResponse{}, &apierror.ValidationError{Msg: "CreateDeployedProduct is not supported for the PostgreSQL data source"}
}

// createDeployedProductSNFirst implements CreateDeployedProduct's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly like deploymentService.createDeploymentSNFirst and for
// the same reason -- a Postgres-first create could leave a Postgres row with
// no ServiceNow counterpart if the async mirror write then failed, a
// PERMANENT orphan. Calling ServiceNow first, and only writing to Postgres
// once that succeeds, makes that orphan impossible.
//
// On success, id/number/createdBy/createdOn come from ServiceNow's own
// response and are used AS-IS for the Postgres insert
// (DeployedProductRepository.CreateDeployedProductFromServiceNow) rather than
// generated -- deployed_product.number is NOT NULL UNIQUE and Postgres has no
// generator for it, the exact same unresolved problem
// CreateDeploymentFromServiceNow already solves for deployment.number.
func (s *deployedProductService) createDeployedProductSNFirst(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	creator, ok := s.snMirror.(deployedProductSNCreator)
	if !ok {
		// Cannot happen with the real constructor (routes.go always passes a
		// *snDeployedProductService as mirror), but a fake mirror in a test
		// that doesn't implement this would otherwise panic on the type
		// assertion below instead of failing cleanly.
		return domain.CreateDeployedProductResponse{}, fmt.Errorf("deployed product SN mirror does not support create")
	}

	id, number, createdBy, createdOn, err := creator.createDeployedProductSNFirstDetails(ctx, req)
	if err != nil {
		// ServiceNow never accepted the deployed product -- nothing is
		// written to Postgres at all, by construction
		// (s.repo.CreateDeployedProductFromServiceNow is simply never called
		// on this path). No orphan gets created.
		return domain.CreateDeployedProductResponse{}, err
	}

	created, err := s.repo.CreateDeployedProductFromServiceNow(ctx, req, id, number, createdBy, createdOn)
	if err != nil {
		// ServiceNow already has the deployed product at this point -- this
		// is now real drift (ServiceNow has it, Postgres doesn't) needing
		// operator attention, not a safely-rejected request. Logged loudly
		// rather than only returned, same convention as
		// createDeploymentSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create deployed product: ServiceNow deployed product created but the Postgres insert failed",
			"deployedProductId", id, "number", number, "deploymentId", req.DeploymentID, "error", err)
		return domain.CreateDeployedProductResponse{}, err
	}

	return domain.CreateDeployedProductResponse{
		Message:         "Deployed product created successfully.",
		DeployedProduct: created,
	}, nil
}

// UpdateDeployedProduct implements DeployedProductService. Not supported for
// the plain PostgreSQL data source (s.snMirror == nil); Postgres-first with
// an asynchronous ServiceNow mirror under
// DATA_SOURCE=postgres-servicenow-dual-write.
//
// UPDATE stays Postgres-first/async -- unlike CreateDeployedProduct (see that
// method's own doc comment for why create is SN-first/synchronous instead): a
// failed async mirror write here just means ServiceNow's copy of an
// EXISTING, already-created deployed product is stale on one field until
// retried, not a permanent orphan the way a failed async CREATE would be.
// Exactly the same reasoning as deploymentService.UpdateDeployment's own
// mirror.
//
// Unlike snCaseService.UpdateCase, no narrower method is needed for the
// mirror call: snDeployedProductService.UpdateDeployedProduct's only
// pre-write side effect is a live search-based IDOR scope check when
// DeploymentID is set (verifying the product belongs to that deployment) --
// not a read-before-write dependency for merging field values, and not an
// event publish. The full request can be forwarded to it directly; it simply
// re-runs that same scope check against ServiceNow's own copy, which is
// harmless and idempotent.
func (s *deployedProductService) UpdateDeployedProduct(ctx context.Context, req domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
	if s.snMirror == nil {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "UpdateDeployedProduct is not supported for the PostgreSQL data source"}
	}

	if err := validateUpdateDeployedProductRequest(req); err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	actor, err := s.resolveActorEmail(ctx)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	updated, err := s.repo.UpdateDeployedProductFields(ctx, req, actor)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "deployed_product", req.ID, "update", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.UpdateDeployedProduct(writeCtx, req)
				return err
			},
		)
	}

	return domain.UpdateDeployedProductResponse{
		Message:         "Deployed product updated successfully.",
		DeployedProduct: updated,
	}, nil
}

// validateUpdateDeployedProductRequest holds the validation rules for
// UpdateDeployedProduct shared by both implementations -- originally only
// snDeployedProductService.UpdateDeployedProduct enforced these (UUID shape,
// exactly one of the detail fields (cores/tps/description/updates) or
// active, active can only be set to false, and each Updates entry's own
// shape); deployedProductService.UpdateDeployedProduct (the
// DATA_SOURCE=postgres-servicenow-dual-write path) skipped all of it and
// wrote straight to Postgres. Extracted here so both call the same checks
// rather than the dual-write path silently accepting a request ServiceNow
// mode would reject -- same precedent as
// deployment_service.go's validateUpdateDeploymentRequest.
func validateUpdateDeployedProductRequest(req domain.UpdateDeployedProductRequest) error {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return err
	}
	if req.DeploymentID != nil {
		if err := validateUUIDs("deploymentId", []string{*req.DeploymentID}); err != nil {
			return err
		}
	}

	hasDetailFields := req.Cores != nil || req.TPS != nil || len(req.Description) > 0 || req.Updates != nil
	if !hasDetailFields && req.Active == nil {
		return &apierror.ValidationError{Msg: "at least one of cores, tps, or description must be provided, or active must be set to false"}
	}
	if req.Active != nil && *req.Active {
		return &apierror.ValidationError{Msg: "active can only be set to false"}
	}
	if req.Active != nil && hasDetailFields {
		return &apierror.ValidationError{Msg: "cores, tps, and description must not be provided when deactivating"}
	}
	if err := validateProductUpdates(req.Updates); err != nil {
		return err
	}
	return nil
}

// resolveActorEmail resolves the caller's email from x-user-id-token, for
// stamping deployed_product.updated_by -- a plain VARCHAR audit string (an
// email, by this codebase's own convention), so unlike caseService.resolveActor
// this needs no UserRepository lookup to a full domain.User. Mirrors
// deploymentService.resolveActorEmail exactly.
func (s *deployedProductService) resolveActorEmail(ctx context.Context) (string, error) {
	email, err := callerEmail(ctx)
	if err != nil {
		return "", err
	}
	return email, nil
}

// SearchDeployedProductMetrics implements DeployedProductService, backed by
// hourly_usage_summary (migration 0054) -- see DeployedProductRepository's own doc
// comment on resolveDeployedProductNodes for how a deployed product's
// instances are resolved.
func (s *deployedProductService) SearchDeployedProductMetrics(ctx context.Context, id string, req domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error) {
	// DATA_SOURCE=postgres-servicenow-dual-write reads from ServiceNow, not
	// hourly_usage_summary/deployment_information -- those tables have no
	// row linked to a real deployment_node anywhere in this database (a
	// confirmed, environment-wide gap, not specific to any one deployment),
	// while ServiceNow has always had the complete history.
	if s.snMirror != nil {
		return s.snMirror.SearchDeployedProductMetrics(ctx, id, req)
	}

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}

	return s.repo.SearchDeployedProductMetrics(ctx, id, req.DeploymentID, req.StartDate, req.EndDate)
}

// SearchDeployedProductUsageCounts implements DeployedProductService, same
// resolution and validation as SearchDeployedProductMetrics -- and the same
// dual-write ServiceNow-read reasoning.
func (s *deployedProductService) SearchDeployedProductUsageCounts(ctx context.Context, id string, req domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error) {
	if s.snMirror != nil {
		return s.snMirror.SearchDeployedProductUsageCounts(ctx, id, req)
	}

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}

	return s.repo.SearchDeployedProductUsageCounts(ctx, id, req.DeploymentID, req.StartDate, req.EndDate)
}
