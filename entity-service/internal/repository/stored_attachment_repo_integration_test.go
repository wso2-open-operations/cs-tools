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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// StoredAttachmentRepository against a real Postgres with every migration
// applied (0144/0145 columns, 0150 comment_attachment.work_item_id, 100026 row-
// level security) and row-level security in force: connect as a role that does
// NOT bypass RLS. Skipped without STORED_ATTACHMENT_TEST_DSN.
//
//	STORED_ATTACHMENT_TEST_DSN=postgres://... go test ./internal/repository/ -run StoredAttachment

package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	saAccount     = "5a000000-0000-0000-0000-000000000001"
	saProject1    = "5a000000-0000-0000-0000-000000000011" // the customer is a member
	saProject2    = "5a000000-0000-0000-0000-000000000012" // the customer is not
	saWIOpen      = "5a000000-0000-0000-0000-000000000021"
	saWIClosed    = "5a000000-0000-0000-0000-000000000022"
	saWIOther     = "5a000000-0000-0000-0000-000000000023" // in project 2
	saWINoProj    = "5a000000-0000-0000-0000-000000000024"
	saDeployment  = "5a000000-0000-0000-0000-000000000031"
	saDeployment2 = "5a000000-0000-0000-0000-000000000032" // in project 2
	saUser        = "5a000000-0000-0000-0000-000000000041"
	saContact     = "5a000000-0000-0000-0000-000000000042"
	saCustomer    = "sa-customer@example.com"
	saStranger    = "sa-stranger@example.com"
)

func storedAttachmentFixture(t *testing.T) (*pgxpool.Pool, repository.StoredAttachmentRepository) {
	t.Helper()
	dsn := os.Getenv("STORED_ATTACHMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("STORED_ATTACHMENT_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		for _, tbl := range []string{"work_item_attachment", "comment_attachment", "deployment_attachment"} {
			_, _ = scoped.Exec(sys, `DELETE FROM `+tbl+` WHERE created_by IN ($1, $2, 'sa-seed')`, saCustomer, saStranger)
		}
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id IN ($1, $2, $3, $4)`, saWIOpen, saWIClosed, saWIOther, saWINoProj)
		_, _ = scoped.Exec(sys, `DELETE FROM deployment WHERE id IN ($1, $2)`, saDeployment, saDeployment2)
		_, _ = scoped.Exec(sys, `DELETE FROM project_contact WHERE email = $1`, saCustomer)
		_, _ = scoped.Exec(sys, `DELETE FROM project WHERE id IN ($1, $2)`, saProject1, saProject2)
		_, _ = scoped.Exec(sys, `DELETE FROM account_contact WHERE id = $1`, saContact)
		_, _ = scoped.Exec(sys, `DELETE FROM "user" WHERE id = $1`, saUser)
		_, _ = scoped.Exec(sys, `DELETE FROM account WHERE id = $1`, saAccount)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	for _, s := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, $2, $2, 't', 't', 'SA Account', 'SA-ACC')`, []any{saAccount, now}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, number, account_id) VALUES ($1, $2, $2, 't', 't', 'SAPROJ1', 'SA 1', 'SA-PRJ-1', $3), ($4, $2, $2, 't', 't', 'SAPROJ2', 'SA 2', 'SA-PRJ-2', $3)`, []any{saProject1, now, saAccount, saProject2}},
		{`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id) VALUES ($1, $2, $2, 't', 't', 'SA-DEP-1', 'SA prod', true, $3), ($4, $2, $2, 't', 't', 'SA-DEP-2', 'SA other', true, $5)`, []any{saDeployment, now, saProject1, saDeployment2, saProject2}},
		{`INSERT INTO "user" (id, created_on, updated_on, user_name, email, user_type) VALUES ($1, $2, $2, 'sa', $3, 'EXTERNAL')`, []any{saUser, now, saCustomer}},
		{`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id) VALUES ($1, $2, $2, 't', 't', 'SA Customer', $3)`, []any{saContact, now, saAccount}},
		{`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state) VALUES (gen_random_uuid(), $1, $1, 't', 't', $2, $3, $4, 'REGISTERED')`, []any{now, saCustomer, saContact, saProject1}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id) VALUES
			($1, $5, $5, 't', 't', 'SA-CS-1', 'WSO2-SA1', 'open', 'CASE', $6),
			($2, $5, $5, 't', 't', 'SA-CS-2', 'WSO2-SA2', 'closed', 'CASE', $6),
			($3, $5, $5, 't', 't', 'SA-CS-3', 'WSO2-SA3', 'other project', 'CASE', $7),
			($4, $5, $5, 't', 't', 'SA-CS-4', 'WSO2-SA4', 'no project', 'CASE', NULL)`, []any{saWIOpen, saWIClosed, saWIOther, saWINoProj, now, saProject1, saProject2}},
		{`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN'::case_state_enum), ($2, 'CLOSED'::case_state_enum), ($3, 'OPEN'::case_state_enum), ($4, 'OPEN'::case_state_enum)`, []any{saWIOpen, saWIClosed, saWIOther, saWINoProj}},
	} {
		if _, err := scoped.Exec(sys, s.sql, s.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", s.sql, err)
		}
	}
	return pool, repository.NewStoredAttachmentRepository(scoped)
}

