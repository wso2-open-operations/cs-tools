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

// Integration test for CheckRLSProtection's SQL. The unit tests prove the
// verdict logic; only a real Postgres can prove the catalog query itself
// (owner exemption, FORCE, inherited ownership, BYPASSRLS, superuser) agrees
// with how Postgres really applies row-level security, so this test builds a
// throwaway schema and roles, connects as each, and also checks the verdict
// against actual row visibility.
//
// It needs a superuser (or at least CREATEROLE + CREATEDB-free schema rights)
// DSN for a scratch database and is skipped when the variable is unset:
//
//	RLS_PROTECTION_TEST_ADMIN_DSN=postgres://... go test ./internal/db/ -run RLSProtectionIntegration

package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestRLSProtectionIntegration(t *testing.T) {
	dsn := os.Getenv("RLS_PROTECTION_TEST_ADMIN_DSN")
	if dsn == "" {
		t.Skip("RLS_PROTECTION_TEST_ADMIN_DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adminCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("connect as admin: %v", err)
	}
	// Registered before the drop-everything cleanup below so it runs after it
	// (t.Cleanup is last-in, first-out); a deferred Close would run first and
	// leave the throwaway schema and roles behind.
	t.Cleanup(func() { admin.Close(context.Background()) })

	suffix := randSuffix(t)
	schema := "rlsprot_" + suffix
	owner := "rlsprot_owner_" + suffix
	app := "rlsprot_app_" + suffix
	member := "rlsprot_member_" + suffix // inherits the owner role
	bypass := "rlsprot_bypass_" + suffix
	pw := "pw_" + suffix

	exec := func(sql string) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql); err != nil {
			t.Fatalf("setup %q: %v", sql, err)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = admin.Exec(c, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
		for _, r := range []string{member, bypass, app, owner} {
			_, _ = admin.Exec(c, fmt.Sprintf("DROP ROLE IF EXISTS %s", r))
		}
	})

	exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS", owner, pw))
	exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS", app, pw))
	exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS INHERIT IN ROLE %s", member, pw, owner))
	exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER BYPASSRLS", bypass, pw))
	exec(fmt.Sprintf("CREATE SCHEMA %s AUTHORIZATION %s", schema, owner))
	exec(fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s, %s, %s", schema, app, member, bypass))
	// One table is enough to exercise every combination; the policy hides every
	// row from a caller without the GUC, the same shape as the real policies.
	exec(fmt.Sprintf("CREATE TABLE %s.t (id int) ", schema))
	exec(fmt.Sprintf("ALTER TABLE %s.t OWNER TO %s", schema, owner))
	exec(fmt.Sprintf("INSERT INTO %s.t VALUES (1), (2), (3)", schema))
	exec(fmt.Sprintf("GRANT SELECT ON %s.t TO %s, %s, %s", schema, app, member, bypass))
	exec(fmt.Sprintf("ALTER TABLE %s.t ENABLE ROW LEVEL SECURITY", schema))
	exec(fmt.Sprintf("CREATE POLICY p ON %s.t USING (current_setting('app.is_internal', true) = 'true')", schema))

	// connect opens a session as role with the throwaway schema as current_schema().
	connect := func(role, password string) *pgx.Conn {
		t.Helper()
		cfg := adminCfg.Copy()
		cfg.User = role
		cfg.Password = password
		cfg.RuntimeParams["search_path"] = schema
		c, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("connect as %s: %v", role, err)
		}
		t.Cleanup(func() { c.Close(context.Background()) })
		return c
	}
	visible := func(c *pgx.Conn) int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil {
			t.Fatalf("count rows: %v", err)
		}
		return n
	}

	type want struct {
		exempt bool
		rows   int // rows a session with NO identity sees: 0 = bound, 3 = exempt
	}
	check := func(label string, c *pgx.Conn, w want) {
		t.Helper()
		p, err := CheckRLSProtection(ctx, c)
		if err != nil {
			t.Fatalf("%s: CheckRLSProtection: %v", label, err)
		}
		if p.Tables != 1 {
			t.Errorf("%s: Tables = %d, want 1 (%s)", label, p.Tables, p.Summary())
		}
		if p.Exempt() != w.exempt {
			t.Errorf("%s: Exempt() = %v, want %v (%s)", label, p.Exempt(), w.exempt, p.Summary())
		}
		// The verdict must agree with what Postgres actually does.
		if got := visible(c); got != w.rows {
			t.Errorf("%s: a session with no identity sees %d rows, want %d", label, got, w.rows)
		}
	}

	// 1. Table forced: even the owner is bound.
	exec(fmt.Sprintf("ALTER TABLE %s.t FORCE ROW LEVEL SECURITY", schema))
	check("owner, FORCE on", connect(owner, pw), want{exempt: false, rows: 0})
	check("member of owner, FORCE on", connect(member, pw), want{exempt: false, rows: 0})
	check("non-owner app role, FORCE on", connect(app, pw), want{exempt: false, rows: 0})
	check("BYPASSRLS role, FORCE on", connect(bypass, pw), want{exempt: true, rows: 3})
	check("superuser, FORCE on", connect(adminCfg.User, adminCfg.Password), want{exempt: true, rows: 3})

	// 2. Table not forced: the owner and anything inheriting it are exempt; a
	// separate non-owner role is still bound. This is the "separate
	// application role" deployment.
	exec(fmt.Sprintf("ALTER TABLE %s.t NO FORCE ROW LEVEL SECURITY", schema))
	check("owner, FORCE off", connect(owner, pw), want{exempt: true, rows: 3})
	check("member of owner, FORCE off", connect(member, pw), want{exempt: true, rows: 3})
	check("non-owner app role, FORCE off", connect(app, pw), want{exempt: false, rows: 0})
	check("BYPASSRLS role, FORCE off", connect(bypass, pw), want{exempt: true, rows: 3})

	// 3. RLS disabled altogether: nothing to be exempt from, nothing flagged.
	exec(fmt.Sprintf("ALTER TABLE %s.t DISABLE ROW LEVEL SECURITY", schema))
	p, err := CheckRLSProtection(ctx, connect(owner, pw))
	if err != nil {
		t.Fatalf("CheckRLSProtection with RLS off: %v", err)
	}
	if p.InUse() || p.Exempt() {
		t.Errorf("RLS disabled: InUse=%v Exempt=%v, want neither (%s)", p.InUse(), p.Exempt(), p.Summary())
	}
}

func randSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(b)
}
