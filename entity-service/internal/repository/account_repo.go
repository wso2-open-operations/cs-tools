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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// AccountRow is the raw shape of one row read from the account table, together
// with the joined technical-owner/account-manager person refs and the joined
// CRE/SRE team refs. It is mapped to domain.AccountView / domain.AccountDetail
// by the service layer.
type AccountRow struct {
	ID                          string
	Name                        string
	Number                      string
	Classification              *string
	Pod                         *string
	SfID                        *string
	Region                      *string
	SubRegion                   *string
	Country                     *string
	City                        *string
	DriveLocation               *string
	ActivationDate              *time.Time
	DeactivationDate            *time.Time
	TechnicalOwnerID            *string
	TechnicalOwnerName          *string
	TechnicalOwnerEmail         *string
	AccountManagerID            *string
	AccountManagerName          *string
	AccountManagerEmail         *string
	RenewalAccountManagerID     *string
	RenewalAccountManagerName   *string
	RenewalAccountManagerEmail  *string
	CustomerSuccessManagerID    *string
	CustomerSuccessManagerName  *string
	CustomerSuccessManagerEmail *string
	CreTeamID                   *string
	CreTeamName                 *string
	SreTeamID                   *string
	SreTeamName                 *string
	HasAgent                    *bool
	HasKbReferences             *bool
	// HasPrimaryPartner is whether the account has a primary partner
	// (account.primary_partner_sf_id, ServiceNow's u_primary_partner_account_id).
	// Other partner links in account_relationship do not count.
	HasPrimaryPartner bool
	CreatedOn  time.Time
	CreatedBy  string
	UpdatedOn  time.Time
	// Health is set only by a SearchAccounts call whose filters asked for the
	// health summary (see AccountHealthRow); nil on every other path.
	Health *AccountHealthRow
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
	// GetAccountHealthDetail returns the per-project customer-health breakdown of
	// the account with the given UUID, or a NotFoundError if no such account
	// exists. See account_health_detail.go for the statements it runs.
	GetAccountHealthDetail(ctx context.Context, id string) (AccountHealthDetailRow, error)
	// UpdateAccountTeams sets the account's CRE and/or SRE team. A nil
	// creTeamID/sreTeamID leaves that assignment unchanged; there is no way
	// to explicitly clear an assignment to "no team" via this method (see
	// its caller, AccountService.UpdateAccountTeams, for why). Returns a
	// ValidationError if either non-nil id does not reference an existing
	// team row, or a NotFoundError if the account does not exist.
	UpdateAccountTeams(ctx context.Context, accountID string, creTeamID, sreTeamID *string) (AccountRow, error)
	// UpsertFromSalesforce writes one Salesforce Account and records state
	// in salesforce_ingest_state within the same transaction.
	UpsertFromSalesforce(ctx context.Context, row domain.SalesforceAccountUpsert, state domain.UpsertSalesforceIngestStateRequest) error
	// SoftDeleteBySfID sets deleted_on on the accounts carrying this
	// Salesforce id and records state (a DELETED ledger row) within the same
	// transaction. found is false when no account carries the id.
	SoftDeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (found bool, err error)
	LookupUserIDByEmail(ctx context.Context, email string) (*string, error)
	// LookupAccountIDBySfID returns the account row the ingest writes for this
	// Salesforce id (resolveAccountBySfIDQuery), or nil when there is none.
	LookupAccountIDBySfID(ctx context.Context, sfID string) (*string, error)
}

// accountRepo's Salesforce ingest methods (UpsertFromSalesforce,
// SoftDeleteBySfID, LookupUserIDByEmail, LookupAccountIDBySfID) run as the
// system: they have no caller to inherit an identity from (the Salesforce
// webhook and the retry worker carry none), and which duplicate sf_id row they
// pick is ranked by accountReferencedOrder's EXISTS over work_item, which must
// not depend on who triggered the ingest. The search/get/patch methods serve
// internal callers only (routes.go wraps them in internalOnly) and use the
// caller's own identity.
type accountRepo struct {
	db *Scoped
}

// NewAccountRepository constructs an AccountRepository backed by the given scoped connection pool.
func NewAccountRepository(db *Scoped) AccountRepository {
	return &accountRepo{db: db}
}

