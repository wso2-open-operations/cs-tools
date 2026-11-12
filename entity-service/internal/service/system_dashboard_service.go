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
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// systemDashboardKeyPattern is a key that is safe in a URL path segment: it
// starts with a letter or digit and goes on with letters, digits, '.', '_' or
// '-' (so a file name such as "support-overview.json" qualifies). The length
// is checked separately, in characters.
var systemDashboardKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type systemDashboardService struct {
	repo                     repository.SystemDashboardRepository
	csmPortalBackendClientID string
}

// NewSystemDashboardService constructs a SystemDashboardService. Every method
// requires the caller to be the CSM portal backend itself (see
// requireCSMPortalBackendCaller).
func NewSystemDashboardService(repo repository.SystemDashboardRepository, csmPortalBackendClientID string) SystemDashboardService {
	return &systemDashboardService{repo: repo, csmPortalBackendClientID: csmPortalBackendClientID}
}

// requireCSMPortalBackendCaller admits only a request made with the CSM portal
// backend's own client credential. Unlike portalBannerService's gate, no
// signed-in person is required: the backend upserts and reads these at
// startup, with no user in the loop, and AccessService.ResolveScope refuses
// that client without a user token. The identity must have been validated, and
// an unset client id admits nobody.
func (s *systemDashboardService) requireCSMPortalBackendCaller(ctx context.Context) error {
	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return &apierror.ServiceUnavailableError{Msg: "system dashboards cannot be served: no verified identity on this request"}
	}
	if s.csmPortalBackendClientID == "" || id.ClientID != s.csmPortalBackendClientID {
		return &apierror.ForbiddenError{Msg: "system dashboards are only available to the CSM portal backend"}
	}
	return nil
}

// List is every system dashboard; never nil.
func (s *systemDashboardService) List(ctx context.Context) (domain.SystemDashboardList, error) {
	if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
		return domain.SystemDashboardList{}, err
	}
	dashboards, err := s.repo.List(ctx)
	if err != nil {
		return domain.SystemDashboardList{}, err
	}
	if dashboards == nil {
		dashboards = []domain.SystemDashboard{}
	}
	return domain.SystemDashboardList{Dashboards: dashboards}, nil
}

// Get is the system dashboard with this key; a NotFoundError when there is none.
func (s *systemDashboardService) Get(ctx context.Context, key string) (domain.SystemDashboard, error) {
	if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
		return domain.SystemDashboard{}, err
	}
	if err := validateSystemDashboardKey(key); err != nil {
		return domain.SystemDashboard{}, err
	}
	d, err := s.repo.Get(ctx, key)
	if err != nil {
		return domain.SystemDashboard{}, err
	}
	if d == nil {
		return domain.SystemDashboard{}, &apierror.NotFoundError{Msg: "system dashboard not found"}
	}
	return *d, nil
}

// Save creates the system dashboard, or overwrites its name and definition
// when the key already exists.
func (s *systemDashboardService) Save(ctx context.Context, key string, req domain.SaveSystemDashboardRequest) (domain.SystemDashboard, error) {
	if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
		return domain.SystemDashboard{}, err
	}
	if err := validateSystemDashboardKey(key); err != nil {
		return domain.SystemDashboard{}, err
	}
	clean, err := normalizeSystemDashboard(req)
	if err != nil {
		return domain.SystemDashboard{}, err
	}
	d, err := s.repo.Upsert(ctx, key, clean)
	if err != nil {
		return domain.SystemDashboard{}, err
	}
	return *d, nil
}

// validateSystemDashboardKey refuses a key that is empty, too long or has
// characters outside systemDashboardKeyPattern, as a 400.
func validateSystemDashboardKey(key string) error {
	if len(key) > domain.MaxSystemDashboardKeyLength || !systemDashboardKeyPattern.MatchString(key) {
		return &apierror.ValidationError{
			Msg: fmt.Sprintf("system dashboard key must be 1 to %d letters, digits, '.', '_' or '-', starting with a letter or digit", domain.MaxSystemDashboardKeyLength),
		}
	}
	return nil
}

// normalizeSystemDashboard trims the name and checks both fields against the
// limits migration 100029 enforces, as normalizeCustomDashboard does for a
// user's dashboard, except that the definition may be up to
// MaxSystemDashboardDefinitionBytes.
func normalizeSystemDashboard(req domain.SaveSystemDashboardRequest) (domain.SaveSystemDashboardRequest, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return req, &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(name) > domain.MaxCustomDashboardNameLength {
		return req, &apierror.ValidationError{
			Msg: fmt.Sprintf("name must be at most %d characters", domain.MaxCustomDashboardNameLength),
		}
	}
	if strings.TrimSpace(req.Definition) == "" {
		return req, &apierror.ValidationError{Msg: "definition is required"}
	}
	if strings.ContainsRune(name, 0) || strings.ContainsRune(req.Definition, 0) {
		return req, &apierror.ValidationError{Msg: "name and definition must not contain NUL characters"}
	}
	if len(req.Definition) > domain.MaxSystemDashboardDefinitionBytes {
		return req, &apierror.ValidationError{
			Msg: fmt.Sprintf("definition must be at most %d bytes", domain.MaxSystemDashboardDefinitionBytes),
		}
	}
	return domain.SaveSystemDashboardRequest{Name: name, Definition: req.Definition}, nil
}
