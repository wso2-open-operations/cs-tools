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
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	// GetUserDetail returns the user with the given id (name, active flag and
	// type; no roles/groups/access), or a NotFoundError.
	GetUserDetail(ctx context.Context, id string) (domain.UserDetail, error)
	// GetUserProjectAccess returns every project_contact row invited under email,
	// with its project, linked contact record and project roles.
	GetUserProjectAccess(ctx context.Context, email string) ([]domain.UserContactAccess, error)
	// GetUserGroups returns every team userID belongs to via team_member
	// (migration 000028), empty if none.
	GetUserGroups(ctx context.Context, userID string) ([]domain.UserGroupRef, error)
	// CreateUser inserts a new "user" row (user_name = lower(email), matching
	// the Salesforce membership ingest's own convention) and, if req.Roles is
	// non-empty, grants each role via user_role in the same transaction.
	// actor is the acting caller's email, stamped as created_by/updated_by on
	// every row this writes. Returns a ConflictError if email is already in
	// use (user_name is UNIQUE), a ServiceUnavailableError naming any
	// requested role not seeded in the role table.
	CreateUser(ctx context.Context, req domain.CreateUserRequest, actor string) (domain.User, error)
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
// userSortColumns maps a validated domain.UserSortField to the SQL expression
// it orders by. name falls back from the display name to first + last name and
// then the user name, because "user".name is NULL for some synced rows, and is
// compared case-insensitively so "alice" does not sort after "Zed". The values
// are fixed strings, never derived from the request.
var userSortColumns = map[domain.UserSortField]string{
	domain.UserSortFieldName: `LOWER(COALESCE(NULLIF(u.name, ''),
		NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.user_name, ''))`,
	domain.UserSortFieldCreatedOn: "u.created_on",
	domain.UserSortFieldUpdatedOn: "u.updated_on",
}

