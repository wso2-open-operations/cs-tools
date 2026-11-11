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
)

// A rota admin runs one family's Team Schedule -- CRE, SRE or SME -- and
// changes that family's only. Everyone reads every family; the writes hold
// the line (requireRotaWriter: a rota admin edits their own family's teams),
// and so do the absence tags: a rota admin's new tag is their family's, and
// they delete only their family's.

// rotaAdminFamilies is the families of the rota admin roles the caller holds;
// none for a caller with no user email or no such role.
func (s *scheduleService) rotaAdminFamilies(ctx context.Context) ([]string, error) {
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return nil, nil
	}
	admin, err := s.repo.RotaAdminFamilies(ctx, email)
	if err != nil || len(admin) == 0 {
		return nil, err
	}
	return admin, nil
}

// tagFamilies is the families whose tags the caller adds and deletes: a rota
// admin's own families; else none, with portalAdmin set, for a portal admin
// (who is not limited to a family, even when they also lead a rota team);
// else the families of the rota teams they lead. A tag belongs to one family
// (a CRE ABT lead's tag is CRE's), so it is offered only on that family's
// rosters.
func (s *scheduleService) tagFamilies(ctx context.Context) (families []string, portalAdmin bool, err error) {
	admin, err := s.rotaAdminFamilies(ctx)
	if err != nil || len(admin) > 0 {
		return admin, false, err
	}
	if portalAdmin, err = s.isPortalAdmin(ctx); err != nil || portalAdmin {
		return nil, portalAdmin, err
	}
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return nil, false, nil
	}
	led, err := s.repo.LeadFamiliesFor(ctx, email)
	if err != nil || len(led) == 0 {
		return nil, false, err
	}
	return led, false, nil
}

// kindFamilyFor is the family a new absence kind is stored under: none (shared)
// for a caller who is no rota admin, else the admin's family -- the requested
// one, which must be theirs, or their only one.
func kindFamilyFor(admin []string, requested string) (*string, error) {
	if len(admin) == 0 {
		return nil, nil
	}
	requested = strings.ToUpper(strings.TrimSpace(requested))
	if requested != "" {
		if !containsFold(admin, requested) {
			return nil, &apierror.ForbiddenError{Msg: fmt.Sprintf("a rota admin can only add tags for their own family, not %s", requested)}
		}
		return &requested, nil
	}
	if len(admin) > 1 {
		return nil, &apierror.ValidationError{Msg: "you run more than one family's rota: say which family the tag is for (family)"}
	}
	f := strings.ToUpper(admin[0])
	return &f, nil
}

// errNoTagFamily refuses a tag change from a caller with no family of their
// own: someone who leads only a team that works no rota (a change approval
// board, say) passes the "any lead" gate, but a tag belongs to a family, so
// only a portal admin may add or delete one without a family.
var errNoTagFamily = &apierror.ForbiddenError{Msg: "only a rota admin or the lead of a rota team can change tags"}

// isPortalAdmin is whether the caller is a portal admin: an admin role on the
// token (AUTH_ADMIN_ROLES), or the database admin role, which
// requireAnyTeamLead also admits (through RotaAdminTeamsFor) and the token's
// roles may not name.
func (s *scheduleService) isPortalAdmin(ctx context.Context) (bool, error) {
	if s.isCSMAdmin(ctx) {
		return true, nil
	}
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return false, nil
	}
	return s.repo.IsPortalAdmin(ctx, email)
}

// portalAdminTagFamily is what a caller who neither runs nor leads a rota (a
// portal admin) may add a tag for: the family they name, or none (a shared
// tag) when they name none. A family that is not CRE, SRE or SME is refused,
// so a mistyped one never becomes a tag every family is offered.
func portalAdminTagFamily(requested string) ([]string, error) {
	switch f := strings.ToUpper(strings.TrimSpace(requested)); f {
	case "":
		return nil, nil
	case "CRE", "SRE", "SME":
		return []string{f}, nil
	default:
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("family must be CRE, SRE or SME, not %s", strings.TrimSpace(requested))}
	}
}
