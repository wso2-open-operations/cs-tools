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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type customDashboardLinkService struct {
	links    repository.CustomDashboardLinkRepository
	userRepo repository.UserRepository
}

// NewCustomDashboardLinkService constructs a CustomDashboardLinkService.
func NewCustomDashboardLinkService(
	links repository.CustomDashboardLinkRepository,
	userRepo repository.UserRepository,
) CustomDashboardLinkService {
	return &customDashboardLinkService{links: links, userRepo: userRepo}
}

// owner is the caller's id for an owner-side call on dashboardID. Ownership
// itself is checked by the repository, which only touches the owner's rows.
func (s *customDashboardLinkService) owner(ctx context.Context, dashboardID string) (string, error) {
	if err := validateCustomDashboardID(dashboardID); err != nil {
		return "", err
	}
	return dashboardCallerID(ctx, s.userRepo)
}

// GetLink is the link state of the caller's dashboard; a NotFoundError when
// it is not the caller's (missing and another user's look the same).
func (s *customDashboardLinkService) GetLink(ctx context.Context, dashboardID string) (domain.CustomDashboardLink, error) {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	l, err := s.links.GetLink(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	if l == nil {
		return domain.CustomDashboardLink{}, errDashboardNotFound()
	}
	return *l, nil
}

// EnableLink turns the caller's dashboard's link on and answers its state;
// turning on a link that is already on keeps it. A NotFoundError when the
// dashboard is not the caller's.
func (s *customDashboardLinkService) EnableLink(ctx context.Context, dashboardID string) (domain.CustomDashboardLink, error) {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	l, err := s.links.EnableLink(ctx, ownerID, dashboardID)
	if err != nil {
		return domain.CustomDashboardLink{}, err
	}
	if l == nil {
		return domain.CustomDashboardLink{}, errDashboardNotFound()
	}
	return *l, nil
}

// DisableLink turns the caller's dashboard's link off; everyone who had the
// dashboard only through the link loses it. Turning off a link that is off is
// not an error. A NotFoundError when the dashboard is not the caller's.
func (s *customDashboardLinkService) DisableLink(ctx context.Context, dashboardID string) error {
	ownerID, err := s.owner(ctx, dashboardID)
	if err != nil {
		return err
	}
	found, err := s.links.DisableLink(ctx, ownerID, dashboardID)
	if err != nil {
		return err
	}
	if !found {
		return errDashboardNotFound()
	}
	return nil
}

// OpenLink is the caller opening a dashboard's link: the dashboard, and
// whether the caller owns it. A link that is unknown or turned off, and a
// caller who is not internal staff, get the same NotFoundError, so the answer
// tells nobody whether a link id ever existed.
func (s *customDashboardLinkService) OpenLink(ctx context.Context, req domain.OpenCustomDashboardLinkRequest) (domain.OpenedCustomDashboardLink, error) {
	// The message never quotes the id: writeServiceError logs validation
	// errors, and a link id is enough to open the dashboard.
	if !uuidPattern.MatchString(req.LinkID) {
		return domain.OpenedCustomDashboardLink{}, &apierror.ValidationError{Msg: "linkId is not a UUID"}
	}
	userID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.OpenedCustomDashboardLink{}, err
	}
	dashboardID, own, err := s.links.OpenLink(ctx, userID, req.LinkID)
	if err != nil {
		return domain.OpenedCustomDashboardLink{}, err
	}
	if dashboardID == "" {
		return domain.OpenedCustomDashboardLink{}, &apierror.NotFoundError{Msg: "this link does not open a dashboard; it may have been turned off"}
	}
	return domain.OpenedCustomDashboardLink{DashboardID: dashboardID, Own: own}, nil
}
