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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setCallerIdentity sets the two session-local GUCs a caller-scoped
// row-level-security policy reads to decide what's visible: app.is_internal
// ('true' bypasses the policy unconditionally -- the same all-access meaning
// Unrestricted already carries everywhere else) and app.viewer_email (the
// resolved caller's own email; ignored by any policy once is_internal is
// true, so it is never required to be non-empty here).
//
// Nothing here is specific to any one table. `announcement`'s policy
// (migration 000085) is the first consumer, but any later RLS policy on any
// other table can read these same two GUCs without new plumbing -- that is
// the point of naming and scoping this generically instead of coupling it to
// "announcement".
//
// tx must be the SAME transaction the caller runs its RLS-protected query
// in, and this must be called before that query. Postgres's
// set_config(..., true) ("LOCAL" scoping) only takes effect for the
// remainder of the CURRENT transaction and reverts automatically at
// COMMIT/ROLLBACK -- called as a bare statement outside a transaction, or in
// a different transaction than the query that depends on it, it silently
// has no effect on that query (confirmed empirically while building the
// first RLS policy that uses this). Calling it as its own statement ahead of
// the query -- never inlined into the query's own WHERE clause -- is also
// required: the query planner is free to evaluate an RLS qual before a
// non-leakproof function call like set_config(), and was observed doing
// exactly that under an index scan in testing, silently using whatever value
// the GUC held before this call ran.
func setCallerIdentity(ctx context.Context, tx pgx.Tx, scope SearchScope) error {
	if _, err := tx.Exec(ctx, "SELECT set_config('app.is_internal', $1, true)", fmt.Sprintf("%t", scope.Unrestricted)); err != nil {
		return fmt.Errorf("set caller identity: is_internal: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.viewer_email', $1, true)", scope.ViewerEmail); err != nil {
		return fmt.Errorf("set caller identity: viewer_email: %w", err)
	}
	return nil
}

// runWithCallerIdentity opens one transaction on pool, sets the caller
// identity via setCallerIdentity, runs fn inside that same transaction, and
// commits. Use this instead of a bare pool.Query/QueryRow for any query that
// reads a table guarded by a caller-scoped row-level-security policy --
// today just `announcement`, but this helper carries no table-specific
// assumption, so a future RLS policy on another table reuses it as-is
// instead of every call site hand-rolling its own Begin/set/Commit sequence.
//
// fn must not call tx.Commit/tx.Rollback itself -- this function owns the
// transaction's lifecycle so the identity-setting and commit steps can never
// drift apart across call sites.
func runWithCallerIdentity(ctx context.Context, pool *pgxpool.Pool, scope SearchScope, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("run with caller identity: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setCallerIdentity(ctx, tx, scope); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("run with caller identity: commit tx: %w", err)
	}
	return nil
}
