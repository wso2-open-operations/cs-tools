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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SalesforceContactRepository is the Contact writer's store: it owns the
// "user" and account_contact rows of a Salesforce Contact and the
// contact-derived part of user_role, independently of any membership, so a
// contact with no project membership (a commercial or billing contact) is
// still represented in CSM. The membership upsert (ProjectMembershipRepository)
// stays the single writer of project_contact; both resolve "user" and
// account_contact through the same helpers, so they agree on the rows.
//
// Every write records the salesforce_ingest_state ledger row
// (domain.SalesforceIngestEntityContact) inside its own transaction.
type SalesforceContactRepository interface {
	// Upsert writes one contact in one transaction: "user" (by sf_id, then by
	// unique email), the global roles, account_contact on in.AccountID (by
	// sf_id, then user_name) with is_primary_contact, the deactivation of
	// the contact's account_contact rows on accounts it has moved away from,
	// the derived admin role, and the ledger row. ConflictError when the
	// email matches more than one user.
	Upsert(ctx context.Context, in domain.SalesforceContactUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceContactUpsertResult, error)
	// DeactivateBySfID soft-deletes a contact Salesforce has deleted:
	// account_contact.is_active and "user".is_active go FALSE by sf_id, and
	// the ledger row is stamped DELETED (keeping its recorded version, as
	// the membership DELETED path does) so a later RESTORED is not skipped as
	// a duplicate. Nothing is hard-deleted: project_contact and
	// onboarding_step reference these rows. found is false when no row
	// carried the id; the ledger is written either way. affected lists the
	// "user" rows that were deactivated (sf_id is not unique there).
	DeactivateBySfID(ctx context.Context, contactSfID string, state domain.UpsertSalesforceIngestStateRequest) (found bool, affected []domain.AffectedUser, err error)
}

type salesforceContactRepo struct {
	db db.Pool
}

// NewSalesforceContactRepository constructs a SalesforceContactRepository backed by the pool.
func NewSalesforceContactRepository(db db.Pool) SalesforceContactRepository {
	return &salesforceContactRepo{db: db}
}

// lockSalesforceContact serialises writers of one Salesforce Contact for the
// life of the transaction. sf_id is not unique on "user" or account_contact
// (migration 0095), so two concurrent events for a new contact would
// otherwise both miss the lookup and both insert. Both the Contact writer and
// the membership upsert take it before resolving "user" and account_contact.
// A blank id takes no lock: there is no contact to serialise on.
func lockSalesforceContact(ctx context.Context, tx querier, contactSfID string) error {
	if strings.TrimSpace(contactSfID) == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, "salesforce-contact|"+contactSfID); err != nil {
		return fmt.Errorf("lock salesforce contact: %w", err)
	}
	return nil
}

func (r *salesforceContactRepo) Upsert(ctx context.Context, in domain.SalesforceContactUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceContactUpsertResult, error) {
	var res domain.SalesforceContactUpsertResult
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("upsert contact: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSalesforceContact(ctx, tx, in.ContactSfID); err != nil {
		return res, err
	}
	res, err = upsertContactTx(ctx, tx, in)
	if err != nil {
		return res, err
	}
	if _, err := upsertSalesforceIngestState(ctx, tx, state); err != nil {
		return res, err
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("upsert contact: commit: %w", err)
	}
	return res, nil
}

// upsertContactTx runs the Contact writer's steps inside tx.
func upsertContactTx(ctx context.Context, tx querier, in domain.SalesforceContactUpsert) (domain.SalesforceContactUpsertResult, error) {
	var res domain.SalesforceContactUpsertResult
	actor := domain.SalesforceSyncActor

	// The ledger row as it stood before this write: a contact this writer
	// soft-deleted (DELETED) and Salesforce has since restored gets its
	// "user" row back. Only that case — is_active is never forced TRUE
	// otherwise, so a user deactivated for another reason stays so.
	prior, err := getSalesforceIngestState(ctx, tx, domain.SalesforceIngestEntityContact, in.ContactSfID)
	if err != nil {
		return res, err
	}

	// 1. user — by sf_id, then by email (must be unique), else insert. The
	// membership upsert's own helper, so both writers resolve the same row.
	var userName string
	res.UserID, userName, res.CreatedUser, err = upsertMembershipUser(ctx, tx, domain.SalesforceMembershipUpsert{
		ContactSfID:         in.ContactSfID,
		ContactEmail:        in.Email,
		ContactName:         in.Name,
		ContactFirstName:    in.FirstName,
		ContactLastName:     in.LastName,
		IsCsIntegrationUser: in.IsCsIntegrationUser,
	}, actor)
	if err != nil {
		return res, err
	}
	if prior != nil && prior.EventType == domain.SalesforceEventDeleted && !res.CreatedUser {
		if _, err := tx.Exec(ctx, `
			UPDATE "user" SET is_active = TRUE, updated_on = NOW(), updated_by = $2 WHERE id = $1`,
			res.UserID, actor); err != nil {
			return res, fmt.Errorf("upsert contact: reactivate user: %w", err)
		}
	}

	// 2. global roles: external plus the customer/partner pair.
	if err := syncGlobalRoles(ctx, tx, res.UserID, in.GlobalRoles, in.ManagedGlobalRoles, actor); err != nil {
		return res, err
	}

	// 3. account_contact on the contact's current account.
	res.AccountContactID, res.CreatedAccountContact, err = upsertAccountContact(ctx, tx, in.ContactSfID, in.AccountID, userName, in.IsPrimaryContact, actor)
	if err != nil {
		return res, err
	}

	// 4. account move.
	res.DeactivatedAccountContacts, err = deactivateMovedAccountContacts(ctx, tx, in.ContactSfID, in.AccountID, userName, actor)
	if err != nil {
		return res, err
	}

	// 5. the derived admin role, from every membership plus isCsAdmin.
	res.IsAccountAdmin, err = syncDerivedAdminRole(ctx, tx, res.UserID, in.AdminRoleName, in.ManagedAdminRoles, in.IsCsAdmin, actor)
	if err != nil {
		return res, err
	}
	return res, nil
}

