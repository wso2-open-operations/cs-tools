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

package service

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestNewConfigurationItemSearch(t *testing.T) {
	t.Run("no query lists everything in name order", func(t *testing.T) {
		for _, q := range []string{"", "   "} {
			got := newConfigurationItemSearch(q)
			if got.where != "" || got.whereArgs != nil || got.orderArgs != nil {
				t.Fatalf("query %q: want no filter, got %+v", q, got)
			}
			if got.orderBy != " ORDER BY so.name, so.id" {
				t.Fatalf("query %q: orderBy = %q", q, got.orderBy)
			}
		}
	})

	t.Run("every word must match, offering or parent service", func(t *testing.T) {
		got := newConfigurationItemSearch("  Integration   DP automation ")
		wantArgs := []any{"%Integration%", "%DP%", "%automation%"}
		if !reflect.DeepEqual(got.whereArgs, wantArgs) {
			t.Fatalf("whereArgs = %v, want %v", got.whereArgs, wantArgs)
		}
		for i := 1; i <= 3; i++ {
			cond := "(COALESCE(so.name, '') || ' ' || COALESCE(s.name, '')) ILIKE $" + string(rune('0'+i)) + ` ESCAPE '\'`
			if !strings.Contains(got.where, cond) {
				t.Fatalf("where %q lacks %q", got.where, cond)
			}
		}
		if strings.Count(got.where, " AND ") != 2 || !strings.HasPrefix(got.where, " WHERE ") {
			t.Fatalf("where = %q, want three conditions joined by AND", got.where)
		}
		// The ranking takes the words as one phrase, spacing normalised, and
		// numbers its arguments after the WHERE clause's.
		wantOrder := []any{"Integration DP automation%", "%Integration DP automation%"}
		if !reflect.DeepEqual(got.orderArgs, wantOrder) {
			t.Fatalf("orderArgs = %v, want %v", got.orderArgs, wantOrder)
		}
		if !strings.Contains(got.orderBy, "ILIKE $4") || !strings.Contains(got.orderBy, "ILIKE $5") {
			t.Fatalf("orderBy = %q, want $4 and $5", got.orderBy)
		}
	})

	t.Run("LIKE wildcards in a word match themselves", func(t *testing.T) {
		got := newConfigurationItemSearch(`100%_up\down`)
		want := []any{`%100\%\_up\\down%`}
		if !reflect.DeepEqual(got.whereArgs, want) {
			t.Fatalf("whereArgs = %v, want %v", got.whereArgs, want)
		}
	})

	t.Run("at most maxSearchTerms words become conditions", func(t *testing.T) {
		got := newConfigurationItemSearch(strings.Repeat("word ", maxSearchTerms+5))
		if len(got.whereArgs) != maxSearchTerms {
			t.Fatalf("%d conditions, want %d", len(got.whereArgs), maxSearchTerms)
		}
	})

	t.Run("ranking uses every word, past the filter's cap", func(t *testing.T) {
		full := "Integration CP Automation Creation and Build Service Offering - US"
		got := newConfigurationItemSearch(full)
		if len(got.whereArgs) != maxSearchTerms {
			t.Fatalf("%d conditions, want %d", len(got.whereArgs), maxSearchTerms)
		}
		want := []any{full + "%", "%" + full + "%"}
		if !reflect.DeepEqual(got.orderArgs, want) {
			t.Fatalf("orderArgs = %v, want %v", got.orderArgs, want)
		}
	})
}

