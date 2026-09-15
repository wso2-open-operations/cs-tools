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

package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These are integration tests against two real, throwaway local Postgres
// databases standing in for csm-sync-service's database and entity-service's
// own database, set up by hand in this sandbox (see the return notes for the
// exact DDL used):
//
//	kbmigrate_test_csmsync: CREATE TABLE "user" (id UUID PRIMARY KEY, email TEXT NOT NULL UNIQUE, user_name TEXT);
//	kbmigrate_test_entity:  CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE);
//	                        CREATE TABLE cases (id UUID PRIMARY KEY);
//
// Both default DSNs can be overridden via KBMIGRATE_TEST_CSMSYNC_DSN /
// KBMIGRATE_TEST_ENTITY_DSN. If neither database is reachable (e.g. CI with
// no local Postgres) these tests skip rather than fail.
const (
	defaultTestCSMSyncDSN = "postgres://localhost:5432/kbmigrate_test_csmsync?sslmode=disable"
	defaultTestEntityDSN  = "postgres://localhost:5432/kbmigrate_test_entity?sslmode=disable"
)

func dsnFromEnv(envVar, fallback string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return fallback
}

func connectOrSkip(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("cannot construct pool for %s: %v (this integration test needs a real local Postgres -- see doc comment)", dsn, err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("cannot reach %s: %v (this integration test needs a real local Postgres -- see doc comment)", dsn, err)
	}
	return pool
}

func TestRealUserResolver_ResolvesViaEmailJoin(t *testing.T) {
	csmSyncPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_CSMSYNC_DSN", defaultTestCSMSyncDSN))
	defer csmSyncPool.Close()
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_ENTITY_DSN", defaultTestEntityDSN))
	defer entityPool.Close()

	ctx := context.Background()
	snSysID := "1111aaaa1111aaaa1111aaaa1111aaaa"
	derivedUUID := sysIDToUUID(snSysID)
	email := "jane.doe@example.com"
	entityUserID := "entity-user-jane-001"

	mustExec(t, ctx, csmSyncPool, `DELETE FROM "user" WHERE id = $1`, derivedUUID)
	mustExec(t, ctx, entityPool, `DELETE FROM users WHERE email = $1`, email)
	mustExec(t, ctx, csmSyncPool, `INSERT INTO "user" (id, email, user_name) VALUES ($1, $2, 'jane.doe')`, derivedUUID, email)
	mustExec(t, ctx, entityPool, `INSERT INTO users (id, email) VALUES ($1, $2)`, entityUserID, email)

	r := newRealUserResolver(csmSyncPool, entityPool)
	got, err := r.ResolveUser(ctx, snSysID)
	if err != nil {
		t.Fatalf("ResolveUser: %v", err)
	}
	if got != entityUserID {
		t.Errorf("ResolveUser = %q, want %q", got, entityUserID)
	}

	// Cached on second call -- delete the underlying rows and confirm the
	// resolver still returns the cached result rather than re-querying.
	mustExec(t, ctx, csmSyncPool, `DELETE FROM "user" WHERE id = $1`, derivedUUID)
	got2, err := r.ResolveUser(ctx, snSysID)
	if err != nil || got2 != entityUserID {
		t.Errorf("expected a cached result after underlying row deletion, got (%q, %v)", got2, err)
	}
}

func TestRealUserResolver_MissingInCSMSyncUserTable(t *testing.T) {
	csmSyncPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_CSMSYNC_DSN", defaultTestCSMSyncDSN))
	defer csmSyncPool.Close()
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_ENTITY_DSN", defaultTestEntityDSN))
	defer entityPool.Close()

	ctx := context.Background()
	snSysID := "deadbeefdeadbeefdeadbeefdeadbeef"
	mustExec(t, ctx, csmSyncPool, `DELETE FROM "user" WHERE id = $1`, sysIDToUUID(snSysID))

	r := newRealUserResolver(csmSyncPool, entityPool)
	if _, err := r.ResolveUser(ctx, snSysID); err == nil {
		t.Fatal("expected an error when the sys_user is absent from csm-sync-service's user table")
	}
}

func TestRealUserResolver_ResolvedEmailMissingInEntityUsers(t *testing.T) {
	csmSyncPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_CSMSYNC_DSN", defaultTestCSMSyncDSN))
	defer csmSyncPool.Close()
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_ENTITY_DSN", defaultTestEntityDSN))
	defer entityPool.Close()

	ctx := context.Background()
	snSysID := "cafefeedcafefeedcafefeedcafefeed"
	derivedUUID := sysIDToUUID(snSysID)
	email := "orphan.user@example.com"

	mustExec(t, ctx, csmSyncPool, `DELETE FROM "user" WHERE id = $1`, derivedUUID)
	mustExec(t, ctx, entityPool, `DELETE FROM users WHERE email = $1`, email)
	mustExec(t, ctx, csmSyncPool, `INSERT INTO "user" (id, email, user_name) VALUES ($1, $2, 'orphan')`, derivedUUID, email)
	// Deliberately NOT inserted into entityPool.users.

	r := newRealUserResolver(csmSyncPool, entityPool)
	if _, err := r.ResolveUser(ctx, snSysID); err == nil {
		t.Fatal("expected an error when the resolved email has no entity-service users row")
	}
}

func TestRealCaseResolver(t *testing.T) {
	entityPool := connectOrSkip(t, dsnFromEnv("KBMIGRATE_TEST_ENTITY_DSN", defaultTestEntityDSN))
	defer entityPool.Close()

	ctx := context.Background()
	existingSysID := "0123456789abcdef0123456789abcdef"
	existingUUID := sysIDToUUID(existingSysID)
	mustExec(t, ctx, entityPool, `DELETE FROM cases WHERE id = $1`, existingUUID)
	mustExec(t, ctx, entityPool, `INSERT INTO cases (id) VALUES ($1)`, existingUUID)

	r := newRealCaseResolver(entityPool)

	t.Run("existing case resolves and exists", func(t *testing.T) {
		uuid, exists, err := r.ResolveCase(ctx, existingSysID)
		if err != nil {
			t.Fatalf("ResolveCase: %v", err)
		}
		if !exists || uuid != existingUUID {
			t.Errorf("got (%q, %v), want (%q, true)", uuid, exists, existingUUID)
		}
	})

	t.Run("non-existent derived case is reported absent, not an error", func(t *testing.T) {
		missingSysID := "ffffffffffffffffffffffffffffffff"
		uuid, exists, err := r.ResolveCase(ctx, missingSysID)
		if err != nil {
			t.Fatalf("ResolveCase: %v", err)
		}
		if exists {
			t.Errorf("expected exists=false for an unmigrated case sys_id, got uuid=%q", uuid)
		}
	})

	t.Run("empty sys_id resolves to absent, not an error", func(t *testing.T) {
		uuid, exists, err := r.ResolveCase(ctx, "")
		if err != nil || exists || uuid != "" {
			t.Errorf("got (%q, %v, %v), want (\"\", false, nil)", uuid, exists, err)
		}
	})
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
