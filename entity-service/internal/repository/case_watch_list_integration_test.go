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

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SetCaseWatchList replaces the list in one statement, tolerates a repeated
// id, and reports an unknown id as a validation error. Skipped without
// CASE_STATS_TEST_DSN.
func TestSetCaseWatchList_ReplacesBatchAndToleratesRepeats(t *testing.T) {
	pool := caseStatsPool(t)
	seedRLSScopedRemaining(t, pool)
	const (
		userA   = "7a000000-0000-4000-8000-000000000101"
		userB   = "7a000000-0000-4000-8000-000000000102"
		missing = "00000000-0000-0000-0000-000000000000"
	)
	caseID := rsWorkItems[0]
	scoped := repository.NewScoped(pool)
	ctx := rsInternal()
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item_watcher WHERE work_item_id = $1`, caseID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id IN ($1, $2)`, userA, userB)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(context.Background(), `INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES
		($1, now(), now(), 'watch-a', 'watch.a@example.com'), ($2, now(), now(), 'watch-b', 'watch.b@example.com')`, userA, userB); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	repo := repository.NewCaseRepository(scoped)

	got, _, err := repo.SetCaseWatchList(ctx, caseID, []string{userA, userB, userA}, "jane.doe@example.com")
	if err != nil {
		t.Fatalf("set [A, B, A]: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("watchers after [A, B, A] = %d, want 2", len(got))
	}

	got, _, err = repo.SetCaseWatchList(ctx, caseID, []string{userB}, "jane.doe@example.com")
	if err != nil {
		t.Fatalf("set [B]: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("watchers after [B] = %d, want 1 (the list is replaced)", len(got))
	}

	_, _, err = repo.SetCaseWatchList(ctx, caseID, []string{userA, missing}, "jane.doe@example.com")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("set with an unknown id: err = %v, want a validation error", err)
	}

	if got, _, err = repo.SetCaseWatchList(ctx, caseID, nil, "jane.doe@example.com"); err != nil || len(got) != 0 {
		t.Errorf("clear: %d watchers, err %v; want 0, nil", len(got), err)
	}
}
