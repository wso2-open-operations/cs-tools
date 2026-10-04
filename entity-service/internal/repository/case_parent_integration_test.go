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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// UpdateCaseParent refuses links that would put a case under itself.
// Skipped without CASE_STATS_TEST_DSN.
func TestUpdateCaseParent_RejectsSelfAndCycles(t *testing.T) {
	pool := caseStatsPool(t)
	seedRLSScopedRemaining(t, pool)
	scoped := repository.NewScoped(pool)
	repo := repository.NewCaseRepository(scoped)
	ctx := rsInternal()
	a, b, c := rsWorkItems[0], rsWorkItems[1], rsWorkItems[2]
	t.Cleanup(func() {
		_, _ = scoped.Exec(ctx, `UPDATE work_item SET parent_id = NULL WHERE id = ANY($1::uuid[])`, []string{a, b, c})
	})

	isValidation := func(err error) bool {
		var ve *apierror.ValidationError
		return errors.As(err, &ve)
	}

	if _, err := repo.UpdateCaseParent(ctx, a, a, "jane.doe@example.com"); !isValidation(err) {
		t.Errorf("A under A: err = %v, want a validation error", err)
	}
	if _, err := repo.UpdateCaseParent(ctx, b, a, "jane.doe@example.com"); err != nil {
		t.Fatalf("B under A: %v", err)
	}
	if _, err := repo.UpdateCaseParent(ctx, c, b, "jane.doe@example.com"); err != nil {
		t.Fatalf("C under B: %v", err)
	}
	if _, err := repo.UpdateCaseParent(ctx, a, b, "jane.doe@example.com"); !isValidation(err) {
		t.Errorf("A under B (B is A's child): err = %v, want a validation error", err)
	}
	if _, err := repo.UpdateCaseParent(ctx, a, c, "jane.doe@example.com"); !isValidation(err) {
		t.Errorf("A under C (C is A's grandchild): err = %v, want a validation error", err)
	}
	if _, err := repo.UpdateCaseParent(ctx, c, a, "jane.doe@example.com"); err != nil {
		t.Errorf("C moved directly under A: %v", err)
	}

	var parentOfA *string
	if err := scoped.QueryRow(ctx, `SELECT parent_id::text FROM work_item WHERE id = $1`, a).Scan(&parentOfA); err != nil {
		t.Fatalf("read A: %v", err)
	}
	if parentOfA != nil {
		t.Errorf("A's parent = %v after rejected links, want none", *parentOfA)
	}
}