// accountSelectColumns' cre/sre joins are the same "group" table
// change_request_repo.go's own customer_group_id join already uses (see
// that file's changeRequestDetailJoins) -- account.cre_team_id/sre_team_id
// (renamed/added by migration 0075, ex-integration_cs_team_id) are real
// FKs into "group" now, unlike when CreTeam/SreTeam were first documented
// as "ServiceNow data source only" on domain.AccountView/AccountDetail;
// this is what actually reads them back for the Postgres data source.
// number/country/city/drive_location and the customer-success-manager join
// were added to expose columns that were already present and populated on
// the account table but never selected here — confirmed against the actual
// migration (000008_accounts_table.up.sql): "number" is NOT NULL UNIQUE, not
// a data gap. ARR and support tier are NOT included here because they
// genuinely are not in this schema — see accountRowCommonFields' own doc
// comment in account_service.go.
const accountSelectColumns = `
	a.id, a.name, a.number, a.classification, a.global_pod, a.sf_id, a.region, a.sub_region,
	a.country, a.city, a.drive_location,
	a.activation_date, a.deactivation_date,
	tow.id, COALESCE(tow.name, NULLIF(TRIM(CONCAT_WS(' ', tow.first_name, tow.last_name)), '')), tow.email,
	mgr.id, COALESCE(mgr.name, NULLIF(TRIM(CONCAT_WS(' ', mgr.first_name, mgr.last_name)), '')), mgr.email,
	ram.id, COALESCE(ram.name, NULLIF(TRIM(CONCAT_WS(' ', ram.first_name, ram.last_name)), '')), ram.email,
	csm.id, COALESCE(csm.name, NULLIF(TRIM(CONCAT_WS(' ', csm.first_name, csm.last_name)), '')), csm.email,
	cre.id, cre.name, sre.id, sre.name,
	a.ai_gen_response_enabled, a.smart_knowledge_base_suggestions_enabled,
	COALESCE(TRIM(a.primary_partner_sf_id), '') <> '',
	a.created_on, a.created_by, a.updated_on`

// accountLeftJoins is every join accountSelectColumns and the search filters
// refer to, without the FROM clause, so a query that reaches account through
// another relation (the health-summary page) can reuse it.
const accountLeftJoins = `
	LEFT JOIN "user" tow ON tow.id = a.technical_owner_id
	LEFT JOIN "user" mgr ON mgr.id = a.account_manager_id
	LEFT JOIN "user" ram ON ram.id = a.renewal_account_manager_id
	LEFT JOIN "user" csm ON csm.id = a.customer_success_manager_id
	LEFT JOIN "group" cre ON cre.id = a.cre_team_id
	LEFT JOIN "group" sre ON sre.id = a.sre_team_id`

const accountFromJoins = `
	FROM account a` + accountLeftJoins

// accountRowScanDests returns the scan destinations for accountSelectColumns,
// in column order.
func accountRowScanDests(a *AccountRow) []any {
	return []any{
		&a.ID, &a.Name, &a.Number, &a.Classification, &a.Pod, &a.SfID, &a.Region, &a.SubRegion,
		&a.Country, &a.City, &a.DriveLocation,
		&a.ActivationDate, &a.DeactivationDate,
		&a.TechnicalOwnerID, &a.TechnicalOwnerName, &a.TechnicalOwnerEmail,
		&a.AccountManagerID, &a.AccountManagerName, &a.AccountManagerEmail,
		&a.RenewalAccountManagerID, &a.RenewalAccountManagerName, &a.RenewalAccountManagerEmail,
		&a.CustomerSuccessManagerID, &a.CustomerSuccessManagerName, &a.CustomerSuccessManagerEmail,
		&a.CreTeamID, &a.CreTeamName, &a.SreTeamID, &a.SreTeamName,
		&a.HasAgent, &a.HasKbReferences, &a.HasPrimaryPartner,
		&a.CreatedOn, &a.CreatedBy, &a.UpdatedOn,
	}
}

func scanAccountRow(row interface{ Scan(...any) error }) (AccountRow, error) {
	var a AccountRow
	err := row.Scan(accountRowScanDests(&a)...)
	return a, err
}

