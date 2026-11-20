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

// The internal-only work item guard: the Go denylist agrees with
// work_item_type_is_internal_only() (migration 100029), and the guard refuses a
// product issue's id while passing every other type. Skipped without
// ENTITY_TEST_DATABASE_URL.

package repository

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func internalOnlyTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestInternalOnlyWorkItemTypesMatchTheSQLFunction(t *testing.T) {
	pool := internalOnlyTestPool(t)
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
		SELECT e::text FROM unnest(enum_range(NULL::work_item_type_enum)) AS e
		WHERE work_item_type_is_internal_only(e)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var fromSQL []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		fromSQL = append(fromSQL, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	fromGo := append([]string(nil), InternalOnlyWorkItemTypes...)
	sort.Strings(fromSQL)
	sort.Strings(fromGo)
	if len(fromSQL) != len(fromGo) {
		t.Fatalf("SQL function says %v internal-only, Go list says %v", fromSQL, fromGo)
	}
	for i := range fromSQL {
		if fromSQL[i] != fromGo[i] {
			t.Fatalf("SQL function says %v internal-only, Go list says %v", fromSQL, fromGo)
		}
	}
}

func TestWorkItemTypeGuard_RefusesOnlyInternalOnlyTypes(t *testing.T) {
	pool := internalOnlyTestPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	insert := func(typ string) string {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, wso2_id)
			VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number($1::work_item_type_enum), 'guard test', $1::work_item_type_enum,
			        CASE WHEN $1 = 'CASE' THEN 'GUARD-' || substr(md5(random()::text), 1, 10) END)
			RETURNING id::text`, typ).Scan(&id)
		if err != nil {
			t.Fatalf("insert %s: %v", typ, err)
		}
		return id
	}
	for typ, wantRefused := range map[string]bool{"PRODUCT_ISSUE": true, "CASE": false, "CHANGE_REQUEST": false, "INCIDENT": false} {
		err := rejectInternalOnlyWorkItem(ctx, tx, insert(typ), "case not found")
		var nfe *apierror.NotFoundError
		if wantRefused != errors.As(err, &nfe) {
			t.Errorf("%s: refused=%v err=%v, want refused=%v", typ, errors.As(err, &nfe), err, wantRefused)
		}
		if !wantRefused && err != nil {
			t.Errorf("%s: unexpected error %v", typ, err)
		}
	}
	// An id that exists nowhere, and one that is not a UUID, pass untouched.
	for _, id := range []string{"0190a4b0-0000-7000-8000-00000000dead", "not-a-uuid"} {
		if err := rejectInternalOnlyWorkItem(ctx, tx, id, "case not found"); err != nil {
			t.Errorf("%q: %v", id, err)
		}
	}
}
