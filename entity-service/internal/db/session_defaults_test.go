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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplySessionDefaults_SetsTimeoutsAndKeepsDSNValues(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@localhost:5432/db?sslmode=disable&statement_timeout=120000")
	if err != nil {
		t.Fatal(err)
	}
	applySessionDefaults(cfg.ConnConfig.RuntimeParams)
	want := map[string]string{
		"jit":                                 "off",
		"statement_timeout":                   "120000", // the DSN's own value wins
		"lock_timeout":                        "10000",
		"idle_in_transaction_session_timeout": "60000",
	}
	for k, v := range want {
		if got := cfg.ConnConfig.RuntimeParams[k]; got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	params := map[string]string{}
	applySessionDefaults(params)
	if params["statement_timeout"] != "30000" {
		t.Errorf("default statement_timeout = %q, want 30000", params["statement_timeout"])
	}
}

// The server must actually apply the defaults. Skipped without
// ENTITY_TEST_DATABASE_URL.
func TestNewPool_ServerReportsSessionTimeouts(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	for setting, want := range map[string]string{
		"statement_timeout":                   "30s",
		"lock_timeout":                        "10s",
		"idle_in_transaction_session_timeout": "1min",
		"jit":                                 "off",
	} {
		var got string
		if err := pool.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&got); err != nil {
			t.Fatalf("current_setting(%s): %v", setting, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", setting, got, want)
		}
	}
}