// SearchAccounts implements AccountRepository.
func (r *accountRepo) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) ([]AccountRow, int, error) {
	where, filterArgs, argIdx := buildAccountSearchWhere(req.Filters)

	if accountSearchNeedsHealth(req.Filters) {
		return r.searchAccountsWithHealth(ctx, req, where, filterArgs, argIdx)
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

// UpsertFromSalesforce writes one Salesforce Account and its
// salesforce_ingest_state row in one transaction. account.sf_id is not
// unique (migration 0095 dropped the constraint, so ON CONFLICT (sf_id) has
// nothing to arbitrate on), so the row is resolved by hand, serialised per
// sf_id by an advisory lock so two concurrent events for a new account
// cannot both insert:
//
//  1. update the one row resolveAccountBySfIDQuery picks for this sf_id;
//  2. else link the row with the same account number that has no sf_id yet
//     (a ServiceNow-synced row), rather than tripping account_number_key;
//  3. else insert.
//
// The UPDATE lists only the columns Salesforce owns (see
// updateAccountFromSalesforceQuery); the CSM-only columns are absent from it,
// so a Salesforce event can never blank them. The upsert also clears
// deleted_on, which is how a RESTORED event (or any later CREATED/UPDATED)
// brings a soft-deleted account back.
func (r *accountRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceAccountUpsert, state domain.UpsertSalesforceIngestStateRequest) error {
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		return upsertAccountFromSalesforce(ctx, tx, row, state)
	})
}

// upsertAccountFromSalesforce is UpsertFromSalesforce's body, run on q (the
// transaction).
func upsertAccountFromSalesforce(ctx context.Context, q querier, row domain.SalesforceAccountUpsert, state domain.UpsertSalesforceIngestStateRequest) error {
	if err := lockAccountSfID(ctx, q, row.SfID); err != nil {
		return fmt.Errorf("upsert account from salesforce: %w", err)
	}

	args := []any{
		salesforceSyncActor,
		row.Name, row.Number, row.SfID,
		row.Industry, row.Region, row.GlobalPod, row.Phone, row.SalesRegion, row.SubRegion,
		row.LifeCycle, row.NAICSIndustry, row.SubIndustry, row.Classification, row.TechnicalOwnerID,
		row.Street, row.City, row.StateProvince, row.PostalCode, row.Country,
		row.AccountManagerID, row.ActivationDate, row.LostDate, row.LostReason,
		row.CustomerSuccessManagerID, row.SecondaryTechnicalOwnerID, row.RenewalAccountManagerID,
		row.AccountVertical, row.LostReasonCategory, row.DeactivationDate,
		row.PrimaryPartnerSfID,
		row.KeepExistingPhone, row.KeepExistingPrimaryPartner,
	}
	_, n, err := updateOneBySfID(ctx, q, updateAccountBySfIDQuery, "account", row.SfID, args...)
	if err != nil {
		return fmt.Errorf("upsert account from salesforce: update by sf_id: %w", err)
	}
	if n > 1 {
		// Soft delete marks every copy, so the restore clears every copy.
		if _, err := q.Exec(ctx, restoreAccountCopiesQuery, row.SfID, salesforceSyncActor); err != nil {
			return fmt.Errorf("upsert account from salesforce: restore copies: %w", err)
		}
	}
	linked := int64(0)
	if n == 0 && row.Number != "" {
		tag, err := q.Exec(ctx, updateAccountFromSalesforceQuery+` WHERE number = $3 AND sf_id IS NULL`, args...)
		if err != nil {
			return fmt.Errorf("upsert account from salesforce: link by number: %w", err)
		}
		linked = tag.RowsAffected()
	}
	if n == 0 && linked == 0 {
		if _, err := q.Exec(ctx, insertAccountFromSalesforceQuery, args[:31]...); err != nil {
			return fmt.Errorf("upsert account from salesforce: insert: %w", err)
		}
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return fmt.Errorf("upsert account from salesforce: %w", err)
	}
	return nil
}

// lockAccountSfID takes the per-sf_id transaction lock the upsert and the
// soft delete share, so a DELETED and an UPDATED for the same account apply
// one after the other.
func lockAccountSfID(ctx context.Context, q querier, sfID string) error {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "account-sf:"+sfID); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	return nil
}

