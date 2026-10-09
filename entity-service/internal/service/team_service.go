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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TeamService defines the operations available on the team entity. Postgres
// only -- there is no ServiceNow-backed counterpart of this service in this
// codebase; SPL's own ABT-team-members feature has always called ServiceNow
// directly from csm-portal-backend rather than through entity-service, so
// this is a new capability, not a migrated one.
type TeamService interface {
	// GetTeamMembers returns the roster of the given team. A NotFoundError is
	// returned when no team with that id exists; an empty roster on an
	// existing team is not an error.
	GetTeamMembers(ctx context.Context, teamID string) (domain.GetTeamMembersResponse, error)
	// SearchTeams returns a paginated list of teams from the `team` table,
	// filtered by an optional name search.
	SearchTeams(ctx context.Context, req domain.SearchTeamsRequest) (domain.SearchTeamsResponse, error)
}

type teamService struct {
	repo repository.TeamRepository
}

// NewTeamService constructs a TeamService backed by the given repository.
func NewTeamService(repo repository.TeamRepository) TeamService {
	return &teamService{repo: repo}
}

func (s *teamService) GetTeamMembers(ctx context.Context, teamID string) (domain.GetTeamMembersResponse, error) {
	// The roster includes member names and emails, so an anonymous caller must
	// not reach it -- auth.Middleware itself lets a tokenless request through
	// (per-endpoint decides), so this check is the only thing standing between
	// the roster and an unauthenticated caller.
	if middleware.UserIDTokenFromContext(ctx) == "" {
		return domain.GetTeamMembersResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if err := validateUUIDs("id", []string{teamID}); err != nil {
		return domain.GetTeamMembersResponse{}, err
	}
	exists, err := s.repo.TeamExists(ctx, teamID)
	if err != nil {
		return domain.GetTeamMembersResponse{}, err
	}
	if !exists {
		return domain.GetTeamMembersResponse{}, &apierror.NotFoundError{Msg: "team not found"}
	}
	rows, err := s.repo.GetTeamMembers(ctx, teamID)
	if err != nil {
		return domain.GetTeamMembersResponse{}, err
	}

	members := make([]domain.TeamMember, 0, len(rows))
	for _, row := range rows {
		role := row.Role
		members = append(members, domain.TeamMember{
			ID:    row.UserID,
			Name:  row.Name,
			Email: row.Email,
			Role:  &role,
		})
	}
	return domain.GetTeamMembersResponse{Members: members}, nil
}

func (s *teamService) SearchTeams(ctx context.Context, req domain.SearchTeamsRequest) (domain.SearchTeamsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTeamsResponse{}, err
	}
	searchQuery := ""
	if req.Filters != nil {
		if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
			return domain.SearchTeamsResponse{}, err
		}
		searchQuery = req.Filters.SearchQuery
	}

	teams, total, err := s.repo.SearchTeams(ctx, searchQuery, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchTeamsResponse{}, err
	}
	return domain.SearchTeamsResponse{
		Teams:  teams,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
