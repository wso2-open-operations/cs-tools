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
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type customDashboardService struct {
	repo     repository.CustomDashboardRepository
	userRepo repository.UserRepository
}

// NewCustomDashboardService constructs a CustomDashboardService.
func NewCustomDashboardService(repo repository.CustomDashboardRepository, userRepo repository.UserRepository) CustomDashboardService {
	return &customDashboardService{repo: repo, userRepo: userRepo}
}

// List is the caller's saved dashboards without their definitions; never nil.
func (s *customDashboardService) List(ctx context.Context) (domain.CustomDashboardList, error) {
	ownerID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.CustomDashboardList{}, err
	}
	dashboards, err := s.repo.List(ctx, ownerID)
	if err != nil {
		return domain.CustomDashboardList{}, err
	}
	if dashboards == nil {
		dashboards = []domain.CustomDashboardSummary{}
	}
	return domain.CustomDashboardList{Dashboards: dashboards}, nil
}

// Get is one of the caller's dashboards; a NotFoundError when the caller has
// none with that id.
func (s *customDashboardService) Get(ctx context.Context, id string) (domain.CustomDashboard, error) {
	if err := validateCustomDashboardID(id); err != nil {
		return domain.CustomDashboard{}, err
	}
	ownerID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.CustomDashboard{}, err
	}
	d, err := s.repo.Get(ctx, ownerID, id)
	if err != nil {
		return domain.CustomDashboard{}, err
	}
	if d == nil {
		return domain.CustomDashboard{}, &apierror.NotFoundError{Msg: "dashboard not found"}
	}
	return *d, nil
}

// Save creates the dashboard, or replaces it when the caller already has one
// with this id. Only a new dashboard counts against MaxCustomDashboards; the
// repository checks that under a per-owner lock, so two saves at the same time
// cannot both pass it.
func (s *customDashboardService) Save(ctx context.Context, id string, req domain.SaveCustomDashboardRequest) (domain.CustomDashboard, error) {
	if err := validateCustomDashboardID(id); err != nil {
		return domain.CustomDashboard{}, err
	}
	clean, err := normalizeCustomDashboard(req)
	if err != nil {
		return domain.CustomDashboard{}, err
	}
	ownerID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.CustomDashboard{}, err
	}
	d, err := s.repo.Upsert(ctx, ownerID, id, clean)
	if err != nil {
		return domain.CustomDashboard{}, err
	}
	if d == nil {
		// The id is another user's dashboard.
		return domain.CustomDashboard{}, &apierror.NotFoundError{Msg: "dashboard not found"}
	}
	return *d, nil
}

// Delete removes one of the caller's dashboards; a NotFoundError when the
// caller has none with that id.
func (s *customDashboardService) Delete(ctx context.Context, id string) error {
	if err := validateCustomDashboardID(id); err != nil {
		return err
	}
	ownerID, err := s.currentUserID(ctx)
	if err != nil {
		return err
	}
	ok, err := s.repo.Delete(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.NotFoundError{Msg: "dashboard not found"}
	}
	return nil
}

// currentUserID resolves the caller exactly as savedFilterViewService does.
func (s *customDashboardService) currentUserID(ctx context.Context) (string, error) {
	return dashboardCallerID(ctx, s.userRepo)
}

// dashboardCallerID is the platform user id of the caller of x-user-id-token,
// for the custom dashboard and dashboard sharing services.
func dashboardCallerID(ctx context.Context, userRepo repository.UserRepository) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

// validateCustomDashboardID refuses a non-UUID id before it reaches the uuid
// column, where Postgres would reject it as a 500 rather than a 400.
func validateCustomDashboardID(id string) error {
	if !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("dashboard id %q is not a UUID", id)}
	}
	return nil
}

// normalizeCustomDashboard trims the name and checks both fields against the
// limits migration 0251 enforces, so a bad request is a 400 rather than a
// constraint error. A NUL character is refused too: PostgreSQL text cannot
// hold one, so it would otherwise fail at the insert as a 500. The definition
// is stored as sent: it is opaque here.
func normalizeCustomDashboard(req domain.SaveCustomDashboardRequest) (domain.SaveCustomDashboardRequest, error) {
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
	if len(req.Definition) > domain.MaxCustomDashboardDefinitionBytes {
		return req, &apierror.ValidationError{
			Msg: fmt.Sprintf("definition must be at most %d bytes", domain.MaxCustomDashboardDefinitionBytes),
		}
	}
	return domain.SaveCustomDashboardRequest{Name: name, Definition: req.Definition}, nil
}
