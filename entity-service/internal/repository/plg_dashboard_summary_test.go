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

// TestDashboardSummaryCountsUnresolvedIngestFailures covers the one part of the
// summary query that nothing else does.
//
// The query is a single SELECT of positional subqueries read back by one Scan
// whose argument order has to match it exactly. Nothing checked that before:
// adding a subquery without its Scan argument, or in the wrong position, is not
// a compile error and no test failed — the first sign would have been a runtime
// scan error on the dashboard, or worse, five correct numbers and one silently
// holding another column's value.
//
// So this asserts on a count the fixture controls, which pins both that the
// subquery is present and that it is read into the field that belongs to it.
func TestDashboardSummaryCountsUnresolvedIngestFailures(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DELETE FROM plg_ingest_failure`); err != nil {
		t.Fatalf("clear ingest failures: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM plg_ingest_failure`)
	})

	repo := NewAnalyticsRepository(pool)
	// The range scopes the other counts; this one is deliberately not
	// period-scoped, so a wide window keeps the fixture honest either way.
	from, to := time.Now().AddDate(-1, 0, 0), time.Now().AddDate(1, 0, 0)
	rng := domain.AnalyticsRange{From: &from, To: &to}

	got, err := repo.Dashboard(ctx, rng)
	if err != nil {
		t.Fatalf("dashboard with an empty failure table: %v", err)
	}
	if got.Summary.UnresolvedIngestFailures != 0 {
		t.Fatalf("empty table should count 0, got %d", got.Summary.UnresolvedIngestFailures)
	}

	// Two open, one already resolved. The resolved one is what makes this a
	// test of the predicate rather than of COUNT(*): a plain count would
	// report three and pass a weaker assertion.
	const insert = `
		INSERT INTO plg_ingest_failure (event_id, event_type, received_at, payload, failure, resolved_on)
		VALUES ($1, 'registration', now(), '{}'::JSONB, $2, $3)`
	rows := []struct {
		id       string
		failure  string
		resolved any
	}{
		{"evt-open-1", "no plg_product row for platform", nil},
		{"evt-open-2", "organizationName missing", nil},
		{"evt-done-1", "fixed by hand", time.Now()},
	}
	for _, r := range rows {
		if _, err := pool.Exec(ctx, insert, r.id, r.failure, r.resolved); err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}

	got, err = repo.Dashboard(ctx, rng)
	if err != nil {
		t.Fatalf("dashboard with failures present: %v", err)
	}
	if got.Summary.UnresolvedIngestFailures != 2 {
		t.Errorf("want 2 unresolved (the third is resolved), got %d",
			got.Summary.UnresolvedIngestFailures)
	}
}
