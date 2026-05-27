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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type accountService struct {
	repo repository.AccountRepository
}

// NewAccountService constructs an AccountService backed by the given repository.
func NewAccountService(repo repository.AccountRepository) AccountService {
	return &accountService{repo: repo}
}

// SearchAccounts implements AccountService.
func (s *accountService) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAccountsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchAccountsResponse{}, err
	}
	accounts, total, err := s.repo.SearchAccounts(ctx, req)
	if err != nil {
		return domain.SearchAccountsResponse{}, err
	}

	return domain.SearchAccountsResponse{
		Accounts: accounts,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(accounts) < total,
	}, nil
}
