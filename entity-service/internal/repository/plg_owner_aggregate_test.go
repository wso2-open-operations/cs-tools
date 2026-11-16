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

// TestAggregateOrganizationsByOwner seeds four organisations registered in
// June 2019 (the registration filter keeps every other test's rows out):
// two owned by Alice, one by Bob, one with no owner.
func TestAggregateOrganizationsByOwner(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	ownerID := func(userName, first, last string) string {
		if err := seedUser(ctx, pool, userName, userName+"@owner.example.com", "INTERNAL", true); err != nil {
			t.Fatalf("seed %s: %v", userName, err)
		}
		var id string
		if err := pool.QueryRow(ctx, `UPDATE "user" SET first_name = $2, last_name = $3 WHERE user_name = $1 RETURNING id::TEXT`,
			userName, first, last).Scan(&id); err != nil {
			t.Fatalf("name %s: %v", userName, err)
		}
		return id
	}
	alice, bob := ownerID("alice.owner", "Alice", "Owner"), ownerID("bob.owner", "Bob", "Owner")

	june := time.Date(2019, 6, 10, 12, 0, 0, 0, time.UTC)
	for i, o := range []struct{ product, owner string }{
		{"IAM", alice}, {"API_PLATFORM", alice}, {"IAM", bob}, {"IAM", ""},
	} {
		name := "Owner Org " + string(rune('A'+i))
		seedRegistration(t, pool, name, o.product, june.AddDate(0, 0, i), false)
		if o.owner != "" {
			if _, err := pool.Exec(ctx, `UPDATE plg_organization SET plg_cs_owner = $2::UUID WHERE organization_name = $1`, name, o.owner); err != nil {
				t.Fatalf("own %s: %v", name, err)
			}
		}
	}

	from, to := time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, 6, 30, 23, 59, 59, 0, time.UTC)
	inJune := func(f domain.OrganizationSearchFilters) domain.OrganizationSearchFilters {
		f.RegisteredFrom, f.RegisteredTo = &from, &to
		return f
	}
	yes := true
	repo := NewOrganizationRepository(pool)
	for _, c := range []struct {
		name      string
		filters   domain.OrganizationSearchFilters
		maxGroups int
		want      domain.AggregateResponse
	}{
		{"every owner, largest first, ties by name", inJune(domain.OrganizationSearchFilters{}), 0,
			domain.AggregateResponse{TotalRecords: 4, Groups: []domain.AggregateBucket{
				{Key: alice, Label: "Alice Owner", Count: 2}, {Key: bob, Label: "Bob Owner", Count: 1}, {Key: "unowned", Label: "Unowned", Count: 1}}}},
		{"the rest folded into others", inJune(domain.OrganizationSearchFilters{}), 1,
			domain.AggregateResponse{TotalRecords: 4, OthersCount: 2, Groups: []domain.AggregateBucket{
				{Key: alice, Label: "Alice Owner", Count: 2}}}},
		{"the search's filters apply", inJune(domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}}), 0,
			domain.AggregateResponse{TotalRecords: 3, Groups: []domain.AggregateBucket{
				{Key: alice, Label: "Alice Owner", Count: 1}, {Key: bob, Label: "Bob Owner", Count: 1}, {Key: "unowned", Label: "Unowned", Count: 1}}}},
		{"unowned only", inJune(domain.OrganizationSearchFilters{Unowned: &yes}), 0,
			domain.AggregateResponse{TotalRecords: 1, Groups: []domain.AggregateBucket{{Key: "unowned", Label: "Unowned", Count: 1}}}},
	} {
		got, err := repo.AggregateByOwner(ctx, c.filters, c.maxGroups)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}
