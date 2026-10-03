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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TeamMemberService answers "who holds which rank on these teams".
//
// It exists for the call-escalation ladder, which resolves four of its five
// rungs from exactly this question: LEVEL_1 and LEVEL_2 are the sub leads and
// the lead of the case's own ABT, and LEVEL_3 and LEVEL_4 are the two heads,
// who sit in a leadership team rather than an ABT. The fifth rung, LEVEL_0, is
// whoever is rostered right now and is a sub lead, which is this answer
// intersected with the on-duty read - two calls rather than one, deliberately,
// because folding rank into the rota response would change a shape the Team
// Schedule page already renders.
type TeamMemberService interface {
	MembersByTeamKeys(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) (domain.TeamMembersResponse, error)
}

type teamMemberService struct {
	repo   repository.TeamMemberRepository
	access AccessService
}

func NewTeamMemberService(repo repository.TeamMemberRepository, access AccessService) TeamMemberService {
	return &teamMemberService{repo: repo, access: access}
}

// requireInternalCaller mirrors scheduleService's helper of the same name, and
// for the same reason: this returns staff names, addresses and rank, which
// belongs to no project and so has no narrower scope a customer could be given.
// An internal caller sees all of it and anyone else sees none.
//
// The escalation resolver reaching this machine to machine is an authorized
// internal client, so it resolves unrestricted without carrying a user token.
func (s *teamMemberService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "team membership is only available to internal staff"}
	}
	return nil
}

// validTeamMemberRole is the CHECK on team_member.role (migration 000106),
// mirrored here so a typo comes back as a 400 naming the field rather than as
// an empty result the caller has to guess at. Kept as a map, the same way
// every other enum in this package is validated.
var validTeamMemberRole = map[string]bool{
	"engineer": true,
	"sub_lead": true,
	"lead":     true,
	"cre_head": true,
	"cs_head":  true,
}

func (s *teamMemberService) MembersByTeamKeys(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) (domain.TeamMembersResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.TeamMembersResponse{}, err
	}
	teamKeys = nonEmptyTrimmed(teamKeys)
	teamTypes = nonEmptyTrimmed(teamTypes)
	// One selector or the other, but never neither: an unfiltered read would
	// hand back every membership in the organisation, which is precisely the
	// staff data this endpoint's internal-caller check exists to protect.
	if len(teamKeys) == 0 && len(teamTypes) == 0 {
		return domain.TeamMembersResponse{}, &apierror.ValidationError{
			Msg: "at least one teamKey or teamType is required",
		}
	}
	roles = nonEmptyTrimmed(roles)
	for _, r := range roles {
		if !validTeamMemberRole[r] {
			return domain.TeamMembersResponse{}, &apierror.ValidationError{
				Msg: "unknown role " + r,
			}
		}
	}

	// Trimmed like every other selector. The handler only splits on commas, so
	// "T1, T2" reached the query as " T2", matched no row, and dropped that
	// nominee from the first rung -- with no error, so the missing recipient
	// was invisible.
	alertTiers = nonEmptyTrimmed(alertTiers)
	members, err := s.repo.MembersByTeamKeys(ctx, teamKeys, roles, alertTiers, teamTypes)
	if err != nil {
		return domain.TeamMembersResponse{}, err
	}
	if members == nil {
		// An empty list, never null: a team with nobody at the asked-for rank
		// is a normal answer, and the ladder treats it as a rung that reaches
		// nobody rather than as a failure.
		members = []domain.TeamMemberEntry{}
	}
	return domain.TeamMembersResponse{Members: members, Count: len(members)}, nil
}

// nonEmptyTrimmed drops blanks, which a trailing comma in a query string
// produces routinely.
func nonEmptyTrimmed(in []string) []string {
	var out []string
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
