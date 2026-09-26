package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AnalyticsService serves the dashboard and the work queue.
type AnalyticsService interface {
	Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error)
	WorkQueue(ctx context.Context, req domain.SearchWorkQueueRequest) (*domain.WorkQueueResponse, error)
}

type analyticsService struct {
	repo repository.AnalyticsRepository
}

// NewAnalyticsService wires an AnalyticsService over its repository.
func NewAnalyticsService(repo repository.AnalyticsRepository) AnalyticsService {
	return &analyticsService{repo: repo}
}

func (s *analyticsService) Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error) {
	return s.repo.Dashboard(ctx, rng)
}

// Valid queue reasons, mirroring the CASE expression in plg_work_queue_v.
//
// The view derives the reason; this map is what stops a filter asking for one
// the view can never produce. Kept in step by hand — the view is the source of
// truth and a new branch there needs an entry here, which is the same
// arrangement entity-service has between its enums and its validXxx maps.
var validQueueReason = map[domain.QueueReason]bool{
	domain.ReasonInProgress: true,
	domain.ReasonNotStarted: true,
	domain.ReasonNoPlaybook: true,
}

func (s *analyticsService) WorkQueue(ctx context.Context, req domain.SearchWorkQueueRequest) (*domain.WorkQueueResponse, error) {
	f := req.Filters
	if err := plgValidateUUIDs("filters.ownerIds", f.OwnerIDs); err != nil {
		return nil, err
	}
	if err := plgValidateUUIDs("filters.organizationIds", f.OrganizationIDs); err != nil {
		return nil, err
	}
	if err := plgValidateUUIDs("filters.playbookIds", f.PlaybookIDs); err != nil {
		return nil, err
	}
	if err := validateEnums("filters.lifecycleStages", f.LifecycleStages, domain.ValidLifecycleStage); err != nil {
		return nil, err
	}
	if err := validateEnums("filters.reasons", f.Reasons, validQueueReason); err != nil {
		return nil, err
	}
	return s.repo.WorkQueue(ctx, f.ToFilters())
}
