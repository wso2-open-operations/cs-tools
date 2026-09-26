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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProjectMembershipRepository writes a Salesforce Project_Contact__c
// (membership) into the Postgres model: "user", user_role, account_contact,
// project_contact and project_contact_group, in one transaction. It is the
// single write path for customer memberships — the Salesforce ingest, the
// portal's replayed envelopes and any reconcile all end up here.
//
// Rows are resolved by natural keys first (sf_id when already stamped, then
// project key / account sf_id / contact email / (project, account_contact)),
// and every Salesforce id is stamped on the way, so replaying the same
// membership is idempotent even though the tables carry no unique constraints
// on those keys. The DATABASE onboarding step is recorded inside the same
// transaction so it can never disagree with the rows.
type ProjectMembershipRepository interface {
	// Upsert writes one membership. NotFoundError when the project or account
	// is unknown, ConflictError when the contact email matches more than one
	// user, ServiceUnavailableError when a role or project_group row the
	// mapping relies on is missing (the ServiceNow sync seeds them).
	Upsert(ctx context.Context, in domain.SalesforceMembershipUpsert, step domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error)
	// UpsertWithin is Upsert with the transaction held open across a
	// caller-supplied step — the portal writes' Salesforce half. See
	// MembershipWritePlan for the ordering and why it is safe.
	UpsertWithin(ctx context.Context, projectID, email string, plan MembershipWritePlan) (domain.SalesforceMembershipUpsertResult, error)
	// GetMembershipByEmail returns the membership of projectID held by
	// email, outside any transaction. A NotFoundError when there is none.
	GetMembershipByEmail(ctx context.Context, projectID, email string) (domain.ProjectMembershipRow, error)
	// DeactivateBySfID sets project_contact.state = DEACTIVATED for the
	// membership with that Salesforce id and marks its DATABASE onboarding
	// step as applied by a DELETED event, so the ingest's duplicate guard does
	// not treat a later RESTORED/replayed event carrying the same Salesforce
	// LastModifiedDate as already ingested. An unknown id is a no-op (the
	// membership was never ingested), so a DELETED event is idempotent.
	DeactivateBySfID(ctx context.Context, membershipSfID string) (bool, error)
}

// ErrMembershipCommitFailed marks the one failure mode this write ordering
// cannot roll back: Salesforce was written and the database commit then
// failed, leaving a Salesforce record with no row behind it.
//
// The next write for the same person adopts that record instead of creating a
// second one (every Salesforce write here searches first), so the residue
// heals itself — but the caller is told, so it can record the orphan for a
// retry rather than lose it. errors.Is against this sentinel is how the
// service tells "nothing happened anywhere" apart from "Salesforce moved and
// the database did not".
var ErrMembershipCommitFailed = errors.New("membership write committed to Salesforce but not to the database")

// MembershipWriteContext is what the Salesforce half of a portal membership
// write is given: the project and account it lands on, and the membership
// that is already there, if any. Both are read inside the write's own
// transaction, behind a transaction-scoped advisory lock on the (project,
// address) pair, so the plan never decides against a state another writer
// has since changed.
type MembershipWriteContext struct {
	Target domain.MembershipWriteTarget
	// Existing is nil when this project has no membership for the address —
	// i.e. this is a first invitation rather than a change.
	Existing *domain.ProjectMembershipRow
}

// MembershipWritePlan performs the Salesforce half of a portal membership
// write and returns the database half.
//
// It runs INSIDE the transaction that will write the rows, and before the
// commit. That ordering is the whole design:
//
//   - Salesforce has no transaction. Once its write returns, it is final.
//   - PostgreSQL does, and rolling it back costs nothing.
//
// So the participant that can be undone goes last. A Salesforce failure
// returns an error from the plan, the transaction rolls back, and neither
// system moved. The only residue is a commit that fails after Salesforce
// succeeded (ErrMembershipCommitFailed), which the search-first rule on every
// Salesforce write makes self-healing.
//
// The Salesforce half runs before the row write rather than after it because
// the rows need the Salesforce ids: project_contact.sf_id, account_contact.
// sf_id and "user".sf_id are stamped from the records this step creates or
// finds. What matters for correctness is unchanged — the database commit is
// still the last thing that happens.
type MembershipWritePlan func(ctx context.Context, wc MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error)