// userOrderBy returns the ORDER BY body for a user search. With no sortBy the
// list is newest-first. A requested sort defaults to ascending, and u.id is
// always the final tie-break so pages are stable across offsets.
func userOrderBy(s domain.UserSortBy) string {
	col, ok := userSortColumns[s.Field]
	if !ok {
		return "u.created_on DESC, u.id"
	}
	dir := "ASC"
	if s.Order == domain.UserSortOrderDesc {
		dir = "DESC"
	}
	return col + " " + dir + ", u.id"
}

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
		//
		// The synced role.name value carries a namespace prefix for at
		// least some roles (e.g. "sn_customerservice.timecard_approver" --
		// see the webapp's own ROLE_CATALOGUE_ALIASES, which exists purely
		// to strip this same prefix back off for display), while a caller
		// filtering by roleIds sends the bare, unnamespaced name (matching
		// CSM_USER_ROLES' own vocabulary). Matching on the suffix after the
		// last "." as well as the exact value handles either shape without
		// hardcoding a specific namespace string, and never matches less
		// than a plain r.name = ANY(...) would have on its own.
		roleNames := make([]string, len(req.Filters.RoleIDs))
		for i, role := range req.Filters.RoleIDs {
			roleNames[i] = string(role)
		}
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
			WHERE ur.user_id = u.id
			  AND (r.name = ANY($%d::text[]) OR regexp_replace(r.name, '^.*\.', '') = ANY($%d::text[]))
		)`, argIdx, argIdx)
		filterArgs = append(filterArgs, roleNames)
		argIdx++
	}

	if len(req.Filters.UserIDs) > 0 {
		where += fmt.Sprintf(" AND u.id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Filters.UserIDs)
		argIdx++
	}

	if len(req.Filters.GroupIDs) > 0 {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM team_member tm WHERE tm.user_id = u.id AND tm.team_id = ANY($%d::uuid[])
		)`, argIdx)
		filterArgs = append(filterArgs, req.Filters.GroupIDs)
		argIdx++
	}

	if len(req.Filters.GroupNames) > 0 {
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM team_member tm JOIN team t ON t.id = tm.team_id
			WHERE tm.user_id = u.id AND t.name = ANY($%d::text[])
		)`, argIdx)
		filterArgs = append(filterArgs, req.Filters.GroupNames)
		argIdx++
	}

	if req.Filters.Active != nil {
		// "user".is_active is nullable; a NULL row counts as active, the same
		// convention AccessService.ResolveScope already uses for this exact
		// column ("user.is_active NULL counts as active" -- access_repo.go).
		// active=false is strict, though: a row with no is_active recorded at
		// all is not known to be inactive, so it must not match that filter.
		if *req.Filters.Active {
			where += " AND (u.is_active IS NULL OR u.is_active = TRUE)"
		} else {
			where += " AND u.is_active = FALSE"
		}
	}

	const fromClause = `FROM "user" u`

	countQuery := "SELECT COUNT(*) " + fromClause + " " + where

	dataQuery := fmt.Sprintf(
		`SELECT %s %s %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		prefixUserColumns, fromClause, where, userOrderBy(req.SortBy), argIdx, argIdx+1,
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

	if err := r.attachRoles(ctx, users); err != nil {
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
		`SELECT id, user_name, first_name, last_name, email, phone, timezone, user_type, created_on, updated_on
		 FROM "user" WHERE id = ANY($1)`,
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

// attachRoles fills in each user's Roles from user_role in ONE query for the
// whole page (not one per user), so the search stays a fixed number of round
// trips whatever the page size. DISTINCT because user_role has no unique
// constraint on (user_id, role_id) and the sync left duplicates (113 user/role
// pairs in staging), which would otherwise list a role twice.
func (r *userRepo) attachRoles(ctx context.Context, users []domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ur.user_id::text, r.name
		FROM user_role ur
		JOIN role r ON r.id = ur.role_id
		WHERE ur.user_id = ANY($1::uuid[])
		ORDER BY ur.user_id::text, r.name`, ids)
	if err != nil {
		return fmt.Errorf("query roles for users: %w", err)
	}
	defer rows.Close()

	byUser := make(map[string][]string, len(users))
	for rows.Next() {
		var userID, role string
		if err := rows.Scan(&userID, &role); err != nil {
			return fmt.Errorf("scan user role: %w", err)
		}
		byUser[userID] = append(byUser[userID], role)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate roles for users: %w", err)
	}
	assignRoles(users, byUser)
	return nil
}

// assignRoles sets Roles on every user; a user with none gets an empty, non-nil
// slice so it serializes as [] rather than null.
func assignRoles(users []domain.User, byUser map[string][]string) {
	for i := range users {
		if roles, ok := byUser[users[i].ID]; ok {
			users[i].Roles = roles
			continue
		}
		users[i].Roles = []string{}
	}
}


// GetUserRoles implements UserRepository.
func (r *userRepo) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	// DISTINCT: user_role has no unique (user_id, role_id), and duplicates exist.
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT r.name FROM user_role ur
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

// GetUserDetail implements UserRepository.
func (r *userRepo) GetUserDetail(ctx context.Context, id string) (domain.UserDetail, error) {
	var (
		d                      domain.UserDetail
		email, userType        *string
		isActive               *bool
		name, firstName, lName *string
	)
	err := r.db.QueryRow(ctx, `
		SELECT id::TEXT, user_name, name, first_name, last_name, email, is_active, user_type::TEXT, created_on, updated_on
		FROM "user" WHERE id = $1::uuid`, id).Scan(
		&d.ID, &d.UserName, &name, &firstName, &lName, &email, &isActive, &userType, &d.CreatedOn, &d.UpdatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UserDetail{}, &apierror.NotFoundError{Msg: "no user found with id: " + id}
	}
	if err != nil {
		return domain.UserDetail{}, fmt.Errorf("get user detail: %w", err)
	}
	d.Email = stringOrEmpty(email)
	d.Name = displayName(name, firstName, lName, d.UserName)
	d.Active = isActive == nil || *isActive
	if userType != nil {
		d.UserType = userTypeFromEnum[*userType]
	}
	return d, nil
}

// displayName is the name shown for a user: the display name, else first + last,
// else the user name ("user".name is empty for a few synced rows).
func displayName(name, first, last *string, userName string) string {
	if n := strings.TrimSpace(stringOrEmpty(name)); n != "" {
		return n
	}
	if n := strings.TrimSpace(stringOrEmpty(first) + " " + stringOrEmpty(last)); n != "" {
		return n
	}
	return userName
}

// GetUserProjectAccess implements UserRepository.
func (r *userRepo) GetUserProjectAccess(ctx context.Context, email string) ([]domain.UserContactAccess, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id::TEXT, COALESCE(p.name, ''), p.key, pc.email,
		       pc.account_contact_id IS NOT NULL, COALESCE(ac.user_name, ''), pc.state::TEXT,
		       COALESCE((
		           SELECT array_agg(DISTINCT pr.role::TEXT ORDER BY pr.role::TEXT)
		           FROM project_contact_group pcg
		           JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
		           JOIN project_role pr ON pr.id = pgr.project_role_id
		           WHERE pcg.project_contact_id = pc.id
		       ), '{}'::TEXT[])
		FROM project_contact pc
		JOIN project p ON p.id = pc.project_id
		LEFT JOIN account_contact ac ON ac.id = pc.account_contact_id
		WHERE LOWER(pc.email) = LOWER($1)
		ORDER BY p.name, p.key, pc.id`, email)
	if err != nil {
		return nil, fmt.Errorf("query user project access: %w", err)
	}
	defer rows.Close()

	access := []domain.UserContactAccess{}
	for rows.Next() {
		var a domain.UserContactAccess
		if err := rows.Scan(&a.ProjectID, &a.ProjectName, &a.ProjectKey, &a.ContactEmail,
			&a.ContactRecordPresent, &a.ContactRecordEmail, &a.RegistrationState, &a.Roles); err != nil {
			return nil, fmt.Errorf("scan user project access: %w", err)
		}
		a.GrantsCaseAccess = a.RegistrationState == registeredContactState
		access = append(access, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user project access: %w", err)
	}
	return access, nil
}

// CreateUser implements UserRepository.
func (r *userRepo) CreateUser(ctx context.Context, req domain.CreateUserRequest, actor string) (domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("create user: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	email := strings.ToLower(strings.TrimSpace(req.Email))
	var u domain.User
	var firstName, lastName, userType *string
	err = tx.QueryRow(ctx, `
		INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
			user_name, first_name, last_name, email, is_active)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3, $4, $2, TRUE)
		RETURNING id, user_name, first_name, last_name, email, user_type::TEXT, created_on, updated_on`,
		actor, email, nullIfBlank(req.FirstName), nullIfBlank(req.LastName),
	).Scan(&u.ID, &u.UserName, &firstName, &lastName, &u.Email, &userType, &u.CreatedOn, &u.UpdatedOn)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, &apierror.ConflictError{Msg: "a user with this email already exists: " + email}
		}
		return domain.User{}, fmt.Errorf("create user: insert user: %w", err)
	}
	u.FirstName = stringOrEmpty(firstName)
	u.LastName = stringOrEmpty(lastName)
	if userType != nil {
		u.UserType = userTypeFromEnum[*userType]
	}

	u.Roles, err = grantRoles(ctx, tx, u.ID, req.Roles, actor)
	if err != nil {
		return domain.User{}, err
	}

	// user_type is trigger-derived from role membership (migration 000007),
	// so the value RETURNING read above -- before any role was granted -- can
	// already be stale once grantRoles has run. Only worth a second read when
	// a role was actually granted; with none, nothing could have changed it.
	if len(u.Roles) > 0 {
		userType = nil
		if err := tx.QueryRow(ctx, `SELECT user_type::TEXT FROM "user" WHERE id = $1`, u.ID).Scan(&userType); err != nil {
			return domain.User{}, fmt.Errorf("create user: reload user type: %w", err)
		}
		if userType != nil {
			u.UserType = userTypeFromEnum[*userType]
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("create user: commit: %w", err)
	}
	return u, nil
}

// grantRoles resolves each of names (role.name, migration 000004) to its id
// and inserts a user_role row for it, all inside tx. Every name must exist in
// role before anything is inserted -- a partially-granted set on an unseeded
// role name would be a confusing half-success. Returns the granted names,
// sorted, for the response (never nil, so it serializes as [] when names is
// empty).
func grantRoles(ctx context.Context, tx pgx.Tx, userID string, names []domain.UserRole, actor string) ([]string, error) {
	if len(names) == 0 {
		return []string{}, nil
	}
	wanted := make([]string, len(names))
	for i, n := range names {
		wanted[i] = string(n)
	}

	roleIDs := map[string]string{}
	rows, err := tx.Query(ctx, `SELECT id, name FROM role WHERE name = ANY($1::text[])`, wanted)
	if err != nil {
		return nil, fmt.Errorf("create user: resolve roles: %w", err)
	}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("create user: scan role: %w", err)
		}
		roleIDs[name] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("create user: iterate roles: %w", err)
	}
	for _, name := range wanted {
		if _, ok := roleIDs[name]; !ok {
			return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("role %q is not seeded in the role table", name)}
		}
	}

	granted := make([]string, 0, len(wanted))
	seen := map[string]bool{}
	for _, name := range wanted {
		if seen[name] {
			continue
		}
		seen[name] = true
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3)`, actor, userID, roleIDs[name]); err != nil {
			return nil, fmt.Errorf("create user: grant role %s: %w", name, err)
		}
		granted = append(granted, name)
	}
	sort.Strings(granted)
	return granted, nil
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