func saCustomerCtx() context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: saCustomer})
}

func saStrangerCtx() context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: saStranger})
}

func saSystemCtx() context.Context { return repository.WithSystemIdentity(context.Background()) }

func TestStoredAttachment_GetParent(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	kind := domain.StoredAttachmentWorkItem

	open, err := repo.GetParent(saCustomerCtx(), kind, saWIOpen)
	if err != nil {
		t.Fatalf("GetParent(open): %v", err)
	}
	if open.Number != "SA-CS-1" || open.ProjectNumber != "SA-PRJ-1" || open.Closed {
		t.Errorf("open parent = %+v", open)
	}
	closed, err := repo.GetParent(saCustomerCtx(), domain.StoredAttachmentComment, saWIClosed)
	if err != nil {
		t.Fatalf("GetParent(closed): %v", err)
	}
	if !closed.Closed {
		t.Errorf("a CLOSED case reads as open: %+v", closed)
	}

	// A member sees only their project's parents; everything else is "not found".
	for name, id := range map[string]string{"other project": saWIOther, "no project": saWINoProj} {
		_, err := repo.GetParent(saCustomerCtx(), kind, id)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("%s: err = %v, want NotFoundError", name, err)
		}
	}
	if _, err := repo.GetParent(saStrangerCtx(), kind, saWIOpen); err == nil {
		t.Error("a caller with no membership could read the parent")
	}
	// Internal callers see them all, including the one with no project.
	p, err := repo.GetParent(saSystemCtx(), kind, saWINoProj)
	if err != nil || p.ProjectNumber != "" {
		t.Errorf("internal GetParent(no project) = %+v, %v", p, err)
	}

	dep, err := repo.GetParent(saCustomerCtx(), domain.StoredAttachmentDeployment, saDeployment)
	if err != nil || dep.Number != "SA-DEP-1" || dep.ProjectNumber != "SA-PRJ-1" || dep.Closed {
		t.Errorf("deployment parent = %+v, %v", dep, err)
	}
}

func TestStoredAttachment_InsertPendingFollowsMembership(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	in := func(parent string) domain.StoredAttachmentInsert {
		return domain.StoredAttachmentInsert{ParentID: parent, Name: "a.png", ContentType: "image/png", SizeBytes: 12, CreatedBy: saCustomer}
	}
	for _, tc := range []struct {
		kind   domain.StoredAttachmentKind
		ok     string
		denied string
		prefix string
	}{
		{domain.StoredAttachmentWorkItem, saWIOpen, saWIOther, "WIA"},
		{domain.StoredAttachmentComment, saWIOpen, saWIOther, "CA"},
		{domain.StoredAttachmentDeployment, saDeployment, saDeployment2, "DA"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			row, err := repo.InsertPending(saCustomerCtx(), tc.kind, in(tc.ok))
			if err != nil {
				t.Fatalf("a member's insert was refused: %v", err)
			}
			if row.ID == "" || !strings.HasPrefix(row.Number, tc.prefix) || row.Status != "PENDING" || row.StorageKey != nil || row.ParentID != tc.ok {
				t.Errorf("row = %+v", row)
			}
			_, err = repo.InsertPending(saCustomerCtx(), tc.kind, in(tc.denied))
			var nf *apierror.NotFoundError
			if !errors.As(err, &nf) {
				t.Errorf("insert under another project's parent: err = %v, want NotFoundError", err)
			}
			// A caller with no membership at all is refused too.
			stranger := in(tc.ok)
			stranger.CreatedBy = saStranger
			if _, err := repo.InsertPending(saStrangerCtx(), tc.kind, stranger); err == nil {
				t.Error("a non-member could add an attachment")
			}
		})
	}
}

