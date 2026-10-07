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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type readOnlyKey struct{}

// WithReadOnly marks ctx so a Router sends its database calls to the read
// pool. The default is NOT read-only: an unmarked context (background workers,
// every route that has not opted in) uses the write pool.
func WithReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, true)
}

// IsReadOnly reports whether ctx was marked by WithReadOnly.
func IsReadOnly(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyKey{}).(bool)
	return v
}

// Router is a Pool that picks a write or read pool per call from the context.
// It cannot infer read from write by method: repository.Scoped sends every
// statement through SendBatch and many writes are INSERT ... RETURNING via
// Query, so only the context marker decides.
//
// With no read pool it behaves exactly like the write pool. There is no
// fallback from the read pool to the write pool on error: a failure on the
// read pool surfaces, so a bad route or a down replica is visible.
type Router struct {
	write Pool
	read  Pool
	desc  string
}

var _ Pool = (*Router)(nil)

// NewRouter builds a Router over write and an optional read (nil for none).
// A nil write yields a nil *Router, keeping "no database" representable; use
// Router.Pool to convert to a Pool.
func NewRouter(write, read Pool) *Router {
	if write == nil {
		return nil
	}
	desc := "db pools: write only"
	if read != nil {
		desc = "db pools: write and read"
	}
	return &Router{write: write, read: read, desc: desc}
}

// Pool returns r as a Pool, or a true nil Pool when r is nil (no database).
func (r *Router) Pool() Pool {
	if r == nil {
		return nil
	}
	return r
}

// Describe is a one-line, secret-free summary of the pools for the startup log.
func (r *Router) Describe() string {
	if r == nil {
		return "db pools: none"
	}
	return r.desc
}

// ReadEnabled reports whether a read pool is configured.
func (r *Router) ReadEnabled() bool { return r != nil && r.read != nil }

func (r *Router) pick(ctx context.Context) Pool {
	if r.read != nil && IsReadOnly(ctx) {
		return r.read
	}
	return r.write
}

func (r *Router) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.pick(ctx).Query(ctx, sql, args...)
}

func (r *Router) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return r.pick(ctx).QueryRow(ctx, sql, args...)
}

func (r *Router) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return r.pick(ctx).Exec(ctx, sql, args...)
}

func (r *Router) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return r.pick(ctx).SendBatch(ctx, b)
}

func (r *Router) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pick(ctx).Begin(ctx)
}

func (r *Router) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return r.pick(ctx).BeginTx(ctx, txOptions)
}

func (r *Router) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return r.pick(ctx).Acquire(ctx)
}

// Ping checks every pool, whatever the context: health must fail if either is
// down. The error carries no detail beyond what pgx returns, and callers that
// expose it must keep it out of response bodies.
func (r *Router) Ping(ctx context.Context) error {
	err := r.write.Ping(ctx)
	if r.read != nil {
		err = errors.Join(err, r.read.Ping(ctx))
	}
	return err
}

// Close closes both pools. Pools that have no Close (test doubles) are skipped.
func (r *Router) Close() {
	if r == nil {
		return
	}
	for _, p := range []Pool{r.write, r.read} {
		if c, ok := p.(interface{ Close() }); ok {
			c.Close()
		}
	}
}
