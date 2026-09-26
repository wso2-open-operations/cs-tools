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

// ProjectContactRow is the raw shape of one row read from project_contact
// (migration 000022), joined through account_contact to "user" (to resolve
// a linked platform identity) and through project_contact_group/
// project_group_role/project_role (migrations 000023-000025) to that
// contact's roles.
type ProjectContactRow struct {
	Email             string
	RegistrationState string
	// ResolvedUserID/ResolvedName/ResolvedEmail come from the "user" row
	// joined via account_contact.user_name, when one exists -- nil means
	// this row has no linked platform identity, the same "no contact
	// record" case domain.ProjectContact's own doc comment describes.
	ResolvedUserID *string
	ResolvedName   *string
	ResolvedEmail  *string
	// Roles is the union of project_role.role across every project_group
	// this contact belongs to (via project_contact_group). Never nil --
	// COALESCE'd to an empty array in the query.
	Roles []string
	// AccountRoles is the resolved user's account-level roles (user_role ->
	// role.name), restricted to the five names the membership write owns:
	// external, customer, partner and the derived customer_admin/
	// partner_admin. Never nil, and empty for a row with no linked "user".
	//
	// A separate list from Roles on purpose -- see
	// domain.ProjectContact.AccountRoles. The admin entry is the derived
	// account-level role: the membership write recomputes it from every
	// membership the user holds and materialises it in user_role, so reading
	// it back is one join rather than a second pass over the project groups.
	AccountRoles []string
}

// accountLevelRoleNames is the allow-list applied to a contact's user_role
// rows before they are returned. Restricting it here rather than returning
// every role the user holds keeps an internal role (admin, agent, internal)
// on a staff account from ever appearing in a customer-facing contact list.
const accountLevelRoleNames = `ARRAY['external','customer','partner','customer_admin','partner_admin']::text[]`

// ProjectContactRepository defines the read operations for the
// project_contact table and its associated group-role tables.
type ProjectContactRepository interface {
	// SearchProjectContacts returns a filtered, paginated slice of
	// projectID's contacts together with the total count of matching rows
	// before pagination. callerEmail is threaded down for a future
	// authorization decision -- see AccountContactRepository's own doc
	// comment for the same convention; not enforced yet.
	SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest, callerEmail string) ([]ProjectContactRow, int, error)
	// GetProjectContactByUserID returns the single contact on projectID
	// whose linked platform identity is userID -- domain.ProjectContact.ID
	// is that user's id, not project_contact's own row id (see that
	// field's own doc comment), so this queries by the resolved join,
	// not project_contact.id. Returns a NotFoundError if no contact on
	// this project resolves to that user id.
	GetProjectContactByUserID(ctx context.Context, projectID, userID, callerEmail string) (ProjectContactRow, error)
}

type projectContactRepo struct {
	db *pgxpool.Pool
}

// NewProjectContactRepository constructs a ProjectContactRepository backed by the given connection pool.
func NewProjectContactRepository(db *pgxpool.Pool) ProjectContactRepository {
	return &projectContactRepo{db: db}
}

const projectContactColumns = `
	pc.email, pc.state,
	u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email,
	COALESCE((
		SELECT array_agg(DISTINCT pr.role::text)
		FROM project_contact_group pcg
		JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
		JOIN project_role pr ON pr.id = pgr.project_role_id
		WHERE pcg.project_contact_id = pc.id
	), ARRAY[]::text[]),
	COALESCE((
		SELECT array_agg(r.name ORDER BY r.name)
		FROM user_role ur
		JOIN role r ON r.id = ur.role_id
		WHERE ur.user_id = u.id AND r.name = ANY(` + accountLevelRoleNames + `)
	), ARRAY[]::text[])`

const projectContactFromJoins = `
	FROM project_contact pc
	JOIN account_contact ac ON ac.id = pc.account_contact_id
	LEFT JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)`

func scanProjectContact(row interface{ Scan(...any) error }) (ProjectContactRow, error) {
	var c ProjectContactRow
	var state *string
	err := row.Scan(&c.Email, &state, &c.ResolvedUserID, &c.ResolvedName, &c.ResolvedEmail, &c.Roles, &c.AccountRoles)
	if err != nil {
		return ProjectContactRow{}, err
	}
	if state != nil {
		c.RegistrationState = *state
	}
	return c, nil
}

// SearchProjectContacts implements ProjectContactRepository.
func (r *projectContactRepo) SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest, _ string) ([]ProjectContactRow, int, error) {
	where := "WHERE pc.project_id = $1"
	args := []any{projectID}
	argIdx := 2

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (pc.email ILIKE $%d ESCAPE '\\' OR u.email ILIKE $%d ESCAPE '\\' OR u.name ILIKE $%d ESCAPE '\\')", argIdx, argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) " + projectContactFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY pc.created_on DESC, pc.id LIMIT $%d OFFSET $%d",
		projectContactColumns, projectContactFromJoins, where, argIdx, argIdx+1)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var rows []ProjectContactRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count project contacts: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		res, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query project contacts: %w", err)
		}
		defer res.Close()

		out := make([]ProjectContactRow, 0, req.Pagination.Limit)
		for res.Next() {
			c, err := scanProjectContact(res)
			if err != nil {
				return fmt.Errorf("scan project contact: %w", err)
			}
			out = append(out, c)
		}
		if err := res.Err(); err != nil {
			return fmt.Errorf("iterate project contacts: %w", err)
		}
		rows = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

// GetProjectContactByUserID implements ProjectContactRepository.
func (r *projectContactRepo) GetProjectContactByUserID(ctx context.Context, projectID, userID string, _ string) (ProjectContactRow, error) {
	query := "SELECT " + projectContactColumns + " " + projectContactFromJoins + " WHERE pc.project_id = $1 AND u.id = $2"
	c, err := scanProjectContact(r.db.QueryRow(ctx, query, projectID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectContactRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
	}
	if err != nil {
		return ProjectContactRow{}, fmt.Errorf("get project contact by user id: %w", err)
	}
	return c, nil
}
