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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type customDashboardTeamShareService struct {
	shares     repository.CustomDashboardTeamShareRepository
	dashboards repository.CustomDashboardRepository
	userRepo   repository.UserRepository
}

// NewCustomDashboardTeamShareService constructs a CustomDashboardTeamShareService.
func NewCustomDashboardTeamShareService(
	shares repository.CustomDashboardTeamShareRepository,
	dashboards repository.CustomDashboardRepository,
	userRepo repository.UserRepository,
) CustomDashboardTeamShareService {
	return &customDashboardTeamShareService{shares: shares, dashboards: dashboards, userRepo: userRepo}
}

// ownedDashboard is the caller's id once the caller is known to own the
// dashboard, or a NotFoundError (missing and another user's look the same).
func (s *customDashboardTeamShareService) ownedDashboard(ctx context.Context, dashboardID string) (string, error) {
	if err := validateCustomDashboardID(dashboardID); err != nil {
		return "", err
	}
	ownerID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return "", err
	}
	d, err := s.dashboards.Get(ctx, ownerID, dashboardID)
	if err != nil {
		return "", err
	}
	if d == nil {
		return "", errDashboardNotFound()
	}
	return ownerID, nil
}

// ListTeamShares is the teams the caller's dashboard is shared with; a
// NotFoundError when it is not the caller's.
func (s *customDashboardTeamShareService) ListTeamShares(ctx context.Context, dashboardID string) (domain.CustomDashboardTeamShareList, error) {
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	return s.teamShareList(ctx, ownerID, dashboardID)
}

// teamShareList is the dashboard's team shares, never nil.
func (s *customDashboardTeamShareService) teamShareList(ctx context.Context, ownerID, dashboardID string) (domain.CustomDashboardTeamShareList, error) {
	shares, err := s.shares.ListTeamShares(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	if shares == nil {
		shares = []domain.CustomDashboardTeamShare{}
	}
	return domain.CustomDashboardTeamShareList{Shares: shares}, nil
}

// AddTeamShare shares the caller's dashboard with a team; sharing again is not
// an error. The answer is the dashboard's team shares after the change. Who
// may see it is decided when it is read (the team's internal members, live).
func (s *customDashboardTeamShareService) AddTeamShare(ctx context.Context, dashboardID, teamID string) (domain.CustomDashboardTeamShareList, error) {
	if err := validateShareTeamID(teamID); err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	owned, err := s.shares.AddTeamShare(ctx, ownerID, dashboardID, teamID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	if !owned {
		// Deleted between the ownership check and the insert.
		return domain.CustomDashboardTeamShareList{}, errDashboardNotFound()
	}
	return s.teamShareList(ctx, ownerID, dashboardID)
}

// RemoveTeamShare stops sharing the caller's dashboard with teamID; a
// NotFoundError when the dashboard is not the caller's or was not shared
// with that team.
func (s *customDashboardTeamShareService) RemoveTeamShare(ctx context.Context, dashboardID, teamID string) error {
	if err := validateShareTeamID(teamID); err != nil {
		return err
	}
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return err
	}
	removed, err := s.shares.RemoveTeamShare(ctx, ownerID, dashboardID, teamID)
	if err != nil {
		return err
	}
	if !removed {
		return &apierror.NotFoundError{Msg: "dashboard is not shared with this team"}
	}
	return nil
}

// validateShareTeamID refuses a non-UUID team id before it reaches a uuid column.
func validateShareTeamID(id string) error {
	if !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("team id %q is not a UUID", id)}
	}
	return nil
}
