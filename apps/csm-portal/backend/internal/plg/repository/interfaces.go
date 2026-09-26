// Package repository declares the data-access interfaces the service layer
// depends on. It holds no implementations.
//
// The data itself lives behind entity-service, so every method here is served
// over HTTP by internal/entityclient rather than by a connection pool. The
// services above are written against these interfaces and neither know nor care
// which it is.
//
// Keeping the interfaces here rather than beside their consumers is a
// deliberate departure from the Go idiom of consumer-defined interfaces: they
// are one contract, read as a set, and the services were written against them
// as a set.
package repository

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// ReferenceRepository serves the data that changes only with a migration.
type ReferenceRepository interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	ListCSUsers(ctx context.Context) ([]domain.UserRef, error)
	GetCSUser(ctx context.Context, email string) (*domain.UserRef, error)
	LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error)
}

// OrganizationRepository serves the organisation list and the overview tab.
type OrganizationRepository interface {
	Search(ctx context.Context, req domain.SearchOrganizationsRequest) ([]domain.OrganizationSummary, int, error)
	Get(ctx context.Context, id string) (*domain.OrganizationDetail, error)
	Patch(ctx context.Context, req domain.PatchOrganizationRequest) error
}

// OrgPlatformRepository serves the product tab.
type OrgPlatformRepository interface {
	Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error)
	Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) error
	Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actor string) error

	AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actor string) error
	DetachRun(ctx context.Context, runID string) (orgID, productCode string, err error)
	PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actor string) (orgID, productCode string, err error)
	RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error)

	CreateNote(ctx context.Context, req domain.CreateNoteRequest, actor string) error
	UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actor string) (orgID, productCode string, err error)
	SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error)
	LocatePairing(ctx context.Context, orgPlatformID string) (orgID, productCode string, err error)
}

// PlaybookRepository serves the playbook templates.
type PlaybookRepository interface {
	ListAll(ctx context.Context) ([]domain.Playbook, error)
	ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error)
	ListForStage(ctx context.Context, productID string, stage domain.LifecycleStage, kinds []domain.PlaybookType) ([]domain.Playbook, error)
	Get(ctx context.Context, id string) (*domain.Playbook, error)
	Create(ctx context.Context, req domain.CreatePlaybookRequest) (string, error)
	Patch(ctx context.Context, req domain.PatchPlaybookRequest) error
	ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) error
	Delete(ctx context.Context, id string) error
}

// AnalyticsRepository serves the dashboard and the work queue.
type AnalyticsRepository interface {
	Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error)
	WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error)
}
