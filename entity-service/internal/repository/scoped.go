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
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// IsRLSPolicyViolation reports whether err is Postgres rejecting a write
// because it violates a row-level-security policy's WITH CHECK clause
// (SQLSTATE 42501) -- the error a customer's INSERT/UPDATE gets back when it
// targets a project they are not a member of (e.g. creating or updating a
// call request against another project's case). Repositories map this the
// same way they already map "genuinely doesn't exist" (pgx.ErrNoRows) to
// apierror.NotFoundError: the caller should not be able to distinguish "that
// case doesn't exist" from "that case isn't yours" by the shape of the
// error they get back, any more than they could by response timing.
func IsRLSPolicyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}

// callerIdentityContextKey is the context key a resolved caller identity is
// stored under, set once per request by the server's identity middleware
// (after auth.Middleware resolves who is calling) and read by every Scoped
// method below. Kept unexported and typed (not a plain string) so nothing
// outside this package can collide with it.
type callerIdentityContextKey struct{}

// WithCallerIdentity attaches the caller's resolved identity to ctx. Called
// exactly once per request, by the server's identity-resolution middleware --
// every downstream Scoped call reads it back via CallerIdentityFromContext.
// Only Unrestricted and ViewerEmail are meaningful here; a SearchScope's
// ProjectIDs (still computed by AccessService for callers not yet migrated to
// RLS) is deliberately not part of what Scoped forwards to Postgres --
// project-membership is the database's job now, not something Go resolves
// and hands down.
func WithCallerIdentity(ctx context.Context, identity SearchScope) context.Context {
	return context.WithValue(ctx, callerIdentityContextKey{}, identity)
}

// CallerIdentityFromContext returns the identity WithCallerIdentity attached,
// or ok=false if none was ever set (a request that never passed through the
// identity middleware, or a background job that forgot to stamp one -- see
// Scoped's own doc comment for why that must fail loudly rather than
// silently falling back to unrestricted).
func CallerIdentityFromContext(ctx context.Context) (SearchScope, bool) {
	v, ok := ctx.Value(callerIdentityContextKey{}).(SearchScope)
	return v, ok
}

// ErrNoCallerIdentity is returned by every Scoped method when ctx carries no
// caller identity. This must never be treated as "assume unrestricted" --
// see WithSystemIdentity for the explicit, deliberate way a background job
// declares itself internal instead.
var ErrNoCallerIdentity = errors.New("scoped query: no caller identity on context (missing identity middleware, or a background job that forgot to stamp one)")

// WithSystemIdentity marks ctx as an internal, unrestricted caller for code
// that has no request to inherit identity from: process-startup workers (the
// GitHub outbound worker, the CR-notice drainer, the SN-writeback dispatcher
// pool), health checks, and the startup ping. Deliberately a separate,
// explicit call rather than a fallback Scoped applies on its own when
// identity is missing -- a silent "no identity means internal" default would
// be the exact failure mode this mechanism exists to prevent: a context that
// SHOULD have carried a real caller's identity (e.g. a customer-triggered
// background job built from context.WithoutCancel(requestCtx), which
// preserves context values) must never quietly upgrade to unrestricted just
// because whoever wrote that call site didn't think about it.
func WithSystemIdentity(ctx context.Context) context.Context {
	return WithCallerIdentity(ctx, SearchScope{Unrestricted: true})
}

