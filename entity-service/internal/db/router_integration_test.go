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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Skipped unless ENTITY_TEST_DATABASE_URL is set (same variable the repository
// integration tests use). It performs no successful write: the only write
// attempted is one the read pool must reject.
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestRouterIntegration ./internal/db/
func TestRouterIntegration_ReadPoolRejectsWrites(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	write, err := NewPool(ctx, dsn, 2, 0, time.Minute, time.Minute)
	if err != nil {
		t.Fatalf("write pool: %v", err)
	}
	read, err := NewReadPool(ctx, dsn, 2, 0, time.Minute, time.Minute)
	if err != nil {
		t.Fatalf("read pool: %v", err)
	}
	r := NewRouter(write, read)
	defer r.Close()

	show := func(c context.Context) string {
		var v string
		if err := r.QueryRow(c, "SHOW default_transaction_read_only").Scan(&v); err != nil {
			t.Fatalf("SHOW: %v", err)
		}
		return v
	}
	if got := show(ctx); got != "off" {
		t.Errorf("unmarked ctx session read_only = %q, want off (write pool)", got)
	}
	ro := WithReadOnly(ctx)
	if got := show(ro); got != "on" {
		t.Errorf("read-marked ctx session read_only = %q, want on (read pool)", got)
	}

	// A write on the read pool must fail loudly, not fall back or succeed.
	_, err = r.Exec(ro, "CREATE TABLE router_read_pool_probe (i int)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Errorf("write via read pool error = %v, want SQLSTATE 25006", err)
	}
	if _, err := r.Exec(ro, "SELECT 1"); err != nil {
		t.Errorf("read via read pool: %v", err)
	}
	if err := r.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