// deactivateMovedAccountContacts deactivates the contact's account_contact
// rows on accounts other than its current one — the contact has moved away
// from them in Salesforce — except where the contact still holds a live
// (non-DEACTIVATED) membership on a project of that account: a partner
// contact legitimately keeps a row there. The memberships that point at a
// deactivated row are re-pointed at the new account's row by the membership
// fan-out that follows the contact write. Returns how many rows it
// deactivated.
func deactivateMovedAccountContacts(ctx context.Context, tx querier, contactSfID, accountID, userName, actor string) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE account_contact ac
		SET is_active = FALSE, updated_on = NOW(), updated_by = $4
		WHERE ac.sf_id = $1
		  AND ac.account_id <> $2
		  AND ac.is_active IS DISTINCT FROM FALSE
		  AND NOT EXISTS (
			SELECT 1
			FROM project_contact pc
			JOIN account_contact mac ON mac.id = pc.account_contact_id
			JOIN project p ON p.id = pc.project_id
			WHERE (mac.sf_id = $1 OR LOWER(mac.user_name) = LOWER($3))
			  AND p.account_id = ac.account_id
			  AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum))`,
		contactSfID, accountID, userName, actor)
	if err != nil {
		return 0, fmt.Errorf("upsert contact: deactivate moved account_contact rows: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *salesforceContactRepo) DeactivateBySfID(ctx context.Context, contactSfID string, state domain.UpsertSalesforceIngestStateRequest) (bool, []domain.AffectedUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("deactivate contact: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSalesforceContact(ctx, tx, contactSfID); err != nil {
		return false, nil, err
	}
	actor := domain.SalesforceSyncActor
	acTag, err := tx.Exec(ctx, `
		UPDATE account_contact SET is_active = FALSE, updated_on = NOW(), updated_by = $2 WHERE sf_id = $1`,
		contactSfID, actor)
	if err != nil {
		return false, nil, fmt.Errorf("deactivate contact: account_contact: %w", err)
	}
	rows, err := tx.Query(ctx, `
		UPDATE "user" SET is_active = FALSE, updated_on = NOW(), updated_by = $2 WHERE sf_id = $1
		RETURNING id::text, COALESCE(email, '')`,
		contactSfID, actor)
	if err != nil {
		return false, nil, fmt.Errorf("deactivate contact: user: %w", err)
	}
	var affected []domain.AffectedUser
	for rows.Next() {
		var u domain.AffectedUser
		if err := rows.Scan(&u.ID, &u.Email); err != nil {
			rows.Close()
			return false, nil, fmt.Errorf("deactivate contact: user: scan: %w", err)
		}
		affected = append(affected, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, nil, fmt.Errorf("deactivate contact: user: %w", err)
	}

	// Salesforce has no LastModifiedDate to offer for a deleted record, so
	// the ledger keeps the version it recorded and only its event_type
	// moves to DELETED — which the duplicate guard never lets block a later
	// RESTORED. A contact never ingested before gets a row stamped now.
	prior, err := getSalesforceIngestState(ctx, tx, domain.SalesforceIngestEntityContact, contactSfID)
	if err != nil {
		return false, nil, err
	}
	if prior != nil {
		state.EventModifiedOn = prior.EventModifiedOn
	} else if state.EventModifiedOn.IsZero() {
		state.EventModifiedOn = time.Now().UTC()
	}
	if _, err := upsertSalesforceIngestState(ctx, tx, state); err != nil {
		return false, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, nil, fmt.Errorf("deactivate contact: commit: %w", err)
	}
	return acTag.RowsAffected() > 0 || len(affected) > 0, affected, nil
}
