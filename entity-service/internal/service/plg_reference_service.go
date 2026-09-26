package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ReferenceService serves PLG's own vocabulary: the platforms and the
// lifecycle catalogue.
type ReferenceService interface {
	ListProducts(ctx context.Context) ([]domain.PlgProduct, error)
	LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error)
}

type referenceService struct {
	repo repository.ReferenceRepository
}

// NewReferenceService wires a ReferenceService over its repository.
func NewReferenceService(repo repository.ReferenceRepository) ReferenceService {
	return &referenceService{repo: repo}
}

// Both methods are straight delegation: there is nothing to validate on a
// request with no input, and nothing to decide about reference data. The layer
// exists so that these two sit where every other entity's queries sit.
func (s *referenceService) ListProducts(ctx context.Context) ([]domain.PlgProduct, error) {
	return s.repo.ListProducts(ctx)
}

func (s *referenceService) LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error) {
	return s.repo.LifecycleCatalogue(ctx)
}
