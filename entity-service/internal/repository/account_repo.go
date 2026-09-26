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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// AccountRow is the raw shape of one row read from the account table, together
// with the joined technical-owner/account-manager person refs and the joined
// CRE/SRE team refs. It is mapped to domain.AccountView / domain.AccountDetail
// by the service layer.
type AccountRow struct {
	ID                         string
	Name                       string
	Classification             *string
	Pod                        *string
	SfID                       *string
	Region                     *string
	ActivationDate             *time.Time
	DeactivationDate           *time.Time
	TechnicalOwnerID           *string
	TechnicalOwnerName         *string
	TechnicalOwnerEmail        *string
	AccountManagerID           *string
	AccountManagerName         *string
	AccountManagerEmail        *string
	RenewalAccountManagerID    *string
	RenewalAccountManagerName  *string
	RenewalAccountManagerEmail *string
	CreTeamID                  *string
	CreTeamName                *string
	SreTeamID                  *string
	SreTeamName                *string
	HasAgent                   *bool
	HasKbReferences            *bool
	CreatedOn                  time.Time
	CreatedBy                  string
	UpdatedOn                  time.Time
}

// AccountRepository defines the persistence operations for the account table.
type AccountRepository interface {
	// SearchAccounts returns a filtered, paginated slice of accounts together
	// with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) ([]AccountRow, int, error)
	// GetAccountByID returns the account with the given UUID, or a NotFoundError
	// if no such account exists.
	GetAccountByID(ctx context.Context, id string) (AccountRow, error)
	// UpdateAccountTeams sets the account's CRE and/or SRE team. A nil
	// creTeamID/sreTeamID leaves that assignment unchanged; there is no way
	// to explicitly clear an assignment to "no team" via this method (see
	// its caller, AccountService.UpdateAccountTeams, for why). Returns a
	// ValidationError if either non-nil id does not reference an existing
	// team row, or a NotFoundError if the account does not exist.
	UpdateAccountTeams(ctx context.Context, accountID string, creTeamID, sreTeamID *string) (AccountRow, error)
	UpsertFromSalesforce(ctx context.Context, row domain.SalesforceAccountUpsert) error
	SoftDeleteBySfID(ctx context.Context, sfID string) error
	LookupUserIDByEmail(ctx context.Context, email string) (*string, error)
}

type accountRepo struct {
	db *pgxpool.Pool
}

// NewAccountRepository constructs an AccountRepository backed by the given connection pool.
func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &accountRepo{db: db}
}

// accountSelectColumns' cre/sre joins are the same "group" table
// change_request_repo.go's own customer_group_id join already uses (see
// that file's changeRequestDetailJoins) -- account.cre_team_id/sre_team_id
// (renamed/added by migration 000074, ex-integration_cs_team_id) are real
// FKs into "group" now, unlike when CreTeam/SreTeam were first documented
// as "ServiceNow data source only" on domain.AccountView/AccountDetail;
// this is what actually reads them back for the Postgres data source.
const accountSelectColumns = `
	a.id, a.name, a.classification, a.global_pod, a.sf_id, a.region,
	a.activation_date, a.deactivation_date,
	tow.id, COALESCE(tow.name, NULLIF(TRIM(CONCAT_WS(' ', tow.first_name, tow.last_name)), '')), tow.email,
	mgr.id, COALESCE(mgr.name, NULLIF(TRIM(CONCAT_WS(' ', mgr.first_name, mgr.last_name)), '')), mgr.email,
	ram.id, COALESCE(ram.name, NULLIF(TRIM(CONCAT_WS(' ', ram.first_name, ram.last_name)), '')), ram.email,
	cre.id, cre.name, sre.id, sre.name,
	a.ai_gen_response_enabled, a.smart_knowledge_base_suggestions_enabled,
	a.created_on, a.created_by, a.updated_on`

const accountFromJoins = `
	FROM account a
	LEFT JOIN "user" tow ON tow.id = a.technical_owner_id
	LEFT JOIN "user" mgr ON mgr.id = a.account_manager_id
	LEFT JOIN "user" ram ON ram.id = a.renewal_account_manager_id
	LEFT JOIN "group" cre ON cre.id = a.cre_team_id
	LEFT JOIN "group" sre ON sre.id = a.sre_team_id`

func scanAccountRow(row interface{ Scan(...any) error }) (AccountRow, error) {
	var a AccountRow
	err := row.Scan(
		&a.ID, &a.Name, &a.Classification, &a.Pod, &a.SfID, &a.Region,
		&a.ActivationDate, &a.DeactivationDate,
		&a.TechnicalOwnerID, &a.TechnicalOwnerName, &a.TechnicalOwnerEmail,
		&a.AccountManagerID, &a.AccountManagerName, &a.AccountManagerEmail,
		&a.RenewalAccountManagerID, &a.RenewalAccountManagerName, &a.RenewalAccountManagerEmail,
		&a.CreTeamID, &a.CreTeamName, &a.SreTeamID, &a.SreTeamName,
		&a.HasAgent, &a.HasKbReferences,
		&a.CreatedOn, &a.CreatedBy, &a.UpdatedOn,
	)
	return a, err
}