type projectMembershipRepo struct {
	db *pgxpool.Pool
}

// NewProjectMembershipRepository constructs a ProjectMembershipRepository backed by the pool.
func NewProjectMembershipRepository(db *pgxpool.Pool) ProjectMembershipRepository {
	return &projectMembershipRepo{db: db}
}

func (r *projectMembershipRepo) DeactivateBySfID(ctx context.Context, membershipSfID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("deactivate project contact: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE project_contact
		SET state = $2::project_contact_state_enum, updated_on = NOW(), updated_by = $3
		WHERE sf_id = $1`,
		membershipSfID, domain.MembershipStateDeactivated, domain.SalesforceSyncActor)
	if err != nil {
		return false, fmt.Errorf("deactivate project contact by sf_id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	// Salesforce has no LastModifiedDate to offer for a deleted record, so
	// the step keeps its recorded version and only its event_type changes;
	// the guard in the ingest ignores DELETED-typed steps.
	if _, err := tx.Exec(ctx, `
		UPDATE onboarding_step
		SET event_type = $2, updated_on = NOW(), updated_by = $3
		WHERE membership_sf_id = $1 AND step = 'DATABASE'::onboarding_step_enum`,
		membershipSfID, string(domain.SalesforceEventDeleted), domain.SalesforceSyncActor); err != nil {
		return false, fmt.Errorf("mark DATABASE step deleted: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("deactivate project contact: commit: %w", err)
	}
	return true, nil
}

func (r *projectMembershipRepo) Upsert(ctx context.Context, in domain.SalesforceMembershipUpsert, step domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, fmt.Errorf("upsert membership: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	res, err := upsertMembershipTx(ctx, tx, in)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}

	step.ProjectID = &res.ProjectID
	step.ProjectContactID = &res.ProjectContactID
	if _, err := upsertOnboardingStep(ctx, tx, step); err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.SalesforceMembershipUpsertResult{}, fmt.Errorf("upsert membership: commit: %w", err)
	}
	return res, nil
}

// UpsertWithin implements ProjectMembershipRepository.
func (r *projectMembershipRepo) UpsertWithin(ctx context.Context, projectID, email string, plan MembershipWritePlan) (domain.SalesforceMembershipUpsertResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, fmt.Errorf("membership write: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockMembershipWriteKey(ctx, tx, projectID, email); err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}

	target, err := resolveWriteTarget(ctx, tx, projectID)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}
	existing, err := membershipByEmail(ctx, tx, projectID, email)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}

	// The Salesforce half. An error here leaves both systems untouched: the
	// deferred Rollback discards whatever this transaction has read, and
	// nothing has been written to either side yet.
	in, step, err := plan(ctx, MembershipWriteContext{Target: target, Existing: existing})
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}
	in.ProjectID = target.ProjectID

	res, err := upsertMembershipTx(ctx, tx, in)
	if err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}
	step.ProjectID = &res.ProjectID
	step.ProjectContactID = &res.ProjectContactID
	if _, err := upsertOnboardingStep(ctx, tx, step); err != nil {
		return domain.SalesforceMembershipUpsertResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("%w: %v", ErrMembershipCommitFailed, err)
	}
	return res, nil
}

// lockMembershipWriteKey takes a transaction-scoped advisory lock on the
// (project, address) pair the write is about, before anything is read.
//
// READ COMMITTED gives the plain SELECTs below no protection against a writer
// that has not committed yet, and the read is what decides between "invite"
// and "change": a double-submitted invitation, or two admins inviting the
// same person at once, would otherwise both see no membership, both search
// Salesforce, and both create -- the search-first rule cannot help, because
// both searches run before either create, and the Salesforce create endpoints
// are not idempotent. The second request waits here instead, and then reads
// the membership the first one made.
//
// The lock is released by COMMIT or ROLLBACK, so it lives exactly as long as
// the write does, including the Salesforce calls inside it -- which is the
// point: the window that has to be closed is precisely the one they open.
// Requests for a different address, or for the same address on a different
// project, never touch it.
func lockMembershipWriteKey(ctx context.Context, tx pgx.Tx, projectID, email string) error {
	key := projectID + "|" + strings.ToLower(strings.TrimSpace(email))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, key); err != nil {
		return fmt.Errorf("membership write: lock write key: %w", err)
	}
	return nil
}

// GetMembershipByEmail implements ProjectMembershipRepository.
func (r *projectMembershipRepo) GetMembershipByEmail(ctx context.Context, projectID, email string) (domain.ProjectMembershipRow, error) {
	row, err := membershipByEmail(ctx, r.db, projectID, email)
	if err != nil {
		return domain.ProjectMembershipRow{}, err
	}
	if row == nil {
		return domain.ProjectMembershipRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
	}
	return *row, nil
}

// resolveWriteTarget reads the project a portal write names and the account
// behind it. A project with no account is a NotFoundError rather than a
// half-resolved target: every Salesforce contact is created under an account,
// so there is nothing this write could do without one.
func resolveWriteTarget(ctx context.Context, q querier, projectID string) (domain.MembershipWriteTarget, error) {
	var t domain.MembershipWriteTarget
	var name, accountID, accountSfID *string
	err := q.QueryRow(ctx, `
		SELECT p.id, p.key, p.name, p.sf_id, a.id, a.sf_id
		FROM project p
		LEFT JOIN account a ON a.id = p.account_id
		WHERE p.id = $1`, projectID).Scan(&t.ProjectID, &t.ProjectKey, &name, &t.ProjectSfID, &accountID, &accountSfID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MembershipWriteTarget{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.MembershipWriteTarget{}, fmt.Errorf("membership write: resolve project: %w", err)
	}
	if name != nil {
		t.ProjectName = *name
	}
	if accountID == nil || accountSfID == nil || strings.TrimSpace(*accountSfID) == "" {
		return domain.MembershipWriteTarget{}, &apierror.NotFoundError{Msg: "project has no Salesforce account to add a contact to"}
	}
	t.AccountID, t.AccountSfID = *accountID, *accountSfID
	return t, nil
}

// membershipByEmail reads one membership of projectID by the address it was
// invited under. nil (with no error) when there is none — absence is an
// ordinary answer on the write path: it is what makes a call an invitation
// rather than a change.
func membershipByEmail(ctx context.Context, q querier, projectID, email string) (*domain.ProjectMembershipRow, error) {
	var row domain.ProjectMembershipRow
	var membershipSfID, contactSfID, userSfID, userID, state, firstName, lastName, projectName *string
	var isIntegrationUser *bool
	err := q.QueryRow(ctx, `
		SELECT pc.id, pc.sf_id, ac.sf_id, u.sf_id, u.id, pc.email, pc.state::text,
			COALESCE((
				SELECT array_agg(pg."group")
				FROM project_contact_group pcg
				JOIN project_group pg ON pg.id = pcg.project_group_id
				WHERE pcg.project_contact_id = pc.id
			), ARRAY[]::text[]),
			u.first_name, u.last_name, u.is_system_user,
			p.key, p.name,
			CASE WHEN p.account_id IS NOT NULL AND ac.account_id <> p.account_id
				THEN $3 ELSE $4 END
		FROM project_contact pc
		JOIN project p ON p.id = pc.project_id
		JOIN account_contact ac ON ac.id = pc.account_contact_id
		LEFT JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)
		WHERE pc.project_id = $1 AND LOWER(pc.email) = LOWER($2)
		ORDER BY pc.created_on
		LIMIT 1`, projectID, strings.ToLower(strings.TrimSpace(email)),
		domain.MembershipTypePartnerContact, domain.MembershipTypeOwnContact).
		Scan(&row.ProjectContactID, &membershipSfID, &contactSfID, &userSfID, &userID, &row.Email, &state, &row.ProjectGroups,
			&firstName, &lastName, &isIntegrationUser, &row.ProjectKey, &projectName, &row.Type)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("membership write: resolve membership by email: %w", err)
	}
	if membershipSfID != nil {
		row.MembershipSfID = *membershipSfID
	}
	// The Salesforce Contact Id is stamped on account_contact and on "user";
	// either is the same id, so whichever is present answers.
	if contactSfID != nil && strings.TrimSpace(*contactSfID) != "" {
		row.ContactSfID = *contactSfID
	} else if userSfID != nil {
		row.ContactSfID = *userSfID
	}
	if userID != nil {
		row.UserID = *userID
	}
	if state != nil {
		row.State = *state
	}
	if firstName != nil {
		row.FirstName = *firstName
	}
	if lastName != nil {
		row.LastName = *lastName
	}
	if isIntegrationUser != nil {
		row.IsIntegrationUser = *isIntegrationUser
	}
	if projectName != nil {
		row.ProjectName = *projectName
	}
	return &row, nil
}

// upsertMembershipTx runs the resolution/write steps inside tx.
func upsertMembershipTx(ctx context.Context, tx pgx.Tx, in domain.SalesforceMembershipUpsert) (domain.SalesforceMembershipUpsertResult, error) {
	var res domain.SalesforceMembershipUpsertResult
	actor := in.Actor
	if actor == "" {
		actor = domain.SalesforceSyncActor
	}

	// 1. project — by id when the caller already holds it (the portal writes
	// address a project by its CSM UUID), else by key, then by sf_id.
	var projectAccountID *string
	var err error
	if in.ProjectID != "" {
		err = tx.QueryRow(ctx, `SELECT id, account_id FROM project WHERE id = $1`, in.ProjectID).Scan(&res.ProjectID, &projectAccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &apierror.NotFoundError{Msg: "project not found"}
		}
	} else {
		err = tx.QueryRow(ctx, `
		SELECT id, account_id FROM project
		WHERE (key = $1 AND $1 <> '') OR (sf_id = $2 AND $2 <> '')
		ORDER BY CASE WHEN key = $1 THEN 0 ELSE 1 END
		LIMIT 1`, in.ProjectKey, in.ProjectSfID).Scan(&res.ProjectID, &projectAccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &apierror.NotFoundError{Msg: fmt.Sprintf("project not found for key %q / sfId %q", in.ProjectKey, in.ProjectSfID)}
		}
	}
	if err != nil {
		return res, fmt.Errorf("upsert membership: resolve project: %w", err)
	}

	// 2. account — the contact's own account by sf_id; an own contact falls
	// back to the project's account.
	if in.ContactAccountSfID != "" {
		err = tx.QueryRow(ctx, `SELECT id FROM account WHERE sf_id = $1`, in.ContactAccountSfID).Scan(&res.AccountID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return res, fmt.Errorf("upsert membership: resolve account: %w", err)
		}
	}
	if res.AccountID == "" {
		if !strings.EqualFold(in.Type, domain.MembershipTypePartnerContact) && projectAccountID != nil {
			res.AccountID = *projectAccountID
		} else {
			return res, &apierror.NotFoundError{Msg: fmt.Sprintf("account not found for sfId %q", in.ContactAccountSfID)}
		}
	}

	// 3. user — by sf_id, then by email (must be unique), else insert.
	var userName string
	res.UserID, userName, res.CreatedUser, err = upsertMembershipUser(ctx, tx, in, actor)
	if err != nil {
		return res, err
	}

	// 4. global roles (external + customer/partner). The admin role is NOT
	// decided here — see step 8.
	if err := syncGlobalRoles(ctx, tx, res.UserID, in.GlobalRoles, actor); err != nil {
		return res, err
	}

	// 5. account_contact — by sf_id, then (account, user_name), else insert.
	res.AccountContactID, res.CreatedAccountContact, err = upsertAccountContact(ctx, tx, in.ContactSfID, res.AccountID, userName, actor)
	if err != nil {
		return res, err
	}

	// 6. project_contact — by sf_id, then (project, account_contact), else insert.
	res.ProjectContactID, res.CreatedProjectContact, res.PreviousState, err = upsertProjectContact(ctx, tx, in, res.ProjectID, res.AccountContactID, actor)
	if err != nil {
		return res, err
	}

	// 7. project groups — this is where ADMIN is now recorded, per project.
	if err := syncProjectGroups(ctx, tx, res.ProjectContactID, in.ProjectGroups, actor); err != nil {
		return res, err
	}

	// 8. the derived account-level admin role, computed AFTER step 7 so the
	// membership just written counts. See syncDerivedAdminRole.
	res.IsAccountAdmin, err = syncDerivedAdminRole(ctx, tx, res.UserID, in.AdminRoleName, in.ManagedAdminRoles, in.IsCsAdmin, actor)
	if err != nil {
		return res, err
	}
	return res, nil
}

func upsertMembershipUser(ctx context.Context, tx pgx.Tx, in domain.SalesforceMembershipUpsert, actor string) (id, userName string, created bool, err error) {
	email := strings.ToLower(strings.TrimSpace(in.ContactEmail))
	if email == "" {
		return "", "", false, &apierror.ValidationError{Msg: "contact email is required to resolve the user"}
	}

	err = tx.QueryRow(ctx, `SELECT id, user_name FROM "user" WHERE sf_id = $1`, in.ContactSfID).Scan(&id, &userName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, fmt.Errorf("upsert membership: resolve user by sf_id: %w", err)
	}
	if id == "" {
		rows, qerr := tx.Query(ctx, `SELECT id, user_name FROM "user" WHERE LOWER(email) = $1 LIMIT 2`, email)
		if qerr != nil {
			return "", "", false, fmt.Errorf("upsert membership: resolve user by email: %w", qerr)
		}
		var matches [][2]string
		for rows.Next() {
			var mid, mname string
			if serr := rows.Scan(&mid, &mname); serr != nil {
				rows.Close()
				return "", "", false, fmt.Errorf("upsert membership: scan user: %w", serr)
			}
			matches = append(matches, [2]string{mid, mname})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return "", "", false, fmt.Errorf("upsert membership: iterate users: %w", err)
		}
		switch len(matches) {
		case 0:
		case 1:
			id, userName = matches[0][0], matches[0][1]
		default:
			return "", "", false, &apierror.ConflictError{Msg: "more than one user matches the contact email; cannot resolve the membership"}
		}
	}

	if id == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
				user_name, name, first_name, last_name, email, is_active, is_system_user, sf_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3, $4, $5, $2, TRUE, $6, $7)
			RETURNING id, user_name`,
			actor, email, nullIfBlank(in.ContactName), nullIfBlank(in.ContactFirstName), nullIfBlank(in.ContactLastName),
			in.IsCsIntegrationUser, in.ContactSfID).Scan(&id, &userName)
		if err != nil {
			return "", "", false, fmt.Errorf("upsert membership: insert user: %w", err)
		}
		return id, userName, true, nil
	}

	// Existing row: refresh profile fields and stamp the Salesforce id, but
	// never touch user_name — it is the join key to account_contact.
	if _, err = tx.Exec(ctx, `
		UPDATE "user"
		SET name = COALESCE($2, name), first_name = COALESCE($3, first_name), last_name = COALESCE($4, last_name),
		    email = $5, is_system_user = $6, sf_id = $7, updated_on = NOW(), updated_by = $8
		WHERE id = $1`,
		id, nullIfBlank(in.ContactName), nullIfBlank(in.ContactFirstName), nullIfBlank(in.ContactLastName),
		email, in.IsCsIntegrationUser, in.ContactSfID, actor); err != nil {
		return "", "", false, fmt.Errorf("upsert membership: update user: %w", err)
	}
	return id, userName, false, nil
}