// Scoped wraps a db.Pool so every statement it runs against a
// caller-scoped, row-level-security-protected table carries the caller's
// identity as the same two session-local GUCs setCallerIdentity already
// established (app.is_internal / app.viewer_email) -- in the SAME
// transaction as the guarded statement, which is the one thing empirically
// proven necessary (see setCallerIdentity's own doc comment).
//
// This is the ONLY way protected repositories reach Postgres: they hold a
// *Scoped, never a raw db.Pool, so there is no call a repository
// method can make that skips identity-setting by omission -- the earlier,
// per-call-site runWithCallerIdentity wrapper this replaces was proven not
// to hold that property (6 of 7 call sites missed it in review).
//
// Concurrency: each method opens its own short-lived transaction (a single
// pgx.Batch sent as one round trip, which pgx runs as an implicit
// transaction) rather than sharing one transaction for a whole request.
// This is deliberate, not a shortcut -- the codebase runs COUNT and page
// queries concurrently via errgroup throughout (case_repo.go alone has 13
// such sites), and a pgx.Tx is one physical connection, unsafe to share
// across goroutines. One transaction per statement preserves that
// concurrency untouched; a single ambient per-request transaction would not.
//
// pool is deliberately unexported: nothing outside this package can obtain
// it directly, so the only way any code in this repository package reaches
// Postgres for a protected table without going through the methods below is
// by holding a SEPARATE reference to the same db.Pool passed in
// alongside Scoped (exactly the shape unprotected-table repos legitimately
// use, and exactly what TestRLSBypassLint_NoRawPoolAgainstAProtectedTable
// scans every non-test file in this package for). An earlier version of
// this design considered a pgxpool.Config.PrepareConn hook as a second,
// runtime-enforced backstop on top of that lint test -- rejecting any
// connection acquire whose context lacks a marker Scoped sets. That was
// deliberately NOT built: this package's protected and unprotected repos
// share ONE pool (NewScoped(db) and e.g. NewProjectRepository(db) both
// close over the same pool from cmd/api/main.go), so a PrepareConn
// hook on it would reject every unprotected-table query too, not just a
// bypass. Doing this safely would need a SECOND, dedicated pool for Scoped
// alone -- and since most real customer traffic (cases, escalations, time
// cards) runs through exactly that pool, getting its connection-limit
// sizing wrong risks real customer-facing connection exhaustion under load,
// a concrete regression traded for a narrow, already-covered residual risk.
// Revisit only alongside a deliberate connection-budget re-tuning exercise,
// not as a quick addition.
type Scoped struct {
	pool db.Pool
}

// NewScoped constructs a Scoped wrapping pool. Protected repository
// constructors take a *Scoped instead of a raw db.Pool.
func NewScoped(pool db.Pool) *Scoped {
	return &Scoped{pool: pool}
}

// identityArgs renders scope as the two set_config argument strings.
func identityArgs(scope SearchScope) (isInternal, viewerEmail string) {
	return strconv.FormatBool(scope.Unrestricted), scope.ViewerEmail
}

// queueIdentity queues the identity-setting statements onto batch, ahead of
// whatever real statement the caller queues next -- never inlined into that
// statement's own WHERE clause (see setCallerIdentity's own doc comment for
// why: the planner can evaluate a non-leakproof function call like
// set_config out of order under an index scan).
//
// The third statement resolves and caches app.viewer_project_ids -- the
// caller's own REGISTERED project ids, computed here in SQL from
// viewerEmail alone (never computed or forwarded by Go: ResolveScope still
// only ever produces Unrestricted/ViewerEmail, unchanged). is_project_member
// (migration 0152) reads this GUC instead of running its own EXISTS query
// against project_contact per row: a literal array the planner can match
// against idx_work_item_project_id, rather than an opaque function call it
// can only evaluate as an unindexed per-row filter. Confirmed empirically
// (a 20k-row synthetic case load, real EXPLAIN ANALYZE): the per-row
// function-call form forces a sequential scan of the entire work_item
// table regardless of how selective the caller's own projects are, roughly
// 2x-4x slower than this cached-array form for a single caller's paginated
// case search. Unrestricted callers get a plain '{}' with no query at all,
// since is_internal already short-circuits every policy before the array
// is ever consulted.
func queueIdentity(batch *pgx.Batch, scope SearchScope) {
	isInternal, viewerEmail := identityArgs(scope)
	batch.Queue("SELECT set_config('app.is_internal', $1, true)", isInternal)
	batch.Queue("SELECT set_config('app.viewer_email', $1, true)", viewerEmail)
	if scope.Unrestricted {
		batch.Queue("SELECT set_config('app.viewer_project_ids', '{}', true)")
	} else {
		batch.Queue(setViewerProjectIDsSQL, viewerEmail)
	}
}

// drainIdentity consumes the three queued identity-setting results ahead of
// the caller's own statement result. On error it closes br itself (the
// caller never got a result to be responsible for closing).
func drainIdentity(br pgx.BatchResults) error {
	if _, err := br.Exec(); err != nil {
		_ = br.Close()
		return fmt.Errorf("scoped: set is_internal: %w", err)
	}
	if _, err := br.Exec(); err != nil {
		_ = br.Close()
		return fmt.Errorf("scoped: set viewer_email: %w", err)
	}
	if _, err := br.Exec(); err != nil {
		_ = br.Close()
		return fmt.Errorf("scoped: set viewer_project_ids: %w", err)
	}
	return nil
}