// updateAccountFromSalesforceQuery is completed with a WHERE clause by
// upsertAccountFromSalesforce; its parameters match
// insertAccountFromSalesforceQuery plus $32 (keep the existing phone) and $33
// (keep the existing primary partner).
//
// $25-$30 are the SE-1 columns (customer success manager, secondary
// technical owner, renewal manager, account vertical, lost reason category,
// deactivation date). Sales Entity does not send them yet, so they are
// COALESCEd with the stored value rather than assigned: a NULL keeps what the
// ServiceNow sync loaded. Switch them to plain assignment once SE-1 is
// deployed, so a value cleared in Salesforce clears here too.
//
// $31 is the primary partner. It is assigned as sent (a partner removed in
// Salesforce clears here too, so hasPrimaryPartner turns false), unless $33
// says Sales Entity did not send the field at all.
//
// number keeps its stored value: it is NOT NULL, so the COALESCE always
// picks account.number (ServiceNow's "ACC" auto-number, or the Salesforce Id
// an earlier insert wrote). It stays in the list only so $3 is referenced
// in the WHERE sf_id = $4 variant, where Postgres would otherwise fail to
// infer its type.
const updateAccountFromSalesforceQuery = `
	UPDATE account SET
		name = $2,
		number = COALESCE(account.number, NULLIF($3::text, '')),
		sf_id = $4,
		industry = $5,
		region = $6,
		global_pod = $7,
		phone = CASE WHEN $32 THEN account.phone ELSE $8 END,
		sales_region = $9,
		sub_region = $10,
		life_cycle = $11,
		naics_industry = $12,
		sub_industry = $13,
		classification = $14,
		technical_owner_id = $15,
		street = $16,
		city = $17,
		state_province = $18,
		postal_code = $19,
		country = $20,
		account_manager_id = $21,
		activation_date = $22,
		lost_date = $23,
		lost_reason = $24,
		customer_success_manager_id = COALESCE($25, account.customer_success_manager_id),
		secondary_technical_owner_id = COALESCE($26, account.secondary_technical_owner_id),
		renewal_account_manager_id = COALESCE($27, account.renewal_account_manager_id),
		account_vertical = COALESCE($28, account.account_vertical),
		lost_reason_category = COALESCE($29, account.lost_reason_category),
		deactivation_date = COALESCE($30, account.deactivation_date),
		primary_partner_sf_id = CASE WHEN $33 THEN account.primary_partner_sf_id ELSE $31 END,
		deleted_on = NULL,
		updated_on = now(),
		updated_by = $1,
		sync_time_stamp = now()`

// updateAccountBySfIDQuery writes the one row resolveAccountBySfIDQuery picks.
const updateAccountBySfIDQuery = updateAccountFromSalesforceQuery + `
	FROM (SELECT a.id, count(*) OVER () AS n FROM account a WHERE a.sf_id = $4
		ORDER BY ` + accountReferencedOrder + ` LIMIT 1) t
	WHERE account.id = t.id
	RETURNING account.id::text, t.n`

// restoreAccountCopiesQuery clears deleted_on on every copy of an sf_id.
const restoreAccountCopiesQuery = `
	UPDATE account SET deleted_on = NULL, updated_on = now(), updated_by = $2, sync_time_stamp = now()
	WHERE sf_id = $1 AND deleted_on IS NOT NULL`

// insertAccountFromSalesforceQuery creates an account Salesforce knows and
// CSM does not. number is the Salesforce Id: nobody issues "ACC" numbers
// once ServiceNow is gone (decision D9 in SALESFORCE_SYNC_PLAN.md).
const insertAccountFromSalesforceQuery = `
	INSERT INTO account (
		id, created_on, updated_on, created_by, updated_by,
		name, number, sf_id,
		industry, region, global_pod, phone, sales_region, sub_region,
		life_cycle, naics_industry, sub_industry, classification, technical_owner_id,
		street, city, state_province, postal_code, country,
		account_manager_id, activation_date, lost_date, lost_reason,
		customer_success_manager_id, secondary_technical_owner_id, renewal_account_manager_id,
		account_vertical, lost_reason_category, deactivation_date,
		primary_partner_sf_id,
		sync_time_stamp
	) VALUES (
		gen_random_uuid(), now(), now(), $1, $1,
		$2, $3, $4,
		$5, $6, $7, $8, $9, $10,
		$11, $12, $13, $14, $15,
		$16, $17, $18, $19, $20,
		$21, $22, $23, $24,
		$25, $26, $27,
		$28, $29, $30,
		$31,
		now()
	)`

// SoftDeleteBySfID marks every account carrying this Salesforce id as deleted
// in Salesforce (deleted_on, migration 0171) and records the DELETED ledger
// row, in one transaction. The account row stays: its projects, cases and
// contacts still reference it, and a Salesforce merge deletes the losing
// account while its children move to the winner. deactivation_date (the
// contract end date Salesforce owns) is not touched. found is false when no
// account carries the id; the ledger row is written regardless, so a later
// RESTORED is never mistaken for a duplicate.
func (r *accountRepo) SoftDeleteBySfID(ctx context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (bool, error) {
		return softDeleteAccountBySfID(ctx, tx, sfID, state)
	})
}