func TestStoredAttachment_ReadsFollowTheParent(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	kind := domain.StoredAttachmentWorkItem
	// An internal caller adds one under each of two projects.
	seed := func(parent string) string {
		row, err := repo.InsertPending(saSystemCtx(), kind, domain.StoredAttachmentInsert{ParentID: parent, Name: "x.txt", ContentType: "text/plain", SizeBytes: 1, CreatedBy: "sa-seed"})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		return row.ID
	}
	mine, theirs := seed(saWIOpen), seed(saWIOther)

	got, err := repo.GetByIDs(saCustomerCtx(), kind, []string{mine, theirs})
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != mine {
		t.Errorf("a member read %d rows, want only their own project's", len(got))
	}
	if got, _ := repo.GetByIDs(saStrangerCtx(), kind, []string{mine, theirs}); len(got) != 0 {
		t.Errorf("a non-member read %d rows", len(got))
	}
	if got, _ := repo.GetByIDs(saSystemCtx(), kind, []string{mine, theirs}); len(got) != 2 {
		t.Errorf("an internal caller read %d rows, want 2", len(got))
	}
}

func TestStoredAttachment_UnlinkedInlineImageIsPrivateToItsUploader(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	kind := domain.StoredAttachmentComment
	row, err := repo.InsertPending(saCustomerCtx(), kind, domain.StoredAttachmentInsert{ParentID: saWIOpen, Name: "i.png", ContentType: "image/png", SizeBytes: 5, CreatedBy: saCustomer})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got, _ := repo.GetByIDs(saCustomerCtx(), kind, []string{row.ID}); len(got) != 1 {
		t.Error("the uploader cannot read their own unlinked image")
	}
	if got, _ := repo.GetByIDs(saStrangerCtx(), kind, []string{row.ID}); len(got) != 0 {
		t.Error("another caller can read an image that is not linked to a comment")
	}
}

func TestStoredAttachment_CompletionIsInternalOnlyAndOneShot(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	kind := domain.StoredAttachmentWorkItem
	row, err := repo.InsertPending(saCustomerCtx(), kind, domain.StoredAttachmentInsert{ParentID: saWIOpen, Name: "r.pdf", ContentType: "application/pdf", SizeBytes: 7, CreatedBy: saCustomer})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// UPDATE is internal-only: as the customer it changes nothing.
	ok, err := repo.MarkComplete(saCustomerCtx(), kind, row.ID, "csm/x/r.pdf", 7)
	if err != nil || ok {
		t.Fatalf("customer MarkComplete = %v, %v; want false (no row visible to UPDATE)", ok, err)
	}
	if got, _ := repo.GetByIDs(saCustomerCtx(), kind, []string{row.ID}); got[0].Status != "PENDING" || got[0].StorageKey != nil {
		t.Errorf("a customer changed the row: %+v", got[0])
	}

	ok, err = repo.MarkComplete(saSystemCtx(), kind, row.ID, "csm/x/r.pdf", 7)
	if err != nil || !ok {
		t.Fatalf("system MarkComplete = %v, %v; want true", ok, err)
	}
	got, _ := repo.GetByIDs(saCustomerCtx(), kind, []string{row.ID})
	if got[0].Status != "COMPLETE" || got[0].StorageKey == nil || *got[0].StorageKey != "csm/x/r.pdf" {
		t.Errorf("row after completion = %+v", got[0])
	}
	// A second completion finds nothing PENDING.
	if ok, _ := repo.MarkComplete(saSystemCtx(), kind, row.ID, "csm/other", 7); ok {
		t.Error("a COMPLETE row was completed again")
	}
}

func TestStoredAttachment_FailedRowCannotBeCompleted(t *testing.T) {
	_, repo := storedAttachmentFixture(t)
	kind := domain.StoredAttachmentDeployment
	row, err := repo.InsertPending(saCustomerCtx(), kind, domain.StoredAttachmentInsert{ParentID: saDeployment, Name: "c.zip", ContentType: "application/zip", SizeBytes: 3, CreatedBy: saCustomer})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.MarkFailed(saSystemCtx(), kind, row.ID, strings.Repeat("x", 900)); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, _ := repo.GetByIDs(saCustomerCtx(), kind, []string{row.ID})
	if got[0].Status != "FAILED" {
		t.Errorf("status = %s, want FAILED", got[0].Status)
	}
	if ok, _ := repo.MarkComplete(saSystemCtx(), kind, row.ID, "csm/k", 3); ok {
		t.Error("a FAILED upload was completed")
	}
}

