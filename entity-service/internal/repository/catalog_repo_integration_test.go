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

// Regression test for migration 0125: catalog_variable's extra fields
// (read_only/hidden/reference_table/max_length/validation_*) and the sibling
// catalog_variable_choice table, exercised through the real
// CatalogRepository.GetCatalogItemVariables Go code. Runs against a real
// Postgres with 0125 applied. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CatalogVariableExtraFields

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	cveCatalogID        = "74000000-0000-0000-0000-000000000001"
	cveCatalogItemID    = "74000000-0000-0000-0000-000000000002"
	cveVarPlainID       = "74000000-0000-0000-0000-000000000003"
	cveVarChoiceID      = "74000000-0000-0000-0000-000000000004"
	cveChoiceActiveID   = "74000000-0000-0000-0000-000000000005"
	cveChoiceInactiveID = "74000000-0000-0000-0000-000000000006"
	cveVarFullID        = "74000000-0000-0000-0000-000000000007"
)

// seedCatalogVariableExtraFieldsFixture creates one sr_category/catalog_item
// pair (linked via catalog_item_category) and three catalog_variable rows:
//   - cveVarPlainID: no extra fields set, no choices.
//   - cveVarChoiceID: one active + one inactive choice.
//   - cveVarFullID: every extra field set (read_only, hidden, max_length,
//     reference_table, validation_*), no choices.
func seedCatalogVariableExtraFieldsFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM catalog_variable_choice WHERE catalog_variable_id = ANY($1::uuid[])`,
			[]string{cveVarPlainID, cveVarChoiceID, cveVarFullID})
		_, _ = pool.Exec(ctx, `DELETE FROM catalog_variable WHERE catalog_item_id = $1`, cveCatalogItemID)
		_, _ = pool.Exec(ctx, `DELETE FROM catalog_item_category WHERE catalog_item_id = $1`, cveCatalogItemID)
		_, _ = pool.Exec(ctx, `DELETE FROM catalog_item WHERE id = $1`, cveCatalogItemID)
		_, _ = pool.Exec(ctx, `DELETE FROM sr_category WHERE id = $1`, cveCatalogID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.120s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO sr_category (id, created_on, updated_on, created_by, updated_by, name, is_active)
		VALUES ($1, $2, $2, 'test', 'test', 'CVE Test Category', true)`, cveCatalogID, now)
	mustExec(`INSERT INTO catalog_item (id, created_on, updated_on, created_by, updated_by, name)
		VALUES ($1, $2, $2, 'test', 'test', 'CVE Test Item')`, cveCatalogItemID, now)
	mustExec(`INSERT INTO catalog_item_category (id, created_on, updated_on, created_by, updated_by, catalog_item_id, sr_category_id)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3)`, now, cveCatalogItemID, cveCatalogID)

	mustExec(`INSERT INTO catalog_variable (id, created_on, updated_on, created_by, updated_by, name, is_active,
			question_text, type, "order", is_mandatory, default_value, catalog_item_id)
		VALUES ($1, $2, $2, 'test', 'test', 'plain_field', true, 'Plain question', 'string', 1, false, NULL, $3)`,
		cveVarPlainID, now, cveCatalogItemID)

	mustExec(`INSERT INTO catalog_variable (id, created_on, updated_on, created_by, updated_by, name, is_active,
			question_text, type, "order", is_mandatory, default_value, catalog_item_id)
		VALUES ($1, $2, $2, 'test', 'test', 'choice_field', true, 'Choice question', 'choice', 2, false, NULL, $3)`,
		cveVarChoiceID, now, cveCatalogItemID)
	mustExec(`INSERT INTO catalog_variable_choice (id, created_on, updated_on, created_by, updated_by,
			catalog_variable_id, value, text, "order", is_inactive)
		VALUES ($1, $2, $2, 'test', 'test', $3, 'active_val', 'Active Choice', 1, false)`,
		cveChoiceActiveID, now, cveVarChoiceID)
	mustExec(`INSERT INTO catalog_variable_choice (id, created_on, updated_on, created_by, updated_by,
			catalog_variable_id, value, text, "order", is_inactive)
		VALUES ($1, $2, $2, 'test', 'test', $3, 'inactive_val', 'Inactive Choice', 2, true)`,
		cveChoiceInactiveID, now, cveVarChoiceID)

	mustExec(`INSERT INTO catalog_variable (id, created_on, updated_on, created_by, updated_by, name, is_active,
			question_text, type, "order", is_mandatory, default_value, catalog_item_id,
			read_only, hidden, reference_table, max_length, validation_name, validation_regex, validation_message)
		VALUES ($1, $2, $2, 'test', 'test', 'full_field', true, 'Full question', 'reference', 3, true, 'default-val', $3,
			true, true, 'sys_user', 80, 'alphanumeric', '^[a-zA-Z0-9]+$', 'Must be alphanumeric')`,
		cveVarFullID, now, cveCatalogItemID)
}

