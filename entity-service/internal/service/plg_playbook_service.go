package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// PlaybookService serves the playbook templates.
//
// The bookend rules, task-code normalisation and option validation are NOT in
// this service — they live in the BFF. See plg_playbook_writes.go.
type PlaybookService interface {
	List(ctx context.Context, productCode string) ([]domain.Playbook, error)
	Get(ctx context.Context, id string) (*domain.Playbook, error)

	// The writes live in plg_playbook_writes.go.
	PlaybookWriter
}

type playbookService struct {
	repo repository.PlaybookRepository
}

// NewPlaybookService wires a PlaybookService over its repository.
func NewPlaybookService(repo repository.PlaybookRepository) PlaybookService {
	return &playbookService{repo: repo}
}

// List returns every playbook, or just one product's.
//
// An empty productCode means "all" — GET /playbooks with no query parameter.
// The two cases are separate repository methods rather than one with an
// optional filter.
func (s *playbookService) List(ctx context.Context, productCode string) ([]domain.Playbook, error) {
	if productCode == "" {
		return s.repo.ListAll(ctx)
	}
	return s.repo.ListByProduct(ctx, productCode)
}

func (s *playbookService) Get(ctx context.Context, id string) (*domain.Playbook, error) {
	if err := validateUUID("playbookId", id); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}
