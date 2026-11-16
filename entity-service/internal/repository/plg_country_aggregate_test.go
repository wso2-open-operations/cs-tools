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
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestAggregateByCountry seeds organisations registered in June 2018 (the
// registration filter keeps every other test's rows out), one registration
// each: three in Sri Lanka, written two ways ("Sri Lanka" twice, " sri lanka "
// once), one in India and one with no country of its own registered by the
// same person (so the view gives it India), one with no country and one
// with a blank one. One Sri Lanka registration is acknowledged.
func TestAggregateByCountry(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	june := time.Date(2018, 6, 10, 12, 0, 0, 0, time.UTC)
	country := func(name string, value any) {
		if _, err := pool.Exec(ctx, `UPDATE plg_organization SET country_name = $2 WHERE organization_name = $1`, name, value); err != nil {
			t.Fatalf("country of %s: %v", name, err)
		}
	}
	for i, o := range []struct {
		name, product string
		country       any
		acknowledged  bool
	}{
		{"Country Org A", "IAM", "Sri Lanka", true},
		{"Country Org B", "API_PLATFORM", " sri lanka ", false},
		{"Country Org C", "IAM", "Sri Lanka", false},
		{"Country Org D", "IAM", "India", false},
		{"Country Org F", "IAM", nil, false},
		{"Country Org G", "API_PLATFORM", "", false},
	} {
		seedRegistration(t, pool, o.name, o.product, june.AddDate(0, 0, i), o.acknowledged)
		country(o.name, o.country)
	}
	// Country Org E: India's registrant signs up a second organisation that
	// carries no country; plg_organization_v inherits India.
	var orgID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO plg_organization (organization_name, created_on, registered_user)
		SELECT 'Country Org E', $1, registered_user FROM plg_organization WHERE organization_name = 'Country Org D'
		RETURNING id::TEXT`, june.AddDate(0, 0, 7)).Scan(&orgID); err != nil {
		t.Fatalf("seed Country Org E: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO plg_org_platform (organization_id, product_id, registered_on)
		VALUES ($1::UUID, (SELECT id FROM plg_product WHERE code = 'IAM'), $2)`, orgID, june.AddDate(0, 0, 7)); err != nil {
		t.Fatalf("seed Country Org E's registration: %v", err)
	}

	from, to := time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2018, 6, 30, 23, 59, 59, 0, time.UTC)
	inJune := func(f domain.OrganizationSearchFilters) domain.OrganizationSearchFilters {
		f.RegisteredFrom, f.RegisteredTo = &from, &to
		return f
	}
	yes := true
	everyCountry := []domain.AggregateBucket{
		{Key: "Sri Lanka", Label: "Sri Lanka", Count: 3}, {Key: "India", Label: "India", Count: 2}, {Key: "unknown", Label: "Unknown", Count: 2}}

	orgs := NewOrganizationRepository(pool)
	regs := NewOrgPlatformRepository(pool, nil)
	for _, c := range []struct {
		name               string
		filters            domain.OrganizationSearchFilters
		unacknowledgedOnly bool
		maxGroups          int
		organizations      domain.AggregateResponse
		registrations      domain.AggregateResponse
	}{
		{name: "every country, largest first, ties by name", filters: inJune(domain.OrganizationSearchFilters{}),
			organizations: domain.AggregateResponse{TotalRecords: 7, Groups: everyCountry},
			registrations: domain.AggregateResponse{TotalRecords: 7, Groups: everyCountry}},
		{name: "the rest folded into others", filters: inJune(domain.OrganizationSearchFilters{}), maxGroups: 1,
			organizations: domain.AggregateResponse{TotalRecords: 7, OthersCount: 4, Groups: everyCountry[:1]},
			registrations: domain.AggregateResponse{TotalRecords: 7, OthersCount: 4, Groups: everyCountry[:1]}},
		// Ties by label as written: "Unknown" sorts before "sri lanka".
		{name: "the search's filters apply", filters: inJune(domain.OrganizationSearchFilters{ProductCodes: []string{"API_PLATFORM"}}),
			organizations: domain.AggregateResponse{TotalRecords: 2, Groups: []domain.AggregateBucket{
				{Key: "unknown", Label: "Unknown", Count: 1}, {Key: "sri lanka", Label: "sri lanka", Count: 1}}},
			registrations: domain.AggregateResponse{TotalRecords: 2, Groups: []domain.AggregateBucket{
				{Key: "unknown", Label: "Unknown", Count: 1}, {Key: "sri lanka", Label: "sri lanka", Count: 1}}}},
		{name: "unacknowledged registrations only", filters: inJune(domain.OrganizationSearchFilters{}), unacknowledgedOnly: true,
			organizations: domain.AggregateResponse{TotalRecords: 7, Groups: everyCountry},
			registrations: domain.AggregateResponse{TotalRecords: 6, Groups: []domain.AggregateBucket{
				{Key: "India", Label: "India", Count: 2}, {Key: "Sri Lanka", Label: "Sri Lanka", Count: 2}, {Key: "unknown", Label: "Unknown", Count: 2}}}},
		{name: "a country, any case", filters: inJune(domain.OrganizationSearchFilters{CountryNames: []string{"SRI LANKA ", "Nepal"}}),
			organizations: domain.AggregateResponse{TotalRecords: 3, Groups: everyCountry[:1]},
			registrations: domain.AggregateResponse{TotalRecords: 3, Groups: everyCountry[:1]}},
		{name: "no country", filters: inJune(domain.OrganizationSearchFilters{CountryUnknown: &yes}),
			organizations: domain.AggregateResponse{TotalRecords: 2, Groups: everyCountry[2:]},
			registrations: domain.AggregateResponse{TotalRecords: 2, Groups: everyCountry[2:]}},
	} {
		got, err := orgs.AggregateByCountry(ctx, c.filters, c.maxGroups)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.organizations) {
			t.Errorf("%s, organisations:\n got %+v\nwant %+v", c.name, got, c.organizations)
		}
		got, err = regs.AggregateRegistrationsByCountry(ctx, domain.SearchRegistrationsRequest{UnacknowledgedOnly: c.unacknowledgedOnly, Filters: c.filters}, c.maxGroups)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.registrations) {
			t.Errorf("%s, registrations:\n got %+v\nwant %+v", c.name, got, c.registrations)
		}
	}

	// A group's click lists what the group counted: the searches and the
	// trend take the same country filters.
	for _, c := range []struct {
		name    string
		filters domain.OrganizationSearchFilters
		want    int
	}{
		{"Sri Lanka", domain.OrganizationSearchFilters{CountryNames: []string{"Sri Lanka"}}, 3},
		{"India, inherited", domain.OrganizationSearchFilters{CountryNames: []string{"india"}}, 2},
		{"Unknown", domain.OrganizationSearchFilters{CountryUnknown: &yes}, 2},
	} {
		f := inJune(c.filters)
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
		if orgTotal != c.want || regTotal != c.want || trendTotal != c.want {
			t.Errorf("%s: organisations %d, registrations %d, trend %d, want %d each", c.name, orgTotal, regTotal, trendTotal, c.want)
		}
	}
}
