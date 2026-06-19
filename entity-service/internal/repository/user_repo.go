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

// Package repository handles all direct database access for the entity service.
// Each exported type corresponds to one database table; all methods accept a
// context so callers can propagate deadlines and cancellations.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// UserRepository defines the persistence operations for the users table.
type UserRepository interface {
	// SearchUsers returns a filtered, paginated slice of users together with
	// the total count of rows that match the filter (before pagination).
	// The count and data queries are executed concurrently on separate pool
	// connections, so total may differ by at most one write from the page —
	// this is acceptable for search-style pagination.
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error)
	// GetUserByEmail returns the user with the given email address, or a
	// NotFoundError if no matching user exists.
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}

type userRepo struct {
	db *pgxpool.Pool
}

// NewUserRepository constructs a UserRepository backed by the given connection pool.
func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepo{db: db}
}

// GetUserByEmail implements UserRepository.
func (r *userRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, user_name, first_name, last_name, email, phone, timezone, user_type, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(
		&u.ID, &u.UserName, &u.FirstName, &u.LastName,
		&u.Email, &u.Phone, &u.Timezone, &u.UserType,
		&u.CreatedOn, &u.UpdatedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, &apierror.NotFoundError{Msg: "no user found with email: " + email}
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// SearchUsers implements UserRepository.
func (r *userRepo) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		// Both branches reference the same positional parameter — PostgreSQL allows $N to appear multiple times.
		where += fmt.Sprintf(
			" AND (user_name ILIKE $%d ESCAPE '\\' OR email ILIKE $%d ESCAPE '\\')",
			argIdx, argIdx,
		)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM users " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, user_name, first_name, last_name, email, phone, timezone, user_type, created_at, updated_at
		 FROM users %s
		 ORDER BY created_at DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	// Run COUNT and SELECT in parallel goroutines — each uses its own pool connection.
	var total int
	var users []domain.User

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query users: %w", err)
		}
		defer rows.Close()

		result := make([]domain.User, 0, req.Pagination.Limit)
		for rows.Next() {
			var u domain.User
			if err := rows.Scan(
				&u.ID, &u.UserName, &u.FirstName, &u.LastName,
				&u.Email, &u.Phone, &u.Timezone, &u.UserType,
				&u.CreatedOn, &u.UpdatedOn,
			); err != nil {
				return fmt.Errorf("scan user: %w", err)
			}
			result = append(result, u)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate users: %w", err)
		}
		users = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
