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

// This is an integration test: SearchCaseActivities' UNION ALL feed has to
// merge comments and the work_item_attachment table (the older case_attachment
// table is not read), and whether its count and rows agree (no fan-out, no
// PENDING) can only be shown by real SQL against the real schema. Same DSN and
// skip-when-unset pattern as case_attachment_sn_repo_integration_test.go
// (same package, so caseStatsPool is reused):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseActivitySyncedAttachments

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	activityAttProjectID = "5a777777-0000-0000-0000-000000000001"
	activityAttCaseID    = "5a999999-0000-0000-0000-000000000001"
	activityAttUserID1   = "5a888888-0000-0000-0000-000000000001"
	activityAttUserID2   = "5a888888-0000-0000-0000-000000000002"
	activityAttUploader  = "jane.doe@example.com"
)

func TestCaseActivitySyncedAttachmentsIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM comment WHERE work_item_id = $1`, activityAttCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM case_attachment WHERE case_id = $1`, activityAttCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item_attachment WHERE work_item_id = $1`, activityAttCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM "case" WHERE id = $1`, activityAttCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, activityAttCaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, activityAttUserID1, activityAttUserID2)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, activityAttProjectID)
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
	          VALUES ($1, now(), now(), 'activity-att-test', 'activity-att-test', 'ACTATT', 'sf-actatt', (now() + INTERVAL '30 days')::date)`,
		activityAttProjectID)
	// Two users share the uploader's email ("user".email is not unique): a
	// plain join on email would turn the one synced attachment into two feed
	// rows while the count still said one.
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name, is_active)
	          VALUES ($1, now(), now(), 'activity-att-test-1', $3, 'Jane', 'Doe', true),
	                 ($2, now(), now(), 'activity-att-test-2', $3, 'Jane', 'Doe', true)`,
		activityAttUserID1, activityAttUserID2, activityAttUploader)
	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
	          VALUES ($1, now(), now(), 'activity-att-test', 'activity-att-test', 'ACTATT01', 'ACTATT-1', 'a case', 'CASE', $2)`,
		activityAttCaseID, activityAttProjectID)
	mustExec(`INSERT INTO "case" (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, activityAttCaseID)

	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

	comment := func(id string, ts time.Time, content string) {
		mustExec(`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		          VALUES ($1, $2, $3, 'COMMENT', $4, $5)`, id, ts, activityAttUploader, activityAttCaseID, content)
	}
	synced := func(id string, ts time.Time, name, createdBy, state string) {
		mustExec(`INSERT INTO work_item_attachment (id, created_on, updated_on, created_by, updated_by, name, content_type, work_item_id, size_bytes, state)
		          VALUES ($1, $2, $2, $3, $3, $4, 'text/plain', $5, 42, $6::work_item_attachment_state_enum)`,
			id, ts, createdBy, name, activityAttCaseID, state)
	}

	const (
		commentOne    = "5a000000-0000-0000-0000-0000000000c1"
		commentTwo    = "5a000000-0000-0000-0000-0000000000c2"
		syncedOnly    = "5a000000-0000-0000-0000-0000000000a1"
		syncedPending = "5a000000-0000-0000-0000-0000000000a2"
		inBothTables  = "5a000000-0000-0000-0000-0000000000a3"
		syncedGhost   = "5a000000-0000-0000-0000-0000000000a4"
		nativeWithDsc = "5a000000-0000-0000-0000-0000000000a5"
		syncedNoName  = "5a000000-0000-0000-0000-0000000000a6"
	)
	comment(commentOne, at(0), "first")
	synced(syncedOnly, at(1), "synced.txt", activityAttUploader, "AVAILABLE")
	comment(commentTwo, at(2), "second")
	synced(syncedPending, at(3), "pending.txt", activityAttUploader, "PENDING")
	// An old upload left in case_attachment under the same id as a stored
	// attachment: only the work_item_attachment row is in the feed.
	mustExec(`INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status, created_on)
	          VALUES ($1, $2, NULL, 'both.txt', 'text/plain', 42, $3, 'complete', $4)`,
		inBothTables, activityAttCaseID, activityAttUserID1, at(4))
	synced(inBothTables, at(4), "both.txt", activityAttUploader, "AVAILABLE")
	// Uploader with no user row at all.
	synced(syncedGhost, at(5), "ghost.txt", "nobody@example.com", "AVAILABLE")
	// An old upload with a description (not in the feed any more), and a
	// stored row with a blank name.
	mustExec(`INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, status, created_on)
	          VALUES ($1, $2, 'cases/native/log.txt', 'log.txt', 'text/plain', 7, 'build log', $3, 'complete', $4)`,
		nativeWithDsc, activityAttCaseID, activityAttUserID1, at(6))
	synced(syncedNoName, at(7), " ", activityAttUploader, "AVAILABLE")

	repo := repository.NewCaseRepository(scoped)
	activity, total, err := repo.SearchCaseActivities(ctx, domain.SearchCaseActivitiesRequest{
		CaseID:     activityAttCaseID,
		Pagination: domain.Pagination{Limit: 50},
	})
	if err != nil {
		t.Fatalf("SearchCaseActivities: %v", err)
	}

	// Newest first. The PENDING row and the case_attachment rows are absent,
	// and the unresolved uploader's row is still returned.
	wantIDs := []string{syncedNoName, syncedGhost, inBothTables, commentTwo, syncedOnly, commentOne}
	if total != len(wantIDs) {
		t.Errorf("total = %d, want %d", total, len(wantIDs))
	}
	if len(activity) != len(wantIDs) {
		t.Fatalf("got %d rows, want %d: %+v", len(activity), len(wantIDs), activity)
	}
	if len(activity) != total {
		t.Errorf("rows (%d) and total (%d) disagree", len(activity), total)
	}
	for i, id := range wantIDs {
		if activity[i].ID != id {
			t.Errorf("row %d id = %s, want %s", i, activity[i].ID, id)
		}
	}

	byID := map[string]domain.CaseActivity{}
	for _, a := range activity {
		byID[a.ID] = a
	}
	s := byID[syncedOnly]
	if s.Type != domain.ActivityTypeAttachment || s.FileName != "synced.txt" || s.ContentType != "text/plain" || s.SizeBytes != 42 {
		t.Errorf("synced attachment = %+v", s)
	}
	if s.CreatedBy == nil || s.CreatedBy.Email != activityAttUploader || s.CreatedBy.Name != "Jane Doe" || s.CreatedByFirstName != "Jane" || s.CreatedByLastName != "Doe" {
		t.Errorf("synced attachment uploader = %+v (%s %s)", s.CreatedBy, s.CreatedByFirstName, s.CreatedByLastName)
	}
	g := byID[syncedGhost]
	if g.Type != domain.ActivityTypeAttachment || g.FileName != "ghost.txt" || g.CreatedBy == nil || g.CreatedBy.Email != "nobody@example.com" {
		t.Errorf("unresolved-uploader attachment = %+v", g)
	}

	// Attachment entries never come back with empty content: the customer
	// portal's activity tab hides any entry with no displayable text.
	for id, want := range map[string]string{
		syncedOnly:   "synced.txt",
		inBothTables: "both.txt",
		syncedGhost:  "ghost.txt",
		syncedNoName: "Attachment", // blank name: the fixed label
	} {
		if got := byID[id].Content; got != want {
			t.Errorf("content of %s = %q, want %q", id, got, want)
		}
	}

	// A page smaller than the feed still reports the full total.
	page, pageTotal, err := repo.SearchCaseActivities(ctx, domain.SearchCaseActivitiesRequest{
		CaseID:     activityAttCaseID,
		Pagination: domain.Pagination{Limit: 2, Offset: 1},
	})
	if err != nil {
		t.Fatalf("SearchCaseActivities (paged): %v", err)
	}
	if _, ok := byID[nativeWithDsc]; ok {
		t.Errorf("case_attachment row %s should not be in the feed", nativeWithDsc)
	}
	if pageTotal != len(wantIDs) || len(page) != 2 || page[0].ID != syncedGhost {
		t.Errorf("paged: total=%d rows=%d first=%v", pageTotal, len(page), page)
	}
}
