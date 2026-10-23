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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// RotaAccessService is the Team Schedule's Rota access tab: granting and
// taking back the rota admin roles (one family each) and rota markers (one
// team each).
//
// Who may do what:
//   - a CSM admin or a portal admin grants and removes rota admin, and any
//     team's marker;
//   - a rota admin grants and removes markers for the teams of their own
//     family;
//   - a team's lead (lead or Americas team lead) grants and removes that
//     team's markers;
//   - everybody else sees an empty tab and may change nothing.
//
// A CSM admin is a CSM Portal admin: a caller whose token's "roles" claim holds
// one of AUTH_ADMIN_ROLES -- the same variable, with the same value, that
// csm-portal-backend reads to decide who its admins are. A portal admin holds
// the global admin role in the database (and is internal); the two are
// treated alike here.
type RotaAccessService interface {
	Get(ctx context.Context) (domain.RotaAccess, error)
	GrantRotaAdmin(ctx context.Context, family, userID string) error
	RevokeRotaAdmin(ctx context.Context, family, userID string) error
	GrantRotaMarker(ctx context.Context, teamKey, userID string) error
	RevokeRotaMarker(ctx context.Context, teamKey, userID string) error
}

type rotaAccessService struct {
	repo       repository.RotaAccessRepository
	schedule   repository.ScheduleRepository
	access     AccessService
	adminRoles []string
}

// NewRotaAccessService constructs a RotaAccessService. adminRoles is
// AUTH_ADMIN_ROLES.
func NewRotaAccessService(repo repository.RotaAccessRepository, schedule repository.ScheduleRepository, access AccessService, adminRoles []string) RotaAccessService {
	return &rotaAccessService{repo: repo, schedule: schedule, access: access, adminRoles: adminRoles}
}

// rotaAdminRoleOf is the stored role behind each family's rota admin.
var rotaAdminRoleOf = map[string]string{
	familyCRE: "cre_rota_admin",
	familySRE: "sre_rota_admin",
	familySME: "sme_rota_admin",
}

// rotaFamilies is every family, in the order the tab lists them.
var rotaFamilies = []string{familyCRE, familySRE, familySME}

// caller is who is asking and what they may change.
type rotaAccessCaller struct {
	email          string
	csmAdmin       bool // a CSM admin or a portal admin
	markerFamilies []string
	markerTeams    []string // teams the caller leads
}

// whoIsAsking resolves the caller's rights. Internal first, so somebody
// outside learns nothing about who holds what.
func (s *rotaAccessService) whoIsAsking(ctx context.Context) (rotaAccessCaller, error) {
	if err := RequireInternalCaller(ctx, s.access, "rota access is only available to internal staff"); err != nil {
		return rotaAccessCaller{}, err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return rotaAccessCaller{}, &apierror.ForbiddenError{Msg: "rota access needs a user token, not a service credential"}
	}
	c := rotaAccessCaller{email: id.UserEmail, markerTeams: []string{}}
	c.csmAdmin = holdsAnyRole(id.Roles, s.adminRoles)
	if !c.csmAdmin {
		portalAdmin, err := s.repo.IsPortalAdmin(ctx, id.UserEmail)
		if err != nil {
			return rotaAccessCaller{}, err
		}
		c.csmAdmin = portalAdmin
	}
	if c.csmAdmin {
		c.markerFamilies = rotaFamilies
		return c, nil
	}
	families, err := s.schedule.RotaAdminFamilies(ctx, id.UserEmail)
	if err != nil {
		return rotaAccessCaller{}, err
	}
	c.markerFamilies = families
	led, err := s.schedule.LeadTeamsFor(ctx, id.UserEmail)
	if err != nil {
		return rotaAccessCaller{}, err
	}
	c.markerTeams = led
	return c, nil
}

// holdsAnyRole is whether the token's roles include one of the wanted ones.
// Exact and case-sensitive, as csm-portal-backend matches AUTH_ADMIN_ROLES, so
// the two services can never disagree about who is an admin.
func holdsAnyRole(have, want []string) bool {
	for _, w := range want {
		if w = strings.TrimSpace(w); w == "" {
			continue
		}
		for _, h := range have {
			if h == w {
				return true
			}
		}
	}
	return false
}

// Get implements RotaAccessService. A caller who may change nothing gets the
// empty tab rather than a refusal: the page asks this to decide whether to
// show the tab at all, and "nothing for you here" is an answer, not an error.
func (s *rotaAccessService) Get(ctx context.Context) (domain.RotaAccess, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.RotaAccess{}, err
	}
	out := domain.RotaAccess{
		CanManage:  domain.RotaAccessRights{RotaAdmins: c.csmAdmin, MarkerFamilies: c.markerFamilies, MarkerTeams: c.markerTeams},
		RotaAdmins: []domain.RotaAdminGrant{},
		Markers:    []domain.RotaMarkerGrant{},
	}
	if out.CanManage.MarkerFamilies == nil {
		out.CanManage.MarkerFamilies = []string{}
	}
	if out.CanManage.MarkerTeams == nil {
		out.CanManage.MarkerTeams = []string{}
	}
	if !c.csmAdmin && len(c.markerFamilies) == 0 && len(c.markerTeams) == 0 {
		return out, nil
	}
	if out.RotaAdmins, err = s.repo.ListRotaAdmins(ctx); err != nil {
		return domain.RotaAccess{}, err
	}
	if out.Markers, err = s.repo.ListRotaMarkers(ctx, c.markerFamilies, c.markerTeams); err != nil {
		return domain.RotaAccess{}, err
	}
	return out, nil
}

