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

// The task SLA reader must not serve the clock of an internal-only work item
// (a product issue). Skipped without ENTITY_TEST_DATABASE_URL:
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/repository/ -run TaskSla

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	taskSlaIOCaseWI  = "5a1a1000-0000-4000-8000-000000000001"
	taskSlaIOIssueWI = "5a1a1000-0000-4000-8000-000000000002"
	taskSlaIOCaseSla = "5a1a1000-0000-4000-8000-0000000000a1"
	taskSlaIOPISla   = "5a1a1000-0000-4000-8000-0000000000a2"
)

func TestTaskSlaIntegration_InternalOnlyClocksAreNotServed(t *testing.T) {
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := repository.WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	clean := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM work_item WHERE id IN ($1::uuid, $2::uuid)`, taskSlaIOCaseWI, taskSlaIOIssueWI) // cascades to sla
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		  VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', 'TSIO-CASE-1', 'TSIO-W-1', 'task sla case', 'CASE')`, []any{taskSlaIOCaseWI}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		  VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', next_work_item_number('PRODUCT_ISSUE'), 'task sla issue', 'PRODUCT_ISSUE')`, []any{taskSlaIOIssueWI}},
		{`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, start_on, stage, is_active)
		  VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', $2::uuid, NOW(), 'IN_PROGRESS', TRUE)`, []any{taskSlaIOCaseSla, taskSlaIOCaseWI}},
		{`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, start_on, stage, is_active, sla_policy_id)
		  VALUES ($1::uuid, NOW(), NOW(), 'test', 'test', $2::uuid, NOW(), 'IN_PROGRESS', TRUE,
		          (SELECT id FROM sla_policy WHERE applies_to = 'PRODUCT_ISSUE' ORDER BY name LIMIT 1))`, []any{taskSlaIOPISla, taskSlaIOIssueWI}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", q.sql, err)
		}
	}
	repo := repository.NewTaskSlaRepository(repository.NewScoped(pool))

	// Without taskIds the search covers every clock; only ours are checked.
	all, _, err := repo.SearchTaskSlas(ctx, nil, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sawCase bool
	for _, v := range all {
		switch v.ID {
		case taskSlaIOPISla:
			t.Error("the search returned a product issue clock")
		case taskSlaIOCaseSla:
			sawCase = true
		}
	}
	if !sawCase {
		t.Error("the search lost the case clock")
	}
	// Asking for the issue by id returns nothing, and does not count it.
	got, total, err := repo.SearchTaskSlas(ctx, []string{taskSlaIOIssueWI, taskSlaIOCaseWI}, 50, 0)
	if err != nil || total != 1 || len(got) != 1 || got[0].ID != taskSlaIOCaseSla {
		t.Errorf("search by task ids = %d/%d %v", len(got), total, err)
	}

	if _, err := repo.GetTaskSla(ctx, taskSlaIOCaseSla); err != nil {
		t.Errorf("get the case clock: %v", err)
	}
	_, err = repo.GetTaskSla(ctx, taskSlaIOPISla)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("get the product issue clock = %v, want not found", err)
	}
}