// scopedRows wraps pgx.Rows so that closing it also releases the batch's
// underlying pooled connection (pgxpool.Pool.SendBatch ties the acquired
// connection to the BatchResults it returns, not to the Rows/Row obtained
// from it -- so whoever consumes those must close both, exactly once).
type scopedRows struct {
	pgx.Rows
	br     pgx.BatchResults
	closed bool
}

func (r *scopedRows) Close() {
	r.Rows.Close()
	if !r.closed {
		r.closed = true
		_ = r.br.Close()
	}
}

// scopedRow is QueryRow's equivalent of scopedRows: Scan is always the last
// thing called on a pgx.Row, so it's the one place to release the
// connection, on every path (success, no-rows, or scan error).
type scopedRow struct {
	row pgx.Row
	br  pgx.BatchResults
}

func (r *scopedRow) Scan(dest ...any) error {
	defer func() { _ = r.br.Close() }()
	return r.row.Scan(dest...)
}

// errRow is returned when identity-setting itself fails, before the
// caller's own statement ever ran -- Scan reports that failure the same way
// a real query's Scan would, rather than panicking or returning a bare nil
// pgx.Row.
type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

// Query implements a caller-scoped equivalent of (*pgxpool.Pool).Query.
func (s *Scoped) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	scope, ok := CallerIdentityFromContext(ctx)
	if !ok {
		return nil, ErrNoCallerIdentity
	}
	batch := &pgx.Batch{}
	queueIdentity(batch, scope)
	batch.Queue(sql, args...)

	br := s.pool.SendBatch(ctx, batch)
	if err := drainIdentity(br); err != nil {
		return nil, err
	}
	rows, err := br.Query()
	if err != nil {
		_ = br.Close()
		return nil, fmt.Errorf("scoped query: %w", err)
	}
	return &scopedRows{Rows: rows, br: br}, nil
}

// QueryRow implements a caller-scoped equivalent of (*pgxpool.Pool).QueryRow.
func (s *Scoped) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	scope, ok := CallerIdentityFromContext(ctx)
	if !ok {
		return errRow{ErrNoCallerIdentity}
	}
	batch := &pgx.Batch{}
	queueIdentity(batch, scope)
	batch.Queue(sql, args...)

	br := s.pool.SendBatch(ctx, batch)
	if err := drainIdentity(br); err != nil {
		return errRow{err}
	}
	return &scopedRow{row: br.QueryRow(), br: br}
}

// Exec implements a caller-scoped equivalent of (*pgxpool.Pool).Exec.
func (s *Scoped) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	scope, ok := CallerIdentityFromContext(ctx)
	if !ok {
		return pgconn.CommandTag{}, ErrNoCallerIdentity
	}
	batch := &pgx.Batch{}
	queueIdentity(batch, scope)
	batch.Queue(sql, args...)

	br := s.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	if err := drainIdentity(br); err != nil {
		return pgconn.CommandTag{}, err
	}
	ct, err := br.Exec()
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("scoped exec: %w", err)
	}
	return ct, nil
}

// InTx runs fn inside one transaction with the caller's identity set once at
// the start -- for the existing multi-statement transactional call sites
// (a FOR UPDATE lock followed by a write, two tables updated atomically,
// etc.) that need several statements to share one transaction rather than
// each getting its own. A drop-in replacement for today's r.db.Begin(ctx)
// at those call sites: same fn func(pgx.Tx) error shape, same
// commit-on-success/rollback-on-error contract, sourced from Scoped instead
// of the raw pool.
func (s *Scoped) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	scope, ok := CallerIdentityFromContext(ctx)
	if !ok {
		return ErrNoCallerIdentity
	}
	return runWithCallerIdentity(ctx, s.pool, scope, fn)
}

// InTxReturning collapses the boilerplate every InTx call site whose closure
// body is really just one delegate call to a Tx-suffixed helper function
// repeated at (declare a result variable above the closure, assign it via a
// second, inner error variable inside the closure, return that inner error,
// then check the outer error and return the zero value on failure). fn's own
// signature -- func(tx pgx.Tx) (T, error) -- is exactly what every one of
// those Tx-suffixed helpers (createCaseTx, createEscalationTx, and similar)
// already returns, so converting a call site to this is typically a direct
// substitution: wrap the helper call in a closure over tx (ctx and any other
// arguments are captured, not passed through InTxReturning itself) and
// return its own two results straight through.
func InTxReturning[T any](ctx context.Context, db *Scoped, fn func(tx pgx.Tx) (T, error)) (T, error) {
	var result T
	err := db.InTx(ctx, func(tx pgx.Tx) error {
		var txErr error
		result, txErr = fn(tx)
		return txErr
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}
