package entityclient

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// Four of the six interfaces declare a method called Get, with four different
// signatures, so one type cannot satisfy them all. These adapters give each
// interface its own receiver over the same Client — and the same connection
// pool, timeout and correlation id.
//
// The alternative was renaming the interface methods and editing every caller.
// Four three-line wrappers cost less than that.

// ReferenceRepo adapts Client to repository.ReferenceRepository.
type ReferenceRepo struct{ *Client }

// OrganizationRepo adapts Client to repository.OrganizationRepository.
type OrganizationRepo struct{ *Client }

// OrgPlatformRepo adapts Client to repository.OrgPlatformRepository.
type OrgPlatformRepo struct{ *Client }

// PlaybookRepo adapts Client to repository.PlaybookRepository.
type PlaybookRepo struct{ *Client }

// AnalyticsRepo adapts Client to repository.AnalyticsRepository.
type AnalyticsRepo struct{ *Client }

// The disambiguating methods. Each forwards to the distinctly-named Client
// method beneath it.

func (r OrgPlatformRepo) Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	return r.Client.GetPairing(ctx, orgID, productCode)
}

func (r OrgPlatformRepo) Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) error {
	return r.Client.PatchPairing(ctx, req, actor)
}

func (r PlaybookRepo) Get(ctx context.Context, id string) (*domain.Playbook, error) {
	return r.Client.GetPlaybook(ctx, id)
}

func (r PlaybookRepo) Create(ctx context.Context, req domain.CreatePlaybookRequest) (string, error) {
	return r.Client.CreatePlaybook(ctx, req)
}

func (r PlaybookRepo) Patch(ctx context.Context, req domain.PatchPlaybookRequest) error {
	return r.Client.PatchPlaybook(ctx, req)
}

func (r PlaybookRepo) Delete(ctx context.Context, id string) error {
	return r.Client.DeletePlaybook(ctx, id)
}
