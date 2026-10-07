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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the set of connection-pool operations the entity service calls.
// Repositories and Scoped hold a Pool rather than a concrete *pgxpool.Pool so
// that the choice of which underlying pool serves a call can later be made
// per call (every method takes a context) without touching any repository.
//
// *pgxpool.Pool satisfies it as-is. The method set is deliberately limited to
// what the code actually uses; add a method only when a caller needs it.
//
// A nil pool must be represented as a nil Pool interface, never as a nil
// *pgxpool.Pool stored in one (that interface would compare non-nil). Use
// FromPgx to convert.
type Pool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
	Ping(ctx context.Context) error
}

var _ Pool = (*pgxpool.Pool)(nil)

// FromPgx converts a concrete pool to a Pool, preserving nil: a nil
// *pgxpool.Pool yields a nil Pool, so existing `pool == nil` checks (a
// deployment with no database) keep working.
func FromPgx(p *pgxpool.Pool) Pool {
	if p == nil {
		return nil
	}
	return p
}
