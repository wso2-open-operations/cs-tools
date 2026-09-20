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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// resolveCallerEmail resolves the authenticated caller's email from their
// x-user-id-token, without a "user" table lookup -- callers that only need
// the claimed email (not a resolved platform id) use this instead of
// duplicating the token-extraction dance. Threaded down to the repository
// layer by callers such as accountContactService/projectContactService
// against a future authorization decision (e.g. restricting an EXTERNAL
// caller to their own account/project) -- not enforced yet.
func resolveCallerEmail(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}

type accountContactService struct {
	repo repository.AccountContactRepository
}

// NewAccountContactService constructs an AccountContactService backed by Postgres.
func NewAccountContactService(repo repository.AccountContactRepository) AccountContactService {
	return &accountContactService{repo: repo}
}

// SearchAccountContacts implements AccountContactService.
func (s *accountContactService) SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest) (domain.SearchAccountContactsResponse, error) {
	if err := validateUUIDs("id", []string{accountID}); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}

	callerEmail, err := resolveCallerEmail(ctx)
	if err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}

	rows, total, err := s.repo.SearchAccountContacts(ctx, accountID, req, callerEmail)
	if err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}

	contacts := make([]domain.AccountContact, 0, len(rows))
	for _, row := range rows {
		// account_contact has no name/email column of its own -- fall back to
		// its free-text user_name when no "user" row resolves one, so a
		// contact with no matching platform user still comes back
		// identifiable rather than blank. See AccountContactRow's own doc
		// comment.
		name := row.UserName
		if row.ResolvedName != nil && *row.ResolvedName != "" {
			name = *row.ResolvedName
		}
		email := row.UserName
		if row.ResolvedEmail != nil && *row.ResolvedEmail != "" {
			email = *row.ResolvedEmail
		}
		isPrimary := row.IsPrimary != nil && *row.IsPrimary

		contacts = append(contacts, domain.AccountContact{
			Name:      name,
			Email:     email,
			IsPrimary: isPrimary,
		})
	}

	return domain.SearchAccountContactsResponse{
		Contacts: contacts,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}
