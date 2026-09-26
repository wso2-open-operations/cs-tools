package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// PairingService serves the product tab and the registrations panel.
//
// Reads here, writes in plg_pairing_writes.go. The split is not cosmetic: every
// write carries a precondition and answers with a row count, never with a
// meaning, because what a failed precondition means is a PLG rule and PLG's
// rules are the BFF's. See plg-docs/ENTITY-SERVICE-CONTRACT.md.
type PairingService interface {
	Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error)
	SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error)

	// The writes live in plg_pairing_writes.go. Embedded rather than listed
	// again so the two files cannot drift apart.
	PairingWriter
}

type pairingService struct {
	repo repository.OrgPlatformRepository
}

// NewPairingService wires a PairingService over its repository.
func NewPairingService(repo repository.OrgPlatformRepository) PairingService {
	return &pairingService{repo: repo}
}

func (s *pairingService) Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	if err := validateUUID("organizationId", orgID); err != nil {
		return nil, err
	}
	// productCode is matched against either the code or the id by the query, so
	// it is not validated as a UUID here — "API_PLATFORM" is the common case.
	return s.repo.Get(ctx, orgID, productCode)
}

func (s *pairingService) SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error) {
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.SearchRegistrationsResponse{}, err
	}
	plgNormalizePagination(&req.Pagination)

	items, total, err := s.repo.SearchRegistrations(ctx, req)
	if err != nil {
		return domain.SearchRegistrationsResponse{}, err
	}
	return domain.SearchRegistrationsResponse{
		Registrations: items,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}
