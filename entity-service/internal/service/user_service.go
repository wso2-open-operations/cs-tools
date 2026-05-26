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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type userService struct {
	repo repository.UserRepository
}

// NewUserService constructs a UserService backed by the given repository.
func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

// SearchUsers implements UserService.
func (s *userService) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error) {
	if req.Pagination.Limit <= 0 {
		req.Pagination.Limit = defaultLimit
	}
	if req.Pagination.Limit > maxLimit {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "limit cannot exceed 100"}
	}
	if req.Pagination.Offset < 0 {
		req.Pagination.Offset = 0
	}
	if req.UserType != nil && *req.UserType != domain.UserTypeInternal && *req.UserType != domain.UserTypeCustomer {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "invalid userType: must be \"internal\" or \"customer\""}
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
