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

// CreateCallRequest numbers a new call request from the CSTASK series
// (migration 0180); the number is read back from the table, and from the
// search view, rather than taken from a return value. Same DSN and
// skip-when-unset pattern as the other call request integration tests. The
// CSTASK series must exist, as it does once migration 0180 is applied:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run TestCallRequestNumberIntegration

package repository_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	crnProjectID = "39b11111-1111-1111-1111-111111111111"
	crnCaseID    = "39b22222-0000-0000-0000-000000000001"
	crnUserID    = "39b33333-0000-0000-0000-000000000001"
)

func TestCallRequestNumberIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	scoped := repository.NewScoped(pool)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM customer_call WHERE work_item_id = $1`, crnCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, crnCaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, crnUserID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, crnProjectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, end_date)
	          VALUES ($1, now(), now(), 'crn-test', 'crn-test', 'CRNTEST', 'sf-crntest', (now() + INTERVAL '30 days')::date)`, crnProjectID)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name) VALUES ($1, now(), now(), 'crn-test-caller')`, crnUserID)
	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ($1, now(), now(), 'crn-test', 'crn-test', 'CRNCASE01', 'crn-wso2-id', 'call request number test', 'CASE', $2)`,
		crnCaseID, crnProjectID)

	repo := repository.NewCallRequestRepository(scoped)
	create := func() {
		t.Helper()
		_, err := repo.CreateCallRequest(ctx, domain.CreateCallRequestRequest{
			CaseID: crnCaseID, Reason: "number test", UTCTimes: []string{"2030-01-01T10:00:00Z"}, DurationMinutes: 30,
		}, crnUserID, "jane.doe@example.com")
		if err != nil {
			t.Fatalf("CreateCallRequest: %v", err)
		}
	}
	create()
	create()

	rows, err := pool.Query(ctx, `SELECT number FROM customer_call WHERE work_item_id = $1 ORDER BY number`, crnCaseID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	defer rows.Close()
	var numbers []string
	for rows.Next() {
		var n *string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if n == nil {
			t.Fatal("call request was created with a NULL number")
		}
		numbers = append(numbers, *n)
	}
	if len(numbers) != 2 {
		t.Fatalf("got %d call requests, want 2", len(numbers))
	}
	shape := regexp.MustCompile(`^CSTASK\d{7,}$`)
	for _, n := range numbers {
		if !shape.MatchString(n) {
			t.Errorf("number %q does not match the CSTASK series shape", n)
		}
	}
	if numbers[0] == numbers[1] {
		t.Errorf("two call requests share the number %q", numbers[0])
	}

	views, _, err := repo.SearchCallRequests(ctx, crnCaseID, nil, domain.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("SearchCallRequests: %v", err)
	}
	for _, v := range views {
		if !shape.MatchString(v.Number) {
			t.Errorf("search view number = %q, want a CSTASK number", v.Number)
		}
	}
}
