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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// recordingQuerier remembers every Exec.
type recordingQuerier struct {
	sql  []string
	args [][]any
}

func (q *recordingQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.sql = append(q.sql, sql)
	q.args = append(q.args, args)
	return pgconn.NewCommandTag("UPDATE 0"), nil
}
func (q *recordingQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (q *recordingQuerier) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }

func TestDeriveOnboarding_ScopesAndBlankIDs(t *testing.T) {
	q := &recordingQuerier{}
	ctx := context.Background()
	if err := deriveProjectOnboarding(ctx, q, ""); err != nil {
		t.Fatal(err)
	}
	if err := deriveOpportunityOnboarding(ctx, q, ""); err != nil {
		t.Fatal(err)
	}
	if len(q.sql) != 0 {
		t.Fatalf("blank ids ran %d statements", len(q.sql))
	}
	if err := deriveProjectOnboarding(ctx, q, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := deriveOpportunityOnboarding(ctx, q, "o1"); err != nil {
		t.Fatal(err)
	}
	// Each derivation takes the shared lock immediately before its statement.
	if len(q.sql) != 4 || !strings.Contains(q.sql[0], "pg_advisory_xact_lock") || q.sql[1] != deriveOnboardingForProjectQuery ||
		!strings.Contains(q.sql[2], "pg_advisory_xact_lock") || q.sql[3] != deriveOnboardingForOpportunityQuery {
		t.Fatalf("unexpected statements: %v", q.sql)
	}
	if q.args[0][0] != onboardingDeriveLockKey || q.args[2][0] != onboardingDeriveLockKey {
		t.Errorf("lock keys = %v, %v", q.args[0], q.args[2])
	}
	if q.args[1][1] != "p1" || q.args[3][1] != "o1" {
		t.Errorf("scope args = %v, %v", q.args[1], q.args[3])
	}
}

// TestDeriveOnboardingSQL_Guards pins the rule's guards in the SQL text; the
// behaviour itself is TestDeriveOnboarding_Postgres.
func TestDeriveOnboardingSQL_Guards(t *testing.T) {
	for _, want := range []string{
		"op.product_name = 'WSO2 Onboarding Services'",
		"op.service_end_date >= CURRENT_DATE",
		"p.onboarding_status IS NULL OR p.onboarding_status = 'NOT_APPLICABLE'",
		"ORDER BY k.project_id, op.service_start_date ASC NULLS LAST, op.id",
	} {
		if !strings.Contains(deriveOnboardingBody, want) {
			t.Errorf("rule SQL lacks %q", want)
		}
	}
	if strings.Contains(deriveOnboardingBody, "service_start_date <=") {
		t.Error("rule must not filter on the start date")
	}
}

// TestDeriveOnboarding_Postgres runs the rule against a real Postgres named by
// ENTITY_SERVICE_TEST_PG_DSN, in temporary tables inside a rolled-back
// transaction. Skipped when the variable is unset.
func TestDeriveOnboarding_Postgres(t *testing.T) {
	dsn := os.Getenv("ENTITY_SERVICE_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("ENTITY_SERVICE_TEST_PG_DSN not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, stmt := range []string{
		`CREATE TYPE pg_temp.onboarding_status_enum AS ENUM ('NOT_STARTED','IN_PROGRESS','COMPLETED','ON_HOLD','NOT_APPLICABLE','EXPIRED','CANCELLED')`,
		`CREATE TEMP TABLE project (id uuid PRIMARY KEY, onboarding_status pg_temp.onboarding_status_enum,
			onboarding_start_date date, onboarding_expiry_date date,
			total_onboarding_duration interval, remaining_onboarding_duration interval,
			updated_on timestamptz, updated_by text)`,
		`CREATE TEMP TABLE sf_opportunity_link (opportunity_id uuid, project_id uuid)`,
		`CREATE TEMP TABLE sf_opportunity_product (id uuid DEFAULT gen_random_uuid(), opportunity_id uuid,
			product_name text, service_start_date date, service_end_date date)`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	// The enum above lives in pg_temp; make the unqualified name in the rule resolve to it.
	if _, err := tx.Exec(ctx, `SET LOCAL search_path = pg_temp, public`); err != nil {
		t.Fatal(err)
	}

	type project struct {
		name, status string // status "" = NULL
		opp          string // "" = no link
		product      string // "" = no product line
		start, end   string // offsets from today, e.g. "+10 days"
		remaining    string // pre-existing remaining hours, "" = NULL
	}
	cases := []project{
		{name: "10000000-0000-0000-0000-000000000001", status: "NOT_APPLICABLE", opp: "a", product: onboardingProductName, start: "-30 days", end: "+60 days"},
		{name: "10000000-0000-0000-0000-000000000002", status: "", opp: "b", product: onboardingProductName, start: "+10 days", end: "+90 days"},
		{name: "10000000-0000-0000-0000-000000000003", status: "IN_PROGRESS", opp: "c", product: onboardingProductName, start: "-30 days", end: "+60 days", remaining: "50 hours"},
		{name: "10000000-0000-0000-0000-000000000004", status: "NOT_APPLICABLE", opp: "d", product: onboardingProductName, start: "-90 days", end: "-1 day"},
		{name: "10000000-0000-0000-0000-000000000005", status: "NOT_APPLICABLE", opp: "e", product: "Something Else", start: "-30 days", end: "+60 days"},
		{name: "10000000-0000-0000-0000-000000000006", status: "NOT_APPLICABLE"},
	}
	oppUUID := map[string]string{"a": "20000000-0000-0000-0000-00000000000a", "b": "20000000-0000-0000-0000-00000000000b",
		"c": "20000000-0000-0000-0000-00000000000c", "d": "20000000-0000-0000-0000-00000000000d", "e": "20000000-0000-0000-0000-00000000000e"}
	for _, c := range cases {
		if _, err := tx.Exec(ctx, `INSERT INTO project (id, onboarding_status, remaining_onboarding_duration) VALUES ($1, NULLIF($2,'')::onboarding_status_enum, NULLIF($3,'')::interval)`, c.name, c.status, c.remaining); err != nil {
			t.Fatal(err)
		}
		if c.opp == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sf_opportunity_link VALUES ($1, $2)`, oppUUID[c.opp], c.name); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sf_opportunity_product (opportunity_id, product_name, service_start_date, service_end_date)
			VALUES ($1, $2, CURRENT_DATE + $3::interval, CURRENT_DATE + $4::interval)`, oppUUID[c.opp], c.product, c.start, c.end); err != nil {
			t.Fatal(err)
		}
	}
	// Project 1 also has a second, later-starting qualifying line: the earliest start must win.
	if _, err := tx.Exec(ctx, `INSERT INTO sf_opportunity_product (opportunity_id, product_name, service_start_date, service_end_date)
		VALUES ($1, $2, CURRENT_DATE + 5, CURRENT_DATE + 400)`, oppUUID["a"], onboardingProductName); err != nil {
		t.Fatal(err)
	}

	// Run twice: the second pass must change nothing (idempotent).
	for pass := 0; pass < 2; pass++ {
		for _, id := range oppUUID {
			if err := deriveOpportunityOnboarding(ctx, tx, id); err != nil {
				t.Fatal(err)
			}
		}
		for _, c := range cases {
			if err := deriveProjectOnboarding(ctx, tx, c.name); err != nil {
				t.Fatal(err)
			}
		}
	}

	type got struct {
		status, startOff, endOff, total, remaining string
	}
	read := func(id string) got {
		var g got
		err := tx.QueryRow(ctx, `SELECT COALESCE(onboarding_status::text,'NULL'),
			COALESCE((onboarding_start_date - CURRENT_DATE)::text,'NULL'), COALESCE((onboarding_expiry_date - CURRENT_DATE)::text,'NULL'),
			COALESCE(ROUND(EXTRACT(EPOCH FROM total_onboarding_duration)/3600),-1)::int::text, COALESCE(ROUND(EXTRACT(EPOCH FROM remaining_onboarding_duration)/3600),-1)::int::text
			FROM project WHERE id = $1`, id).Scan(&g.status, &g.startOff, &g.endOff, &g.total, &g.remaining)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	want := map[string]got{
		// promoted; earliest-start line (-30 days) wins over the +5 days line
		cases[0].name: {"NOT_STARTED", "-30", "60", "200", "200"},
		// NULL status promoted, future start still qualifies
		cases[1].name: {"NOT_STARTED", "10", "90", "200", "200"},
		// other status kept, remaining hours kept, dates and total follow the line
		cases[2].name: {"IN_PROGRESS", "-30", "60", "200", "50"},
		// expired line: nothing
		cases[3].name: {"NOT_APPLICABLE", "NULL", "NULL", "-1", "-1"},
		// other product: nothing
		cases[4].name: {"NOT_APPLICABLE", "NULL", "NULL", "-1", "-1"},
		// no link: nothing
		cases[5].name: {"NOT_APPLICABLE", "NULL", "NULL", "-1", "-1"},
	}
	for id, w := range want {
		if g := read(id); g != w {
			t.Errorf("project %s = %+v, want %+v", id, g, w)
		}
	}
}

// TestDeriveOnboarding_PostgresConcurrentLinkAndLine: a link and its onboarding
// line are written in two concurrent transactions. Each derives right after its
// own write; the shared lock makes the second derive wait for the first commit
// and then see both rows, so the project ends NOT_STARTED whichever writer
// commits second. Needs ENTITY_SERVICE_TEST_PG_DSN; creates and drops its own
// schema.
func TestDeriveOnboarding_PostgresConcurrentLinkAndLine(t *testing.T) {
	dsn := os.Getenv("ENTITY_SERVICE_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("ENTITY_SERVICE_TEST_PG_DSN not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	const schema = "onb_derive_concurrency_test"
	for _, stmt := range []string{
		`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`,
		`CREATE SCHEMA ` + schema,
		`CREATE TYPE ` + schema + `.onboarding_status_enum AS ENUM ('NOT_STARTED','IN_PROGRESS','COMPLETED','ON_HOLD','NOT_APPLICABLE','EXPIRED','CANCELLED')`,
		`CREATE TABLE ` + schema + `.project (id uuid PRIMARY KEY, onboarding_status ` + schema + `.onboarding_status_enum,
			onboarding_start_date date, onboarding_expiry_date date, total_onboarding_duration interval,
			remaining_onboarding_duration interval, updated_on timestamptz, updated_by text)`,
		`CREATE TABLE ` + schema + `.sf_opportunity_link (opportunity_id uuid, project_id uuid, link_sf_id text)`,
		`CREATE TABLE ` + schema + `.sf_opportunity_product (id uuid DEFAULT gen_random_uuid(), line_item_sf_id text, opportunity_id uuid,
			product_name text, service_start_date date, service_end_date date)`,
		`INSERT INTO ` + schema + `.project (id, onboarding_status) VALUES ('30000000-0000-0000-0000-000000000001', 'NOT_APPLICABLE')`,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	defer func() { _, _ = admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()

	const projectID, oppID = "30000000-0000-0000-0000-000000000001", "40000000-0000-0000-0000-000000000001"
	begin := func() (*pgx.Conn, pgx.Tx) {
		c, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := c.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `SET LOCAL search_path = `+schema+`, public`); err != nil {
			t.Fatal(err)
		}
		return c, tx
	}
	cA, txA := begin()
	cB, txB := begin()
	defer cA.Close(ctx)
	defer cB.Close(ctx)

	// A: the link writer. Writes its row, derives (no line visible yet).
	if _, err := txA.Exec(ctx, `INSERT INTO sf_opportunity_link VALUES ($1, $2, 'lnk')`, oppID, projectID); err != nil {
		t.Fatal(err)
	}
	if err := deriveProjectOnboarding(ctx, txA, projectID); err != nil {
		t.Fatal(err)
	}
	// B: the line item writer. Writes its row (A's link is uncommitted, invisible), then derives.
	if _, err := txB.Exec(ctx, `INSERT INTO sf_opportunity_product (opportunity_id, line_item_sf_id, product_name, service_start_date, service_end_date)
		VALUES ($1, 'li', $2, CURRENT_DATE - 1, CURRENT_DATE + 30)`, oppID, onboardingProductName); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- deriveOpportunityOnboarding(ctx, txB, oppID) }()
	select {
	case err := <-done:
		t.Fatalf("B derived without waiting for A to commit (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := txA.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B still blocked after A committed")
	}
	if err := txB.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := admin.QueryRow(ctx, `SELECT onboarding_status::text FROM `+schema+`.project WHERE id = $1`, projectID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "NOT_STARTED" {
		t.Errorf("status = %s, want NOT_STARTED", status)
	}

	// The previous-owner reads return what the rows hold (and an empty slice for none).
	if _, err := admin.Exec(ctx, `SET search_path = `+schema+`, public`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ sql, key, want string }{
		{previousLinkProjectsQuery, "lnk", projectID},
		{previousLineItemOpportunitiesQuery, "li", oppID},
		{previousLinkProjectsQuery, "none", ""},
	} {
		got, err := previousIDs(ctx, admin, c.sql, c.key)
		if err != nil {
			t.Fatal(err)
		}
		if (c.want == "" && len(got) != 0) || (c.want != "" && (len(got) != 1 || got[0] != c.want)) {
			t.Errorf("%.40s %q = %v, want [%s]", c.sql, c.key, got, c.want)
		}
	}
}