// SearchAccounts implements AccountRepository.
func (r *accountRepo) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) ([]AccountRow, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (a.name ILIKE $%d ESCAPE '\\' OR a.sf_id ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}
	if req.Filters.Pod != "" {
		where += fmt.Sprintf(" AND a.global_pod = $%d", argIdx)
		filterArgs = append(filterArgs, req.Filters.Pod)
		argIdx++
	}
	if req.Filters.Classification != "" {
		where += fmt.Sprintf(" AND a.classification = $%d", argIdx)
		filterArgs = append(filterArgs, req.Filters.Classification)
		argIdx++
	}
	if req.Filters.Active != nil {
		if *req.Filters.Active {
			where += " AND a.deactivation_date IS NULL"
		} else {
			where += " AND a.deactivation_date IS NOT NULL"
		}
	}

	countQuery := "SELECT COUNT(*) " + accountFromJoins + " " + where

	dataQuery := fmt.Sprintf(
		"SELECT %s %s %s ORDER BY a.created_on DESC, a.id LIMIT $%d OFFSET $%d",
		accountSelectColumns, accountFromJoins, where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	// Run COUNT and SELECT in parallel goroutines — each uses its own pool connection.
	var total int
	var accounts []AccountRow

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

		result := make([]AccountRow, 0, req.Pagination.Limit)
		for rows.Next() {
			a, err := scanAccountRow(rows)
			if err != nil {
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
func (r *accountRepo) GetAccountByID(ctx context.Context, id string) (AccountRow, error) {
	query := "SELECT " + accountSelectColumns + " " + accountFromJoins + " WHERE a.id = $1"
	a, err := scanAccountRow(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountRow{}, &apierror.NotFoundError{Msg: "account not found"}
	}
	if err != nil {
		return AccountRow{}, fmt.Errorf("get account by id: %w", err)
	}
	return a, nil
}

// updateAccountTeamsQuery leaves cre_team_id/sre_team_id unchanged when the
// corresponding parameter is NULL (a nil Go pointer) -- the same "nil means
// don't touch this field" convention UpdateCase uses, here expressed with
// COALESCE rather than a CASE/empty-string guard since UUID has no such
// sentinel value. There is no parameter combination that clears a team
// assignment to "no team" once set; see UpdateAccountTeams's doc comment.
const updateAccountTeamsQuery = `
	UPDATE account
	SET cre_team_id = COALESCE($2::uuid, cre_team_id),
	    sre_team_id = COALESCE($3::uuid, sre_team_id),
	    updated_on = now()
	WHERE id = $1`

// UpdateAccountTeams implements AccountRepository.
func (r *accountRepo) UpdateAccountTeams(ctx context.Context, accountID string, creTeamID, sreTeamID *string) (AccountRow, error) {
	tag, err := r.db.Exec(ctx, updateAccountTeamsQuery, accountID, creTeamID, sreTeamID)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// foreign_key_violation — creTeamID or sreTeamID does not reference an existing team.
			return AccountRow{}, &apierror.ValidationError{Msg: "one or more referenced team IDs do not exist: " + pgErr.Detail}
		}
		return AccountRow{}, fmt.Errorf("update account teams: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return AccountRow{}, &apierror.NotFoundError{Msg: "account not found"}
	}
	return r.GetAccountByID(ctx, accountID)
}

const salesforceSyncActor = domain.SalesforceSyncActor

func (r *accountRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceAccountUpsert) error {
	query := `
		INSERT INTO account (
			id, created_on, updated_on, created_by, updated_by,
			name, number, sf_id,
			industry, region, global_pod, phone, sales_region, sub_region,
			account_vertical, life_cycle, naics_industry, sub_industry,
			classification, technical_owner_id, secondary_technical_owner_id,
			deactivation_date, sync_time_stamp
		) VALUES (
			gen_random_uuid(), now(), now(), $1, $1,
			$2, $3, $4,
			$5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17,
			NULL, now()
		)
		ON CONFLICT (sf_id) DO UPDATE SET
			name = EXCLUDED.name,
			industry = EXCLUDED.industry,
			region = EXCLUDED.region,
			global_pod = EXCLUDED.global_pod,
			phone = CASE WHEN $18 THEN account.phone ELSE EXCLUDED.phone END,
			sales_region = EXCLUDED.sales_region,
			sub_region = EXCLUDED.sub_region,
			account_vertical = EXCLUDED.account_vertical,
			life_cycle = EXCLUDED.life_cycle,
			naics_industry = EXCLUDED.naics_industry,
			sub_industry = EXCLUDED.sub_industry,
			classification = EXCLUDED.classification,
			technical_owner_id = EXCLUDED.technical_owner_id,
			secondary_technical_owner_id = EXCLUDED.secondary_technical_owner_id,
			deactivation_date = NULL,
			updated_on = now(),
			updated_by = EXCLUDED.updated_by,
			sync_time_stamp = now()`
	_, err := r.db.Exec(ctx, query,
		salesforceSyncActor,
		row.Name, row.Number, row.SfID,
		row.Industry, row.Region, row.GlobalPod, row.Phone, row.SalesRegion, row.SubRegion,
		row.AccountVertical, row.LifeCycle, row.NAICSIndustry, row.SubIndustry,
		row.Classification, row.TechnicalOwnerID, row.SecondaryTechnicalOwnerID,
		row.KeepExistingPhone,
	)
	if err != nil {
		return fmt.Errorf("upsert account from salesforce: %w", err)
	}
	return nil
}

func (r *accountRepo) SoftDeleteBySfID(ctx context.Context, sfID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE account
		SET deactivation_date = COALESCE(deactivation_date, CURRENT_DATE),
		    updated_on = now(),
		    updated_by = $2,
		    sync_time_stamp = now()
		WHERE sf_id = $1`, sfID, salesforceSyncActor)
	if err != nil {
		return fmt.Errorf("soft-delete account by sf_id: %w", err)
	}
	return nil
}

func (r *accountRepo) LookupUserIDByEmail(ctx context.Context, email string) (*string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id::text FROM "user" WHERE lower(email) = lower($1)`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user id by email: %w", err)
	}
	return &id, nil
}
