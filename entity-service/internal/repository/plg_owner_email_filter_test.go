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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestOwnerEmailFilter seeds, in March 2017: Org A (IAM and API Platform)
// owned by Carol, whose email is stored in mixed case, Org B (IAM) owned by
// Dave and Org C (IAM) with no owner.
func TestOwnerEmailFilter(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	ownerID := func(userName, email string) string {
		if err := seedUser(ctx, pool, userName, email, "INTERNAL", true); err != nil {
			t.Fatalf("seed %s: %v", userName, err)
		}
		var id string
		if err := pool.QueryRow(ctx, `SELECT id::TEXT FROM "user" WHERE user_name = $1`, userName).Scan(&id); err != nil {
			t.Fatalf("read %s: %v", userName, err)
		}
		return id
	}
	carol := ownerID("carol.emailowner", "Carol.EmailOwner@Example.com")
	dave := ownerID("dave.emailowner", "dave.emailowner@example.com")

	var seeded bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM plg_organization WHERE organization_name = 'Owner Email Org A')`).Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	march := time.Date(2017, 3, 10, 12, 0, 0, 0, time.UTC)
	if !seeded {
		for i, o := range []struct{ name, owner string }{
			{"Owner Email Org A", carol}, {"Owner Email Org B", dave}, {"Owner Email Org C", ""},
		} {
			seedRegistration(t, pool, o.name, "IAM", march.AddDate(0, 0, i), false)
			if o.owner != "" {
				if _, err := pool.Exec(ctx, `UPDATE plg_organization SET plg_cs_owner = $2::UUID WHERE organization_name = $1`, o.name, o.owner); err != nil {
					t.Fatalf("own %s: %v", o.name, err)
				}
			}
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO plg_org_platform (organization_id, product_id, registered_on)
			SELECT id, (SELECT id FROM plg_product WHERE code = 'API_PLATFORM'), $1
			FROM plg_organization WHERE organization_name = 'Owner Email Org A'`, march.AddDate(0, 0, 5)); err != nil {
			t.Fatalf("seed Org A's API Platform registration: %v", err)
		}
	}

	from, to := time.Date(2017, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2017, 3, 31, 23, 59, 59, 0, time.UTC)
	orgs := NewOrganizationRepository(pool)
	regs := NewOrgPlatformRepository(pool, nil)
	for _, c := range []struct {
		name                 string
		filters              domain.OrganizationSearchFilters
		organizations, count int // count: registrations, and the trend's total
		owners               map[string]int
	}{
		{"an owner, ignoring case and spaces", domain.OrganizationSearchFilters{OwnerEmails: []string{" carol.emailowner@example.COM "}}, 1, 2, map[string]int{carol: 1}},
		{"any of two owners", domain.OrganizationSearchFilters{OwnerEmails: []string{"carol.emailowner@example.com", "dave.emailowner@example.com"}}, 2, 3, map[string]int{carol: 1, dave: 1}},
		{"an email nobody owns with", domain.OrganizationSearchFilters{OwnerEmails: []string{"nobody@example.com"}}, 0, 0, map[string]int{}},
		{"with the other filters", domain.OrganizationSearchFilters{OwnerEmails: []string{"carol.emailowner@example.com"}, ProductCodes: []string{"API_PLATFORM"}}, 1, 1, map[string]int{carol: 1}},
		{"no owner email filter", domain.OrganizationSearchFilters{}, 3, 4, map[string]int{carol: 1, dave: 1, "unowned": 1}},
	} {
		f := c.filters
		f.RegisteredFrom, f.RegisteredTo = &from, &to
		_, orgTotal, err := orgs.Search(ctx, domain.SearchOrganizationsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_, regTotal, err := regs.SearchRegistrations(ctx, domain.SearchRegistrationsRequest{Filters: f, Pagination: domain.Pagination{Limit: 50}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		days, err := regs.RegistrationVolumeTrend(ctx, domain.SearchRegistrationsRequest{Filters: c.filters}, from, to)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		trendTotal := 0
		for _, n := range days {
			trendTotal += n
		}
		if orgTotal != c.organizations || regTotal != c.count || trendTotal != c.count {
			t.Errorf("%s: organisations %d, registrations %d, trend %d; want %d, %d, %d",
				c.name, orgTotal, regTotal, trendTotal, c.organizations, c.count, c.count)
		}

		// The aggregates count what the searches list, each owner as one group.
		byOwner, err := orgs.AggregateByOwner(ctx, f, 0)
		if err != nil {
			t.Fatalf("%s, by owner: %v", c.name, err)
		}
		owners := map[string]int{}
		for _, g := range byOwner.Groups {
			owners[g.Key] = g.Count
		}
		if len(owners) != len(c.owners) {
			t.Errorf("%s: owner groups %v, want %v", c.name, owners, c.owners)
		}
		for k, n := range c.owners {
			if owners[k] != n {
				t.Errorf("%s: owner groups %v, want %v", c.name, owners, c.owners)
				break
			}
		}
		byCountry, err := regs.AggregateRegistrationsByCountry(ctx, domain.SearchRegistrationsRequest{Filters: f}, 0)
		if err != nil {
			t.Fatalf("%s, by country: %v", c.name, err)
		}
		if byOwner.TotalRecords != c.organizations || byCountry.TotalRecords != c.count {
			t.Errorf("%s: aggregates %d organisations, %d registrations; want %d, %d",
				c.name, byOwner.TotalRecords, byCountry.TotalRecords, c.organizations, c.count)
		}
	}
}
