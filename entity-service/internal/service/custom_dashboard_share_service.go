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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type customDashboardShareService struct {
	shares     repository.CustomDashboardShareRepository
	dashboards repository.CustomDashboardRepository
	userRepo   repository.UserRepository
}

// NewCustomDashboardShareService constructs a CustomDashboardShareService.
func NewCustomDashboardShareService(
	shares repository.CustomDashboardShareRepository,
	dashboards repository.CustomDashboardRepository,
	userRepo repository.UserRepository,
) CustomDashboardShareService {
	return &customDashboardShareService{shares: shares, dashboards: dashboards, userRepo: userRepo}
}

// errDashboardNotFound is the answer for a dashboard the caller cannot see:
// missing, or another user's (never said apart, so ids are not probed).
func errDashboardNotFound() error { return &apierror.NotFoundError{Msg: "dashboard not found"} }

// ownedDashboard is the caller's id once the caller is known to own the
// dashboard, or a NotFoundError.
func (s *customDashboardShareService) ownedDashboard(ctx context.Context, dashboardID string) (string, error) {
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

// ListShares is the people the caller's dashboard is shared with; a
// NotFoundError when it is not the caller's.
func (s *customDashboardShareService) ListShares(ctx context.Context, dashboardID string) (domain.CustomDashboardShareList, error) {
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	return s.shareList(ctx, ownerID, dashboardID)
}

// shareList is the dashboard's person shares, never nil.
func (s *customDashboardShareService) shareList(ctx context.Context, ownerID, dashboardID string) (domain.CustomDashboardShareList, error) {
	shares, err := s.shares.ListShares(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	if shares == nil {
		shares = []domain.CustomDashboardShare{}
	}
	return domain.CustomDashboardShareList{Shares: shares}, nil
}

// AddShare shares the caller's dashboard with userID, who must be internal
// staff and not the caller; sharing again is not an error. The answer is the
// dashboard's shares after the change.
func (s *customDashboardShareService) AddShare(ctx context.Context, dashboardID, userID string) (domain.CustomDashboardShareList, error) {
	if err := validateShareUserID(userID); err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	ownerID, err := s.ownedDashboard(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardShareList{}, err
	}
	// The id check accepts upper-case UUIDs while ownerID is the database's
	// lower-case form; PostgreSQL stores both as the same UUID.
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

// RemoveShare stops sharing the caller's dashboard with userID; a
// NotFoundError when the dashboard is not the caller's or was not shared
// with that user.
func (s *customDashboardShareService) RemoveShare(ctx context.Context, dashboardID, userID string) error {
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

// ListSharedWithMe is the dashboards shared with the caller, by person or
// team share; never nil.
func (s *customDashboardShareService) ListSharedWithMe(ctx context.Context) (domain.SharedCustomDashboardList, error) {
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

// GetSharedWithMe is one dashboard shared with the caller, with its
// definition; a NotFoundError when it is not shared with them.
func (s *customDashboardShareService) GetSharedWithMe(ctx context.Context, dashboardID string) (domain.SharedCustomDashboard, error) {
	if err := validateCustomDashboardID(dashboardID); err != nil {
		return domain.SharedCustomDashboard{}, err
	}
	userID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.SharedCustomDashboard{}, err
	}
	d, err := s.shares.GetSharedWithUser(ctx, userID, dashboardID)
	if err != nil {
		return domain.SharedCustomDashboard{}, err
	}
	if d == nil {
		return domain.SharedCustomDashboard{}, errDashboardNotFound()
	}
	return *d, nil
}

// validateShareUserID refuses a non-UUID user id before it reaches a uuid column.
func validateShareUserID(id string) error {
	if !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("user id %q is not a UUID", id)}
	}
	return nil
}