// TestPGConfigurationItemSearchLive runs the picker's query against a real
// PostgreSQL, over TEMPORARY service and service_offering tables that shadow
// the real ones on this connection only: it never reads or writes real rows.
// The pool holds one connection so every statement sees those tables.
//
// Skipped unless PG_CI_SEARCH_TEST_DSN is set, so an ordinary go test never
// opens a connection.
func TestPGConfigurationItemSearchLive(t *testing.T) {
	dsn := os.Getenv("PG_CI_SEARCH_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_CI_SEARCH_TEST_DSN not set; this test needs a real database")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns, cfg.MinConns = 1, 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	setup := []string{
		`CREATE TEMPORARY TABLE service (id uuid PRIMARY KEY, name text)`,
		`CREATE TEMPORARY TABLE service_offering (id uuid PRIMARY KEY, name text, parent_id uuid)`,
		`INSERT INTO service VALUES
		   ('00000000-0000-0000-0000-000000000001', 'Integration Control Plane Service - US'),
		   ('00000000-0000-0000-0000-000000000002', 'Integration Data Plane Service - EU'),
		   ('00000000-0000-0000-0000-000000000003', 'Devant Control Plane Service - US'),
		   ('00000000-0000-0000-0000-000000000004', 'Integration Control Plane Service - EU')`,
		`INSERT INTO service_offering VALUES
		   ('10000000-0000-0000-0000-000000000001', 'Integration CP Automation Creation and Build Service Offering - US', '00000000-0000-0000-0000-000000000001'),
		   ('10000000-0000-0000-0000-000000000002', 'Integration DP Automation Execution Service Offering - EU', '00000000-0000-0000-0000-000000000002'),
		   ('10000000-0000-0000-0000-000000000003', 'Devant CP Integration Automation Service Offering - US', '00000000-0000-0000-0000-000000000003'),
		   ('10000000-0000-0000-0000-000000000004', 'Devant CP 100%_Uptime Service Offering - US', '00000000-0000-0000-0000-000000000003'),
		   ('10000000-0000-0000-0000-000000000005', 'Orphan Offering', NULL),
		   ('10000000-0000-0000-0000-000000000006', 'Integration CP Automation Creation and Build Service Offering - EU', '00000000-0000-0000-0000-000000000004'),
		   ('10000000-0000-0000-0000-000000000007', NULL, '00000000-0000-0000-0000-000000000002')`,
	}
	const unnamed = "<no name>"
	for _, q := range setup {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("setup %q: %v", q, err)
		}
	}

	svc := NewConfigurationItemService(pool)
	search := func(q string) ([]string, int) {
		t.Helper()
		resp, err := svc.SearchConfigurationItems(ctx, domain.SearchConfigurationItemsRequest{
			Filters:    &domain.SearchConfigurationItemsFilters{SearchQuery: q},
			Pagination: domain.Pagination{Limit: 20},
		})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		names := make([]string, 0, len(resp.ConfigurationItems))
		for _, ci := range resp.ConfigurationItems {
			if ci.Name == nil {
				names = append(names, unnamed)
				continue
			}
			names = append(names, *ci.Name)
		}
		return names, resp.Total
	}

	cases := []struct {
		query string
		want  []string
	}{
		// Words in any order; "devant" offerings that mention Integration rank
		// after the ones whose names start with the query.
		{"automation integration", []string{
			"Devant CP Integration Automation Service Offering - US",
			"Integration CP Automation Creation and Build Service Offering - EU",
			"Integration CP Automation Creation and Build Service Offering - US",
			"Integration DP Automation Execution Service Offering - EU",
		}},
		{"Integration DP", []string{
			"Integration DP Automation Execution Service Offering - EU",
		}},
		// "Data Plane" is only in the parent service's name.
		{"automation data plane", []string{
			"Integration DP Automation Execution Service Offering - EU",
		}},
		// A name that starts with the query comes before one that only contains
		// it; an offering with no name, found through its parent, comes last.
		{"Integration", []string{
			"Integration CP Automation Creation and Build Service Offering - EU",
			"Integration CP Automation Creation and Build Service Offering - US",
			"Integration DP Automation Execution Service Offering - EU",
			"Devant CP Integration Automation Service Offering - US",
			unnamed,
		}},
		// Ten words: the filter takes eight, which both twins match, and the
		// ranking takes all ten, which only the US one starts with.
		{"Integration CP Automation Creation and Build Service Offering - US", []string{
			"Integration CP Automation Creation and Build Service Offering - US",
			"Integration CP Automation Creation and Build Service Offering - EU",
		}},
		// An offering with no name still matches through its parent service.
		{"data plane eu", []string{
			"Integration DP Automation Execution Service Offering - EU",
			unnamed,
		}},
		{"100%_", []string{"Devant CP 100%_Uptime Service Offering - US"}},
		{"100%x", []string{}},
		{"orphan", []string{"Orphan Offering"}},
	}
	for _, c := range cases {
		got, total := search(c.query)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("search %q:\n got  %q\n want %q", c.query, got, c.want)
		}
		if total != len(c.want) {
			t.Errorf("search %q: total %d, want %d", c.query, total, len(c.want))
		}
	}
	if all, total := search(""); total != 7 || len(all) != 7 {
		t.Errorf("empty query: %d items, total %d, want 7 and 7", len(all), total)
	}
}
