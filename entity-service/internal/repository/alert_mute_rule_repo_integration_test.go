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

// This is an integration test: it applies migrations 0278 and 100028 (twice each, to prove they are safe to re-run) with
// 0180's number series in a private schema and runs AlertMuteRuleRepository against it. Skipped without a DSN:
//
//	ALERT_MUTE_RULE_TEST_DSN=postgres://... go test ./internal/repository/ -run AlertMuteRule
//
// The server must be PostgreSQL 18+ (uuidv7). A superuser connection bypasses row-level security, so this checks that
// 100028 installs its policy, not that the policy filters.

package repository_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func alertMuteRuleTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ALERT_MUTE_RULE_TEST_DSN")
	if dsn == "" {
		t.Skip("ALERT_MUTE_RULE_TEST_DSN not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("alert_mute_rule_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to schema: %v", err)
	}
	t.Cleanup(pool.Close)

	numbering, err := os.ReadFile("../../migrations/0180_native_number_series.sql")
	if err != nil {
		t.Fatalf("read 0180: %v", err)
	}
	for _, part := range []struct{ from, to string }{
		{"CREATE TABLE number_series (", ");"},
		{"CREATE OR REPLACE FUNCTION next_series_number(", "$$ LANGUAGE plpgsql;"},
	} {
		sql := string(numbering)
		start := strings.Index(sql, part.from)
		end := strings.Index(sql[start:], part.to)
		if start < 0 || end < 0 {
			t.Fatalf("0180 no longer holds %q", part.from)
		}
		if _, err := pool.Exec(ctx, sql[start:start+end+len(part.to)]); err != nil {
			t.Fatalf("apply 0180's %q: %v", part.from, err)
		}
	}
	for _, file := range []string{"0278_alert_mute_rule.sql", "0278_alert_mute_rule.sql", "100028_alert_mute_rule_rls.sql", "100028_alert_mute_rule_rls.sql"} {
		sql, err := os.ReadFile("../../migrations/" + file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	return pool
}

func alertMuteRuleWrite(name string, start, end time.Time, service string) domain.AlertMuteRuleWrite {
	return domain.AlertMuteRuleWrite{
		Name: name, TimeZone: "Asia/Colombo", StartOn: start, EndOn: end,
		Conditions: []domain.AlertMuteRuleCondition{{Field: "service", Value: service}, {Field: "environment", Value: "prod"}},
	}
}

func TestAlertMuteRuleIntegration_LifecycleAndGuards(t *testing.T) {
	pool := alertMuteRuleTestPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAlertMuteRuleRepository(repository.NewScoped(pool))
	now := time.Now().UTC().Truncate(time.Microsecond)

	scheduled, err := repo.Create(ctx, "a@wso2.com", alertMuteRuleWrite("Later", now.Add(time.Hour), now.Add(2*time.Hour), "svc-later"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if scheduled.Number != "AMR0000001" || scheduled.CreatedBy != "a@wso2.com" || scheduled.UpdatedBy != "a@wso2.com" || scheduled.EndedOn != nil {
		t.Fatalf("created %+v", scheduled)
	}
	if len(scheduled.Conditions) != 2 || scheduled.Conditions[0] != (domain.AlertMuteRuleCondition{Field: "service", Value: "svc-later"}) {
		t.Fatalf("conditions %+v", scheduled.Conditions)
	}
	active, err := repo.Create(ctx, "b@wso2.com", alertMuteRuleWrite("Now", now.Add(-time.Minute), now.Add(time.Hour), "svc-now"))
	if err != nil {
		t.Fatalf("create active: %v", err)
	}
	if active.Number != "AMR0000002" {
		t.Fatalf("second number %q", active.Number)
	}

	// The 72-hour limit and the window order are enforced by the table too.
	if _, err := repo.Create(ctx, "a@wso2.com", alertMuteRuleWrite("Too long", now, now.Add(73*time.Hour), "x")); err == nil {
		t.Fatal("created a rule longer than 72 hours")
	}

	got, err := repo.Active(ctx, now)
	if err != nil || len(got) != 1 || got[0].ID != active.ID {
		t.Fatalf("active = %+v, %v; want only %s", got, err, active.Number)
	}

	// Guards: an active rule cannot be edited or deleted, a scheduled one cannot be ended.
	if b, err := repo.Update(ctx, "c@wso2.com", active.ID, alertMuteRuleWrite("x", now.Add(time.Hour), now.Add(2*time.Hour), "x"), now); err != nil || b != nil {
		t.Fatalf("update of an active rule: %+v, %v", b, err)
	}
	if ok, err := repo.Delete(ctx, active.ID, now); err != nil || ok {
		t.Fatalf("delete of an active rule: %v, %v", ok, err)
	}
	if b, err := repo.End(ctx, "c@wso2.com", scheduled.ID, now); err != nil || b != nil {
		t.Fatalf("end of a scheduled rule: %+v, %v", b, err)
	}

	edited, err := repo.Update(ctx, "c@wso2.com", scheduled.ID, alertMuteRuleWrite("Later, renamed", now.Add(2*time.Hour), now.Add(3*time.Hour), "svc-x"), now)
	if err != nil || edited == nil || edited.Name != "Later, renamed" || edited.UpdatedBy != "c@wso2.com" || edited.CreatedBy != "a@wso2.com" {
		t.Fatalf("update: %+v, %v", edited, err)
	}

	// Extend: later only, and within 72 hours of the start.
	if b, err := repo.Extend(ctx, "d@wso2.com", active.ID, now.Add(30*time.Minute), now); err != nil || b != nil {
		t.Fatalf("extend to an earlier end: %+v, %v", b, err)
	}
	if b, err := repo.Extend(ctx, "d@wso2.com", active.ID, active.StartOn.Add(72*time.Hour+time.Second), now); err != nil || b != nil {
		t.Fatalf("extend past 72 hours: %+v, %v", b, err)
	}
	extended, err := repo.Extend(ctx, "d@wso2.com", active.ID, now.Add(5*time.Hour), now)
	if err != nil || extended == nil || !extended.EndOn.Equal(now.Add(5*time.Hour)) {
		t.Fatalf("extend: %+v, %v", extended, err)
	}

	ended, err := repo.End(ctx, "e@wso2.com", active.ID, now)
	if err != nil || ended == nil || ended.EndedBy == nil || *ended.EndedBy != "e@wso2.com" || ended.EndedOn == nil || !ended.EndedOn.Equal(now) {
		t.Fatalf("end: %+v, %v", ended, err)
	}
	if ended.StateAt(now) != domain.AlertMuteRuleStateEnded {
		t.Fatalf("state after end = %s", ended.StateAt(now))
	}
	if got, err := repo.Active(ctx, now); err != nil || len(got) != 0 {
		t.Fatalf("active after end = %+v, %v", got, err)
	}
	if b, err := repo.Extend(ctx, "d@wso2.com", active.ID, now.Add(6*time.Hour), now); err != nil || b != nil {
		t.Fatalf("extend of an ended rule: %+v, %v", b, err)
	}

	if ok, err := repo.Delete(ctx, scheduled.ID, now); err != nil || !ok {
		t.Fatalf("delete of a scheduled rule: %v, %v", ok, err)
	}
	if b, err := repo.Get(ctx, scheduled.ID); err != nil || b != nil {
		t.Fatalf("get after delete: %+v, %v", b, err)
	}
}

func TestAlertMuteRuleIntegration_SearchStatesAndQuery(t *testing.T) {
	pool := alertMuteRuleTestPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewAlertMuteRuleRepository(repository.NewScoped(pool))
	now := time.Now().UTC().Truncate(time.Microsecond)

	mk := func(name string, start, end time.Time, service string) domain.AlertMuteRule {
		b, err := repo.Create(ctx, "owner@wso2.com", alertMuteRuleWrite(name, start, end, service))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return b
	}
	scheduled := mk("Scheduled", now.Add(time.Hour), now.Add(2*time.Hour), "svc-a")
	mk("Active", now.Add(-time.Hour), now.Add(time.Hour), "svc-b")
	mk("Expired", now.Add(-3*time.Hour), now.Add(-2*time.Hour), "svc-c")
	endedEarly := mk("Ended", now.Add(-time.Hour), now.Add(time.Hour), "special_value%")
	if _, err := repo.End(ctx, "x@wso2.com", endedEarly.ID, now); err != nil {
		t.Fatalf("end: %v", err)
	}

	ids := func(list []domain.AlertMuteRule) []string {
		out := []string{}
		for _, b := range list {
			out = append(out, b.Name)
		}
		return out
	}
	for _, tc := range []struct {
		states []domain.AlertMuteRuleState
		query  string
		want   []string
	}{
		{nil, "", []string{"Ended", "Expired", "Active", "Scheduled"}},
		{[]domain.AlertMuteRuleState{domain.AlertMuteRuleStateActive}, "", []string{"Active"}},
		{[]domain.AlertMuteRuleState{domain.AlertMuteRuleStateScheduled}, "", []string{"Scheduled"}},
		{[]domain.AlertMuteRuleState{domain.AlertMuteRuleStateExpired}, "", []string{"Ended", "Expired"}},
		{[]domain.AlertMuteRuleState{domain.AlertMuteRuleStateActive, domain.AlertMuteRuleStateScheduled}, "", []string{"Active", "Scheduled"}},
		{nil, "SVC-B", []string{"Active"}},
		{nil, scheduled.Number, []string{"Scheduled"}},
		{nil, "OWNER@", []string{"Ended", "Expired", "Active", "Scheduled"}},
		// LIKE wildcards in the query are literal.
		{nil, "value%", []string{"Ended"}},
		{nil, "_", []string{"Ended"}},
	} {
		list, total, err := repo.Search(ctx, tc.states, tc.query, now, 20, 0)
		if err != nil {
			t.Fatalf("search %v %q: %v", tc.states, tc.query, err)
		}
		if got := ids(list); strings.Join(got, ",") != strings.Join(tc.want, ",") || total != len(tc.want) {
			t.Errorf("search %v %q = %v (total %d), want %v", tc.states, tc.query, got, total, tc.want)
		}
	}
	list, total, err := repo.Search(ctx, nil, "", now, 2, 1)
	if err != nil || total != 4 || strings.Join(ids(list), ",") != "Expired,Active" {
		t.Fatalf("paged search = %v (total %d), %v", ids(list), total, err)
	}

	var policies int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'alert_mute_rule' AND cmd = 'ALL'`).Scan(&policies); err != nil || policies != 1 {
		t.Fatalf("alert_mute_rule policies = %d, %v; want one FOR ALL policy", policies, err)
	}
}