// requireCSMAdmin is the gate for the rota admin roles themselves.
func (s *rotaAccessService) requireCSMAdmin(ctx context.Context) (rotaAccessCaller, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return c, err
	}
	if !c.csmAdmin {
		return c, &apierror.ForbiddenError{Msg: "only a CSM admin can grant or remove rota admin"}
	}
	return c, nil
}

// rotaAdminRole validates the family and returns its role.
func rotaAdminRole(family string) (string, error) {
	role, ok := rotaAdminRoleOf[strings.ToUpper(strings.TrimSpace(family))]
	if !ok {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf("family %q is not CRE, SRE or SME", family)}
	}
	return role, nil
}

// GrantRotaAdmin implements RotaAccessService. Only internal staff: a rota
// role never makes anyone internal, so a grant to anybody else would do
// nothing and is refused rather than stored.
func (s *rotaAccessService) GrantRotaAdmin(ctx context.Context, family, userID string) error {
	role, err := rotaAdminRole(family)
	if err != nil {
		return err
	}
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	c, err := s.requireCSMAdmin(ctx)
	if err != nil {
		return err
	}
	if err := s.requireInternalUser(ctx, userID); err != nil {
		return err
	}
	_, err = s.repo.GrantRotaAdmin(ctx, userID, role, c.email)
	return err
}

// RevokeRotaAdmin implements RotaAccessService. Removing a grant that is not
// there is not an error: the outcome the caller asked for holds.
func (s *rotaAccessService) RevokeRotaAdmin(ctx context.Context, family, userID string) error {
	role, err := rotaAdminRole(family)
	if err != nil {
		return err
	}
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	c, err := s.requireCSMAdmin(ctx)
	if err != nil {
		return err
	}
	_, err = s.repo.RevokeRotaAdmin(ctx, userID, role, c.email)
	return err
}

// requireMarkerManager is the gate for one team's markers: a CSM or portal
// admin, a rota admin for that team's family, or the team's lead. It returns
// the caller.
func (s *rotaAccessService) requireMarkerManager(ctx context.Context, teamKey string) (rotaAccessCaller, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return c, err
	}
	family, ok, err := s.repo.RosteredTeamFamily(ctx, teamKey)
	if err != nil {
		return c, err
	}
	if !ok {
		return c, &apierror.NotFoundError{Msg: fmt.Sprintf("%s is not a team on the schedule", teamKey)}
	}
	if !c.csmAdmin && !containsFold(c.markerFamilies, family) && !containsFold(c.markerTeams, teamKey) {
		return c, &apierror.ForbiddenError{
			Msg: fmt.Sprintf("a rota manager for %s is granted by a CSM admin, the %s rota admin or the team's lead", teamKey, family),
		}
	}
	return c, nil
}

// GrantRotaMarker implements RotaAccessService. A marker marks their own
// team, so the person must be a member of it, and internal staff.
func (s *rotaAccessService) GrantRotaMarker(ctx context.Context, teamKey, userID string) error {
	if strings.TrimSpace(teamKey) == "" {
		return &apierror.ValidationError{Msg: "teamKey is required"}
	}
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	c, err := s.requireMarkerManager(ctx, teamKey)
	if err != nil {
		return err
	}
	if err := s.requireInternalUser(ctx, userID); err != nil {
		return err
	}
	member, err := s.schedule.UserInTeam(ctx, userID, teamKey)
	if err != nil {
		return err
	}
	if !member {
		return &apierror.ValidationError{Msg: fmt.Sprintf("a rota manager marks their own team, and that person is not on %s", teamKey)}
	}
	_, err = s.repo.GrantRotaMarker(ctx, userID, teamKey, c.email)
	return err
}

// RevokeRotaMarker implements RotaAccessService. No membership check: a grant
// held by somebody who has since left the team is exactly what gets removed.
func (s *rotaAccessService) RevokeRotaMarker(ctx context.Context, teamKey, userID string) error {
	if strings.TrimSpace(teamKey) == "" {
		return &apierror.ValidationError{Msg: "teamKey is required"}
	}
	if err := requireGrantUserID(userID); err != nil {
		return err
	}
	c, err := s.requireMarkerManager(ctx, teamKey)
	if err != nil {
		return err
	}
	_, err = s.repo.RevokeRotaMarker(ctx, userID, teamKey, c.email)
	return err
}

// requireGrantUserID is validateUserID without its allowance for an empty id,
// which a grant always names.
func requireGrantUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return &apierror.ValidationError{Msg: "userId is required"}
	}
	return validateUserID(userID)
}

func (s *rotaAccessService) requireInternalUser(ctx context.Context, userID string) error {
	ok, err := s.repo.UserIsInternal(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.ValidationError{Msg: "rota access is only for active internal staff, and that person is not one"}
	}
	return nil
}
