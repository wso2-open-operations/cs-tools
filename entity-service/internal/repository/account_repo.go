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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// AccountRepository defines the persistence operations for the accounts table.
type AccountRepository interface {
	// SearchAccounts returns a filtered, paginated slice of accounts together
	// with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) ([]domain.Account, int, error)
	// GetAccountByID returns the account with the given UUID, or a NotFoundError
	// if no such account exists.
	GetAccountByID(ctx context.Context, id string) (domain.Account, error)
}

type accountRepo struct {
	db *pgxpool.Pool
}

// NewAccountRepository constructs an AccountRepository backed by the given connection pool.
func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

// SearchAccounts implements AccountRepository.
func (r *accountRepo) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) ([]domain.Account, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (name ILIKE $%d ESCAPE '\\' OR sf_id ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM accounts " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, sf_id, name, tier, region, activation_date, deactivation_date,
		        owner_id, technical_owner_id, agent_enabled, kb_references_enabled,
		        created_at, updated_at
		 FROM accounts %s
		 ORDER BY created_at DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	// Run COUNT and SELECT in parallel goroutines — each uses its own pool connection.
	var total int
	var accounts []domain.Account

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count accounts: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query accounts: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Account, 0, req.Pagination.Limit)
		for rows.Next() {
			var a domain.Account
			if err := rows.Scan(
				&a.ID, &a.SfID, &a.Name, &a.Tier, &a.Region,
				&a.ActivationDate, &a.DeactivationDate,
				&a.OwnerID, &a.TechnicalOwnerID,
				&a.AgentEnabled, &a.KbReferencesEnabled,
				&a.CreatedOn, &a.UpdatedOn,
			); err != nil {
				return fmt.Errorf("scan account: %w", err)
			}
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate accounts: %w", err)
		}
		accounts = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return accounts, total, nil
}

// GetAccountByID implements AccountRepository.
func (r *accountRepo) GetAccountByID(ctx context.Context, id string) (domain.Account, error) {
	var a domain.Account
	err := r.db.QueryRow(ctx,
		`SELECT id, sf_id, name, tier, region, activation_date, deactivation_date,
		        owner_id, technical_owner_id, agent_enabled, kb_references_enabled,
		        created_at, updated_at
		 FROM accounts WHERE id = $1`, id,
	).Scan(
		&a.ID, &a.SfID, &a.Name, &a.Tier, &a.Region,
		&a.ActivationDate, &a.DeactivationDate,
		&a.OwnerID, &a.TechnicalOwnerID,
		&a.AgentEnabled, &a.KbReferencesEnabled,
		&a.CreatedOn, &a.UpdatedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, &apierror.NotFoundError{Msg: "account not found"}
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("get account by id: %w", err)
	}
	return a, nil
}
