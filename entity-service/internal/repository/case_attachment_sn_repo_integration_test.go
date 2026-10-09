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

// This is an integration test: it exercises CreateCaseAttachmentFromServiceNow
// and the NULL-storage_key scan fix in SearchCaseAttachments/
// GetCaseAttachmentByID against a live PostgreSQL instance with migration
// 0185 applied. A fake pool can't reproduce this: the bug this guards against
// (pgx failing to scan a real SQL NULL into a non-pointer string local) can
// only be reproduced by a real NULL value coming back from a real query. Same
// DSN and skip-when-unset pattern as project_stats_repo_integration_test.go/
// time_card_repo_test.go (package repository_test, same package, so
// caseStatsPool below is reused):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseAttachmentSNIntegration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	caseAttachmentSNTestProjectID    = "47777777-0000-0000-0000-000000000001"
	caseAttachmentSNTestUserID       = "48888888-0000-0000-0000-000000000001"
	caseAttachmentSNTestCaseID       = "49999999-0000-0000-0000-000000000001"
	caseAttachmentSNTestAttachmentID = "4aaaaaaa-0000-0000-0000-000000000001"
	// caseAttachmentPGTestAttachmentID is a second, plain-Postgres-style
	// (SFTPGo-backed, storage_key set) row seeded alongside the
	// ServiceNow-sourced one, so SearchCaseAttachments/GetCaseAttachmentByID
	// are proven to handle BOTH a NULL and a non-NULL storage_key in the same
	// result set -- not just a table that happens to contain only one shape.
	caseAttachmentPGTestAttachmentID = "4bbbbbbb-0000-0000-0000-000000000001"
)

func seedCaseAttachmentSNFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM case_attachment WHERE case_id = $1`, caseAttachmentSNTestCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM "case" WHERE id = $1`, caseAttachmentSNTestCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, caseAttachmentSNTestCaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, caseAttachmentSNTestUserID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, caseAttachmentSNTestProjectID)
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
	          VALUES ($1, now(), now(), 'case-attachment-sn-test', 'case-attachment-sn-test', 'CASNTEST', 'sf-casntest', (now() + INTERVAL '30 days')::date)`,
		caseAttachmentSNTestProjectID)

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
	          VALUES ($1, now(), now(), 'case-attachment-sn-test@example.com', 'case-attachment-sn-test@example.com', true)`,
		caseAttachmentSNTestUserID)

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ($1, now(), now(), 'case-attachment-sn-test', 'case-attachment-sn-test', 'CASN01', 'CASN-1', 'a case', 'CASE', $2)`,
		caseAttachmentSNTestCaseID, caseAttachmentSNTestProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, caseAttachmentSNTestCaseID)

	// A plain, SFTPGo-backed (storage_key set) row, seeded directly rather
	// than through CreateCaseAttachment, so this fixture doesn't depend on
	// that method's own behavior -- only on the schema shape.
	mustExec(`INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status, created_on)
	          VALUES ($1, $2, 'cases/pg-backed/notes.txt', 'notes.txt', 'text/plain', 11, $3, 'complete', now())`,
		caseAttachmentPGTestAttachmentID, caseAttachmentSNTestCaseID, caseAttachmentSNTestUserID)
}

// TestCaseAttachmentSNIntegration_CreateAndReadBackWithNullStorageKey is the
// regression test for the dual-write attachment metadata gap: before
// migration 0185 (storage_key DROP NOT NULL) and the corresponding
// SearchCaseAttachments/GetCaseAttachmentByID scan fix, a ServiceNow-sourced
// row with no storage_key either couldn't be inserted at all (NOT NULL
// violation) or, once the column was made nullable, would crash those two
// read paths with "cannot scan NULL into *string" the moment one existed
// alongside an ordinary SFTPGo-backed row.
func TestCaseAttachmentSNIntegration_CreateAndReadBackWithNullStorageKey(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseAttachmentSNFixture(t, pool)

	ctx := repository.WithSystemIdentity(context.Background())
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	description := "a servicenow-sourced attachment"
	req := domain.CreateAttachmentRequest{
		ReferenceID:   caseAttachmentSNTestCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "diagnostics.log",
		Type:          "text/plain",
		Description:   &description,
	}

	// Bracket the insert with the database's own clock so the assertion below
	// does not depend on the test machine's clock or timezone.
	var before, after time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&before); err != nil {
		t.Fatalf("read database clock: %v", err)
	}
	created, err := repo.CreateCaseAttachmentFromServiceNow(ctx, req, caseAttachmentSNTestAttachmentID, 2048, caseAttachmentSNTestUserID)
	if err != nil {
		t.Fatalf("CreateCaseAttachmentFromServiceNow() error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&after); err != nil {
		t.Fatalf("read database clock: %v", err)
	}
	// created_on is the database's own clock, never a ServiceNow timestamp: the
	// zone-less createdOn ServiceNow replies with is local wall-clock time, and
	// storing it once put rows hours in the future ("uploaded 5h from now").
	if created.CreatedOn.Before(before) || created.CreatedOn.After(after) {
		t.Errorf("created.CreatedOn = %v, want within the database clock window [%v, %v]", created.CreatedOn, before, after)
	}
	if created.ID != caseAttachmentSNTestAttachmentID {
		t.Errorf("created.ID = %q, want %q", created.ID, caseAttachmentSNTestAttachmentID)
	}
	if created.StorageKey != nil {
		t.Errorf("created.StorageKey = %v, want nil for a ServiceNow-sourced row", created.StorageKey)
	}
	if created.Status != domain.AttachmentStatusComplete {
		t.Errorf("created.Status = %q, want %q", created.Status, domain.AttachmentStatusComplete)
	}

	t.Run("SearchCaseAttachmentsToleratesMixedNullAndSetStorageKeys", func(t *testing.T) {
		attachments, total, err := repo.SearchCaseAttachments(ctx, caseAttachmentSNTestCaseID, domain.Pagination{Limit: 10, Offset: 0})
		if err != nil {
			t.Fatalf("SearchCaseAttachments() error = %v, want no error scanning a NULL storage_key row", err)
		}
		if total != 2 || len(attachments) != 2 {
			t.Fatalf("SearchCaseAttachments() returned %d/%d rows, want exactly 2 (one SN-sourced, one SFTPGo-backed)", len(attachments), total)
		}
		var sawNilStorageKey, sawSetStorageKey bool
		for _, a := range attachments {
			if a.ID == caseAttachmentSNTestAttachmentID {
				if a.StorageKey != nil {
					t.Errorf("SN-sourced row StorageKey = %v, want nil", a.StorageKey)
				}
				sawNilStorageKey = true
			}
			if a.ID == caseAttachmentPGTestAttachmentID {
				if a.StorageKey == nil || *a.StorageKey != "cases/pg-backed/notes.txt" {
					t.Errorf("SFTPGo-backed row StorageKey = %v, want the seeded key", a.StorageKey)
				}
				sawSetStorageKey = true
			}
		}
		if !sawNilStorageKey || !sawSetStorageKey {
			t.Fatalf("expected to see both a nil and a set storageKey, sawNil=%v sawSet=%v", sawNilStorageKey, sawSetStorageKey)
		}
	})

	t.Run("GetCaseAttachmentByIDToleratesNullStorageKey", func(t *testing.T) {
		a, err := repo.GetCaseAttachmentByID(ctx, caseAttachmentSNTestAttachmentID)
		if err != nil {
			t.Fatalf("GetCaseAttachmentByID() error = %v, want no error scanning a NULL storage_key row", err)
		}
		if a.StorageKey != nil {
			t.Errorf("StorageKey = %v, want nil", a.StorageKey)
		}
		if a.Description == nil || *a.Description != description {
			t.Errorf("Description = %v, want %q", a.Description, description)
		}
		if !a.CreatedOn.Equal(created.CreatedOn) {
			t.Errorf("stored CreatedOn = %v, want the value the create returned, %v", a.CreatedOn, created.CreatedOn)
		}
	})
}
