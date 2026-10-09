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

// Package testdb picks the Postgres database DB-backed tests run against.
//
// Several tests TRUNCATE tables or rewrite rows, so they must never share the
// developer's working database. Tests therefore ignore DATABASE_URL and read
// TEST_DATABASE_URL instead, defaulting to a separate "gid_test" database on
// the compose Postgres (created and migrated by `make test-db`). As a second
// line of defence, a database whose name does not end in "_test" is refused.
package testdb

import (
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

const defaultURL = "postgres://gid:gid@localhost:5433/gid_test?sslmode=disable"

// URL returns the DSN for DB-backed tests, skipping the test when it does not
// name a "_test" database.
func URL(t testing.TB) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultURL
	}
	if !isTestDatabase(dsn) {
		t.Skipf("skipping: TEST_DATABASE_URL must name a database ending in _test (tests truncate tables)")
	}
	return dsn
}

// isTestDatabase reports whether the database pgx will actually connect to
// for dsn ends in "_test". The DSN is parsed with pgx rather than read as a
// URL path because pgx honours a ?dbname= parameter (or a keyword/value
// dbname=) over the path, so the path alone can name a different database
// than the one that gets truncated.
func isTestDatabase(dsn string) bool {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return false
	}
	return strings.HasSuffix(cfg.Database, "_test")
}
