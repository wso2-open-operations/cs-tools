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

// CaseEscalationAccessService lets a CSM admin name the people who may
// de-escalate any case, besides the leads of the case's own ABT team.
type CaseEscalationAccessService interface {
	// ListDeescalators returns every grant. Admin only.
	ListDeescalators(ctx context.Context) (domain.CaseDeescalators, error)
	// GrantDeescalator names userID. Admin only; active internal staff only.
	GrantDeescalator(ctx context.Context, userID string) error
	// RevokeDeescalator removes userID. Admin only; removing a grant that is
	// not there is not an error.
	RevokeDeescalator(ctx context.Context, userID string) error
}

type caseEscalationAccessService struct {
	repo       repository.CaseEscalationAccessRepository
	access     AccessService
	adminRoles []string
}

// NewCaseEscalationAccessService constructs a CaseEscalationAccessService.
// adminRoles is AUTH_ADMIN_ROLES.
func NewCaseEscalationAccessService(repo repository.CaseEscalationAccessRepository, access AccessService, adminRoles []string) CaseEscalationAccessService {
	return &caseEscalationAccessService{repo: repo, access: access, adminRoles: adminRoles}
}

// requireAdmin is the gate: an internal caller with a user token that is a
// CSM admin (a token role from AUTH_ADMIN_ROLES) or a portal admin. It
// returns the caller's email for the audit trail.
func (s *caseEscalationAccessService) requireAdmin(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, "escalation access is only available to internal staff"); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return "", &apierror.ForbiddenError{Msg: "escalation access needs a user token, not a service credential"}
	}
	if holdsAnyRole(id.Roles, s.adminRoles) {
		return id.UserEmail, nil
	}
	portalAdmin, err := s.repo.IsPortalAdmin(ctx, id.UserEmail)
	if err != nil {
		return "", err
	}
	if !portalAdmin {
		return "", &apierror.ForbiddenError{Msg: "only a CSM admin can change who may de-escalate"}
	}
	return id.UserEmail, nil
}

// ListDeescalators implements CaseEscalationAccessService.
func (s *caseEscalationAccessService) ListDeescalators(ctx context.Context) (domain.CaseDeescalators, error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return domain.CaseDeescalators{}, err
	}
	grants, err := s.repo.ListDeescalators(ctx)
	if err != nil {
		return domain.CaseDeescalators{}, err
	}
	return domain.CaseDeescalators{Deescalators: grants}, nil
}

// GrantDeescalator implements CaseEscalationAccessService.
func (s *caseEscalationAccessService) GrantDeescalator(ctx context.Context, userID string) error {
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	ok, err := s.repo.UserIsInternal(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.ValidationError{Msg: "only active internal staff can be given the right to de-escalate, and that person is not one"}
	}
	_, err = s.repo.GrantDeescalator(ctx, userID, actor)
	return err
}

// RevokeDeescalator implements CaseEscalationAccessService.
func (s *caseEscalationAccessService) RevokeDeescalator(ctx context.Context, userID string) error {
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return err
	}
	_, err = s.repo.RevokeDeescalator(ctx, userID, actor)
	return err
}
