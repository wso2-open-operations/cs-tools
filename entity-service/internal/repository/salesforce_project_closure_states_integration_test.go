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

// Integration test: a project inserted by the Salesforce project ingest starts
// with all four closure states OPEN, so the Account Closure Process's
// wso2_closure_state = 'OPEN' listing returns it, and migration 0291's column
// defaults give the same for an insert that omits them. Runs against a real
// Postgres with the migrations applied, skipped without CASE_STATS_TEST_DSN:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SalesforceProjectInsertClosureStates

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	sfProjectClosureInsertedID = "93000000-0000-0000-0000-0000000000b1"
	sfProjectClosureDefaultID  = "93000000-0000-0000-0000-0000000000b2"
)

func TestSalesforceProjectInsertClosureStatesIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id IN ($1, $2)`, sfProjectClosureInsertedID, sfProjectClosureDefaultID)
		_, _ = pool.Exec(ctx, `DELETE FROM salesforce_ingest_state WHERE entity = 'project' AND sf_id = 'sf-CLOSUREINS'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	repo := repository.NewSalesforceProjectRepository(repository.NewScoped(pool))
	id := sfProjectClosureInsertedID
	res, err := repo.UpsertFromSalesforce(ctx, domain.SalesforceProjectUpsert{
		SfID: "sf-CLOSUREINS", Key: "CLOSUREINS", AllowInsert: true, InsertID: &id,
	}, domain.UpsertSalesforceIngestStateRequest{
		Entity: "project", SfID: "sf-CLOSUREINS", EventModifiedOn: time.Now(),
		EventType: "CREATED", Status: domain.SalesforceIngestSucceeded,
	})
	if err != nil {
		t.Fatalf("UpsertFromSalesforce: %v", err)
	}
	if res.ProjectID != sfProjectClosureInsertedID {
		t.Fatalf("inserted id = %q, want %q", res.ProjectID, sfProjectClosureInsertedID)
	}

	const states = `SELECT wso2_closure_state::text, end_date_closure_state::text,
		invoice_due_date_closure_state::text, compliance_violation_closure_state::text
		FROM project WHERE id = $1`
	assertOpen := func(label, projectID string) {
		t.Helper()
		var got [4]*string
		if err := pool.QueryRow(ctx, states, projectID).Scan(&got[0], &got[1], &got[2], &got[3]); err != nil {
			t.Fatalf("%s: read states: %v", label, err)
		}
		for i, g := range got {
			if g == nil || *g != "OPEN" {
				t.Errorf("%s: closure column %d = %v, want OPEN", label, i, g)
			}
		}
	}
	assertOpen("salesforce insert", sfProjectClosureInsertedID)

	// The ACP candidate listing filters on wso2_closure_state = 'OPEN'.
	var listed bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project WHERE id = $1 AND wso2_closure_state = 'OPEN')`,
		sfProjectClosureInsertedID).Scan(&listed); err != nil || !listed {
		t.Errorf("ACP OPEN filter returns the inserted project = %v (err %v), want true", listed, err)
	}

	// A raw insert that omits the columns gets the migration's DEFAULT.
	if _, err := pool.Exec(ctx, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id)
		VALUES ($1, now(), now(), 'closure-test', 'closure-test', 'CLOSUREDEF', 'sf-CLOSUREDEF')`, sfProjectClosureDefaultID); err != nil {
		t.Fatalf("seed default-insert project: %v", err)
	}
	assertOpen("column default", sfProjectClosureDefaultID)
}
