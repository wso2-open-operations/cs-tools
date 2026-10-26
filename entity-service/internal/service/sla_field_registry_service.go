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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// requireSLAAdmin is the one gate every SLA admin-config service in this
// package applies before touching anything: an internal caller (never a
// customer/unresolved identity, same RequireInternalCaller primitive
// GET /sla-status and the Case Alert Settings tab already use) holding one
// of AUTH_ADMIN_ROLES (the same variable csm-portal-backend's own admin
// check reads, so the two can never disagree about who is an admin). SLA
// configuration has no per-project scoping concept -- it is global, so
// there is no finer-grained authority to fall back to the way rota access
// does for its own non-admin callers.
func requireSLAAdmin(ctx context.Context, access AccessService, adminRoles []string) error {
	if err := RequireInternalCaller(ctx, access, "SLA configuration is only available to internal staff"); err != nil {
		return err
	}
	id := auth.IdentityFromContext(ctx)
	if !holdsAnyRole(id.Roles, adminRoles) {
		return &apierror.ForbiddenError{Msg: "SLA configuration is admin-only"}
	}
	return nil
}

// slaActorEmail resolves the caller's email for created_by/updated_by
// stamping -- requireSLAAdmin already guarantees a user token (an M2M
// caller has no roles claim to hold an admin role at all), so this is
// never empty once that gate has passed.
func slaActorEmail(ctx context.Context) string {
	return auth.IdentityFromContext(ctx).UserEmail
}

// SLAFieldRegistryService is the admin-facing surface of the SLA module's
// field registry (R5, R16) -- GET /sla/work-item-types, GET
// /sla/field-catalog, PATCH /sla/field-catalog/{id}/enablement.
type SLAFieldRegistryService interface {
	ListWorkItemTypes(ctx context.Context) ([]domain.SLAWorkItemType, error)
	ListFieldCatalog(ctx context.Context, workItemType string) ([]domain.SLAFieldCatalogEntry, error)
	SetFieldEnabled(ctx context.Context, fieldCatalogEntryID string, req domain.UpdateSLAFieldEnablementRequest) (domain.SLAFieldCatalogEntry, error)
}

type slaFieldRegistryService struct {
	repo       repository.SLAFieldRegistryRepository
	access     AccessService
	adminRoles []string
}

// NewSLAFieldRegistryService constructs an SLAFieldRegistryService.
// adminRoles is AUTH_ADMIN_ROLES.
func NewSLAFieldRegistryService(repo repository.SLAFieldRegistryRepository, access AccessService, adminRoles []string) SLAFieldRegistryService {
	return &slaFieldRegistryService{repo: repo, access: access, adminRoles: adminRoles}
}

func (s *slaFieldRegistryService) ListWorkItemTypes(ctx context.Context) ([]domain.SLAWorkItemType, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return nil, err
	}
	return s.repo.ListWorkItemTypes(ctx)
}

func (s *slaFieldRegistryService) ListFieldCatalog(ctx context.Context, workItemType string) ([]domain.SLAFieldCatalogEntry, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return nil, err
	}
	return s.repo.ListFieldCatalog(ctx, workItemType)
}

func (s *slaFieldRegistryService) SetFieldEnabled(ctx context.Context, fieldCatalogEntryID string, req domain.UpdateSLAFieldEnablementRequest) (domain.SLAFieldCatalogEntry, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLAFieldCatalogEntry{}, err
	}
	if fieldCatalogEntryID == "" {
		return domain.SLAFieldCatalogEntry{}, &apierror.ValidationError{Msg: "fieldCatalogEntryId is required"}
	}
	if err := validateUUIDs("fieldCatalogEntryId", []string{fieldCatalogEntryID}); err != nil {
		return domain.SLAFieldCatalogEntry{}, err
	}
	return s.repo.SetFieldEnabled(ctx, fieldCatalogEntryID, req.Enabled, slaActorEmail(ctx))
}
