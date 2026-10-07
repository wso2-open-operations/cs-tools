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

// Runs against a migrated database (0187 included); skipped unless ENTITY_TEST_DATABASE_URL is set.
const (
	itAccountLive    = "a110c000-0000-4000-8000-000000000001"
	itAccountDeleted = "a110c000-0000-4000-8000-000000000002"
	itUser           = "a110c000-0000-4000-8000-000000000011"
	itLineEngagement = "a110c000-0000-4000-8000-000000000021"
	itSfIDEngagement = "a110c000-0000-4000-8000-000000000022"
	itSfIDOnlyEng    = "a110c000-0000-4000-8000-000000000023"
	itSfIDOnlySf     = "00kITESTALLOC03AAA"
	itLineItemRow    = "a110c000-0000-4000-8000-000000000031"
	itNewLineItemRow = "a110c000-0000-4000-8000-000000000032"
	itOpportunity    = "a110c000-0000-4000-8000-000000000041"
	itNewLineItemSf  = "00kITESTALLOC02AAA"
	itAccountSfID    = "001ITESTALLOC0001A"
	itLineItemSfID   = "00kITESTALLOC01AAA"
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
			`DELETE FROM customer_engagement WHERE engagement_id LIKE 'EIT%' OR id IN ('` + itLineEngagement + `', '` + itSfIDEngagement + `', '` + itSfIDOnlyEng + `')`,
			`DELETE FROM sf_opportunity_product WHERE id IN ('` + itLineItemRow + `', '` + itNewLineItemRow + `')`,
			`DELETE FROM sf_opportunity WHERE id = '` + itOpportunity + `'`,
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
		`INSERT INTO sf_opportunity (id, created_on, updated_on, created_by, updated_by, name) VALUES
			('` + itOpportunity + `', now(), now(), 't', 't', 'Alloc ITest Opp')`,
		`INSERT INTO sf_opportunity_product (id, created_on, updated_on, created_by, updated_by, line_item_sf_id, opportunity_id) VALUES
			('` + itLineItemRow + `', now(), now(), 't', 't', '` + itLineItemSfID + `', NULL),
			('` + itNewLineItemRow + `', now(), now(), 't', 't', '` + itNewLineItemSf + `', '` + itOpportunity + `')`,
		// Older sf_id-only row first: the line_item_id match must still win.
		`INSERT INTO customer_engagement (id, created_on, updated_on, name, sf_id, line_item_id) VALUES
			('` + itSfIDEngagement + `', now() - interval '1 day', now(), 'Sf id engagement', '` + itLineItemSfID + `', NULL),
			('` + itLineEngagement + `', now(), now(), 'Line engagement', NULL, '` + itLineItemRow + `'),
			('` + itSfIDOnlyEng + `', now(), now(), 'Sf id only engagement', '` + itSfIDOnlySf + `', NULL)`,
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
		repository.NewCustomerEngagementAllocationRepository(repository.NewScoped(pool)))
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
		AND account_id = $1 AND delivery_mode = 'OFFSITE' AND state = 'NEW' AND engagement_type = 'FIREFIGHTING'
		AND name = 'Acme - Support Related Customer Firefighting' AND created_by = 'allocation-sync'
		AND sf_id IS NULL AND line_item_id IS NULL AND opportunity_id IS NULL`,
		itAccountLive); n != 1 {
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

	// Line-item path through line_item_id -> sf_opportunity_product (sf_id empty).
	li := allocLineItemEvent()
	li.ID, li.Email, li.Engagement.EngagementID = "AIT0002", "alloc.itest", "EIT0002"
	li.Engagement.ProductID = allocStr(itLineItemSfID)
	res, err = svc.ProcessAllocationEvent(ctx, li)
	if err != nil || res.Result != domain.AllocationEventCreated || res.EngagementID == nil || *res.EngagementID != itLineEngagement {
		t.Fatalf("line item = %+v, %v", res, err)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement WHERE id = $1 AND engagement_id IS NULL`, itLineEngagement); n != 1 {
		t.Error("a line-item match must not fill engagement_id")
	}

	// sf_id-only engagement, matched by the 15-character id.
	sf := allocLineItemEvent()
	sf.ID, sf.Email = "AIT0004", "alloc.itest"
	sf.Engagement.ProductID = allocStr(itSfIDOnlySf[:15])
	res, err = svc.ProcessAllocationEvent(ctx, sf)
	if err != nil || res.EngagementID == nil || *res.EngagementID != itSfIDOnlyEng {
		t.Fatalf("sf_id match = %+v, %v", res, err)
	}

	// No engagement for the line item: skipped, nothing created, even with a new engagement id.
	miss := allocLineItemEvent()
	miss.ID, miss.Email, miss.CustomerCode = "AIT0003", "alloc-itest@wso2.com", allocStr(itAccountSfID)
	miss.Engagement.EngagementID, miss.Engagement.ProductID = "EIT0003", allocStr(itNewLineItemSf)
	res, err = svc.ProcessAllocationEvent(ctx, miss)
	if err != nil || res.Result != domain.AllocationEventSkipped || res.Reason != AllocationSkipNoLineItem {
		t.Fatalf("missing line item = %+v, %v", res, err)
	}
	if n := itCount(t, pool, `SELECT count(*) FROM customer_engagement WHERE engagement_id = 'EIT0003'`); n != 0 {
		t.Error("an engagement was created for a non-firefighting allocation")
	}
}
