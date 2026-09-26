package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// OrganizationService serves the organisation list and the overview tab.
type OrganizationService interface {
	Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error)
	Get(ctx context.Context, id string) (*domain.OrganizationDetail, error)
	Patch(ctx context.Context, req domain.PatchOrganizationRequest) (*domain.OrganizationDetail, error)
}

type organizationService struct {
	repo repository.OrganizationRepository
}

// NewOrganizationService wires an OrganizationService over its repository.
func NewOrganizationService(repo repository.OrganizationRepository) OrganizationService {
	return &organizationService{repo: repo}
}

// validateOrganizationFilters checks the enums and the date range.
//
// The owner filter carries ids, so it is checked as UUIDs rather than passed
// through as free text.
func validateOrganizationFilters(f *domain.OrganizationSearchFilters) error {
	if err := validateEnums("lifecycleStage", f.LifecycleStages, domain.ValidLifecycleStage); err != nil {
		return err
	}
	if err := validateEnums("subscriptionTier", f.SubscriptionTiers, domain.ValidSubscriptionTier); err != nil {
		return err
	}
	if err := plgValidateUUIDs("filters.ownerIds", f.OwnerIDs); err != nil {
		return err
	}
	if f.RegisteredFrom != nil && f.RegisteredTo != nil && f.RegisteredTo.Before(*f.RegisteredFrom) {
		return &apierror.ValidationError{Msg: "registeredTo must not be earlier than registeredFrom"}
	}
	return nil
}

func (s *organizationService) Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error) {
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.SearchOrganizationsResponse{}, err
	}
	plgNormalizePagination(&req.Pagination)

	orgs, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchOrganizationsResponse{}, err
	}
	return domain.SearchOrganizationsResponse{
		Organizations: orgs,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

func (s *organizationService) Get(ctx context.Context, id string) (*domain.OrganizationDetail, error) {
	if err := validateUUID("organizationId", id); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

// Patch sets the organisation's CS owner, and does not judge the value.
//
// WHAT IS DELIBERATELY ABSENT. Nothing here refuses to clear an owner, even
// though an ownerless organisation is a hole — the work queue reads ownership
// to decide whose queue a pairing sits in. That is a PLG lifecycle rule, and
// lifecycle rules live in the BFF (see plg-docs/ENTITY-SERVICE-CONTRACT.md,
// S1), which rejects an empty ownerId with a 400 before this is ever called.
//
// So this will happily write a NULL owner if asked. That is not an oversight:
// entity-service's job is to store what it is told and keep referential
// integrity, and a later caller with a legitimate reason to unassign should not
// have to fight a rule that belongs to someone else's product.
func (s *organizationService) Patch(ctx context.Context, req domain.PatchOrganizationRequest) (*domain.OrganizationDetail, error) {
	if err := validateUUID("organizationId", req.ID); err != nil {
		return nil, err
	}
	if req.OwnerID != nil && *req.OwnerID != "" {
		if err := validateUUID("ownerId", *req.OwnerID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Patch(ctx, req); err != nil {
		return nil, err
	}
	// Returns the reloaded detail: the caller's response is the whole
	// organisation, and reading it back is what makes the owner's name and email
	// correct without the caller resolving them.
	return s.repo.Get(ctx, req.ID)
}