// softDeleteAccountBySfID is SoftDeleteBySfID's body, run on q (the
// transaction). A repeated DELETED keeps the first deleted_on.
func softDeleteAccountBySfID(ctx context.Context, q querier, sfID string, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	if err := lockAccountSfID(ctx, q, sfID); err != nil {
		return false, fmt.Errorf("soft-delete account by sf_id: %w", err)
	}
	tag, err := q.Exec(ctx, `
		UPDATE account
		SET deleted_on = COALESCE(deleted_on, now()),
		    updated_on = now(),
		    updated_by = $2,
		    sync_time_stamp = now()
		WHERE sf_id = $1`, sfID, salesforceSyncActor)
	if err != nil {
		return false, fmt.Errorf("soft-delete account by sf_id: %w", err)
	}
	if _, err := upsertSalesforceIngestState(ctx, q, state); err != nil {
		return false, fmt.Errorf("soft-delete account by sf_id: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *accountRepo) LookupUserIDByEmail(ctx context.Context, email string) (*string, error) {
	ctx = WithSystemIdentity(ctx)
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

func (r *accountRepo) LookupAccountIDBySfID(ctx context.Context, sfID string) (*string, error) {
	return resolveIDBySfID(WithSystemIdentity(ctx), r.db, resolveAccountBySfIDQuery, "account", sfID)
}

// buildAccountSearchWhere renders the WHERE clause for an account search over
// accountFromJoins (aliases a, tow, mgr, ram, cre, sre), its bound arguments,
// and the next free placeholder index.
func buildAccountSearchWhere(f domain.SearchAccountsFilters) (string, []any, int) {
	filterArgs := []any{}
	argIdx := 1

	// An account deleted in Salesforce (deleted_on, migration 0171) is not a
	// live account and is left out of every list; GetAccountByID still
	// resolves it, so the projects and cases that reference it keep working.
	where := "WHERE a.deleted_on IS NULL"

	if f.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.SearchQuery)
		pattern := "%" + escaped + "%"
		// a.number added so a caller can resolve an account by its
		// ServiceNow-style number (e.g. "ACC0001") the same way case number
		// resolution already works elsewhere — search, then match the exact
		// "number" field in the response (see this repo's own AccountRow.Number).
		where += fmt.Sprintf(" AND (a.name ILIKE $%d ESCAPE '\\' OR a.sf_id ILIKE $%d ESCAPE '\\' OR a.number ILIKE $%d ESCAPE '\\')", argIdx, argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}
	if f.Pod != "" {
		where += fmt.Sprintf(" AND a.global_pod = $%d", argIdx)
		filterArgs = append(filterArgs, f.Pod)
		argIdx++
	}
	if f.Classification != "" {
		where += fmt.Sprintf(" AND a.classification = $%d", argIdx)
		filterArgs = append(filterArgs, f.Classification)
		argIdx++
	}
	if f.OwnerEmail != "" {
		where += fmt.Sprintf(" AND (lower(tow.email) = lower($%d) OR lower(mgr.email) = lower($%d) OR lower(ram.email) = lower($%d))", argIdx, argIdx, argIdx)
		filterArgs = append(filterArgs, f.OwnerEmail)
		argIdx++
	}
	if f.Active != nil {
		if *f.Active {
			where += " AND a.deactivation_date IS NULL"
		} else {
			where += " AND a.deactivation_date IS NOT NULL"
		}
	}
	if f.LifeCycle != "" {
		where += fmt.Sprintf(" AND a.life_cycle = $%d", argIdx)
		filterArgs = append(filterArgs, f.LifeCycle)
		argIdx++
	}
	if f.HasProject {
		where += " AND EXISTS (SELECT 1 FROM project p WHERE p.account_id = a.id)"
	}

	if len(f.SubRegion) > 0 {
		where += fmt.Sprintf(" AND a.sub_region = ANY($%d::text[])", argIdx)
		filterArgs = append(filterArgs, f.SubRegion)
		argIdx++
	}
	if team := strings.TrimSpace(f.AbtTeam); team != "" {
		where += fmt.Sprintf(" AND (lower(cre.name) = lower($%d) OR lower(sre.name) = lower($%d))", argIdx, argIdx)
		filterArgs = append(filterArgs, team)
		argIdx++
	}
	if product := strings.TrimSpace(f.Product); product != "" {
		where += fmt.Sprintf(" AND "+accountHasProductPredicate, argIdx)
		filterArgs = append(filterArgs, product)
		argIdx++
	}

	return where, filterArgs, argIdx
}
