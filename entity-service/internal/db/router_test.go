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
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// fakePool records which methods were called and can fail Ping. Embedding the
// interface leaves every other method nil: a test that reaches one would panic,
// which is the right failure for an unexpected call.
type fakePool struct {
	Pool
	name    string
	calls   []string
	pingErr error
	closed  bool
}

func (f *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.calls = append(f.calls, "Query")
	return nil, nil
}
func (f *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.calls = append(f.calls, "QueryRow")
	return nil
}
func (f *fakePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.calls = append(f.calls, "Exec")
	return pgconn.CommandTag{}, nil
}
func (f *fakePool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	f.calls = append(f.calls, "SendBatch")
	return nil
}
func (f *fakePool) Begin(ctx context.Context) (pgx.Tx, error) {
	f.calls = append(f.calls, "Begin")
	return nil, nil
}
func (f *fakePool) BeginTx(ctx context.Context, o pgx.TxOptions) (pgx.Tx, error) {
	f.calls = append(f.calls, "BeginTx")
	return nil, nil
}
func (f *fakePool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	f.calls = append(f.calls, "Acquire")
	return nil, nil
}
func (f *fakePool) Ping(ctx context.Context) error {
	f.calls = append(f.calls, "Ping")
	return f.pingErr
}
func (f *fakePool) Close() { f.closed = true }

// callEach invokes every Pool method once on p.
func callEach(ctx context.Context, p Pool) []string {
	_, _ = p.Query(ctx, "")
	_ = p.QueryRow(ctx, "")
	_, _ = p.Exec(ctx, "")
	_ = p.SendBatch(ctx, &pgx.Batch{})
	_, _ = p.Begin(ctx)
	_, _ = p.BeginTx(ctx, pgx.TxOptions{})
	_, _ = p.Acquire(ctx)
	return []string{"Query", "QueryRow", "Exec", "SendBatch", "Begin", "BeginTx", "Acquire"}
}

func TestRouter_RoutesByContext(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		withRead  bool
		wantWrite bool
	}{
		{"unmarked goes to write", context.Background(), true, true},
		{"read-marked goes to read", WithReadOnly(context.Background()), true, false},
		{"read-marked without a read pool goes to write", WithReadOnly(context.Background()), false, true},
		{"unmarked without a read pool goes to write", context.Background(), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, r := &fakePool{name: "w"}, &fakePool{name: "r"}
			var read Pool
			if tc.withRead {
				read = r
			}
			want := callEach(tc.ctx, NewRouter(w, read))
			hit, other := r, w
			if tc.wantWrite {
				hit, other = w, r
			}
			if len(hit.calls) != len(want) {
				t.Errorf("serving pool calls = %v, want %v", hit.calls, want)
			}
			for i, c := range want {
				if i < len(hit.calls) && hit.calls[i] != c {
					t.Errorf("call %d = %s, want %s", i, hit.calls[i], c)
				}
			}
			if len(other.calls) != 0 {
				t.Errorf("other pool was called: %v", other.calls)
			}
		})
	}
}

func TestReadOnlyMarker(t *testing.T) {
	bg := context.Background()
	if IsReadOnly(bg) {
		t.Fatal("background context must not be read-only")
	}
	marked := WithReadOnly(bg)
	if !IsReadOnly(marked) {
		t.Fatal("WithReadOnly context must be read-only")
	}
	// Deriving from a marked ctx keeps the mark; the parent stays unmarked.
	type k struct{}
	if !IsReadOnly(context.WithValue(marked, k{}, 1)) {
		t.Error("child of a marked context lost the marker")
	}
	if IsReadOnly(bg) {
		t.Error("marking a child leaked into the parent")
	}
	// A foreign key with the same underlying value must not collide.
	if IsReadOnly(context.WithValue(bg, struct{}{}, true)) {
		t.Error("an unrelated context key was read as the marker")
	}
}

func TestRouter_Ping(t *testing.T) {
	boom := errors.New("down")
	for _, tc := range []struct {
		name         string
		writeErr     error
		readErr      error
		withRead     bool
		wantErr      bool
		wantReadPing bool
	}{
		{"both healthy", nil, nil, true, false, true},
		{"read down fails health", nil, boom, true, true, true},
		{"write down fails health", boom, nil, true, true, true},
		{"no read pool, write healthy", nil, nil, false, false, false},
		{"no read pool, write down", boom, nil, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, r := &fakePool{pingErr: tc.writeErr}, &fakePool{pingErr: tc.readErr}
			var read Pool
			if tc.withRead {
				read = r
			}
			// An unmarked ctx: Ping must hit both regardless of the marker.
			err := NewRouter(w, read).Ping(context.Background())
			if (err != nil) != tc.wantErr {
				t.Errorf("Ping err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(w.calls) != 1 {
				t.Errorf("write pings = %d, want 1", len(w.calls))
			}
			if got := len(r.calls) == 1; got != tc.wantReadPing {
				t.Errorf("read pinged = %v, want %v", got, tc.wantReadPing)
			}
		})
	}
}

func TestRouter_Close(t *testing.T) {
	w, r := &fakePool{}, &fakePool{}
	NewRouter(w, r).Close()
	if !w.closed || !r.closed {
		t.Errorf("Close: write closed=%v read closed=%v, want both", w.closed, r.closed)
	}
	var nilRouter *Router
	nilRouter.Close() // must not panic
}

// A deployment with no database must stay a true nil Pool all the way through,
// or every `pool != nil` gate would pass and call into a nil pool.
func TestRouter_NoDatabaseStaysTrueNil(t *testing.T) {
	var nilPgx *pgxpool.Pool
	if FromPgx(nilPgx) != nil {
		t.Error("FromPgx(nil) is a non-nil interface")
	}
	if NewRouter(nil, nil) != nil {
		t.Error("NewRouter(nil, nil) is not a nil *Router")
	}
	var r *Router
	if r.Pool() != nil {
		t.Error("nil Router.Pool() is a non-nil interface")
	}
	if r.ReadEnabled() {
		t.Error("nil Router reports a read pool")
	}

	got, err := NewRouterIfNeeded(&config.Config{DataSource: config.DataSourceServiceNow})
	if err != nil || got != nil {
		t.Fatalf("NewRouterIfNeeded with no database = (%v, %v), want (nil, nil)", got, err)
	}
	if got.Pool() != nil {
		t.Error("Pool() of the no-database router is a non-nil interface")
	}
}

func TestRouter_Describe(t *testing.T) {
	if got := NewRouter(&fakePool{}, nil).Describe(); got != "db pools: write only" {
		t.Errorf("Describe() = %q", got)
	}
	var r *Router
	if r.Describe() == "" {
		t.Error("nil Router Describe() is empty")
	}
}
