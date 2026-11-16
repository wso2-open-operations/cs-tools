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

// This is an integration test: SearchCaseAttachments reads work_item_attachment
// only (the older case_attachment table is not listed), and whether its total
// and its pages agree (no fan-out, no PENDING) can only be shown by real SQL
// against the real schema. Same DSN and skip-when-unset pattern as
// case_activity_synced_attachments_integration_test.go (same package):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseAttachmentsSynced

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	attUnionProjectID = "5b777777-0000-0000-0000-000000000001"
	attUnionCaseID    = "5b999999-0000-0000-0000-000000000001"
	attUnionEmptyCase = "5b999999-0000-0000-0000-000000000002"
	attUnionUserID1   = "5b888888-0000-0000-0000-000000000001"
	attUnionUserID2   = "5b888888-0000-0000-0000-000000000002"
	attUnionUploader  = "jane.doe@example.com"
)

func TestCaseAttachmentsSyncedUnionIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		for _, id := range []string{attUnionCaseID, attUnionEmptyCase} {
			_, _ = scoped.Exec(ctx, `DELETE FROM case_attachment WHERE case_id = $1`, id)
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item_attachment WHERE work_item_id = $1`, id)
			_, _ = scoped.Exec(ctx, `DELETE FROM "case" WHERE id = $1`, id)
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, id)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, attUnionUserID1, attUnionUserID2)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, attUnionProjectID)
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
	          VALUES ($1, now(), now(), 'att-union-test', 'att-union-test', 'ATTUNI', 'sf-attuni', (now() + INTERVAL '30 days')::date)`,
		attUnionProjectID)
	// Two users share the uploader's email ("user".email is not unique): a
	// plain join on email would turn one synced attachment into two rows while
	// the count still said one.
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name, is_active)
	          VALUES ($1, now(), now(), 'att-union-test-1', $3, 'Jane', 'Doe', true),
	                 ($2, now(), now(), 'att-union-test-2', $3, 'Jane', 'Doe', true)`,
		attUnionUserID1, attUnionUserID2, attUnionUploader)
	for i, id := range []string{attUnionCaseID, attUnionEmptyCase} {
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		          VALUES ($1, now(), now(), 'att-union-test', 'att-union-test', $3, $4, 'a case', 'CASE', $2)`,
			id, attUnionProjectID, []string{"ATTUNI01", "ATTUNI02"}[i], []string{"ATTUNI-1", "ATTUNI-2"}[i])
		mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, id)
	}

	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

	native := func(id string, ts time.Time, name, status string) {
		mustExec(`INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status, created_on)
		          VALUES ($1, $2, $3, $4, 'text/plain', 7, $5, $6, $7)`,
			id, attUnionCaseID, "cases/"+id+"/"+name, name, attUnionUserID1, status, ts)
	}
	synced := func(id string, ts time.Time, name, createdBy, state string) {
		mustExec(`INSERT INTO work_item_attachment (id, created_on, updated_on, created_by, updated_by, name, content_type, work_item_id, size_bytes, state)
		          VALUES ($1, $2, $2, $3, $3, $4, 'application/pdf', $5, 42, $6::work_item_attachment_state_enum)`,
			id, ts, createdBy, name, attUnionCaseID, state)
	}

	const (
		nativeOne     = "5b000000-0000-0000-0000-0000000000a1"
		nativePending = "5b000000-0000-0000-0000-0000000000a2"
		inBothTables  = "5b000000-0000-0000-0000-0000000000a3"
		syncedOne     = "5b000000-0000-0000-0000-0000000000b1"
		syncedTwo     = "5b000000-0000-0000-0000-0000000000b2"
		syncedPending = "5b000000-0000-0000-0000-0000000000b3"
		syncedGhost   = "5b000000-0000-0000-0000-0000000000b4"
		syncedSameTs1 = "5b000000-0000-0000-0000-0000000000b5"
		syncedSameTs2 = "5b000000-0000-0000-0000-0000000000b6"
	)
	native(nativeOne, at(1), "native.txt", "complete")
	native(nativePending, at(2), "uploading.txt", "pending")
	synced(syncedOne, at(3), "synced-1.pdf", attUnionUploader, "AVAILABLE")
	// An old upload left in case_attachment under the same id as a stored
	// attachment: only the work_item_attachment row is listed.
	native(inBothTables, at(4), "both.txt", "complete")
	synced(inBothTables, at(4), "both.txt", attUnionUploader, "AVAILABLE")
	synced(syncedPending, at(5), "pending.pdf", attUnionUploader, "PENDING")
	synced(syncedGhost, at(6), "ghost.pdf", "nobody@example.com", "AVAILABLE")
	synced(syncedTwo, at(7), "synced-2.pdf", attUnionUploader, "AVAILABLE")
	// Identical timestamps: the id tiebreaker keeps paging stable.
	synced(syncedSameTs1, at(8), "same-1.pdf", attUnionUploader, "AVAILABLE")
	synced(syncedSameTs2, at(8), "same-2.pdf", attUnionUploader, "AVAILABLE")

	repo := repository.NewCaseRepository(scoped)
	search := func(caseID string, limit, offset int) ([]domain.Attachment, int) {
		t.Helper()
		got, total, err := repo.SearchCaseAttachments(ctx, caseID, domain.Pagination{Limit: limit, Offset: offset})
		if err != nil {
			t.Fatalf("SearchCaseAttachments(limit=%d offset=%d): %v", limit, offset, err)
		}
		return got, total
	}

	// Newest first, id ascending on a tie. The case_attachment rows (complete
	// or pending) and the PENDING stored row are absent.
	wantIDs := []string{syncedSameTs1, syncedSameTs2, syncedTwo, syncedGhost, inBothTables, syncedOne}

	all, total := search(attUnionCaseID, 50, 0)
	if total != len(wantIDs) {
		t.Errorf("total = %d, want %d", total, len(wantIDs))
	}
	if len(all) != len(wantIDs) {
		t.Fatalf("got %d rows, want %d: %+v", len(all), len(wantIDs), all)
	}
	for i, id := range wantIDs {
		if all[i].ID != id {
			t.Errorf("row %d id = %s, want %s", i, all[i].ID, id)
		}
	}

	byID := map[string]domain.Attachment{}
	for _, a := range all {
		byID[a.ID] = a
	}
	// A case_attachment row is never listed, and the stored row that shares an
	// id with one is the one that shows.
	if _, ok := byID[nativeOne]; ok {
		t.Errorf("case_attachment row %s should not be listed", nativeOne)
	}
	if b := byID[inBothTables]; b.StorageKey != nil || b.Name != "both.txt" || b.Type != "application/pdf" {
		t.Errorf("same-id attachment should be the work_item_attachment row: %+v", b)
	}
	// Stored row: no storage key, complete, shaped like the work-item search.
	s := byID[syncedOne]
	if s.StorageKey != nil || s.Name != "synced-1.pdf" || s.Type != "application/pdf" || s.SizeBytes != 42 ||
		s.Status != domain.AttachmentStatusComplete || s.ReferenceType != domain.ReferenceTypeCase || s.ReferenceID != attUnionCaseID ||
		s.CreatedBy == nil || s.CreatedBy.Email != attUnionUploader || s.CreatedBy.Name != "Jane Doe" {
		t.Errorf("synced attachment = %+v (createdBy %+v)", s, s.CreatedBy)
	}
	// An uploader with no user row is still returned, carrying the email.
	g := byID[syncedGhost]
	if g.CreatedBy == nil || g.CreatedBy.Email != "nobody@example.com" || g.CreatedBy.ID != nil {
		t.Errorf("unresolved-uploader attachment createdBy = %+v", g.CreatedBy)
	}

	// Pages of every size add up to the full list: no duplicate, no miss.
	for _, limit := range []int{1, 2, 3, 7} {
		var seen []string
		for offset := 0; offset < len(wantIDs)+limit; offset += limit {
			page, pageTotal := search(attUnionCaseID, limit, offset)
			if pageTotal != len(wantIDs) {
				t.Errorf("limit=%d offset=%d total = %d, want %d", limit, offset, pageTotal, len(wantIDs))
			}
			for _, a := range page {
				seen = append(seen, a.ID)
			}
		}
		if len(seen) != len(wantIDs) {
			t.Errorf("limit=%d: paged %d rows, want %d: %v", limit, len(seen), len(wantIDs), seen)
			continue
		}
		for i, id := range wantIDs {
			if seen[i] != id {
				t.Errorf("limit=%d: paged row %d = %s, want %s", limit, i, seen[i], id)
			}
		}
	}

	// A case with no attachments at all is an empty list.
	empty, emptyTotal := search(attUnionEmptyCase, 50, 0)
	if emptyTotal != 0 || len(empty) != 0 {
		t.Errorf("empty case: total=%d rows=%d", emptyTotal, len(empty))
	}
}
