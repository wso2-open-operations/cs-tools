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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// seedRegistration adds one organisation registered for productCode at
// registeredOn, acknowledged when acknowledged is set.
func seedRegistration(t *testing.T, pool *pgxpool.Pool, name, productCode string, registeredOn time.Time, acknowledged bool) {
	t.Helper()
	ctx := context.Background()
	var personID, orgID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO plg_person (email) VALUES ($1) RETURNING id::TEXT`, name+"@trend.example.com").Scan(&personID); err != nil {
		t.Fatalf("seed a person: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO plg_organization (organization_name, created_on, registered_user)
		VALUES ($1, $2, $3::UUID) RETURNING id::TEXT`, name, registeredOn, personID).Scan(&orgID); err != nil {
		t.Fatalf("seed an organisation: %v", err)
	}
	var acknowledgedOn any
	if acknowledged {
		acknowledgedOn = registeredOn.Add(time.Hour)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO plg_org_platform (organization_id, product_id, registered_on, acknowledged_on)
		VALUES ($1::UUID, (SELECT id FROM plg_product WHERE code = $2), $3, $4)`,
		orgID, productCode, registeredOn, acknowledgedOn); err != nil {
		t.Fatalf("seed a registration: %v", err)
	}
}

// TestRegistrationVolumeTrendCountsUTCDays seeds registrations around a
// five-day window in March 2020 (away from every other test's NOW() rows)
// and reads them through a session in UTC+14, so a count by the session's
// day instead of the UTC day would move them.
func TestRegistrationVolumeTrendCountsUTCDays(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	utc := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for i, r := range []struct {
		product      string
		registeredOn string
		acknowledged bool
	}{
		{"IAM", "2020-02-29T23:59:59Z", false}, // the day before the window
		{"IAM", "2020-03-02T10:00:00Z", false},
		{"IAM", "2020-03-02T23:30:00Z", false},
		{"API_PLATFORM", "2020-03-03T00:30:00Z", false},
		{"API_PLATFORM", "2020-03-03T23:30:00-02:00", false}, // 2020-03-04 01:30 UTC
		{"IAM", "2020-03-04T12:00:00Z", true},
		{"IAM", "2020-03-05T23:59:59Z", false}, // the window's last second
		{"IAM", "2020-03-06T00:00:00Z", false}, // the day after the window
	} {
		seedRegistration(t, pool, fmt.Sprintf("Trend Org %d", i), r.product, utc(r.registeredOn), r.acknowledged)
	}

	cfg := pool.Config()
	cfg.ConnConfig.RuntimeParams["timezone"] = "Pacific/Kiritimati"
	farEast, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer farEast.Close()
	repo := NewOrgPlatformRepository(farEast, nil)

	start, end := utc("2020-03-01T00:00:00Z"), utc("2020-03-05T00:00:00Z")
	for _, c := range []struct {
		name string
		req  domain.SearchRegistrationsRequest
		want map[string]int
	}{
		{"every registration", domain.SearchRegistrationsRequest{},
			map[string]int{"2020-03-01": 0, "2020-03-02": 2, "2020-03-03": 1, "2020-03-04": 2, "2020-03-05": 1}},
		{"not yet acknowledged", domain.SearchRegistrationsRequest{UnacknowledgedOnly: true},
			map[string]int{"2020-03-01": 0, "2020-03-02": 2, "2020-03-03": 1, "2020-03-04": 1, "2020-03-05": 1}},
		{"one product", domain.SearchRegistrationsRequest{Filters: domain.OrganizationSearchFilters{ProductCodes: []string{"API_PLATFORM"}}},
			map[string]int{"2020-03-01": 0, "2020-03-02": 0, "2020-03-03": 1, "2020-03-04": 1, "2020-03-05": 0}},
	} {
		got, err := repo.RegistrationVolumeTrend(ctx, c.req, start, end)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	if _, err := repo.RegistrationVolumeTrend(ctx, domain.SearchRegistrationsRequest{}, end, start); err == nil {
		t.Error("an end before the start: got no error")
	}
}