func TestCatalogRepository_GetCatalogItemVariables_ExtraFieldsAndChoices(t *testing.T) {
	pool := caseStatsPool(t)
	seedCatalogVariableExtraFieldsFixture(t, pool)

	repo := repository.NewCatalogRepository(repository.NewScoped(pool))
	variables, err := repo.GetCatalogItemVariables(context.Background(), cveCatalogID, cveCatalogItemID)
	if err != nil {
		t.Fatalf("GetCatalogItemVariables: unexpected error = %v", err)
	}
	if len(variables) != 3 {
		t.Fatalf("GetCatalogItemVariables: got %d variables, want 3", len(variables))
	}

	byID := make(map[string]int, len(variables))
	for i, v := range variables {
		byID[v.ID] = i
	}

	t.Run("variable with no choices omits Choices", func(t *testing.T) {
		v := variables[byID[cveVarPlainID]]
		if v.Choices != nil {
			t.Fatalf("Choices = %#v, want nil (omitted)", v.Choices)
		}
		if v.Validation != nil {
			t.Fatalf("Validation = %#v, want nil", v.Validation)
		}
		if v.ReadOnly || v.Hidden {
			t.Fatalf("ReadOnly=%v Hidden=%v, want false/false (NULL -> zero value)", v.ReadOnly, v.Hidden)
		}
		if v.MaxLength != nil {
			t.Fatalf("MaxLength = %v, want nil", v.MaxLength)
		}
		if v.ReferenceTable != nil {
			t.Fatalf("ReferenceTable = %v, want nil", v.ReferenceTable)
		}
	})

	t.Run("active+inactive choice: only the active one is returned", func(t *testing.T) {
		v := variables[byID[cveVarChoiceID]]
		if len(v.Choices) != 1 {
			t.Fatalf("Choices = %#v, want exactly 1 (inactive choice excluded)", v.Choices)
		}
		if v.Choices[0].Value == nil || *v.Choices[0].Value != "active_val" {
			t.Fatalf("Choices[0].Value = %v, want \"active_val\"", v.Choices[0].Value)
		}
		if v.Choices[0].Text == nil || *v.Choices[0].Text != "Active Choice" {
			t.Fatalf("Choices[0].Text = %v, want \"Active Choice\"", v.Choices[0].Text)
		}
	})

	t.Run("variable with every extra field set", func(t *testing.T) {
		v := variables[byID[cveVarFullID]]
		if !v.ReadOnly || !v.Hidden {
			t.Fatalf("ReadOnly=%v Hidden=%v, want true/true", v.ReadOnly, v.Hidden)
		}
		if v.MaxLength == nil || *v.MaxLength != 80 {
			t.Fatalf("MaxLength = %v, want 80", v.MaxLength)
		}
		if v.ReferenceTable == nil || *v.ReferenceTable != "sys_user" {
			t.Fatalf("ReferenceTable = %v, want \"sys_user\"", v.ReferenceTable)
		}
		if v.Validation == nil {
			t.Fatalf("Validation = nil, want non-nil")
		} else {
			if v.Validation.Name != "alphanumeric" {
				t.Fatalf("Validation.Name = %q, want \"alphanumeric\"", v.Validation.Name)
			}
			if v.Validation.Regex != "^[a-zA-Z0-9]+$" {
				t.Fatalf("Validation.Regex = %q, want \"^[a-zA-Z0-9]+$\"", v.Validation.Regex)
			}
			if v.Validation.Message != "Must be alphanumeric" {
				t.Fatalf("Validation.Message = %q, want \"Must be alphanumeric\"", v.Validation.Message)
			}
		}
		if v.Choices != nil {
			t.Fatalf("Choices = %#v, want nil (omitted)", v.Choices)
		}
	})
}
