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

package service

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Runs against a migrated database (0178 included); skipped unless ENTITY_TEST_DATABASE_URL is set.
const (
	itAccountLive    = "a110c000-0000-4000-8000-000000000001"
	itAccountDeleted = "a110c000-0000-4000-8000-000000000002"
	itUser           = "a110c000-0000-4000-8000-000000000011"
	itLineEngagement = "a110c000-0000-4000-8000-000000000021"
	itLineItemRow    = "a110c000-0000-4000-8000-000000000031"
	itAccountSfID    = "001ITESTALLOC0001A"
	itLineItemSfID   = "00kITESTALLOC0001A"
)

func newAllocationIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	clean := func() {
		for _, stmt := range []string{
			`DELETE FROM customer_engagement WHERE engagement_id LIKE 'EIT%' OR id = '` + itLineEngagement + `'`,
			`DELETE FROM sf_opportunity_product WHERE id = '` + itLineItemRow + `'`,
			`DELETE FROM account WHERE id IN ('` + itAccountLive + `', '` + itAccountDeleted + `')`,
			`DELETE FROM "user" WHERE id = '` + itUser + `'`,
		} {
			if _, err := pool.Exec(ctx, stmt); err != nil {
				t.Fatalf("clean: %v", err)
			}
		}
	}
	clean()
	t.Cleanup(clean)
	for _, stmt := range []string{
		`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, deleted_on) VALUES
			('` + itAccountDeleted + `', now() - interval '1 day', now(), 't', 't', 'Alloc ITest Co', 'ACC-ALLOC-IT-2', '` + itAccountSfID + `', now()),
			('` + itAccountLive + `', now(), now(), 't', 't', 'Alloc ITest Co Live', 'ACC-ALLOC-IT-1', '` + itAccountSfID + `', NULL)`,
		`INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES
			('` + itUser + `', now(), now(), 'alloc.itest', 'Alloc-ITest@wso2.com')`,
		`INSERT INTO sf_opportunity_product (id, created_on, updated_on, created_by, updated_by, line_item_sf_id) VALUES
			('` + itLineItemRow + `', now(), now(), 't', 't', '` + itLineItemSfID + `')`,
		`INSERT INTO customer_engagement (id, created_on, updated_on, name, line_item_id_ref) VALUES
			('` + itLineEngagement + `', now(), now(), 'Line engagement', replace('` + itLineItemRow + `', '-', ''))`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return pool
}

func itCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestAllocationEventIntegration(t *testing.T) {
	pool := newAllocationIntegrationPool(t)
	svc := NewCustomerEngagementAllocationService(
		repository.NewCustomerEngagementAllocationRepository(repository.NewScoped(pool)), testFirefightingTypeID)
	ctx := repository.WithSystemIdentity(context.Background())

	ff := allocFirefightingEvent()
	ff.ID, ff.Email, ff.Engagement.EngagementID = "AIT0001", "alloc-itest@wso2.com", "EIT0001"
	ff.CustomerCode = allocStr(itAccountSfID[:15])

	// Concurrent repeats of a new event still give one engagement and one allocation row.
	var wg sync.WaitGroup
	results := make([]domain.AllocationEventResult, 4)
	errs := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.ProcessAllocationEvent(ctx, ff)
		}(i)
	}
	wg.Wait()
	created := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
		if results[i].EngagementCreated {
			created++
		}
	}
	if created != 1 {
		t.Errorf("engagementCreated on %d events, want 1", created)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement WHERE engagement_id = 'EIT0001'`); n != 1 {
		t.Fatalf("engagements = %d", n)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement_allocation_resource WHERE allocation_id = 'AIT0001'`); n != 1 {
		t.Fatalf("allocation rows = %d", n)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement WHERE engagement_id = 'EIT0001'
		AND account_id = $1 AND delivery_mode = 'OFFSITE' AND state = 'NEW' AND engagement_type_id = $2
		AND name = 'Acme - Support Related Customer Firefighting' AND created_by = 'allocation-sync'`,
		itAccountLive, testFirefightingTypeID); n != 1 {
		t.Error("engagement columns are not as expected (live account by 15-char sf_id, OFFSITE, NEW)")
	}

	// Update path: dates, timezone and state change on the same row.
	ff.EndDate, ff.TimeZone, ff.ClearanceStatus = "2026-12-31", allocStr("Europe/London"), allocStr("Rejected - Visa Issues")
	res, err := svc.ProcessAllocationEvent(ctx, ff)
	if err != nil || res.Result != domain.AllocationEventUpdated || res.EngagementCreated {
		t.Fatalf("update = %+v, %v", res, err)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement_allocation_resource WHERE id = $1
		AND end_date = '2026-12-31' AND timezone = 'Europe/London' AND state = 'REJECTED_VISA_ISSUES' AND resource_id = $2`,
		*res.AllocationResourceID, itUser); n != 1 {
		t.Error("allocation row not updated")
	}

	// Line-item path through line_item_id_ref -> sf_opportunity_product.
	li := allocLineItemEvent()
	li.ID, li.Email = "AIT0002", "alloc.itest"
	li.Engagement.ProductID = allocStr(itLineItemSfID)
	res, err = svc.ProcessAllocationEvent(ctx, li)
	if err != nil || res.Result != domain.AllocationEventCreated || res.EngagementID == nil || *res.EngagementID != itLineEngagement {
		t.Fatalf("line item = %+v, %v", res, err)
	}
	li.Engagement.ProductID = allocStr("00kITESTMISSING01A")
	res, err = svc.ProcessAllocationEvent(ctx, li)
	if err != nil || res.Result != domain.AllocationEventSkipped || res.Reason != AllocationSkipNoLineItem {
		t.Fatalf("missing line item = %+v, %v", res, err)
	}
}
