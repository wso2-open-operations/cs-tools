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

package db

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// RLSProtection describes whether row-level security actually binds the role
// this service connects as.
//
// RLS only protects a connection that it applies to. Postgres exempts a
// superuser and any role with BYPASSRLS always, and exempts a table's owner
// unless the table also has FORCE ROW LEVEL SECURITY. If the service connects
// as an exempt role, every policy is silently ignored and customers are not
// isolated by the database at all, while every query still "works". Nothing
// else in the service would notice, which is what this check is for.
type RLSProtection struct {
	// Role is the role this connection runs as.
	Role string
	// Tables is how many tables in the current schema have row-level security
	// enabled. Zero means RLS is not in use here (for example the migrations
	// have not been applied), so there is nothing to be exempt from.
	Tables int
	// RoleBypassesAll is true for a superuser or a role with BYPASSRLS.
	RoleBypassesAll bool
	// OwnerExemptTables counts RLS-enabled tables that this role owns (directly
	// or by inheriting the owner role) and that do not have FORCE ROW LEVEL
	// SECURITY, so the policies do not apply to it.
	OwnerExemptTables int
}

// InUse reports whether any table in the schema has row-level security enabled.
func (p RLSProtection) InUse() bool { return p.Tables > 0 }

// Exempt reports whether RLS is in use but does not bind this role.
func (p RLSProtection) Exempt() bool {
	return p.InUse() && (p.RoleBypassesAll || p.OwnerExemptTables > 0)
}

// Summary is a one-line, secret-free description for logs.
func (p RLSProtection) Summary() string {
	switch {
	case !p.InUse():
		return fmt.Sprintf("role %q; row-level security is not enabled on any table in this schema", p.Role)
	case p.RoleBypassesAll:
		return fmt.Sprintf("role %q is a superuser or has BYPASSRLS; row-level security is enabled on %d tables but never applies to it", p.Role, p.Tables)
	case p.OwnerExemptTables > 0:
		return fmt.Sprintf("role %q owns %d of %d RLS-enabled tables that do not have FORCE ROW LEVEL SECURITY, so their policies do not apply to it", p.Role, p.OwnerExemptTables, p.Tables)
	default:
		return fmt.Sprintf("role %q is bound by row-level security on all %d RLS-enabled tables", p.Role, p.Tables)
	}
}

// rlsQuerier is the one method CheckRLSProtection needs; *pgxpool.Pool and
// pgx.Tx both satisfy it.
type rlsQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// rlsProtectionQuery reads the connecting role's flags and, for every table in
// the current schema with RLS enabled, whether that role would be exempt as an
// owner. pg_has_role(..., 'USAGE') is the same inheritance rule Postgres uses
// for the owner exemption, so a role that is a member of the owner role counts
// as the owner here too.
const rlsProtectionQuery = `
SELECT current_user::text,
       COALESCE(bool_or(r.rolsuper OR r.rolbypassrls), false),
       count(c.oid)::int,
       count(c.oid) FILTER (WHERE NOT c.relforcerowsecurity AND pg_has_role(current_user, c.relowner, 'USAGE'))::int
FROM pg_roles r
LEFT JOIN pg_class c
       ON c.relrowsecurity
      AND c.relkind IN ('r', 'p')
      AND c.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = current_schema())
WHERE r.rolname = current_user`

// CheckRLSProtection reports whether row-level security binds the role q is
// connected as.
func CheckRLSProtection(ctx context.Context, q rlsQuerier) (RLSProtection, error) {
	var p RLSProtection
	if err := q.QueryRow(ctx, rlsProtectionQuery).Scan(&p.Role, &p.RoleBypassesAll, &p.Tables, &p.OwnerExemptTables); err != nil {
		return RLSProtection{}, fmt.Errorf("check RLS protection: %w", err)
	}
	return p, nil
}

// VerifyRLSProtection runs CheckRLSProtection at startup and reports the result
// in the log. If row-level security is in use but this role is exempt from it,
// that is logged as an error, and when required is true it is returned as an
// error so the caller can refuse to start. The check itself failing is only
// logged: a monitoring query must never be what takes the service down.
func VerifyRLSProtection(ctx context.Context, q rlsQuerier, required bool) error {
	p, err := CheckRLSProtection(ctx, q)
	if err != nil {
		log.Printf("WARN: RLS protection check could not run, continuing: %v", err)
		return nil
	}
	if p.Exempt() {
		msg := "row-level security is ENABLED but does not apply to this service's database role, so customers are NOT isolated by the database: " + p.Summary() +
			". Connect as a role that does not own the tables and has no BYPASSRLS, or keep FORCE ROW LEVEL SECURITY on (see docs/rls-database-roles.md)"
		if required {
			return errors.New(msg)
		}
		log.Printf("ERROR: %s", msg)
		return nil
	}
	log.Printf("RLS protection: %s", p.Summary())
	return nil
}