// syncGlobalRoles grants every role in wanted that the user does not already
// hold. It revokes nothing: the only roles this write path is allowed to take
// away are the two admin roles, and those are decided by syncDerivedAdminRole
// from the user's whole set of memberships, not from the one being written.
func syncGlobalRoles(ctx context.Context, tx pgx.Tx, userID string, wanted []string, actor string) error {
	if len(wanted) > 0 {
		roleIDs := map[string]string{}
		rows, err := tx.Query(ctx, `SELECT id, name FROM role WHERE name = ANY($1::text[])`, wanted)
		if err != nil {
			return fmt.Errorf("upsert membership: resolve roles: %w", err)
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return fmt.Errorf("upsert membership: scan role: %w", err)
			}
			roleIDs[name] = id
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("upsert membership: iterate roles: %w", err)
		}
		for _, name := range wanted {
			if _, ok := roleIDs[name]; !ok {
				return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("role %q is not seeded in the role table", name)}
			}
		}

		held := map[string]bool{}
		hrows, err := tx.Query(ctx, `SELECT r.name FROM user_role ur JOIN role r ON r.id = ur.role_id WHERE ur.user_id = $1`, userID)
		if err != nil {
			return fmt.Errorf("upsert membership: query user roles: %w", err)
		}
		for hrows.Next() {
			var name string
			if err := hrows.Scan(&name); err != nil {
				hrows.Close()
				return fmt.Errorf("upsert membership: scan user role: %w", err)
			}
			held[name] = true
		}
		hrows.Close()
		if err := hrows.Err(); err != nil {
			return fmt.Errorf("upsert membership: iterate user roles: %w", err)
		}

		for _, name := range wanted {
			if held[name] {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
				VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3)`, actor, userID, roleIDs[name]); err != nil {
				return fmt.Errorf("upsert membership: grant role %s: %w", name, err)
			}
		}
	}

	return nil
}

// syncDerivedAdminRole decides the user's account-level admin roles from ALL
// of their memberships and grants or revokes each one accordingly.
//
// Admin is stored per project (the ADMIN project_role, reached through the
// Admin project_group — migration 000084). The account-level role is derived
// from it: admin on ANY project under an account means admin on EVERY project
// under that account, and nothing outside it.
//
// "Nothing outside it" is why the two managed roles are decided SEPARATELY,
// each from the memberships that could support it. A membership is a partner
// one exactly when the contact's account is not the project's (the same test
// membershipByEmail applies), so an ADMIN membership of that kind supports
// partner_admin and one of the other kind supports customer_admin. Deciding
// both from a single "is this user an admin anywhere" answer, and then
// keeping whichever role the membership in hand happens to map to, let one
// account's admin rights be traded for another's: processing a non-admin
// PARTNER CONTACT membership for a user who is a customer admin elsewhere
// would grant partner_admin, which no ADMIN membership supports, and revoke
// the customer_admin they had earned.
//
// This also replaces an older revoke rule that read `managed - wanted` from
// the single membership being processed, which meant processing one
// non-admin membership stripped the user's admin on every project they had.
// Neither query here can do that: both are computed over the user's
// memberships, run after the membership in hand has already been written, so
// the answer accounts for the change being made and for every project not
// involved in it. Do not narrow either of them to the membership in hand.
//
// The contact's own Salesforce isCsAdmin flag is an additional grant of the
// role this membership maps to (adminRole), never a revocation condition,
// and is the reason this takes it as a parameter rather than reading it back
// too.
//
// adminRole empty (an integration user) means the whole admin question does
// not apply: nothing is granted and nothing is revoked. The bool returned is
// whether the user ends up holding adminRole — the decision for the
// membership just written.
func syncDerivedAdminRole(ctx context.Context, tx querier, userID, adminRole string, managed []string, isCsAdmin bool, actor string) (bool, error) {
	if adminRole == "" {
		return false, nil
	}

	var customerAdmin, partnerAdmin bool
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(bool_or(ac.account_id = p.account_id), FALSE),
			COALESCE(bool_or(ac.account_id <> p.account_id), FALSE)
		FROM "user" u
		JOIN account_contact ac ON LOWER(ac.user_name) = LOWER(u.user_name)
		JOIN project_contact pc ON pc.account_contact_id = ac.id
		JOIN project p ON p.id = pc.project_id
		JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
		JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
		JOIN project_role pr ON pr.id = pgr.project_role_id
		WHERE u.id = $1
		  AND pr.role = 'ADMIN'::project_role_enum
		  AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum)`,
		userID).Scan(&customerAdmin, &partnerAdmin); err != nil {
		return false, fmt.Errorf("upsert membership: derive account admin role: %w", err)
	}

	earned := map[string]bool{
		globalRoleCustomerAdmin: customerAdmin,
		globalRolePartnerAdmin:  partnerAdmin,
	}
	// isCsAdmin grants the role THIS membership maps to, and only that one.
	if isCsAdmin {
		earned[adminRole] = true
	}

	var revoke []string
	for _, m := range managed {
		if earned[m] {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
				SELECT gen_random_uuid(), NOW(), NOW(), $1, $1, $2, r.id
				FROM role r
				WHERE r.name = $3
				  AND NOT EXISTS (SELECT 1 FROM user_role ur WHERE ur.user_id = $2 AND ur.role_id = r.id)`,
				actor, userID, m); err != nil {
				return false, fmt.Errorf("upsert membership: grant role %s: %w", m, err)
			}
			continue
		}
		revoke = append(revoke, m)
	}
	if len(revoke) > 0 {
		if _, err := tx.Exec(ctx, `
			DELETE FROM user_role ur USING role r
			WHERE ur.role_id = r.id AND ur.user_id = $1 AND r.name = ANY($2::text[])`, userID, revoke); err != nil {
			return false, fmt.Errorf("upsert membership: revoke roles: %w", err)
		}
	}
	return earned[adminRole], nil
}

// The two derived account-level admin role names. internal/service owns the
// Salesforce-to-role mapping and the repository must not import it, so the
// names are spelled here too — syncDerivedAdminRole has to know which of the
// managed roles a partner membership supports and which a customer one does.
const (
	globalRoleCustomerAdmin = "customer_admin"
	globalRolePartnerAdmin  = "partner_admin"
)

func upsertAccountContact(ctx context.Context, tx pgx.Tx, contactSfID, accountID, userName, actor string) (id string, created bool, err error) {
	err = tx.QueryRow(ctx, `SELECT id FROM account_contact WHERE sf_id = $1 AND account_id = $2`, contactSfID, accountID).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("upsert membership: resolve account_contact by sf_id: %w", err)
	}
	if id == "" {
		err = tx.QueryRow(ctx, `
			SELECT id FROM account_contact WHERE account_id = $1 AND LOWER(user_name) = LOWER($2)
			ORDER BY created_on LIMIT 1`, accountID, userName).Scan(&id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", false, fmt.Errorf("upsert membership: resolve account_contact: %w", err)
		}
	}
	if id == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by,
				is_active, user_name, is_primary_contact, account_id, sf_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, TRUE, $2, FALSE, $3, $4)
			RETURNING id`, actor, userName, accountID, contactSfID).Scan(&id)
		if err != nil {
			return "", false, fmt.Errorf("upsert membership: insert account_contact: %w", err)
		}
		return id, true, nil
	}
	if _, err = tx.Exec(ctx, `
		UPDATE account_contact SET is_active = TRUE, sf_id = $2, updated_on = NOW(), updated_by = $3 WHERE id = $1`,
		id, contactSfID, actor); err != nil {
		return "", false, fmt.Errorf("upsert membership: update account_contact: %w", err)
	}
	return id, false, nil
}

