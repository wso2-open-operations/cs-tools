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

package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// AccountContactRow is the raw shape of one row read from account_contact
// (migration 000020), joined against "user" to resolve a display name/email
// where possible. account_contact has no name/email column of its own --
// only user_name, a free-text identifying string -- so ResolvedName/
// ResolvedEmail are nil whenever no "user" row's own user_name matches it
// case-insensitively.
type AccountContactRow struct {
	UserName  string
	IsPrimary *bool
	IsActive  *bool
	// ResolvedName/ResolvedEmail come from the "user" row joined on
	// LOWER(u.user_name) = LOWER(ac.user_name), when one exists.
	ResolvedName  *string
	ResolvedEmail *string
}

// AccountContactRepository defines the read operations for the
// account_contact table.
type AccountContactRepository interface {
	// SearchAccountContacts returns a filtered, paginated slice of accountID's
	// contacts together with the total count of matching rows before
	// pagination. callerEmail is the caller's own resolved identity (from
	// their x-user-id-token), threaded down to this layer at explicit
	// request against a future authorization decision (e.g. restricting an
	// EXTERNAL caller to their own account) -- not enforced yet; every
	// caller sees the same result today, same as before this method existed.
	SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest, callerEmail string) ([]AccountContactRow, int, error)
}

type accountContactRepo struct {
	db *pgxpool.Pool
}

// NewAccountContactRepository constructs an AccountContactRepository backed by the given connection pool.
func NewAccountContactRepository(db *pgxpool.Pool) AccountContactRepository {
	return &accountContactRepo{db: db}
}

const accountContactColumns = `ac.user_name, ac.is_primary_contact, ac.is_active, u.email, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''))`

const accountContactFromJoins = `
	FROM account_contact ac
	LEFT JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)`

func scanAccountContact(row interface{ Scan(...any) error }) (AccountContactRow, error) {
	var c AccountContactRow
	err := row.Scan(&c.UserName, &c.IsPrimary, &c.IsActive, &c.ResolvedEmail, &c.ResolvedName)
	return c, err
}

// SearchAccountContacts implements AccountContactRepository.
func (r *accountContactRepo) SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest, _ string) ([]AccountContactRow, int, error) {
	where := "WHERE ac.account_id = $1"
	args := []any{accountID}
	argIdx := 2

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (ac.user_name ILIKE $%d ESCAPE '\\' OR u.email ILIKE $%d ESCAPE '\\' OR u.name ILIKE $%d ESCAPE '\\')", argIdx, argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) " + accountContactFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY ac.created_on DESC, ac.id LIMIT $%d OFFSET $%d",
		accountContactColumns, accountContactFromJoins, where, argIdx, argIdx+1)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var rows []AccountContactRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count account contacts: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		res, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query account contacts: %w", err)
		}
		defer res.Close()

		out := make([]AccountContactRow, 0, req.Pagination.Limit)
		for res.Next() {
			c, err := scanAccountContact(res)
			if err != nil {
				return fmt.Errorf("scan account contact: %w", err)
			}
			out = append(out, c)
		}
		if err := res.Err(); err != nil {
			return fmt.Errorf("iterate account contacts: %w", err)
		}
		rows = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}
