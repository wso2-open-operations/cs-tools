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

// UserRepository defines the persistence operations for the "user" table
// (migration 000001).
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
	// GetUsersByIDs returns every user matching the given ids. Unlike
	// SearchUsers, this is not gated to the ServiceNow data source.
	GetUsersByIDs(ctx context.Context, ids []string) ([]domain.User, error)
	// GetUserRoles returns the role names assigned to userID via user_role
	// (migration 000006), empty if none.
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
	// GetUserGroups returns every team userID belongs to via team_member
	// (migration 000028), empty if none.
	GetUserGroups(ctx context.Context, userID string) ([]domain.UserGroupRef, error)
}

type userRepo struct {
	db *pgxpool.Pool
}

// NewUserRepository constructs a UserRepository backed by the given connection pool.
func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepo{db: db}
}

// userColumns is the column list shared by GetUserByEmail and SearchUsers.
// The "user" table (migration 000001) has no phone/timezone column at all --
// unlike account.phone, there is nothing to select for domain.User's Phone/
// Timezone fields, so both are simply left nil (Go's pointer zero value)
// rather than queried. Postgres-backed PatchMe/TimeZone support does not
// exist today regardless (UserService has no PatchMe method at all -- only
// the ServiceNow-backed SNUserService does).
const userColumns = `id, user_name, first_name, last_name, email, user_type::TEXT, created_on, updated_on`

// prefixUserColumns is userColumns qualified with the "u" alias SearchUsers'
// query uses (needed once EXISTS subqueries reference u.id for role
// filtering); GetUserByEmail queries the unaliased table directly and uses
// userColumns as-is.
const prefixUserColumns = `u.id, u.user_name, u.first_name, u.last_name, u.email, u.user_type::TEXT, u.created_on, u.updated_on`

// userTypeFromEnum maps "user".user_type's real user_type_enum labels
// (migration 000007) to domain.UserType. EXTERNAL becomes UserTypeCustomer,
// not UserTypeExternal -- see UserTypeExternal's own doc comment: "the
// postgres source emits customer, ServiceNow emits external" for the same
// underlying concept. NOT_AVAILABLE (recompute_user_type's fallback when a
// user holds no role at all) has no domain equivalent and is left "" (the
// zero value), same as a NULL user_type.
var userTypeFromEnum = map[string]domain.UserType{
	"SYSTEM":   domain.UserTypeSystem,
	"INTERNAL": domain.UserTypeInternal,
	"EXTERNAL": domain.UserTypeCustomer,
}

func scanUser(row interface{ Scan(...any) error }) (domain.User, error) {
	var u domain.User
	var firstName, lastName, email, userType *string
	err := row.Scan(&u.ID, &u.UserName, &firstName, &lastName, &email, &userType, &u.CreatedOn, &u.UpdatedOn)
	if err != nil {
		return domain.User{}, err
	}
	// first_name/last_name/email/user_type (migration 000001/000007) all
	// have no NOT NULL constraint; the domain.User fields they fill are
	// required (non-pointer), so a NULL column becomes "" rather than
	// failing the scan.
	u.FirstName = stringOrEmpty(firstName)
	u.LastName = stringOrEmpty(lastName)
	u.Email = stringOrEmpty(email)
	if userType != nil {
		u.UserType = userTypeFromEnum[*userType]
	}
	return u, nil
}

// GetUserByEmail implements UserRepository.
func (r *userRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM "user" WHERE email = $1`, email))
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

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		// Both branches reference the same positional parameter — PostgreSQL allows $N to appear multiple times.
		where += fmt.Sprintf(
			" AND (u.user_name ILIKE $%d ESCAPE '\\' OR u.email ILIKE $%d ESCAPE '\\')",
			argIdx, argIdx,
		)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	if len(req.Filters.UserNames) > 0 {
		where += fmt.Sprintf(" AND u.user_name = ANY($%d::text[])", argIdx)
		filterArgs = append(filterArgs, req.Filters.UserNames)
		argIdx++
	}

	if len(req.Filters.Emails) > 0 {
		where += fmt.Sprintf(" AND u.email = ANY($%d::text[])", argIdx)
		filterArgs = append(filterArgs, req.Filters.Emails)
		argIdx++
	}

	if len(req.Filters.RoleIDs) > 0 {
		// RoleIDs holds role NAMEs (role.name, migration 000004), not UUIDs,
		// despite the field's name -- see domain.UserRole's own doc comment
		// ("deliberately an open string type"). Matches if the user holds
		// ANY of the given roles (OR semantics), via user_role (migration
		// 000006).
		roleNames := make([]string, len(req.Filters.RoleIDs))
		for i, role := range req.Filters.RoleIDs {
			roleNames[i] = string(role)
		}
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
			WHERE ur.user_id = u.id AND r.name = ANY($%d::text[])
		)`, argIdx)
		filterArgs = append(filterArgs, roleNames)
		argIdx++
	}

	const fromClause = `FROM "user" u`

	countQuery := "SELECT COUNT(*) " + fromClause + " " + where

	dataQuery := fmt.Sprintf(
		`SELECT %s %s %s ORDER BY u.created_on DESC, u.id LIMIT $%d OFFSET $%d`,
		prefixUserColumns, fromClause, where, argIdx, argIdx+1,
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
			u, err := scanUser(rows)
			if err != nil {
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

// GetUsersByIDs returns every user matching the given ids, in Postgres
// mode. Unlike SearchUsers, this is not gated to ServiceNow -- ids are
// this platform's own identifiers, so an id-based lookup is always safe
// regardless of data source.
func (r *userRepo) GetUsersByIDs(ctx context.Context, ids []string) ([]domain.User, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_name, first_name, last_name, email, phone, timezone, user_type, created_at, updated_at
		 FROM users WHERE id = ANY($1)`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}
	defer rows.Close()

	users := make([]domain.User, 0, len(ids))
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.UserName, &u.FirstName, &u.LastName, &u.Email, &u.Phone, &u.Timezone, &u.UserType, &u.CreatedOn, &u.UpdatedOn); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetUserRoles implements UserRepository.
func (r *userRepo) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.name FROM user_role ur
		JOIN role r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user roles: %w", err)
	}
	defer rows.Close()

	roles := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		roles = append(roles, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user roles: %w", err)
	}
	return roles, nil
}

// GetUserGroups implements UserRepository.
func (r *userRepo) GetUserGroups(ctx context.Context, userID string) ([]domain.UserGroupRef, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name FROM team_member tm
		JOIN team t ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user groups: %w", err)
	}
	defer rows.Close()

	groups := []domain.UserGroupRef{}
	for rows.Next() {
		var g domain.UserGroupRef
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, fmt.Errorf("scan user group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user groups: %w", err)
	}
	return groups, nil
}