// upsertProjectContact writes the membership row and reports the state it
// held beforehand. previousState is empty when the row was created here; the
// ingest's echo suppression reads it to tell "Salesforce moved this state"
// apart from "this is our own portal write coming back".
func upsertProjectContact(ctx context.Context, tx pgx.Tx, in domain.SalesforceMembershipUpsert, projectID, accountContactID, actor string) (id string, created bool, previousState string, err error) {
	var prior *string
	err = tx.QueryRow(ctx, `SELECT id, state::text FROM project_contact WHERE sf_id = $1`, in.MembershipSfID).Scan(&id, &prior)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", false, "", fmt.Errorf("upsert membership: resolve project_contact by sf_id: %w", err)
	}
	if id == "" {
		err = tx.QueryRow(ctx, `
			SELECT id, state::text FROM project_contact WHERE project_id = $1 AND account_contact_id = $2
			ORDER BY created_on LIMIT 1`, projectID, accountContactID).Scan(&id, &prior)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", false, "", fmt.Errorf("upsert membership: resolve project_contact: %w", err)
		}
	}
	if prior != nil {
		previousState = *prior
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if id == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by,
				email, state, account_contact_id, project_id, sf_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3::project_contact_state_enum, $4, $5, $6)
			RETURNING id`, actor, email, in.State, accountContactID, projectID, in.MembershipSfID).Scan(&id)
		if err != nil {
			return "", false, "", fmt.Errorf("upsert membership: insert project_contact: %w", err)
		}
		return id, true, "", nil
	}
	if _, err = tx.Exec(ctx, `
		UPDATE project_contact
		SET email = $2, state = $3::project_contact_state_enum, account_contact_id = $4, sf_id = $5,
		    updated_on = NOW(), updated_by = $6
		WHERE id = $1`,
		id, email, in.State, accountContactID, in.MembershipSfID, actor); err != nil {
		return "", false, "", fmt.Errorf("upsert membership: update project_contact: %w", err)
	}
	return id, false, previousState, nil
}

func syncProjectGroups(ctx context.Context, tx pgx.Tx, projectContactID string, groups []string, actor string) error {
	groupIDs := map[string]string{}
	if len(groups) > 0 {
		rows, err := tx.Query(ctx, `SELECT id, "group" FROM project_group WHERE "group" = ANY($1::text[])`, groups)
		if err != nil {
			return fmt.Errorf("upsert membership: resolve project groups: %w", err)
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return fmt.Errorf("upsert membership: scan project group: %w", err)
			}
			groupIDs[name] = id
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("upsert membership: iterate project groups: %w", err)
		}
		for _, g := range groups {
			if _, ok := groupIDs[g]; !ok {
				return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("project group %q is not present in project_group", g)}
			}
		}
	}

	wanted := make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		wanted = append(wanted, id)
	}
	// Remove memberships outside the target set (an empty set removes all).
	if _, err := tx.Exec(ctx, `
		DELETE FROM project_contact_group
		WHERE project_contact_id = $1 AND NOT (project_group_id::text = ANY($2::text[]))`,
		projectContactID, wanted); err != nil {
		return fmt.Errorf("upsert membership: remove project groups: %w", err)
	}
	// Add the missing ones.
	for _, gid := range wanted {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_group_id, project_contact_id)
			SELECT gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3
			WHERE NOT EXISTS (
				SELECT 1 FROM project_contact_group WHERE project_group_id = $2 AND project_contact_id = $3)`,
			actor, gid, projectContactID); err != nil {
			return fmt.Errorf("upsert membership: add project group: %w", err)
		}
	}
	return nil
}

func nullIfBlank(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}
