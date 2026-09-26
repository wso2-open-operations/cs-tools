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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// AccessUser is one "user" row matching a caller's email, reduced to what an
// access decision needs.
type AccessUser struct {
	// UserType is the raw user_type_enum label ("INTERNAL", "EXTERNAL",
	// "SYSTEM", "NOT_AVAILABLE"), or "" when the column is NULL.
	UserType string
	Active   bool
}

// registeredContactState is the project_contact.state that grants a customer
// access to a project. The access scope and the profile's grantsCaseAccess both
// use it, so what the profile reports is what is enforced.
const registeredContactState = "REGISTERED"

// AccessRepository reads what decides which projects a caller may see.
type AccessRepository interface {
	// UsersByEmail returns every "user" row with this email (case-insensitive).
	// user.email is not unique -- staging has emails shared across rows -- so
	// the caller must decide how to combine them.
	UsersByEmail(ctx context.Context, email string) ([]AccessUser, error)
	// RegisteredProjectIDs returns the projects this email is a REGISTERED
	// contact of. INVITED/RE-INVITED (not yet accepted) and DEACTIVATED
	// contacts are deliberately excluded: they do not grant access.
	RegisteredProjectIDs(ctx context.Context, email string) ([]string, error)
}

type accessRepo struct {
	db *pgxpool.Pool
}

// NewAccessRepository constructs an AccessRepository backed by the given connection pool.
// db may be nil: a deployment with no DB_* configured gets no pool, and the
// deployment-licence route is registered without one (see server.NewRouter).
// Scoping a caller then fails closed rather than dereferencing the nil pool.
func NewAccessRepository(db *pgxpool.Pool) AccessRepository {
	return &accessRepo{db: db}
}

// errNoPool is what both reads return when this service runs without a
// database. Only a user-token caller reaches them — an internal client is
// resolved as unrestricted before any query — so this refuses exactly the
// callers whose scope genuinely cannot be determined.
func (r *accessRepo) errNoPool() error {
	return &apierror.ServiceUnavailableError{Msg: "the caller's access scope cannot be resolved: this deployment has no database configured"}
}

// UsersByEmail implements AccessRepository. user.is_active is nullable; a NULL
// counts as active (only an explicit FALSE deactivates), matching how the
// rest of this schema treats an unset flag.
func (r *accessRepo) UsersByEmail(ctx context.Context, email string) ([]AccessUser, error) {
	if r.db == nil {
		return nil, r.errNoPool()
	}
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(user_type::TEXT, ''), (is_active IS DISTINCT FROM FALSE)
		 FROM "user" WHERE LOWER(email) = LOWER($1)`, email)
	if err != nil {
		return nil, fmt.Errorf("access: users by email: %w", err)
	}
	defer rows.Close()

	var out []AccessUser
	for rows.Next() {
		var u AccessUser
		if err := rows.Scan(&u.UserType, &u.Active); err != nil {
			return nil, fmt.Errorf("access: scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// RegisteredProjectIDs implements AccessRepository.
func (r *accessRepo) RegisteredProjectIDs(ctx context.Context, email string) ([]string, error) {
	if r.db == nil {
		return nil, r.errNoPool()
	}
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT project_id::TEXT FROM project_contact
		 WHERE LOWER(email) = LOWER($1) AND state::TEXT = $2
		 ORDER BY 1`, email, registeredContactState)
	if err != nil {
		return nil, fmt.Errorf("access: registered projects: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("access: scan project id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