// MarkDeleted soft-deletes a COMPLETE attachment (once), records who and when,
// and the lists then skip it, along with a failed upload and an upload still in
// flight, while a migrated row that is merely waiting for its file stays listed.
func TestStoredAttachment_MarkDeletedAndTheLists(t *testing.T) {
	pool, repo := storedAttachmentFixture(t)
	ctx := saSystemCtx()
	scoped := repository.NewScoped(pool)
	cases := repository.NewCaseRepository(scoped)

	const (
		failedID   = "5a000000-0000-0000-0000-0000000000f1"
		inFlightID = "5a000000-0000-0000-0000-0000000000f2"
		legacyID   = "5a000000-0000-0000-0000-0000000000f3"
	)
	insert := func(id, status, state string) {
		t.Helper()
		if _, err := scoped.Exec(ctx, `
			INSERT INTO work_item_attachment (id, created_on, updated_on, created_by, updated_by, name, work_item_id, size_bytes, status, state)
			VALUES ($1, now(), now(), 'sa-seed', 'sa-seed', 'seed.bin', $2, 5, $3::attachment_status_enum, NULLIF($4, '')::work_item_attachment_state_enum)`,
			id, saWIOpen, status, state); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	insert(failedID, "FAILED", "")
	insert(inFlightID, "PENDING", "")        // an upload the portal started: no legacy state
	insert(legacyID, "PENDING", "AVAILABLE") // a migrated row waiting for its file

	row, err := repo.InsertPending(saCustomerCtx(), domain.StoredAttachmentWorkItem, domain.StoredAttachmentInsert{
		ParentID: saWIOpen, Name: "a.pdf", ContentType: "application/pdf", SizeBytes: 10, CreatedBy: saCustomer,
	})
	if err != nil {
		t.Fatalf("InsertPending: %v", err)
	}

	listed := func() map[string]bool {
		t.Helper()
		got, _, err := cases.SearchCaseAttachments(ctx, saWIOpen, domain.Pagination{Limit: 50})
		if err != nil {
			t.Fatalf("SearchCaseAttachments: %v", err)
		}
		out := map[string]bool{}
		for _, a := range got {
			out[a.ID] = true
		}
		return out
	}

	// Only a COMPLETE attachment can be deleted.
	if ok, err := repo.MarkDeleted(ctx, domain.StoredAttachmentWorkItem, row.ID, saCustomer); err != nil || ok {
		t.Fatalf("MarkDeleted on a PENDING row = %v, %v; want false, nil", ok, err)
	}
	if l := listed(); l[row.ID] || l[failedID] || l[inFlightID] || !l[legacyID] {
		t.Errorf("before completion: listed = %v, want only the migrated row", l)
	}

	if ok, err := repo.MarkComplete(ctx, domain.StoredAttachmentWorkItem, row.ID, "csm/x/wia_a.pdf", 10); err != nil || !ok {
		t.Fatalf("MarkComplete = %v, %v", ok, err)
	}
	if l := listed(); !l[row.ID] || !l[legacyID] || l[failedID] || l[inFlightID] {
		t.Errorf("after completion: listed = %v", l)
	}

	if ok, err := repo.MarkDeleted(ctx, domain.StoredAttachmentWorkItem, row.ID, saCustomer); err != nil || !ok {
		t.Fatalf("MarkDeleted = %v, %v; want true", ok, err)
	}
	if ok, err := repo.MarkDeleted(ctx, domain.StoredAttachmentWorkItem, row.ID, saCustomer); err != nil || ok {
		t.Errorf("a second MarkDeleted = %v, %v; want false (already deleted)", ok, err)
	}

	var status, by string
	var on *time.Time
	if err := scoped.QueryRow(ctx, `SELECT status::text, COALESCE(deleted_by, ''), deleted_on FROM work_item_attachment WHERE id = $1`, row.ID).Scan(&status, &by, &on); err != nil {
		t.Fatal(err)
	}
	if status != "DELETED" || by != saCustomer || on == nil {
		t.Errorf("row = %s / %q / %v, want DELETED by the caller with a time", status, by, on)
	}
	if l := listed(); l[row.ID] || !l[legacyID] {
		t.Errorf("after delete: listed = %v, want the deleted row gone and the migrated row kept", l)
	}

	// The caller cannot reach it any other way either: UPDATE is internal-only.
	if ok, err := repo.MarkDeleted(saCustomerCtx(), domain.StoredAttachmentWorkItem, legacyID, saCustomer); err == nil && ok {
		t.Error("an external caller marked an attachment deleted directly")
	}
}
