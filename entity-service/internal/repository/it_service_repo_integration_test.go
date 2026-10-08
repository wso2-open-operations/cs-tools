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

// This is an integration test: it runs ITServiceRepository.SearchITServices
// against a live PostgreSQL instance, regression-testing the Create Incident
// "Service" picker, which showed the newest 20 matches and so could never reach
// an old service ("Choreo") whose name is also the prefix of many newer
// "Choreo EU - ... Service Offering" records. It checks the parts only a real
// database can: the parameter numbering of the ranked ORDER BY, the LIKE escape
// character, and the order of the pages.
//
// It builds a private schema holding just the two tables the query reads
// (service and "group", with the columns of migrations 0044 and 0075) and drops
// it afterwards, so it needs no real data and touches none. Skipped without a
// DSN, like the other integration tests here:
//
//	IT_SERVICE_TEST_DSN=postgres://... go test ./internal/repository/ -run ITServiceSearchRanking

package repository_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const itServiceTestGroupID = "5a000000-0000-4000-8000-000000000001"

// itServiceTestPool creates a private schema with the two tables and returns a
// pool whose search_path is that schema. The schema is dropped on cleanup.
func itServiceTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("IT_SERVICE_TEST_DSN")
	if dsn == "" {
		t.Skip("IT_SERVICE_TEST_DSN not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := fmt.Sprintf("it_service_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	for _, ddl := range []string{
		`CREATE TABLE ` + schema + `."group" (id UUID PRIMARY KEY, name VARCHAR(255))`,
		`CREATE TABLE ` + schema + `.service (
			id UUID PRIMARY KEY,
			created_on TIMESTAMPTZ NOT NULL,
			name VARCHAR(255) NOT NULL,
			number VARCHAR(100) NOT NULL,
			category VARCHAR(100),
			business_criticality VARCHAR(30),
			support_group_id UUID REFERENCES ` + schema + `."group"(id))`,
	} {
		if _, err := admin.Exec(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect with search_path: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedITServices inserts the services oldest first, one minute apart, so the
// last name in the list is the newest and, without ranking, comes back first.
func seedITServices(t *testing.T, pool *pgxpool.Pool, names []string, groupOf map[string]bool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO "group" (id, name) VALUES ($1, 'Artemis SRE Group')`, itServiceTestGroupID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	base := time.Now().Add(-time.Duration(len(names)) * time.Minute)
	for i, name := range names {
		var group any
		if groupOf[name] {
			group = itServiceTestGroupID
		}
		_, err := pool.Exec(ctx,
			`INSERT INTO service (id, created_on, name, number, support_group_id)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4::uuid)`,
			base.Add(time.Duration(i)*time.Minute), name, fmt.Sprintf("SVC%07d", i), group)
		if err != nil {
			t.Fatalf("seed service %q: %v", name, err)
		}
	}
}

func searchServiceNames(t *testing.T, repo repository.ITServiceRepository, query string, limit, offset int) ([]string, int) {
	t.Helper()
	services, total, err := repo.SearchITServices(context.Background(), query, limit, offset)
	if err != nil {
		t.Fatalf("SearchITServices(%q, %d, %d): %v", query, limit, offset, err)
	}
	names := make([]string, len(services))
	for i, s := range services {
		if s.Name != nil {
			names[i] = *s.Name
		}
	}
	return names, total
}

// The reported failure, against a real database: the plain "Choreo" service is
// the oldest of the matches, so the newest-first page of 20 never contained it.
func TestITServiceSearchRankingPutsTheExactNameFirst(t *testing.T) {
	pool := itServiceTestPool(t)

	// Oldest first: "Choreo" is the oldest, then two services that start with
	// it, then 30 newer ones that start with it too, then the services that only
	// contain "choreo".
	names := []string{"Choreo", "Choreo Control Plane Service - US", "Choreo Data Plane Service - EU", "Unrelated Billing"}
	for i := 0; i < 30; i++ {
		names = append(names, fmt.Sprintf("Choreo EU - DP Thing %02d Service", i))
	}
	names = append(names, "client-btchoreo-alert-integration", "client-uoechoreosub-alert-integration")
	seedITServices(t, pool, names, map[string]bool{"Choreo": true})

	repo := repository.NewITServiceRepository(pool)

	for _, query := range []string{"Choreo", "choreo", "CHOREO", "  choreo "} {
		got, total := searchServiceNames(t, repo, query, 20, 0)
		if total != len(names)-1 { // everything but "Unrelated Billing"
			t.Errorf("query %q: total = %d, want %d", query, total, len(names)-1)
		}
		if len(got) != 20 || got[0] != "Choreo" {
			t.Errorf("query %q: page = %q, want 20 rows starting with the exact match", query, got)
		}
	}

	// Pages continue one ordering: exact, then prefix matches (newest first),
	// then the names that only contain the query.
	first, _ := searchServiceNames(t, repo, "choreo", 20, 0)
	second, _ := searchServiceNames(t, repo, "choreo", 20, 20)
	third, _ := searchServiceNames(t, repo, "choreo", 20, 40)
	all := append(append(first, second...), third...)
	if len(all) != len(names)-1 {
		t.Fatalf("pages hold %d rows in all, want %d", len(all), len(names)-1)
	}
	seen := map[string]bool{}
	for _, n := range all {
		if seen[n] {
			t.Errorf("%q is on two pages", n)
		}
		seen[n] = true
	}
	for _, n := range all[len(all)-2:] {
		if !strings.HasPrefix(n, "client-") {
			t.Errorf("the last rows should be the names that only contain the query, got %q", n)
		}
	}
	if all[1] != "Choreo EU - DP Thing 29 Service" || all[2] != "Choreo EU - DP Thing 28 Service" {
		t.Errorf("prefix matches should follow, newest first, got %q then %q", all[1], all[2])
	}
	if all[len(all)-3] != "Choreo Control Plane Service - US" {
		t.Errorf("the oldest prefix match should be the last of them, got %q", all[len(all)-3])
	}
}

// The support group is still joined on the ranked query, since the picker uses
// it to show which group an incident will be assigned to.
func TestITServiceSearchRankingKeepsTheSupportGroup(t *testing.T) {
	pool := itServiceTestPool(t)
	seedITServices(t, pool, []string{"Choreo", "Choreo EU - Plane"}, map[string]bool{"Choreo": true})

	services, _, err := repository.NewITServiceRepository(pool).SearchITServices(context.Background(), "choreo", 20, 0)
	if err != nil {
		t.Fatalf("SearchITServices: %v", err)
	}
	if len(services) != 2 || services[0].Name == nil || *services[0].Name != "Choreo" {
		t.Fatalf("services = %+v, want Choreo first", services)
	}
	if g := services[0].SupportGroup; g == nil || g.Name != "Artemis SRE Group" || g.ID != itServiceTestGroupID {
		t.Errorf("Choreo's support group = %+v, want Artemis SRE Group", g)
	}
	if services[1].SupportGroup != nil {
		t.Errorf("Choreo EU - Plane has no support group, got %+v", services[1].SupportGroup)
	}
}

// LIKE metacharacters in the query are literal in the match AND in the ranking.
// Each pair below is seeded oldest first with the genuine prefix match older than
// the row that only looks like one when the metacharacter is read as a wildcard:
// with unescaped rank patterns the newer look-alike would sort first.
func TestITServiceSearchRankingTreatsWildcardsLiterally(t *testing.T) {
	pool := itServiceTestPool(t)
	seedITServices(t, pool, []string{
		"a_b real prefix", "axb and a_b", "axb decoy", // underscore: one wildcard character
		"100% uptime", "1000 and 100%", "1000 decoy", // percent: any run of characters
		`back\slash real prefix`, `xback\slash tail`, "backxslash decoy", // backslash: the escape character itself
	}, nil)
	repo := repository.NewITServiceRepository(pool)

	cases := []struct {
		query string
		want  []string
	}{
		{"a_b", []string{"a_b real prefix", "axb and a_b"}},
		{"100%", []string{"100% uptime", "1000 and 100%"}},
		{`back\slash`, []string{`back\slash real prefix`, `xback\slash tail`}},
	}
	for _, tc := range cases {
		got, total := searchServiceNames(t, repo, tc.query, 20, 0)
		if total != len(tc.want) || strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("query %q: got %q (total %d), want %q in that order", tc.query, got, total, tc.want)
		}
	}
}

// A word that starts with the query ranks the same whether a space or a hyphen
// precedes it, and both rank ahead of a query buried inside a word, however old.
func TestITServiceSearchRankingWordPrefixTier(t *testing.T) {
	pool := itServiceTestPool(t)
	seedITServices(t, pool, []string{"my-choreo-thing", "my choreo thing", "xchoreox buried"}, nil)

	got, total := searchServiceNames(t, repository.NewITServiceRepository(pool), "choreo", 20, 0)
	want := "my choreo thing|my-choreo-thing|xchoreox buried" // equal tier keeps newest first
	if total != 3 || strings.Join(got, "|") != want {
		t.Errorf("got %q (total %d), want %q", got, total, want)
	}
}

// With nothing typed the page is still the newest services first, and the count
// query (which takes no ranking parameters) still runs.
func TestITServiceSearchWithoutAQueryIsNewestFirst(t *testing.T) {
	pool := itServiceTestPool(t)
	seedITServices(t, pool, []string{"oldest", "middle", "newest"}, nil)

	got, total := searchServiceNames(t, repository.NewITServiceRepository(pool), "", 20, 0)
	if total != 3 || strings.Join(got, ",") != "newest,middle,oldest" {
		t.Errorf("got %q (total %d), want newest,middle,oldest", got, total)
	}
}

// A match on the service number alone ranks last but is still returned.
func TestITServiceSearchStillMatchesTheNumber(t *testing.T) {
	pool := itServiceTestPool(t)
	seedITServices(t, pool, []string{"Alpha", "Beta"}, nil)

	got, total := searchServiceNames(t, repository.NewITServiceRepository(pool), "SVC0000001", 20, 0)
	if total != 1 || len(got) != 1 || got[0] != "Beta" {
		t.Errorf("number search = %q (total %d), want Beta", got, total)
	}
}
