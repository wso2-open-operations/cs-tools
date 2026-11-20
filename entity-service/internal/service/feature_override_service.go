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
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// maxFeatureNavNodeIDLen caps a nav-node id. The portal's own ids are far shorter.
const maxFeatureNavNodeIDLen = 200

// featureNavNodeID is the shape of a portal nav-node id: lower-case words
// joined by dots or hyphens (operations.incidents, kb-articles.to-review).
var featureNavNodeID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

type featureOverrideService struct {
	repo                     repository.FeatureOverrideRepository
	access                   AccessService
	csmPortalBackendClientID string
}

// NewFeatureOverrideService constructs a FeatureOverrideService.
// csmPortalBackendClientID is CSM_PORTAL_BACKEND_CLIENT_ID. Writes are gated on
// it exactly as banner changes are (see requireCSMPortalBackendCaller): the
// admin check happens once, in csm-portal-backend's PermAdmin, and this
// service does not re-derive it from AUTH_ADMIN_ROLES.
func NewFeatureOverrideService(repo repository.FeatureOverrideRepository, access AccessService, csmPortalBackendClientID string) FeatureOverrideService {
	return &featureOverrideService{repo: repo, access: access, csmPortalBackendClientID: csmPortalBackendClientID}
}

func (s *featureOverrideService) List(ctx context.Context) (domain.FeatureOverrideList, error) {
	if err := RequireInternalCaller(ctx, s.access, "feature visibility is only available to internal callers"); err != nil {
		return domain.FeatureOverrideList{}, err
	}
	return s.repo.List(ctx)
}

func (s *featureOverrideService) Put(ctx context.Context, req domain.SaveFeatureOverrideRequest) (domain.FeatureOverride, error) {
	actor, err := s.requireCSMPortalBackendCaller(ctx)
	if err != nil {
		return domain.FeatureOverride{}, err
	}
	req.NavNodeID = strings.TrimSpace(req.NavNodeID)
	if req.NavNodeID == "" || len(req.NavNodeID) > maxFeatureNavNodeIDLen || !featureNavNodeID.MatchString(req.NavNodeID) {
		return domain.FeatureOverride{}, &apierror.ValidationError{Msg: "navNodeId must be a nav-node id such as operations.incidents"}
	}
	req.State = strings.TrimSpace(req.State)
	if !domain.IsFeatureState(req.State) {
		return domain.FeatureOverride{}, &apierror.ValidationError{Msg: "state must be enabled or preview"}
	}
	return s.repo.Put(ctx, actor, req)
}

// requireCSMPortalBackendCaller mirrors portalBannerService's gate: an
// internal caller, a signed-in person, and csm-portal-backend's own client id.
// It returns the person's email for updated_by.
func (s *featureOverrideService) requireCSMPortalBackendCaller(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, "feature visibility is only available to internal callers"); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return "", &apierror.ForbiddenError{Msg: "feature visibility can only be changed by a signed-in person"}
	}
	if s.csmPortalBackendClientID == "" || id.ClientID != s.csmPortalBackendClientID {
		return "", &apierror.ForbiddenError{Msg: "feature visibility can only be changed through the CSM Portal"}
	}
	return id.UserEmail, nil
}
