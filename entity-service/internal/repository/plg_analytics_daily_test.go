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

func TestRegistrationsOverTimeIsOneDailyTotalPerOrganisation(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	ingest := NewIngestRepository(pool)
	analytics := NewAnalyticsRepository(pool)
	const owner = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"

	today := time.Now().UTC().Truncate(24 * time.Hour)
	from, to := today.AddDate(0, 0, -3), time.Now().UTC()
	before, err := analytics.Dashboard(ctx, domain.AnalyticsRange{From: &from, To: &to})
	if err != nil {
		t.Fatalf("dashboard before: %v", err)
	}

	// Organisation A has two pairings, B one: three rows, two organisations.
	for company, products := range map[string][]string{
		"daily-company-a": {"identity-platform", "api-platform"},
		"daily-company-b": {"integration-platform"},
	} {
		for _, p := range products {
			if _, err := ingest.Register(ctx, domain.Registration{CompanyID: company, UserID: owner, InitiatingProduct: p}, nil); err != nil {
				t.Fatalf("%s %s: %v", company, p, err)
			}
		}
	}
	day := today.AddDate(0, 0, -2)
	if _, err := pool.Exec(ctx, `
		UPDATE plg_org_platform op SET registered_on = $1
		FROM plg_organization o
		WHERE o.id = op.organization_id AND o.moesif_company_id IN ('daily-company-a', 'daily-company-b')`,
		day.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}

	after, err := analytics.Dashboard(ctx, domain.AnalyticsRange{From: &from, To: &to})
	if err != nil {
		t.Fatalf("dashboard after: %v", err)
	}
	b, a := before.RegistrationsOverTime, after.RegistrationsOverTime
	if len(a.Series) != 1 || a.Series[0].Label != "Choreo Cloud Registrations" {
		t.Fatalf("series = %+v, want the one Choreo Cloud Registrations total", a.Series)
	}
	// Every day of the period is a point, the quiet ones included.
	if len(a.Buckets) != 4 {
		t.Fatalf("buckets = %v, want 4 days", a.Buckets)
	}
	for i, bucket := range a.Buckets {
		if want := from.AddDate(0, 0, i).Format("2006-01-02"); bucket != want {
			t.Errorf("bucket %d = %s, want %s", i, bucket, want)
		}
		delta := a.Series[0].Counts[i] - b.Series[0].Counts[i]
		want := 0
		if bucket == day.Format("2006-01-02") {
			want = 2 // each organisation once, whatever its pairings
		}
		if delta != want {
			t.Errorf("%s: %d new registrations, want %d", bucket, delta, want)
		}
	}
}

func TestCloudProductIsNamedChoreoCloud(t *testing.T) {
	pool := requireDB(t)
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT name FROM plg_product WHERE code = 'WSO2_CLOUD'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Choreo Cloud" {
		t.Errorf("WSO2_CLOUD is shown as %q, want Choreo Cloud", name)
	}
}
