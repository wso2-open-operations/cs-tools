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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestReadPoolServesMarkedReadPaths runs the repository calls behind routes
// that are marked read-only (see readOnlyRoutes in internal/server/routes.go)
// through a db.Router whose read pool has default_transaction_read_only=on, and
// asserts none of them hits SQLSTATE 25006. Covered: a plain Scoped.Query
// search (case search), a grouped aggregate (case aggregate), a project search,
// and a Scoped.InTx read (project case stats), which opens a transaction on the
// read pool and sets the caller identity with transaction-local set_config.
//
// It writes nothing. The only write attempted is a probe that must fail, to
// prove the read pool is really in effect. Skipped unless
// ENTITY_TEST_DATABASE_URL is set (the variable the other repository
// integration tests use):
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestReadPoolServesMarkedReadPaths ./internal/repository/
func TestReadPoolServesMarkedReadPaths(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set; skipping the live-database test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	write, err := db.NewPool(ctx, dsn, 2, 0, time.Minute, time.Minute)
	if err != nil {
		t.Fatalf("write pool: %v", err)
	}
	read, err := db.NewReadPool(ctx, dsn, 4, 0, time.Minute, time.Minute)
	if err != nil {
		t.Fatalf("read pool: %v", err)
	}
	router := db.NewRouter(write, read)
	defer router.Close()
	scoped := NewScoped(router.Pool())

	// The request context as the identity middleware leaves it, then marked the
	// way middleware.ReadOnly marks it.
	ro := db.WithReadOnly(WithSystemIdentity(ctx))

	// Control: the marked context really lands on a read-only session, and a
	// write there is rejected. Without this a green run could just mean the
	// router sent everything to the write pool.
	var mode string
	if err := router.QueryRow(ro, "SHOW default_transaction_read_only").Scan(&mode); err != nil || mode != "on" {
		t.Fatalf("marked ctx session read_only = %q (err %v), want on", mode, err)
	}
	_, err = router.Exec(ro, "CREATE TABLE read_pool_routes_probe (i int)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Fatalf("control write via read pool error = %v, want SQLSTATE 25006", err)
	}

	scope := SearchScope{Unrestricted: true}
	page := domain.Pagination{Limit: 5}

	t.Run("case search", func(t *testing.T) {
		if _, _, err := NewCaseRepository(scoped).SearchCases(ro, domain.SearchCasesRequest{
			SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
			Pagination: page,
		}, scope); err != nil {
			t.Fatalf("SearchCases on the read pool: %v", err)
		}
	})
	t.Run("case aggregate", func(t *testing.T) {
		if _, err := NewCaseRepository(scoped).AggregateCases(ro, domain.SearchCasesRequest{Pagination: page}, "state", scope); err != nil {
			t.Fatalf("AggregateCases on the read pool: %v", err)
		}
	})
	t.Run("project search", func(t *testing.T) {
		if _, _, err := NewProjectRepository(scoped).SearchProjects(ro, domain.SearchProjectsRequest{Pagination: page}, scope); err != nil {
			t.Fatalf("SearchProjects on the read pool: %v", err)
		}
	})
	t.Run("project case stats via InTx", func(t *testing.T) {
		// Any project id exercises the SQL; an unmatched one just returns no rows.
		projectID := "00000000-0000-0000-0000-000000000000"
		_ = router.QueryRow(ro, "SELECT id::TEXT FROM project LIMIT 1").Scan(&projectID)
		f := ProjectCaseStatsFilter{ProjectID: projectID, Types: []string{"CASE"}}
		if _, err := NewProjectCaseStatsRepository(scoped).StateSeverityCounts(ro, f); err != nil {
			t.Fatalf("StateSeverityCounts (InTx) on the read pool: %v", err)
		}
	})
}
