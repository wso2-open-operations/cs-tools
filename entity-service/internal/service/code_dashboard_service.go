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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type codeDashboardService struct {
	repo     repository.CodeDashboardRepository
	userRepo repository.UserRepository
}

// NewCodeDashboardService constructs a CodeDashboardService.
func NewCodeDashboardService(repo repository.CodeDashboardRepository, userRepo repository.UserRepository) CodeDashboardService {
	return &codeDashboardService{repo: repo, userRepo: userRepo}
}

// List is the caller's saved code dashboards without their source; never nil.
func (s *codeDashboardService) List(ctx context.Context) (domain.CodeDashboardList, error) {
	ownerID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.CodeDashboardList{}, err
	}
	dashboards, err := s.repo.List(ctx, ownerID)
	if err != nil {
		return domain.CodeDashboardList{}, err
	}
	if dashboards == nil {
		dashboards = []domain.CodeDashboardSummary{}
	}
	return domain.CodeDashboardList{Dashboards: dashboards}, nil
}

// Get is one of the caller's code dashboards; a NotFoundError when the caller
// has none with that id.
func (s *codeDashboardService) Get(ctx context.Context, id string) (domain.CodeDashboard, error) {
	if err := validateCustomDashboardID(id); err != nil {
		return domain.CodeDashboard{}, err
	}
	ownerID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.CodeDashboard{}, err
	}
	d, err := s.repo.Get(ctx, ownerID, id)
	if err != nil {
		return domain.CodeDashboard{}, err
	}
	if d == nil {
		return domain.CodeDashboard{}, &apierror.NotFoundError{Msg: "dashboard not found"}
	}
	return *d, nil
}

// Save creates the code dashboard, or replaces it when the caller already has
// one with this id. Only a new dashboard counts against MaxCodeDashboards; the
// repository checks that under a per-owner lock.
func (s *codeDashboardService) Save(ctx context.Context, id string, req domain.SaveCodeDashboardRequest) (domain.CodeDashboard, error) {
	if err := validateCustomDashboardID(id); err != nil {
		return domain.CodeDashboard{}, err
	}
	clean, err := normalizeCodeDashboard(req)
	if err != nil {
		return domain.CodeDashboard{}, err
	}
	ownerID, err := dashboardCallerID(ctx, s.userRepo)
	if err != nil {
		return domain.CodeDashboard{}, err
	}
	d, err := s.repo.Upsert(ctx, ownerID, id, clean)
	if err != nil {
		return domain.CodeDashboard{}, err
	}
	if d == nil {
		// The id is another user's dashboard, or one with no owner.
		return domain.CodeDashboard{}, &apierror.NotFoundError{Msg: "dashboard not found"}
	}
	return *d, nil
}

// Delete removes one of the caller's code dashboards; a NotFoundError when
// the caller has none with that id.
func (s *codeDashboardService) Delete(ctx context.Context, id string) error {
	if err := validateCustomDashboardID(id); err != nil {
		return err
	}
	ownerID, err := dashboardCallerID(ctx, s.userRepo)
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

// normalizeCodeDashboard trims the name and checks both fields against the
// limits migration 0295 enforces, so a bad request is a 400 rather than a
// constraint error; a NUL character is refused as normalizeCustomDashboard
// does. The source is stored as sent: it is opaque here.
func normalizeCodeDashboard(req domain.SaveCodeDashboardRequest) (domain.SaveCodeDashboardRequest, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return req, &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(name) > domain.MaxCustomDashboardNameLength {
		return req, &apierror.ValidationError{
			Msg: fmt.Sprintf("name must be at most %d characters", domain.MaxCustomDashboardNameLength),
		}
	}
	if strings.TrimSpace(req.Source) == "" {
		return req, &apierror.ValidationError{Msg: "source is required"}
	}
	if strings.ContainsRune(name, 0) || strings.ContainsRune(req.Source, 0) {
		return req, &apierror.ValidationError{Msg: "name and source must not contain NUL characters"}
	}
	if len(req.Source) > domain.MaxCodeDashboardSourceBytes {
		return req, &apierror.ValidationError{
			Msg: fmt.Sprintf("source must be at most %d bytes", domain.MaxCodeDashboardSourceBytes),
		}
	}
	return domain.SaveCodeDashboardRequest{Name: name, Source: req.Source}, nil
}
