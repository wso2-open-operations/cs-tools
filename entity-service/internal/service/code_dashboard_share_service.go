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

type codeDashboardShareService struct {
	shares     repository.CodeDashboardShareRepository
	dashboards repository.CodeDashboardRepository
	userRepo   repository.UserRepository
}

// NewCodeDashboardShareService constructs a CodeDashboardShareService.
func NewCodeDashboardShareService(
	shares repository.CodeDashboardShareRepository,
	dashboards repository.CodeDashboardRepository,
	userRepo repository.UserRepository,
) CodeDashboardShareService {
	return &codeDashboardShareService{shares: shares, dashboards: dashboards, userRepo: userRepo}
}

// ownedDashboard is the caller's id once the caller is known to own the code
// dashboard, or a NotFoundError (missing and another user's look the same).
func (s *codeDashboardShareService) ownedDashboard(ctx context.Context, dashboardID string) (string, error) {
	ownerID, err := s.owner(ctx, dashboardID)
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

// owner is the caller's id for an owner-side call on dashboardID. The link
// calls leave ownership to the repository, which only touches the owner's
// rows, as customDashboardLinkService does.
func (s *codeDashboardShareService) owner(ctx context.Context, dashboardID string) (string, error) {
	if err := validateCustomDashboardID(dashboardID); err != nil {
		return "", err
	}
	return dashboardCallerID(ctx, s.userRepo)
}

// ListShares is the people the caller's code dashboard is shared with.
func (s *codeDashboardShareService) ListShares(ctx context.Context, dashboardID string) (domain.CustomDashboardShareList, error) {
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	return s.shareList(ctx, ownerID, dashboardID)
}

// shareList is the dashboard's person shares, never nil.
func (s *codeDashboardShareService) shareList(ctx context.Context, ownerID, dashboardID string) (domain.CustomDashboardShareList, error) {
	shares, err := s.shares.ListShares(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	if shares == nil {
		shares = []domain.CustomDashboardShare{}
	}
	return domain.CustomDashboardShareList{Shares: shares}, nil
}

// AddShare shares the caller's code dashboard with userID, by the custom
// dashboard rules: internal staff only, never the caller; sharing again is
// not an error. The answer is the dashboard's shares after the change.
func (s *codeDashboardShareService) AddShare(ctx context.Context, dashboardID, userID string) (domain.CustomDashboardShareList, error) {
	if err := validateShareUserID(userID); err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	if strings.EqualFold(userID, ownerID) {
		return domain.CustomDashboardShareList{}, &apierror.ValidationError{Msg: "you cannot share a dashboard with yourself"}
	}
	users, err := s.userRepo.GetUsersByIDs(ctx, []string{userID})
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	if len(users) == 0 {
		return domain.CustomDashboardShareList{}, &apierror.NotFoundError{Msg: "user not found"}
	}
	if users[0].UserType != domain.UserTypeInternal {
		return domain.CustomDashboardShareList{}, &apierror.ValidationError{Msg: "dashboards can only be shared with internal staff"}
	}
	owned, err := s.shares.AddShare(ctx, ownerID, dashboardID, userID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	if !owned {
		// Deleted between the ownership check and the insert.
		return domain.CustomDashboardShareList{}, errDashboardNotFound()
	}
	return s.shareList(ctx, ownerID, dashboardID)
}

// RemoveShare stops sharing the caller's code dashboard with userID.
func (s *codeDashboardShareService) RemoveShare(ctx context.Context, dashboardID, userID string) error {
	if err := validateShareUserID(userID); err != nil {
		return err
	}
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return err
	}
	removed, err := s.shares.RemoveShare(ctx, ownerID, dashboardID, userID)
	if err != nil {
		return err
	}
	if !removed {
		return &apierror.NotFoundError{Msg: "dashboard is not shared with this user"}
	}
	return nil
}

// ListTeamShares is the teams the caller's code dashboard is shared with.
func (s *codeDashboardShareService) ListTeamShares(ctx context.Context, dashboardID string) (domain.CustomDashboardTeamShareList, error) {
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	return s.teamShareList(ctx, ownerID, dashboardID)
}

// teamShareList is the dashboard's team shares, never nil.
func (s *codeDashboardShareService) teamShareList(ctx context.Context, ownerID, dashboardID string) (domain.CustomDashboardTeamShareList, error) {
	shares, err := s.shares.ListTeamShares(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardTeamShareList{}, err
	}
	if shares == nil {
		shares = []domain.CustomDashboardTeamShare{}
	}
	return domain.CustomDashboardTeamShareList{Shares: shares}, nil
}

// AddTeamShare shares the caller's code dashboard with a team; who may see it
// is decided when it is read (the team's internal members, live).
func (s *codeDashboardShareService) AddTeamShare(ctx context.Context, dashboardID, teamID string) (domain.CustomDashboardTeamShareList, error) {
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
		return domain.CustomDashboardTeamShareList{}, errDashboardNotFound()
	}
	return s.teamShareList(ctx, ownerID, dashboardID)
}

// RemoveTeamShare stops sharing the caller's code dashboard with teamID.
func (s *codeDashboardShareService) RemoveTeamShare(ctx context.Context, dashboardID, teamID string) error {
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

// GetLink is the link state of the caller's code dashboard.
func (s *codeDashboardShareService) GetLink(ctx context.Context, dashboardID string) (domain.CustomDashboardLink, error) {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	l, err := s.shares.GetLink(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	if l == nil {
		return domain.CustomDashboardLink{}, errDashboardNotFound()
	}
	return *l, nil
}

// EnableLink turns the caller's code dashboard's link on; on again keeps it.
func (s *codeDashboardShareService) EnableLink(ctx context.Context, dashboardID string) (domain.CustomDashboardLink, error) {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	l, err := s.shares.EnableLink(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	if l == nil {
		return domain.CustomDashboardLink{}, errDashboardNotFound()
	}
	return *l, nil
}

// DisableLink turns the caller's code dashboard's link off; everyone who had
// it only through the link loses it.
func (s *codeDashboardShareService) DisableLink(ctx context.Context, dashboardID string) error {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return err
	}
	found, err := s.shares.DisableLink(ctx, ownerID, dashboardID)
	if err != nil {
		return err
	}
	if !found {
		return errDashboardNotFound()
	}
	return nil
}

// OpenLink is the caller opening a code dashboard's link. As for custom
// dashboards, an unknown or switched-off link and a caller who is not
// internal staff get the same NotFoundError, and no message quotes the link
// id (it is enough to open the dashboard).
func (s *codeDashboardShareService) OpenLink(ctx context.Context, req domain.OpenCustomDashboardLinkRequest) (domain.OpenedCustomDashboardLink, error) {
	if !uuidPattern.MatchString(req.LinkID) {
		return domain.OpenedCustomDashboardLink{}, &apierror.ValidationError{Msg: "linkId is not a UUID"}
	}
	userID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.OpenedCustomDashboardLink{}, err
	}
	dashboardID, own, err := s.shares.OpenLink(ctx, userID, req.LinkID)
	if err != nil {
		return domain.OpenedCustomDashboardLink{}, err
	}
	if dashboardID == "" {
		return domain.OpenedCustomDashboardLink{}, &apierror.NotFoundError{Msg: "this link does not open a dashboard; it may have been turned off"}
	}
	return domain.OpenedCustomDashboardLink{DashboardID: dashboardID, Own: own}, nil
}

// ListSharedWithMe is the code dashboards shared with the caller; never nil.
func (s *codeDashboardShareService) ListSharedWithMe(ctx context.Context) (domain.SharedCustomDashboardList, error) {
	userID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.SharedCustomDashboardList{}, err
	}
	list, err := s.shares.ListSharedWithUser(ctx, userID)
	if err != nil {
		return domain.SharedCustomDashboardList{}, err
	}
	if list == nil {
		list = []domain.SharedCustomDashboardSummary{}
	}
	return domain.SharedCustomDashboardList{Dashboards: list}, nil
}

// GetSharedWithMe is one code dashboard shared with the caller, with its
// source; a NotFoundError when it is not shared with them.
func (s *codeDashboardShareService) GetSharedWithMe(ctx context.Context, dashboardID string) (domain.SharedCodeDashboard, error) {
	if err := validateCustomDashboardID(dashboardID); err != nil {
		return domain.SharedCodeDashboard{}, err
	}
	userID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.SharedCodeDashboard{}, err
	}
	d, err := s.shares.GetSharedWithUser(ctx, userID, dashboardID)
	if err != nil {
		return domain.SharedCodeDashboard{}, err
	}
	if d == nil {
		return domain.SharedCodeDashboard{}, errDashboardNotFound()
	}
	return *d, nil
}
