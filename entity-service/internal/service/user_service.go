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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var uuidRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validateUUIDs returns a ValidationError if any element of ids is not a valid UUID.
func validateUUIDs(field string, ids []string) error {
	for _, id := range ids {
		if !uuidRE.MatchString(id) {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s contains invalid UUID: %q", field, id)}
		}
	}
	return nil
}

const (
	defaultLimit      = 20
	maxLimit          = 100
	maxSearchQueryLen = 200

	defaultUserLimit = 10
	maxUserLimit     = 50
)

// normalizePagination applies defaults and clamps to p in-place.
// Returns a ValidationError if the limit exceeds maxLimit.
func normalizePagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > maxLimit {
		return &apierror.ValidationError{Msg: "limit cannot exceed 100"}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// normalizeUserPagination applies user-search-specific defaults (limit 10, max 50).
func normalizeUserPagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultUserLimit
	}
	if p.Limit > maxUserLimit {
		return &apierror.ValidationError{Msg: "limit cannot exceed 50"}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// validateSearchQuery returns a ValidationError if q exceeds the character limit.
func validateSearchQuery(q string) error {
	if utf8.RuneCountInString(q) > maxSearchQueryLen {
		return &apierror.ValidationError{Msg: "searchQuery cannot exceed 200 characters"}
	}
	return nil
}

type userService struct {
	repo repository.UserRepository
}

// NewUserService constructs a UserService backed by the given repository.
func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

// SearchUsers implements UserService.
func (s *userService) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error) {
	if err := normalizeUserPagination(&req.Pagination); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if len(req.Filters.Roles) > 0 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "roles filter is only supported for the ServiceNow data source"}
	}
	if req.Filters.Active != nil {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "active filter is only supported for the ServiceNow data source"}
	}
	if req.SortBy.Field != "" {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "sortBy is only supported for the ServiceNow data source"}
	}
	if len(req.Filters.UserNames) > 50 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "userNames cannot contain more than 50 values"}
	}
	if len(req.Filters.Emails) > 50 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "emails cannot contain more than 50 values"}
	}

	users, total, err := s.repo.SearchUsers(ctx, req)
	if err != nil {
		return domain.SearchUsersResponse{}, err
	}

	return domain.SearchUsersResponse{
		Users:   users,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(users) < total,
	}, nil
}
